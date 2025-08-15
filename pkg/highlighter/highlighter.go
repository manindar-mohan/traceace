package highlighter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/loganalyzer/traceace/pkg/config"
	"github.com/loganalyzer/traceace/pkg/models"
)

// Highlighter provides syntax highlighting for log lines
type Highlighter struct {
	rules  []HighlightRule
	styles map[string]lipgloss.Style
	theme  Theme
}

// HighlightRule represents a single highlighting rule
type HighlightRule struct {
	Name        string
	Pattern     *regexp.Regexp
	TokenType   models.TokenType
	ColorMapper ColorMapper
	StyleFunc   StyleFunc
}

// ColorMapper is a function that returns a color based on the matched text
type ColorMapper func(text string) lipgloss.Color

// StyleFunc is a function that returns a style based on the matched text
type StyleFunc func(text string) lipgloss.Style

// Theme represents a color theme
type Theme struct {
	Name       string
	Background lipgloss.Color
	Foreground lipgloss.Color
	Colors     map[string]lipgloss.Color
}

// Predefined themes
var (
	DarkTheme = Theme{
		Name:       "dark",
		Background: lipgloss.Color("#1e1e1e"),
		Foreground: lipgloss.Color("#d4d4d4"),
		Colors: map[string]lipgloss.Color{
			"timestamp":   lipgloss.Color("#4fc1ff"),
			"level_debug": lipgloss.Color("#9cdcfe"),
			"level_info":  lipgloss.Color("#4ec9b0"),
			"level_warn":  lipgloss.Color("#dcdcaa"),
			"level_error": lipgloss.Color("#f44747"),
			"level_fatal": lipgloss.Color("#ff6b6b"),
			"ip":          lipgloss.Color("#ce9178"),
			"status_2xx":  lipgloss.Color("#4ec9b0"),
			"status_3xx":  lipgloss.Color("#dcdcaa"),
			"status_4xx":  lipgloss.Color("#ffa500"),
			"status_5xx":  lipgloss.Color("#f44747"),
			"uuid":        lipgloss.Color("#d7ba7d"),
			"url":         lipgloss.Color("#569cd6"),
			"number":      lipgloss.Color("#b5cea8"),
			"string":      lipgloss.Color("#ce9178"),
			"keyword":     lipgloss.Color("#c586c0"),
			"json":        lipgloss.Color("#6a9955"),
			"error_text":  lipgloss.Color("#f44747"),
		},
	}

	LightTheme = Theme{
		Name:       "light",
		Background: lipgloss.Color("#ffffff"),
		Foreground: lipgloss.Color("#333333"),
		Colors: map[string]lipgloss.Color{
			"timestamp":   lipgloss.Color("#0969da"),
			"level_debug": lipgloss.Color("#656d76"),
			"level_info":  lipgloss.Color("#1f883d"),
			"level_warn":  lipgloss.Color("#9a6700"),
			"level_error": lipgloss.Color("#d1242f"),
			"level_fatal": lipgloss.Color("#a40e26"),
			"ip":          lipgloss.Color("#0550ae"),
			"status_2xx":  lipgloss.Color("#1f883d"),
			"status_3xx":  lipgloss.Color("#9a6700"),
			"status_4xx":  lipgloss.Color("#bc4c00"),
			"status_5xx":  lipgloss.Color("#d1242f"),
			"uuid":        lipgloss.Color("#6639ba"),
			"url":         lipgloss.Color("#0969da"),
			"number":      lipgloss.Color("#0550ae"),
			"string":      lipgloss.Color("#0a3069"),
			"keyword":     lipgloss.Color("#8250df"),
			"json":        lipgloss.Color("#1f883d"),
			"error_text":  lipgloss.Color("#d1242f"),
		},
	}

	MonochromeTheme = Theme{
		Name:       "monochrome",
		Background: lipgloss.Color("#000000"),
		Foreground: lipgloss.Color("#ffffff"),
		Colors: map[string]lipgloss.Color{
			"timestamp":   lipgloss.Color("#ffffff"),
			"level_debug": lipgloss.Color("#808080"),
			"level_info":  lipgloss.Color("#ffffff"),
			"level_warn":  lipgloss.Color("#ffffff"),
			"level_error": lipgloss.Color("#ffffff"),
			"level_fatal": lipgloss.Color("#ffffff"),
			"ip":          lipgloss.Color("#ffffff"),
			"status_2xx":  lipgloss.Color("#ffffff"),
			"status_3xx":  lipgloss.Color("#ffffff"),
			"status_4xx":  lipgloss.Color("#ffffff"),
			"status_5xx":  lipgloss.Color("#ffffff"),
			"uuid":        lipgloss.Color("#ffffff"),
			"url":         lipgloss.Color("#ffffff"),
			"number":      lipgloss.Color("#ffffff"),
			"string":      lipgloss.Color("#ffffff"),
			"keyword":     lipgloss.Color("#ffffff"),
			"json":        lipgloss.Color("#ffffff"),
			"error_text":  lipgloss.Color("#ffffff"),
		},
	}
)

