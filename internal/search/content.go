package search

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"

	"github.com/davidpaquet/claude-session-browser/internal/model"
)

type ContentEngine interface {
	SearchContent(ctx context.Context, query string, sessions []model.SessionInfo) ([]SearchResult, error)
}

type contentEngine struct {
	maxWorkers int
	rgPath     string
}

func NewContentEngine() ContentEngine {
	return &contentEngine{
		maxWorkers: 4,
		rgPath:     findRipgrep(),
	}
}

func findRipgrep() string {
	// Try common ripgrep locations
	paths := []string{
		"rg",
		"/usr/local/bin/rg",
		"/usr/bin/rg",
		"/opt/homebrew/bin/rg",
		"/opt/homebrew/Cellar/ripgrep/14.1.1/bin/rg",
	}
	
	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
	}
	
	// Fallback to "rg" and hope it's in PATH
	return "rg"
}

type searchJob struct {
	query        string
	session      model.SessionInfo
	sessionIndex int
}

func (c *contentEngine) SearchContent(ctx context.Context, query string, sessions []model.SessionInfo) ([]SearchResult, error) {
	jobs := make(chan searchJob, len(sessions))
	results := make(chan SearchResult, len(sessions))
	
	var wg sync.WaitGroup
	
	// Start workers
	for i := 0; i < c.maxWorkers; i++ {
		wg.Add(1)
		go c.worker(ctx, &wg, jobs, results)
	}
	
	// Queue jobs
	for i, session := range sessions {
		select {
		case <-ctx.Done():
			close(jobs)
			return nil, ctx.Err()
		case jobs <- searchJob{
			query:        query,
			session:      session,
			sessionIndex: i,
		}:
		}
	}
	close(jobs)
	
	// Wait and collect results
	go func() {
		wg.Wait()
		close(results)
	}()
	
	var searchResults []SearchResult
	for result := range results {
		if len(result.Matches) > 0 {
			searchResults = append(searchResults, result)
		}
	}
	
	return searchResults, nil
}

func (c *contentEngine) worker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan searchJob, results chan<- SearchResult) {
	defer wg.Done()
	
	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
			matches, err := c.searchFile(job.query, job.session.FilePath)
			if err == nil && len(matches) > 0 {
				results <- SearchResult{
					SessionID:    job.session.ID,
					SessionIndex: job.sessionIndex,
					Matches:      matches,
					Score:        float64(len(matches)),
				}
			}
		}
	}
}

func (c *contentEngine) searchFile(query, filePath string) ([]Match, error) {
	cmd := exec.Command(c.rgPath,
		"--json",
		"--max-count", "20", // Limit matches per file
		"--context", "1",    // Lines of context
		"--ignore-case",     // Correct flag name
		query,
		filePath,
	)
	
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no matches, which is not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	
	var matches []Match
	scanner := bufio.NewScanner(bytes.NewReader(output))
	
	for scanner.Scan() {
		var result map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
			continue
		}
		
		if result["type"] == "match" {
			if data, ok := result["data"].(map[string]interface{}); ok {
				match := Match{}
				
				// Extract line number
				if lineNumber, ok := data["line_number"].(float64); ok {
					match.LineNumber = int(lineNumber)
				}
				
				// Extract the matched text and create context
				if lines, ok := data["lines"].(map[string]interface{}); ok {
					if text, ok := lines["text"].(string); ok {
						match.Text = text
						
						// Extract match positions
						var matchStartPos, matchEndPos int
						hasPositions := false
						
						if submatches, ok := data["submatches"].([]interface{}); ok && len(submatches) > 0 {
							if submatch, ok := submatches[0].(map[string]interface{}); ok {
								if start, ok := submatch["start"].(float64); ok {
									matchStartPos = int(start)
									match.StartOffset = matchStartPos
									hasPositions = true
								}
								if end, ok := submatch["end"].(float64); ok {
									matchEndPos = int(end)
									match.EndOffset = matchEndPos
								}
							}
						}
						
						// Create context around the match
						if hasPositions {
							match.Context = extractContext(text, matchStartPos, matchEndPos)
						}
					}
				}
				
				matches = append(matches, match)
			}
		}
	}
	
	return matches, nil
}

// extractContext extracts meaningful context around a match in a JSON line
func extractContext(text string, matchStart, matchEnd int) string {
	// If this looks like a Claude message JSON, extract just the content
	if strings.Contains(text, `"content":"`) {
		contentStart := strings.Index(text, `"content":"`)
		if contentStart != -1 {
			contentStart += len(`"content":"`)
			contentEnd := strings.Index(text[contentStart:], `"`)
			if contentEnd != -1 {
				content := text[contentStart : contentStart+contentEnd]
				
				// Check if the match is within the content field
				if matchStart >= contentStart && matchStart < contentStart+contentEnd {
					// Calculate position within content
					posInContent := matchStart - contentStart
					
					// Get context around the match (50 chars before and after)
					contextStart := posInContent - 50
					if contextStart < 0 {
						contextStart = 0
					}
					contextEnd := posInContent + (matchEnd - matchStart) + 50
					if contextEnd > len(content) {
						contextEnd = len(content)
					}
					
					// Build context with ellipsis
					var result strings.Builder
					if contextStart > 0 {
						result.WriteString("...")
					}
					result.WriteString(content[contextStart:contextEnd])
					if contextEnd < len(content) {
						result.WriteString("...")
					}
					
					return result.String()
				}
			}
		}
	}
	
	// For non-JSON content or if match is outside content field,
	// just show context around the match
	contextStart := matchStart - 30
	if contextStart < 0 {
		contextStart = 0
	}
	contextEnd := matchEnd + 30
	if contextEnd > len(text) {
		contextEnd = len(text)
	}
	
	var result strings.Builder
	if contextStart > 0 {
		result.WriteString("...")
	}
	result.WriteString(text[contextStart:contextEnd])
	if contextEnd < len(text) {
		result.WriteString("...")
	}
	
	return result.String()
}