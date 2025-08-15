package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/loganalyzer/traceace/pkg/models"
)

// addLogLine adds a new log line using simple batching
func (m *Model) addLogLine(line *models.LogLine) {
	// Parse the line first
	m.parser.ParseLogLine(line)
	
	// Use simple batcher to process in 1000-line chunks
	m.simpleBatcher.AddLine(line, m)
	
	// Batch updates for performance - only auto-scroll every 10 lines or 100ms
	m.batchedUpdates++
	now := time.Now()
	if m.batchedUpdates >= 10 || now.Sub(m.lastRender) > 100*time.Millisecond {
		m.autoScrollToBottom()
		m.batchedUpdates = 0
		m.lastRender = now
	}
}

// updateSearch handles search input updates
func (m *Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	
	switch key {
	case "enter":
		// Apply the search
		m.searchActive = false
		if err := m.applySearch(); err != nil {
			m.setStatusMessage(fmt.Sprintf("Search error: %s", err.Error()))
		}
		return m, nil
		
	case "esc":
		// Cancel search
		m.searchActive = false
		return m, nil
		
	case "backspace":
		// Remove character
		if m.searchCursor > 0 {
			m.searchInput = m.searchInput[:m.searchCursor-1] + m.searchInput[m.searchCursor:]
			m.searchCursor--
		}
		return m, nil
		
	case "left":
		// Move cursor left
		if m.searchCursor > 0 {
			m.searchCursor--
		}
		return m, nil
		
	case "right":
		// Move cursor right
		if m.searchCursor < len(m.searchInput) {
			m.searchCursor++
		}
		return m, nil
		
	case "home":
		// Move to beginning
		m.searchCursor = 0
		return m, nil
		
	case "end":
		// Move to end
		m.searchCursor = len(m.searchInput)
		return m, nil
		
	default:
		// Add character if printable
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			m.searchInput = m.searchInput[:m.searchCursor] + key + m.searchInput[m.searchCursor:]
			m.searchCursor++
		}
		return m, nil
	}
}

// applySearch applies the current search input as a filter
func (m *Model) applySearch() error {
	if m.searchInput == "" {
		m.filter.Clear()
		m.filteredBuffer.Clear()
		m.setStatusMessage("Filter cleared")
		return nil
	}
	
	// Check for predefined shortcuts
	actualQuery := m.expandShortcuts(m.searchInput)
	
	// Check if this is an advanced query (contains logical operators or complex syntax)
	isAdvanced := m.isAdvancedQuery(actualQuery)
	
	if isAdvanced {
		// Use advanced filtering
		if err := m.filter.SetAdvancedFilter(actualQuery); err != nil {
			return err
		}
	} else {
		// Use simple filtering for backward compatibility
		isRegex := strings.ContainsAny(actualQuery, ".*+?^${}[]|()")
		
		if err := m.filter.ValidateQuery(actualQuery, isRegex); err != nil {
			return err
		}
		
		options := models.FilterOptions{
			Query:         actualQuery,
			IsRegex:       isRegex,
			CaseSensitive: false,
		}
		
		if err := m.filter.SetFilter(options); err != nil {
			return err
		}
	}
	
	// Force flush any pending batch
	m.simpleBatcher.ForceBatch(m)
	
	// Process all existing lines in 1000-line batches
	if err := m.ProcessAllExistingLines(); err != nil {
		return err
	}
	
	return nil
}

// expandShortcuts expands common search shortcuts
func (m *Model) expandShortcuts(query string) string {
	shortcuts := map[string]string{
		"errors":     "level:ERROR",
		"warnings":   "level:WARN", 
		"info":       "level:INFO",
		"debug":      "level:DEBUG",
		"5xx":        "status:>=500",
		"4xx":        "status:>=400 AND status:<500",
		"3xx":        "status:>=300 AND status:<400",
		"2xx":        "status:>=200 AND status:<300",
		"slow":       "response_time:>1000",
		"today":      fmt.Sprintf("time:[%s TO %s]", 
			time.Now().Format("2006-01-02")+" 00:00:00",
			time.Now().Format("2006-01-02")+" 23:59:59"),
		"last_hour":  fmt.Sprintf("time:[%s TO %s]",
			time.Now().Add(-time.Hour).Format("15:04:05"),
			time.Now().Format("15:04:05")),
	}
	
	if expanded, exists := shortcuts[strings.ToLower(query)]; exists {
		return expanded
	}
	return query
}

