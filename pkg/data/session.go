// Package data provides session management functionality for conference talk ranking.
// It implements session state persistence, comparison tracking, and integration
// with the Elo rating engine for seamless rating updates and convergence tracking.
package data

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Error types for session management
var (
	ErrSessionNotFound       = errors.New("session not found")
	ErrInvalidSessionState   = errors.New("invalid session state")
	ErrSessionCorrupted      = errors.New("session data corrupted")
	ErrComparisonNotActive   = errors.New("no active comparison")
	ErrInvalidComparison     = errors.New("invalid comparison data")
	ErrAtomicOperationFailed = errors.New("atomic operation failed")
	ErrModeDetectionFailed   = errors.New("session mode detection failed")
	ErrSessionNameInvalid    = errors.New("session name contains invalid characters")
)

// SessionMode represents the detected operational mode for automatic mode detection
type SessionMode int

const (
	// StartMode indicates a new session should be created
	StartMode SessionMode = iota
	// ResumeMode indicates an existing session should be resumed
	ResumeMode
)

// String returns a string representation of the SessionMode
func (sm SessionMode) String() string {
	switch sm {
	case StartMode:
		return "Start"
	case ResumeMode:
		return "Resume"
	default:
		return "Unknown"
	}
}

// SessionStatus represents the current state of a ranking session
type SessionStatus string

const (
	StatusCreated  SessionStatus = "created"
	StatusActive   SessionStatus = "active"
	StatusPaused   SessionStatus = "paused"
	StatusComplete SessionStatus = "complete"
)

// ComparisonMethod represents the type of comparison being performed
type ComparisonMethod string

const (
	MethodPairwise ComparisonMethod = "pairwise"
	MethodTrio     ComparisonMethod = "trio"
	MethodQuartet  ComparisonMethod = "quartet"
)

// Session manages the complete ranking workflow and persistent state
type Session struct {
	// Core identity
	ID        string        `json:"id"`         // Unique session identifier
	Name      string        `json:"name"`       // Human-readable session name
	Status    SessionStatus `json:"status"`     // Current session state
	CreatedAt time.Time     `json:"created_at"` // Session creation timestamp
	UpdatedAt time.Time     `json:"updated_at"` // Last modification timestamp

	// Configuration and data
	Config         SessionConfig      `json:"config"`                    // Session configuration
	Proposals      []Proposal         `json:"-"`                         // Collection of proposals (reloaded from CSV, never serialized)
	ProposalScores map[string]float64 `json:"proposal_scores"`           // Current scores by ID (lightweight persistence)
	ProposalIndex  map[string]int     `json:"-"`                         // Fast ID lookup (not serialized)
	InputCSVPath   string             `json:"input_csv_path"`            // Original input CSV file path for export

	// Comparison tracking (lightweight persistence for progress/confidence)
	ComparisonCounts    map[string]int `json:"comparison_counts"`     // Per-proposal comparison count for confidence
	TotalComparisons    int            `json:"total_comparisons"`     // Total comparisons performed for progress
	CurrentComparison   *ComparisonState `json:"-"`                   // Active comparison state (not persisted)
	CompletedComparisons []Comparison    `json:"-"`                   // Historical comparisons (not persisted)

	// Analytics and optimization
	ConvergenceMetrics *ConvergenceMetrics `json:"convergence_metrics"` // Progress tracking
	MatchupHistory     []MatchupHistory    `json:"matchup_history"`     // Pairing optimization data
	RatingBins         []RatingBin         `json:"rating_bins"`         // Strategic grouping

	// Internal state management
	mutex            sync.RWMutex `json:"-"` // Thread safety (not serialized)
	storageDirectory string       `json:"-"` // Where to persist session
}

// ComparisonState represents the current active comparison
type ComparisonState struct {
	ID             string           `json:"id"`              // Unique comparison identifier
	ProposalIDs    []string         `json:"proposal_ids"`    // Proposals being compared
	Method         ComparisonMethod `json:"method"`          // Comparison type
	StartedAt      time.Time        `json:"started_at"`      // When comparison began
	PresentedOrder []string         `json:"presented_order"` // Order shown to user (for consistency)
}

// Comparison records a completed evaluation event between proposals
type Comparison struct {
	ID          string           `json:"id"`           // Unique comparison identifier
	SessionID   string           `json:"session_id"`   // Parent session reference
	ProposalIDs []string         `json:"proposal_ids"` // Proposals that were compared
	WinnerID    string           `json:"winner_id"`    // Selected best proposal ID (empty if skipped)
	Rankings    []string         `json:"rankings"`     // Full ranking order for multi-proposal (optional)
	Method      ComparisonMethod `json:"method"`       // Comparison type
	Timestamp   time.Time        `json:"timestamp"`    // When comparison was completed
	Duration    time.Duration    `json:"duration"`     // Time spent on comparison
	Skipped     bool             `json:"skipped"`      // Whether comparison was skipped
	SkipReason  string           `json:"skip_reason"`  // Why comparison was skipped (optional)
	EloUpdates  []EloUpdate      `json:"elo_updates"`  // Rating changes from this comparison
}

// EloUpdate records rating changes from a single comparison
type EloUpdate struct {
	ID           string  `json:"id"`            // Unique update identifier
	ComparisonID string  `json:"comparison_id"` // Parent comparison
	ProposalID   string  `json:"proposal_id"`   // Affected proposal
	OldRating    float64 `json:"old_rating"`    // Rating before comparison
	NewRating    float64 `json:"new_rating"`    // Rating after comparison
	RatingDelta  float64 `json:"rating_delta"`  // Change amount (NewRating - OldRating)
	KFactor      int     `json:"k_factor"`      // K-factor used for this calculation
}

// ConvergenceMetrics tracks session progress and convergence indicators
type ConvergenceMetrics struct {
	SessionID           string    `json:"session_id"`            // Parent session identifier
	TotalComparisons    int       `json:"total_comparisons"`     // Number of comparisons performed
	AvgRatingChange     float64   `json:"avg_rating_change"`     // Rolling average of rating changes
	RatingVariance      float64   `json:"rating_variance"`       // Variance in recent rating changes
	RankingStability    float64   `json:"ranking_stability"`     // Percentage of stable top-N positions
	CoveragePercentage  float64   `json:"coverage_percentage"`   // Percentage of meaningful pairs compared
	ConvergenceScore    float64   `json:"convergence_score"`     // Overall convergence indicator 0-1
	LastCalculated      time.Time `json:"last_calculated"`       // When metrics were last updated
	RecentRatingChanges []float64 `json:"recent_rating_changes"` // Last N rating changes for variance calc
}

// MatchupHistory tracks comparison pairings to optimize future matchup selection
type MatchupHistory struct {
	SessionID               string    `json:"session_id"`                // Parent session identifier
	ProposalA               string    `json:"proposal_a"`                // First proposal ID
	ProposalB               string    `json:"proposal_b"`                // Second proposal ID
	ComparisonCount         int       `json:"comparison_count"`          // Times this pair has been compared
	LastCompared            time.Time `json:"last_compared"`             // Most recent comparison timestamp
	RatingDifferenceHistory []float64 `json:"rating_difference_history"` // Rating gaps at each comparison
	InformationGain         float64   `json:"information_gain"`          // Measured impact on ranking stability
}

