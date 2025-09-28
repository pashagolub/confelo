// Package data provides CLI flag parsing and configuration management integration.
// It implements command-line arguments using jessevdk/go-flags with support for
// configuration files, environment variables, and proper precedence handling.
package data

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jessevdk/go-flags"
)

// CLIOptions defines command-line flags for the confelo application
type CLIOptions struct {
	// Configuration file options
	ConfigFile string `long:"config" short:"c" description:"Configuration file path" default:"confelo.yaml"`
	NoConfig   bool   `long:"no-config" description:"Skip loading configuration file"`

	// CSV parsing options
	CSVFile        string `long:"csv" description:"Input CSV file path"`
	IDColumn       string `long:"id-column" description:"CSV column name for proposal ID"`
	TitleColumn    string `long:"title-column" description:"CSV column name for proposal title"`
	AbstractColumn string `long:"abstract-column" description:"CSV column name for abstract"`
	SpeakerColumn  string `long:"speaker-column" description:"CSV column name for speaker"`
	ScoreColumn    string `long:"score-column" description:"CSV column name for existing score"`
	CommentColumn  string `long:"comment-column" description:"CSV column name for reviewer comments"`
	ConflictColumn string `long:"conflict-column" description:"CSV column name for conflict tags"`
	NoHeader       bool   `long:"no-header" description:"CSV file has no header row"`
	Delimiter      string `long:"delimiter" description:"CSV field separator" default:","`

	// Elo engine options
	InitialRating float64 `long:"initial-rating" description:"Starting rating for new proposals" default:"1500"`
	KFactor       int     `long:"k-factor" description:"Rating change sensitivity" default:"32"`
	MinRating     float64 `long:"min-rating" description:"Minimum allowed rating" default:"0"`
	MaxRating     float64 `long:"max-rating" description:"Maximum allowed rating" default:"3000"`
	OutputMin     float64 `long:"output-min" description:"Minimum output scale value"`
	OutputMax     float64 `long:"output-max" description:"Maximum output scale value" default:"-1"`
	UseDecimals   bool    `long:"use-decimals" description:"Use decimal places in output"`

	// UI preferences
	ComparisonMode   string        `long:"mode" short:"m" description:"Comparison mode (pairwise/trio/quartet)" default:"pairwise"`
	NoProgress       bool          `long:"no-progress" description:"Hide progress indicators"`
	NoConfidence     bool          `long:"no-confidence" description:"Hide rating confidence"`
	NoAutoSave       bool          `long:"no-auto-save" description:"Disable automatic session saving"`
	AutoSaveInterval time.Duration `long:"auto-save-interval" description:"Auto-save frequency" default:"5m"`

	// Export options
	OutputFile    string `long:"output" short:"o" description:"Output file path"`
	Format        string `long:"format" description:"Output format (csv/json/yaml)" default:"csv"`
	NoMetadata    bool   `long:"no-metadata" description:"Exclude original CSV metadata"`
	SortBy        string `long:"sort-by" description:"Sort criterion (rating/title/speaker/id)" default:"rating"`
	SortOrder     string `long:"sort-order" description:"Sort direction (asc/desc)" default:"desc"`
	NoScaling     bool   `long:"no-scaling" description:"Skip output scaling"`
	RoundDecimals int    `long:"round-decimals" description:"Decimal places for output" default:"2"`

	// Global options
	Verbose bool `long:"verbose" short:"v" description:"Enable verbose output"`
	Version bool `long:"version" description:"Show version information"`
	Help    bool `long:"help" short:"h" description:"Show this help message"`
}

