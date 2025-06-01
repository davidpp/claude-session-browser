package search

import (
	"fmt"
	"strings"

	"github.com/davidpaquet/claude-session-browser/internal/model"
	"github.com/sahilm/fuzzy"
)

type FilterEngine interface {
	Filter(query string, sessions []model.SessionInfo) []SearchResult
}

type filterEngine struct{}

func NewFilterEngine() FilterEngine {
	return &filterEngine{}
}

type sessionSource struct {
	sessions []model.SessionInfo
}

func (s sessionSource) String(i int) string {
	session := s.sessions[i]
	// Combine searchable fields for better matching
	searchText := fmt.Sprintf("%s %s", 
		session.ID,
		session.LastActive.Format("2006-01-02 15:04"),
	)
	return searchText
}

func (s sessionSource) Len() int {
	return len(s.sessions)
}

func (f *filterEngine) Filter(query string, sessions []model.SessionInfo) []SearchResult {
	if query == "" {
		// Return all sessions when query is empty
		results := make([]SearchResult, len(sessions))
		for i, session := range sessions {
			results[i] = SearchResult{
				SessionID:    session.ID,
				SessionIndex: i,
				Score:        1.0,
			}
		}
		return results
	}

	source := sessionSource{sessions: sessions}
	matches := fuzzy.FindFrom(query, source)

	results := make([]SearchResult, 0, len(matches))
	for _, match := range matches {
		// Extract match positions for highlighting
		matchIndices := make([]Match, 0)
		for _, idx := range match.MatchedIndexes {
			matchIndices = append(matchIndices, Match{
				StartOffset: idx,
				EndOffset:   idx + 1,
			})
		}

		results = append(results, SearchResult{
			SessionID:    sessions[match.Index].ID,
			SessionIndex: match.Index,
			Score:        float64(match.Score),
			Matches:      matchIndices,
		})
	}

	return results
}

// HighlightText applies highlighting to matched characters
func HighlightText(text string, indices []int, highlightStyle func(string) string) string {
	if len(indices) == 0 {
		return text
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(text)
	var result strings.Builder

	// Create a map for faster lookup
	indexMap := make(map[int]bool)
	for _, idx := range indices {
		if idx < len(runes) {
			indexMap[idx] = true
		}
	}

	for i, r := range runes {
		if indexMap[i] {
			result.WriteString(highlightStyle(string(r)))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}