// RatingBin groups proposals by rating ranges for strategic matchup selection
type RatingBin struct {
	SessionID   string    `json:"session_id"`   // Parent session identifier
	BinIndex    int       `json:"bin_index"`    // Numeric bin identifier
	MinRating   float64   `json:"min_rating"`   // Lower bound of rating range
	MaxRating   float64   `json:"max_rating"`   // Upper bound of rating range
	ProposalIDs []string  `json:"proposal_ids"` // Proposals currently in this bin
	LastUpdated time.Time `json:"last_updated"` // When bin assignments were recalculated
}

// NewSession creates a new ranking session with the given proposals and configuration
// inputCSVPath is required - it's used to reload proposals when resuming the session
func NewSession(name string, proposals []Proposal, config SessionConfig, inputCSVPath string) (*Session, error) {
	if len(proposals) < 2 {
		return nil, fmt.Errorf("%w: session must contain at least 2 proposals", ErrInvalidSessionState)
	}

	if name == "" {
		return nil, fmt.Errorf("%w: session name is required", ErrRequiredField)
	}

	if inputCSVPath == "" {
		return nil, fmt.Errorf("%w: input CSV path is required", ErrRequiredField)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session configuration: %w", err)
	}

	// Generate unique session ID
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	now := time.Now()

	// Build proposal index for fast lookups
	proposalIndex := make(map[string]int, len(proposals))
	for i, proposal := range proposals {
		proposalIndex[proposal.ID] = i
	}

	// Initialize convergence metrics
	convergenceMetrics := &ConvergenceMetrics{
		SessionID:           sessionID,
		TotalComparisons:    0,
		AvgRatingChange:     0.0,
		RatingVariance:      0.0,
		RankingStability:    0.0,
		CoveragePercentage:  0.0,
		ConvergenceScore:    0.0,
		LastCalculated:      now,
		RecentRatingChanges: make([]float64, 0, 10), // Keep last 10 changes for variance
	}

	session := &Session{
		ID:                   sessionID,
		Name:                 name,
		Status:               StatusCreated,
		CreatedAt:            now,
		UpdatedAt:            now,
		Config:               config,
		Proposals:            proposals,
		ProposalIndex:        proposalIndex,
		InputCSVPath:         inputCSVPath, // Store CSV path for reload on resume
		ComparisonCounts:     make(map[string]int),
		TotalComparisons:     0,
		CurrentComparison:    nil,
		CompletedComparisons: make([]Comparison, 0),
		ConvergenceMetrics:   convergenceMetrics,
		MatchupHistory:       make([]MatchupHistory, 0),
		RatingBins:           make([]RatingBin, 0),
		storageDirectory:     "./sessions", // Default storage directory
	}

	// Initialize rating bins
	session.updateRatingBins()

	return session, nil
}

// generateSessionID creates a unique identifier for the session
func generateSessionID() (string, error) {
	// Generate timestamp-based prefix for readability
	timestamp := time.Now().Format("20060102_150405")

	// Add random suffix for uniqueness
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	randomSuffix := hex.EncodeToString(randomBytes)

	return fmt.Sprintf("session_%s_%s", timestamp, randomSuffix), nil
}

// LoadSession loads an existing session from the storage directory
// This is a convenience function that uses FileStorage internally
func LoadSession(sessionID string, storageDir string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session ID is required", ErrRequiredField)
	}

	sessionFile := filepath.Join(storageDir, sessionID+".json")

	// Check if session file exists
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: session %s", ErrSessionNotFound, sessionID)
	}

	// Use FileStorage to load session properly (handles proposal reloading from CSV)
	storage := NewFileStorage(filepath.Join(storageDir, "backups"))
	session, err := storage.LoadSession(sessionFile)
	if err != nil {
		return nil, err
	}

	// Set storage directory
	session.storageDirectory = storageDir

	// Validate loaded session
	if err := session.validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSessionCorrupted, err)
	}

	return session, nil
}

// validate performs integrity checks on the loaded session
func (s *Session) validate() error {
	// Check basic fields
	if s.ID == "" {
		return errors.New("session ID is empty")
	}
	if s.Name == "" {
		return errors.New("session name is empty")
	}
	if len(s.Proposals) < 2 {
		return errors.New("session must have at least 2 proposals")
	}

	// Validate proposal index consistency
	if len(s.ProposalIndex) != len(s.Proposals) {
		return errors.New("proposal index size mismatch")
	}

	for i, proposal := range s.Proposals {
		if idx, exists := s.ProposalIndex[proposal.ID]; !exists || idx != i {
			return fmt.Errorf("proposal index inconsistency for ID: %s", proposal.ID)
		}
	}

	// Validate configuration
	if err := s.Config.Validate(); err != nil {
		return fmt.Errorf("invalid session configuration: %w", err)
	}

	return nil
}

// Save persists the session to storage using atomic operations
func (s *Session) Save() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Update timestamp
	s.UpdatedAt = time.Now()

	// Ensure storage directory exists
	if err := os.MkdirAll(s.storageDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	sessionFile := filepath.Join(s.storageDirectory, s.ID+".json")
	tempFile := sessionFile + ".tmp"

	// Serialize to JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	// Write to temporary file first (atomic operation)
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary session file: %w", err)
	}

	// Atomically replace the original file
	if err := os.Rename(tempFile, sessionFile); err != nil {
		// Clean up temp file on failure
		os.Remove(tempFile)
		return fmt.Errorf("%w: failed to replace session file", ErrAtomicOperationFailed)
	}

	return nil
}

// SetStorageDirectory configures where the session should be persisted
func (s *Session) SetStorageDirectory(dir string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.storageDirectory = dir
}

// GetStatus returns the current session status (thread-safe)
func (s *Session) GetStatus() SessionStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.Status
}

// GetProposalCount returns the number of proposals in the session
func (s *Session) GetProposalCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.Proposals)
}

// GetProposalByID retrieves a proposal by its ID
func (s *Session) GetProposalByID(id string) (*Proposal, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	idx, exists := s.ProposalIndex[id]
	if !exists {
		return nil, fmt.Errorf("proposal not found: %s", id)
	}

	// Return a copy to prevent external modifications
	proposal := s.Proposals[idx]
	return &proposal, nil
}

// GetProposals returns a copy of all proposals (thread-safe)
func (s *Session) GetProposals() []Proposal {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a deep copy to prevent external modifications
	proposals := make([]Proposal, len(s.Proposals))
	copy(proposals, s.Proposals)
	return proposals
}

// updateRatingBins recalculates rating bin assignments based on current proposal ratings
func (s *Session) updateRatingBins() {
	// This is a placeholder implementation - will be fully implemented in integration task
	// For now, just initialize empty bins
	s.RatingBins = make([]RatingBin, 0)
	// TODO: Implement intelligent binning algorithm based on rating distribution
}

