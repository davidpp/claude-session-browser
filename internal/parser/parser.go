package parser

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidpaquet/claude-session-browser/internal/model"
)

// Parser handles parsing
type Parser struct{}

// NewParser creates a new parser
func NewParser() *Parser {
	return &Parser{}
}

// ListSessions returns basic session info without parsing content
func (p *Parser) ListSessions(claudeDir string) ([]model.SessionInfo, error) {
	entries, err := os.ReadDir(claudeDir)
	if err != nil {
		return nil, err
	}

	var sessions []model.SessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		sessions = append(sessions, model.SessionInfo{
			ID:         model.GetSessionID(entry.Name()),
			FilePath:   filepath.Join(claudeDir, entry.Name()),
			LastActive: info.ModTime(), // Use file modification time
		})
	}

	return sessions, nil
}

// ParseFullSession parses a single session with all details
func (p *Parser) ParseFullSession(filePath string) (*model.FullSession, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	session := &model.FullSession{
		ID:       model.GetSessionID(filePath),
		FilePath: filePath,
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var allLines []string
	var lastUserMessages []string
	messageCount := 0
	totalCost := 0.0

	// Read all lines
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		allLines = append(allLines, line)

		// Try to parse for basic info
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(line), &data); err == nil {
			// Count messages
			if msgType, ok := data["type"].(string); ok {
				if msgType == "user" || msgType == "assistant" {
					messageCount++
				}

				// Collect user messages for summary
				if msgType == "user" {
					if msg, ok := data["message"].(map[string]interface{}); ok {
						if content, ok := msg["content"].(string); ok {
							content = strings.TrimSpace(content)
							if !strings.Contains(content, "system-reminder") {
								lastUserMessages = append(lastUserMessages, content)
							}
						}
					}
				}
			}

			// Get timestamp
			if ts, ok := data["timestamp"].(string); ok {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					session.LastActive = t
				}
			}

			// Get cost
			if cost, ok := data["costUSD"].(float64); ok {
				totalCost += cost
			}
		}
	}

	// Set summary from last 3 user messages
	if len(lastUserMessages) > 0 {
		start := len(lastUserMessages) - 3
		if start < 0 {
			start = 0
		}
		summaryParts := []string{}
		for i := start; i < len(lastUserMessages); i++ {
			msg := lastUserMessages[i]
			if len(msg) > 150 {
				msg = msg[:147] + "..."
			}
			summaryParts = append(summaryParts, msg)
		}
		session.Summary = strings.Join(summaryParts, " | ")
	}

	// Get just the LAST raw message - complete and untruncated
	if len(allLines) > 0 {
		session.LastRawMessages = []string{allLines[len(allLines)-1]}
	}

	session.MessageCount = messageCount
	session.TotalCostUSD = totalCost

	return session, nil
}