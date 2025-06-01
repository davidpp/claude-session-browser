package search

import (
	"context"

	"github.com/davidpaquet/claude-session-browser/internal/model"
)

type SearchType int

const (
	SearchTypeFilter SearchType = iota
	SearchTypeContent
)

type SearchResult struct {
	SessionID    string
	SessionIndex int
	Matches      []Match
	Score        float64
}

type Match struct {
	Text        string
	LineNumber  int
	StartOffset int
	EndOffset   int
	Context     string
}

type Engine interface {
	Search(ctx context.Context, query string, searchType SearchType) ([]SearchResult, error)
	UpdateSessions(sessions []model.SessionInfo)
}

type engine struct {
	sessions      []model.SessionInfo
	filterEngine  FilterEngine
	contentEngine ContentEngine
}

func NewEngine(sessions []model.SessionInfo) Engine {
	return &engine{
		sessions:      sessions,
		filterEngine:  NewFilterEngine(),
		contentEngine: NewContentEngine(),
	}
}

func (e *engine) Search(ctx context.Context, query string, searchType SearchType) ([]SearchResult, error) {
	switch searchType {
	case SearchTypeFilter:
		return e.filterEngine.Filter(query, e.sessions), nil
	case SearchTypeContent:
		return e.contentEngine.SearchContent(ctx, query, e.sessions)
	default:
		return []SearchResult{}, nil
	}
}

func (e *engine) UpdateSessions(sessions []model.SessionInfo) {
	e.sessions = sessions
}