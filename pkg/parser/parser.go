package parser

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/loganalyzer/traceace/pkg/models"
	"gopkg.in/yaml.v3"
)

// LogParser handles parsing of log lines
type LogParser struct {
	timestampPatterns []*regexp.Regexp
	levelPatterns     []*regexp.Regexp
	levelMapping      map[string]models.LogLevel
}

// New creates a new LogParser
func New() *LogParser {
	return &LogParser{
		timestampPatterns: compileTimestampPatterns(),
		levelPatterns:     compileLevelPatterns(),
		levelMapping:      createLevelMapping(),
	}
}

// ParseLogLine parses a raw log line and extracts structured information
func (p *LogParser) ParseLogLine(line *models.LogLine) {
	if line == nil || line.Raw == "" {
		return
	}

	// Try to parse as JSON
	if p.tryParseJSON(line) {
		return
	}

	// Try to parse as YAML
	if p.tryParseYAML(line) {
		return
	}

	// Parse as unstructured text
	p.parseUnstructured(line)
}

// tryParseJSON attempts to parse the line as JSON
func (p *LogParser) tryParseJSON(line *models.LogLine) bool {
	trimmed := strings.TrimSpace(line.Raw)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return false
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return false
	}

	line.Parsed = parsed

	// Extract common fields
	if timestamp, ok := p.extractTimestampFromParsed(parsed); ok {
		line.Timestamp = timestamp
	}

	if level, ok := p.extractLevelFromParsed(parsed); ok {
		line.Level = string(level)
	}

	return true
}

// tryParseYAML attempts to parse the line as YAML
func (p *LogParser) tryParseYAML(line *models.LogLine) bool {
	// YAML is more complex to detect, look for key-value patterns
	trimmed := strings.TrimSpace(line.Raw)
	
	// Be more strict about YAML detection - must have key: value pattern
	// and not look like a simple log line
	if !strings.Contains(trimmed, ":") {
		return false
	}
	
	// Skip if it looks like a timestamp-based log line
	if strings.Contains(trimmed, " INFO:") || strings.Contains(trimmed, " DEBUG:") ||
	   strings.Contains(trimmed, " WARN:") || strings.Contains(trimmed, " ERROR:") ||
	   strings.Contains(trimmed, " FATAL:") {
		return false
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return false
	}
	
	// Must have at least 2 key-value pairs to be considered structured YAML
	if len(parsed) < 2 {
		return false
	}

	line.Parsed = parsed

	// Extract common fields
	if timestamp, ok := p.extractTimestampFromParsed(parsed); ok {
		line.Timestamp = timestamp
	}

	if level, ok := p.extractLevelFromParsed(parsed); ok {
		line.Level = string(level)
	}

	return true
}

// parseUnstructured parses unstructured text and extracts common patterns
func (p *LogParser) parseUnstructured(line *models.LogLine) {
	// Extract timestamp
	if line.Timestamp.IsZero() {
		if timestamp := p.extractTimestamp(line.Raw); !timestamp.IsZero() {
			line.Timestamp = timestamp
		}
	}

	// Extract log level
	if line.Level == "" {
		if level := p.extractLevel(line.Raw); level != "" {
			line.Level = string(level)
		}
	}
}

// extractTimestampFromParsed extracts timestamp from parsed structured data
func (p *LogParser) extractTimestampFromParsed(parsed map[string]interface{}) (time.Time, bool) {
	// Common timestamp field names
	timestampFields := []string{
		"timestamp", "time", "ts", "@timestamp", "datetime", "created_at", "logged_at",
	}

	for _, field := range timestampFields {
		if val, exists := parsed[field]; exists {
			if timestamp, ok := p.parseTimestampValue(val); ok {
				return timestamp, true
			}
		}
	}

	return time.Time{}, false
}

// extractLevelFromParsed extracts log level from parsed structured data
func (p *LogParser) extractLevelFromParsed(parsed map[string]interface{}) (models.LogLevel, bool) {
	// Common level field names
	levelFields := []string{"level", "severity", "priority", "loglevel", "log_level"}

	for _, field := range levelFields {
		if val, exists := parsed[field]; exists {
			if str, ok := val.(string); ok {
				if level, exists := p.levelMapping[strings.ToUpper(str)]; exists {
					return level, true
				}
			}
		}
	}

	return "", false
}

