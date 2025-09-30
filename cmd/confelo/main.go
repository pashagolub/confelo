// Package main provides the command-line interface for the confelo conference talk ranking application.
// It implements subcommands for starting sessions, resuming work, exporting results, and validation
// following the CLI interface contract with support for both interactive TUI and batch modes.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"

	"github.com/pashagolub/confelo/pkg/data"
)

// Version information - set by build process
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// Commands are handled by go-flags directly through struct tag annotations

// GlobalOptions defines global CLI flags
type GlobalOptions struct {
	Config  string `long:"config" short:"c" description:"Configuration file path" default:"confelo.yaml"`
	Verbose bool   `long:"verbose" short:"v" description:"Enable verbose logging"`
	Version bool   `long:"version" description:"Show version information"`
	Help    bool   `long:"help" short:"h" description:"Show help message"`
}

// StartCommand handles 'confelo start' subcommand
type StartCommand struct {
	Input          string  `long:"input" short:"i" description:"Path to CSV file containing proposals" required:"true"`
	SessionName    string  `long:"session-name" description:"Name for the ranking session"`
	ComparisonMode string  `long:"comparison-mode" description:"Comparison type (pairwise/trio/quartet)" default:"pairwise"`
	InitialRating  float64 `long:"initial-rating" description:"Starting Elo rating" default:"1500.0"`
	OutputScale    string  `long:"output-scale" description:"Output scale format (e.g., '0-10', '1-5')" default:"0-100"`
	Batch          bool    `long:"batch" description:"Run in batch mode (non-interactive)"`

	Global *GlobalOptions
}

// ResumeCommand handles 'confelo resume' subcommand
type ResumeCommand struct {
	SessionID      string `long:"session-id" description:"Identifier of existing session to resume" required:"true"`
	ComparisonMode string `long:"comparison-mode" description:"Override comparison mode"`
	Batch          bool   `long:"batch" description:"Run in batch mode (non-interactive)"`

	Global *GlobalOptions
}

// ExportCommand handles 'confelo export' subcommand
type ExportCommand struct {
	SessionID    string `long:"session-id" description:"Session to export results from" required:"true"`
	Output       string `long:"output" short:"o" description:"Output file path"`
	Format       string `long:"format" description:"Export format (csv/json/text)" default:"csv"`
	IncludeStats bool   `long:"include-stats" description:"Include rating statistics"`
	IncludeAudit bool   `long:"include-audit" description:"Include comparison history"`

	Global *GlobalOptions
}

// ListCommand handles 'confelo list' subcommand
type ListCommand struct {
	Format string `long:"format" description:"Output format (table/json/csv)" default:"table"`
	Status string `long:"status" description:"Filter by status (active/complete/all)" default:"all"`

	Global *GlobalOptions
}

// ValidateCommand handles 'confelo validate' subcommand
type ValidateCommand struct {
	Input   string `long:"input" short:"i" description:"Path to CSV file to validate" required:"true"`
	Config  string `long:"config" description:"Configuration file for column mapping"`
	Preview int    `long:"preview" description:"Number of rows to preview" default:"5"`

	// Global options embedded
	ConfigFile string `long:"config-file" short:"c" description:"Configuration file path" default:"confelo.yaml"`
	Verbose    bool   `long:"verbose" short:"v" description:"Enable verbose logging"`
	Version    bool   `long:"version" description:"Show version information"`
}

// ErrorCode represents CLI exit codes
type ErrorCode int

const (
	ExitSuccess ErrorCode = iota
	ExitFileError
	ExitConfigError
	ExitSessionError
	ExitExportError
	ExitValidationError
)

// CLIError represents a CLI error with exit code
type CLIError struct {
	Code        ErrorCode
	Message     string
	Details     map[string]interface{}
	Suggestions []string
}

func (e *CLIError) Error() string {
	return e.Message
}