// New creates a new Highlighter with the specified theme
func New(cfg *config.Config) *Highlighter {
	h := &Highlighter{
		rules:  []HighlightRule{},
		styles: make(map[string]lipgloss.Style),
	}

	// Set theme
	switch cfg.UI.Theme {
	case "light":
		h.theme = LightTheme
	case "monochrome":
		h.theme = MonochromeTheme
	default:
		h.theme = DarkTheme
	}

	// Build highlight rules from config
	h.buildRules(cfg.HighlightRules)

	return h
}

// buildRules builds highlighting rules from configuration
func (h *Highlighter) buildRules(configRules []config.HighlightRule) {
	for _, rule := range configRules {
		pattern, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue // Skip invalid patterns
		}

		hlRule := HighlightRule{
			Name:    rule.Name,
			Pattern: pattern,
		}

		// Set color mapper and style function based on rule type
		switch rule.Name {
		case "timestamp":
			hlRule.TokenType = models.TokenTimestamp
			hlRule.ColorMapper = h.getStaticColor("timestamp")
			hlRule.StyleFunc = h.getStaticStyle("timestamp", false, false)

		case "loglevel":
			hlRule.TokenType = models.TokenLevel
			hlRule.ColorMapper = h.getLevelColor
			hlRule.StyleFunc = h.getLevelStyle

		case "ip_address":
			hlRule.TokenType = models.TokenIP
			hlRule.ColorMapper = h.getStaticColor("ip")
			hlRule.StyleFunc = h.getStaticStyle("ip", false, false)

		case "status_code":
			hlRule.TokenType = models.TokenStatusCode
			hlRule.ColorMapper = h.getStatusCodeColor
			hlRule.StyleFunc = h.getStatusCodeStyle

		case "uuid":
			hlRule.TokenType = models.TokenUUID
			hlRule.ColorMapper = h.getStaticColor("uuid")
			hlRule.StyleFunc = h.getStaticStyle("uuid", false, false)

		case "url":
			hlRule.TokenType = models.TokenURL
			hlRule.ColorMapper = h.getStaticColor("url")
			hlRule.StyleFunc = h.getStaticStyle("url", false, true)

		default:
			hlRule.TokenType = models.TokenDefault
			hlRule.ColorMapper = h.getConfigColor(rule.Color)
			hlRule.StyleFunc = h.getConfigStyle(rule.Color, rule.Style)
		}

		h.rules = append(h.rules, hlRule)
	}

	// Add built-in rules
	h.addBuiltinRules()
}

// addBuiltinRules adds built-in highlighting rules
func (h *Highlighter) addBuiltinRules() {
	builtinRules := []struct {
		name      string
		pattern   string
		tokenType models.TokenType
		colorKey  string
		bold      bool
		underline bool
	}{
		{"number", `\b\d+\b`, models.TokenNumber, "number", false, false},
		{"quoted_string", `"[^"]*"`, models.TokenString, "string", false, false},
		{"json_brace", `[{}[\]]`, models.TokenJSON, "json", false, false},
		{"error_keywords", `\b(error|exception|failed|failure|fatal|panic|crash)\b`, models.TokenError, "error_text", true, false},
	}

	for _, rule := range builtinRules {
		pattern, err := regexp.Compile(`(?i)` + rule.pattern)
		if err != nil {
			continue
		}

		hlRule := HighlightRule{
			Name:        rule.name,
			Pattern:     pattern,
			TokenType:   rule.tokenType,
			ColorMapper: h.getStaticColor(rule.colorKey),
			StyleFunc:   h.getStaticStyle(rule.colorKey, rule.bold, rule.underline),
		}

		h.rules = append(h.rules, hlRule)
	}
}

