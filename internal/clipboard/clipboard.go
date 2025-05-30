package clipboard

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
)

// Manager handles clipboard operations
type Manager struct {
	useNative bool
}

// NewManager creates a new clipboard manager
func NewManager() *Manager {
	return &Manager{
		useNative: true, // Try native methods first
	}
}

// Copy copies text to the clipboard
func (m *Manager) Copy(text string) error {
	// Try the cross-platform library first
	err := clipboard.WriteAll(text)
	if err == nil {
		return nil
	}

	// Fallback to platform-specific commands
	return m.copyWithCommand(text)
}

// copyWithCommand uses platform-specific commands
func (m *Manager) copyWithCommand(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// macOS: use pbcopy
		cmd = exec.Command("pbcopy")
		
	case "linux":
		// Linux: try different clipboard commands
		if m.commandExists("xclip") {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if m.commandExists("xsel") {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else if m.commandExists("wl-copy") {
			// Wayland
			cmd = exec.Command("wl-copy")
		} else {
			return fmt.Errorf("no clipboard command found (install xclip, xsel, or wl-clipboard)")
		}
		
	case "windows":
		// Windows: use clip.exe
		cmd = exec.Command("clip.exe")
		
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Send text to command's stdin
	cmd.Stdin = strings.NewReader(text)
	
	// Execute
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clipboard command failed: %w", err)
	}

	return nil
}

// commandExists checks if a command exists in PATH
func (m *Manager) commandExists(name string) bool {
	cmd := exec.Command("which", name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