// formatErrorJSON formats error as JSON for structured output
func formatErrorJSON(err *CLIError) string {
	errorObj := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
	}

	if err.Details != nil {
		errorObj["error"].(map[string]interface{})["details"] = err.Details
	}

	if err.Suggestions != nil {
		errorObj["error"].(map[string]interface{})["suggestions"] = err.Suggestions
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
	parser := flags.NewParser(nil, flags.Default)
	parser.Usage = "[OPTIONS] COMMAND [COMMAND-OPTIONS]"

	// Add subcommands
	startCmd := &StartCommand{}
	resumeCmd := &ResumeCommand{}
	exportCmd := &ExportCommand{}
	listCmd := &ListCommand{}
	validateCmd := &ValidateCommand{}

	parser.AddCommand("start", "Start a new ranking session", "", startCmd)
	parser.AddCommand("resume", "Resume an existing session", "", resumeCmd)
	parser.AddCommand("export", "Export ranking results", "", exportCmd)
	parser.AddCommand("list", "List available sessions", "", listCmd)
	parser.AddCommand("validate", "Validate CSV input file", "", validateCmd)

	// Parse command line
	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			switch flagsErr.Type {
			case flags.ErrHelp:
				return nil
			case flags.ErrCommandRequired:
				fmt.Fprintln(os.Stderr, "Error: No command specified")
				parser.WriteHelp(os.Stderr)
				return &CLIError{
					Code:    ExitConfigError,
					Message: "No command specified",
					Suggestions: []string{
						"Use 'confelo start --input file.csv' to begin ranking",
						"Use 'confelo --help' to see all available commands",
					},
				}
			default:
				return &CLIError{
					Code:    ExitConfigError,
					Message: fmt.Sprintf("Invalid arguments: %v", err),
				}
			}
		}
		return err
	}

	return nil
}

// Execute implements the Command interface for StartCommand
func (c *StartCommand) Execute(args []string) error {
	if c.Global != nil && c.Global.Version {
		return showVersion()
	}

	// Load configuration
	config, err := loadConfiguration(c.Global.Config, c.Global.Verbose)
	if err != nil {
		return &CLIError{
			Code:    ExitConfigError,
			Message: fmt.Sprintf("Failed to load configuration: %v", err),
			Suggestions: []string{
				"Check configuration file syntax",
				"Use --config flag to specify different config file",
				"Run with --verbose for more details",
			},
		}
	}

	// Override configuration with command-line flags
	applyStartConfigOverrides(config, c)

	// Validate CSV file exists
	if _, err := os.Stat(c.Input); os.IsNotExist(err) {
		return &CLIError{
			Code:    ExitFileError,
			Message: fmt.Sprintf("Input file not found: %s", c.Input),
			Details: map[string]interface{}{
				"file": c.Input,
			},
			Suggestions: []string{
				"Check file path and name",
				"Ensure file has .csv extension",
				"Use absolute path if needed",
			},
		}
	}

	// Create storage
	storage := &data.FileStorage{}

	// Load proposals from CSV
	parseResult, err := storage.LoadProposalsFromCSV(c.Input, config.CSV)
	if err != nil {
		return &CLIError{
			Code:    ExitFileError,
			Message: fmt.Sprintf("Failed to load CSV file: %v", err),
			Details: map[string]interface{}{
				"file": c.Input,
			},
			Suggestions: []string{
				"Validate CSV format with 'confelo validate --input " + c.Input + "'",
				"Check for missing required columns",
				"Ensure file encoding is UTF-8",
			},
		}
	}

	// Create new session
	sessionID := generateSessionID()
	if c.SessionName == "" {
		c.SessionName = fmt.Sprintf("Session %s", time.Now().Format("2006-01-02 15:04"))
	}

	session := &data.Session{
		ID:        sessionID,
		Name:      c.SessionName,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Proposals: parseResult.Proposals,
	}

	// Save session
	sessionFile := filepath.Join("sessions", sessionID+".json")
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

	fmt.Printf("Created new session: %s\n", sessionID)
	if len(parseResult.ParseErrors) > 0 && c.Global.Verbose {
		fmt.Println("Parse Issues:")
		for _, parseErr := range parseResult.ParseErrors {
			fmt.Printf("  - Row %d: %s\n", parseErr.RowNumber, parseErr.Message)
		}
	}

	// Start interactive or batch mode
	if c.Batch {
		return runBatchMode(session, config, storage)
	} else {
		return runInteractiveMode(session, config, storage)
	}
}

