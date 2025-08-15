package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/loganalyzer/traceace/pkg/config"
	"github.com/loganalyzer/traceace/pkg/filter"
	"github.com/loganalyzer/traceace/pkg/highlighter"
	"github.com/loganalyzer/traceace/pkg/models"
	"github.com/loganalyzer/traceace/pkg/parser"
	"github.com/loganalyzer/traceace/pkg/tailer"
)

// Model represents the main TUI model
type Model struct {
	// Core components
	config      *config.Config
	tailer      *tailer.Tailer
	parser      *parser.LogParser
	filter      *filter.FilterEngine
	highlighter *highlighter.Highlighter
	
	// UI State
	width           int
	height          int
	ready           bool
	quitting        bool
	
	// Panes
	allLogsPane     *LogPane
	filteredPane    *LogPane
	activePane      PaneType
	
	// Search
	searchInput     string
	searchActive    bool
	searchCursor    int
	
	// Help
	showHelp        bool
	
	// Status
	statusMessage   string
	statusTimeout   time.Time
	
	// Performance
	lastRender      time.Time
	batchedUpdates  int
	
	// Data
	allLinesBuffer    *CircularBuffer
	filteredBuffer    *CircularBuffer
	objectPool        *ObjectPool
	simpleBatcher     *SimpleBatcher
	maxBufferSize     int
	isPaused          bool
	
	// Bookmarks
	bookmarks       []models.Bookmark
	
	// Context
	ctx            context.Context
	cancel         context.CancelFunc
}

// PaneType represents the type of pane
type PaneType int

const (
	PaneAllLogs PaneType = iota
	PaneFiltered
)

// LogPane represents a log viewing pane
type LogPane struct {
	scrollY        int
	cursorY        int
	height         int
	width          int
	title          string
	showCursor     bool
	userScrolled   bool  // Track if user has manually scrolled
}

// NewModel creates a new TUI model
func NewModel(cfg *config.Config, ctx context.Context) (*Model, error) {
	ctx, cancel := context.WithCancel(ctx)
	
	// Initialize components
	parser := parser.New()
	filterEngine := filter.New(parser)
	highlighter := highlighter.New(cfg)
	tailer := tailer.New(ctx)
	
	model := &Model{
		config:         cfg,
		tailer:         tailer,
		parser:         parser,
		filter:         filterEngine,
		highlighter:    highlighter,
		ctx:            ctx,
		cancel:         cancel,
		maxBufferSize:  cfg.UI.MaxBufferLines,
		activePane:     PaneAllLogs,
		allLinesBuffer: NewCircularBuffer(cfg.UI.MaxBufferLines),
		filteredBuffer: NewCircularBuffer(cfg.UI.MaxBufferLines),
		objectPool:     NewObjectPool(),
		bookmarks:      make([]models.Bookmark, 0),
	}
	
	// Initialize panes
	model.allLogsPane = &LogPane{
		title:      "All Logs",
		showCursor: true,
	}
	model.filteredPane = &LogPane{
		title:      "Filtered Logs", 
		showCursor: false,
	}
	
	// Ensure filtered buffer starts empty
	model.filteredBuffer.Clear()
	
	// Initialize simple batcher
	model.simpleBatcher = NewSimpleBatcher()
	
	return model, nil
}

// Init implements the bubbletea.Model interface
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.listenForTailerEvents(),
	)
}