// StartComparison initiates a new comparison with the specified proposals
func (s *Session) StartComparison(proposalIDs []string, method ComparisonMethod) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Validate input
	if len(proposalIDs) < 2 {
		return fmt.Errorf("%w: comparison requires at least 2 proposals", ErrInvalidComparison)
	}

	// Validate comparison method matches proposal count
	expectedCount := 2
	switch method {
	case MethodPairwise:
		expectedCount = 2
	case MethodTrio:
		expectedCount = 3
	case MethodQuartet:
		expectedCount = 4
	default:
		return fmt.Errorf("%w: unknown comparison method: %s", ErrInvalidComparison, method)
	}

	if len(proposalIDs) != expectedCount {
		return fmt.Errorf("%w: method %s requires exactly %d proposals, got %d",
			ErrInvalidComparison, method, expectedCount, len(proposalIDs))
	}

	// Verify all proposals exist
	for _, id := range proposalIDs {
		if _, exists := s.ProposalIndex[id]; !exists {
			return fmt.Errorf("%w: proposal not found: %s", ErrInvalidComparison, id)
		}
	}

	// Check if there's already an active comparison
	if s.CurrentComparison != nil {
		return fmt.Errorf("%w: comparison already in progress", ErrInvalidSessionState)
	}

	// Generate comparison ID
	comparisonID, err := generateComparisonID()
	if err != nil {
		return fmt.Errorf("failed to generate comparison ID: %w", err)
	}

	// Create comparison state
	s.CurrentComparison = &ComparisonState{
		ID:             comparisonID,
		ProposalIDs:    make([]string, len(proposalIDs)),
		Method:         method,
		StartedAt:      time.Now(),
		PresentedOrder: make([]string, len(proposalIDs)),
	}

	// Copy proposal IDs and create presentation order
	copy(s.CurrentComparison.ProposalIDs, proposalIDs)
	copy(s.CurrentComparison.PresentedOrder, proposalIDs)
	// TODO: Add randomization of presentation order to avoid bias

	// Update session status
	if s.Status == StatusCreated {
		s.Status = StatusActive
	}

	s.UpdatedAt = time.Now()

	// Note: Autosave disabled for now to avoid mutex deadlock issues
	// TODO: Implement proper autosave mechanism

	return nil
}

// CompleteComparison finishes the current comparison with the specified result
// Automatically saves the session after completion
func (s *Session) CompleteComparison(winnerID string, rankings []string, skipped bool, skipReason string) (*Comparison, error) {
	s.mutex.Lock()
	comparison, err := s.completeComparisonInternal(winnerID, rankings, skipped, skipReason)
	s.mutex.Unlock()

	// Auto-save after each comparison (outside mutex to avoid deadlock)
	if err == nil && s.storageDirectory != "" {
		if saveErr := s.Save(); saveErr != nil {
			// Log warning but don't fail the comparison
			fmt.Printf("Warning: failed to auto-save session after comparison: %v\n", saveErr)
		}
	}

	return comparison, err
}

// completeComparisonInternal performs comparison completion without acquiring mutex (internal use)
func (s *Session) completeComparisonInternal(winnerID string, rankings []string, skipped bool, skipReason string) (*Comparison, error) {
	// Check if there's an active comparison
	if s.CurrentComparison == nil {
		return nil, ErrComparisonNotActive
	}

	comparison := &Comparison{
		ID:          s.CurrentComparison.ID,
		SessionID:   s.ID,
		ProposalIDs: make([]string, len(s.CurrentComparison.ProposalIDs)),
		WinnerID:    winnerID,
		Rankings:    rankings,
		Method:      s.CurrentComparison.Method,
		Timestamp:   time.Now(),
		Duration:    time.Since(s.CurrentComparison.StartedAt),
		Skipped:     skipped,
		SkipReason:  skipReason,
		EloUpdates:  make([]EloUpdate, 0),
	}

	copy(comparison.ProposalIDs, s.CurrentComparison.ProposalIDs)

	// Validate result if not skipped
	if !skipped {
		if winnerID == "" {
			return nil, fmt.Errorf("%w: winner ID required for non-skipped comparison", ErrInvalidComparison)
		}

		// Verify winner is in the comparison
		winnerFound := false
		for _, id := range comparison.ProposalIDs {
			if id == winnerID {
				winnerFound = true
				break
			}
		}
		if !winnerFound {
			return nil, fmt.Errorf("%w: winner ID not in comparison proposals", ErrInvalidComparison)
		}

		// Validate rankings if provided (for multi-proposal comparisons)
		if rankings != nil {
			if len(rankings) != len(comparison.ProposalIDs) {
				return nil, fmt.Errorf("%w: rankings length must match proposal count", ErrInvalidComparison)
			}

			// Verify all proposals are in rankings
			rankingSet := make(map[string]bool)
			for _, id := range rankings {
				if rankingSet[id] {
					return nil, fmt.Errorf("%w: duplicate proposal in rankings: %s", ErrInvalidComparison, id)
				}
				rankingSet[id] = true
			}

			for _, id := range comparison.ProposalIDs {
				if !rankingSet[id] {
					return nil, fmt.Errorf("%w: proposal missing from rankings: %s", ErrInvalidComparison, id)
				}
			}
		}
	}

	// Add to completed comparisons
	s.CompletedComparisons = append(s.CompletedComparisons, *comparison)

	// Clear current comparison
	s.CurrentComparison = nil

	// Update convergence metrics
	s.updateConvergenceMetrics()

	s.UpdatedAt = time.Now()

	// Note: Autosave disabled for now to avoid mutex deadlock issues
	// TODO: Implement proper autosave mechanism

	return comparison, nil
}

// CancelComparison aborts the current active comparison
func (s *Session) CancelComparison() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.CurrentComparison == nil {
		return ErrComparisonNotActive
	}

	s.CurrentComparison = nil
	s.UpdatedAt = time.Now()

	return nil
}

// PauseSession pauses the ranking session
func (s *Session) PauseSession() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.Status != StatusActive {
		return fmt.Errorf("%w: can only pause active sessions", ErrInvalidSessionState)
	}

	// Cancel any active comparison
	s.CurrentComparison = nil
	s.Status = StatusPaused

	s.UpdatedAt = time.Now()

	// Force save when pausing
	return s.saveInternal()
}

// ResumeSession resumes a paused ranking session
func (s *Session) ResumeSession() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.Status != StatusPaused {
		return fmt.Errorf("%w: can only resume paused sessions", ErrInvalidSessionState)
	}

	s.Status = StatusActive

	s.UpdatedAt = time.Now()

	return nil
}

// CompleteSession marks the session as complete
func (s *Session) CompleteSession() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Cancel any active comparison
	s.CurrentComparison = nil
	s.Status = StatusComplete

	s.UpdatedAt = time.Now()

	// Force save when completing
	return s.saveInternal()
}

// GetCurrentComparison returns the current active comparison (thread-safe copy)
func (s *Session) GetCurrentComparison() *ComparisonState {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.CurrentComparison == nil {
		return nil
	}

	// Return a deep copy to prevent external modifications
	comparison := &ComparisonState{
		ID:             s.CurrentComparison.ID,
		ProposalIDs:    make([]string, len(s.CurrentComparison.ProposalIDs)),
		Method:         s.CurrentComparison.Method,
		StartedAt:      s.CurrentComparison.StartedAt,
		PresentedOrder: make([]string, len(s.CurrentComparison.PresentedOrder)),
	}

	copy(comparison.ProposalIDs, s.CurrentComparison.ProposalIDs)
	copy(comparison.PresentedOrder, s.CurrentComparison.PresentedOrder)

	return comparison
}

