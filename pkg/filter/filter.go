package filter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/loganalyzer/traceace/pkg/models"
	"github.com/loganalyzer/traceace/pkg/parser"
)

// FilterEngine handles search and filtering operations
type FilterEngine struct {
	parser             *parser.LogParser
	compiledQuery      *CompiledQuery
	advancedExpression QueryExpression // For advanced query expressions
	lastOptions        models.FilterOptions
}

// CompiledQuery represents a pre-compiled filter query for performance
type CompiledQuery struct {
	IsRegex        bool
	CaseSensitive  bool
	Pattern        *regexp.Regexp
	KeywordQuery   string
	FieldQueries   []FieldQuery
	BooleanOp      BooleanOperator
	TimeRange      *models.TimeRange
	LogLevels      map[string]bool
	Sources        map[string]bool
}

// FieldQuery represents a field-specific query (e.g., level:ERROR)
type FieldQuery struct {
	Field    string
	Operator QueryOperator
	Value    string
	Pattern  *regexp.Regexp
}

// BooleanOperator represents boolean operations
type BooleanOperator int

const (
	OpAND BooleanOperator = iota
	OpOR
	OpNOT
)

// QueryOperator represents field query operators
type QueryOperator int

const (
	OpEquals QueryOperator = iota
	OpNotEquals
	OpContains
	OpRegex
	OpGreater
	OpLess
	OpGreaterEqual
	OpLessEqual
)

// New creates a new FilterEngine
func New(p *parser.LogParser) *FilterEngine {
	return &FilterEngine{
		parser: p,
	}
}

// SetFilter compiles and sets the filter options
func (f *FilterEngine) SetFilter(options models.FilterOptions) error {
	f.lastOptions = options
	
	query, err := f.compileQuery(options)
	if err != nil {
		return fmt.Errorf("failed to compile query: %w", err)
	}
	
	f.compiledQuery = query
	return nil
}

// Match returns true if the log line matches the current filter
func (f *FilterEngine) Match(line *models.LogLine) bool {
	// Use advanced expression if available
	if f.advancedExpression != nil {
		return f.advancedExpression.Evaluate(line, f)
	}
	
	if f.compiledQuery == nil {
		return false // No filter set, match nothing (filtered pane should be empty)
	}
	
	return f.matchLine(line, f.compiledQuery)
}

// GetLastOptions returns the last set filter options
func (f *FilterEngine) GetLastOptions() models.FilterOptions {
	return f.lastOptions
}

// compileQuery compiles filter options into an optimized query structure
func (f *FilterEngine) compileQuery(options models.FilterOptions) (*CompiledQuery, error) {
	query := &CompiledQuery{
		IsRegex:       options.IsRegex,
		CaseSensitive: options.CaseSensitive,
		TimeRange:     options.TimeRange,
		LogLevels:     make(map[string]bool),
		Sources:       make(map[string]bool),
	}
	
	// Build log levels map for quick lookup
	for _, level := range options.LogLevels {
		query.LogLevels[strings.ToUpper(level)] = true
	}
	
	// Build sources map for quick lookup
	for _, source := range options.Sources {
		query.Sources[source] = true
	}
	
	// Parse the main query
	if options.Query != "" {
		if err := f.parseMainQuery(options.Query, query); err != nil {
			return nil, err
		}
	}
	
	return query, nil
}

// parseMainQuery parses the main query string
func (f *FilterEngine) parseMainQuery(queryStr string, query *CompiledQuery) error {
	// Check if this is a field-based query (contains ':')
	if strings.Contains(queryStr, ":") {
		return f.parseFieldQueries(queryStr, query)
	}
	
	// Handle as keyword or regex query
	if query.IsRegex {
		flags := ""
		if !query.CaseSensitive {
			flags = "(?i)"
		}
		
		pattern, err := regexp.Compile(flags + queryStr)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
		query.Pattern = pattern
	} else {
		query.KeywordQuery = queryStr
		if !query.CaseSensitive {
			query.KeywordQuery = strings.ToLower(queryStr)
		}
	}
	
	return nil
}

