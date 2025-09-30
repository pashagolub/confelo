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
	ExportProposalsToCSV(proposals []Proposal, filename string, config CSVConfig, exportConfig ExportConfig) error

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
// This is the primary entry point - CSV is the source of truth
func (fs *FileStorage) LoadProposalsFromCSV(filename string, config CSVConfig) (*CSVParseResult, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot open CSV file %s: %v", ErrCSVFormat, filename, err)
	}
	defer file.Close()

	return fs.parseCSVFromReader(file, config)
}

// parseCSVFromReader handles the actual CSV parsing logic
func (fs *FileStorage) parseCSVFromReader(reader io.Reader, config CSVConfig) (*CSVParseResult, error) {
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

		proposal, err := fs.parseProposalFromRow(row, rowIdx+1, headers, idCol, titleCol, speakerCol, abstractCol, scoreCol, conflictCol, config)
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
func (fs *FileStorage) parseProposalFromRow(row []string, rowNum int, headers []string, idCol, titleCol, speakerCol, abstractCol, scoreCol, conflictCol int, config CSVConfig) (*Proposal, error) {
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
	if scoreCol >= 0 && scoreCol < len(row) {
		scoreStr := strings.TrimSpace(row[scoreCol])
		if scoreStr != "" {
			parsedScore, err := strconv.ParseFloat(scoreStr, 64)
			if err != nil {
				return nil, CSVParseError{
					RowNumber: rowNum,
					Field:     "score",
					Value:     scoreStr,
					Message:   fmt.Sprintf("invalid score: %v", err),
				}
			}
			score = parsedScore
		}
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
		OriginalScore: &score, // Preserve original score for auditing
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

// ExportProposalsToCSV implements CSV export - the final delivery format
// Following KISS: preserve original format + add new ratings
func (fs *FileStorage) ExportProposalsToCSV(proposals []Proposal, filename string, config CSVConfig, exportConfig ExportConfig) error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if len(proposals) == 0 {
		return fmt.Errorf("%w: no proposals to export", ErrStorageOperation)
	}

	// Create or truncate file atomically
	var finalPath string
	var tempPath string

	if fs.atomicWrites {
		tempPath = filename + ".tmp"
		finalPath = filename
	} else {
		finalPath = filename
		tempPath = filename
	}

	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("%w: cannot create export file %s: %v", ErrStorageOperation, tempPath, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	// Configure CSV writer
	delimiter := ','
	if config.Delimiter != "" && len(config.Delimiter) > 0 {
		delimiter = rune(config.Delimiter[0])
	}
	writer.Comma = delimiter

	// Determine headers from first proposal's metadata or configuration
	var headers []string
	if len(proposals) > 0 && len(proposals[0].Metadata) > 0 {
		// Use original headers from metadata, preserving order
		headerSet := make(map[string]bool)
		for _, proposal := range proposals {
			for header := range proposal.Metadata {
				if !headerSet[header] {
					headers = append(headers, header)
					headerSet[header] = true
				}
			}
		}
	} else {
		// Fallback to standard columns
		headers = []string{config.IDColumn, config.TitleColumn, config.SpeakerColumn, config.AbstractColumn, config.ScoreColumn}
	}

	// Add new rating column if requested
	newRatingColumn := "final_rating"
	if exportConfig.Format == "csv" { // Only add for CSV exports
		headers = append(headers, newRatingColumn)
	}

	// Write header if configured
	if config.HasHeader {
		if err := writer.Write(headers); err != nil {
			return fmt.Errorf("%w: failed to write CSV header: %v", ErrStorageOperation, err)
		}
	}

	// Sort proposals if requested
	sortedProposals := fs.sortProposalsForExport(proposals, exportConfig)

	// Write data rows
	for _, proposal := range sortedProposals {
		row := make([]string, len(headers))

		// Fill in values from metadata (preserves original data)
		for i, header := range headers {
			if header == newRatingColumn {
				// Add the new rating
				if exportConfig.ScaleOutput {
					// Apply scaling if configured (placeholder - would need EloConfig)
					row[i] = fmt.Sprintf("%.1f", proposal.Score)
				} else {
					if exportConfig.RoundDecimals >= 0 {
						format := fmt.Sprintf("%%.%df", exportConfig.RoundDecimals)
						row[i] = fmt.Sprintf(format, proposal.Score)
					} else {
						row[i] = fmt.Sprintf("%.0f", proposal.Score)
					}
				}
			} else if value, exists := proposal.Metadata[header]; exists {
				row[i] = value
			} else {
				// Fallback to standard fields
				switch strings.ToLower(header) {
				case strings.ToLower(config.IDColumn):
					row[i] = proposal.ID
				case strings.ToLower(config.TitleColumn):
					row[i] = proposal.Title
				case strings.ToLower(config.SpeakerColumn):
					row[i] = proposal.Speaker
				case strings.ToLower(config.AbstractColumn):
					row[i] = proposal.Abstract
				case strings.ToLower(config.ScoreColumn):
					if proposal.OriginalScore != nil {
						row[i] = fmt.Sprintf("%.1f", *proposal.OriginalScore)
					}
				case strings.ToLower(config.ConflictColumn):
					row[i] = strings.Join(proposal.ConflictTags, ",")
				}
			}
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("%w: failed to write CSV row: %v", ErrStorageOperation, err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("%w: CSV writer error: %v", ErrStorageOperation, err)
	}

	// Ensure file is properly closed before atomic move (Windows requirement)
	file.Close()

	// Atomic move if using temporary file
	if fs.atomicWrites {
		if err := os.Rename(tempPath, finalPath); err != nil {
			os.Remove(tempPath) // Cleanup temp file
			return fmt.Errorf("%w: atomic move failed: %v", ErrAtomicWrite, err)
		}
	}

	return nil
}

// sortProposalsForExport applies sorting based on export configuration
func (fs *FileStorage) sortProposalsForExport(proposals []Proposal, config ExportConfig) []Proposal {
	// Make a copy to avoid modifying original slice
	sorted := make([]Proposal, len(proposals))
	copy(sorted, proposals)

	switch config.SortBy {
	case "rating", "score":
		sort.Slice(sorted, func(i, j int) bool {
			if config.SortOrder == "desc" {
				return sorted[i].Score > sorted[j].Score
			}
			return sorted[i].Score < sorted[j].Score
		})
	case "title":
		sort.Slice(sorted, func(i, j int) bool {
			if config.SortOrder == "desc" {
				return sorted[i].Title > sorted[j].Title
			}
			return sorted[i].Title < sorted[j].Title
		})
	case "speaker":
		sort.Slice(sorted, func(i, j int) bool {
			if config.SortOrder == "desc" {
				return sorted[i].Speaker > sorted[j].Speaker
			}
			return sorted[i].Speaker < sorted[j].Speaker
		})
	}

	return sorted
}

// SaveSession implements JSON session file serialization with atomic writes
// JSON is only for session management - CSV remains source of truth
func (fs *FileStorage) SaveSession(session *Session, filename string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if session == nil {
		return fmt.Errorf("%w: session cannot be nil", ErrJSONSerialization)
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
	if session.ID == "" {
		return nil, fmt.Errorf("%w: session has no ID", ErrCorruptedFile)
	}

	// Initialize audit trail for loaded session
	sessionDir := filepath.Dir(filename)
	session.storageDirectory = sessionDir
	if err := session.InitializeAuditTrail(sessionDir); err != nil {
		// Log warning but don't fail loading - audit trail is supplementary
		fmt.Printf("Warning: failed to initialize audit trail for loaded session: %v\n", err)
	}

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
