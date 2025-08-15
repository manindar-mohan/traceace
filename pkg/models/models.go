package models

import (
	"time"
)

// LogLine represents a single log entry with all associated metadata
type LogLine struct {
	ID        string                 `json:"id"`        // unique id (file:offset or UUID)
	Source    string                 `json:"source"`    // filename or adapter id
	Raw       string                 `json:"raw"`       // original raw text
	Timestamp time.Time              `json:"timestamp"` // if detected
	Parsed    map[string]interface{} `json:"parsed"`    // parsed JSON/YAML if present
	Level     string                 `json:"level"`     // normalized log level (INFO/WARN/ERROR/DEBUG)
	Tokens    []Token                `json:"tokens"`    // tokens for syntax highlighting
	Offset    int64                  `json:"offset"`    // byte offset in file when available
	LineNum   int                    `json:"line_num"`  // line number in file
}

// Token represents a highlighted token in a log line
type Token struct {
	Text      string    `json:"text"`
	TokenType TokenType `json:"type"`
	Start     int       `json:"start"`
	End       int       `json:"end"`
}

// TokenType defines the different types of tokens for syntax highlighting
type TokenType string

const (
	TokenDefault    TokenType = "default"
	TokenTimestamp  TokenType = "timestamp"
	TokenLevel      TokenType = "level"
	TokenIP         TokenType = "ip"
	TokenStatusCode TokenType = "status_code"
	TokenUUID       TokenType = "uuid"
	TokenURL        TokenType = "url"
	TokenError      TokenType = "error"
	TokenNumber     TokenType = "number"
	TokenString     TokenType = "string"
	TokenJSON       TokenType = "json"
	TokenKeyword    TokenType = "keyword"
)

// LogLevel represents normalized log levels
type LogLevel string

const (
	LevelDebug LogLevel = "DEBUG"
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
	LevelFatal LogLevel = "FATAL"
	LevelTrace LogLevel = "TRACE"
)

// Bookmark represents a saved position in the log
type Bookmark struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"`
	LineID    string    `json:"line_id"`
	Timestamp time.Time `json:"timestamp"`
	Context   string    `json:"context"` // excerpt of the line for display
}

// SavedQuery represents a saved search query
type SavedQuery struct {
	Name        string `json:"name"`
	Query       string `json:"query"`
	Description string `json:"description"`
	IsRegex     bool   `json:"is_regex"`
}

// FilterOptions represents search and filter configuration
type FilterOptions struct {
	Query          string   `json:"query"`
	IsRegex        bool     `json:"is_regex"`
	CaseSensitive  bool     `json:"case_sensitive"`
	LogLevels      []string `json:"log_levels"`
	Sources        []string `json:"sources"`
	TimeRange      *TimeRange `json:"time_range,omitempty"`
}

// TimeRange represents a time filter range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// UIState represents the current state of the user interface
type UIState struct {
	CurrentView     ViewMode    `json:"current_view"`
	IsPaused        bool        `json:"is_paused"`
	ShowSearch      bool        `json:"show_search"`
	ShowHelp        bool        `json:"show_help"`
	SelectedLine    int         `json:"selected_line"`
	ScrollOffset    int         `json:"scroll_offset"`
	FilteredCount   int         `json:"filtered_count"`
	TotalCount      int         `json:"total_count"`
	ActiveBookmarks []Bookmark  `json:"active_bookmarks"`
	CurrentFilter   FilterOptions `json:"current_filter"`
}

// ViewMode represents different view modes for the UI
type ViewMode string

const (
	ViewAll      ViewMode = "all"      // Show all logs
	ViewFiltered ViewMode = "filtered" // Show only filtered logs
	ViewSplit    ViewMode = "split"    // Show both all and filtered in split panes
)

// SessionState represents the current session state for persistence
type SessionState struct {
	Sources       []string      `json:"sources"`
	Bookmarks     []Bookmark    `json:"bookmarks"`
	SavedQueries  []SavedQuery  `json:"saved_queries"`
	LastFilter    FilterOptions `json:"last_filter"`
	UIState       UIState       `json:"ui_state"`
	LastAccessed  time.Time     `json:"last_accessed"`
}

// TailerEvent represents events from file tailers
type TailerEvent struct {
	Type    TailerEventType `json:"type"`
	Source  string          `json:"source"`
	Line    *LogLine        `json:"line,omitempty"`
	Error   error           `json:"error,omitempty"`
	Message string          `json:"message,omitempty"`
}

// TailerEventType represents different types of tailer events
type TailerEventType string

const (
	EventNewLine     TailerEventType = "new_line"
	EventFileRotated TailerEventType = "file_rotated"
	EventFileError   TailerEventType = "file_error"
	EventEOF         TailerEventType = "eof"
)