// Update implements the bubbletea.Model interface
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updatePaneSizes()
		
	case tea.KeyMsg:
		if m.searchActive {
			return m.updateSearch(msg)
		}
		
		switch key := msg.String(); key {
		case "ctrl+c", "q":
			m.quitting = true
			m.cancel()
			return m, tea.Quit
			
		case "/":
			m.searchActive = true
			m.searchInput = ""
			m.searchCursor = 0
			return m, nil
			
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
			
		case "esc":
			if m.showHelp {
				m.showHelp = false
			} else if m.searchActive {
				m.searchActive = false
			}
			return m, nil
			
		case " ":
			m.isPaused = !m.isPaused
			status := "Resumed"
			if m.isPaused {
				status = "Paused"
			}
			m.setStatusMessage(fmt.Sprintf("Stream %s", status))
			return m, nil
			
		case "t":
			if m.activePane == PaneAllLogs {
				m.activePane = PaneFiltered
				m.allLogsPane.showCursor = false
				m.filteredPane.showCursor = true
			} else {
				m.activePane = PaneAllLogs
				m.allLogsPane.showCursor = true
				m.filteredPane.showCursor = false
			}
			return m, nil
			
		case "j", "down":
			m.scrollDown()
			return m, nil
			
		case "k", "up":
			m.scrollUp()
			return m, nil
			
		case "ctrl+d":
			m.pageDown()
			return m, nil
			
		case "ctrl+u":
			m.pageUp()
			return m, nil
			
		case "g":
			m.goToTop()
			return m, nil
			
		case "G":
			m.goToBottom()
			return m, nil
			
		case "b":
			m.addBookmark()
			return m, nil
			
		case "c":
			m.clearFilter()
			return m, nil
			
		case "n":
			m.nextMatch()
			return m, nil
			
		case "N":
			m.previousMatch()
			return m, nil
		}
		
	case TailerEventMsg:
		return m.handleTailerEvent(msg.Event)
		
	case tickMsg:
		return m, m.tick()
	}
	
	return m, tea.Batch(cmds...)
}

// View implements the bubbletea.Model interface
func (m *Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	
	if m.quitting {
		return "Shutting down...\n"
	}
	
	if m.showHelp {
		return m.renderHelp()
	}
	
	// Main layout: two panes + search bar + footer
	var sections []string
	
	// Panes (split view)
	panesView := m.renderPanes()
	sections = append(sections, panesView)
	
	// Search bar
	if m.searchActive {
		searchView := m.renderSearchBar()
		sections = append(sections, searchView)
	}
	
	// Footer
	footerView := m.renderFooter()
	sections = append(sections, footerView)
	
	return strings.Join(sections, "\n")
}

// renderPanes renders the two-pane view
func (m *Model) renderPanes() string {
	if m.height < 10 {
		return "Terminal too small"
	}
	
	searchHeight := 0
	if m.searchActive {
		searchHeight = 2
	}
	
	footerHeight := 2
	availableHeight := m.height - searchHeight - footerHeight - 2 // -2 for pane borders
	
	// Split height between two panes (60/40 split)
	allLogsHeight := availableHeight * 6 / 10
	filteredHeight := availableHeight - allLogsHeight
	
	// Render all logs pane
	m.allLogsPane.height = allLogsHeight
	m.allLogsPane.width = m.width
	allLogsView := m.renderLogPane(m.allLogsPane, m.activePane == PaneAllLogs, m.allLinesBuffer)
	
	// Render filtered logs pane
	m.filteredPane.height = filteredHeight
	m.filteredPane.width = m.width
	filteredView := m.renderLogPane(m.filteredPane, m.activePane == PaneFiltered, m.filteredBuffer)
	
	return allLogsView + "\n" + filteredView
}

// renderLogPane renders a single log pane
func (m *Model) renderLogPane(pane *LogPane, isActive bool, buffer *CircularBuffer) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#666666"))
		
	if isActive {
		style = style.BorderForeground(lipgloss.Color("#00ff00"))
	}
	
	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#333333")).
		Padding(0, 1)
		
	var header string
	if pane == m.allLogsPane {
		header = fmt.Sprintf("%s (%d lines)", pane.title, buffer.Size())
	} else {
		// For filtered pane, only show count if filter is active
		if m.filter.HasFilter() {
			header = fmt.Sprintf("%s (%d/%d lines)", pane.title, buffer.Size(), m.allLinesBuffer.Size())
		} else {
			header = fmt.Sprintf("%s (no filter active)", pane.title)
		}
	}
	
	if isActive {
		header += " [ACTIVE]"
	}
	
	headerView := headerStyle.Render(header)
	
	// Add persistent header for all logs pane with file information
	var persistentHeader string
	if pane == m.allLogsPane {
		persistentHeader = m.renderPersistentHeader()
	}
	
	// Content
	contentHeight := pane.height - 3 // -3 for border and header
	if pane == m.allLogsPane && persistentHeader != "" {
		contentHeight -= 2 // -2 for persistent header
	}
	if contentHeight < 1 {
		contentHeight = 1
	}
	
	content := m.renderPaneContent(pane, contentHeight, buffer)
	
	// Combine header, persistent header, and content
	var paneContent string
	if persistentHeader != "" {
		paneContent = headerView + "\n" + persistentHeader + "\n" + content
	} else {
		paneContent = headerView + "\n" + content
	}
	
	return style.Width(pane.width-2).Height(pane.height).Render(paneContent)
}

