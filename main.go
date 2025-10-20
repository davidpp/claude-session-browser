package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidpaquet/claude-session-browser/internal/ui"
)

const version = "v0.2.0"

func main() {
	// Parse command line flags
	var claudeDir string
	flag.StringVar(&claudeDir, "claude-dir", "", "Claude projects directory (default: ~/.claude/projects)")
	flag.StringVar(&claudeDir, "d", "", "Claude projects directory (shorthand)")
	
	var help bool
	flag.BoolVar(&help, "help", false, "Show help")
	flag.BoolVar(&help, "h", false, "Show help (shorthand)")
	
	flag.Parse()
	
	// Show help if requested
	if help {
		showHelp()
		os.Exit(0)
	}
	
	// Set Claude directory
	if claudeDir == "" {
		claudeDir = os.Getenv("CLAUDE_DIR")
	}
	if claudeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("Failed to get home directory:", err)
		}
		claudeDir = filepath.Join(home, ".claude", "projects")
	}
	
	// Set CLAUDE_DIR environment variable for the app
	os.Setenv("CLAUDE_DIR", claudeDir)
	
	// Get current working directory and convert to Claude path format
	cwd, _ := os.Getwd()
	claudePath := convertToClaudePath(cwd)
	
	// Check if this project exists in the Claude directory
	projectPath := filepath.Join(claudeDir, claudePath)
	if _, err := os.Stat(projectPath); err == nil && hasJSONLFiles(projectPath) {
		// Found matching project for current directory
		claudeDir = projectPath
	} else {
		// No match for current directory, just use first project with JSONL files
		entries, err := os.ReadDir(claudeDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					testPath := filepath.Join(claudeDir, entry.Name())
					if hasJSONLFiles(testPath) {
						claudeDir = testPath
						break
					}
				}
			}
		}
	}
	
	app := ui.NewApp(claudeDir, version)
	
	// Create the Bubble Tea program
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(), // Use alternate screen buffer
	)
	
	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatal("Error running program:", err)
	}
}

func convertToClaudePath(path string) string {
	// Convert filesystem path to Claude format
	// e.g., "/Users/davidpaquet/Projects/roo-task-cli" -> "-Users-davidpaquet-Projects-roo-task-cli"
	claudePath := strings.ReplaceAll(path, string(filepath.Separator), "-")
	if !strings.HasPrefix(claudePath, "-") {
		claudePath = "-" + claudePath
	}
	return claudePath
}

func hasJSONLFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
			return true
		}
	}
	return false
}

func showHelp() {
	fmt.Println(`Claude Session Browser

A terminal user interface for browsing and resuming Claude Code sessions.

Usage:
  claude-session-browser [options]

Options:
  -d, --claude-dir PATH    Claude projects directory (default: ~/.claude/projects)
  -h, --help              Show this help message

Environment Variables:
  CLAUDE_DIR              Alternative way to set Claude projects directory

Keyboard Shortcuts:
  ↑/↓, j/k               Navigate sessions
  Enter                  Copy resume command to clipboard
  r                      Refresh session list
  q                      Quit

Examples:
  # Run with default directory
  claude-session-browser

  # Specify custom Claude directory
  claude-session-browser --claude-dir ~/my-claude-projects

  # Use environment variable
  export CLAUDE_DIR=~/my-claude-projects
  claude-session-browser`)
}