// Package journal provides audit trail and logging functionality for the conference
// talk ranking application. It implements append-only audit logs using JSON Lines
// format with tamper-evident features to ensure complete transparency and integrity
// of the comparison process.
package journal

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Error types for audit trail operations
var (
	ErrAuditLogCorrupted = errors.New("audit log corrupted or tampered")
	ErrInvalidLogEntry   = errors.New("invalid log entry format")
	ErrLogFileNotFound   = errors.New("audit log file not found")
	ErrPermissionDenied  = errors.New("insufficient permissions for audit log")
)

// AuditEventType represents the type of event being logged
type AuditEventType string

const (
	EventComparisonStarted   AuditEventType = "comparison_started"
	EventComparisonCompleted AuditEventType = "comparison_completed"
	EventComparisonSkipped   AuditEventType = "comparison_skipped"
	EventRatingUpdated       AuditEventType = "rating_updated"
	EventSessionCreated      AuditEventType = "session_created"
	EventSessionResumed      AuditEventType = "session_resumed"
	EventSessionPaused       AuditEventType = "session_paused"
	EventSessionCompleted    AuditEventType = "session_completed"
)

// AuditEntry represents a single entry in the audit log
type AuditEntry struct {
	// Core identification
	ID        string         `json:"id"`         // Unique entry identifier
	Timestamp time.Time      `json:"timestamp"`  // When the event occurred
	EventType AuditEventType `json:"event_type"` // Type of event being logged
	SessionID string         `json:"session_id"` // Session this event belongs to

	// Event data
	Data map[string]any `json:"data"` // Event-specific payload

	// Integrity protection
	PreviousHash string `json:"previous_hash"` // Hash of previous entry (tamper detection)
	EntryHash    string `json:"entry_hash"`    // Hash of this entry's content
	Sequence     uint64 `json:"sequence"`      // Sequential entry number
}

// ComparisonAuditData represents audit data for comparison events
type ComparisonAuditData struct {
	ComparisonID string   `json:"comparison_id"`
	ProposalIDs  []string `json:"proposal_ids"`
	Method       string   `json:"method"`
	WinnerID     string   `json:"winner_id,omitempty"`
	Rankings     []string `json:"rankings,omitempty"`
	Duration     string   `json:"duration,omitempty"`
	SkipReason   string   `json:"skip_reason,omitempty"`
}

// RatingAuditData represents audit data for rating change events
type RatingAuditData struct {
	ComparisonID string  `json:"comparison_id"`
	ProposalID   string  `json:"proposal_id"`
	OldRating    float64 `json:"old_rating"`
	NewRating    float64 `json:"new_rating"`
	RatingDelta  float64 `json:"rating_delta"`
	KFactor      int     `json:"k_factor"`
}

// AuditTrail manages the append-only audit log for a session
type AuditTrail struct {
	sessionID     string
	logFilePath   string
	file          *os.File
	mutex         sync.Mutex
	lastHash      string
	sequence      uint64
	isInitialized bool
}

// NewAuditTrail creates a new audit trail for the specified session
func NewAuditTrail(sessionID, logDirectory string) (*AuditTrail, error) {
	if sessionID == "" {
		return nil, errors.New("session ID cannot be empty")
	}

	// Ensure log directory exists
	if err := os.MkdirAll(logDirectory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file path
	logFileName := fmt.Sprintf("audit_%s.jsonl", sessionID)
	logFilePath := filepath.Join(logDirectory, logFileName)

	audit := &AuditTrail{
		sessionID:   sessionID,
		logFilePath: logFilePath,
	}

	// Initialize the audit trail
	if err := audit.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize audit trail: %w", err)
	}

	return audit, nil
}

// initialize prepares the audit trail for use
func (a *AuditTrail) initialize() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Check if log file exists
	if _, err := os.Stat(a.logFilePath); os.IsNotExist(err) {
		// Create new log file
		file, err := os.OpenFile(a.logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to create audit log file: %w", err)
		}
		a.file = file
		a.lastHash = ""
		a.sequence = 0
	} else {
		// Open existing log file and validate integrity
		file, err := os.OpenFile(a.logFilePath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open audit log file: %w", err)
		}
		a.file = file

		// Validate existing log and get last hash/sequence
		if err := a.validateAndLoadState(); err != nil {
			a.file.Close()
			return fmt.Errorf("audit log validation failed: %w", err)
		}
	}

	a.isInitialized = true
	return nil
}

