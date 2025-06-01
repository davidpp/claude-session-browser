package search

import (
	"os"
	"os/exec"
	"testing"
)

func TestRipgrepDirectly(t *testing.T) {
	// Test ripgrep directly on a simple file
	tmpFile := "/tmp/test-rg.jsonl"
	content := `{"type":"message","role":"user","content":"I need help with OAuth implementation"}
{"type":"message","role":"assistant","content":"I'll help you implement OAuth. Here's how..."}`
	
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer os.Remove(tmpFile)
	
	// Test different ripgrep commands
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "Basic search",
			args: []string{"OAuth", tmpFile},
		},
		{
			name: "Case insensitive",
			args: []string{"-i", "oauth", tmpFile},
		},
		{
			name: "JSON output",
			args: []string{"--json", "OAuth", tmpFile},
		},
		{
			name: "JSON with case insensitive",
			args: []string{"--json", "-i", "oauth", tmpFile},
		},
		{
			name: "Our exact command",
			args: []string{"--json", "--max-count", "20", "--context", "1", "--case-insensitive", "OAuth", tmpFile},
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command("rg", test.args...)
			output, err := cmd.CombinedOutput()
			
			t.Logf("Command: rg %v", test.args)
			t.Logf("Exit code: %v", cmd.ProcessState.ExitCode())
			t.Logf("Error: %v", err)
			t.Logf("Output:\n%s", string(output))
			t.Logf("---")
		})
	}
}

func TestSearchFileDebug(t *testing.T) {
	engine := &contentEngine{
		maxWorkers: 1,
		rgPath:     "rg",
	}
	
	tmpFile := "/tmp/test-search.jsonl"
	content := `{"type":"message","role":"user","content":"I need help with OAuth implementation"}`
	
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer os.Remove(tmpFile)
	
	matches, err := engine.searchFile("OAuth", tmpFile)
	t.Logf("Search result - Error: %v, Matches: %d", err, len(matches))
	for i, match := range matches {
		t.Logf("Match %d: Text=%q, Line=%d, Context=%q", i, match.Text, match.LineNumber, match.Context)
	}
}