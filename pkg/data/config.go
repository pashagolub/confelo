// Package data provides configuration management and data structures for the confelo application.
// It handles CSV parsing configuration, Elo engine settings, UI preferences, and export options
// with validation and environment variable support.
package data

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Error types for configuration validation
var (
	ErrInvalidCSVConfig    = errors.New("invalid CSV configuration")
	ErrInvalidEloConfig    = errors.New("invalid Elo configuration")
	ErrInvalidUIConfig     = errors.New("invalid UI configuration")
	ErrInvalidExportConfig = errors.New("invalid export configuration")
	ErrConfigNotFound      = errors.New("configuration file not found")
	ErrConfigParseError    = errors.New("failed to parse configuration file")
)

// SessionConfig is the top-level configuration for a ranking session
type SessionConfig struct {
	CSV         CSVConfig         `yaml:"csv" json:"csv"`
	Elo         EloConfig         `yaml:"elo" json:"elo"`
	UI          UIConfig          `yaml:"ui" json:"ui"`
	Export      ExportConfig      `yaml:"export" json:"export"`
	Convergence ConvergenceConfig `yaml:"convergence" json:"convergence"`
}

// CSVConfig defines how to parse input CSV files
type CSVConfig struct {
	IDColumn       string `yaml:"id_column" json:"id_column"`             // Column name for proposal ID (required)
	TitleColumn    string `yaml:"title_column" json:"title_column"`       // Column name for title (required)
	AbstractColumn string `yaml:"abstract_column" json:"abstract_column"` // Column name for abstract (optional)
	SpeakerColumn  string `yaml:"speaker_column" json:"speaker_column"`   // Column name for speaker (optional)
	ScoreColumn    string `yaml:"score_column" json:"score_column"`       // Column name for existing score (optional)
	CommentColumn  string `yaml:"comment_column" json:"comment_column"`   // Column name for reviewer comments (optional)
	ConflictColumn string `yaml:"conflict_column" json:"conflict_column"` // Column name for conflict tags (optional)
	HasHeader      bool   `yaml:"has_header" json:"has_header"`           // Whether CSV has header row
	Delimiter      string `yaml:"delimiter" json:"delimiter"`             // CSV field separator (default comma)
}

// EloConfig holds settings for Elo rating calculations
type EloConfig struct {
	InitialRating float64 `yaml:"initial_rating" json:"initial_rating"` // Starting rating for new proposals (default 1500)
	KFactor       int     `yaml:"k_factor" json:"k_factor"`             // Rating change sensitivity (default 32)
	MinRating     float64 `yaml:"min_rating" json:"min_rating"`         // Minimum allowed rating (default 0)
	MaxRating     float64 `yaml:"max_rating" json:"max_rating"`         // Maximum allowed rating (default 3000)
	OutputMin     float64 `yaml:"output_min" json:"output_min"`         // Minimum output scale value
	OutputMax     float64 `yaml:"output_max" json:"output_max"`         // Maximum output scale value
	UseDecimals   bool    `yaml:"use_decimals" json:"use_decimals"`     // Whether output uses decimal places
}

// UIConfig holds terminal interface preferences
type UIConfig struct {
	ComparisonMode   string        `yaml:"comparison_mode" json:"comparison_mode"`       // Default comparison type (pairwise/trio/quartet)
	ShowProgress     bool          `yaml:"show_progress" json:"show_progress"`           // Display progress indicators
	ShowConfidence   bool          `yaml:"show_confidence" json:"show_confidence"`       // Display rating confidence
	AutoSave         bool          `yaml:"auto_save" json:"auto_save"`                   // Automatically save session
	AutoSaveInterval time.Duration `yaml:"auto_save_interval" json:"auto_save_interval"` // Save frequency
}