// Highlight processes a log line and applies syntax highlighting
func (h *Highlighter) Highlight(line *models.LogLine) string {
	if line == nil || line.Raw == "" {
		return ""
	}

	// Start with the raw text
	result := line.Raw
	tokens := []models.Token{}

	// Apply all rules
	for _, rule := range h.rules {
		matches := rule.Pattern.FindAllStringSubmatch(result, -1)
		indices := rule.Pattern.FindAllStringIndex(result, -1)

		for i, match := range matches {
			if len(match) > 0 && len(indices) > i {
				start, end := indices[i][0], indices[i][1]
				text := match[0]

				// Create token
				token := models.Token{
					Text:      text,
					TokenType: rule.TokenType,
					Start:     start,
					End:       end,
				}

				tokens = append(tokens, token)
			}
		}
	}

	// Sort tokens by position
	for i := 0; i < len(tokens)-1; i++ {
		for j := i + 1; j < len(tokens); j++ {
			if tokens[i].Start > tokens[j].Start {
				tokens[i], tokens[j] = tokens[j], tokens[i]
			}
		}
	}

	// Remove overlapping tokens (keep the first one found for each position)
	tokens = h.removeOverlappingTokens(tokens)

	// Apply styling
	styledResult := h.applyStyles(result, tokens)

	// Store tokens in the line
	line.Tokens = tokens

	return styledResult
}

// applyStyles applies styles to the text based on tokens
func (h *Highlighter) applyStyles(text string, tokens []models.Token) string {
	if len(tokens) == 0 {
		return text
	}

	var result strings.Builder
	lastEnd := 0

	for _, token := range tokens {
		// Add unstyled text before this token
		if token.Start > lastEnd {
			result.WriteString(text[lastEnd:token.Start])
		}

		// Find the appropriate style for this token
		style := h.getTokenStyle(token)
		styledText := style.Render(token.Text)
		result.WriteString(styledText)

		lastEnd = token.End
	}

	// Add any remaining unstyled text
	if lastEnd < len(text) {
		result.WriteString(text[lastEnd:])
	}

	return result.String()
}

// getTokenStyle returns the appropriate style for a token
func (h *Highlighter) getTokenStyle(token models.Token) lipgloss.Style {
	// Find the rule that created this token
	for _, rule := range h.rules {
		if rule.TokenType == token.TokenType {
			return rule.StyleFunc(token.Text)
		}
	}

	// Default style
	return lipgloss.NewStyle().Foreground(h.theme.Foreground)
}

// Color mapper functions
func (h *Highlighter) getStaticColor(key string) ColorMapper {
	return func(text string) lipgloss.Color {
		if color, exists := h.theme.Colors[key]; exists {
			return color
		}
		return h.theme.Foreground
	}
}

func (h *Highlighter) getLevelColor(text string) lipgloss.Color {
	level := strings.ToUpper(strings.Trim(text, "[] "))
	key := fmt.Sprintf("level_%s", strings.ToLower(level))
	
	if color, exists := h.theme.Colors[key]; exists {
		return color
	}
	
	// Fallback based on level type
	switch level {
	case "ERROR", "FATAL", "PANIC":
		return h.theme.Colors["level_error"]
	case "WARN", "WARNING":
		return h.theme.Colors["level_warn"]
	case "INFO":
		return h.theme.Colors["level_info"]
	case "DEBUG", "TRACE":
		return h.theme.Colors["level_debug"]
	default:
		return h.theme.Foreground
	}
}

func (h *Highlighter) getStatusCodeColor(text string) lipgloss.Color {
	if len(text) >= 1 {
		switch text[0] {
		case '2':
			return h.theme.Colors["status_2xx"]
		case '3':
			return h.theme.Colors["status_3xx"]
		case '4':
			return h.theme.Colors["status_4xx"]
		case '5':
			return h.theme.Colors["status_5xx"]
		}
	}
	return h.theme.Foreground
}

