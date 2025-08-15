package filter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	
	"github.com/loganalyzer/traceace/pkg/models"
)

// QueryExpression represents a parsed query expression tree
type QueryExpression interface {
	Evaluate(line *models.LogLine, f *FilterEngine) bool
	String() string
}

// AndExpression represents AND logic
type AndExpression struct {
	Left  QueryExpression
	Right QueryExpression
}

func (e *AndExpression) Evaluate(line *models.LogLine, f *FilterEngine) bool {
	return e.Left.Evaluate(line, f) && e.Right.Evaluate(line, f)
}

func (e *AndExpression) String() string {
	return fmt.Sprintf("(%s AND %s)", e.Left.String(), e.Right.String())
}

// OrExpression represents OR logic
type OrExpression struct {
	Left  QueryExpression
	Right QueryExpression
}

func (e *OrExpression) Evaluate(line *models.LogLine, f *FilterEngine) bool {
	return e.Left.Evaluate(line, f) || e.Right.Evaluate(line, f)
}

func (e *OrExpression) String() string {
	return fmt.Sprintf("(%s OR %s)", e.Left.String(), e.Right.String())
}

// NotExpression represents NOT logic
type NotExpression struct {
	Expression QueryExpression
}

func (e *NotExpression) Evaluate(line *models.LogLine, f *FilterEngine) bool {
	return !e.Expression.Evaluate(line, f)
}

func (e *NotExpression) String() string {
	return fmt.Sprintf("NOT %s", e.Expression.String())
}

// FieldExpression represents field-based queries
type FieldExpression struct {
	Field    string
	Operator string
	Value    string
	Pattern  *regexp.Regexp
}

func (e *FieldExpression) Evaluate(line *models.LogLine, f *FilterEngine) bool {
	fieldValue := f.extractFieldValue(line, e.Field)
	return f.matchFieldExpression(fieldValue, e)
}

func (e *FieldExpression) String() string {
	return fmt.Sprintf("%s%s%s", e.Field, e.Operator, e.Value)
}

// TextExpression represents simple text searches
type TextExpression struct {
	Text          string
	CaseSensitive bool
	IsRegex       bool
	Pattern       *regexp.Regexp
}

func (e *TextExpression) Evaluate(line *models.LogLine, f *FilterEngine) bool {
	text := line.Raw
	
	if e.IsRegex && e.Pattern != nil {
		return e.Pattern.MatchString(text)
	}
	
	if !e.CaseSensitive {
		text = strings.ToLower(text)
		return strings.Contains(text, strings.ToLower(e.Text))
	}
	
	return strings.Contains(text, e.Text)
}

func (e *TextExpression) String() string {
	if e.IsRegex {
		return fmt.Sprintf("~\"%s\"", e.Text)
	}
	return fmt.Sprintf("\"%s\"", e.Text)
}

// TimeRangeExpression represents time-based filtering
type TimeRangeExpression struct {
	Start time.Time
	End   time.Time
}

func (e *TimeRangeExpression) Evaluate(line *models.LogLine, f *FilterEngine) bool {
	if line.Timestamp.IsZero() {
		return false
	}
	return !line.Timestamp.Before(e.Start) && !line.Timestamp.After(e.End)
}

func (e *TimeRangeExpression) String() string {
	return fmt.Sprintf("time:[%s TO %s]", e.Start.Format("15:04:05"), e.End.Format("15:04:05"))
}

// AdvancedQueryParser parses complex filter expressions
type AdvancedQueryParser struct {
	input string
	pos   int
}

// ParseAdvancedQuery parses an advanced query string into an expression tree
func (f *FilterEngine) ParseAdvancedQuery(query string) (QueryExpression, error) {
	parser := &AdvancedQueryParser{
		input: strings.TrimSpace(query),
		pos:   0,
	}
	
	return parser.parseExpression()
}

// parseExpression parses the main expression (handles OR at top level)
func (p *AdvancedQueryParser) parseExpression() (QueryExpression, error) {
	left, err := p.parseAndExpression()
	if err != nil {
		return nil, err
	}
	
	for p.pos < len(p.input) {
		p.skipWhitespace()
		if !p.matchKeyword("OR") {
			break
		}
		
		right, err := p.parseAndExpression()
		if err != nil {
			return nil, err
		}
		
		left = &OrExpression{Left: left, Right: right}
	}
	
	return left, nil
}

// parseAndExpression parses AND expressions
func (p *AdvancedQueryParser) parseAndExpression() (QueryExpression, error) {
	left, err := p.parseUnaryExpression()
	if err != nil {
		return nil, err
	}
	
	for p.pos < len(p.input) {
		p.skipWhitespace()
		
		// Check for explicit AND or implicit AND (space)
		isExplicitAnd := p.matchKeyword("AND")
		if !isExplicitAnd {
			// Check if next token looks like another expression
			oldPos := p.pos
			if !p.lookaheadForExpression() {
				p.pos = oldPos
				break
			}
			p.pos = oldPos
		}
		
		right, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}
		
		left = &AndExpression{Left: left, Right: right}
	}
	
	return left, nil
}