// Execute implements the Command interface for ResumeCommand
func (c *ResumeCommand) Execute(args []string) error {
	if c.Global != nil && c.Global.Version {
		return showVersion()
	}

	// Load configuration
	config, err := loadConfiguration(c.Global.Config, c.Global.Verbose)
	if err != nil {
		return &CLIError{
			Code:    ExitConfigError,
			Message: fmt.Sprintf("Failed to load configuration: %v", err),
		}
	}

	// Override comparison mode if specified
	if c.ComparisonMode != "" {
		config.UI.ComparisonMode = c.ComparisonMode
	}

	// Create storage
	storage := &data.FileStorage{}

	// Load existing session
	sessionFile := filepath.Join("sessions", c.SessionID+".json")
	session, err := storage.LoadSession(sessionFile)
	if err != nil {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Failed to load session '%s': %v", c.SessionID, err),
			Details: map[string]interface{}{
				"session_id": c.SessionID,
			},
			Suggestions: []string{
				"Use 'confelo list' to see available sessions",
				"Check session ID spelling",
				"Ensure session file exists in sessions/ directory",
			},
		}
	}

	fmt.Printf("Resuming session: %s (%s)\n", session.Name, session.ID)

	// Start interactive or batch mode
	if c.Batch {
		return runBatchMode(session, config, storage)
	} else {
		return runInteractiveMode(session, config, storage)
	}
}

// Execute implements the Command interface for ExportCommand
func (c *ExportCommand) Execute(args []string) error {
	if c.Global != nil && c.Global.Version {
		return showVersion()
	}

	// Create storage
	storage := &data.FileStorage{}

	// Load configuration for export
	config, configErr := loadConfiguration(c.Global.Config, c.Global.Verbose)
	if configErr != nil {
		// Use defaults if config loading fails
		defaultConfig := data.DefaultSessionConfig()
		config = &defaultConfig
	}

	// Load session
	sessionFile := filepath.Join("sessions", c.SessionID+".json")
	session, err := storage.LoadSession(sessionFile)
	if err != nil {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Session not found: %s", c.SessionID),
			Details: map[string]interface{}{
				"session_id": c.SessionID,
			},
			Suggestions: []string{
				"Use 'confelo list' to see available sessions",
				"Check session ID spelling",
			},
		}
	}

	// Generate output filename if not specified
	outputFile := c.Output
	if outputFile == "" {
		ext := "csv"
		if c.Format == "json" {
			ext = "json"
		} else if c.Format == "text" {
			ext = "txt"
		}
		outputFile = fmt.Sprintf("rankings_%s.%s", c.SessionID, ext)
	}

	// Create export configuration
	exportConfig := data.ExportConfig{
		Format:          c.Format,
		IncludeMetadata: true,
		SortBy:          "rating",
		SortOrder:       "desc",
		ScaleOutput:     true,
		RoundDecimals:   2,
	}

	// Export based on format
	switch c.Format {
	case "json":
		// For JSON export, create a structured export
		exportData := map[string]interface{}{
			"session_id":   session.ID,
			"session_name": session.Name,
			"exported_at":  time.Now(),
			"proposals":    session.Proposals,
			"statistics": map[string]interface{}{
				"total_proposals":       len(session.Proposals),
				"completed_comparisons": len(session.CompletedComparisons),
			},
		}

		if c.IncludeAudit {
			exportData["comparisons"] = session.CompletedComparisons
		}

		jsonData, jsonErr := json.MarshalIndent(exportData, "", "  ")
		if jsonErr != nil {
			err = jsonErr
		} else {
			err = os.WriteFile(outputFile, jsonData, 0644)
		}
	default:
		// Use CSV export from storage
		err = storage.ExportProposalsToCSV(session.Proposals, outputFile, config.CSV, exportConfig)
	}

	if err != nil {
		return &CLIError{
			Code:    ExitExportError,
			Message: fmt.Sprintf("Export failed: %v", err),
			Details: map[string]interface{}{
				"output_file": outputFile,
				"format":      c.Format,
			},
			Suggestions: []string{
				"Check output directory permissions",
				"Ensure sufficient disk space",
				"Try different output format",
			},
		}
	}

	fmt.Printf("Exported rankings to: %s\n", outputFile)
	if c.Global.Verbose {
		fmt.Printf("Format: %s\n", c.Format)
		fmt.Printf("Proposals: %d\n", len(session.Proposals))
		if c.IncludeStats {
			fmt.Println("Statistics included")
		}
		if c.IncludeAudit {
			fmt.Println("Audit trail included")
		}
	}

	return nil
}