func (h *Highlighter) getConfigColor(colorName string) ColorMapper {
	return func(text string) lipgloss.Color {
		if colorName == "auto" {
			return h.theme.Foreground // Let other mappers handle auto
		}
		return lipgloss.Color(colorName)
	}
}

// Style function generators
func (h *Highlighter) getStaticStyle(key string, bold, underline bool) StyleFunc {
	return func(text string) lipgloss.Style {
		color := h.getStaticColor(key)(text)
		style := lipgloss.NewStyle().Foreground(color)
		
		if bold {
			style = style.Bold(true)
		}
		if underline {
			style = style.Underline(true)
		}
		
		return style
	}
}

func (h *Highlighter) getLevelStyle(text string) lipgloss.Style {
	color := h.getLevelColor(text)
	return lipgloss.NewStyle().Foreground(color).Bold(true)
}

func (h *Highlighter) getStatusCodeStyle(text string) lipgloss.Style {
	color := h.getStatusCodeColor(text)
	return lipgloss.NewStyle().Foreground(color)
}

func (h *Highlighter) getConfigStyle(colorName, styleName string) StyleFunc {
	return func(text string) lipgloss.Style {
		var style lipgloss.Style
		
		if colorName == "auto" {
			style = lipgloss.NewStyle().Foreground(h.theme.Foreground)
		} else {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorName))
		}
		
		switch styleName {
		case "bold":
			style = style.Bold(true)
		case "underline":
			style = style.Underline(true)
		case "italic":
			style = style.Italic(true)
		}
		
		return style
	}
}

// RenderStructured renders structured data (JSON/YAML) with highlighting
func (h *Highlighter) RenderStructured(data map[string]interface{}, indent int) string {
	var result strings.Builder
	indentStr := strings.Repeat("  ", indent)
	
	for key, value := range data {
		result.WriteString(indentStr)
		
		// Highlight key
		keyStyle := lipgloss.NewStyle().Foreground(h.theme.Colors["keyword"]).Bold(true)
		result.WriteString(keyStyle.Render(key))
		result.WriteString(": ")
		
		// Highlight value based on type
		switch v := value.(type) {
		case string:
			stringStyle := lipgloss.NewStyle().Foreground(h.theme.Colors["string"])
			result.WriteString(stringStyle.Render(fmt.Sprintf("\"%s\"", v)))
		case float64, int, int64:
			numberStyle := lipgloss.NewStyle().Foreground(h.theme.Colors["number"])
			result.WriteString(numberStyle.Render(fmt.Sprintf("%v", v)))
		case bool:
			keywordStyle := lipgloss.NewStyle().Foreground(h.theme.Colors["keyword"])
			result.WriteString(keywordStyle.Render(fmt.Sprintf("%t", v)))
		case map[string]interface{}:
			result.WriteString("{\n")
			result.WriteString(h.RenderStructured(v, indent+1))
			result.WriteString(indentStr + "}")
		default:
			result.WriteString(fmt.Sprintf("%v", v))
		}
		
		result.WriteString("\n")
	}
	
	return result.String()
}

// SetTheme changes the current theme
func (h *Highlighter) SetTheme(themeName string) {
	switch themeName {
	case "light":
		h.theme = LightTheme
	case "monochrome":
		h.theme = MonochromeTheme
	default:
		h.theme = DarkTheme
	}
}

// GetAvailableThemes returns the list of available themes
func (h *Highlighter) GetAvailableThemes() []string {
	return []string{"dark", "light", "monochrome"}
}

// removeOverlappingTokens removes overlapping tokens, keeping the first one for each position
func (h *Highlighter) removeOverlappingTokens(tokens []models.Token) []models.Token {
	if len(tokens) <= 1 {
		return tokens
	}
	
	var filtered []models.Token
	for _, token := range tokens {
		overlap := false
		for _, existing := range filtered {
			// Check if tokens overlap
			if (token.Start >= existing.Start && token.Start < existing.End) ||
			   (token.End > existing.Start && token.End <= existing.End) ||
			   (token.Start <= existing.Start && token.End >= existing.End) {
				overlap = true
				break
			}
		}
		if !overlap {
			filtered = append(filtered, token)
		}
	}
	
	return filtered
}
