// Package data provides file-based persistence functionality for conference talk ranking.
// It implements CSV input parsing with configurable formats and JSON session serialization
// with atomic writes, following the KISS principle where CSV is source of truth and delivery.
package data

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Error types for storage operations
var (
	ErrStorageOperation  = errors.New("storage operation failed")
	ErrCSVFormat         = errors.New("CSV format error")
	ErrJSONSerialization = errors.New("JSON serialization error")
	ErrAtomicWrite       = errors.New("atomic write operation failed")
	ErrBackupRotation    = errors.New("backup rotation failed")
	ErrCorruptedFile     = errors.New("corrupted file detected")
)

// Storage interface defines the contract for file-based persistence operations
// Following KISS principle: CSV in → JSON for sessions → CSV out
type Storage interface {
	// CSV Operations - Source of truth
	LoadProposalsFromCSV(filename string, config CSVConfig) (*CSVParseResult, error)
	LoadProposalsFromCSVWithElo(filename string, config CSVConfig, eloConfig *EloConfig) (*CSVParseResult, error)
	UpdateCSVScores(proposals []Proposal, filename string, config CSVConfig, eloConfig *EloConfig) error

	// JSON Operations - Session management only
	SaveSession(session *Session, filename string) error
	LoadSession(filename string) (*Session, error)

	// Backup and Recovery
	CreateBackup(filename string) (string, error)
	RecoverFromBackup(filename string) error
	RotateBackups(basePath string, maxBackups int) error
}

// FileStorage implements the Storage interface with file-based operations
type FileStorage struct {
	mu           sync.RWMutex // Protects concurrent operations
	backupDir    string       // Directory for backup files
	maxBackups   int          // Maximum number of backups to keep
	atomicWrites bool         // Whether to use atomic writes for safety
}

// NewFileStorage creates a new FileStorage instance with sensible defaults
func NewFileStorage(backupDir string) *FileStorage {
	if backupDir == "" {
		backupDir = "backups"
	}

	return &FileStorage{
		backupDir:    backupDir,
		maxBackups:   5,
		atomicWrites: true,
	}
}

// SetAtomicWrites enables or disables atomic write operations
func (fs *FileStorage) SetAtomicWrites(enabled bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.atomicWrites = enabled
}

// SetMaxBackups sets the maximum number of backups to retain
func (fs *FileStorage) SetMaxBackups(max int) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.maxBackups = max
}

// ensureBackupDir creates the backup directory if it doesn't exist
func (fs *FileStorage) ensureBackupDir() error {
	return os.MkdirAll(fs.backupDir, 0755)
}

// LoadProposalsFromCSV implements CSV input parsing with configurable formats
// This is where proposals enter the system (CSV → Proposal structs)
func (fs *FileStorage) LoadProposalsFromCSV(filename string, config CSVConfig) (*CSVParseResult, error) {
	return fs.LoadProposalsFromCSVWithElo(filename, config, nil)
}

// LoadProposalsFromCSVWithElo implements CSV input parsing with Elo score conversion
// If eloConfig is provided, scores in the CSV are converted from the output scale to Elo scale
func (fs *FileStorage) LoadProposalsFromCSVWithElo(filename string, config CSVConfig, eloConfig *EloConfig) (*CSVParseResult, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot open CSV file %s: %v", ErrCSVFormat, filename, err)
	}
	defer file.Close()

	return fs.parseCSVFromReader(file, config, eloConfig)
}