// Execute implements the Command interface for ListCommand
func (c *ListCommand) Execute(args []string) error {
	if c.Global != nil && c.Global.Version {
		return showVersion()
	}

	// Find session files
	sessionsDir := "sessions"
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		if c.Format == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No sessions found")
		}
		return nil
	}

	files, err := filepath.Glob(filepath.Join(sessionsDir, "*.json"))
	if err != nil {
		return &CLIError{
			Code:    ExitSessionError,
			Message: fmt.Sprintf("Failed to list sessions: %v", err),
		}
	}

	if len(files) == 0 {
		if c.Format == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No sessions found")
		}
		return nil
	}

	// Load and display sessions
	storage := &data.FileStorage{}
	var sessions []*data.Session

	for _, file := range files {
		session, err := storage.LoadSession(file)
		if err != nil {
			if c.Global.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to load %s: %v\n", file, err)
			}
			continue
		}

		// Filter by status if requested
		if c.Status != "all" && string(session.Status) != c.Status {
			continue
		}

		sessions = append(sessions, session)
	}

	// Output in requested format
	switch c.Format {
	case "json":
		return outputSessionsJSON(sessions)
	case "csv":
		return outputSessionsCSV(sessions)
	default:
		return outputSessionsTable(sessions)
	}
}

// Execute implements the Command interface for ValidateCommand
func (c *ValidateCommand) Execute(args []string) error {
	if c.Version {
		return showVersion()
	}

	// Check if file exists
	if _, err := os.Stat(c.Input); os.IsNotExist(err) {
		return &CLIError{
			Code:    ExitFileError,
			Message: fmt.Sprintf("Input file not found: %s", c.Input),
			Details: map[string]interface{}{
				"file": c.Input,
			},
		}
	}

	// Load configuration for validation
	configPath := c.Config
	if configPath == "" {
		configPath = c.ConfigFile
	}
	config, err := loadConfiguration(configPath, c.Verbose)
	if err != nil {
		// Use defaults if config loading fails
		defaultConfig := data.DefaultSessionConfig()
		config = &defaultConfig
	}

	// Create storage and attempt to parse
	storage := &data.FileStorage{}
	parseResult, err := storage.LoadProposalsFromCSV(c.Input, config.CSV)

	// Display validation results
	fmt.Printf("Validation Results for: %s\n", c.Input)
	fmt.Printf("===========================================\n\n")

	if err != nil {
		fmt.Printf("❌ INVALID: %v\n\n", err)

		return &CLIError{
			Code:    ExitValidationError,
			Message: fmt.Sprintf("CSV validation failed: %v", err),
			Details: map[string]interface{}{
				"file": c.Input,
			},
			Suggestions: []string{
				"Check CSV format and encoding",
				"Ensure required columns are present",
				"Verify delimiter and quote characters",
			},
		}
	}

	fmt.Printf("✅ VALID CSV file\n\n")

	// Display file statistics
	fmt.Printf("File Statistics:\n")
	fmt.Printf("  Rows: %d proposals\n", len(parseResult.Proposals))
	fmt.Printf("  Columns: %d\n", len(parseResult.Metadata.Headers))

	// Display column mapping
	fmt.Printf("\nColumn Mapping:\n")
	for i, header := range parseResult.Metadata.Headers {
		fmt.Printf("  [%d] %s\n", i+1, header)
	}

	// Display parse errors if any
	if len(parseResult.ParseErrors) > 0 {
		fmt.Printf("\nParse Errors:\n")
		for _, parseErr := range parseResult.ParseErrors {
			fmt.Printf("  ⚠️  Row %d: %s\n", parseErr.RowNumber, parseErr.Message)
		}
	}

	// Display data preview
	if c.Preview > 0 && len(parseResult.Proposals) > 0 {
		fmt.Printf("\nData Preview (%d rows):\n", min(c.Preview, len(parseResult.Proposals)))
		for i, proposal := range parseResult.Proposals {
			if i >= c.Preview {
				break
			}
			fmt.Printf("  [%d] %s - %s\n", i+1, proposal.ID, proposal.Title)
			if proposal.Speaker != "" {
				fmt.Printf("      Speaker: %s\n", proposal.Speaker)
			}
		}
	}

	return nil
}

// Helper functions

func showVersion() error {
	fmt.Printf("confelo version %s\n", Version)
	fmt.Printf("Build date: %s\n", BuildDate)
	fmt.Printf("Git commit: %s\n", GitCommit)
	return nil
}

func loadConfiguration(configPath string, verbose bool) (*data.SessionConfig, error) {
	if configPath == "" {
		configPath = "confelo.yaml"
	}

	config, err := data.LoadWithEnvironment(configPath)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Config load error: %v\n", err)
		}
		// Use defaults if config file not found
		defaultConfig := data.DefaultSessionConfig()
		return &defaultConfig, nil
	}

	return config, nil
}