// GetComparisonHistory returns all completed comparisons (thread-safe copy)
func (s *Session) GetComparisonHistory() []Comparison {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a deep copy to prevent external modifications
	history := make([]Comparison, len(s.CompletedComparisons))
	copy(history, s.CompletedComparisons)
	return history
}

// GetConvergenceMetrics returns current convergence metrics (thread-safe copy)
func (s *Session) GetConvergenceMetrics() *ConvergenceMetrics {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.ConvergenceMetrics == nil {
		return nil
	}

	// Return a copy to prevent external modifications
	metrics := *s.ConvergenceMetrics
	metrics.RecentRatingChanges = make([]float64, len(s.ConvergenceMetrics.RecentRatingChanges))
	copy(metrics.RecentRatingChanges, s.ConvergenceMetrics.RecentRatingChanges)

	return &metrics
}

// generateComparisonID creates a unique identifier for a comparison
func generateComparisonID() (string, error) {
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("comp_%s", hex.EncodeToString(randomBytes)), nil
}

// saveInternal performs internal save without locking (caller must hold mutex)
func (s *Session) saveInternal() error {
	// Update timestamp
	s.UpdatedAt = time.Now()

	// Ensure storage directory exists
	if err := os.MkdirAll(s.storageDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	sessionFile := filepath.Join(s.storageDirectory, s.ID+".json")
	tempFile := sessionFile + ".tmp"

	// Serialize to JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	// Write to temporary file first (atomic operation)
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary session file: %w", err)
	}

	// Atomically replace the original file
	if err := os.Rename(tempFile, sessionFile); err != nil {
		// Clean up temp file on failure
		os.Remove(tempFile)
		return fmt.Errorf("%w: failed to replace session file", ErrAtomicOperationFailed)
	}

	return nil
}

// updateConvergenceMetrics recalculates convergence indicators
func (s *Session) updateConvergenceMetrics() {
	if s.ConvergenceMetrics == nil {
		return
	}

	s.ConvergenceMetrics.TotalComparisons = len(s.CompletedComparisons)
	s.ConvergenceMetrics.LastCalculated = time.Now()

	// Perform full convergence calculations
	s.calculateConvergenceMetrics()
}

// AddEloUpdate records a rating change from a comparison
func (s *Session) AddEloUpdate(comparisonID, proposalID string, oldRating, newRating float64, kFactor int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Find the comparison
	var targetComparison *Comparison
	for i := range s.CompletedComparisons {
		if s.CompletedComparisons[i].ID == comparisonID {
			targetComparison = &s.CompletedComparisons[i]
			break
		}
	}

	if targetComparison == nil {
		return fmt.Errorf("comparison not found: %s", comparisonID)
	}

	// Verify proposal is part of this comparison
	proposalFound := false
	for _, id := range targetComparison.ProposalIDs {
		if id == proposalID {
			proposalFound = true
			break
		}
	}
	if !proposalFound {
		return fmt.Errorf("proposal %s not part of comparison %s", proposalID, comparisonID)
	}

	// Generate update ID
	updateID, err := generateUpdateID()
	if err != nil {
		return fmt.Errorf("failed to generate update ID: %w", err)
	}

	// Create EloUpdate
	eloUpdate := EloUpdate{
		ID:           updateID,
		ComparisonID: comparisonID,
		ProposalID:   proposalID,
		OldRating:    oldRating,
		NewRating:    newRating,
		RatingDelta:  newRating - oldRating,
		KFactor:      kFactor,
	}

	// Add to comparison's update list
	targetComparison.EloUpdates = append(targetComparison.EloUpdates, eloUpdate)

	// Update the proposal's rating
	if idx, exists := s.ProposalIndex[proposalID]; exists {
		s.Proposals[idx].Score = newRating
		s.Proposals[idx].UpdatedAt = time.Now()
	}

	// Update convergence metrics with new rating change
	if s.ConvergenceMetrics != nil {
		// Add to recent rating changes (keep last 10)
		s.ConvergenceMetrics.RecentRatingChanges = append(s.ConvergenceMetrics.RecentRatingChanges, eloUpdate.RatingDelta)
		if len(s.ConvergenceMetrics.RecentRatingChanges) > 10 {
			s.ConvergenceMetrics.RecentRatingChanges = s.ConvergenceMetrics.RecentRatingChanges[1:]
		}

		// Recalculate metrics
		s.calculateConvergenceMetrics()
	}

	// Update rating bins
	s.updateRatingBins()

	s.UpdatedAt = time.Now()

	return nil
}

// UpdateProposalRating directly updates a proposal's rating (used by Elo engine)
func (s *Session) UpdateProposalRating(proposalID string, newRating float64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	idx, exists := s.ProposalIndex[proposalID]
	if !exists {
		return fmt.Errorf("proposal not found: %s", proposalID)
	}

	s.Proposals[idx].Score = newRating
	s.Proposals[idx].UpdatedAt = time.Now()
	s.UpdatedAt = time.Now()

	return nil
}

// RecordMatchup tracks a pairwise comparison for optimization purposes
func (s *Session) RecordMatchup(proposalA, proposalB string, informationGain float64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.recordMatchupInternal(proposalA, proposalB, informationGain)
}

// recordMatchupInternal tracks a pairwise comparison without acquiring mutex (internal use)
func (s *Session) recordMatchupInternal(proposalA, proposalB string, informationGain float64) {
	// Ensure consistent ordering (A < B alphabetically)
	if proposalA > proposalB {
		proposalA, proposalB = proposalB, proposalA
	}

	// Find existing matchup history
	var matchup *MatchupHistory
	for i := range s.MatchupHistory {
		if s.MatchupHistory[i].ProposalA == proposalA && s.MatchupHistory[i].ProposalB == proposalB {
			matchup = &s.MatchupHistory[i]
			break
		}
	}

	// Create new matchup history if not found
	if matchup == nil {
		s.MatchupHistory = append(s.MatchupHistory, MatchupHistory{
			SessionID:               s.ID,
			ProposalA:               proposalA,
			ProposalB:               proposalB,
			ComparisonCount:         0,
			LastCompared:            time.Time{},
			RatingDifferenceHistory: make([]float64, 0),
			InformationGain:         0.0,
		})
		matchup = &s.MatchupHistory[len(s.MatchupHistory)-1]
	}

	// Update matchup record
	matchup.ComparisonCount++
	matchup.LastCompared = time.Now()
	matchup.InformationGain = informationGain

	// Calculate current rating difference
	ratingA := s.getProposalRating(proposalA)
	ratingB := s.getProposalRating(proposalB)
	ratingDiff := ratingA - ratingB
	if ratingDiff < 0 {
		ratingDiff = -ratingDiff
	}

	matchup.RatingDifferenceHistory = append(matchup.RatingDifferenceHistory, ratingDiff)
}

// getProposalRating gets the current rating for a proposal (internal, no locking)
func (s *Session) getProposalRating(proposalID string) float64 {
	if idx, exists := s.ProposalIndex[proposalID]; exists {
		return s.Proposals[idx].Score
	}
	return s.Config.Elo.InitialRating // Default rating if not found
}

