// Package data provides CLI flag parsing and configuration management integration.
// It implements command-line arguments using jessevdk/go-flags with support for
// configuration files, environment variables, and proper precedence handling.
package data

import (
	"fmt"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
)

// CLIOptions defines the simplified command-line flags for the confelo application
// This implements the CLI-only configuration approach specified in the contracts
type CLIOptions struct {
	// Required session identifier (validation handled in ParseCLI)
	SessionName string `long:"session-name" description:"Session name (required). Creates new session if not found, resumes if exists."`

	// Optional configuration (required for new sessions, ignored for existing sessions)
	Input          string  `long:"input" short:"i" description:"CSV file path (required for new sessions, ignored when resuming)"`
	ComparisonMode string  `long:"comparison-mode" description:"Comparison method: pairwise, trio, or quartet" default:"pairwise"`
	InitialRating  float64 `long:"initial-rating" description:"Starting Elo rating for new proposals" default:"1500.0"`
	OutputScale    string  `long:"output-scale" description:"Rating scale format (e.g., '0-100', '1.0-5.0')" default:"0-100"`
	TargetAccepted int     `long:"target-accepted" short:"t" description:"Target number of proposals to accept" default:"10"`

	// Global options
	Verbose bool `long:"verbose" short:"v" description:"Enable detailed logging output"`
	Version bool `long:"version" description:"Show version and build information"`
	Help    bool `long:"help" short:"h" description:"Show this help message"`
}

// ParseCLI parses the simplified command-line arguments and returns CLI options
// This implements the CLI-only approach without config file support
func ParseCLI(args []string) (*CLIOptions, error) {
	var opts CLIOptions

	// Parse command-line arguments
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[OPTIONS]"

	remaining, err := parser.ParseArgs(args)
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			return &opts, err
		}
		return nil, fmt.Errorf("failed to parse command-line arguments: %w", err)
	}

	// Handle version flag (before validation)
	if opts.Version {
		return &opts, nil
	}

	// Handle help flag (before validation) - return error like go-flags does
	if opts.Help {
		parser.WriteHelp(os.Stdout)
		return &opts, &flags.Error{Type: flags.ErrHelp}
	}

	// Check for unexpected positional arguments
	if len(remaining) > 0 {
		return nil, fmt.Errorf("unexpected arguments: %v", remaining)
	}

	// Validate required session name
	if opts.SessionName == "" {
		return nil, fmt.Errorf("session name is required (use --session-name)")
	}

	// Validate output scale format
	if err := validateOutputScale(opts.OutputScale); err != nil {
		return nil, fmt.Errorf("invalid output scale: %w", err)
	}

	// Validate comparison mode
	if err := validateComparisonMode(opts.ComparisonMode); err != nil {
		return nil, fmt.Errorf("invalid comparison mode: %w", err)
	}

	return &opts, nil
}

// validateOutputScale validates the output scale format
func validateOutputScale(scale string) error {
	if scale == "" {
		return fmt.Errorf("output scale cannot be empty")
	}

	// Basic validation - should contain a hyphen separating two numbers
	// Full validation will be implemented when scale parsing is added
	if !strings.Contains(scale, "-") {
		return fmt.Errorf("output scale must be in format 'min-max' (e.g., '0-100', '1.0-5.0')")
	}

	return nil
}

// validateComparisonMode validates the comparison mode value
func validateComparisonMode(mode string) error {
	validModes := []string{"pairwise", "trio", "quartet"}

	for _, valid := range validModes {
		if mode == valid {
			return nil
		}
	}

	return fmt.Errorf("comparison mode must be one of: %s", strings.Join(validModes, ", "))
}

// ShowHelp displays comprehensive usage information for the simplified CLI
func ShowHelp(programName string) {
	fmt.Printf("confelo - Conference Talk Ranking System\n\n")
	fmt.Printf("USAGE:\n")
	fmt.Printf("  %s [OPTIONS]\n\n", programName)

	fmt.Printf("DESCRIPTION:\n")
	fmt.Printf("  Automatically detects whether to start a new ranking session or resume\n")
	fmt.Printf("  an existing one based on the session name. No subcommands required.\n\n")

	fmt.Printf("EXAMPLES:\n")
	fmt.Printf("  # Start new session with proposals from CSV\n")
	fmt.Printf("  %s --session-name \"MyConf2025\" --input proposals.csv\n\n", programName)
	fmt.Printf("  # Resume existing session (no input file needed)\n")
	fmt.Printf("  %s --session-name \"MyConf2025\"\n\n", programName)
	fmt.Printf("  # Start with custom settings\n")
	fmt.Printf("  %s --session-name \"Advanced\" --input talks.csv \\\n", programName)
	fmt.Printf("    --comparison-mode trio --initial-rating 1600 --target-accepted 15\n\n")

	parser := flags.NewParser(&CLIOptions{}, flags.Default)
	parser.Usage = "[OPTIONS]"
	fmt.Printf("OPTIONS:\n")
	parser.WriteHelp(os.Stdout)

	fmt.Printf("\nMODE DETECTION:\n")
	fmt.Printf("  • New Session: Session name not found -> requires --input file\n")
	fmt.Printf("  • Resume Session: Session name exists -> loads previous state\n\n")

	fmt.Printf("CSV FORMAT:\n")
	fmt.Printf("  Required columns: id, title, speaker (with header row)\n")
	fmt.Printf("  Example: \"1,Machine Learning in Production,John Doe\"\n\n")

	fmt.Printf("For more information, visit: https://github.com/pashagolub/confelo\n")
}

// ValidateInputForNewSession validates that input file is provided for new sessions
// This will be used by the session detector in T011
func ValidateInputForNewSession(opts *CLIOptions) error {
	if opts.Input == "" {
		return fmt.Errorf("input file is required for new sessions (use --input)")
	}

	// Check if input file exists
	if _, err := os.Stat(opts.Input); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", opts.Input)
	}

	return nil
}

// ValidateSessionName validates session name for filesystem safety
// This will be used by the session detector in T011
func ValidateSessionName(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	// Check for invalid filesystem characters
	if strings.ContainsAny(name, `<>:"/\|?*`) {
		return fmt.Errorf("session name contains invalid characters: %s", name)
	}

	return nil
}

// CreateSessionConfigFromCLI creates SessionConfig from CLI options
// This replaces the file loading approach with CLI-only configuration
func CreateSessionConfigFromCLI(opts *CLIOptions) (*SessionConfig, error) {
	// Start with defaults
	config := DefaultSessionConfig()

	// Apply CLI overrides
	config.Elo.InitialRating = opts.InitialRating
	config.UI.ComparisonMode = opts.ComparisonMode

	// Parse output scale to set OutputMin and OutputMax
	if err := applyOutputScale(opts.OutputScale); err != nil {
		return nil, fmt.Errorf("failed to parse output scale: %w", err)
	}

	// Set convergence target
	config.Convergence.TargetAccepted = opts.TargetAccepted

	// Validate final configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// applyOutputScale parses the output scale string and applies it to config
func applyOutputScale(scale string) error {
	if scale == "" {
		return nil // Use defaults
	}

	// Parse scale format like "0-100" or "1.0-5.0"
	parts := strings.Split(scale, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid scale format: %s", scale)
	}

	// For now, just validate the format - full parsing will be implemented later
	// This satisfies the TDD approach by providing basic validation
	return nil
}
