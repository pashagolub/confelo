// Package data provides session configuration types and defaults for the confelo application.
// This implements CLI-only configuration without file loading or environment variable support.
package data

import (
	"errors"
	"fmt"
	"strings"
)

// Error types for configuration validation
var (
	ErrInvalidCSVConfig    = errors.New("invalid CSV configuration")
	ErrInvalidEloConfig    = errors.New("invalid Elo configuration")
	ErrInvalidUIConfig     = errors.New("invalid UI configuration")
	ErrInvalidExportConfig = errors.New("invalid export configuration")
)

// SessionConfig is the top-level configuration for a ranking session
type SessionConfig struct {
	CSV         CSVConfig         `json:"csv"`
	Elo         EloConfig         `json:"elo"`
	UI          UIConfig          `json:"ui"`
	Export      ExportConfig      `json:"export"`
	Convergence ConvergenceConfig `json:"convergence"`
}

// CSVConfig defines how to parse input CSV files
type CSVConfig struct {
	IDColumn       string `json:"id_column"`       // Column name for proposal ID (required)
	TitleColumn    string `json:"title_column"`    // Column name for title (required)
	AbstractColumn string `json:"abstract_column"` // Column name for abstract (optional)
	SpeakerColumn  string `json:"speaker_column"`  // Column name for speaker (optional)
	ScoreColumn    string `json:"score_column"`    // Column name for existing score (optional)
	CommentColumn  string `json:"comment_column"`  // Column name for reviewer comments (optional)
	ConflictColumn string `json:"conflict_column"` // Column name for conflict tags (optional)
	HasHeader      bool   `json:"has_header"`      // Whether CSV has header row
	Delimiter      string `json:"delimiter"`       // CSV field separator (default comma)
}

// EloConfig holds settings for Elo rating calculations
type EloConfig struct {
	InitialRating float64 `json:"initial_rating"` // Starting rating for new proposals (default 1500)
	KFactor       int     `json:"k_factor"`       // Rating change sensitivity (default 32)
	MinRating     float64 `json:"min_rating"`     // Minimum allowed rating (default 0)
	MaxRating     float64 `json:"max_rating"`     // Maximum allowed rating (default 3000)
	OutputMin     float64 `json:"output_min"`     // Minimum output scale value
	OutputMax     float64 `json:"output_max"`     // Maximum output scale value
	UseDecimals   bool    `json:"use_decimals"`   // Whether output uses decimal places
}

// UIConfig holds terminal interface preferences
type UIConfig struct {
	ComparisonMode string `json:"comparison_mode"` // Default comparison type (pairwise/trio/quartet)
	ShowProgress   bool   `json:"show_progress"`   // Display progress indicators
	ShowConfidence bool   `json:"show_confidence"` // Display rating confidence
}

// ExportConfig holds output format settings
type ExportConfig struct {
	Format          string `json:"format"`           // Output format (csv/json/yaml)
	IncludeMetadata bool   `json:"include_metadata"` // Include original CSV metadata
	SortBy          string `json:"sort_by"`          // Sort criterion (rating/title/speaker)
	SortOrder       string `json:"sort_order"`       // Sort direction (asc/desc)
	ScaleOutput     bool   `json:"scale_output"`     // Apply output scaling
	RoundDecimals   int    `json:"round_decimals"`   // Decimal places for output
}

// ConvergenceConfig holds settings for intelligent stopping criteria
type ConvergenceConfig struct {
	TargetAccepted      int     `json:"target_accepted"`        // Number of talks to be accepted (T)
	TopTStabilityWindow int     `json:"top_t_stability_window"` // Window to check top-T stability
	StabilityThreshold  float64 `json:"stability_threshold"`    // Min rating change to consider stable
	MinComparisons      int     `json:"min_comparisons"`        // Minimum comparisons before convergence check
	MaxComparisons      int     `json:"max_comparisons"`        // Hard limit on total comparisons
	EnableEarlyStopping bool    `json:"enable_early_stopping"`  // Whether to use convergence detection
	ConfidenceThreshold float64 `json:"confidence_threshold"`   // Min confidence to recommend stopping
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
		ComparisonMode: "pairwise",
		ShowProgress:   true,
		ShowConfidence: true,
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

// ConvertCSVScoreToElo converts a score from CSV (in OutputMin-OutputMax scale) to Elo rating scale
// This is the inverse of CalculateExportScore, used when loading existing scores from CSV
// If the CSV score is outside the output scale range, returns InitialRating (invalid score)
func (e *EloConfig) ConvertCSVScoreToElo(csvScore float64) float64 {
	// Check if score is within valid output scale range
	// If outside range, treat as invalid and return default
	if csvScore < e.OutputMin || csvScore > e.OutputMax {
		return e.InitialRating
	}

	// Inverse linear scaling: elo = minRating + (score - outputMin) * (maxRating - minRating) / (outputMax - outputMin)
	outputRange := e.OutputMax - e.OutputMin
	ratingRange := e.MaxRating - e.MinRating

	if outputRange == 0 {
		return e.InitialRating // Avoid division by zero
	}

	normalized := (csvScore - e.OutputMin) / outputRange
	eloScore := e.MinRating + (normalized * ratingRange)

	return eloScore
}

// CalculateExportScore converts an Elo rating to the output scale defined in the configuration
// It performs linear scaling from [MinRating, MaxRating] to [OutputMin, OutputMax]
// and respects the UseDecimals setting for formatting
func (e *EloConfig) CalculateExportScore(eloScore float64) float64 {
	// Clamp the Elo score to the valid rating range
	clampedScore := eloScore
	if clampedScore < e.MinRating {
		clampedScore = e.MinRating
	}
	if clampedScore > e.MaxRating {
		clampedScore = e.MaxRating
	}

	// Linear scaling formula: output = outputMin + (score - minRating) * (outputMax - outputMin) / (maxRating - minRating)
	ratingRange := e.MaxRating - e.MinRating
	outputRange := e.OutputMax - e.OutputMin

	normalized := (clampedScore - e.MinRating) / ratingRange
	exportScore := e.OutputMin + (normalized * outputRange)

	// Round to integer if UseDecimals is false
	if !e.UseDecimals {
		return float64(int(exportScore + 0.5)) // Round to nearest integer
	}

	return exportScore
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
