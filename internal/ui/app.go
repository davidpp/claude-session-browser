package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidpaquet/claude-session-browser/internal/clipboard"
	"github.com/davidpaquet/claude-session-browser/internal/model"
	"github.com/davidpaquet/claude-session-browser/internal/parser"
)

// Model is the app model
type Model struct {
	// Data
	sessions      []model.SessionInfo
	fullSession   *model.FullSession
	parser        *parser.Parser
	clipboardMgr  *clipboard.Manager
	claudeDir     string
	
	// UI State
	width         int
	height        int
	selected      int
	scrollOffset  int
	loading       bool
	err           error
	
	// Status
	statusMsg     string
	statusTimer   time.Time
}

// NewApp creates a new app
func NewApp(claudeDir string) *Model {
	return &Model{
		parser:       parser.NewParser(),
		clipboardMgr: clipboard.NewManager(),
		claudeDir:    claudeDir,
		loading:      true,
		width:        80,
		height:       24,
	}
}

func (m *Model) Init() tea.Cmd {
	return m.loadSessions()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
		
	case sessionsLoadedMsg:
		m.loading = false
		m.sessions = msg.sessions
		m.err = msg.err
		
		// Sort by most recent
		sort.Slice(m.sessions, func(i, j int) bool {
			return m.sessions[i].LastActive.After(m.sessions[j].LastActive)
		})
		
		// Select first and load it
		if len(m.sessions) > 0 {
			m.selected = 0
			m.scrollOffset = 0 // Reset scroll
			return m, m.loadFullSession(m.sessions[0].FilePath)
		}
		return m, nil
		
	case fullSessionLoadedMsg:
		m.fullSession = msg.session
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			m.statusTimer = time.Now()
		}
		return m, nil
		
	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil
		
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
			
		case "up", "k":
			if m.selected > 0 {
				m.selected--
				m.ensureVisible()
				if m.selected < len(m.sessions) {
					return m, m.loadFullSession(m.sessions[m.selected].FilePath)
				}
			}
			
		case "down", "j":
			if m.selected < len(m.sessions)-1 {
				m.selected++
				m.ensureVisible()
				if m.selected < len(m.sessions) {
					return m, m.loadFullSession(m.sessions[m.selected].FilePath)
				}
			}
			
		case "enter":
			if m.fullSession != nil {
				cmd := m.fullSession.GetResumeCommand()
				if err := m.clipboardMgr.Copy(cmd); err != nil {
					m.statusMsg = fmt.Sprintf("Copy failed: %v", err)
				} else {
					m.statusMsg = "Copied to clipboard!"
				}
				m.statusTimer = time.Now()
				// Clear the message after 2 seconds
				return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
					return clearStatusMsg{}
				})
			}
			
		case "r":
			m.loading = true
			return m, m.loadSessions()
		}
	}
	
	return m, nil
}

func (m *Model) View() string {
	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			"Loading sessions...")
	}
	
	if m.err != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			errorStyle.Render(fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)))
	}
	
	// Calculate pane dimensions
	// Reserve 1 for status bar (top margins are now in styles)
	availableHeight := m.height - 1
	
	// Fixed width for left pane (including margin)
	leftWidth := 40
	if m.width < 80 {
		leftWidth = m.width / 2
	}
	// Right pane gets remaining width minus the left margin
	rightWidth := m.width - leftWidth - 1
	
	// Render panes with consistent height
	leftPane := m.renderSessionList(leftWidth, availableHeight)
	rightPane := m.renderDetails(rightWidth, availableHeight)
	
	// Join horizontally with no gap
	main := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	
	// Add status bar
	status := m.renderStatusBar()
	
	// Final layout
	return lipgloss.JoinVertical(
		lipgloss.Left,
		main,
		status,
	)
}

func (m *Model) renderSessionList(width, height int) string {
	// Account for border, padding, and margins (1 border + 1 padding = 2 each side, +1 top margin)
	innerHeight := height - 5
	innerWidth := width - 4
	
	// Build content
	lines := []string{}
	lines = append(lines, titleStyle.Render("Sessions"))
	lines = append(lines, "")
	
	// Calculate how many items we can show (minus title and blank line)
	itemsHeight := innerHeight - 2
	if itemsHeight < 1 {
		itemsHeight = 1
	}
	
	// Ensure scroll offset is valid
	maxScroll := len(m.sessions) - itemsHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	
	// Render visible sessions
	visibleStart := m.scrollOffset
	visibleEnd := m.scrollOffset + itemsHeight
	if visibleEnd > len(m.sessions) {
		visibleEnd = len(m.sessions)
	}
	
	for i := visibleStart; i < visibleEnd; i++ {
		session := m.sessions[i]
		
		// Format relative time
		timeStr := getRelativeTime(session.LastActive)
		
		// Truncate ID
		id := session.ID
		if len(id) > 24 {
			id = "..." + id[len(id)-21:]
		}
		
		// Format line to fit within inner width
		line := fmt.Sprintf("%-24s %s", id, timeStr)
		if len(line) > innerWidth {
			line = line[:innerWidth]
		}
		
		// Apply selection style
		if i == m.selected {
			line = selectedItemStyle.Render(line)
		} else {
			line = sessionItemStyle.Render(line)
		}
		
		lines = append(lines, line)
	}
	
	// Pad to fill the inner height
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	
	// Join lines and apply container style
	content := strings.Join(lines, "\n")
	return sessionListStyle.
		Width(width).
		Height(height).
		Render(content)
}

