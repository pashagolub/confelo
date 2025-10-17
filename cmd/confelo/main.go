// Package main provides the command-line interface for the confelo conference talk ranking application.
// It implements automatic mode detection based on session name existence, eliminating the need for subcommands.
// The application automatically detects whether to start a new session or resume an existing one.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"

	"github.com/pashagolub/confelo/pkg/data"
	"github.com/pashagolub/confelo/pkg/tui"
	"github.com/pashagolub/confelo/pkg/tui/screens"
)

// Version information - set by build process
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// ErrorCode represents CLI exit codes
type ErrorCode int

const (
	ExitSuccess ErrorCode = iota
	ExitFileError
	ExitConfigError
	ExitSessionError
	ExitExportError
	ExitValidationError
	ExitUsageError
	ExitImplementationError
)

// CLIError represents a CLI error with exit code
type CLIError struct {
	Code        ErrorCode
	Message     string
	Details     map[string]any
	Suggestions []string
}

func (e *CLIError) Error() string {
	return e.Message
}

// formatErrorJSON formats error as JSON for structured output
func formatErrorJSON(err *CLIError) string {
	errorObj := map[string]any{
		"error": map[string]any{
			"code":    err.Code,
			"message": err.Message,
		},
	}

	if err.Details != nil {
		errorObj["error"].(map[string]any)["details"] = err.Details
	}

	if err.Suggestions != nil {
		errorObj["error"].(map[string]any)["suggestions"] = err.Suggestions
	}

	jsonBytes, _ := json.MarshalIndent(errorObj, "", "  ")
	return string(jsonBytes)
}

func main() {
	if err := run(); err != nil {
		if cliErr, ok := err.(*CLIError); ok {
			fmt.Fprintln(os.Stderr, formatErrorJSON(cliErr))
			os.Exit(int(cliErr.Code))
		}
		log.Fatal(err)
	}
}

func run() error {
	// Use the standardized CLI parsing from data package
	options, err := data.ParseCLI(os.Args[1:])
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			switch flagsErr.Type {
			case flags.ErrHelp:
				return nil
			default:
				return &CLIError{
					Code:    ExitUsageError,
					Message: fmt.Sprintf("Invalid arguments: %v", err),
				}
			}
		}
		return &CLIError{
			Code:    ExitUsageError,
			Message: fmt.Sprintf("Failed to parse CLI arguments: %v", err),
		}
	}

	// Handle version flag first
	if options.Version {
		fmt.Printf("confelo %s (built %s, commit %s)\n", Version, BuildDate, GitCommit)
		return nil
	}

	return executeWithAutomaticModeDetection(options)
}

// executeWithAutomaticModeDetection handles the new automatic mode detection logic
func executeWithAutomaticModeDetection(options *data.CLIOptions) error {
	if options.Version {
		return showVersion()
	}

	// Create SessionDetector for the sessions directory
	sessionsDir := "sessions"
	detector := data.NewSessionDetector(sessionsDir)

	// Detect the mode based on session name
	mode, err := detector.DetectMode(options.SessionName)
	if err != nil {
		return handleModeDetectionError(err, options.SessionName)
	}

	if options.Verbose {
		fmt.Printf("Mode detected: %s for session '%s'\n", mode, options.SessionName)
	}

	// Convert SimplifiedCLIOptions to data.CLIOptions
	cliOptions := &data.CLIOptions{
		SessionName:    options.SessionName,
		Input:          options.Input,
		ComparisonMode: options.ComparisonMode,
		InitialRating:  options.InitialRating,
		OutputScale:    options.OutputScale,
		TargetAccepted: options.TargetAccepted,
	}

	// Handle mode-specific logic
	switch mode {
	case data.StartMode:
		return executeStartMode(cliOptions, options.Verbose)
	case data.ResumeMode:
		return executeResumeMode(cliOptions, options.Verbose)
	default:
		return &CLIError{
			Code:    ExitImplementationError,
			Message: fmt.Sprintf("Unknown session mode: %s", mode),
			Suggestions: []string{
				"This appears to be an internal error",
				"Please report this as a bug",
			},
		}
	}
}