// isAdvancedQuery determines if a query uses advanced syntax
func (m *Model) isAdvancedQuery(query string) bool {
	return strings.Contains(query, " AND ") ||
		   strings.Contains(query, " OR ") ||
		   strings.Contains(query, " NOT ") ||
		   strings.Contains(query, "time:[") ||
		   strings.Contains(query, ":>") ||
		   strings.Contains(query, ":<") ||
		   strings.Contains(query, ":!=") ||
		   strings.Contains(query, ":~") ||
		   strings.Count(query, ":") > 1 // Multiple field queries
}

// rebuildFilteredLines rebuilds the filtered lines based on current filter
func (m *Model) rebuildFilteredLines() {
	m.filteredBuffer.Clear()
	
	m.allLinesBuffer.ForEach(func(line *models.LogLine) bool {
		if m.filter.Match(line) {
			m.filteredBuffer.Add(line)
		}
		return true
	})
}

// getContentHeight returns the available height for content in a pane
func (m *Model) getContentHeight(pane *LogPane) int {
	baseHeight := pane.height - 3 // -3 for border and header
	if pane == m.allLogsPane {
		baseHeight -= 1 // -1 additional for persistent header (reduced from 2)
	}
	if baseHeight < 1 {
		baseHeight = 1
	}
	return baseHeight
}

// scrollDown scrolls the active pane down by one line
func (m *Model) scrollDown() {
	activePane := m.getActivePane()
	buffer := m.getActiveBuffer()
	if activePane == nil || buffer == nil || buffer.Size() == 0 {
		return
	}
	
	contentHeight := m.getContentHeight(activePane)
	maxScroll := buffer.Size() - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	
	if activePane.scrollY < maxScroll {
		activePane.scrollY++
		activePane.userScrolled = true
	}
}

// scrollUp scrolls the active pane up by one line
func (m *Model) scrollUp() {
	activePane := m.getActivePane()
	if activePane == nil {
		return
	}
	
	if activePane.scrollY > 0 {
		activePane.scrollY--
		activePane.userScrolled = true
	}
}

// pageDown scrolls the active pane down by a page
func (m *Model) pageDown() {
	activePane := m.getActivePane()
	buffer := m.getActiveBuffer()
	if activePane == nil || buffer == nil || buffer.Size() == 0 {
		return
	}
	
	pageSize := m.getContentHeight(activePane)
	maxScroll := buffer.Size() - pageSize
	if maxScroll < 0 {
		maxScroll = 0
	}
	
	activePane.scrollY += pageSize
	if activePane.scrollY > maxScroll {
		activePane.scrollY = maxScroll
	}
	activePane.userScrolled = true
}

// pageUp scrolls the active pane up by a page
func (m *Model) pageUp() {
	activePane := m.getActivePane()
	if activePane == nil {
		return
	}
	
	pageSize := m.getContentHeight(activePane)
	activePane.scrollY -= pageSize
	if activePane.scrollY < 0 {
		activePane.scrollY = 0
	}
	activePane.userScrolled = true
}

// goToTop scrolls to the top of the active pane
func (m *Model) goToTop() {
	activePane := m.getActivePane()
	if activePane != nil {
		activePane.scrollY = 0
		activePane.cursorY = 0
		activePane.userScrolled = true
	}
}

// goToBottom scrolls to the bottom of the active pane
func (m *Model) goToBottom() {
	activePane := m.getActivePane()
	buffer := m.getActiveBuffer()
	if activePane == nil || buffer == nil || buffer.Size() == 0 {
		return
	}
	
	pageSize := m.getContentHeight(activePane)
	maxScroll := buffer.Size() - pageSize
	if maxScroll < 0 {
		maxScroll = 0
	}
	
	activePane.scrollY = maxScroll
	activePane.cursorY = buffer.Size() - 1 - activePane.scrollY
	activePane.userScrolled = false  // Reset user scroll flag when going to bottom
}