// extractTimestamp extracts timestamp from raw text using regex patterns
func (p *LogParser) extractTimestamp(text string) time.Time {
	for _, pattern := range p.timestampPatterns {
		if matches := pattern.FindStringSubmatch(text); len(matches) > 0 {
			if timestamp, err := p.parseTimestampString(matches[0]); err == nil {
				return timestamp
			}
		}
	}
	return time.Time{}
}

// extractLevel extracts log level from raw text using regex patterns
func (p *LogParser) extractLevel(text string) models.LogLevel {
	for _, pattern := range p.levelPatterns {
		if matches := pattern.FindStringSubmatch(text); len(matches) > 0 {
			levelStr := strings.ToUpper(matches[0])
			if level, exists := p.levelMapping[levelStr]; exists {
				return level
			}
		}
	}
	return ""
}

// parseTimestampValue parses various timestamp value types
func (p *LogParser) parseTimestampValue(val interface{}) (time.Time, bool) {
	switch v := val.(type) {
	case string:
		if timestamp, err := p.parseTimestampString(v); err == nil {
			return timestamp, true
		}
	case int64:
		return time.Unix(v, 0), true
	case float64:
		return time.Unix(int64(v), 0), true
	case time.Time:
		return v, true
	}
	return time.Time{}, false
}

// parseTimestampString parses timestamp from string using various formats
func (p *LogParser) parseTimestampString(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000",
		"Jan 02 15:04:05",
		"Jan  2 15:04:05",
		"2006/01/02 15:04:05",
		"02/Jan/2006:15:04:05 -0700", // Apache log format
		"Mon Jan _2 15:04:05 2006",    // Unix date format
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, nil
}

// compileTimestampPatterns compiles regex patterns for timestamp detection
func compileTimestampPatterns() []*regexp.Regexp {
	patterns := []string{
		// ISO 8601
		`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`,
		// Common log formats
		`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`,
		`\d{2}/\w{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}`, // Apache
		`\w{3} \d{1,2} \d{2}:\d{2}:\d{2}`,                // Syslog
		`\w{3} \s?\d{1,2} \d{2}:\d{2}:\d{2} \d{4}`,       // Unix date
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// compileLevelPatterns compiles regex patterns for log level detection
func compileLevelPatterns() []*regexp.Regexp {
	patterns := []string{
		`\b(TRACE|DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PANIC)\b`,
		`\[(TRACE|DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PANIC)\]`,
		`"(level|severity)"\s*:\s*"(TRACE|DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PANIC)"`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if re, err := regexp.Compile(`(?i)` + pattern); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// createLevelMapping creates mapping from strings to log levels
func createLevelMapping() map[string]models.LogLevel {
	return map[string]models.LogLevel{
		"TRACE":   models.LevelTrace,
		"DEBUG":   models.LevelDebug,
		"INFO":    models.LevelInfo,
		"WARN":    models.LevelWarn,
		"WARNING": models.LevelWarn,
		"ERROR":   models.LevelError,
		"FATAL":   models.LevelFatal,
		"PANIC":   models.LevelFatal,
	}
}

// GetParsedField retrieves a field from parsed structured data
func (p *LogParser) GetParsedField(line *models.LogLine, fieldPath string) interface{} {
	if line.Parsed == nil {
		return nil
	}

	// Split field path by dots for nested access
	parts := strings.Split(fieldPath, ".")
	current := line.Parsed

	for i, part := range parts {
		if val, exists := current[part]; exists {
			if i == len(parts)-1 {
				// Last part, return the value
				return val
			}
			// Continue traversing
			if nested, ok := val.(map[string]interface{}); ok {
				current = nested
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	return nil
}

// IsStructured returns whether the log line contains structured data
func (p *LogParser) IsStructured(line *models.LogLine) bool {
	return line.Parsed != nil && len(line.Parsed) > 0
}