// parseUnaryExpression parses NOT expressions and primary expressions
func (p *AdvancedQueryParser) parseUnaryExpression() (QueryExpression, error) {
	p.skipWhitespace()
	
	if p.matchKeyword("NOT") {
		expr, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}
		return &NotExpression{Expression: expr}, nil
	}
	
	return p.parsePrimaryExpression()
}

// parsePrimaryExpression parses field queries, text queries, and parentheses
func (p *AdvancedQueryParser) parsePrimaryExpression() (QueryExpression, error) {
	p.skipWhitespace()
	
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of query")
	}
	
	// Handle parentheses
	if p.input[p.pos] == '(' {
		p.pos++ // consume '('
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		
		p.skipWhitespace()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++ // consume ')'
		return expr, nil
	}
	
	// Parse field or text expression
	token := p.parseToken()
	if token == "" {
		return nil, fmt.Errorf("empty expression")
	}
	
	// Check for field query (contains ':')
	if strings.Contains(token, ":") {
		return p.parseFieldExpression(token)
	}
	
	// Check for time range query
	if strings.HasPrefix(token, "time:[") {
		return p.parseTimeRangeExpression(token)
	}
	
	// Handle text expression
	return p.parseTextExpression(token)
}

// parseFieldExpression parses field-based expressions
func (p *AdvancedQueryParser) parseFieldExpression(token string) (*FieldExpression, error) {
	// Find operator
	operators := []string{":!=", ":~", ":>", ":<", ":>=", ":<=", ":"}
	var field, operator, value string
	
	for _, op := range operators {
		if idx := strings.Index(token, op); idx != -1 {
			field = token[:idx]
			operator = op
			value = token[idx+len(op):]
			break
		}
	}
	
	if field == "" || operator == "" {
		return nil, fmt.Errorf("invalid field expression: %s", token)
	}
	
	expr := &FieldExpression{
		Field:    field,
		Operator: operator,
		Value:    value,
	}
	
	// Compile regex if needed
	if operator == ":~" || (strings.Contains(value, "|") && strings.Contains(value, "(")) {
		pattern, err := regexp.Compile(value)
		if err != nil {
			return nil, fmt.Errorf("invalid regex in field expression: %w", err)
		}
		expr.Pattern = pattern
	}
	
	return expr, nil
}

// parseTimeRangeExpression parses time range expressions
func (p *AdvancedQueryParser) parseTimeRangeExpression(token string) (*TimeRangeExpression, error) {
	// Format: time:[HH:MM:SS TO HH:MM:SS] or time:[YYYY-MM-DD HH:MM:SS TO YYYY-MM-DD HH:MM:SS]
	if !strings.HasPrefix(token, "time:[") || !strings.HasSuffix(token, "]") {
		return nil, fmt.Errorf("invalid time range format: %s", token)
	}
	
	timeSpec := token[6 : len(token)-1] // Remove "time:[" and "]"
	parts := strings.Split(timeSpec, " TO ")
	if len(parts) != 2 {
		return nil, fmt.Errorf("time range must have format: time:[start TO end]")
	}
	
	start, err := p.parseTimeValue(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}
	
	end, err := p.parseTimeValue(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}
	
	return &TimeRangeExpression{Start: start, End: end}, nil
}

// parseTimeValue parses various time formats
func (p *AdvancedQueryParser) parseTimeValue(timeStr string) (time.Time, error) {
	formats := []string{
		"15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	
	now := time.Now()
	
	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			// If only time is specified, use today's date
			if format == "15:04:05" {
				return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, now.Location()), nil
			}
			return t, nil
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

// parseTextExpression parses text-based expressions
func (p *AdvancedQueryParser) parseTextExpression(token string) (*TextExpression, error) {
	expr := &TextExpression{
		Text:          token,
		CaseSensitive: false,
		IsRegex:       false,
	}
	
	// Check for regex prefix
	if strings.HasPrefix(token, "~") {
		expr.IsRegex = true
		expr.Text = token[1:]
		
		pattern, err := regexp.Compile("(?i)" + expr.Text)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
		expr.Pattern = pattern
	}
	
	// Remove quotes if present
	if strings.HasPrefix(expr.Text, "\"") && strings.HasSuffix(expr.Text, "\"") {
		expr.Text = expr.Text[1 : len(expr.Text)-1]
		expr.CaseSensitive = true
	}
	
	return expr, nil
}

// Utility functions

func (p *AdvancedQueryParser) skipWhitespace() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
		p.pos++
	}
}

func (p *AdvancedQueryParser) matchKeyword(keyword string) bool {
	p.skipWhitespace()
	if p.pos+len(keyword) > len(p.input) {
		return false
	}
	
	if strings.ToUpper(p.input[p.pos:p.pos+len(keyword)]) == keyword {
		// Check that it's a complete word
		end := p.pos + len(keyword)
		if end < len(p.input) && isAlphaNumeric(p.input[end]) {
			return false
		}
		p.pos += len(keyword)
		return true
	}
	
	return false
}