// parseCSVFromReader handles the actual CSV parsing logic
func (fs *FileStorage) parseCSVFromReader(reader io.Reader, config CSVConfig, eloConfig *EloConfig) (*CSVParseResult, error) {
	csvReader := csv.NewReader(reader)

	// Configure CSV reader based on config
	delimiter := ','
	if config.Delimiter != "" {
		if len(config.Delimiter) > 0 {
			delimiter = rune(config.Delimiter[0])
		}
	}
	csvReader.Comma = delimiter
	csvReader.LazyQuotes = true // Handle malformed quotes gracefully
	csvReader.TrimLeadingSpace = true

	// Read all records
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse CSV: %v", ErrCSVFormat, err)
	}

	if len(records) == 0 {
		return &CSVParseResult{
			Proposals:      []Proposal{},
			ParseErrors:    []CSVParseError{},
			TotalRows:      0,
			SuccessfulRows: 0,
			Metadata: CSVParseMetadata{
				Headers:         []string{},
				DetectedColumns: map[string]int{},
				UnmappedColumns: []string{},
				ParsedAt:        time.Now(),
			},
		}, nil
	}

	var headers []string
	startRow := 0

	// Handle header row
	if config.HasHeader && len(records) > 0 {
		headers = records[0]
		startRow = 1
	} else {
		// Generate column indices as headers if no header row
		for i := 0; i < len(records[0]); i++ {
			headers = append(headers, fmt.Sprintf("col_%d", i))
		}
	}

	// Build column mapping
	columnMap := make(map[string]int)
	for i, header := range headers {
		columnMap[strings.TrimSpace(strings.ToLower(header))] = i
	}

	// Find required columns
	idCol := fs.findColumn(config.IDColumn, columnMap)
	titleCol := fs.findColumn(config.TitleColumn, columnMap)
	speakerCol := fs.findColumn(config.SpeakerColumn, columnMap)
	abstractCol := fs.findColumn(config.AbstractColumn, columnMap)
	scoreCol := fs.findColumn(config.ScoreColumn, columnMap)
	conflictCol := fs.findColumn(config.ConflictColumn, columnMap)

	if idCol == -1 {
		return nil, fmt.Errorf("%w: required ID column '%s' not found", ErrCSVFormat, config.IDColumn)
	}
	if titleCol == -1 {
		return nil, fmt.Errorf("%w: required title column '%s' not found", ErrCSVFormat, config.TitleColumn)
	}

	var proposals []Proposal
	var parseErrors []CSVParseError
	var skippedRows []int
	successfulRows := 0

	// Process data rows
	for rowIdx := startRow; rowIdx < len(records); rowIdx++ {
		row := records[rowIdx]

		// Skip empty rows
		if fs.isEmptyRow(row) {
			skippedRows = append(skippedRows, rowIdx+1)
			continue
		}

		proposal, err := fs.parseProposalFromRow(row, rowIdx+1, headers, idCol, titleCol, speakerCol, abstractCol, scoreCol, conflictCol, eloConfig)
		if err != nil {
			if csvErr, ok := err.(CSVParseError); ok {
				parseErrors = append(parseErrors, csvErr)
			} else {
				parseErrors = append(parseErrors, CSVParseError{
					RowNumber: rowIdx + 1,
					Message:   err.Error(),
				})
			}
			continue
		}

		proposals = append(proposals, *proposal)
		successfulRows++
	}

	// Identify unmapped columns for metadata preservation
	unmappedColumns := []string{}
	usedColumns := map[string]bool{
		strings.ToLower(config.IDColumn):       true,
		strings.ToLower(config.TitleColumn):    true,
		strings.ToLower(config.SpeakerColumn):  true,
		strings.ToLower(config.AbstractColumn): true,
		strings.ToLower(config.ScoreColumn):    true,
		strings.ToLower(config.ConflictColumn): true,
	}

	for _, header := range headers {
		if !usedColumns[strings.ToLower(header)] {
			unmappedColumns = append(unmappedColumns, header)
		}
	}

	return &CSVParseResult{
		Proposals:      proposals,
		ParseErrors:    parseErrors,
		SkippedRows:    skippedRows,
		TotalRows:      len(records),
		SuccessfulRows: successfulRows,
		Metadata: CSVParseMetadata{
			Headers:         headers,
			DetectedColumns: columnMap,
			UnmappedColumns: unmappedColumns,
			ParsedAt:        time.Now(),
		},
	}, nil
}