// ExportConfig holds output format settings
type ExportConfig struct {
	Format          string `yaml:"format" json:"format"`                     // Output format (csv/json/yaml)
	IncludeMetadata bool   `yaml:"include_metadata" json:"include_metadata"` // Include original CSV metadata
	SortBy          string `yaml:"sort_by" json:"sort_by"`                   // Sort criterion (rating/title/speaker)
	SortOrder       string `yaml:"sort_order" json:"sort_order"`             // Sort direction (asc/desc)
	ScaleOutput     bool   `yaml:"scale_output" json:"scale_output"`         // Apply output scaling
	RoundDecimals   int    `yaml:"round_decimals" json:"round_decimals"`     // Decimal places for output
}

// ConvergenceConfig holds settings for intelligent stopping criteria
type ConvergenceConfig struct {
	TargetAccepted      int     `yaml:"target_accepted" json:"target_accepted"`               // Number of talks to be accepted (T)
	TopTStabilityWindow int     `yaml:"top_t_stability_window" json:"top_t_stability_window"` // Window to check top-T stability
	StabilityThreshold  float64 `yaml:"stability_threshold" json:"stability_threshold"`       // Min rating change to consider stable
	MinComparisons      int     `yaml:"min_comparisons" json:"min_comparisons"`               // Minimum comparisons before convergence check
	MaxComparisons      int     `yaml:"max_comparisons" json:"max_comparisons"`               // Hard limit on total comparisons
	EnableEarlyStopping bool    `yaml:"enable_early_stopping" json:"enable_early_stopping"`   // Whether to use convergence detection
	ConfidenceThreshold float64 `yaml:"confidence_threshold" json:"confidence_threshold"`     // Min confidence to recommend stopping
}

// DefaultSessionConfig returns a configuration with sensible defaults
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		CSV:         DefaultCSVConfig(),
		Elo:         DefaultEloConfig(),
		UI:          DefaultUIConfig(),
		Export:      DefaultExportConfig(),
		Convergence: DefaultConvergenceConfig(),
	}
}

// DefaultCSVConfig returns CSV parsing defaults
func DefaultCSVConfig() CSVConfig {
	return CSVConfig{
		IDColumn:       "id",
		TitleColumn:    "title",
		AbstractColumn: "abstract",
		SpeakerColumn:  "speaker",
		ScoreColumn:    "score",
		CommentColumn:  "comments",
		ConflictColumn: "conflicts",
		HasHeader:      true,
		Delimiter:      ",",
	}
}

// DefaultEloConfig returns Elo calculation defaults matching constitutional requirements
func DefaultEloConfig() EloConfig {
	return EloConfig{
		InitialRating: 1500.0,
		KFactor:       32,
		MinRating:     0.0,
		MaxRating:     3000.0,
		OutputMin:     0.0,
		OutputMax:     10.0,
		UseDecimals:   true,
	}
}

// DefaultUIConfig returns TUI interface defaults
func DefaultUIConfig() UIConfig {
	return UIConfig{
		ComparisonMode:   "pairwise",
		ShowProgress:     true,
		ShowConfidence:   true,
		AutoSave:         true,
		AutoSaveInterval: 5 * time.Minute,
	}
}

// DefaultExportConfig returns export format defaults
func DefaultExportConfig() ExportConfig {
	return ExportConfig{
		Format:          "csv",
		IncludeMetadata: true,
		SortBy:          "rating",
		SortOrder:       "desc",
		ScaleOutput:     true,
		RoundDecimals:   2,
	}
}

// DefaultConvergenceConfig returns convergence detection defaults
func DefaultConvergenceConfig() ConvergenceConfig {
	return ConvergenceConfig{
		TargetAccepted:      10,   // Typical conference acceptance: 10-20 talks
		TopTStabilityWindow: 5,    // Check stability over last 5 comparisons
		StabilityThreshold:  5.0,  // <5 point rating changes considered stable
		MinComparisons:      20,   // Minimum comparisons before early stopping
		MaxComparisons:      1000, // Hard limit to prevent infinite sessions
		EnableEarlyStopping: true, // Enable intelligent convergence detection
		ConfidenceThreshold: 0.8,  // 80% confidence required for early stopping
	}
}

