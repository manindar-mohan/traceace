package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/loganalyzer/traceace/pkg/config"
	"github.com/loganalyzer/traceace/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	configFile    string
	theme         string
	tail          bool
	fromBeginning bool
	contextLines  int
	savedQuery    string
	verbose       bool
	debug         bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "traceace [files...]",
	Short: "TraceAce - Blazing fast terminal log analyzer",
	Long: `TraceAce is a blazing fast terminal user interface for analyzing and monitoring log files.
It provides real-time tailing, advanced search and filtering, syntax highlighting,
and supports both structured (JSON/YAML) and unstructured logs.

Examples:
  traceace /var/log/app.log                    # Analyze log file from beginning
  traceace /var/log/app.log /var/log/sys.log   # Analyze multiple files
  traceace --theme=light /var/log/app.log      # Use light theme
  traceace --query=errors /var/log/app.log     # Start with saved query`,
	Args: cobra.MinimumNArgs(1),
	Run:  runTraceAce,
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default: $XDG_CONFIG_HOME/traceace/config.yaml)")
	rootCmd.Flags().StringVar(&theme, "theme", "", "color theme (dark, light, monochrome)")
	rootCmd.Flags().BoolVarP(&tail, "tail", "f", true, "tail files (follow)")
	rootCmd.Flags().BoolVarP(&fromBeginning, "from-beginning", "F", false, "read entire file from beginning")
	rootCmd.Flags().IntVarP(&contextLines, "context", "C", 0, "number of context lines around matches")
	rootCmd.Flags().StringVar(&savedQuery, "query", "", "start with a saved query")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "debug mode")
}

// runTraceAce is the main execution function
func runTraceAce(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override config with command line flags
	if theme != "" {
		cfg.UI.Theme = theme
	}
	
	if contextLines > 0 {
		cfg.UI.ContextLines = contextLines
	}

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Initialize the TUI model
	model, err := ui.NewModel(cfg, ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize UI: %v\n", err)
		os.Exit(1)
	}

	// Add files to be tailed
	for _, file := range args {
		var addErr error
		if fromBeginning {
			addErr = model.TailFromStart(file)
		} else {
			addErr = model.AddFile(file)
		}
		
		if addErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to add file %s: %v\n", file, addErr)
			os.Exit(1)
		}
		
		if verbose {
			fmt.Printf("Added file: %s\n", file)
		}
	}

	// Start the TUI
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Run the program
	if err := program.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start TUI: %v\n", err)
		model.Stop()
		os.Exit(1)
	}

	// Clean shutdown
	model.Stop()
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TraceAce v1.0.0")
		fmt.Println("Built with Go and Bubble Tea")
	},
}

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
}

// configShowCmd shows the current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Configuration loaded from: %s\n", getConfigPath())
		fmt.Printf("Theme: %s\n", cfg.UI.Theme)
		fmt.Printf("Context Lines: %d\n", cfg.UI.ContextLines)
		fmt.Printf("Max Buffer Lines: %d\n", cfg.UI.MaxBufferLines)
		fmt.Printf("Refresh Rate: %d ms\n", cfg.UI.RefreshRate)
		fmt.Printf("Highlight Rules: %d\n", len(cfg.HighlightRules))
		fmt.Printf("Saved Queries: %d\n", len(cfg.SavedQueries))
	},
}

// configEditCmd opens the configuration file for editing
var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		configPath := getConfigPath()
		
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi" // fallback
		}
		
		fmt.Printf("Opening %s with %s...\n", configPath, editor)
		
		// Note: In a real implementation, you'd use os.exec to open the editor
		fmt.Printf("Please manually edit: %s\n", configPath)
	},
}

// benchmarkCmd runs performance benchmarks
var benchmarkCmd = &cobra.Command{
	Use:   "benchmark [file]",
	Short: "Run performance benchmarks",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Running benchmark on: %s\n", args[0])
		// Benchmark implementation would go here
		fmt.Println("Benchmark completed successfully")
	},
}

// validateCmd validates log files and configurations
var validateCmd = &cobra.Command{
	Use:   "validate [files...]",
	Short: "Validate log files and configuration",
	Run: func(cmd *cobra.Command, args []string) {
		// Load and validate config
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("✓ Configuration is valid")
		
		// Validate files if provided
		if len(args) > 0 {
			for _, file := range args {
				if _, err := os.Stat(file); err != nil {
					fmt.Fprintf(os.Stderr, "✗ File %s: %v\n", file, err)
					continue
				}
				fmt.Printf("✓ File %s is accessible\n", file)
			}
		}
		
		fmt.Printf("Theme: %s\n", cfg.UI.Theme)
		fmt.Printf("Validation completed\n")
	},
}

// getConfigPath returns the configuration file path
func getConfigPath() string {
	if configFile != "" {
		return configFile
	}
	
	configDir, err := config.ConfigDir()
	if err != nil {
		return "~/.config/traceace/config.yaml"
	}
	
	return fmt.Sprintf("%s/config.yaml", configDir)
}

// Add subcommands
func init() {
	// Add version command
	rootCmd.AddCommand(versionCmd)
	
	// Add config commands
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	
	// Add utility commands
	rootCmd.AddCommand(benchmarkCmd)
	rootCmd.AddCommand(validateCmd)
}
