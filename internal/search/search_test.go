package search

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/davidpaquet/claude-session-browser/internal/model"
)

func TestContentSearch(t *testing.T) {
	// Create a temporary directory with test JSONL files
	tmpDir, err := os.MkdirTemp("", "claude-search-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test JSONL content with known search terms
	testContent1 := `{"type":"message","role":"user","content":"I need help with OAuth implementation"}
{"type":"message","role":"assistant","content":"I'll help you implement OAuth. Here's how..."}
{"type":"message","role":"user","content":"Can you show me the OAuth flow?"}
`

	testContent2 := `{"type":"message","role":"user","content":"My webpack build is failing"}
{"type":"message","role":"assistant","content":"Let me help debug your webpack configuration"}
{"type":"message","role":"user","content":"The error says webpack cannot find module"}
`

	// Write test files
	file1 := filepath.Join(tmpDir, "session1.jsonl")
	file2 := filepath.Join(tmpDir, "session2.jsonl")
	
	if err := os.WriteFile(file1, []byte(testContent1), 0644); err != nil {
		t.Fatalf("Failed to write test file 1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(testContent2), 0644); err != nil {
		t.Fatalf("Failed to write test file 2: %v", err)
	}

	// Create test sessions
	sessions := []model.SessionInfo{
		{
			ID:       "test-session-1",
			FilePath: file1,
		},
		{
			ID:       "test-session-2",
			FilePath: file2,
		},
	}

	// Test the content engine
	engine := NewContentEngine()
	ctx := context.Background()

	// Test 1: Search for "OAuth"
	t.Run("Search for OAuth", func(t *testing.T) {
		results, err := engine.SearchContent(ctx, "OAuth", sessions)
		if err != nil {
			t.Errorf("Search failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result for 'OAuth', got %d", len(results))
		}
		if len(results) > 0 && len(results[0].Matches) != 3 {
			t.Errorf("Expected 3 matches for 'OAuth', got %d", len(results[0].Matches))
		}
	})

	// Test 2: Search for "webpack"
	t.Run("Search for webpack", func(t *testing.T) {
		results, err := engine.SearchContent(ctx, "webpack", sessions)
		if err != nil {
			t.Errorf("Search failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result for 'webpack', got %d", len(results))
		}
		if len(results) > 0 && len(results[0].Matches) != 3 {
			t.Errorf("Expected 3 matches for 'webpack', got %d", len(results[0].Matches))
		}
	})

	// Test 3: Search for term in both files
	t.Run("Search across multiple files", func(t *testing.T) {
		results, err := engine.SearchContent(ctx, "help", sessions)
		if err != nil {
			t.Errorf("Search failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 results for 'help', got %d", len(results))
		}
	})

	// Test 4: Search for non-existent term
	t.Run("Search for non-existent term", func(t *testing.T) {
		results, err := engine.SearchContent(ctx, "nonexistentterm", sessions)
		if err != nil {
			t.Errorf("Search failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results for 'nonexistentterm', got %d", len(results))
		}
	})
}

// Test ripgrep detection
func TestFindRipgrep(t *testing.T) {
	rgPath := findRipgrep()
	if rgPath == "" {
		t.Skip("Ripgrep not found, skipping test")
	}
	
	// Try to execute it
	cmd := exec.Command(rgPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("Failed to execute ripgrep at %s: %v", rgPath, err)
	}
	
	t.Logf("Found ripgrep at: %s", rgPath)
	t.Logf("Version: %s", string(output))
}