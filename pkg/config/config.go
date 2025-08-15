package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/loganalyzer/traceace/pkg/models"
	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	UI             UIConfig                `mapstructure:"ui" yaml:"ui"`
	HighlightRules []HighlightRule         `mapstructure:"highlight_rules" yaml:"highlight_rules"`
	SavedQueries   []models.SavedQuery     `mapstructure:"saved_queries" yaml:"saved_queries"`
	Keybindings    map[string]string       `mapstructure:"keybindings" yaml:"keybindings"`
	General        GeneralConfig           `mapstructure:"general" yaml:"general"`
}

// UIConfig represents UI-specific configuration
type UIConfig struct {
	Theme           string `mapstructure:"theme" yaml:"theme"`
	ContextLines    int    `mapstructure:"context_lines" yaml:"context_lines"`
	MaxBufferLines  int    `mapstructure:"max_buffer_lines" yaml:"max_buffer_lines"`
	RefreshRate     int    `mapstructure:"refresh_rate_ms" yaml:"refresh_rate_ms"`
	ShowLineNumbers bool   `mapstructure:"show_line_numbers" yaml:"show_line_numbers"`
}

// HighlightRule represents a syntax highlighting rule
type HighlightRule struct {
	Name    string `mapstructure:"name" yaml:"name"`
	Pattern string `mapstructure:"pattern" yaml:"pattern"`
	Color   string `mapstructure:"color" yaml:"color"`
	Style   string `mapstructure:"style" yaml:"style"`
}

// GeneralConfig represents general application settings
type GeneralConfig struct {
	LogLevel           string `mapstructure:"log_level" yaml:"log_level"`
	EnableTelemetry    bool   `mapstructure:"enable_telemetry" yaml:"enable_telemetry"`
	MaxIndexSize       int64  `mapstructure:"max_index_size" yaml:"max_index_size"`
	FileRotationCheck  int    `mapstructure:"file_rotation_check_ms" yaml:"file_rotation_check_ms"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		UI: UIConfig{
			Theme:           "dark",
			ContextLines:    3,
			MaxBufferLines:  1000000,  // 1M lines for large file support
			RefreshRate:     50,       // Faster refresh for smoother scrolling
			ShowLineNumbers: true,
		},
		HighlightRules: []HighlightRule{
			{
				Name:    "timestamp",
				Pattern: `\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`,
				Color:   "cyan",
				Style:   "normal",
			},
			{
				Name:    "loglevel",
				Pattern: `\b(ERROR|WARN|INFO|DEBUG|TRACE|FATAL)\b`,
				Color:   "auto", // auto-color based on level
				Style:   "bold",
			},
			{
				Name:    "ip_address",
				Pattern: `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
				Color:   "yellow",
				Style:   "normal",
			},
			{
				Name:    "status_code",
				Pattern: `\b[1-5]\d{2}\b`,
				Color:   "auto", // auto-color based on status range
				Style:   "normal",
			},
			{
				Name:    "uuid",
				Pattern: `\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`,
				Color:   "magenta",
				Style:   "normal",
			},
			{
				Name:    "url",
				Pattern: `https?://[^\s]+`,
				Color:   "blue",
				Style:   "underline",
			},
		},
		SavedQueries: []models.SavedQuery{
			{
				Name:        "errors",
				Query:       "level:ERROR",
				Description: "Show all error level logs",
				IsRegex:     false,
			},
			{
				Name:        "warnings_and_errors",
				Query:       "level:(ERROR|WARN)",
				Description: "Show warnings and errors",
				IsRegex:     true,
			},
		},
		Keybindings: map[string]string{
			"search":           "/",
			"escape":           "esc",
			"next_match":       "n",
			"prev_match":       "N",
			"pause_resume":     "space",
			"bookmark":         "b",
			"export":           "e",
			"toggle_view":      "t",
			"help":             "?",
			"quit":             "q",
			"scroll_up":        "k",
			"scroll_down":      "j",
			"page_up":          "ctrl+u",
			"page_down":        "ctrl+d",
			"goto_top":         "g",
			"goto_bottom":      "G",
			"next_tab":         "tab",
			"prev_tab":         "shift+tab",
		},
		General: GeneralConfig{
			LogLevel:           "info",
			EnableTelemetry:    false,
			MaxIndexSize:       100 * 1024 * 1024, // 100MB
			FileRotationCheck:  1000,               // 1 second
		},
	}
}

// ConfigDir returns the configuration directory path
func ConfigDir() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config")
	}
	
	appConfigDir := filepath.Join(configDir, "traceace")
	
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(appConfigDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}
	
	return appConfigDir, nil
}

// Load loads the configuration from the config file
func Load() (*Config, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	
	// configFile := filepath.Join(configDir, "config.yaml")
	
	// Start with defaults
	config := DefaultConfig()
	
	// Set up viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)
	
	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, create it with defaults
			if err := Save(config); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		// Unmarshal the config
		if err := viper.Unmarshal(config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}
	
	return config, nil
}

// Save saves the configuration to the config file
func Save(config *Config) error {
	configDir, err := ConfigDir()
	if err != nil {
		return err
	}
	
	configFile := filepath.Join(configDir, "config.yaml")
	
	// Set up viper with the config
	viper.Set("ui", config.UI)
	viper.Set("highlight_rules", config.HighlightRules)
	viper.Set("saved_queries", config.SavedQueries)
	viper.Set("keybindings", config.Keybindings)
	viper.Set("general", config.General)
	
	// Write to file
	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	return nil
}

// AddSavedQuery adds a new saved query to the configuration
func (c *Config) AddSavedQuery(query models.SavedQuery) error {
	// Check if query with this name already exists
	for i, existing := range c.SavedQueries {
		if existing.Name == query.Name {
			c.SavedQueries[i] = query
			return Save(c)
		}
	}
	
	// Add new query
	c.SavedQueries = append(c.SavedQueries, query)
	return Save(c)
}

// RemoveSavedQuery removes a saved query by name
func (c *Config) RemoveSavedQuery(name string) error {
	for i, query := range c.SavedQueries {
		if query.Name == name {
			c.SavedQueries = append(c.SavedQueries[:i], c.SavedQueries[i+1:]...)
			return Save(c)
		}
	}
	return nil
}

// GetKeybinding returns the key binding for a given action
func (c *Config) GetKeybinding(action string) string {
	if binding, exists := c.Keybindings[action]; exists {
		return binding
	}
	// Return default if not found
	defaults := DefaultConfig()
	return defaults.Keybindings[action]
}
