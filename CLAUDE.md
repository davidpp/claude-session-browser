# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claude Session Browser is a Terminal User Interface (TUI) application for browsing and resuming Claude Code sessions. It reads JSONL session files from `~/.claude/projects/` and provides an interactive interface for managing sessions.

## Commands

```bash
# Build the application
go build

# Build with optimizations (smaller binary)
go build -ldflags="-s -w" -o claude-session-browser

# Run tests
go test ./...

# Run the application
./claude-session-browser

# Run with custom Claude directory
./claude-session-browser --claude-dir ~/my-claude-projects
```

## Architecture

The codebase follows a clean modular architecture with all packages under `internal/`:

- **main.go**: Entry point that handles CLI arguments and initializes the Bubble Tea TUI
- **internal/model**: Core data structures (SessionInfo, FullSession) and business logic
- **internal/parser**: JSONL file parsing - both quick listing and full session parsing
- **internal/ui**: Bubble Tea-based TUI with split-pane layout (session list + details)
- **internal/clipboard**: Cross-platform clipboard operations with fallback mechanisms
- **internal/search**: Search functionality with fuzzy filtering and future content search support

Key architectural decisions:
- Sessions are parsed on-demand, not all at once, for performance
- The UI uses Bubble Tea's Model-View-Update pattern for reactivity
- Clipboard operations have both library and command-line fallbacks
- All paths are converted between filesystem and Claude format automatically

## Key Dependencies

- **charmbracelet/bubbletea**: TUI framework providing the reactive architecture
- **charmbracelet/lipgloss**: Terminal styling for borders, colors, and layout
- **charmbracelet/bubbles**: UI components including textinput for search
- **sahilm/fuzzy**: Fuzzy string matching for session search
- **atotto/clipboard**: Cross-platform clipboard library

## Development Notes

- The project uses Go 1.24+ (note the unusually high version requirement)
- GitHub Actions automatically builds releases for 6 platforms when tags are pushed
- The UI color scheme uses purple (#9B59B6) for the session list and green (#2ECC71) for details
- Session summaries are extracted from the last 3 user messages in each session file