// findColumn locates a column by name in the column mapping, case-insensitive
func (fs *FileStorage) findColumn(columnName string, columnMap map[string]int) int {
	if columnName == "" {
		return -1
	}

	normalizedName := strings.TrimSpace(strings.ToLower(columnName))
	if idx, exists := columnMap[normalizedName]; exists {
		return idx
	}

	return -1
}

// isEmptyRow checks if a CSV row is empty or contains only whitespace
func (fs *FileStorage) isEmptyRow(row []string) bool {
	for _, field := range row {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

// parseProposalFromRow creates a Proposal from a CSV row
func (fs *FileStorage) parseProposalFromRow(row []string, rowNum int, headers []string, idCol, titleCol, speakerCol, abstractCol, scoreCol, conflictCol int, eloConfig *EloConfig) (*Proposal, error) {
	// Validate row has enough columns
	maxCol := fs.maxIndex(idCol, titleCol, speakerCol, abstractCol, scoreCol, conflictCol)
	if len(row) <= maxCol {
		return nil, CSVParseError{
			RowNumber: rowNum,
			Message:   fmt.Sprintf("row has %d columns but needs at least %d", len(row), maxCol+1),
		}
	}

	// Extract required fields
	id := strings.TrimSpace(row[idCol])
	if id == "" {
		return nil, CSVParseError{
			RowNumber: rowNum,
			Field:     "id",
			Message:   "ID cannot be empty",
		}
	}

	title := strings.TrimSpace(row[titleCol])
	if title == "" {
		return nil, CSVParseError{
			RowNumber: rowNum,
			Field:     "title",
			Message:   "title cannot be empty",
		}
	}

	// Extract optional fields
	var speaker, abstract string
	if speakerCol >= 0 && speakerCol < len(row) {
		speaker = strings.TrimSpace(row[speakerCol])
	}
	if abstractCol >= 0 && abstractCol < len(row) {
		abstract = strings.TrimSpace(row[abstractCol])
	}

	// Parse score
	var score float64 = 1500.0 // Default from Elo config
	var originalScore *float64
	if scoreCol >= 0 && scoreCol < len(row) {
		scoreStr := strings.TrimSpace(row[scoreCol])
		if scoreStr != "" {
			parsedScore, err := strconv.ParseFloat(scoreStr, 64)
			if err != nil {
				// Invalid score - use default but don't fail
				score = 1500.0
				if eloConfig != nil {
					score = eloConfig.InitialRating
				}
			} else {
				originalScore = &parsedScore
				// Convert CSV score to Elo scale if config provided
				if eloConfig != nil {
					score = eloConfig.ConvertCSVScoreToElo(parsedScore)
				} else {
					score = parsedScore
				}
			}
		} else if eloConfig != nil {
			score = eloConfig.InitialRating
		}
	} else if eloConfig != nil {
		score = eloConfig.InitialRating
	}

	// Parse conflict tags
	var conflictTags []string
	if conflictCol >= 0 && conflictCol < len(row) {
		conflictStr := strings.TrimSpace(row[conflictCol])
		if conflictStr != "" {
			// Split by common separators
			for _, separator := range []string{",", ";", "|"} {
				if strings.Contains(conflictStr, separator) {
					parts := strings.Split(conflictStr, separator)
					for _, part := range parts {
						if trimmed := strings.TrimSpace(part); trimmed != "" {
							conflictTags = append(conflictTags, trimmed)
						}
					}
					break
				}
			}
			// If no separator found, treat as single tag
			if len(conflictTags) == 0 {
				conflictTags = []string{conflictStr}
			}
		}
	}

	// Preserve all metadata
	metadata := make(map[string]string)
	for i, value := range row {
		if i < len(headers) && strings.TrimSpace(value) != "" {
			// Store all columns as metadata for potential export
			metadata[headers[i]] = strings.TrimSpace(value)
		}
	}

	now := time.Now()
	proposal := &Proposal{
		ID:            id,
		Title:         title,
		Abstract:      abstract,
		Speaker:       speaker,
		Score:         score,
		OriginalScore: originalScore, // Preserve original CSV score for export
		Metadata:      metadata,
		ConflictTags:  conflictTags,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	return proposal, nil
}

// maxIndex returns the maximum value among the given indices
func (fs *FileStorage) maxIndex(indices ...int) int {
	max := -1
	for _, idx := range indices {
		if idx > max {
			max = idx
		}
	}
	return max
}

// UpdateCSVScores updates the score column in the original CSV file with export scores
// This preserves all original data and only modifies the score column
func (fs *FileStorage) UpdateCSVScores(proposals []Proposal, filename string, config CSVConfig, eloConfig *EloConfig) error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Read the original CSV file
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot open CSV file %s: %w", filename, err)
	}

	csvReader := csv.NewReader(file)
	if config.Delimiter != "" && len(config.Delimiter) > 0 {
		csvReader.Comma = rune(config.Delimiter[0])
	}

	records, err := csvReader.ReadAll()
	file.Close()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("CSV file is empty")
	}

	// Find score column index
	var scoreColIdx int = -1
	var headers []string
	startRow := 0

	if config.HasHeader {
		headers = records[0]
		startRow = 1
		for i, header := range headers {
			if strings.EqualFold(strings.TrimSpace(header), config.ScoreColumn) {
				scoreColIdx = i
				break
			}
		}
	}

	if scoreColIdx == -1 {
		return fmt.Errorf("score column '%s' not found in CSV", config.ScoreColumn)
	}

	// Create proposal ID to export score map
	scoreMap := make(map[string]string)
	for _, proposal := range proposals {
		var exportScore float64
		if eloConfig != nil {
			exportScore = eloConfig.CalculateExportScore(proposal.Score)
		} else {
			exportScore = proposal.Score
		}

		// Format based on UseDecimals setting
		if eloConfig != nil && !eloConfig.UseDecimals {
			scoreMap[proposal.ID] = fmt.Sprintf("%d", int(exportScore))
		} else {
			scoreMap[proposal.ID] = fmt.Sprintf("%.1f", exportScore)
		}
	}

	// Update score column for each row
	for rowIdx := startRow; rowIdx < len(records); rowIdx++ {
		row := records[rowIdx]
		if len(row) <= scoreColIdx {
			continue
		}

		// Find proposal ID (assuming it's in first column or ID column)
		var proposalID string
		if config.HasHeader {
			for i, header := range headers {
				if strings.EqualFold(strings.TrimSpace(header), config.IDColumn) {
					if i < len(row) {
						proposalID = strings.TrimSpace(row[i])
					}
					break
				}
			}
		} else if len(row) > 0 {
			proposalID = strings.TrimSpace(row[0])
		}

		// Update score if we have a mapping
		if exportScore, exists := scoreMap[proposalID]; exists {
			records[rowIdx][scoreColIdx] = exportScore
		}
	}

	// Write back to file atomically
	tempFile := filename + ".tmp"
	outFile, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", err)
	}

	writer := csv.NewWriter(outFile)
	if config.Delimiter != "" && len(config.Delimiter) > 0 {
		writer.Comma = rune(config.Delimiter[0])
	}

	err = writer.WriteAll(records)
	outFile.Close()
	if err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to write CSV: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, filename); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// SaveSession implements JSON session file serialization with atomic writes
