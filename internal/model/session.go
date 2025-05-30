package model

import (
	"path/filepath"
	"strings"
	"time"
)

// SessionInfo represents minimal session info for listing
type SessionInfo struct {
	ID         string
	FilePath   string
	LastActive time.Time
}

// GetSessionID extracts the session ID from a filename
func GetSessionID(filename string) string {
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, ".jsonl")
}

// FullSession represents a fully parsed session
type FullSession struct {
	ID              string
	FilePath        string
	Summary         string
	LastActive      time.Time
	MessageCount    int
	TotalCostUSD    float64
	LastRawMessages []string
}

// GetResumeCommand returns the command to resume this session
func (s *FullSession) GetResumeCommand() string {
	return "claude --resume " + s.ID
}