// validateAndLoadState validates the existing audit log and loads the current state
func (a *AuditTrail) validateAndLoadState() error {
	// Open file for reading
	readFile, err := os.Open(a.logFilePath)
	if err != nil {
		return err
	}
	defer readFile.Close()

	scanner := bufio.NewScanner(readFile)
	var lastEntry *AuditEntry
	var previousHash string
	sequence := uint64(0)

	// Read and validate each entry
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // Skip empty lines
		}

		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return fmt.Errorf("invalid JSON in audit log at sequence %d: %w", sequence+1, err)
		}

		// Validate sequence
		if entry.Sequence != sequence {
			return fmt.Errorf("sequence mismatch at entry %d: expected %d, got %d",
				sequence, sequence, entry.Sequence)
		}

		// Validate previous hash
		if entry.PreviousHash != previousHash {
			return fmt.Errorf("hash chain broken at sequence %d: expected %s, got %s",
				sequence, previousHash, entry.PreviousHash)
		}

		// Validate entry hash
		expectedHash := a.calculateEntryHash(&entry)
		if entry.EntryHash != expectedHash {
			return fmt.Errorf("entry hash mismatch at sequence %d: expected %s, got %s",
				sequence, expectedHash, entry.EntryHash)
		}

		lastEntry = &entry
		previousHash = entry.EntryHash
		sequence++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading audit log: %w", err)
	}

	// Update state
	if lastEntry != nil {
		a.lastHash = lastEntry.EntryHash
	} else {
		a.lastHash = ""
	}
	a.sequence = sequence

	return nil
}

// LogComparison logs a comparison event to the audit trail
func (a *AuditTrail) LogComparison(eventType AuditEventType, data ComparisonAuditData) error {
	if !a.isInitialized {
		return errors.New("audit trail not initialized")
	}

	eventData := map[string]any{
		"comparison_id": data.ComparisonID,
		"proposal_ids":  data.ProposalIDs,
		"method":        data.Method,
	}

	if data.WinnerID != "" {
		eventData["winner_id"] = data.WinnerID
	}
	if len(data.Rankings) > 0 {
		eventData["rankings"] = data.Rankings
	}
	if data.Duration != "" {
		eventData["duration"] = data.Duration
	}
	if data.SkipReason != "" {
		eventData["skip_reason"] = data.SkipReason
	}

	return a.logEntry(eventType, eventData)
}

// LogRatingUpdate logs a rating change event to the audit trail
func (a *AuditTrail) LogRatingUpdate(data RatingAuditData) error {
	if !a.isInitialized {
		return errors.New("audit trail not initialized")
	}

	eventData := map[string]any{
		"comparison_id": data.ComparisonID,
		"proposal_id":   data.ProposalID,
		"old_rating":    data.OldRating,
		"new_rating":    data.NewRating,
		"rating_delta":  data.RatingDelta,
		"k_factor":      data.KFactor,
	}

	return a.logEntry(EventRatingUpdated, eventData)
}

// LogSessionEvent logs a session lifecycle event to the audit trail
func (a *AuditTrail) LogSessionEvent(eventType AuditEventType, metadata map[string]any) error {
	if !a.isInitialized {
		return errors.New("audit trail not initialized")
	}

	return a.logEntry(eventType, metadata)
}

// logEntry writes a new entry to the audit log
func (a *AuditTrail) logEntry(eventType AuditEventType, data map[string]any) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Create new audit entry
	entry := AuditEntry{
		ID:           a.generateEntryID(),
		Timestamp:    time.Now().UTC(),
		EventType:    eventType,
		SessionID:    a.sessionID,
		Data:         data,
		PreviousHash: a.lastHash,
		Sequence:     a.sequence,
	}

	// Calculate entry hash
	entry.EntryHash = a.calculateEntryHash(&entry)

	// Serialize to JSON Lines format
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	// Write to file
	if _, err := a.file.Write(append(jsonData, '\n')); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}

	// Ensure data is written to disk
	if err := a.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync audit log: %w", err)
	}

	// Update state
	a.lastHash = entry.EntryHash
	a.sequence++

	return nil
}

