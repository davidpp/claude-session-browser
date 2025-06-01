package search

import (
	"os"
	"strings"
	"testing"
)

func TestContextExtraction(t *testing.T) {
	engine := &contentEngine{
		maxWorkers: 1,
		rgPath:     "rg",
	}
	
	// Create test file with realistic Claude session content
	tmpFile := "/tmp/test-context.jsonl"
	content := `{"type":"message","role":"user","content":"I'm working on a React application and need help with implementing OAuth authentication. Can you guide me through the process?"}
{"type":"message","role":"assistant","content":"I'll help you implement OAuth authentication in your React application. Here's a comprehensive guide to get you started with OAuth implementation."}
{"type":"message","role":"user","content":"The OAuth redirect is not working properly, I get an error"}
`
	
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer os.Remove(tmpFile)
	
	matches, err := engine.searchFile("OAuth", tmpFile)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	t.Logf("Found %d matches", len(matches))
	for i, match := range matches {
		t.Logf("Match %d:", i)
		t.Logf("  Line: %d", match.LineNumber)
		t.Logf("  Text: %q", match.Text)
		t.Logf("  Context: %q", match.Context)
		t.Logf("  ---")
	}
	
	// Check that context is extracted properly
	if len(matches) < 1 {
		t.Fatal("Expected at least one match")
	}
	
	// First match should have context around "OAuth authentication"
	if matches[0].Context == "" {
		t.Error("First match should have context")
	}
	
	// Context should not be the full JSON line
	if matches[0].Context == matches[0].Text {
		t.Error("Context should be different from full text")
	}
	
	// Context should contain the search term
	if !strings.Contains(matches[0].Context, "OAuth") {
		t.Error("Context should contain the search term")
	}
}