// executeStartMode handles starting a new session
func executeStartMode(options *data.CLIOptions, verbose bool) error {
	// Validate that input file is provided for new sessions
	if options.Input == "" {
		return &CLIError{
			Code:    ExitValidationError,
			Message: "Input CSV file is required when starting a new session",
			Suggestions: []string{
				"Provide --input path/to/proposals.csv",
				"Use an existing CSV file with proposal data",
			},
		}
	}

	// Validate CSV file exists
	if _, err := os.Stat(options.Input); os.IsNotExist(err) {
		return &CLIError{
			Code:    ExitFileError,
			Message: fmt.Sprintf("Input file not found: %s", options.Input),
			Suggestions: []string{
				"Check file path and name",
				"Ensure file has .csv extension",
				"Use absolute path if needed",
			},
		}
	}

	// Create session configuration from CLI options
	config, err := data.CreateSessionConfigFromCLI(options)
	if err != nil {
		return &CLIError{
			Code:    ExitValidationError,
			Message: fmt.Sprintf("Failed to create session config: %v", err),
			Suggestions: []string{
				"Check that all CLI parameters are valid",
				"Verify the comparison mode and output scale format",
			},
		}
	}

	if verbose {
		fmt.Printf("Starting new session '%s' with input '%s'\n", options.SessionName, options.Input)
		fmt.Printf("Session config created: comparison=%s, rating=%.1f\n", config.UI.ComparisonMode, config.Elo.InitialRating)
	}

	// Create storage
	storage := &data.FileStorage{}

	// Load proposals from CSV with Elo conversion
	parseResult, err := storage.LoadProposalsFromCSVWithElo(options.Input, config.CSV, &config.Elo)
	if err != nil {
		return &CLIError{
			Code:    ExitFileError,
			Message: fmt.Sprintf("Failed to load CSV file: %v", err),
			Suggestions: []string{
				"Check CSV format and encoding",
				"Ensure required columns are present",
				"Verify delimiter and quote characters",
			},
		}
	}

	// Create new session with proper initialization
	session, err := data.NewSession(options.SessionName, parseResult.Proposals, *config, options.Input)
	if err != nil {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Failed to create session: %v", err),
			Suggestions: []string{
				"Check that proposals were loaded correctly",
				"Verify session configuration is valid",
			},
		}
	}

	// Save session using the session name as filename
	sessionFile := filepath.Join("sessions", data.SanitizeFilename(options.SessionName)+".json")
	if err := os.MkdirAll("sessions", 0755); err != nil {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Failed to create sessions directory: %v", err),
		}
	}

	if err := storage.SaveSession(session, sessionFile); err != nil {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Failed to save session: %v", err),
		}
	}

	if verbose {
		fmt.Printf("Created new session: %s (file: %s)\n", options.SessionName, filepath.Base(sessionFile))
		if len(parseResult.ParseErrors) > 0 {
			fmt.Println("Parse Issues:")
			for _, parseErr := range parseResult.ParseErrors {
				fmt.Printf("  - Row %d: %s\n", parseErr.RowNumber, parseErr.Message)
			}
		}
	}

	// Launch TUI in interactive mode
	return runInteractiveMode(session, config, storage)
}

// executeResumeMode handles resuming an existing session
func executeResumeMode(options *data.CLIOptions, verbose bool) error {
	if verbose {
		fmt.Printf("Resuming existing session '%s'\n", options.SessionName)
	}

	// Create storage
	storage := &data.FileStorage{}

	// Create session detector to find the session file
	sessionsDir := "sessions"
	detector := data.NewSessionDetector(sessionsDir)

	sessionFile, err := detector.FindSessionFile(options.SessionName)
	if err != nil {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Failed to find session file: %v", err),
			Suggestions: []string{
				"Check that the session name is correct",
				"Ensure the sessions directory is accessible",
			},
		}
	}

	if sessionFile == "" {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Session '%s' not found", options.SessionName),
			Suggestions: []string{
				"Use a different session name to start a new session",
				"Check available sessions in the sessions/ directory",
			},
		}
	}

	// Load existing session
	session, err := storage.LoadSession(sessionFile)
	if err != nil {
		return handleSessionLoadError(err, options.SessionName, sessionFile)
	}

	// Create session configuration from CLI options (allows overriding settings)
	config, err := data.CreateSessionConfigFromCLI(options)
	if err != nil {
		return &CLIError{
			Code:    ExitValidationError,
			Message: fmt.Sprintf("Failed to create session config: %v", err),
			Suggestions: []string{
				"Check that all CLI parameters are valid",
				"Verify the comparison mode and output scale format",
			},
		}
	}

	if verbose {
		fmt.Printf("Loaded session: %s (%s)\n", session.Name, session.ID)
		fmt.Printf("Session config: comparison=%s, rating=%.1f\n", config.UI.ComparisonMode, config.Elo.InitialRating)
	}

	// Launch TUI in interactive mode
	return runInteractiveMode(session, config, storage)
}

// Helper functions

func showVersion() error {
	fmt.Printf("confelo version %s\n", Version)
	fmt.Printf("Build date: %s\n", BuildDate)
	fmt.Printf("Git commit: %s\n", GitCommit)
	return nil
}