// GetMatchupHistory returns matchup optimization data (thread-safe copy)
func (s *Session) GetMatchupHistory() []MatchupHistory {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a deep copy to prevent external modifications
	history := make([]MatchupHistory, len(s.MatchupHistory))
	for i, matchup := range s.MatchupHistory {
		history[i] = MatchupHistory{
			SessionID:       matchup.SessionID,
			ProposalA:       matchup.ProposalA,
			ProposalB:       matchup.ProposalB,
			ComparisonCount: matchup.ComparisonCount,
			LastCompared:    matchup.LastCompared,
			InformationGain: matchup.InformationGain,
		}
		// Deep copy rating difference history
		history[i].RatingDifferenceHistory = make([]float64, len(matchup.RatingDifferenceHistory))
		copy(history[i].RatingDifferenceHistory, matchup.RatingDifferenceHistory)
	}

	return history
}

// calculateConvergenceMetrics performs detailed convergence analysis
func (s *Session) calculateConvergenceMetrics() {
	if s.ConvergenceMetrics == nil {
		return
	}

	metrics := s.ConvergenceMetrics

	// Calculate average rating change from recent changes
	if len(metrics.RecentRatingChanges) > 0 {
		sum := 0.0
		for _, change := range metrics.RecentRatingChanges {
			if change < 0 {
				change = -change
			}
			sum += change
		}
		metrics.AvgRatingChange = sum / float64(len(metrics.RecentRatingChanges))

		// Calculate variance
		if len(metrics.RecentRatingChanges) > 1 {
			variance := 0.0
			for _, change := range metrics.RecentRatingChanges {
				if change < 0 {
					change = -change
				}
				diff := change - metrics.AvgRatingChange
				variance += diff * diff
			}
			metrics.RatingVariance = variance / float64(len(metrics.RecentRatingChanges)-1)
		}
	}

	// Calculate coverage percentage (simplified)
	totalPossiblePairs := len(s.Proposals) * (len(s.Proposals) - 1) / 2
	uniquePairs := len(s.MatchupHistory)
	if totalPossiblePairs > 0 {
		metrics.CoveragePercentage = float64(uniquePairs) / float64(totalPossiblePairs) * 100.0
	}

	// Calculate convergence score (0-1)
	// Simple heuristic: high coverage + low variance = high convergence
	coverageFactor := metrics.CoveragePercentage / 100.0
	varianceFactor := 1.0
	if metrics.RatingVariance > 0 {
		varianceFactor = 1.0 / (1.0 + metrics.RatingVariance/10.0) // Normalize variance impact
	}

	metrics.ConvergenceScore = (coverageFactor + varianceFactor) / 2.0
	if metrics.ConvergenceScore > 1.0 {
		metrics.ConvergenceScore = 1.0
	}

	// TODO: Implement more sophisticated ranking stability calculation
	metrics.RankingStability = 0.0 // Placeholder
}

// generateUpdateID creates a unique identifier for an Elo update
func generateUpdateID() (string, error) {
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("upd_%s", hex.EncodeToString(randomBytes)), nil
}

// ListSessions returns all available session IDs in the storage directory
func ListSessions(storageDir string) ([]string, error) {
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		return make([]string, 0), nil
	}

	entries, err := os.ReadDir(storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}

	sessions := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			sessionID := strings.TrimSuffix(entry.Name(), ".json")
			sessions = append(sessions, sessionID)
		}
	}

	return sessions, nil
}

// BackupSession creates a backup copy of the session file
func (s *Session) BackupSession() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sessionFile := filepath.Join(s.storageDirectory, s.ID+".json")
	backupFile := filepath.Join(s.storageDirectory, fmt.Sprintf("%s_backup_%d.json", s.ID, time.Now().Unix()))

	// Check if original file exists
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		return fmt.Errorf("session file not found: %s", sessionFile)
	}

	// Read original file
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return fmt.Errorf("failed to read session file for backup: %w", err)
	}

	// Write backup file
	if err := os.WriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}

	return nil
}

// RecoverFromBackup attempts to recover a session from its most recent backup
func RecoverFromBackup(sessionID, storageDir string) (*Session, error) {
	// Find backup files for this session
	pattern := filepath.Join(storageDir, sessionID+"_backup_*.json")
	backups, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for backup files: %w", err)
	}

	if len(backups) == 0 {
		return nil, fmt.Errorf("no backup files found for session: %s", sessionID)
	}

	// Sort backups to get the most recent
	sort.Strings(backups)
	mostRecentBackup := backups[len(backups)-1]

	// Try to load from backup
	data, err := os.ReadFile(mostRecentBackup)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}

	// Parse JSON
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("backup file is also corrupted: %w", err)
	}

	// Rebuild proposal index
	session.ProposalIndex = make(map[string]int, len(session.Proposals))
	for i, proposal := range session.Proposals {
		session.ProposalIndex[proposal.ID] = i
	}

	// Set storage directory
	session.storageDirectory = storageDir

	// Validate recovered session
	if err := session.validate(); err != nil {
		return nil, fmt.Errorf("recovered session is invalid: %w", err)
	}

	return &session, nil
}

// ValidateSessionFile checks if a session file is valid without fully loading it
func ValidateSessionFile(sessionID, storageDir string) error {
	sessionFile := filepath.Join(storageDir, sessionID+".json")

	// Check if file exists
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		return fmt.Errorf("%w: session file not found", ErrSessionNotFound)
	}

	// Read and parse file
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return fmt.Errorf("failed to read session file: %w", err)
	}

	// Basic JSON validation
	var rawSession map[string]any
	if err := json.Unmarshal(data, &rawSession); err != nil {
		return fmt.Errorf("%w: invalid JSON format", ErrSessionCorrupted)
	}

	// Check required fields
	requiredFields := []string{"id", "name", "status", "created_at", "proposals", "config"}
	for _, field := range requiredFields {
		if _, exists := rawSession[field]; !exists {
			return fmt.Errorf("%w: missing required field: %s", ErrSessionCorrupted, field)
		}
	}

	return nil
}

// CleanupBackups removes old backup files, keeping only the most recent N backups
func CleanupBackups(sessionID, storageDir string, keepCount int) error {
	if keepCount <= 0 {
		return fmt.Errorf("keep count must be positive")
	}

	// Find backup files for this session
	pattern := filepath.Join(storageDir, sessionID+"_backup_*.json")
	backups, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to search for backup files: %w", err)
	}

	if len(backups) <= keepCount {
		return nil // Nothing to clean up
	}

	// Sort backups by name (which includes timestamp)
	sort.Strings(backups)

	// Remove oldest backups
	toRemove := backups[:len(backups)-keepCount]
	for _, backup := range toRemove {
		if err := os.Remove(backup); err != nil {
			// Log warning but continue cleanup
			// TODO: Add proper logging
		}
	}

	return nil
}

