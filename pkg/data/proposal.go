// Package data provides data structures and validation for conference talk proposals.
// It implements the core proposal data model with validation, conflict-of-interest
// handling, and CSV metadata preservation functionality.
package data

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Error types for proposal validation and processing
var (
	ErrInvalidProposal  = errors.New("invalid proposal")
	ErrDuplicateID      = errors.New("duplicate proposal ID")
	ErrRequiredField    = errors.New("required field missing")
	ErrInvalidScore     = errors.New("invalid score value")
	ErrCSVParsing       = errors.New("CSV parsing error")
	ErrValidationFailed = errors.New("proposal validation failed")
)

// Proposal represents a single conference talk submission
type Proposal struct {
	ID            string            `json:"id"`                       // Unique identifier (from CSV)
	Title         string            `json:"title"`                    // Talk title
	Abstract      string            `json:"abstract,omitempty"`       // Full description (optional)
	Speaker       string            `json:"speaker,omitempty"`        // Presenter information (optional)
	Score         float64           `json:"score"`                    // Current Elo rating
	OriginalScore *float64          `json:"original_score,omitempty"` // Initial rating from CSV (optional)
	Metadata      map[string]string `json:"metadata,omitempty"`       // Additional CSV columns
	ConflictTags  []string          `json:"conflict_tags,omitempty"`  // Conflict-of-interest identifiers
	CreatedAt     time.Time         `json:"created_at"`               // When proposal was loaded
	UpdatedAt     time.Time         `json:"updated_at"`               // Last modification time
}

// ProposalCollection manages a collection of proposals with validation
type ProposalCollection struct {
	Proposals        []Proposal       `json:"proposals"`
	IDIndex          map[string]int   `json:"-"` // Internal index for fast ID lookups
	ValidationConfig ValidationConfig `json:"-"` // Validation settings
}

// ValidationConfig holds validation parameters
type ValidationConfig struct {
	RequireTitle   bool    // Whether title is mandatory (default: true)
	MinTitleLength int     // Minimum title length (default: 1)
	MaxTitleLength int     // Maximum title length (default: 500)
	MinScore       float64 // Minimum allowed score
	MaxScore       float64 // Maximum allowed score
	DefaultScore   float64 // Default score for new proposals
}

// CSVParseResult contains the result of parsing CSV data
type CSVParseResult struct {
	Proposals      []Proposal       `json:"proposals"`
	ParseErrors    []CSVParseError  `json:"parse_errors,omitempty"`
	SkippedRows    []int            `json:"skipped_rows,omitempty"`
	TotalRows      int              `json:"total_rows"`
	SuccessfulRows int              `json:"successful_rows"`
	Metadata       CSVParseMetadata `json:"metadata"`
}