// Validate checks that the session configuration is valid
func (sc *SessionConfig) Validate() error {
	if err := sc.CSV.Validate(); err != nil {
		return fmt.Errorf("CSV config validation failed: %w", err)
	}

	if err := sc.Elo.Validate(); err != nil {
		return fmt.Errorf("Elo config validation failed: %w", err)
	}

	if err := sc.UI.Validate(); err != nil {
		return fmt.Errorf("UI config validation failed: %w", err)
	}

	if err := sc.Export.Validate(); err != nil {
		return fmt.Errorf("export config validation failed: %w", err)
	}

	return nil
}

// Validate checks that CSV configuration is valid
func (c *CSVConfig) Validate() error {
	// Required columns
	if strings.TrimSpace(c.IDColumn) == "" {
		return fmt.Errorf("%w: id_column is required", ErrInvalidCSVConfig)
	}

	if strings.TrimSpace(c.TitleColumn) == "" {
		return fmt.Errorf("%w: title_column is required", ErrInvalidCSVConfig)
	}

	// Check for duplicate column names (only for non-empty columns)
	columns := make(map[string]bool)
	columnNames := []struct {
		name  string
		field string
	}{
		{c.IDColumn, "id_column"},
		{c.TitleColumn, "title_column"},
		{c.AbstractColumn, "abstract_column"},
		{c.SpeakerColumn, "speaker_column"},
		{c.ScoreColumn, "score_column"},
		{c.CommentColumn, "comment_column"},
		{c.ConflictColumn, "conflict_column"},
	}

	for _, col := range columnNames {
		if col.name != "" {
			if columns[col.name] {
				return fmt.Errorf("%w: duplicate column name '%s' in field %s", ErrInvalidCSVConfig, col.name, col.field)
			}
			columns[col.name] = true
		}
	}

	// Validate delimiter
	if c.Delimiter == "" {
		return fmt.Errorf("%w: delimiter cannot be empty", ErrInvalidCSVConfig)
	}

	// Common CSV delimiters
	validDelimiters := map[string]bool{
		",": true, ";": true, "\t": true, "|": true,
	}

	if !validDelimiters[c.Delimiter] {
		return fmt.Errorf("%w: delimiter '%s' is not a common CSV separator", ErrInvalidCSVConfig, c.Delimiter)
	}

	return nil
}

// Validate checks that Elo configuration is valid
func (e *EloConfig) Validate() error {
	// K-factor validation
	if e.KFactor <= 0 {
		return fmt.Errorf("%w: k_factor must be positive, got %d", ErrInvalidEloConfig, e.KFactor)
	}

	if e.KFactor > 100 {
		return fmt.Errorf("%w: k_factor %d is unusually high (typical range: 10-50)", ErrInvalidEloConfig, e.KFactor)
	}

	// Rating bounds validation
	if e.MinRating >= e.MaxRating {
		return fmt.Errorf("%w: min_rating (%.2f) must be less than max_rating (%.2f)", ErrInvalidEloConfig, e.MinRating, e.MaxRating)
	}

	if e.InitialRating < e.MinRating || e.InitialRating > e.MaxRating {
		return fmt.Errorf("%w: initial_rating (%.2f) must be between min_rating (%.2f) and max_rating (%.2f)",
			ErrInvalidEloConfig, e.InitialRating, e.MinRating, e.MaxRating)
	}

	// Output scale validation
	if e.OutputMin >= e.OutputMax {
		return fmt.Errorf("%w: output_min (%.2f) must be less than output_max (%.2f)", ErrInvalidEloConfig, e.OutputMin, e.OutputMax)
	}

	return nil
}

// Validate checks that UI configuration is valid
func (u *UIConfig) Validate() error {
	// Comparison mode validation
	validModes := map[string]bool{
		"pairwise": true,
		"trio":     true,
		"quartet":  true,
	}

	if !validModes[u.ComparisonMode] {
		return fmt.Errorf("%w: comparison_mode '%s' must be one of: pairwise, trio, quartet", ErrInvalidUIConfig, u.ComparisonMode)
	}

	// Auto-save validation
	if u.AutoSave && u.AutoSaveInterval <= 0 {
		return fmt.Errorf("%w: auto_save_interval must be positive when auto_save is enabled, got %v", ErrInvalidUIConfig, u.AutoSaveInterval)
	}

	if u.AutoSaveInterval > 24*time.Hour {
		return fmt.Errorf("%w: auto_save_interval %v is unusually long (max recommended: 1 hour)", ErrInvalidUIConfig, u.AutoSaveInterval)
	}

	return nil
}