// renderPaneContent renders the content of a pane
func (m *Model) renderPaneContent(pane *LogPane, height int, buffer *CircularBuffer) string {
	totalLines := buffer.Size()
	if totalLines == 0 {
		if pane == m.filteredPane && !m.filter.HasFilter() {
			return "No filter active. Press '/' to search/filter logs."
		}
		return "No logs"
	}
	
	// Calculate visible range
	startIdx := pane.scrollY
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= totalLines {
		startIdx = totalLines - 1
		if startIdx < 0 {
			startIdx = 0
		}
	}
	
	endIdx := startIdx + height
	if endIdx > totalLines {
		endIdx = totalLines
	}
	
	var content []string
	for i := startIdx; i < endIdx; i++ {
		line := buffer.Get(i)
		if line == nil {
			continue
		}
		
		// Lazy highlight the line (only when actually visible)
		highlighted := m.highlighter.Highlight(line)
		
		// Add cursor indicator
		if pane.showCursor && i == startIdx+pane.cursorY {
			highlighted = "> " + highlighted
		} else {
			highlighted = "  " + highlighted
		}
		
		// Truncate if too long (account for ANSI escape codes)
		maxWidth := pane.width - 4
		if maxWidth > 10 { // Ensure we have reasonable minimum width
			// Simple approach: only truncate if the raw text (without ANSI codes) is too long
			if len(line.Raw) > maxWidth {
				// Re-highlight the truncated raw text
				truncatedLine := &models.LogLine{
					ID:        line.ID,
					Source:    line.Source,
					Raw:       line.Raw[:maxWidth-3] + "...",
					LineNum:   line.LineNum,
					Timestamp: line.Timestamp,
					Level:     line.Level,
					Parsed:    line.Parsed,
					Tokens:    line.Tokens,
					Offset:    line.Offset,
				}
				highlighted = m.highlighter.Highlight(truncatedLine)
			}
		}
		
		content = append(content, highlighted)
	}
	
	// Pad with empty lines if needed
	for len(content) < height {
		content = append(content, "")
	}
	
	return strings.Join(content, "\n")
}

// renderSearchBar renders the search input bar
func (m *Model) renderSearchBar() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#ffff00")).
		Padding(0, 1)
	
	prompt := "Filter (try: errors, 4xx, level:ERROR AND status:>400): "
	input := m.searchInput
	
	// Add cursor
	if len(input) == 0 {
		input = "█"
	} else {
		if m.searchCursor < len(input) {
			input = input[:m.searchCursor] + "█" + input[m.searchCursor:]
		} else {
			input += "█"
		}
	}
	
	searchText := prompt + input
	
	return style.Width(m.width-2).Render(searchText)
}

// renderFooter renders the status footer
func (m *Model) renderFooter() string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color("#333333")).
		Foreground(lipgloss.Color("#ffffff")).
		Padding(0, 1)
	
	// Left side - status
	var leftParts []string
	
	if m.isPaused {
		leftParts = append(leftParts, "[PAUSED]")
	}
	
	if m.filter.HasFilter() {
		filterSummary := m.filter.GetFilterSummary()
		leftParts = append(leftParts, fmt.Sprintf("Filter: %s", filterSummary))
	}
	
	if m.statusMessage != "" && time.Now().Before(m.statusTimeout) {
		leftParts = append(leftParts, m.statusMessage)
	}
	
	leftSide := strings.Join(leftParts, " | ")
	
	// Right side - help and examples  
	rightSide := "⚡ Fast Filter | ? help | / search | Examples: level:ERROR AND status:>400"
	
	// Calculate spacing
	totalUsed := len(leftSide) + len(rightSide)
	spacing := m.width - totalUsed - 4 // -4 for padding
	if spacing < 0 {
		spacing = 0
	}
	
	footer := leftSide + strings.Repeat(" ", spacing) + rightSide
	
	return style.Width(m.width).Render(footer)
}

