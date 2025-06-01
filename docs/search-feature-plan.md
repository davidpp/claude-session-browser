# Search Feature Implementation Plan

## Overview

This document outlines the implementation plan for adding search functionality to Claude Session Browser, including both quick filtering and deep content search capabilities.

## Product Requirements

### User Needs
- **Primary Users**: Developers with dozens to hundreds of Claude sessions
- **Core Problem**: Manually scanning through sessions to find specific conversations

### Use Cases
1. Find specific code discussions: "Where did I discuss implementing OAuth?"
2. Locate error solutions: "Which session had the fix for that webpack error?"
3. Resume related work: "Find all sessions about the React refactoring"
4. Audit/review: "What sessions touched the payment system?"

### Success Criteria
- Search results appear in <500ms
- Minimal false positives
- Single keystroke activation
- Seamless integration with existing UI

## UX Design

### Search Modes
1. **Quick Filter** (`/` key) - Filters the session list by title/metadata
2. **Deep Search** (`Ctrl+F` key) - Searches inside session content

### Visual Design
```
┌─────────────────────────────────────────────────────────────┐
│ Claude Session Browser                                      │
├─────────────────────────┬───────────────────────────────────┤
│ Sessions (12 matches)   │ Session Details                   │
│                         │                                   │
│ [🔍] project-xyz (3)    │ Search Matches:                   │
│ > session-abc           │ ─────────────────────────────     │
│   session-def           │ "...implementing OAuth2 flow..."  │
│                         │ "...OAuth token refresh..."       │
├─────────────────────────┴───────────────────────────────────┤
│ 🔍 Search: OAuth                                    [Esc] ✕ │
└─────────────────────────────────────────────────────────────┘
```

### Interaction Flow
- `/` activates quick filter with real-time results
- `Ctrl+F` activates deep search with async results
- `Esc` exits search mode
- `Enter` selects first/current result
- Match highlighting in yellow
- Search icon [🔍] indicates content matches

## Technical Architecture

### Package Structure
```
internal/
├── search/
│   ├── engine.go      // Core search interface
│   ├── filter.go      // Quick filter using fuzzy search
│   ├── content.go     // Deep search using ripgrep
│   └── cache.go       // Search result caching
```

### Core Interfaces
```go
type SearchEngine interface {
    Search(ctx context.Context, query string, searchType SearchType) ([]SearchResult, error)
    ClearCache()
}

type SearchResult struct {
    SessionID    string
    SessionIndex int
    Matches      []Match
    Score        float64
}
```

### Implementation Strategy
1. **Quick Filter**: Use `github.com/sahilm/fuzzy` for in-memory fuzzy search
2. **Deep Search**: Use ripgrep (`rg`) as external process for file content search
3. **Performance**: Worker pool for parallel search, 5-minute result cache
4. **UI Integration**: Bubble Tea commands for async search operations

## Implementation Phases

### Phase 1: Foundation (MVP - Quick Filter)

#### 1.1 Add Dependencies
- Add `github.com/sahilm/fuzzy` for fuzzy search
- Add `github.com/charmbracelet/bubbles/textinput` for search input

#### 1.2 Create Search Package
- Create `internal/search/` directory
- Implement basic `SearchEngine` interface
- Implement `FilterEngine` for session list filtering

#### 1.3 Update UI Model
- Add search state fields to `Model` struct
- Initialize search input component
- Add search mode enum

#### 1.4 Implement Search Activation
- Handle `/` key to enter search mode
- Focus search input
- Display search bar at bottom

#### 1.5 Implement Live Filtering
- Hook up fuzzy search to filter sessions
- Update session list display
- Highlight matching text

#### 1.6 Search Navigation
- Handle Esc to exit search
- Handle Enter to select result
- Maintain selection state during search

### Phase 2: Deep Content Search

#### 2.1 Implement Content Engine
- Create ripgrep wrapper
- Parse JSON output from rg
- Handle ripgrep not installed gracefully

#### 2.2 Add Deep Search Mode
- Handle `Ctrl+F` for content search
- Differentiate UI between filter/content modes
- Show search progress indicator

#### 2.3 Display Content Matches
- Show match count in session list
- Display match previews in details pane
- Add search icon indicator

#### 2.4 Async Search Implementation
- Implement search command with tea.Cmd
- Add timeout handling
- Show loading state

### Phase 3: Polish & Optimization

#### 3.1 Add Search Caching
- Implement simple TTL cache
- Cache content search results
- Clear cache on session refresh

#### 3.2 Performance Optimization
- Add search debouncing (300ms)
- Implement worker pool for parallel search
- Limit results for better performance

#### 3.3 Enhanced UX
- Add match highlighting with colors
- Show "No results" message
- Add search statistics

#### 3.4 Error Handling
- Handle ripgrep errors gracefully
- Add fallback for missing ripgrep
- User-friendly error messages

## Implementation Order

1. Basic search infrastructure (1.1-1.3)
2. Quick filter functionality (1.4-1.6)
3. Test and refine filter mode
4. Add content search (2.1-2.4)
5. Performance and polish (3.1-3.4)

## Dependencies

### Go Modules to Add
```
github.com/sahilm/fuzzy v0.1.0
github.com/charmbracelet/bubbles v0.18.0
```

### External Tools
- ripgrep (`rg`) - for content search (optional, with graceful fallback)

## Testing Strategy

1. Unit tests for search engines
2. Integration tests for UI search flow
3. Performance benchmarks for large session counts
4. Manual testing of keyboard interactions

## Estimated Timeline

- Phase 1: 2-3 hours (MVP working)
- Phase 2: 3-4 hours (Full feature set)
- Phase 3: 2-3 hours (Production ready)

Total: 7-10 hours for complete implementation