// Validate checks that export configuration is valid
func (e *ExportConfig) Validate() error {
	// Format validation
	validFormats := map[string]bool{
		"csv":  true,
		"json": true,
		"yaml": true,
	}

	if !validFormats[e.Format] {
		return fmt.Errorf("%w: format '%s' must be one of: csv, json, yaml", ErrInvalidExportConfig, e.Format)
	}

	// Sort criteria validation
	validSortBy := map[string]bool{
		"rating":  true,
		"title":   true,
		"speaker": true,
		"id":      true,
	}

	if !validSortBy[e.SortBy] {
		return fmt.Errorf("%w: sort_by '%s' must be one of: rating, title, speaker, id", ErrInvalidExportConfig, e.SortBy)
	}

	// Sort order validation
	validOrder := map[string]bool{
		"asc":  true,
		"desc": true,
	}

	if !validOrder[e.SortOrder] {
		return fmt.Errorf("%w: sort_order '%s' must be 'asc' or 'desc'", ErrInvalidExportConfig, e.SortOrder)
	}

	// Decimal places validation
	if e.RoundDecimals < 0 || e.RoundDecimals > 10 {
		return fmt.Errorf("%w: round_decimals %d must be between 0 and 10", ErrInvalidExportConfig, e.RoundDecimals)
	}

	return nil
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(filename string) (*SessionConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, filename)
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	var config SessionConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfigParseError, filename, err)
	}

	// Apply defaults for missing values
	config = mergeWithDefaults(config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", filename, err)
	}

	return &config, nil
}

// LoadWithEnvironment loads configuration from file and applies environment variable overrides
func LoadWithEnvironment(filename string) (*SessionConfig, error) {
	// Start with defaults
	config := DefaultSessionConfig()

	// Load from file if it exists
	if filename != "" {
		fileConfig, err := LoadFromFile(filename)
		if err != nil && !errors.Is(err, ErrConfigNotFound) {
			return nil, err
		}
		if err == nil {
			config = *fileConfig
		}
	}

	// Apply environment variable overrides
	applyEnvironmentOverrides(&config)

	// Validate final configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid final configuration: %w", err)
	}

	return &config, nil
}

// SaveToFile saves configuration to a YAML file
func (sc *SessionConfig) SaveToFile(filename string) error {
	data, err := yaml.Marshal(sc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", filename, err)
	}

	return nil
}

// mergeWithDefaults fills in missing values with defaults
func mergeWithDefaults(config SessionConfig) SessionConfig {
	defaults := DefaultSessionConfig()

	// Merge CSV config
	if config.CSV.IDColumn == "" {
		config.CSV.IDColumn = defaults.CSV.IDColumn
	}
	if config.CSV.TitleColumn == "" {
		config.CSV.TitleColumn = defaults.CSV.TitleColumn
	}
	if config.CSV.Delimiter == "" {
		config.CSV.Delimiter = defaults.CSV.Delimiter
	}

	// Merge Elo config
	if config.Elo.InitialRating == 0 {
		config.Elo.InitialRating = defaults.Elo.InitialRating
	}
	if config.Elo.KFactor == 0 {
		config.Elo.KFactor = defaults.Elo.KFactor
	}
	if config.Elo.MaxRating == 0 {
		config.Elo.MaxRating = defaults.Elo.MaxRating
	}
	if config.Elo.OutputMax == 0 {
		config.Elo.OutputMax = defaults.Elo.OutputMax
	}

	// Merge UI config
	if config.UI.ComparisonMode == "" {
		config.UI.ComparisonMode = defaults.UI.ComparisonMode
	}
	if config.UI.AutoSaveInterval == 0 {
		config.UI.AutoSaveInterval = defaults.UI.AutoSaveInterval
	}

	// Merge Export config
	if config.Export.Format == "" {
		config.Export.Format = defaults.Export.Format
	}
	if config.Export.SortBy == "" {
		config.Export.SortBy = defaults.Export.SortBy
	}
	if config.Export.SortOrder == "" {
		config.Export.SortOrder = defaults.Export.SortOrder
	}

	return config
}