// autoScrollToBottom automatically scrolls to bottom if already at bottom
func (m *Model) autoScrollToBottom() {
	// Auto-scroll all logs pane if at bottom and user hasn't manually scrolled
	if m.allLogsPane != nil && m.allLinesBuffer.Size() > 0 && !m.allLogsPane.userScrolled {
		pageSize := m.getContentHeight(m.allLogsPane)
		maxScroll := m.allLinesBuffer.Size() - pageSize
		if maxScroll < 0 {
			maxScroll = 0
		}
		
		// Only auto-scroll if we're exactly at the bottom (not near)
		if m.allLogsPane.scrollY >= maxScroll {
			m.allLogsPane.scrollY = maxScroll
		}
	}
	
	// Auto-scroll filtered pane if at bottom and user hasn't manually scrolled
	if m.filteredPane != nil && m.filteredBuffer.Size() > 0 && !m.filteredPane.userScrolled {
		pageSize := m.getContentHeight(m.filteredPane)
		maxScroll := m.filteredBuffer.Size() - pageSize
		if maxScroll < 0 {
			maxScroll = 0
		}
		
		// Only auto-scroll if we're exactly at the bottom (not near)
		if m.filteredPane.scrollY >= maxScroll {
			m.filteredPane.scrollY = maxScroll
		}
	}
}

// nextMatch moves to the next search match
func (m *Model) nextMatch() {
	if !m.filter.HasFilter() {
		m.setStatusMessage("No search filter active")
		return
	}
	
	activePane := m.getActivePane()
	buffer := m.getActiveBuffer()
	if activePane == nil || buffer == nil || buffer.Size() == 0 {
		return
	}
	
	// Find next match starting from current position
	currentPos := activePane.scrollY + activePane.cursorY
	
	for i := currentPos + 1; i < buffer.Size(); i++ {
		line := buffer.Get(i)
		if line != nil && m.filter.Match(line) {
			m.scrollToLine(i)
			return
		}
	}
	
	// Wrap around to beginning
	for i := 0; i <= currentPos; i++ {
		line := buffer.Get(i)
		if line != nil && m.filter.Match(line) {
			m.scrollToLine(i)
			return
		}
	}
	
	m.setStatusMessage("No more matches")
}

// previousMatch moves to the previous search match
func (m *Model) previousMatch() {
	if !m.filter.HasFilter() {
		m.setStatusMessage("No search filter active")
		return
	}
	
	activePane := m.getActivePane()
	buffer := m.getActiveBuffer()
	if activePane == nil || buffer == nil || buffer.Size() == 0 {
		return
	}
	
	// Find previous match starting from current position
	currentPos := activePane.scrollY + activePane.cursorY
	
	for i := currentPos - 1; i >= 0; i-- {
		line := buffer.Get(i)
		if line != nil && m.filter.Match(line) {
			m.scrollToLine(i)
			return
		}
	}
	
	// Wrap around to end
	for i := buffer.Size() - 1; i >= currentPos; i-- {
		line := buffer.Get(i)
		if line != nil && m.filter.Match(line) {
			m.scrollToLine(i)
			return
		}
	}
	
	m.setStatusMessage("No more matches")
}

// scrollToLine scrolls the active pane to show a specific line
func (m *Model) scrollToLine(lineIndex int) {
	activePane := m.getActivePane()
	buffer := m.getActiveBuffer()
	if activePane == nil || buffer == nil || buffer.Size() == 0 {
		return
	}
	
	if lineIndex < 0 || lineIndex >= buffer.Size() {
		return
	}
	
	pageSize := m.getContentHeight(activePane)
	
	// Center the line in the view if possible
	newScrollY := lineIndex - pageSize/2
	if newScrollY < 0 {
		newScrollY = 0
	}
	
	maxScroll := buffer.Size() - pageSize
	if maxScroll < 0 {
		maxScroll = 0
	}
	
	if newScrollY > maxScroll {
		newScrollY = maxScroll
	}
	
	activePane.scrollY = newScrollY
	activePane.cursorY = lineIndex - activePane.scrollY
	activePane.userScrolled = true
}