// calculateEntryHash computes the SHA-256 hash of an entry's content
func (a *AuditTrail) calculateEntryHash(entry *AuditEntry) string {
	// Create hash content (exclude EntryHash field to avoid circular dependency)
	hashContent := fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s",
		entry.ID,
		entry.Timestamp.Format(time.RFC3339Nano),
		entry.EventType,
		entry.SessionID,
		entry.PreviousHash,
		entry.Sequence,
		a.hashData(entry.Data))

	hash := sha256.Sum256([]byte(hashContent))
	return hex.EncodeToString(hash[:])
}

// hashData creates a deterministic hash of the data map
func (a *AuditTrail) hashData(data map[string]any) string {
	jsonData, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])
}

// generateEntryID creates a unique identifier for an audit entry
func (a *AuditTrail) generateEntryID() string {
	timestamp := time.Now().UnixNano()
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s_%d_%d", a.sessionID, timestamp, a.sequence)))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter IDs
}

// Close closes the audit trail and releases resources
func (a *AuditTrail) Close() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.file != nil {
		err := a.file.Close()
		a.file = nil
		a.isInitialized = false
		return err
	}

	return nil
}

// GetLogPath returns the path to the audit log file
func (a *AuditTrail) GetLogPath() string {
	return a.logFilePath
}

// GetSequence returns the current sequence number
func (a *AuditTrail) GetSequence() uint64 {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.sequence
}

// QueryOptions defines filtering criteria for audit log queries
type QueryOptions struct {
	EventTypes   []AuditEventType `json:"event_types,omitempty"`   // Filter by event types
	StartTime    *time.Time       `json:"start_time,omitempty"`    // Filter entries after this time
	EndTime      *time.Time       `json:"end_time,omitempty"`      // Filter entries before this time
	ComparisonID string           `json:"comparison_id,omitempty"` // Filter by specific comparison
	ProposalID   string           `json:"proposal_id,omitempty"`   // Filter by specific proposal
	Limit        int              `json:"limit,omitempty"`         // Maximum number of entries to return
	Offset       int              `json:"offset,omitempty"`        // Number of entries to skip
}

// QueryResult contains the results of an audit log query
type QueryResult struct {
	Entries      []AuditEntry `json:"entries"`       // Matching audit entries
	TotalCount   int          `json:"total_count"`   // Total number of matching entries
	HasMore      bool         `json:"has_more"`      // Whether there are more entries beyond the limit
	QueryOptions QueryOptions `json:"query_options"` // Original query parameters
}

// Query searches the audit log for entries matching the specified criteria
func (a *AuditTrail) Query(options QueryOptions) (*QueryResult, error) {
	if !a.isInitialized {
		return nil, errors.New("audit trail not initialized")
	}

	// Open file for reading
	readFile, err := os.Open(a.logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty result for non-existent log file
			return &QueryResult{
				Entries:      []AuditEntry{},
				TotalCount:   0,
				HasMore:      false,
				QueryOptions: options,
			}, nil
		}
		return nil, fmt.Errorf("failed to open audit log for reading: %w", err)
	}
	defer readFile.Close()

	var allMatches []AuditEntry
	scanner := bufio.NewScanner(readFile)

	// Read and filter entries
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // Skip malformed entries in query (log warning in production)
		}

		if a.matchesQuery(&entry, options) {
			allMatches = append(allMatches, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading audit log during query: %w", err)
	}

	// Apply limit and offset
	totalCount := len(allMatches)
	start := options.Offset
	if start > totalCount {
		start = totalCount
	}

	end := start + options.Limit
	if options.Limit <= 0 || end > totalCount {
		end = totalCount
	}

	resultEntries := allMatches[start:end]
	hasMore := end < totalCount

	return &QueryResult{
		Entries:      resultEntries,
		TotalCount:   totalCount,
		HasMore:      hasMore,
		QueryOptions: options,
	}, nil
}

