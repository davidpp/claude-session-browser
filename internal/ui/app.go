package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidpaquet/claude-session-browser/internal/clipboard"
	"github.com/davidpaquet/claude-session-browser/internal/model"
	"github.com/davidpaquet/claude-session-browser/internal/parser"
	"github.com/davidpaquet/claude-session-browser/internal/search"
)

// SearchState represents the current search mode
type SearchState int

const (
	SearchStateNormal SearchState = iota  // No search active
	SearchStateInput                      // User is typing in search box
	SearchStateResults                    // User is navigating filtered results
)

// Model is the app model
type Model struct {
	// Data
	sessions      []model.SessionInfo
	fullSession   *model.FullSession
	parser        *parser.Parser
	clipboardMgr  *clipboard.Manager
	claudeDir     string
	version       string

	// UI State
	width         int
	height        int
	selected      int
	scrollOffset  int
	loading       bool
	err           error

	// Search State
	searchEngine     search.Engine
	searchState      SearchState
	searchInput      textinput.Model
	searchQuery      string
	searchResults    []search.SearchResult
	filteredSessions []model.SessionInfo

	// Status
	statusMsg     string
	statusTimer   time.Time
}

// NewApp creates a new app
func NewApp(claudeDir, version string) *Model {
	// Initialize search input
	searchInput := textinput.New()
	searchInput.Placeholder = "Search sessions..."
	searchInput.CharLimit = 100
	searchInput.Width = 30

	return &Model{
		parser:       parser.NewParser(),
		clipboardMgr: clipboard.NewManager(),
		claudeDir:    claudeDir,
		version:      version,
		loading:      true,
		width:        80,
		height:       24,
		searchInput:  searchInput,
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
		
		// Initialize search engine with sessions
		if len(m.sessions) > 0 {
			m.searchEngine = search.NewEngine(m.sessions)
			m.filteredSessions = m.sessions // Initially show all sessions
		}
		
		// Select first and load it
		if len(m.filteredSessions) > 0 {
			m.selected = 0
			m.scrollOffset = 0 // Reset scroll
			return m, m.loadFullSession(m.filteredSessions[0].FilePath)
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
		
	case searchCompleteMsg:
		// Ignore if search query has changed
		if msg.query != m.searchQuery {
			return m, nil
		}
		
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Search error: %v", msg.err)
			m.statusTimer = time.Now()
			return m, nil
		}
		
		// Store search results
		m.searchResults = msg.results
		
		// Update filtered sessions
		m.filteredSessions = make([]model.SessionInfo, 0, len(msg.results))
		for _, result := range msg.results {
			if result.SessionIndex < len(m.sessions) {
				m.filteredSessions = append(m.filteredSessions, m.sessions[result.SessionIndex])
			}
		}
		
		// Update status
		if len(m.filteredSessions) == 0 {
			m.statusMsg = fmt.Sprintf("No matches found for '%s'", m.searchQuery)
		} else {
			m.statusMsg = fmt.Sprintf("Found %d sessions matching '%s'", len(m.filteredSessions), m.searchQuery)
		}
		m.statusTimer = time.Now()
		
		// Reset selection and load first session if available
		if len(m.filteredSessions) > 0 {
			m.selected = 0
			m.scrollOffset = 0
			return m, m.loadFullSession(m.filteredSessions[0].FilePath)
		}
		
		return m, nil
		
	case tea.KeyMsg:
		// Handle based on current search state
		switch m.searchState {
		case SearchStateInput:
			// In search input mode
			switch msg.String() {
			case "esc":
				// Cancel search entirely
				m.clearSearch()
				return m, nil
			case "tab", "enter":
				// Exit input mode, enter results mode
				if m.searchQuery != "" {
					m.searchState = SearchStateResults
					m.searchInput.Blur()
				}
				return m, nil
			default:
				// Update search input
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.searchQuery = m.searchInput.Value()
				
				// Trigger async search
				if m.searchQuery != "" {
					m.statusMsg = "Searching..."
					m.statusTimer = time.Now()
					return m, tea.Batch(cmd, m.performSearchCmd())
				} else {
					// Clear search immediately if query is empty
					m.filteredSessions = m.sessions
					m.searchResults = nil
					m.statusMsg = ""
				}
				return m, cmd
			}
			
		case SearchStateResults:
			// In search results mode - handle navigation
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "esc":
				// Clear search and return to normal
				m.clearSearch()
				return m, nil
			case "/":
				// Return to search input mode
				m.searchState = SearchStateInput
				m.searchInput.Focus()
				return m, textinput.Blink
			case "up", "k":
				if m.selected > 0 {
					m.selected--
					m.ensureVisible()
					if m.selected < len(m.filteredSessions) {
						return m, m.loadFullSession(m.filteredSessions[m.selected].FilePath)
					}
				}
			case "down", "j":
				if m.selected < len(m.filteredSessions)-1 {
					m.selected++
					m.ensureVisible()
					if m.selected < len(m.filteredSessions) {
						return m, m.loadFullSession(m.filteredSessions[m.selected].FilePath)
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
					return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
						return clearStatusMsg{}
					})
				}
			case "r":
				m.loading = true
				m.clearSearch()
				return m, m.loadSessions()
			}
			
		default:
			// Normal mode - no search active
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
				
			case "/":
				m.enterSearchMode()
				return m, textinput.Blink
				
			case "up", "k":
				if m.selected > 0 {
					m.selected--
					m.ensureVisible()
					if m.selected < len(m.filteredSessions) {
						return m, m.loadFullSession(m.filteredSessions[m.selected].FilePath)
					}
				}
				
			case "down", "j":
				if m.selected < len(m.filteredSessions)-1 {
					m.selected++
					m.ensureVisible()
					if m.selected < len(m.filteredSessions) {
						return m, m.loadFullSession(m.filteredSessions[m.selected].FilePath)
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
				m.clearSearch()
				return m, m.loadSessions()
			}
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
	// Reserve space for status bar and search bar if active
	reservedHeight := 1 // status bar
	if m.searchState != SearchStateNormal {
		reservedHeight += 3 // search bar with border
	}
	availableHeight := m.height - reservedHeight
	
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
	
	// Add search bar if in search mode
	components := []string{main}
	if m.searchState != SearchStateNormal {
		searchBar := m.renderSearchBar()
		components = append(components, searchBar)
	}
	
	// Add status bar
	status := m.renderStatusBar()
	components = append(components, status)
	
	// Final layout
	return lipgloss.JoinVertical(
		lipgloss.Left,
		components...,
	)
}

func (m *Model) renderSessionList(width, height int) string {
	// Account for border, padding, and margins (1 border + 1 padding = 2 each side, +1 top margin)
	innerHeight := height - 5
	innerWidth := width - 4
	
	// Build content
	lines := []string{}
	title := "Sessions"
	if m.searchState != SearchStateNormal {
		title = fmt.Sprintf("Sessions (%d matches)", len(m.filteredSessions))
	}
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, "")
	
	// Calculate how many items we can show (minus title and blank line)
	itemsHeight := innerHeight - 2
	if itemsHeight < 1 {
		itemsHeight = 1
	}
	
	// Ensure scroll offset is valid
	maxScroll := len(m.filteredSessions) - itemsHeight
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
	if visibleEnd > len(m.filteredSessions) {
		visibleEnd = len(m.filteredSessions)
	}
	
	for i := visibleStart; i < visibleEnd; i++ {
		session := m.filteredSessions[i]
		
		// Format relative time
		timeStr := getRelativeTime(session.LastActive)
		
		// Truncate ID
		id := session.ID
		if len(id) > 24 {
			id = "..." + id[len(id)-21:]
		}
		
		// Add match indicator if searching
		matchIndicator := ""
		if m.searchQuery != "" {
			// Find match count for this session
			for _, result := range m.searchResults {
				if result.SessionID == session.ID {
					matchIndicator = fmt.Sprintf(" [%d]", len(result.Matches))
					break
				}
			}
		}
		
		// Format line to fit within inner width
		line := fmt.Sprintf("%-24s%s %s", id, matchIndicator, timeStr)
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
	
	// Show search matches if searching
	if m.searchQuery != "" {
		// Find matches for current session
		var currentMatches []search.Match
		for _, result := range m.searchResults {
			if result.SessionID == m.fullSession.ID {
				currentMatches = result.Matches
				break
			}
		}
		
		if len(currentMatches) > 0 {
			lines = append(lines, fmt.Sprintf("Search Matches (%d):", len(currentMatches)))
			lines = append(lines, strings.Repeat("─", innerWidth-2))
			
			// Show up to 5 matches
			shown := 0
			for _, match := range currentMatches {
				if shown >= 5 {
					lines = append(lines, fmt.Sprintf("  ... and %d more matches", len(currentMatches)-shown))
					break
				}
				
				// Use context if available, otherwise fall back to text
				displayText := match.Context
				if displayText == "" {
					displayText = strings.TrimSpace(match.Text)
				}
				
				// Ensure it fits within width
				if len(displayText) > innerWidth-4 {
					displayText = displayText[:innerWidth-7] + "..."
				}
				
				lines = append(lines, fmt.Sprintf("  %s", displayText))
				shown++
			}
			lines = append(lines, "")
		}
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
	var leftText string

	// Show status message if present, otherwise show key hints
	statusDuration := 3 * time.Second
	// Show ripgrep warning for longer
	if strings.Contains(m.statusMsg, "ripgrep") {
		statusDuration = 10 * time.Second
	}
	if m.statusMsg != "" && time.Since(m.statusTimer) < statusDuration {
		leftText = m.statusMsg
	} else if m.searchState == SearchStateInput {
		leftText = "[Tab/Enter] Navigate results  [Esc] Cancel  Type to search..."
	} else if m.searchState == SearchStateResults {
		leftText = "[↑↓] Navigate  [/] Edit search  [Esc] Clear search  [Enter] Copy"
	} else {
		leftText = "[↑↓] Navigate  [Enter] Copy  [/] Search  [r] Refresh  [q] Quit"
	}

	// Create left and right content sections
	leftStyle := keyHelpStyle.Width(m.width - lipgloss.Width(m.version) - 2)
	rightStyle := keyHelpStyle.Align(lipgloss.Right)

	leftContent := leftStyle.Render(leftText)
	rightContent := rightStyle.Render(m.version)

	// Join horizontally with bottom alignment
	content := lipgloss.JoinHorizontal(lipgloss.Bottom, leftContent, rightContent)

	return statusBarStyle.Width(m.width).Render(content)
}

func (m *Model) renderSearchBar() string {
	// Different styles for focused vs unfocused
	var borderColor lipgloss.Color
	var statusText string
	
	if m.searchState == SearchStateInput {
		// Focused - bright purple border
		borderColor = lipgloss.Color("#9B59B6")
		statusText = ""
	} else {
		// Unfocused - dimmed border
		borderColor = lipgloss.Color("#4B5563")
		if m.searchQuery != "" && len(m.filteredSessions) == 0 {
			statusText = " (no matches)"
		} else if len(m.filteredSessions) > 0 {
			statusText = fmt.Sprintf(" (%d matches)", len(m.filteredSessions))
		}
	}
	
	searchStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(m.width - 2)
	
	searchIcon := "🔍 "
	var prompt string
	
	if m.searchState == SearchStateInput {
		// Show cursor when focused
		prompt = searchIcon + "Search: " + m.searchInput.View()
	} else {
		// Show static text when unfocused
		prompt = searchIcon + "Search: " + m.searchQuery + statusText
		if m.searchState == SearchStateResults {
			prompt += " [Press / to edit]"
		}
	}
	
	return searchStyle.Render(prompt)
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
	maxScroll := len(m.filteredSessions) - itemsHeight
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

type searchCompleteMsg struct {
	results []search.SearchResult
	query   string
	err     error
}

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

// Search helper methods
func (m *Model) enterSearchMode() {
	// Check if ripgrep is available
	if !m.checkRipgrep() {
		m.statusMsg = "Warning: ripgrep (rg) not found. Install it for search to work."
		m.statusTimer = time.Now()
		// Still enter search mode but user is warned
	}
	
	m.searchState = SearchStateInput
	m.searchInput.Focus()
	m.searchInput.SetValue(m.searchQuery) // Keep existing query if any
}

func (m *Model) checkRipgrep() bool {
	_, err := exec.LookPath("rg")
	return err == nil
}

func (m *Model) clearSearch() {
	m.searchState = SearchStateNormal
	m.searchInput.Blur()
	m.searchInput.SetValue("")
	m.searchQuery = ""
	m.searchResults = nil
	// Reset to show all sessions
	m.filteredSessions = m.sessions
	m.selected = 0
	m.scrollOffset = 0
}

func (m *Model) performSearchCmd() tea.Cmd {
	return func() tea.Msg {
		if m.searchEngine == nil || m.searchQuery == "" {
			return searchCompleteMsg{
				results: []search.SearchResult{},
				query:   m.searchQuery,
				err:     nil,
			}
		}
		
		// Perform FULL TEXT SEARCH across all session content
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		results, err := m.searchEngine.Search(ctx, m.searchQuery, search.SearchTypeContent)
		
		return searchCompleteMsg{
			results: results,
			query:   m.searchQuery,
			err:     err,
		}
	}
}