func (m *Model) renderDetails(width, height int) string {
	// Account for border, padding, and margins (1 border + 1 padding = 2 each side, +1 top margin)
	innerHeight := height - 5
	innerWidth := width - 4
	
	if innerHeight < 1 || innerWidth < 1 {
		return detailsStyle.Width(width).Height(height).Render("")
	}
	
	lines := []string{}
	
	if m.fullSession == nil {
		lines = append(lines, "Select a session...")
		// Pad to fill height
		for len(lines) < innerHeight {
			lines = append(lines, "")
		}
		content := strings.Join(lines, "\n")
		return detailsStyle.Width(width).Height(height).Render(content)
	}
	
	// Build content
	lines = append(lines, titleStyle.Render("Session Details"))
	lines = append(lines, "")
	
	// Basic info
	lines = append(lines, fmt.Sprintf("ID: %s", m.fullSession.ID))
	lines = append(lines, fmt.Sprintf("Messages: %d", m.fullSession.MessageCount))
	lines = append(lines, fmt.Sprintf("Cost: $%.4f", m.fullSession.TotalCostUSD))
	lines = append(lines, "")
	
	// Summary
	if m.fullSession.Summary != "" {
		lines = append(lines, "Summary:")
		wrapped := wrapText(m.fullSession.Summary, innerWidth-2)
		for _, line := range wrapped {
			lines = append(lines, "  "+line)
		}
		lines = append(lines, "")
	}
	
	// Resume command
	lines = append(lines, "Resume:")
	cmd := m.fullSession.GetResumeCommand()
	if len(cmd)+2 > innerWidth {
		cmd = cmd[:innerWidth-5] + "..."
	}
	lines = append(lines, infoStyle.Render("  "+cmd))
	lines = append(lines, "")
	
	// Check remaining space for JSON
	usedLines := len(lines)
	remainingLines := innerHeight - usedLines - 2 // -2 for JSON header
	
	if remainingLines > 3 { // Only show JSON if we have decent space
		lines = append(lines, "Last Raw Message (Complete):")
		lines = append(lines, "")
		
		if len(m.fullSession.LastRawMessages) > 0 {
			// Pretty print JSON with limited lines
			var prettyJSON bytes.Buffer
			rawMsg := m.fullSession.LastRawMessages[0]
			if err := json.Indent(&prettyJSON, []byte(rawMsg), "", "  "); err == nil {
				jsonLines := strings.Split(prettyJSON.String(), "\n")
				shown := 0
				for _, line := range jsonLines {
					if shown >= remainingLines-1 {
						lines = append(lines, mutedTextStyle.Render("  ... (more)"))
						break
					}
					if len(line) > innerWidth-2 {
						line = line[:innerWidth-5] + "..."
					}
					lines = append(lines, mutedTextStyle.Render("  "+line))
					shown++
				}
			}
		}
	}
	
	// Ensure we don't exceed inner height
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	
	// Pad to fill height
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	
	content := strings.Join(lines, "\n")
	return detailsStyle.Width(width).Height(height).Render(content)
}

func (m *Model) renderStatusBar() string {
	var content string
	
	// Show status message if present, otherwise show key hints
	if m.statusMsg != "" && time.Since(m.statusTimer) < 3*time.Second {
		content = infoStyle.Render(m.statusMsg)
	} else {
		content = keyHelpStyle.Render("[↑↓] Navigate  [Enter] Copy  [r] Refresh  [q] Quit")
	}
	
	return statusBarStyle.Width(m.width).Render(content)
}

func (m *Model) ensureVisible() {
	// Calculate actual visible items (accounting for title and padding)
	innerHeight := m.height - 1 - 5 // -1 for status, -5 for borders/padding/margins
	itemsHeight := innerHeight - 2  // -2 for title and blank line
	
	if itemsHeight < 1 {
		itemsHeight = 1
	}
	
	// Adjust scroll to keep selection visible
	if m.selected < m.scrollOffset {
		m.scrollOffset = m.selected
	} else if m.selected >= m.scrollOffset + itemsHeight {
		m.scrollOffset = m.selected - itemsHeight + 1
	}
	
	// Ensure scroll offset is valid
	maxScroll := len(m.sessions) - itemsHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m *Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.parser.ListSessions(m.claudeDir)
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (m *Model) loadFullSession(filePath string) tea.Cmd {
	return func() tea.Msg {
		session, err := m.parser.ParseFullSession(filePath)
		return fullSessionLoadedMsg{session: session, err: err}
	}
}

// Messages
type sessionsLoadedMsg struct {
	sessions []model.SessionInfo
	err      error
}

type fullSessionLoadedMsg struct {
	session *model.FullSession
	err     error
}

type clearStatusMsg struct{}

// Helper functions
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}
	
	lines := []string{}
	currentLine := ""
	
	for _, word := range words {
		if currentLine == "" {
			currentLine = word
		} else if len(currentLine+" "+word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	
	return lines
}

func getRelativeTime(t time.Time) string {
	diff := time.Since(t)
	
	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if diff < 30*24*time.Hour {
		weeks := int(diff.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if diff < 365*24*time.Hour {
		months := int(diff.Hours() / (24 * 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	} else {
		years := int(diff.Hours() / (24 * 365))
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}