// JSON is only for session management - CSV remains source of truth
func (fs *FileStorage) SaveSession(session *Session, filename string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if session == nil {
		return fmt.Errorf("%w: session cannot be nil", ErrJSONSerialization)
	}

	// Extract current proposal scores for lightweight persistence
	// Proposals will be reloaded from CSV when resuming
	session.ProposalScores = make(map[string]float64, len(session.Proposals))
	for _, proposal := range session.Proposals {
		session.ProposalScores[proposal.ID] = proposal.Score
	}

	// Create backup before overwriting existing session
	if _, err := os.Stat(filename); err == nil {
		if _, err := fs.CreateBackup(filename); err != nil {
			// Log backup failure but don't fail save operation
			// In production, might want to log this warning
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("%w: cannot create session directory: %v", ErrJSONSerialization, err)
	}

	// Choose write strategy based on configuration
	if fs.atomicWrites {
		return fs.saveSessionAtomic(session, filename)
	}
	return fs.saveSessionDirect(session, filename)
}

// saveSessionAtomic performs an atomic write using temporary file + rename
func (fs *FileStorage) saveSessionAtomic(session *Session, filename string) error {
	tempFile := filename + ".tmp"

	// Write to temporary file first
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("%w: cannot create temp session file: %v", ErrAtomicWrite, err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty print for debugging

	if err := encoder.Encode(session); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("%w: failed to encode session: %v", ErrJSONSerialization, err)
	}

	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("%w: failed to sync session file: %v", ErrAtomicWrite, err)
	}

	file.Close()

	// Atomic rename
	if err := os.Rename(tempFile, filename); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("%w: atomic rename failed: %v", ErrAtomicWrite, err)
	}

	return nil
}

