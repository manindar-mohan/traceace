package models

import (
	"testing"
	"time"
)

func TestLogLine(t *testing.T) {
	now := time.Now()
	
	line := &LogLine{
		ID:        "test:1",
		Source:    "/var/log/test.log",
		Raw:       "2024-01-15 14:30:22 ERROR: Test error message",
		Timestamp: now,
		Level:     "ERROR",
		LineNum:   1,
		Offset:    0,
	}
	
	if line.ID != "test:1" {
		t.Errorf("Expected ID 'test:1', got '%s'", line.ID)
	}
	
	if line.Source != "/var/log/test.log" {
		t.Errorf("Expected source '/var/log/test.log', got '%s'", line.Source)
	}
	
	if line.Level != "ERROR" {
		t.Errorf("Expected level 'ERROR', got '%s'", line.Level)
	}
	
	if line.Timestamp != now {
		t.Errorf("Expected timestamp %v, got %v", now, line.Timestamp)
	}
}

func TestToken(t *testing.T) {
	token := Token{
		Text:      "ERROR",
		TokenType: TokenLevel,
		Start:     10,
		End:       15,
	}
	
	if token.Text != "ERROR" {
		t.Errorf("Expected text 'ERROR', got '%s'", token.Text)
	}
	
	if token.TokenType != TokenLevel {
		t.Errorf("Expected token type TokenLevel, got %v", token.TokenType)
	}
	
	if token.Start != 10 || token.End != 15 {
		t.Errorf("Expected start=10, end=15, got start=%d, end=%d", token.Start, token.End)
	}
}

func TestBookmark(t *testing.T) {
	now := time.Now()
	
	bookmark := Bookmark{
		ID:        "bookmark_1",
		Name:      "Test Bookmark",
		Source:    "/var/log/test.log",
		LineID:    "test:1",
		Timestamp: now,
		Context:   "Test context",
	}
	
	if bookmark.ID != "bookmark_1" {
		t.Errorf("Expected ID 'bookmark_1', got '%s'", bookmark.ID)
	}
	
	if bookmark.Name != "Test Bookmark" {
		t.Errorf("Expected name 'Test Bookmark', got '%s'", bookmark.Name)
	}
}

func TestSavedQuery(t *testing.T) {
	query := SavedQuery{
		Name:        "errors",
		Query:       "level:ERROR",
		Description: "Show error logs",
		IsRegex:     false,
	}
	
	if query.Name != "errors" {
		t.Errorf("Expected name 'errors', got '%s'", query.Name)
	}
	
	if query.Query != "level:ERROR" {
		t.Errorf("Expected query 'level:ERROR', got '%s'", query.Query)
	}
	
	if query.IsRegex {
		t.Error("Expected IsRegex to be false")
	}
}

func TestFilterOptions(t *testing.T) {
	timeRange := &TimeRange{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	}
	
	options := FilterOptions{
		Query:         "test query",
		IsRegex:       true,
		CaseSensitive: false,
		LogLevels:     []string{"ERROR", "WARN"},
		Sources:       []string{"/var/log/app.log"},
		TimeRange:     timeRange,
	}
	
	if options.Query != "test query" {
		t.Errorf("Expected query 'test query', got '%s'", options.Query)
	}
	
	if !options.IsRegex {
		t.Error("Expected IsRegex to be true")
	}
	
	if options.CaseSensitive {
		t.Error("Expected CaseSensitive to be false")
	}
	
	if len(options.LogLevels) != 2 {
		t.Errorf("Expected 2 log levels, got %d", len(options.LogLevels))
	}
	
	if len(options.Sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(options.Sources))
	}
	
	if options.TimeRange != timeRange {
		t.Error("Expected TimeRange to match")
	}
}

func TestTailerEvent(t *testing.T) {
	line := &LogLine{
		ID:     "test:1",
		Source: "/var/log/test.log",
		Raw:    "Test line",
	}
	
	event := TailerEvent{
		Type:    EventNewLine,
		Source:  "/var/log/test.log",
		Line:    line,
		Message: "New line received",
	}
	
	if event.Type != EventNewLine {
		t.Errorf("Expected type EventNewLine, got %v", event.Type)
	}
	
	if event.Source != "/var/log/test.log" {
		t.Errorf("Expected source '/var/log/test.log', got '%s'", event.Source)
	}
	
	if event.Line != line {
		t.Error("Expected Line to match")
	}
}

func TestUIState(t *testing.T) {
	state := UIState{
		CurrentView:   ViewSplit,
		IsPaused:      false,
		ShowSearch:    true,
		ShowHelp:      false,
		SelectedLine:  10,
		ScrollOffset:  5,
		FilteredCount: 25,
		TotalCount:    100,
	}
	
	if state.CurrentView != ViewSplit {
		t.Errorf("Expected CurrentView ViewSplit, got %v", state.CurrentView)
	}
	
	if state.IsPaused {
		t.Error("Expected IsPaused to be false")
	}
	
	if !state.ShowSearch {
		t.Error("Expected ShowSearch to be true")
	}
	
	if state.SelectedLine != 10 {
		t.Errorf("Expected SelectedLine 10, got %d", state.SelectedLine)
	}
	
	if state.FilteredCount != 25 {
		t.Errorf("Expected FilteredCount 25, got %d", state.FilteredCount)
	}
	
	if state.TotalCount != 100 {
		t.Errorf("Expected TotalCount 100, got %d", state.TotalCount)
	}
}