// addBookmark adds a bookmark at the current cursor position
func (m *Model) addBookmark() {
	activePane := m.getActivePane()
	buffer := m.getActiveBuffer()
	if activePane == nil || buffer == nil || buffer.Size() == 0 {
		m.setStatusMessage("No line to bookmark")
		return
	}
	
	currentLineIndex := activePane.scrollY + activePane.cursorY
	if currentLineIndex >= buffer.Size() {
		return
	}
	
	line := buffer.Get(currentLineIndex)
	if line == nil {
		return
	}
	
	// Create bookmark
	bookmark := models.Bookmark{
		ID:        fmt.Sprintf("bookmark_%d", time.Now().Unix()),
		Name:      fmt.Sprintf("Line %d", currentLineIndex+1),
		Source:    line.Source,
		LineID:    line.ID,
		Timestamp: time.Now(),
		Context:   line.Raw,
	}
	
	// Truncate context if too long
	if len(bookmark.Context) > 100 {
		bookmark.Context = bookmark.Context[:97] + "..."
	}
	
	m.bookmarks = append(m.bookmarks, bookmark)
	m.setStatusMessage(fmt.Sprintf("Bookmarked line %d", currentLineIndex+1))
}

// clearFilter clears the current filter
func (m *Model) clearFilter() {
	m.filter.Clear()
	m.filteredBuffer.Clear() // Explicitly clear the filtered buffer
	
	// Force flush any remaining batch
	m.simpleBatcher.ForceBatch(m)
	
	m.setStatusMessage("Filter cleared")
}

// setStatusMessage sets a temporary status message
func (m *Model) setStatusMessage(message string) {
	m.statusMessage = message
	m.statusTimeout = time.Now().Add(3 * time.Second)
}

// updatePaneSizes updates the sizes of the panes based on window size
func (m *Model) updatePaneSizes() {
	if m.allLogsPane != nil {
		m.allLogsPane.width = m.width
	}
	if m.filteredPane != nil {
		m.filteredPane.width = m.width
	}
}

// getActivePane returns the currently active pane
func (m *Model) getActivePane() *LogPane {
	switch m.activePane {
	case PaneAllLogs:
		return m.allLogsPane
	case PaneFiltered:
		return m.filteredPane
	default:
		return m.allLogsPane
	}
}

// getActiveBuffer returns the currently active buffer
func (m *Model) getActiveBuffer() *CircularBuffer {
	switch m.activePane {
	case PaneAllLogs:
		return m.allLinesBuffer
	case PaneFiltered:
		return m.filteredBuffer
	default:
		return m.allLinesBuffer
	}
}

// AddFile adds a file to be tailed
func (m *Model) AddFile(filePath string) error {
	return m.tailer.AddFile(filePath)
}

// TailFromStart starts tailing a file from the beginning
func (m *Model) TailFromStart(filePath string) error {
	return m.tailer.TailFromStart(filePath)
}

// GetBookmarks returns the current bookmarks
func (m *Model) GetBookmarks() []models.Bookmark {
	return m.bookmarks
}

// Stop stops the model and all its components
func (m *Model) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.tailer != nil {
		m.tailer.Stop()
	}
}

// GetStats returns statistics about the current state
func (m *Model) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_lines":     m.allLinesBuffer.Size(),
		"filtered_lines":  m.filteredBuffer.Size(),
		"is_paused":       m.isPaused,
		"active_pane":     m.activePane,
		"has_filter":      m.filter.HasFilter(),
		"bookmark_count":  len(m.bookmarks),
		"watched_files":   m.tailer.GetWatchedFiles(),
	}
}

// SetTheme changes the UI theme
func (m *Model) SetTheme(themeName string) {
	if m.highlighter != nil {
		m.highlighter.SetTheme(themeName)
	}
	m.config.UI.Theme = themeName
}

// GetAvailableThemes returns available themes
func (m *Model) GetAvailableThemes() []string {
	if m.highlighter != nil {
		return m.highlighter.GetAvailableThemes()
	}
	return []string{"dark", "light", "monochrome"}
}