// saveSessionDirect performs direct file write (non-atomic)
func (fs *FileStorage) saveSessionDirect(session *Session, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("%w: cannot create session file: %v", ErrJSONSerialization, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(session); err != nil {
		return fmt.Errorf("%w: failed to encode session: %v", ErrJSONSerialization, err)
	}

	return file.Sync()
}

// LoadSession implements JSON session loading with corruption recovery
func (fs *FileStorage) LoadSession(filename string) (*Session, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Try to load the main file
	session, err := fs.loadSessionFromFile(filename)
	if err != nil {
		// If main file is corrupted, try backup recovery
		if errors.Is(err, ErrCorruptedFile) {
			if backupSession, backupErr := fs.tryBackupRecovery(filename); backupErr == nil {
				return backupSession, nil
			}
		}
		return nil, err
	}

	return session, nil
}

// loadSessionFromFile loads session from a specific file with corruption detection
func (fs *FileStorage) loadSessionFromFile(filename string) (*Session, error) {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: session file does not exist", ErrSessionNotFound)
		}
		return nil, fmt.Errorf("%w: cannot open session file: %v", ErrJSONSerialization, err)
	}
	defer file.Close()

	var session Session
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&session); err != nil {
		return nil, fmt.Errorf("%w: corrupted session file: %v", ErrCorruptedFile, err)
	}

	// Basic validation to ensure session is valid
	if session.Name == "" {
		return nil, fmt.Errorf("%w: session has no name", ErrCorruptedFile)
	}

	// Validate that InputCSVPath is set - required for proposal reloading
	if session.InputCSVPath == "" {
		return nil, fmt.Errorf("%w: session has no input CSV path", ErrCorruptedFile)
	}

	// Reload proposals from original CSV file
	// Proposals are never saved in session JSON to keep files small
	result, err := fs.LoadProposalsFromCSVWithElo(session.InputCSVPath, session.Config.CSV, &session.Config.Elo)
	if err != nil {
		return nil, fmt.Errorf("failed to reload proposals from CSV %s: %w", session.InputCSVPath, err)
	}
	session.Proposals = result.Proposals

	// Restore saved scores from ProposalScores map
	if session.ProposalScores != nil {
		for i := range session.Proposals {
			if savedScore, exists := session.ProposalScores[session.Proposals[i].ID]; exists {
				session.Proposals[i].Score = savedScore
			}
		}
	}

	// Rebuild proposal index for fast lookup
	session.ProposalIndex = make(map[string]int, len(session.Proposals))
	for i, proposal := range session.Proposals {
		session.ProposalIndex[proposal.ID] = i
	}

	// Initialize or update ConvergenceMetrics based on loaded comparison counts
	if session.ConvergenceMetrics == nil {
		// Create new metrics if none exist (old session format)
		session.ConvergenceMetrics = &ConvergenceMetrics{
			TotalComparisons:    session.TotalComparisons,
			AvgRatingChange:     0.0,
			RatingVariance:      0.0,
			RankingStability:    0.0,
			CoveragePercentage:  0.0,
			ConvergenceScore:    0.0,
			LastCalculated:      time.Now(),
			RecentRatingChanges: make([]float64, 0, 10),
		}
	} else {
		// Update total comparisons count from persisted data
		session.ConvergenceMetrics.TotalComparisons = session.TotalComparisons
	}

	// Initialize comparison counts map if nil (backward compatibility)
	if session.ComparisonCounts == nil {
		session.ComparisonCounts = make(map[string]int)
	}

	// Set storage directory for loaded session
	sessionDir := filepath.Dir(filename)
	session.storageDirectory = sessionDir

	return &session, nil
}