func applyStartConfigOverrides(config *data.SessionConfig, cmd *StartCommand) {
	if cmd.ComparisonMode != "" {
		config.UI.ComparisonMode = cmd.ComparisonMode
	}
	if cmd.InitialRating != 0 {
		config.Elo.InitialRating = cmd.InitialRating
	}
	if cmd.OutputScale != "" {
		// Parse output scale format like "0-100" or "1-5"
		parts := strings.Split(cmd.OutputScale, "-")
		if len(parts) == 2 {
			if min, err := strconv.ParseFloat(parts[0], 64); err == nil {
				config.Elo.OutputMin = min
			}
			if max, err := strconv.ParseFloat(parts[1], 64); err == nil {
				config.Elo.OutputMax = max
			}
		}
	}
}

func generateSessionID() string {
	return fmt.Sprintf("session_%s_%08x",
		time.Now().Format("20060102_150405"),
		time.Now().UnixNano()&0xffffffff)
}

func runBatchMode(session *data.Session, config *data.SessionConfig, storage data.Storage) error {
	fmt.Printf("Running in batch mode for session: %s\n", session.ID)
	fmt.Printf("Loaded %d proposals\n", len(session.Proposals))

	// In batch mode, we don't do comparisons, just save the session and exit
	// This allows users to set up sessions via CLI for later interactive use
	fmt.Printf("Session saved. Use 'confelo resume %s' to start interactive comparisons.\n", session.ID)

	return nil
}

func runInteractiveMode(session *data.Session, config *data.SessionConfig, storage data.Storage) error {
	// For now, provide a message that interactive mode needs screen setup
	// This is a placeholder until the TUI screen registration is properly implemented
	fmt.Printf("Interactive mode started for session: %s\n", session.ID)
	fmt.Printf("Session: %s (%s)\n", session.Name, session.ID)
	fmt.Printf("Proposals loaded: %d\n", len(session.Proposals))
	fmt.Println()
	fmt.Println("Interactive TUI mode is not fully implemented yet.")
	fmt.Println("The TUI screens need to be properly integrated with the CLI interface.")
	fmt.Println()
	fmt.Printf("To export the session results, use:\n")
	fmt.Printf("  confelo export --session-id %s --output rankings.csv\n", session.ID)
	fmt.Println()
	fmt.Printf("To resume this session later, use:\n")
	fmt.Printf("  confelo resume --session-id %s\n", session.ID)

	return nil
}

func outputSessionsJSON(sessions []*data.Session) error {
	type sessionSummary struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Status      string    `json:"status"`
		Proposals   int       `json:"proposals"`
		Comparisons int       `json:"comparisons"`
		Created     time.Time `json:"created"`
	}

	var summaries []sessionSummary
	for _, session := range sessions {
		summaries = append(summaries, sessionSummary{
			ID:          session.ID,
			Name:        session.Name,
			Status:      string(session.Status),
			Proposals:   len(session.Proposals),
			Comparisons: len(session.CompletedComparisons),
			Created:     session.CreatedAt,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summaries)
}

func outputSessionsCSV(sessions []*data.Session) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"ID", "Name", "Status", "Proposals", "Comparisons", "Created"}); err != nil {
		return err
	}

	// Write data
	for _, session := range sessions {
		record := []string{
			session.ID,
			session.Name,
			string(session.Status),
			fmt.Sprintf("%d", len(session.Proposals)),
			fmt.Sprintf("%d", len(session.CompletedComparisons)),
			session.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func outputSessionsTable(sessions []*data.Session) error {
	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	// Print header
	fmt.Printf("%-25s %-15s %-10s %-10s %-12s %s\n",
		"SESSION ID", "NAME", "STATUS", "PROPOSALS", "COMPARISONS", "CREATED")
	fmt.Println(strings.Repeat("-", 95))

	// Print sessions
	for _, session := range sessions {
		name := session.Name
		if len(name) > 15 {
			name = name[:12] + "..."
		}

		fmt.Printf("%-25s %-15s %-10s %-10d %-12d %s\n",
			session.ID,
			name,
			session.Status,
			len(session.Proposals),
			len(session.CompletedComparisons),
			session.CreatedAt.Format("2006-01-02 15:04"))
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