func runInteractiveMode(session *data.Session, config *data.SessionConfig, storage data.Storage) error {
	// Import TUI components - we need to add these imports at the top
	tuiApp, err := createTUIApp(session, config, storage)
	if err != nil {
		return fmt.Errorf("failed to create TUI application: %w", err)
	}

	// Start the TUI application
	runErr := tuiApp.Run()
	
	// Save session on exit (regardless of error)
	// This ensures progress is saved even if app exits unexpectedly
	sessionFile := filepath.Join("sessions", data.SanitizeFilename(session.Name)+".json")
	if saveErr := storage.SaveSession(session, sessionFile); saveErr != nil {
		// Log save error but don't override run error
		fmt.Fprintf(os.Stderr, "Warning: Failed to save session on exit: %v\n", saveErr)
	}
	
	return runErr
}

// createTUIApp creates and configures the TUI application with all screens
func createTUIApp(session *data.Session, config *data.SessionConfig, storage data.Storage) (*tui.App, error) {
	// Create the main TUI application
	app, err := tui.NewApp(config, storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUI app: %w", err)
	}

	// Set the current session
	app.SetSession(session)

	// Create and register screens
	comparisonScreen := screens.NewComparisonScreen()
	rankingScreen := screens.NewRankingScreen()

	// Register screens with the app
	if err := app.RegisterScreen(tui.ScreenComparison, comparisonScreen); err != nil {
		return nil, fmt.Errorf("failed to register comparison screen: %w", err)
	}
	if err := app.RegisterScreen(tui.ScreenRanking, rankingScreen); err != nil {
		return nil, fmt.Errorf("failed to register ranking screen: %w", err)
	}

	return app, nil
}

// handleModeDetectionError provides specific error handling for different mode detection failure types
func handleModeDetectionError(err error, sessionName string) *CLIError {
	// Check for specific error types using errors.Is
	if errors.Is(err, data.ErrSessionNameInvalid) {
		return &CLIError{
			Code:    ExitUsageError,
			Message: fmt.Sprintf("Invalid session name '%s': %v", sessionName, err),
			Suggestions: []string{
				"Use alphanumeric characters, hyphens, and underscores only",
				"Avoid filesystem reserved names (CON, PRN, AUX, etc.)",
				"Choose a different session name",
			},
		}
	}

	if errors.Is(err, data.ErrSessionCorrupted) {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Session '%s' file is corrupted: %v", sessionName, err),
			Details: map[string]any{
				"session_name": sessionName,
				"action":       "delete_corrupted_session",
			},
			Suggestions: []string{
				fmt.Sprintf("Delete the corrupted session files in sessions/ directory matching '*%s*'", sessionName),
				"Start a new session with the same name",
				"Use a different session name to avoid conflicts",
			},
		}
	}

	if errors.Is(err, data.ErrModeDetectionFailed) {
		return &CLIError{
			Code:    ExitValidationError,
			Message: fmt.Sprintf("Cannot determine session mode: %v", err),
			Details: map[string]any{
				"session_name": sessionName,
				"sessions_dir": "sessions",
			},
			Suggestions: []string{
				"Check that the sessions directory is accessible",
				"Ensure you have read/write permissions for the sessions directory",
				"Try running the command with administrator privileges if needed",
			},
		}
	}

	// Fallback for unexpected errors
	return &CLIError{
		Code:    ExitValidationError,
		Message: fmt.Sprintf("Failed to detect session mode: %v", err),
		Suggestions: []string{
			"Check that the session name is valid",
			"Ensure the sessions directory is accessible",
			"Use a different session name if the existing session is corrupted",
		},
	}
}

// handleSessionLoadError provides specific error handling for session loading failures
func handleSessionLoadError(err error, sessionName, sessionFile string) *CLIError {
	// Check for specific error types
	if errors.Is(err, data.ErrSessionCorrupted) {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Session '%s' is corrupted and cannot be loaded", sessionName),
			Details: map[string]any{
				"session_name": sessionName,
				"session_file": sessionFile,
				"error_type":   "corruption",
			},
			Suggestions: []string{
				fmt.Sprintf("Delete the corrupted session file: %s", sessionFile),
				"Start a new session with the same name",
				"Restore from backup if available",
			},
		}
	}

	// Check for file access issues
	if os.IsPermission(err) {
		return &CLIError{
			Code:    ExitFileError,
			Message: fmt.Sprintf("Permission denied accessing session '%s'", sessionName),
			Details: map[string]any{
				"session_name": sessionName,
				"session_file": sessionFile,
			},
			Suggestions: []string{
				"Check file permissions for the sessions directory",
				"Run with administrator privileges if needed",
				"Ensure the session file is not locked by another process",
			},
		}
	}

	// Fallback for other errors
	return &CLIError{
		Code:    ExitSessionError,
		Message: fmt.Sprintf("Failed to load session '%s': %v", sessionName, err),
		Details: map[string]any{
			"session_name": sessionName,
			"session_file": sessionFile,
		},
		Suggestions: []string{
			"The session file may be corrupted or inaccessible",
			"Try starting a new session with a different name",
			"Check if the sessions directory exists and is readable",
		},
	}
}