func (p *AdvancedQueryParser) parseToken() string {
	p.skipWhitespace()
	start := p.pos
	
	if p.pos >= len(p.input) {
		return ""
	}
	
	// Handle quoted strings
	if p.input[p.pos] == '"' {
		p.pos++ // consume opening quote
		for p.pos < len(p.input) && p.input[p.pos] != '"' {
			p.pos++
		}
		if p.pos < len(p.input) {
			p.pos++ // consume closing quote
		}
		return p.input[start:p.pos]
	}
	
	// Handle time range expressions
	if strings.HasPrefix(p.input[p.pos:], "time:[") {
		depth := 0
		for p.pos < len(p.input) {
			if p.input[p.pos] == '[' {
				depth++
			} else if p.input[p.pos] == ']' {
				depth--
				if depth == 0 {
					p.pos++
					break
				}
			}
			p.pos++
		}
		return p.input[start:p.pos]
	}
	
	// Handle regular tokens
	for p.pos < len(p.input) && !isWhitespace(p.input[p.pos]) && p.input[p.pos] != '(' && p.input[p.pos] != ')' {
		p.pos++
	}
	
	return p.input[start:p.pos]
}

func (p *AdvancedQueryParser) lookaheadForExpression() bool {
	oldPos := p.pos
	defer func() { p.pos = oldPos }()
	
	token := p.parseToken()
	return token != "" && (strings.Contains(token, ":") || 
		strings.HasPrefix(token, "time:[") || 
		strings.HasPrefix(token, "~") ||
		strings.HasPrefix(token, "\""))
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isAlphaNumeric(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// extractFieldValue extracts field value from a log line
func (f *FilterEngine) extractFieldValue(line *models.LogLine, field string) string {
	switch strings.ToLower(field) {
	case "level", "severity", "lvl":
		return line.Level
	case "source", "file", "src":
		return line.Source
	case "message", "msg", "text", "raw":
		return line.Raw
	case "timestamp", "time", "ts":
		if !line.Timestamp.IsZero() {
			return line.Timestamp.Format(time.RFC3339)
		}
		return ""
	case "id":
		return line.ID
	case "line", "linenum":
		return strconv.Itoa(line.LineNum)
	case "offset":
		return strconv.FormatInt(line.Offset, 10)
	default:
		// Check parsed fields for structured logs (JSON/YAML)
		if line.Parsed != nil {
			if val := f.parser.GetParsedField(line, field); val != nil {
				return fmt.Sprintf("%v", val)
			}
		}
		return ""
	}
}

// matchFieldExpression matches a field value against an expression
func (f *FilterEngine) matchFieldExpression(fieldValue string, expr *FieldExpression) bool {
	switch expr.Operator {
	case ":":
		return strings.EqualFold(fieldValue, expr.Value)
	case ":!=":
		return !strings.EqualFold(fieldValue, expr.Value)
	case ":~":
		if expr.Pattern != nil {
			return expr.Pattern.MatchString(fieldValue)
		}
		return false
	case ":>", ":<", ":>=", ":<=":
		return f.matchComparison(fieldValue, expr.Value, expr.Operator)
	default:
		return strings.EqualFold(fieldValue, expr.Value)
	}
}

// matchComparison handles comparison operations
func (f *FilterEngine) matchComparison(fieldValue, queryValue, operator string) bool {
	// Try numeric comparison first
	fieldNum, err1 := strconv.ParseFloat(fieldValue, 64)
	queryNum, err2 := strconv.ParseFloat(queryValue, 64)
	
	if err1 == nil && err2 == nil {
		switch operator {
		case ":>":
			return fieldNum > queryNum
		case ":<":
			return fieldNum < queryNum
		case ":>=":
			return fieldNum >= queryNum
		case ":<=":
			return fieldNum <= queryNum
		}
	}
	
	// Fallback to string comparison
	switch operator {
	case ":>":
		return fieldValue > queryValue
	case ":<":
		return fieldValue < queryValue
	case ":>=":
		return fieldValue >= queryValue
	case ":<=":
		return fieldValue <= queryValue
	}
	
	return false
}

// SetAdvancedFilter sets an advanced filter using the expression tree
func (f *FilterEngine) SetAdvancedFilter(query string) error {
	if query == "" {
		f.Clear()
		return nil
	}
	
	expr, err := f.ParseAdvancedQuery(query)
	if err != nil {
		return fmt.Errorf("failed to parse advanced query: %w", err)
	}
	
	// Store the compiled expression
	f.compiledQuery = &CompiledQuery{
		KeywordQuery: query, // Store original query for summary
	}
	f.advancedExpression = expr
	
	return nil
}

// FilterEngine methods for advanced filtering