// CSVParseError represents an error encountered while parsing a CSV row
type CSVParseError struct {
	RowNumber int    `json:"row_number"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	Message   string `json:"error"`
}

// Error implements the error interface
func (e CSVParseError) Error() string {
	return fmt.Sprintf("row %d, field '%s' (value: '%s'): %s", e.RowNumber, e.Field, e.Value, e.Message)
}

// CSVParseMetadata contains information about the CSV parsing process
type CSVParseMetadata struct {
	Headers         []string       `json:"headers"`
	DetectedColumns map[string]int `json:"detected_columns"`
	UnmappedColumns []string       `json:"unmapped_columns"`
	ParsedAt        time.Time      `json:"parsed_at"`
}

// DefaultValidationConfig returns sensible validation defaults
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		RequireTitle:   true,
		MinTitleLength: 1,
		MaxTitleLength: 500,
		MinScore:       0.0,
		MaxScore:       3000.0,
		DefaultScore:   1500.0,
	}
}

// NewProposal creates a new proposal with validation
func NewProposal(id, title string, config ValidationConfig) (*Proposal, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: ID cannot be empty", ErrRequiredField)
	}

	proposal := &Proposal{
		ID:           strings.TrimSpace(id),
		Title:        strings.TrimSpace(title),
		Score:        config.DefaultScore,
		Metadata:     make(map[string]string),
		ConflictTags: make([]string, 0),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := proposal.Validate(config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidationFailed, err)
	}

	return proposal, nil
}

// Validate checks that the proposal meets all validation rules
func (p *Proposal) Validate(config ValidationConfig) error {
	// Validate required ID
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("%w: ID is required", ErrInvalidProposal)
	}

	// Validate title requirements
	if config.RequireTitle {
		if strings.TrimSpace(p.Title) == "" {
			return fmt.Errorf("%w: title is required", ErrInvalidProposal)
		}

		titleLen := len(strings.TrimSpace(p.Title))
		if titleLen < config.MinTitleLength {
			return fmt.Errorf("%w: title too short (minimum %d characters)", ErrInvalidProposal, config.MinTitleLength)
		}

		if titleLen > config.MaxTitleLength {
			return fmt.Errorf("%w: title too long (maximum %d characters)", ErrInvalidProposal, config.MaxTitleLength)
		}
	}

	// Validate score bounds
	if p.Score < config.MinScore || p.Score > config.MaxScore {
		return fmt.Errorf("%w: score %.2f outside valid range [%.2f, %.2f]", ErrInvalidScore, p.Score, config.MinScore, config.MaxScore)
	}

	// Validate original score if present
	if p.OriginalScore != nil {
		if *p.OriginalScore < config.MinScore || *p.OriginalScore > config.MaxScore {
			return fmt.Errorf("%w: original score %.2f outside valid range [%.2f, %.2f]", ErrInvalidScore, *p.OriginalScore, config.MinScore, config.MaxScore)
		}
	}

	return nil
}

// UpdateScore updates the proposal's current score and timestamp
func (p *Proposal) UpdateScore(newScore float64) {
	p.Score = newScore
	p.UpdatedAt = time.Now().UTC()
}

// AddConflictTag adds a conflict-of-interest tag if not already present
func (p *Proposal) AddConflictTag(tag string) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return
	}

	// Check if tag already exists
	for _, existing := range p.ConflictTags {
		if existing == tag {
			return
		}
	}

	p.ConflictTags = append(p.ConflictTags, tag)
	p.UpdatedAt = time.Now().UTC()
}

// RemoveConflictTag removes a conflict-of-interest tag
func (p *Proposal) RemoveConflictTag(tag string) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return
	}

	for i, existing := range p.ConflictTags {
		if existing == tag {
			p.ConflictTags = append(p.ConflictTags[:i], p.ConflictTags[i+1:]...)
			p.UpdatedAt = time.Now().UTC()
			return
		}
	}
}

// HasConflictTag checks if the proposal has a specific conflict tag
func (p *Proposal) HasConflictTag(tag string) bool {
	tag = strings.TrimSpace(tag)
	for _, existing := range p.ConflictTags {
		if existing == tag {
			return true
		}
	}
	return false
}

// SetMetadata sets a metadata key-value pair
func (p *Proposal) SetMetadata(key, value string) {
	if p.Metadata == nil {
		p.Metadata = make(map[string]string)
	}
	p.Metadata[key] = value
	p.UpdatedAt = time.Now().UTC()
}

// GetMetadata retrieves a metadata value by key
func (p *Proposal) GetMetadata(key string) (string, bool) {
	if p.Metadata == nil {
		return "", false
	}
	value, exists := p.Metadata[key]
	return value, exists
}

// NewProposalCollection creates a new collection with validation config
func NewProposalCollection(config ValidationConfig) *ProposalCollection {
	return &ProposalCollection{
		Proposals:        make([]Proposal, 0),
		IDIndex:          make(map[string]int),
		ValidationConfig: config,
	}
}

// AddProposal adds a proposal to the collection with duplicate ID checking
func (pc *ProposalCollection) AddProposal(proposal Proposal) error {
	// Validate the proposal
	if err := proposal.Validate(pc.ValidationConfig); err != nil {
		return fmt.Errorf("%w: %v", ErrValidationFailed, err)
	}

	// Check for duplicate ID
	if _, exists := pc.IDIndex[proposal.ID]; exists {
		return fmt.Errorf("%w: ID '%s' already exists", ErrDuplicateID, proposal.ID)
	}

	// Add to collection
	index := len(pc.Proposals)
	pc.Proposals = append(pc.Proposals, proposal)
	pc.IDIndex[proposal.ID] = index

	return nil
}

// GetProposalByID retrieves a proposal by its ID
func (pc *ProposalCollection) GetProposalByID(id string) (*Proposal, bool) {
	index, exists := pc.IDIndex[id]
	if !exists || index >= len(pc.Proposals) {
		return nil, false
	}
	return &pc.Proposals[index], true
}

// UpdateProposal updates an existing proposal in the collection
func (pc *ProposalCollection) UpdateProposal(proposal Proposal) error {
	// Validate the proposal
	if err := proposal.Validate(pc.ValidationConfig); err != nil {
		return fmt.Errorf("%w: %v", ErrValidationFailed, err)
	}

	index, exists := pc.IDIndex[proposal.ID]
	if !exists {
		return fmt.Errorf("%w: proposal ID '%s' not found", ErrInvalidProposal, proposal.ID)
	}

	pc.Proposals[index] = proposal
	return nil
}

// Count returns the number of proposals in the collection
func (pc *ProposalCollection) Count() int {
	return len(pc.Proposals)
}

// IDs returns all proposal IDs in the collection
func (pc *ProposalCollection) IDs() []string {
	ids := make([]string, len(pc.Proposals))
	for i, proposal := range pc.Proposals {
		ids[i] = proposal.ID
	}
	return ids
}

// FilterByConflictTag returns proposals that have the specified conflict tag
func (pc *ProposalCollection) FilterByConflictTag(tag string) []Proposal {
	filtered := make([]Proposal, 0)
	for _, proposal := range pc.Proposals {
		if proposal.HasConflictTag(tag) {
			filtered = append(filtered, proposal)
		}
	}
	return filtered
}

// ExcludeByConflictTag returns proposals that do NOT have the specified conflict tag
func (pc *ProposalCollection) ExcludeByConflictTag(tag string) []Proposal {
	filtered := make([]Proposal, 0)
	for _, proposal := range pc.Proposals {
		if !proposal.HasConflictTag(tag) {
			filtered = append(filtered, proposal)
		}
	}
	return filtered
}

// ParseCSVFromReader parses proposals from a CSV reader using the given configuration
func ParseCSVFromReader(reader io.Reader, csvConfig CSVConfig, validationConfig ValidationConfig) (*CSVParseResult, error) {
	csvReader := csv.NewReader(reader)
	if csvConfig.Delimiter != "" && len(csvConfig.Delimiter) > 0 {
		csvReader.Comma = rune(csvConfig.Delimiter[0])
	}

	result := &CSVParseResult{
		Proposals:   make([]Proposal, 0),
		ParseErrors: make([]CSVParseError, 0),
		SkippedRows: make([]int, 0),
		Metadata: CSVParseMetadata{
			DetectedColumns: make(map[string]int),
			UnmappedColumns: make([]string, 0),
			ParsedAt:        time.Now().UTC(),
		},
	}

	var headers []string
	var columnMap map[string]int
	rowNumber := 0

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read CSV: %v", ErrCSVParsing, err)
		}

		rowNumber++
		result.TotalRows++

		// Handle header row
		if rowNumber == 1 && csvConfig.HasHeader {
			headers = record
			result.Metadata.Headers = headers
			columnMap = buildColumnMap(headers, csvConfig)
			result.Metadata.DetectedColumns = columnMap
			result.Metadata.UnmappedColumns = findUnmappedColumns(headers, csvConfig)
			continue
		}

		// Initialize column map for no-header CSV with numeric indices
		if rowNumber == 1 && !csvConfig.HasHeader && columnMap == nil {
			columnMap = buildColumnMapFromIndices(csvConfig, len(record))
		}

		// Parse data row
		proposal, parseErrors := parseCSVRow(record, columnMap, validationConfig, rowNumber, headers)
		if len(parseErrors) > 0 {
			result.ParseErrors = append(result.ParseErrors, parseErrors...)
			result.SkippedRows = append(result.SkippedRows, rowNumber)
			continue
		}

		if proposal != nil {
			result.Proposals = append(result.Proposals, *proposal)
			result.SuccessfulRows++
		}
	}

	return result, nil
}

// buildColumnMapFromIndices creates a mapping using numeric column indices
func buildColumnMapFromIndices(csvConfig CSVConfig, numColumns int) map[string]int {
	columnMap := make(map[string]int)

	if idx, err := strconv.Atoi(csvConfig.IDColumn); err == nil && idx >= 0 && idx < numColumns {
		columnMap["id"] = idx
	}
	if idx, err := strconv.Atoi(csvConfig.TitleColumn); err == nil && idx >= 0 && idx < numColumns {
		columnMap["title"] = idx
	}
	if csvConfig.AbstractColumn != "" {
		if idx, err := strconv.Atoi(csvConfig.AbstractColumn); err == nil && idx >= 0 && idx < numColumns {
			columnMap["abstract"] = idx
		}
	}
	if csvConfig.SpeakerColumn != "" {
		if idx, err := strconv.Atoi(csvConfig.SpeakerColumn); err == nil && idx >= 0 && idx < numColumns {
			columnMap["speaker"] = idx
		}
	}
	if csvConfig.ScoreColumn != "" {
		if idx, err := strconv.Atoi(csvConfig.ScoreColumn); err == nil && idx >= 0 && idx < numColumns {
			columnMap["score"] = idx
		}
	}
	if csvConfig.ConflictColumn != "" {
		if idx, err := strconv.Atoi(csvConfig.ConflictColumn); err == nil && idx >= 0 && idx < numColumns {
			columnMap["conflict"] = idx
		}
	}

	return columnMap
}

// buildColumnMap creates a mapping from CSV column names to their indices
func buildColumnMap(headers []string, csvConfig CSVConfig) map[string]int {
	columnMap := make(map[string]int)

	for i, header := range headers {
		header = strings.TrimSpace(header)
		if header == csvConfig.IDColumn {
			columnMap["id"] = i
		} else if header == csvConfig.TitleColumn {
			columnMap["title"] = i
		} else if header == csvConfig.AbstractColumn && csvConfig.AbstractColumn != "" {
			columnMap["abstract"] = i
		} else if header == csvConfig.SpeakerColumn && csvConfig.SpeakerColumn != "" {
			columnMap["speaker"] = i
		} else if header == csvConfig.ScoreColumn && csvConfig.ScoreColumn != "" {
			columnMap["score"] = i
		} else if header == csvConfig.CommentColumn && csvConfig.CommentColumn != "" {
			columnMap["comment"] = i
		} else if header == csvConfig.ConflictColumn && csvConfig.ConflictColumn != "" {
			columnMap["conflict"] = i
		}
	}

	return columnMap
}

// findUnmappedColumns identifies CSV columns that are not mapped to proposal fields
func findUnmappedColumns(headers []string, csvConfig CSVConfig) []string {
	mappedColumns := make(map[string]bool)
	mappedColumns[csvConfig.IDColumn] = true
	mappedColumns[csvConfig.TitleColumn] = true
	if csvConfig.AbstractColumn != "" {
		mappedColumns[csvConfig.AbstractColumn] = true
	}
	if csvConfig.SpeakerColumn != "" {
		mappedColumns[csvConfig.SpeakerColumn] = true
	}
	if csvConfig.ScoreColumn != "" {
		mappedColumns[csvConfig.ScoreColumn] = true
	}
	if csvConfig.CommentColumn != "" {
		mappedColumns[csvConfig.CommentColumn] = true
	}
	if csvConfig.ConflictColumn != "" {
		mappedColumns[csvConfig.ConflictColumn] = true
	}

	unmapped := make([]string, 0)
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if !mappedColumns[header] && header != "" {
			unmapped = append(unmapped, header)
		}
	}

	return unmapped
}

// parseCSVRow parses a single CSV row into a Proposal
func parseCSVRow(record []string, columnMap map[string]int, validationConfig ValidationConfig, rowNumber int, headers []string) (*Proposal, []CSVParseError) {
	var errors []CSVParseError

	// Helper function to safely get column value
	getColumn := func(field string) string {
		if index, exists := columnMap[field]; exists && index < len(record) {
			return strings.TrimSpace(record[index])
		}
		return ""
	}

	// Extract required fields
	id := getColumn("id")
	title := getColumn("title")

	if id == "" {
		errors = append(errors, CSVParseError{
			RowNumber: rowNumber,
			Field:     "id",
			Value:     id,
			Message:   "ID is required",
		})
	}

	if title == "" && validationConfig.RequireTitle {
		errors = append(errors, CSVParseError{
			RowNumber: rowNumber,
			Field:     "title",
			Value:     title,
			Message:   "title is required",
		})
	}

	// If we have critical errors, return early
	if len(errors) > 0 {
		return nil, errors
	}

	// Create proposal
	proposal := &Proposal{
		ID:           id,
		Title:        title,
		Abstract:     getColumn("abstract"),
		Speaker:      getColumn("speaker"),
		Score:        validationConfig.DefaultScore,
		Metadata:     make(map[string]string),
		ConflictTags: make([]string, 0),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Parse score if provided
	if scoreStr := getColumn("score"); scoreStr != "" {
		if score, err := strconv.ParseFloat(scoreStr, 64); err == nil {
			proposal.Score = score
			proposal.OriginalScore = &score
		} else {
			errors = append(errors, CSVParseError{
				RowNumber: rowNumber,
				Field:     "score",
				Value:     scoreStr,
				Message:   fmt.Sprintf("invalid score format: %v", err),
			})
		}
	}

	// Parse conflict tags if provided
	if conflictStr := getColumn("conflict"); conflictStr != "" {
		tags := strings.Split(conflictStr, ";")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				proposal.ConflictTags = append(proposal.ConflictTags, tag)
			}
		}
	}

	// Preserve all CSV columns as metadata
	if headers != nil {
		for i, value := range record {
			if i < len(headers) {
				header := strings.TrimSpace(headers[i])
				if header != "" {
					proposal.SetMetadata(header, strings.TrimSpace(value))
				}
			}
		}
	}

	// Validate the proposal
	if err := proposal.Validate(validationConfig); err != nil {
		errors = append(errors, CSVParseError{
			RowNumber: rowNumber,
			Field:     "validation",
			Value:     "",
			Message:   err.Error(),
		})
		return nil, errors
	}

	return proposal, nil
}