// ParseCLI parses command-line arguments and returns combined configuration
func ParseCLI(args []string) (*SessionConfig, *CLIOptions, error) {
	var opts CLIOptions

	// Parse command-line arguments
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[OPTIONS] --csv input.csv"

	remaining, err := parser.ParseArgs(args)
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			return nil, &opts, err
		}
		return nil, nil, fmt.Errorf("failed to parse command-line arguments: %w", err)
	}

	// Handle version flag (before validation)
	if opts.Version {
		return nil, &opts, nil
	}

	// Handle help flag (before validation) - return error like go-flags does
	if opts.Help {
		parser.WriteHelp(os.Stdout)
		return nil, &opts, &flags.Error{Type: flags.ErrHelp}
	}

	// Check for unexpected positional arguments
	if len(remaining) > 0 {
		return nil, nil, fmt.Errorf("unexpected arguments: %v", remaining)
	}

	// Validate required CSV file (after version/help handling)
	if opts.CSVFile == "" {
		return nil, nil, fmt.Errorf("CSV file path is required (use --csv)")
	}

	// Load base configuration
	var config *SessionConfig
	if !opts.NoConfig {
		// Try to load from specified or default config file
		configPath := opts.ConfigFile
		if !filepath.IsAbs(configPath) {
			// Look for config in current directory or user config dir
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				homeDir, _ := os.UserHomeDir()
				altPath := filepath.Join(homeDir, ".config", "confelo", configPath)
				if _, err := os.Stat(altPath); err == nil {
					configPath = altPath
				}
			}
		}

		loadedConfig, err := LoadWithEnvironment(configPath)
		if err != nil {
			// Only fail if user explicitly specified a config file AND it's not a "not found" error
			if opts.ConfigFile != "confelo.yaml" && !errors.Is(err, ErrConfigNotFound) {
				return nil, nil, fmt.Errorf("failed to load configuration file: %w", err)
			}
			// Use defaults if default config file doesn't exist or explicit file not found
			defaultConfig := DefaultSessionConfig()
			config = &defaultConfig
		} else {
			config = loadedConfig
		}
	} else {
		// Use defaults when config is disabled, but still apply environment overrides
		defaultConfig := DefaultSessionConfig()
		config = &defaultConfig
		applyEnvironmentOverrides(config)
	}

	// Apply CLI flag overrides (highest precedence)
	applyCLIOverrides(config, &opts)

	// Validate final configuration
	if err := config.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, &opts, nil
}

// applyCLIOverrides applies command-line flag values to the configuration
func applyCLIOverrides(config *SessionConfig, opts *CLIOptions) {
	// Get default configurations to compare against
	csvDefaults := DefaultCSVConfig()
	eloDefaults := DefaultEloConfig()
	uiDefaults := DefaultUIConfig()
	exportDefaults := DefaultExportConfig()
	// CSV configuration overrides
	if opts.IDColumn != "" {
		config.CSV.IDColumn = opts.IDColumn
	}
	if opts.TitleColumn != "" {
		config.CSV.TitleColumn = opts.TitleColumn
	}
	if opts.AbstractColumn != "" {
		config.CSV.AbstractColumn = opts.AbstractColumn
	}
	if opts.SpeakerColumn != "" {
		config.CSV.SpeakerColumn = opts.SpeakerColumn
	}
	if opts.ScoreColumn != "" {
		config.CSV.ScoreColumn = opts.ScoreColumn
	}
	if opts.CommentColumn != "" {
		config.CSV.CommentColumn = opts.CommentColumn
	}
	if opts.ConflictColumn != "" {
		config.CSV.ConflictColumn = opts.ConflictColumn
	}
	if opts.NoHeader {
		config.CSV.HasHeader = false
	}
	if opts.Delimiter != csvDefaults.Delimiter {
		config.CSV.Delimiter = opts.Delimiter
	}

	// Elo configuration overrides - check if CLI provided explicit values
	if opts.InitialRating != eloDefaults.InitialRating {
		config.Elo.InitialRating = opts.InitialRating
	}
	if opts.KFactor != eloDefaults.KFactor {
		config.Elo.KFactor = opts.KFactor
	}
	if opts.MinRating != eloDefaults.MinRating {
		config.Elo.MinRating = opts.MinRating
	}
	if opts.MaxRating != eloDefaults.MaxRating {
		config.Elo.MaxRating = opts.MaxRating
	}
	if opts.OutputMin != eloDefaults.OutputMin {
		config.Elo.OutputMin = opts.OutputMin
	}
	if opts.OutputMax >= 0 { // Use sentinel value check for OutputMax only
		config.Elo.OutputMax = opts.OutputMax
	}
	if opts.UseDecimals {
		config.Elo.UseDecimals = true
	}

	// UI configuration overrides
	if opts.ComparisonMode != uiDefaults.ComparisonMode {
		config.UI.ComparisonMode = opts.ComparisonMode
	}
	if opts.NoProgress {
		config.UI.ShowProgress = false
	}
	if opts.NoConfidence {
		config.UI.ShowConfidence = false
	}
	if opts.NoAutoSave {
		config.UI.AutoSave = false
	}
	if opts.AutoSaveInterval != uiDefaults.AutoSaveInterval {
		config.UI.AutoSaveInterval = opts.AutoSaveInterval
	}

	// Export configuration overrides
	if opts.Format != exportDefaults.Format {
		config.Export.Format = opts.Format
	}
	if opts.NoMetadata {
		config.Export.IncludeMetadata = false
	}
	if opts.SortBy != exportDefaults.SortBy {
		config.Export.SortBy = opts.SortBy
	}
	if opts.SortOrder != exportDefaults.SortOrder {
		config.Export.SortOrder = opts.SortOrder
	}
	if opts.NoScaling {
		config.Export.ScaleOutput = false
	}
	if opts.RoundDecimals != exportDefaults.RoundDecimals {
		config.Export.RoundDecimals = opts.RoundDecimals
	}
}