// GetSessionInfo returns basic session information without loading the full session
func GetSessionInfo(sessionID, storageDir string) (*SessionInfo, error) {
	sessionFile := filepath.Join(storageDir, sessionID+".json")

	// Check if file exists
	stat, err := os.Stat(sessionFile)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: session %s", ErrSessionNotFound, sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat session file: %w", err)
	}

	// Read file
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Parse just the fields we need
	var partial struct {
		ID                   string     `json:"id"`
		Name                 string     `json:"name"`
		Status               string     `json:"status"`
		CreatedAt            time.Time  `json:"created_at"`
		UpdatedAt            time.Time  `json:"updated_at"`
		Proposals            []struct{} `json:"proposals"`
		CompletedComparisons []struct{} `json:"completed_comparisons"`
	}

	if err := json.Unmarshal(data, &partial); err != nil {
		return nil, fmt.Errorf("%w: failed to parse session metadata", ErrSessionCorrupted)
	}

	return &SessionInfo{
		ID:              partial.ID,
		Name:            partial.Name,
		Status:          SessionStatus(partial.Status),
		CreatedAt:       partial.CreatedAt,
		UpdatedAt:       partial.UpdatedAt,
		ProposalCount:   len(partial.Proposals),
		ComparisonCount: len(partial.CompletedComparisons),
		FileSize:        stat.Size(),
		LastModified:    stat.ModTime(),
	}, nil
}