// matchesQuery determines if an entry matches the query criteria
func (a *AuditTrail) matchesQuery(entry *AuditEntry, options QueryOptions) bool {
	// Filter by event types
	if len(options.EventTypes) > 0 {
		matches := false
		for _, eventType := range options.EventTypes {
			if entry.EventType == eventType {
				matches = true
				break
			}
		}
		if !matches {
			return false
		}
	}

	// Filter by time range
	if options.StartTime != nil && entry.Timestamp.Before(*options.StartTime) {
		return false
	}
	if options.EndTime != nil && entry.Timestamp.After(*options.EndTime) {
		return false
	}

	// Filter by comparison ID
	if options.ComparisonID != "" {
		if comparisonID, ok := entry.Data["comparison_id"].(string); !ok || comparisonID != options.ComparisonID {
			return false
		}
	}

	// Filter by proposal ID
	if options.ProposalID != "" {
		// Check if proposal ID is in proposal_ids array
		if proposalIDs, ok := entry.Data["proposal_ids"].([]any); ok {
			found := false
			for _, id := range proposalIDs {
				if strID, ok := id.(string); ok && strID == options.ProposalID {
					found = true
					break
				}
			}
			if !found {
				// Also check if it's the specific proposal_id in rating updates
				if proposalID, ok := entry.Data["proposal_id"].(string); !ok || proposalID != options.ProposalID {
					return false
				}
			}
		} else if proposalID, ok := entry.Data["proposal_id"].(string); !ok || proposalID != options.ProposalID {
			return false
		}
	}

	return true
}

// GetComparisonHistory retrieves the complete history for a specific comparison
func (a *AuditTrail) GetComparisonHistory(comparisonID string) ([]AuditEntry, error) {
	result, err := a.Query(QueryOptions{
		ComparisonID: comparisonID,
	})
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// GetProposalHistory retrieves all audit entries related to a specific proposal
func (a *AuditTrail) GetProposalHistory(proposalID string) ([]AuditEntry, error) {
	result, err := a.Query(QueryOptions{
		ProposalID: proposalID,
	})
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// GetSessionHistory retrieves all audit entries for the session
func (a *AuditTrail) GetSessionHistory() ([]AuditEntry, error) {
	result, err := a.Query(QueryOptions{})
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// VerifyIntegrity performs a complete integrity check of the audit log
func (a *AuditTrail) VerifyIntegrity() error {
	if !a.isInitialized {
		return errors.New("audit trail not initialized")
	}

	// This uses the same validation logic as validateAndLoadState
	// but doesn't update internal state
	readFile, err := os.Open(a.logFilePath)
	if err != nil {
		return fmt.Errorf("failed to open audit log for verification: %w", err)
	}
	defer readFile.Close()

	scanner := bufio.NewScanner(readFile)
	var previousHash string
	sequence := uint64(0)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return fmt.Errorf("integrity check failed: invalid JSON at sequence %d: %w", sequence, err)
		}

		// Verify sequence
		if entry.Sequence != sequence {
			return fmt.Errorf("integrity check failed: sequence mismatch at entry %d", sequence)
		}

		// Verify previous hash
		if entry.PreviousHash != previousHash {
			return fmt.Errorf("integrity check failed: hash chain broken at sequence %d", sequence)
		}

		// Verify entry hash
		expectedHash := a.calculateEntryHash(&entry)
		if entry.EntryHash != expectedHash {
			return fmt.Errorf("integrity check failed: entry hash mismatch at sequence %d", sequence)
		}

		previousHash = entry.EntryHash
		sequence++
	}

	return scanner.Err()
}

// GetStatistics returns statistics about the audit log
func (a *AuditTrail) GetStatistics() (*AuditStatistics, error) {
	if !a.isInitialized {
		return nil, errors.New("audit trail not initialized")
	}

	stats := &AuditStatistics{
		SessionID:    a.sessionID,
		TotalEntries: 0,
		EventCounts:  make(map[AuditEventType]int),
		LastUpdated:  time.Now().UTC(),
	}

	result, err := a.Query(QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to generate statistics: %w", err)
	}

	stats.TotalEntries = result.TotalCount

	if len(result.Entries) > 0 {
		stats.FirstEntry = &result.Entries[0].Timestamp
		stats.LastEntry = &result.Entries[len(result.Entries)-1].Timestamp
	}

	// Count events by type
	for _, entry := range result.Entries {
		stats.EventCounts[entry.EventType]++
	}

	return stats, nil
}

// AuditStatistics provides summary information about the audit log
type AuditStatistics struct {
	SessionID    string                 `json:"session_id"`
	TotalEntries int                    `json:"total_entries"`
	EventCounts  map[AuditEventType]int `json:"event_counts"`
	FirstEntry   *time.Time             `json:"first_entry,omitempty"`
	LastEntry    *time.Time             `json:"last_entry,omitempty"`
	LastUpdated  time.Time              `json:"last_updated"`
}