// renderHelp renders the help screen
func (m *Model) renderHelp() string {
	helpContent := `
TraceAce - Help

NAVIGATION:
  j, ↓       Scroll down
  k, ↑       Scroll up  
  Ctrl+d     Page down
  Ctrl+u     Page up
  g          Go to top
  G          Go to bottom

SEARCH & FILTER:
  /          Open search
  n          Next match
  N          Previous match
  c          Clear filter
  Esc        Close search/help

CONTROLS:
  Space      Pause/resume stream
  t          Toggle active pane
  b          Add bookmark
  q          Quit
  ?          Toggle help

ADVANCED SEARCH SYNTAX:
  Simple:
    error                    Text search
    level:ERROR              Field equals
    level:~(ERROR|WARN)      Field regex
    level:!=INFO             Field not equals
    status:>200              Numeric greater than
    time:[14:30:00 TO 15:00:00]  Time range
  
  Complex (Logical Operators):
    level:ERROR AND status:>400           Multiple conditions
    ip:192.168.1.1 OR ip:10.0.0.1        Alternative conditions  
    level:ERROR AND NOT source:test.log   Exclusion
    (level:ERROR OR level:WARN) AND status:>400  Grouping
  
  Supported Fields:
    level, source, message, timestamp, status, ip, user, method, url
    Plus any JSON/YAML field (e.g., user.id, response.time)
  
Press Esc or ? to close help.
`
	
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00ffff")).
		Padding(1, 2)
	
	return style.Width(m.width-4).Height(m.height-4).Render(helpContent)
}

// Event handling and utility methods

// listenForTailerEvents sets up listening for tailer events
func (m *Model) listenForTailerEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-m.tailer.Events():
			return TailerEventMsg{Event: event}
		case <-m.ctx.Done():
			return nil
		}
	}
}

// tick provides regular updates
func (m *Model) tick() tea.Cmd {
	return tea.Tick(time.Duration(m.config.UI.RefreshRate)*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// handleTailerEvent processes tailer events
func (m *Model) handleTailerEvent(event models.TailerEvent) (tea.Model, tea.Cmd) {
	switch event.Type {
	case models.EventNewLine:
		if event.Line != nil && !m.isPaused {
			m.addLogLine(event.Line)
		}
		
	case models.EventFileError:
		m.setStatusMessage(fmt.Sprintf("File error: %s", event.Message))
		
	case models.EventFileRotated:
		m.setStatusMessage(fmt.Sprintf("File rotated: %s", event.Source))
	}
	
	return m, tea.Batch(m.listenForTailerEvents(), m.tick())
}

// Message types
type TailerEventMsg struct {
	Event models.TailerEvent
}

type tickMsg time.Time

// renderPersistentHeader renders a persistent header with file information
func (m *Model) renderPersistentHeader() string {
	// Get list of watched files
	watchedFiles := m.tailer.GetWatchedFiles()
	if len(watchedFiles) == 0 {
		return ""
	}
	
	// Create a fixed header style that stands out
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#80c0ff")).
		Bold(true).
		Padding(0, 1).
		Width(m.width - 6) // Account for padding and borders
	
	// Build file info string
	fileInfo := strings.Join(watchedFiles, ", ")
	
	// Add more file stats if available
	var extraInfo string
	if m.allLinesBuffer.Size() > 0 {
		minTime := time.Now()
		maxTime := time.Time{}
		
		m.allLinesBuffer.ForEach(func(line *models.LogLine) bool {
			if !line.Timestamp.IsZero() {
				if line.Timestamp.Before(minTime) {
					minTime = line.Timestamp
				}
				if line.Timestamp.After(maxTime) {
					maxTime = line.Timestamp
				}
			}
			return true
		})
		
		if !maxTime.IsZero() && !minTime.Equal(time.Now()) {
			timeRange := "Time range: " + minTime.Format("2006-01-02 15:04:05") + " → " + maxTime.Format("15:04:05")
			extraInfo = "  |  " + timeRange
		}
	}
	
	// Combine all info and render
	headerText := "Files: " + fileInfo + extraInfo
	return headerStyle.Render(headerText)
}

// Utility functions continue in next part...