// applyEnvironmentOverrides applies environment variable overrides
func applyEnvironmentOverrides(config *SessionConfig) {
	// CSV configuration overrides
	if val := os.Getenv("CONFELO_CSV_ID_COLUMN"); val != "" {
		config.CSV.IDColumn = val
	}
	if val := os.Getenv("CONFELO_CSV_TITLE_COLUMN"); val != "" {
		config.CSV.TitleColumn = val
	}
	if val := os.Getenv("CONFELO_CSV_ABSTRACT_COLUMN"); val != "" {
		config.CSV.AbstractColumn = val
	}
	if val := os.Getenv("CONFELO_CSV_SPEAKER_COLUMN"); val != "" {
		config.CSV.SpeakerColumn = val
	}
	if val := os.Getenv("CONFELO_CSV_SCORE_COLUMN"); val != "" {
		config.CSV.ScoreColumn = val
	}
	if val := os.Getenv("CONFELO_CSV_DELIMITER"); val != "" {
		config.CSV.Delimiter = val
	}
	if val := os.Getenv("CONFELO_CSV_HAS_HEADER"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.CSV.HasHeader = parsed
		}
	}

	// Elo configuration overrides
	if val := os.Getenv("CONFELO_ELO_INITIAL_RATING"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			config.Elo.InitialRating = parsed
		}
	}
	if val := os.Getenv("CONFELO_ELO_K_FACTOR"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.Elo.KFactor = parsed
		}
	}
	if val := os.Getenv("CONFELO_ELO_MIN_RATING"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			config.Elo.MinRating = parsed
		}
	}
	if val := os.Getenv("CONFELO_ELO_MAX_RATING"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			config.Elo.MaxRating = parsed
		}
	}
	if val := os.Getenv("CONFELO_ELO_OUTPUT_MIN"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			config.Elo.OutputMin = parsed
		}
	}
	if val := os.Getenv("CONFELO_ELO_OUTPUT_MAX"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			config.Elo.OutputMax = parsed
		}
	}
	if val := os.Getenv("CONFELO_ELO_USE_DECIMALS"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.Elo.UseDecimals = parsed
		}
	}

	// UI configuration overrides
	if val := os.Getenv("CONFELO_UI_COMPARISON_MODE"); val != "" {
		config.UI.ComparisonMode = val
	}
	if val := os.Getenv("CONFELO_UI_SHOW_PROGRESS"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.UI.ShowProgress = parsed
		}
	}
	if val := os.Getenv("CONFELO_UI_SHOW_CONFIDENCE"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.UI.ShowConfidence = parsed
		}
	}
	if val := os.Getenv("CONFELO_UI_AUTO_SAVE"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.UI.AutoSave = parsed
		}
	}
	if val := os.Getenv("CONFELO_UI_AUTO_SAVE_INTERVAL"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			config.UI.AutoSaveInterval = parsed
		}
	}

	// Export configuration overrides
	if val := os.Getenv("CONFELO_EXPORT_FORMAT"); val != "" {
		config.Export.Format = val
	}
	if val := os.Getenv("CONFELO_EXPORT_INCLUDE_METADATA"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.Export.IncludeMetadata = parsed
		}
	}
	if val := os.Getenv("CONFELO_EXPORT_SORT_BY"); val != "" {
		config.Export.SortBy = val
	}
	if val := os.Getenv("CONFELO_EXPORT_SORT_ORDER"); val != "" {
		config.Export.SortOrder = val
	}
	if val := os.Getenv("CONFELO_EXPORT_SCALE_OUTPUT"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.Export.ScaleOutput = parsed
		}
	}
	if val := os.Getenv("CONFELO_EXPORT_ROUND_DECIMALS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.Export.RoundDecimals = parsed
		}
	}
}
