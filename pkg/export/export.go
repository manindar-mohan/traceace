package export

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loganalyzer/traceace/pkg/models"
)

// Exporter handles exporting log data to various formats
type Exporter struct {
	// configuration options
}

// ExportFormat represents different export formats
type ExportFormat string

const (
	FormatText ExportFormat = "text"
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
	FormatHTML ExportFormat = "html"
)

// ExportOptions contains configuration for export operations
type ExportOptions struct {
	Format       ExportFormat      `json:"format"`
	OutputPath   string           `json:"output_path"`
	IncludeRaw   bool             `json:"include_raw"`
	IncludeParsed bool            `json:"include_parsed"`
	IncludeTokens bool            `json:"include_tokens"`
	TimeRange    *models.TimeRange `json:"time_range,omitempty"`
	Metadata     map[string]string `json:"metadata"`
}

// New creates a new Exporter
func New() *Exporter {
	return &Exporter{}
}

// ExportLines exports log lines to a file
func (e *Exporter) ExportLines(lines []*models.LogLine, options ExportOptions) error {
	if len(lines) == 0 {
		return fmt.Errorf("no lines to export")
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(options.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Open output file
	file, err := os.Create(options.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Filter lines by time range if specified
	filteredLines := lines
	if options.TimeRange != nil {
		filteredLines = e.filterByTimeRange(lines, options.TimeRange)
	}

	// Export based on format
	switch options.Format {
	case FormatText:
		return e.exportText(file, filteredLines, options)
	case FormatJSON:
		return e.exportJSON(file, filteredLines, options)
	case FormatCSV:
		return e.exportCSV(file, filteredLines, options)
	case FormatHTML:
		return e.exportHTML(file, filteredLines, options)
	default:
		return fmt.Errorf("unsupported export format: %s", options.Format)
	}
}

// ExportSession exports the entire session state
func (e *Exporter) ExportSession(session models.SessionState, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	
	if err := encoder.Encode(session); err != nil {
		return fmt.Errorf("failed to encode session: %w", err)
	}

	return nil
}

// ImportSession imports a session state from a file
func (e *Exporter) ImportSession(inputPath string) (*models.SessionState, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	var session models.SessionState
	decoder := json.NewDecoder(file)
	
	if err := decoder.Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	return &session, nil
}

// filterByTimeRange filters lines by the specified time range
func (e *Exporter) filterByTimeRange(lines []*models.LogLine, timeRange *models.TimeRange) []*models.LogLine {
	var filtered []*models.LogLine
	
	for _, line := range lines {
		if line.Timestamp.IsZero() {
			continue // Skip lines without timestamps
		}
		
		if line.Timestamp.After(timeRange.Start) && line.Timestamp.Before(timeRange.End) {
			filtered = append(filtered, line)
		}
	}
	
	return filtered
}

// exportText exports lines as plain text
func (e *Exporter) exportText(writer io.Writer, lines []*models.LogLine, options ExportOptions) error {
	w := bufio.NewWriter(writer)
	defer w.Flush()

	// Write header with metadata
	if len(options.Metadata) > 0 {
		w.WriteString("# Export Metadata\n")
		for key, value := range options.Metadata {
			w.WriteString(fmt.Sprintf("# %s: %s\n", key, value))
		}
		w.WriteString(fmt.Sprintf("# Exported at: %s\n", time.Now().Format(time.RFC3339)))
		w.WriteString(fmt.Sprintf("# Total lines: %d\n\n", len(lines)))
	}

	// Write lines
	for _, line := range lines {
		if options.IncludeRaw {
			w.WriteString(line.Raw)
		} else {
			// Format the line with timestamp and source if available
			formatted := e.formatLineForText(line)
			w.WriteString(formatted)
		}
		w.WriteString("\n")
	}

	return nil
}

// exportJSON exports lines as JSON
func (e *Exporter) exportJSON(writer io.Writer, lines []*models.LogLine, options ExportOptions) error {
	// Create export structure
	export := map[string]interface{}{
		"metadata": map[string]interface{}{
			"exported_at":  time.Now().Format(time.RFC3339),
			"total_lines":  len(lines),
			"format":       "json",
			"options":      options,
		},
		"lines": lines,
	}

	// Add custom metadata
	if len(options.Metadata) > 0 {
		for key, value := range options.Metadata {
			export["metadata"].(map[string]interface{})[key] = value
		}
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	
	return encoder.Encode(export)
}

// exportCSV exports lines as CSV
func (e *Exporter) exportCSV(writer io.Writer, lines []*models.LogLine, options ExportOptions) error {
	w := bufio.NewWriter(writer)
	defer w.Flush()

	// Write header
	header := []string{"timestamp", "source", "level", "raw"}
	if options.IncludeParsed {
		header = append(header, "parsed_json")
	}
	w.WriteString(strings.Join(header, ",") + "\n")

	// Write lines
	for _, line := range lines {
		row := []string{
			e.escapeCSV(line.Timestamp.Format(time.RFC3339)),
			e.escapeCSV(line.Source),
			e.escapeCSV(line.Level),
			e.escapeCSV(line.Raw),
		}

		if options.IncludeParsed && line.Parsed != nil {
			parsedJSON, _ := json.Marshal(line.Parsed)
			row = append(row, e.escapeCSV(string(parsedJSON)))
		}

		w.WriteString(strings.Join(row, ",") + "\n")
	}

	return nil
}

// exportHTML exports lines as HTML
func (e *Exporter) exportHTML(writer io.Writer, lines []*models.LogLine, options ExportOptions) error {
	w := bufio.NewWriter(writer)
	defer w.Flush()

	// Write HTML header
	w.WriteString(`<!DOCTYPE html>
<html>
<head>
    <title>Log Export</title>
    <meta charset="utf-8">
    <style>
        body { font-family: monospace; background-color: #1e1e1e; color: #d4d4d4; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        .metadata { background-color: #2d2d30; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
        .log-line { padding: 5px; border-bottom: 1px solid #404040; }
        .log-line:hover { background-color: #2d2d30; }
        .timestamp { color: #4fc1ff; }
        .source { color: #ce9178; }
        .level-info { color: #4ec9b0; }
        .level-warn { color: #dcdcaa; }
        .level-error { color: #f44747; }
        .level-debug { color: #9cdcfe; }
        .raw-text { white-space: pre-wrap; }
        .parsed-data { background-color: #0f1419; padding: 10px; margin-top: 5px; border-left: 3px solid #007acc; }
    </style>
</head>
<body>
    <div class="container">
`)

	// Write metadata section
	w.WriteString(`        <div class="metadata">`)
	w.WriteString(fmt.Sprintf(`            <h2>Export Information</h2>`))
	w.WriteString(fmt.Sprintf(`            <p>Exported at: %s</p>`, time.Now().Format(time.RFC3339)))
	w.WriteString(fmt.Sprintf(`            <p>Total lines: %d</p>`, len(lines)))
	
	for key, value := range options.Metadata {
		w.WriteString(fmt.Sprintf(`            <p>%s: %s</p>`, e.escapeHTML(key), e.escapeHTML(value)))
	}
	
	w.WriteString(`        </div>`)

	// Write log lines
	w.WriteString(`        <div class="log-lines">`)
	
	for _, line := range lines {
		w.WriteString(`            <div class="log-line">`)
		
		// Timestamp
		if !line.Timestamp.IsZero() {
			w.WriteString(fmt.Sprintf(`                <span class="timestamp">%s</span> `, 
				line.Timestamp.Format("2006-01-02 15:04:05")))
		}
		
		// Source
		if line.Source != "" {
			w.WriteString(fmt.Sprintf(`<span class="source">[%s]</span> `, e.escapeHTML(line.Source)))
		}
		
		// Level
		if line.Level != "" {
			levelClass := "level-" + strings.ToLower(line.Level)
			w.WriteString(fmt.Sprintf(`<span class="%s">%s</span> `, levelClass, line.Level))
		}
		
		// Raw text
		w.WriteString(fmt.Sprintf(`<span class="raw-text">%s</span>`, e.escapeHTML(line.Raw)))
		
		// Parsed data if requested
		if options.IncludeParsed && line.Parsed != nil {
			parsedJSON, _ := json.MarshalIndent(line.Parsed, "", "  ")
			w.WriteString(fmt.Sprintf(`                <div class="parsed-data">%s</div>`, 
				e.escapeHTML(string(parsedJSON))))
		}
		
		w.WriteString(`            </div>`)
	}
	
	w.WriteString(`        </div>`)

	// Write HTML footer
	w.WriteString(`    </div>
</body>
</html>`)

	return nil
}

// formatLineForText formats a line for text output
func (e *Exporter) formatLineForText(line *models.LogLine) string {
	var parts []string
	
	// Add timestamp if available
	if !line.Timestamp.IsZero() {
		parts = append(parts, line.Timestamp.Format("2006-01-02 15:04:05"))
	}
	
	// Add source if available
	if line.Source != "" {
		parts = append(parts, fmt.Sprintf("[%s]", line.Source))
	}
	
	// Add level if available
	if line.Level != "" {
		parts = append(parts, fmt.Sprintf("%s:", line.Level))
	}
	
	// Add raw text
	parts = append(parts, line.Raw)
	
	return strings.Join(parts, " ")
}

// escapeCSV escapes a string for CSV output
func (e *Exporter) escapeCSV(s string) string {
	// If the string contains comma, quote, or newline, wrap it in quotes
	if strings.ContainsAny(s, ",\"\n\r") {
		// Escape internal quotes by doubling them
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return "\"" + s + "\""
	}
	return s
}

// escapeHTML escapes a string for HTML output
func (e *Exporter) escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// GetSupportedFormats returns the list of supported export formats
func (e *Exporter) GetSupportedFormats() []ExportFormat {
	return []ExportFormat{FormatText, FormatJSON, FormatCSV, FormatHTML}
}

// GenerateDefaultOptions returns default export options
func (e *Exporter) GenerateDefaultOptions(outputPath string, format ExportFormat) ExportOptions {
	return ExportOptions{
		Format:        format,
		OutputPath:    outputPath,
		IncludeRaw:    true,
		IncludeParsed: false,
		IncludeTokens: false,
		Metadata: map[string]string{
			"tool":    "traceace",
			"version": "1.0.0",
		},
	}
}