// SessionInfo provides summary information about a session
type SessionInfo struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Status          SessionStatus `json:"status"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	ProposalCount   int           `json:"proposal_count"`
	ComparisonCount int           `json:"comparison_count"`
	FileSize        int64         `json:"file_size"`
	LastModified    time.Time     `json:"last_modified"`
}

// DeleteSession removes a session and all its backups from storage
func DeleteSession(sessionID, storageDir string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	// Remove main session file
	sessionFile := filepath.Join(storageDir, sessionID+".json")
	if err := os.Remove(sessionFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session file: %w", err)
	}

	// Remove backup files
	pattern := filepath.Join(storageDir, sessionID+"_backup_*.json")
	backups, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to search for backup files: %w", err)
	}

	for _, backup := range backups {
		if err := os.Remove(backup); err != nil {
			// Continue removing other backups even if one fails
			// TODO: Add proper logging
		}
	}

	return nil
}

// EloEngine represents the interface to the Elo rating calculation engine
type EloEngine interface {
	CalculatePairwise(winner, loser EloRating) (EloRating, EloRating, error)
	CalculatePairwiseWithResult(winner, loser EloRating) (EloComparisonResult, error)
}

// EloRating represents a proposal's rating for Elo calculations
type EloRating struct {
	ID         string  // Unique proposal identifier
	Score      float64 // Current Elo rating
	Confidence float64 // Statistical confidence (0.0-1.0)
	Games      int     // Number of comparisons participated in
}

// EloComparisonResult represents the result of Elo calculations
type EloComparisonResult struct {
	Updates   []EloRatingUpdate // Rating changes for each affected proposal
	Method    ComparisonMethod  // Type of comparison performed
	Timestamp time.Time         // When calculation was performed
}

// EloRatingUpdate represents an individual rating change
type EloRatingUpdate struct {
	ProposalID string  // Proposal being updated
	OldRating  float64 // Rating before comparison
	NewRating  float64 // Rating after comparison
	Delta      float64 // Change in rating (NewRating - OldRating)
	KFactor    int     // K-factor used for this update
}

// ProcessPairwiseComparison integrates with Elo engine to process a pairwise comparison
func (s *Session) ProcessPairwiseComparison(winnerID, loserID string, engine EloEngine) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Verify we have an active comparison
	if s.CurrentComparison == nil {
		return ErrComparisonNotActive
	}

	// Verify this is a pairwise comparison
	if s.CurrentComparison.Method != MethodPairwise {
		return fmt.Errorf("%w: expected pairwise comparison, got %s", ErrInvalidComparison, s.CurrentComparison.Method)
	}

	// Verify proposals are in the comparison
	if len(s.CurrentComparison.ProposalIDs) != 2 {
		return fmt.Errorf("%w: pairwise comparison must have exactly 2 proposals", ErrInvalidComparison)
	}

	winnerFound, loserFound := false, false
	for _, id := range s.CurrentComparison.ProposalIDs {
		if id == winnerID {
			winnerFound = true
		}
		if id == loserID {
			loserFound = true
		}
	}

	if !winnerFound {
		return fmt.Errorf("%w: winner not in current comparison: %s", ErrInvalidComparison, winnerID)
	}
	if !loserFound {
		return fmt.Errorf("%w: loser not in current comparison: %s", ErrInvalidComparison, loserID)
	}

	// Get current proposal ratings
	winnerProposal, err := s.getProposalByIDInternal(winnerID)
	if err != nil {
		return fmt.Errorf("winner proposal not found: %w", err)
	}

	loserProposal, err := s.getProposalByIDInternal(loserID)
	if err != nil {
		return fmt.Errorf("loser proposal not found: %w", err)
	}

	// Convert to Elo rating format
	winnerRating := EloRating{
		ID:         winnerProposal.ID,
		Score:      winnerProposal.Score,
		Confidence: 0.8, // TODO: Calculate actual confidence based on games played
		Games:      s.getProposalGameCount(winnerID),
	}

	loserRating := EloRating{
		ID:         loserProposal.ID,
		Score:      loserProposal.Score,
		Confidence: 0.8, // TODO: Calculate actual confidence based on games played
		Games:      s.getProposalGameCount(loserID),
	}

	// Calculate new ratings using Elo engine
	result, err := engine.CalculatePairwiseWithResult(winnerRating, loserRating)
	if err != nil {
		return fmt.Errorf("Elo calculation failed: %w", err)
	}

	// Complete the comparison with results (internal version, mutex already held)
	comparison, err := s.completeComparisonInternal(winnerID, nil, false, "")
	if err != nil {
		return fmt.Errorf("failed to complete comparison: %w", err)
	}

	// Apply rating updates
	for _, update := range result.Updates {
		eloUpdate := EloUpdate{
			ID:           fmt.Sprintf("upd_%s_%d", update.ProposalID, time.Now().UnixNano()),
			ComparisonID: comparison.ID,
			ProposalID:   update.ProposalID,
			OldRating:    update.OldRating,
			NewRating:    update.NewRating,
			RatingDelta:  update.Delta,
			KFactor:      update.KFactor,
		}

		// Add to comparison
		for i := range s.CompletedComparisons {
			if s.CompletedComparisons[i].ID == comparison.ID {
				s.CompletedComparisons[i].EloUpdates = append(s.CompletedComparisons[i].EloUpdates, eloUpdate)
				break
			}
		}

		// Update proposal rating
		if err := s.updateProposalRatingInternal(update.ProposalID, update.NewRating); err != nil {
			return fmt.Errorf("failed to update proposal rating: %w", err)
		}
	}

	// Record matchup for optimization
	informationGain := s.calculateInformationGain(winnerID, loserID, result.Updates)
	s.recordMatchupInternal(winnerID, loserID, informationGain)

	// Update convergence metrics
	s.updateConvergenceMetrics()

	// Update rating bins
	s.updateRatingBins()

	return nil
}

// ProcessMultiProposalComparison integrates with Elo engine to process trio/quartet comparisons
func (s *Session) ProcessMultiProposalComparison(rankings []string, engine EloEngine) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Verify we have an active comparison
	if s.CurrentComparison == nil {
		return ErrComparisonNotActive
	}

	// Verify this is a multi-proposal comparison
	if s.CurrentComparison.Method == MethodPairwise {
		return fmt.Errorf("%w: use ProcessPairwiseComparison for pairwise comparisons", ErrInvalidComparison)
	}

	// Validate rankings
	if len(rankings) != len(s.CurrentComparison.ProposalIDs) {
		return fmt.Errorf("%w: rankings length must match proposal count", ErrInvalidComparison)
	}

	// Verify all proposals are in rankings
	rankingSet := make(map[string]bool)
	for _, id := range rankings {
		if rankingSet[id] {
			return fmt.Errorf("%w: duplicate proposal in rankings: %s", ErrInvalidComparison, id)
		}
		rankingSet[id] = true
	}

	for _, id := range s.CurrentComparison.ProposalIDs {
		if !rankingSet[id] {
			return fmt.Errorf("%w: proposal missing from rankings: %s", ErrInvalidComparison, id)
		}
	}

	// Decompose multi-proposal comparison into pairwise comparisons
	allUpdates := make([]EloRatingUpdate, 0)

	for i := 0; i < len(rankings); i++ {
		for j := i + 1; j < len(rankings); j++ {
			winnerID := rankings[i] // Better ranked (lower index)
			loserID := rankings[j]  // Worse ranked (higher index)

			// Get current proposals
			winnerProposal, err := s.getProposalByIDInternal(winnerID)
			if err != nil {
				return fmt.Errorf("winner proposal not found: %w", err)
			}

			loserProposal, err := s.getProposalByIDInternal(loserID)
			if err != nil {
				return fmt.Errorf("loser proposal not found: %w", err)
			}

			// Convert to Elo rating format
			winnerRating := EloRating{
				ID:         winnerProposal.ID,
				Score:      winnerProposal.Score,
				Confidence: 0.8, // TODO: Calculate based on games
				Games:      s.getProposalGameCount(winnerID),
			}

			loserRating := EloRating{
				ID:         loserProposal.ID,
				Score:      loserProposal.Score,
				Confidence: 0.8, // TODO: Calculate based on games
				Games:      s.getProposalGameCount(loserID),
			}

			// Calculate pairwise Elo update
			result, err := engine.CalculatePairwiseWithResult(winnerRating, loserRating)
			if err != nil {
				return fmt.Errorf("Elo calculation failed for %s vs %s: %w", winnerID, loserID, err)
			}

			// Collect updates
			allUpdates = append(allUpdates, result.Updates...)

			// Record matchup
			informationGain := s.calculateInformationGain(winnerID, loserID, result.Updates)
			s.recordMatchupInternal(winnerID, loserID, informationGain)
		}
	}

	// Complete the comparison (internal version, mutex already held)
	winnerID := rankings[0] // Best ranked proposal
	comparison, err := s.completeComparisonInternal(winnerID, rankings, false, "")
	if err != nil {
		return fmt.Errorf("failed to complete comparison: %w", err)
	}

	// Apply all rating updates
	updatesByProposal := make(map[string][]EloRatingUpdate)
	for _, update := range allUpdates {
		updatesByProposal[update.ProposalID] = append(updatesByProposal[update.ProposalID], update)
	}

	for proposalID, updates := range updatesByProposal {
		// Calculate net rating change for this proposal
		totalDelta := 0.0
		finalRating := 0.0
		for _, update := range updates {
			totalDelta += update.Delta
			finalRating = update.NewRating // Use last calculated rating
		}

		// Create consolidated Elo update
		eloUpdate := EloUpdate{
			ID:           fmt.Sprintf("upd_%s_%d", proposalID, time.Now().UnixNano()),
			ComparisonID: comparison.ID,
			ProposalID:   proposalID,
			OldRating:    updates[0].OldRating,
			NewRating:    finalRating,
			RatingDelta:  totalDelta,
			KFactor:      updates[0].KFactor, // Use K-factor from first update
		}

		// Add to comparison
		for i := range s.CompletedComparisons {
			if s.CompletedComparisons[i].ID == comparison.ID {
				s.CompletedComparisons[i].EloUpdates = append(s.CompletedComparisons[i].EloUpdates, eloUpdate)
				break
			}
		}

		// Update proposal rating
		if err := s.updateProposalRatingInternal(proposalID, finalRating); err != nil {
			return fmt.Errorf("failed to update proposal rating: %w", err)
		}
	}

	// Update convergence metrics
	s.updateConvergenceMetrics()

	// Update rating bins
	s.updateRatingBins()

	return nil
}

// getProposalByIDInternal retrieves a proposal by ID (internal, no locking)
func (s *Session) getProposalByIDInternal(id string) (*Proposal, error) {
	idx, exists := s.ProposalIndex[id]
	if !exists {
		return nil, fmt.Errorf("proposal not found: %s", id)
	}
	return &s.Proposals[idx], nil
}

// updateProposalRatingInternal updates a proposal's rating (internal, no locking)
func (s *Session) updateProposalRatingInternal(proposalID string, newRating float64) error {
	idx, exists := s.ProposalIndex[proposalID]
	if !exists {
		return fmt.Errorf("proposal not found: %s", proposalID)
	}

	s.Proposals[idx].Score = newRating
	s.Proposals[idx].UpdatedAt = time.Now()
	s.UpdatedAt = time.Now()

	return nil
}

// getProposalGameCount counts how many comparisons a proposal has participated in
func (s *Session) getProposalGameCount(proposalID string) int {
	count := 0
	for _, comparison := range s.CompletedComparisons {
		for _, id := range comparison.ProposalIDs {
			if id == proposalID {
				count++
				break
			}
		}
	}
	return count
}

// calculateInformationGain estimates the information value of a comparison
func (s *Session) calculateInformationGain(proposalA, proposalB string, updates []EloRatingUpdate) float64 {
	// Simple heuristic: larger rating changes indicate more informative comparisons
	totalAbsDelta := 0.0
	for _, update := range updates {
		if update.ProposalID == proposalA || update.ProposalID == proposalB {
			delta := update.Delta
			if delta < 0 {
				delta = -delta
			}
			totalAbsDelta += delta
		}
	}

	// Normalize to 0-1 scale (typical Elo changes are 0-64 points with K=32)
	return math.Min(totalAbsDelta/64.0, 1.0)
}

// GetOptimalMatchups returns suggested proposal pairs for next comparisons
func (s *Session) GetOptimalMatchups(count int) []ProposalPair {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if count <= 0 {
		return make([]ProposalPair, 0)
	}

	type matchupCandidate struct {
		ProposalA       string
		ProposalB       string
		Priority        float64 // Higher is better
		RatingDistance  float64
		ComparisonCount int
		LastCompared    time.Time
	}

	candidates := make([]matchupCandidate, 0)

	// Generate all possible pairs
	for i := 0; i < len(s.Proposals); i++ {
		for j := i + 1; j < len(s.Proposals); j++ {
			proposalA := s.Proposals[i].ID
			proposalB := s.Proposals[j].ID

			// Calculate rating distance
			ratingA := s.Proposals[i].Score
			ratingB := s.Proposals[j].Score
			distance := ratingA - ratingB
			if distance < 0 {
				distance = -distance
			}

			// Find existing matchup history
			comparisonCount := 0
			lastCompared := time.Time{}
			for _, matchup := range s.MatchupHistory {
				if (matchup.ProposalA == proposalA && matchup.ProposalB == proposalB) ||
					(matchup.ProposalA == proposalB && matchup.ProposalB == proposalA) {
					comparisonCount = matchup.ComparisonCount
					lastCompared = matchup.LastCompared
					break
				}
			}

			// Calculate priority (prefer close ratings, fewer previous comparisons, older comparisons)
			priority := 0.0

			// Rating distance factor (closer ratings are more informative)
			if distance > 0 {
				priority += 100.0 / (1.0 + distance/100.0) // Normalize by typical rating range
			}

			// Comparison count factor (prefer less compared pairs)
			priority += 50.0 / (1.0 + float64(comparisonCount))

			// Recency factor (prefer pairs not recently compared)
			if !lastCompared.IsZero() {
				hoursSince := time.Since(lastCompared).Hours()
				priority += math.Min(hoursSince/24.0*10.0, 25.0) // Up to 25 points for day+ old comparisons
			} else {
				priority += 25.0 // Never compared gets full recency points
			}

			candidates = append(candidates, matchupCandidate{
				ProposalA:       proposalA,
				ProposalB:       proposalB,
				Priority:        priority,
				RatingDistance:  distance,
				ComparisonCount: comparisonCount,
				LastCompared:    lastCompared,
			})
		}
	}

	// Sort by priority (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority > candidates[j].Priority
	})

	// Return top candidates
	maxResults := min(count, len(candidates))
	results := make([]ProposalPair, maxResults)
	for i := 0; i < maxResults; i++ {
		candidate := candidates[i]
		results[i] = ProposalPair{
			ProposalA:       candidate.ProposalA,
			ProposalB:       candidate.ProposalB,
			Priority:        candidate.Priority,
			RatingDistance:  candidate.RatingDistance,
			ComparisonCount: candidate.ComparisonCount,
			LastCompared:    candidate.LastCompared,
		}
	}

	return results
}

// ProposalPair represents a suggested pairing for comparison
type ProposalPair struct {
	ProposalA       string    `json:"proposal_a"`
	ProposalB       string    `json:"proposal_b"`
	Priority        float64   `json:"priority"`
	RatingDistance  float64   `json:"rating_distance"`
	ComparisonCount int       `json:"comparison_count"`
	LastCompared    time.Time `json:"last_compared"`
}

// Close closes the session and releases resources including the audit trail
func (s *Session) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SessionDetector provides session existence detection and mode determination
type SessionDetector struct {
	sessionsDir string
}

// NewSessionDetector creates a new session detector for the specified sessions directory
func NewSessionDetector(sessionsDir string) *SessionDetector {
	return &SessionDetector{
		sessionsDir: sessionsDir,
	}
}

// DetectMode determines whether to start a new session or resume an existing one
// based on the session name and existing session files
func (sd *SessionDetector) DetectMode(sessionName string) (SessionMode, error) {
	// Validate session name first
	if err := sd.validateSessionName(sessionName); err != nil {
		return StartMode, fmt.Errorf("%w: %v", ErrSessionNameInvalid, err)
	}

	// Ensure sessions directory exists
	if err := sd.ensureSessionsDirectory(); err != nil {
		return StartMode, fmt.Errorf("%w: failed to access sessions directory: %v", ErrModeDetectionFailed, err)
	}

	// Look for existing session files matching the session name
	sessionFile, err := sd.FindSessionFile(sessionName)
	if err != nil {
		return StartMode, fmt.Errorf("%w: %v", ErrModeDetectionFailed, err)
	}

	if sessionFile == "" {
		// No existing session found - start new session
		return StartMode, nil
	}

	// Validate the found session file
	if err := sd.ValidateSession(sessionFile); err != nil {
		return StartMode, fmt.Errorf("%w: session file corrupted: %v", ErrSessionCorrupted, err)
	}

	// Valid existing session found - resume mode
	return ResumeMode, nil
}

// FindSessionFile locates a session file by session name
// Returns empty string if no session file is found
func (sd *SessionDetector) FindSessionFile(sessionName string) (string, error) {
	if sessionName == "" {
		return "", fmt.Errorf("session name cannot be empty")
	}

	// Sanitize session name for filesystem
	safeName := SanitizeFilename(sessionName)
	
	// Construct direct filename
	sessionFile := filepath.Join(sd.sessionsDir, safeName+".json")
	
	// Check if file exists
	if _, err := os.Stat(sessionFile); err != nil {
		if os.IsNotExist(err) {
			return "", nil // No session found
		}
		return "", fmt.Errorf("failed to check session file: %w", err)
	}

	return sessionFile, nil
}

// ValidateSession validates the integrity of a session file
func (sd *SessionDetector) ValidateSession(sessionPath string) error {
	if sessionPath == "" {
		return fmt.Errorf("session path cannot be empty")
	}

	// Check if file exists and is readable
	file, err := os.Open(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session file not found: %s", sessionPath)
		}
		return fmt.Errorf("cannot read session file: %w", err)
	}
	defer file.Close()

	// Validate JSON structure by attempting to decode
	var sessionData map[string]any
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&sessionData); err != nil {
		return fmt.Errorf("invalid JSON in session file: %w", err)
	}

	// Check for required fields
	requiredFields := []string{"id", "name", "created_at", "config"}
	for _, field := range requiredFields {
		if _, exists := sessionData[field]; !exists {
			return fmt.Errorf("missing required field '%s' in session file", field)
		}
	}

	return nil
}

// validateSessionName validates the session name for filesystem safety
func (sd *SessionDetector) validateSessionName(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	// Check for invalid filesystem characters
	invalidChars := `<>:"/\|?*`
	if strings.ContainsAny(name, invalidChars) {
		return fmt.Errorf("session name contains invalid characters: %s", name)
	}

	// Check for reserved names (Windows)
	reservedNames := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	upperName := strings.ToUpper(name)
	for _, reserved := range reservedNames {
		if upperName == reserved {
			return fmt.Errorf("session name '%s' is reserved", name)
		}
	}

	return nil
}

// SanitizeFilename converts a session name into a safe filename
func SanitizeFilename(name string) string {
	// Replace invalid filesystem characters with underscores
	invalidChars := `<>:"/\|?*`
	result := name
	for _, char := range invalidChars {
		result = strings.ReplaceAll(result, string(char), "_")
	}
	
	// Replace spaces with underscores for cleaner filenames
	result = strings.ReplaceAll(result, " ", "_")
	
	// Ensure it's not empty after sanitization
	if result == "" {
		result = "session"
	}
	
	return result
}

// ensureSessionsDirectory creates the sessions directory if it doesn't exist
func (sd *SessionDetector) ensureSessionsDirectory() error {
	if sd.sessionsDir == "" {
		return fmt.Errorf("sessions directory path is empty")
	}

	// Check if directory exists
	if _, err := os.Stat(sd.sessionsDir); os.IsNotExist(err) {
		// Create directory with appropriate permissions
		if err := os.MkdirAll(sd.sessionsDir, 0755); err != nil {
			return fmt.Errorf("failed to create sessions directory: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to access sessions directory: %w", err)
	}

	return nil
}