// ShowHelp displays usage information
func ShowHelp(programName string) {
	parser := flags.NewParser(&CLIOptions{}, flags.Default)
	parser.Usage = "[OPTIONS] --csv input.csv"
	parser.WriteHelp(os.Stdout)
}

// ValidateCLIOptions validates command-line specific options
func ValidateCLIOptions(opts *CLIOptions) error {
	// Validate required CSV file
	if opts.CSVFile == "" {
		return fmt.Errorf("CSV file path is required (use --csv)")
	}

	// Check if CSV file exists
	if _, err := os.Stat(opts.CSVFile); os.IsNotExist(err) {
		return fmt.Errorf("CSV file not found: %s", opts.CSVFile)
	}

	// Validate output file path if specified
	if opts.OutputFile != "" {
		// Check if output directory exists
		outputDir := filepath.Dir(opts.OutputFile)
		if outputDir != "." {
			if _, err := os.Stat(outputDir); os.IsNotExist(err) {
				return fmt.Errorf("output directory does not exist: %s", outputDir)
			}
		}

		// Check if we can write to the output file
		if err := checkWritable(opts.OutputFile); err != nil {
			return fmt.Errorf("cannot write to output file %s: %w", opts.OutputFile, err)
		}
	}

	return nil
}

// checkWritable checks if we can write to the specified file path
func checkWritable(filePath string) error {
	// Try to create/open the file for writing
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	// Write a test byte and remove the file
	_, writeErr := file.Write([]byte{})
	closeErr := file.Close()
	removeErr := os.Remove(filePath)

	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if removeErr != nil {
		return removeErr
	}

	return nil
}

// GetConfigSearchPaths returns possible configuration file locations
func GetConfigSearchPaths(filename string) []string {
	paths := []string{}

	// Current directory
	paths = append(paths, filename)

	// User config directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(homeDir, ".config", "confelo", filename))
		paths = append(paths, filepath.Join(homeDir, ".confelo", filename))
	}

	// System config directory (Unix-like systems)
	paths = append(paths, filepath.Join("/etc", "confelo", filename))

	return paths
}

// CreateDefaultConfig creates a default configuration file at the specified path
func CreateDefaultConfig(filePath string) error {
	config := DefaultSessionConfig()

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save default configuration
	if err := config.SaveToFile(filePath); err != nil {
		return fmt.Errorf("failed to create default config: %w", err)
	}

	return nil
}