// tryBackupRecovery attempts to recover from the most recent backup
func (fs *FileStorage) tryBackupRecovery(filename string) (*Session, error) {
	backupPattern := fs.getBackupPath(filename, "*")
	matches, err := filepath.Glob(backupPattern)
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("no backups found for recovery")
	}

	// Try backups in reverse chronological order (most recent first)
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	for _, backupFile := range matches {
		if session, err := fs.loadSessionFromFile(backupFile); err == nil {
			return session, nil
		}
	}

	return nil, fmt.Errorf("all backups are corrupted")
}

// CreateBackup creates a timestamped backup of the specified file
func (fs *FileStorage) CreateBackup(filename string) (string, error) {
	if err := fs.ensureBackupDir(); err != nil {
		return "", fmt.Errorf("%w: cannot create backup directory: %v", ErrBackupRotation, err)
	}

	// Check if source file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return "", fmt.Errorf("%w: source file does not exist", ErrBackupRotation)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fs.getBackupPath(filename, timestamp)

	// Copy file to backup location
	if err := fs.copyFile(filename, backupPath); err != nil {
		return "", fmt.Errorf("%w: failed to copy file to backup: %v", ErrBackupRotation, err)
	}

	return backupPath, nil
}

// RecoverFromBackup restores a file from its most recent backup
func (fs *FileStorage) RecoverFromBackup(filename string) error {
	backupPattern := fs.getBackupPath(filename, "*")
	matches, err := filepath.Glob(backupPattern)
	if err != nil {
		return fmt.Errorf("%w: failed to find backups: %v", ErrBackupRotation, err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("%w: no backups found", ErrBackupRotation)
	}

	// Get most recent backup (lexicographically last due to timestamp format)
	sort.Strings(matches)
	mostRecent := matches[len(matches)-1]

	// Copy backup to original location
	if err := fs.copyFile(mostRecent, filename); err != nil {
		return fmt.Errorf("%w: failed to restore from backup: %v", ErrBackupRotation, err)
	}

	return nil
}

// RotateBackups removes old backups to maintain maximum count
func (fs *FileStorage) RotateBackups(basePath string, maxBackups int) error {
	if maxBackups <= 0 {
		return nil // No rotation needed
	}

	backupPattern := fs.getBackupPath(basePath, "*")
	matches, err := filepath.Glob(backupPattern)
	if err != nil {
		return fmt.Errorf("%w: failed to find backups for rotation: %v", ErrBackupRotation, err)
	}

	if len(matches) <= maxBackups {
		return nil // Within limits
	}

	// Sort by filename (timestamp) and remove oldest
	sort.Strings(matches)
	toRemove := matches[:len(matches)-maxBackups]

	for _, backup := range toRemove {
		if err := os.Remove(backup); err != nil {
			// Continue removing others even if one fails
			continue
		}
	}

	return nil
}

// getBackupPath generates backup file path with timestamp pattern
func (fs *FileStorage) getBackupPath(originalPath, timestamp string) string {
	baseName := filepath.Base(originalPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	backupName := fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)
	return filepath.Join(fs.backupDir, backupName)
}

// copyFile copies a file from source to destination
func (fs *FileStorage) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}