// parseFieldQueries parses field-based queries (e.g., level:ERROR user.id:123)
func (f *FilterEngine) parseFieldQueries(queryStr string, query *CompiledQuery) error {
	// Simple parser for field queries
	// Supports: field:value, field:regex, field:(value1|value2), etc.
	
	parts := f.splitQuery(queryStr)
	
	for _, part := range parts {
		if !strings.Contains(part, ":") {
			// Not a field query, treat as keyword
			if query.KeywordQuery == "" {
				query.KeywordQuery = part
				if !query.CaseSensitive {
					query.KeywordQuery = strings.ToLower(part)
				}
			}
			continue
		}
		
		fieldQuery, err := f.parseFieldQuery(part, query)
		if err != nil {
			return err
		}
		
		query.FieldQueries = append(query.FieldQueries, fieldQuery)
	}
	
	return nil
}

// splitQuery splits a query string into individual parts, respecting parentheses
func (f *FilterEngine) splitQuery(queryStr string) []string {
	var parts []string
	var current strings.Builder
	var inParens int
	var inQuotes bool
	
	for _, char := range queryStr {
		switch char {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(char)
		case '(':
			if !inQuotes {
				inParens++
			}
			current.WriteRune(char)
		case ')':
			if !inQuotes {
				inParens--
			}
			current.WriteRune(char)
		case ' ':
			if !inQuotes && inParens == 0 {
				if current.Len() > 0 {
					parts = append(parts, strings.TrimSpace(current.String()))
					current.Reset()
				}
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}
	
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	
	return parts
}

// parseFieldQuery parses a single field query
func (f *FilterEngine) parseFieldQuery(queryPart string, query *CompiledQuery) (FieldQuery, error) {
	colonIndex := strings.Index(queryPart, ":")
	if colonIndex == -1 {
		return FieldQuery{}, fmt.Errorf("invalid field query: %s", queryPart)
	}
	
	field := strings.TrimSpace(queryPart[:colonIndex])
	value := strings.TrimSpace(queryPart[colonIndex+1:])
	
	fieldQuery := FieldQuery{
		Field:    field,
		Operator: OpEquals,
		Value:    value,
	}
	
	// Determine operator
	if strings.HasPrefix(value, "!") {
		fieldQuery.Operator = OpNotEquals
		fieldQuery.Value = value[1:]
	} else if strings.HasPrefix(value, "~") {
		fieldQuery.Operator = OpRegex
		fieldQuery.Value = value[1:]
	} else if strings.HasPrefix(value, ">") {
		fieldQuery.Operator = OpGreater
		fieldQuery.Value = value[1:]
	} else if strings.HasPrefix(value, "<") {
		fieldQuery.Operator = OpLess
		fieldQuery.Value = value[1:]
	} else if strings.Contains(value, "*") || strings.Contains(value, "?") {
		fieldQuery.Operator = OpContains
	}
	
	// Compile regex if needed
	if fieldQuery.Operator == OpRegex || (query.IsRegex && strings.Contains(value, "|")) {
		flags := ""
		if !query.CaseSensitive {
			flags = "(?i)"
		}
		
		pattern, err := regexp.Compile(flags + fieldQuery.Value)
		if err != nil {
			return FieldQuery{}, fmt.Errorf("invalid regex in field query: %w", err)
		}
		fieldQuery.Pattern = pattern
	}
	
	return fieldQuery, nil
}

// matchLine checks if a log line matches the compiled query
func (f *FilterEngine) matchLine(line *models.LogLine, query *CompiledQuery) bool {
	// Check time range
	if query.TimeRange != nil {
		if !line.Timestamp.IsZero() {
			if line.Timestamp.Before(query.TimeRange.Start) || line.Timestamp.After(query.TimeRange.End) {
				return false
			}
		}
	}
	
	// Check log levels
	if len(query.LogLevels) > 0 {
		if line.Level == "" || !query.LogLevels[strings.ToUpper(line.Level)] {
			return false
		}
	}
	
	// Check sources
	if len(query.Sources) > 0 {
		if !query.Sources[line.Source] {
			return false
		}
	}
	
	// Check main query (keyword or regex)
	mainQueryMatch := true
	if query.KeywordQuery != "" {
		searchText := line.Raw
		if !query.CaseSensitive {
			searchText = strings.ToLower(searchText)
		}
		mainQueryMatch = strings.Contains(searchText, query.KeywordQuery)
	} else if query.Pattern != nil {
		mainQueryMatch = query.Pattern.MatchString(line.Raw)
	}
	
	// Check field queries
	fieldQueriesMatch := true
	if len(query.FieldQueries) > 0 {
		fieldQueriesMatch = f.matchFieldQueries(line, query.FieldQueries)
	}
	
	// Combine results (default is AND)
	return mainQueryMatch && fieldQueriesMatch
}

// matchFieldQueries checks if a line matches field-based queries
func (f *FilterEngine) matchFieldQueries(line *models.LogLine, fieldQueries []FieldQuery) bool {
	for _, fieldQuery := range fieldQueries {
		if !f.matchFieldQuery(line, fieldQuery) {
			return false // All field queries must match (AND logic)
		}
	}
	return true
}

// matchFieldQuery checks if a line matches a single field query
func (f *FilterEngine) matchFieldQuery(line *models.LogLine, fieldQuery FieldQuery) bool {
	var fieldValue string
	
	// Extract field value based on field name
	switch strings.ToLower(fieldQuery.Field) {
	case "level", "severity":
		fieldValue = line.Level
	case "source", "file":
		fieldValue = line.Source
	case "message", "msg", "text":
		fieldValue = line.Raw
	case "timestamp", "time", "ts":
		fieldValue = line.Timestamp.Format(time.RFC3339)
	default:
		// Check parsed fields for structured logs
		if line.Parsed != nil {
			if val := f.parser.GetParsedField(line, fieldQuery.Field); val != nil {
				fieldValue = fmt.Sprintf("%v", val)
			}
		}
	}
	
	return f.matchFieldValue(fieldValue, fieldQuery)
}

// matchFieldValue matches a field value against a field query
func (f *FilterEngine) matchFieldValue(fieldValue string, fieldQuery FieldQuery) bool {
	switch fieldQuery.Operator {
	case OpEquals:
		return strings.EqualFold(fieldValue, fieldQuery.Value)
		
	case OpNotEquals:
		return !strings.EqualFold(fieldValue, fieldQuery.Value)
		
	case OpContains:
		return strings.Contains(strings.ToLower(fieldValue), strings.ToLower(fieldQuery.Value))
		
	case OpRegex:
		if fieldQuery.Pattern != nil {
			return fieldQuery.Pattern.MatchString(fieldValue)
		}
		return false
		
	case OpGreater, OpLess, OpGreaterEqual, OpLessEqual:
		return f.matchNumericComparison(fieldValue, fieldQuery)
		
	default:
		return strings.EqualFold(fieldValue, fieldQuery.Value)
	}
}

// matchNumericComparison handles numeric comparisons
func (f *FilterEngine) matchNumericComparison(fieldValue string, fieldQuery FieldQuery) bool {
	fieldNum, err1 := strconv.ParseFloat(fieldValue, 64)
	queryNum, err2 := strconv.ParseFloat(fieldQuery.Value, 64)
	
	if err1 != nil || err2 != nil {
		// Fallback to string comparison for timestamps, etc.
		return f.matchStringComparison(fieldValue, fieldQuery)
	}
	
	switch fieldQuery.Operator {
	case OpGreater:
		return fieldNum > queryNum
	case OpLess:
		return fieldNum < queryNum
	case OpGreaterEqual:
		return fieldNum >= queryNum
	case OpLessEqual:
		return fieldNum <= queryNum
	default:
		return false
	}
}

// matchStringComparison handles string-based comparisons for non-numeric values
func (f *FilterEngine) matchStringComparison(fieldValue string, fieldQuery FieldQuery) bool {
	switch fieldQuery.Operator {
	case OpGreater:
		return fieldValue > fieldQuery.Value
	case OpLess:
		return fieldValue < fieldQuery.Value
	case OpGreaterEqual:
		return fieldValue >= fieldQuery.Value
	case OpLessEqual:
		return fieldValue <= fieldQuery.Value
	default:
		return false
	}
}

// GetMatchingIndices returns indices of lines that match the filter
func (f *FilterEngine) GetMatchingIndices(lines []*models.LogLine) []int {
	var indices []int
	for i, line := range lines {
		if f.Match(line) {
			indices = append(indices, i)
		}
	}
	return indices
}

// GetMatchCount returns the count of matching lines
func (f *FilterEngine) GetMatchCount(lines []*models.LogLine) int {
	count := 0
	for _, line := range lines {
		if f.Match(line) {
			count++
		}
	}
	return count
}

// Clear clears the current filter
func (f *FilterEngine) Clear() {
	f.compiledQuery = nil
	f.advancedExpression = nil
	f.lastOptions = models.FilterOptions{}
}

// HasFilter returns true if a filter is currently set
func (f *FilterEngine) HasFilter() bool {
	return f.advancedExpression != nil || 
		   (f.compiledQuery != nil && 
		   (f.compiledQuery.KeywordQuery != "" || 
			f.compiledQuery.Pattern != nil || 
			len(f.compiledQuery.FieldQueries) > 0 ||
			len(f.compiledQuery.LogLevels) > 0 ||
			len(f.compiledQuery.Sources) > 0 ||
			f.compiledQuery.TimeRange != nil))
}

// ValidateQuery validates a query string without compiling it
func (f *FilterEngine) ValidateQuery(queryStr string, isRegex bool) error {
	if queryStr == "" {
		return nil
	}
	
	if isRegex {
		_, err := regexp.Compile(queryStr)
		if err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
	}
	
	// Basic validation for field queries
	if strings.Contains(queryStr, ":") {
		parts := f.splitQuery(queryStr)
		for _, part := range parts {
			if strings.Contains(part, ":") {
				colonIndex := strings.Index(part, ":")
				if colonIndex == 0 || colonIndex == len(part)-1 {
					return fmt.Errorf("invalid field query format: %s", part)
				}
			}
		}
	}
	
	return nil
}

// GetFilterSummary returns a human-readable summary of the current filter
func (f *FilterEngine) GetFilterSummary() string {
	if f.compiledQuery == nil {
		return "No filter"
	}
	
	var parts []string
	
	if f.compiledQuery.KeywordQuery != "" {
		parts = append(parts, fmt.Sprintf("text:\"%s\"", f.compiledQuery.KeywordQuery))
	}
	
	if f.compiledQuery.Pattern != nil {
		parts = append(parts, fmt.Sprintf("regex:\"%s\"", f.compiledQuery.Pattern.String()))
	}
	
	for _, fieldQuery := range f.compiledQuery.FieldQueries {
		parts = append(parts, fmt.Sprintf("%s:%s", fieldQuery.Field, fieldQuery.Value))
	}
	
	if len(f.compiledQuery.LogLevels) > 0 {
		var levels []string
		for level := range f.compiledQuery.LogLevels {
			levels = append(levels, level)
		}
		parts = append(parts, fmt.Sprintf("levels:%v", levels))
	}
	
	if len(f.compiledQuery.Sources) > 0 {
		var sources []string
		for source := range f.compiledQuery.Sources {
			sources = append(sources, source)
		}
		parts = append(parts, fmt.Sprintf("sources:%v", sources))
	}
	
	if f.compiledQuery.TimeRange != nil {
		parts = append(parts, fmt.Sprintf("time:%s to %s", 
			f.compiledQuery.TimeRange.Start.Format("15:04:05"),
			f.compiledQuery.TimeRange.End.Format("15:04:05")))
	}
	
	if len(parts) == 0 {
		return "No filter"
	}
	
	return strings.Join(parts, ", ")
}
