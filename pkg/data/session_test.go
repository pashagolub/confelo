package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Elo Engine for testing
type MockEloEngine struct {
	Results map[string]EloComparisonResult
}

func NewMockEloEngine() *MockEloEngine {
	return &MockEloEngine{
		Results: make(map[string]EloComparisonResult),
	}
}

func (m *MockEloEngine) CalculatePairwise(winner, loser EloRating) (EloRating, EloRating, error) {
	// Simple mock calculation: winner gains 16 points, loser loses 16 points
	newWinner := winner
	newLoser := loser

	newWinner.Score += 16
	newLoser.Score -= 16

	return newWinner, newLoser, nil
}

func (m *MockEloEngine) CalculatePairwiseWithResult(winner, loser EloRating) (EloComparisonResult, error) {
	newWinner, newLoser, err := m.CalculatePairwise(winner, loser)
	if err != nil {
		return EloComparisonResult{}, err
	}

	result := EloComparisonResult{
		Updates: []EloRatingUpdate{
			{
				ProposalID: winner.ID,
				OldRating:  winner.Score,
				NewRating:  newWinner.Score,
				Delta:      newWinner.Score - winner.Score,
				KFactor:    32,
			},
			{
				ProposalID: loser.ID,
				OldRating:  loser.Score,
				NewRating:  newLoser.Score,
				Delta:      newLoser.Score - loser.Score,
				KFactor:    32,
			},
		},
		Method:    MethodPairwise,
		Timestamp: time.Now(),
	}

	return result, nil
}

// Test helpers
func createTestProposals() []Proposal {
	return []Proposal{
		{
			ID:        "prop1",
			Title:     "Test Proposal 1",
			Abstract:  "First test proposal",
			Speaker:   "Speaker 1",
			Score:     1500.0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "prop2",
			Title:     "Test Proposal 2",
			Abstract:  "Second test proposal",
			Speaker:   "Speaker 2",
			Score:     1500.0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "prop3",
			Title:     "Test Proposal 3",
			Abstract:  "Third test proposal",
			Speaker:   "Speaker 3",
			Score:     1500.0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

func createTestConfig() SessionConfig {
	return DefaultSessionConfig()
}

func createTempDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "confelo_session_test_")
	require.NoError(t, err)
	return tempDir
}

func createTempCSV(t *testing.T, tempDir string) string {
	csvContent := `id,title,abstract,speaker
prop1,"Test Proposal 1","First test proposal","Speaker 1"
prop2,"Test Proposal 2","Second test proposal","Speaker 2"
prop3,"Test Proposal 3","Third test proposal","Speaker 3"
`
	csvPath := filepath.Join(tempDir, "test_proposals.csv")
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)
	return csvPath
}

func cleanupTempDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	require.NoError(t, err)
}

// Test Session Creation
func TestNewSession(t *testing.T) {
	proposals := createTestProposals()
	config := createTestConfig()

	t.Run("Valid session creation", func(t *testing.T) {
		session, err := NewSession("Test Session", proposals, config, "test.csv")
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, "Test Session", session.Name)
		assert.Equal(t, StatusCreated, session.Status)
		assert.Equal(t, len(proposals), len(session.Proposals))
		assert.NotEmpty(t, session.ID)
		assert.NotNil(t, session.ConvergenceMetrics)
		assert.Len(t, session.ProposalIndex, len(proposals))

		// Verify proposal index is correct
		for i, proposal := range proposals {
			assert.Equal(t, i, session.ProposalIndex[proposal.ID])
		}
	})

	t.Run("Insufficient proposals", func(t *testing.T) {
		oneProposal := proposals[:1]
		_, err := NewSession("Test Session", oneProposal, config, "test.csv")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2 proposals")
	})

	t.Run("Empty name", func(t *testing.T) {
		_, err := NewSession("", proposals, config, "test.csv")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session name is required")
	})
}

// Test Session Persistence
func TestSessionSaveAndLoad(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	// Create test CSV file
	csvPath := createTempCSV(t, tempDir)

	proposals := createTestProposals()
	config := createTestConfig()

	// Create and save session
	originalSession, err := NewSession("Test Session", proposals, config, csvPath)
	require.NoError(t, err)

	originalSession.SetStorageDirectory(tempDir)
	err = originalSession.Save()
	require.NoError(t, err)

	// Load session
	loadedSession, err := LoadSession(originalSession.ID, tempDir)
	require.NoError(t, err)

	// Verify loaded session matches original
	assert.Equal(t, originalSession.ID, loadedSession.ID)
	assert.Equal(t, originalSession.Name, loadedSession.Name)
	assert.Equal(t, originalSession.Status, loadedSession.Status)
	assert.Equal(t, len(originalSession.Proposals), len(loadedSession.Proposals))

	// Verify proposal index was rebuilt correctly
	assert.Len(t, loadedSession.ProposalIndex, len(proposals))
	for i, proposal := range loadedSession.Proposals {
		assert.Equal(t, i, loadedSession.ProposalIndex[proposal.ID])
	}
}

func TestSessionPersistenceErrorHandling(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	t.Run("Load non-existent session", func(t *testing.T) {
		_, err := LoadSession("nonexistent", tempDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("Load corrupted session", func(t *testing.T) {
		// Create a corrupted session file
		corruptedFile := filepath.Join(tempDir, "corrupted.json")
		err := os.WriteFile(corruptedFile, []byte("{invalid json"), 0644)
		require.NoError(t, err)

		_, err = LoadSession("corrupted", tempDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "corrupted")
	})
}

// Test Session Lifecycle
func TestSessionLifecycle(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	proposals := createTestProposals()
	config := createTestConfig()

	session, err := NewSession("Test Session", proposals, config, "test.csv")
	require.NoError(t, err)
	session.SetStorageDirectory(tempDir)

	// Test status transitions
	assert.Equal(t, StatusCreated, session.GetStatus())

	// Start comparison to become active
	err = session.StartComparison([]string{"prop1", "prop2"}, MethodPairwise)
	require.NoError(t, err)
	assert.Equal(t, StatusActive, session.GetStatus())

	// Pause session
	err = session.PauseSession()
	require.NoError(t, err)
	assert.Equal(t, StatusPaused, session.GetStatus())
	assert.Nil(t, session.GetCurrentComparison()) // Should cancel active comparison

	// Resume session
	err = session.ResumeSession()
	require.NoError(t, err)
	assert.Equal(t, StatusActive, session.GetStatus())

	// Complete session
	err = session.CompleteSession()
	require.NoError(t, err)
	assert.Equal(t, StatusComplete, session.GetStatus())
}

// Test Comparison Management
func TestComparisonManagement(t *testing.T) {
	proposals := createTestProposals()
	config := createTestConfig()

	session, err := NewSession("Test Session", proposals, config, "test.csv")
	require.NoError(t, err)

	t.Run("Start pairwise comparison", func(t *testing.T) {
		err := session.StartComparison([]string{"prop1", "prop2"}, MethodPairwise)
		require.NoError(t, err)

		comparison := session.GetCurrentComparison()
		assert.NotNil(t, comparison)
		assert.Equal(t, MethodPairwise, comparison.Method)
		assert.Len(t, comparison.ProposalIDs, 2)
		assert.Contains(t, comparison.ProposalIDs, "prop1")
		assert.Contains(t, comparison.ProposalIDs, "prop2")
	})

	t.Run("Complete comparison", func(t *testing.T) {
		comparison, err := session.CompleteComparison("prop1", nil, false, "")
		require.NoError(t, err)
		assert.NotNil(t, comparison)
		assert.Equal(t, "prop1", comparison.WinnerID)
		assert.False(t, comparison.Skipped)

		// Should clear current comparison
		assert.Nil(t, session.GetCurrentComparison())

		// Should add to history
		history := session.GetComparisonHistory()
		assert.Len(t, history, 1)
		assert.Equal(t, comparison.ID, history[0].ID)
	})

	t.Run("Start trio comparison", func(t *testing.T) {
		err := session.StartComparison([]string{"prop1", "prop2", "prop3"}, MethodTrio)
		require.NoError(t, err)

		comparison := session.GetCurrentComparison()
		assert.NotNil(t, comparison)
		assert.Equal(t, MethodTrio, comparison.Method)
		assert.Len(t, comparison.ProposalIDs, 3)
	})

	t.Run("Complete with rankings", func(t *testing.T) {
		rankings := []string{"prop1", "prop2", "prop3"}
		comparison, err := session.CompleteComparison("prop1", rankings, false, "")
		require.NoError(t, err)
		assert.Equal(t, rankings, comparison.Rankings)
	})

	t.Run("Skip comparison", func(t *testing.T) {
		err := session.StartComparison([]string{"prop1", "prop2"}, MethodPairwise)
		require.NoError(t, err)

		comparison, err := session.CompleteComparison("", nil, true, "Conflict of interest")
		require.NoError(t, err)
		assert.True(t, comparison.Skipped)
		assert.Equal(t, "Conflict of interest", comparison.SkipReason)
		assert.Empty(t, comparison.WinnerID)
	})
}

// Test Comparison Error Handling
func TestComparisonErrorHandling(t *testing.T) {
	proposals := createTestProposals()
	config := createTestConfig()

	session, err := NewSession("Test Session", proposals, config, "test.csv")
	require.NoError(t, err)

	t.Run("Invalid comparison parameters", func(t *testing.T) {
		// Wrong number of proposals for method
		err := session.StartComparison([]string{"prop1"}, MethodPairwise)
		assert.Error(t, err)

		err = session.StartComparison([]string{"prop1", "prop2", "prop3"}, MethodPairwise)
		assert.Error(t, err)

		// Non-existent proposal
		err = session.StartComparison([]string{"prop1", "nonexistent"}, MethodPairwise)
		assert.Error(t, err)
	})

	t.Run("Complete without active comparison", func(t *testing.T) {
		_, err := session.CompleteComparison("prop1", nil, false, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active comparison")
	})

	t.Run("Complete with invalid winner", func(t *testing.T) {
		err := session.StartComparison([]string{"prop1", "prop2"}, MethodPairwise)
		require.NoError(t, err)

		_, err = session.CompleteComparison("prop3", nil, false, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "winner ID not in comparison")
	})
}

// Test Elo Integration
func TestEloIntegration(t *testing.T) {
	proposals := createTestProposals()
	config := createTestConfig()

	session, err := NewSession("Test Session", proposals, config, "test.csv")
	require.NoError(t, err)

	mockEngine := NewMockEloEngine()

	t.Run("Process pairwise comparison", func(t *testing.T) {
		// Start comparison
		err := session.StartComparison([]string{"prop1", "prop2"}, MethodPairwise)
		require.NoError(t, err)

		// Get initial ratings
		prop1, _ := session.GetProposalByID("prop1")
		prop2, _ := session.GetProposalByID("prop2")
		initialRating1 := prop1.Score
		initialRating2 := prop2.Score

		// Process with Elo engine
		err = session.ProcessPairwiseComparison("prop1", "prop2", mockEngine)
		require.NoError(t, err)

		// Verify ratings were updated
		prop1Updated, _ := session.GetProposalByID("prop1")
		prop2Updated, _ := session.GetProposalByID("prop2")

		assert.Greater(t, prop1Updated.Score, initialRating1) // Winner gained points
		assert.Less(t, prop2Updated.Score, initialRating2)    // Loser lost points

		// Verify comparison was completed
		assert.Nil(t, session.GetCurrentComparison())
		history := session.GetComparisonHistory()
		assert.Len(t, history, 1)

		// Verify Elo updates were recorded
		comparison := history[0]
		assert.Len(t, comparison.EloUpdates, 2)
	})

	t.Run("Process multi-proposal comparison", func(t *testing.T) {
		// Start trio comparison
		err := session.StartComparison([]string{"prop1", "prop2", "prop3"}, MethodTrio)
		require.NoError(t, err)

		// Process with rankings
		rankings := []string{"prop1", "prop2", "prop3"}
		err = session.ProcessMultiProposalComparison(rankings, mockEngine)
		require.NoError(t, err)

		// Verify comparison was completed
		assert.Nil(t, session.GetCurrentComparison())
		history := session.GetComparisonHistory()
		assert.Len(t, history, 2) // Should have 2 comparisons now

		// Verify matchup history was updated
		matchups := session.GetMatchupHistory()
		assert.NotEmpty(t, matchups)
	})
}

// Test Convergence Tracking
func TestConvergenceTracking(t *testing.T) {
	proposals := createTestProposals()
	config := createTestConfig()

	session, err := NewSession("Test Session", proposals, config, "test.csv")
	require.NoError(t, err)

	mockEngine := NewMockEloEngine()

	// Perform several comparisons
	comparisons := []struct {
		props  []string
		method ComparisonMethod
		winner string
	}{
		{[]string{"prop1", "prop2"}, MethodPairwise, "prop1"},
		{[]string{"prop2", "prop3"}, MethodPairwise, "prop2"},
		{[]string{"prop1", "prop3"}, MethodPairwise, "prop1"},
	}

	for _, comp := range comparisons {
		err := session.StartComparison(comp.props, comp.method)
		require.NoError(t, err)

		err = session.ProcessPairwiseComparison(comp.winner, comp.props[1], mockEngine)
		require.NoError(t, err)
	}

	// Check convergence metrics
	metrics := session.GetConvergenceMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, 3, metrics.TotalComparisons)
	assert.Greater(t, metrics.CoveragePercentage, 0.0)
	assert.GreaterOrEqual(t, metrics.ConvergenceScore, 0.0)
	assert.LessOrEqual(t, metrics.ConvergenceScore, 1.0)
}

// Test Matchup Optimization
func TestMatchupOptimization(t *testing.T) {
	proposals := createTestProposals()
	config := createTestConfig()

	session, err := NewSession("Test Session", proposals, config, "test.csv")
	require.NoError(t, err)

	mockEngine := NewMockEloEngine()

	// Perform one comparison to create history
	err = session.StartComparison([]string{"prop1", "prop2"}, MethodPairwise)
	require.NoError(t, err)
	err = session.ProcessPairwiseComparison("prop1", "prop2", mockEngine)
	require.NoError(t, err)

	// Get optimal matchups
	matchups := session.GetOptimalMatchups(5)
	assert.NotEmpty(t, matchups)

	// Should suggest remaining pairs
	expectedPairs := []struct{ a, b string }{
		{"prop1", "prop3"},
		{"prop2", "prop3"},
	}

	foundPairs := make(map[string]bool)
	for _, matchup := range matchups {
		key := fmt.Sprintf("%s-%s", matchup.ProposalA, matchup.ProposalB)
		foundPairs[key] = true
		assert.Greater(t, matchup.Priority, 0.0)
	}

	for _, expected := range expectedPairs {
		key1 := fmt.Sprintf("%s-%s", expected.a, expected.b)
		key2 := fmt.Sprintf("%s-%s", expected.b, expected.a)
		assert.True(t, foundPairs[key1] || foundPairs[key2], "Expected pair not found: %s", key1)
	}
}

// Test Thread Safety
func TestThreadSafety(t *testing.T) {
	proposals := createTestProposals()
	config := createTestConfig()

	session, err := NewSession("Test Session", proposals, config, "test.csv")
	require.NoError(t, err)

	// Concurrent read operations should not cause data races
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Perform various read operations concurrently
			_ = session.GetStatus()
			_ = session.GetProposalCount()
			_ = session.GetProposals()
			_ = session.GetComparisonHistory()
			_ = session.GetConvergenceMetrics()
			_ = session.GetMatchupHistory()
			_ = session.GetOptimalMatchups(3)
		}()
	}

	wg.Wait() // Should complete without panics or data races
}

// Test Backup and Recovery
func TestBackupAndRecovery(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	proposals := createTestProposals()
	config := createTestConfig()

	// Create and save session
	session, err := NewSession("Test Session", proposals, config, "test.csv")
	require.NoError(t, err)
	session.SetStorageDirectory(tempDir)

	err = session.Save()
	require.NoError(t, err)

	// Create backup
	err = session.BackupSession()
	require.NoError(t, err)

	// Verify backup file exists
	backupPattern := filepath.Join(tempDir, session.ID+"_backup_*.json")
	backups, err := filepath.Glob(backupPattern)
	require.NoError(t, err)
	assert.Len(t, backups, 1)

	// Corrupt the main session file
	sessionFile := filepath.Join(tempDir, session.ID+".json")
	err = os.WriteFile(sessionFile, []byte("{corrupted"), 0644)
	require.NoError(t, err)

	// Recover from backup
	recoveredSession, err := RecoverFromBackup(session.ID, tempDir)
	require.NoError(t, err)

	// Verify recovered session
	assert.Equal(t, session.ID, recoveredSession.ID)
	assert.Equal(t, session.Name, recoveredSession.Name)
	assert.Equal(t, len(session.Proposals), len(recoveredSession.Proposals))
}

// Test Session Management Utilities
func TestSessionManagementUtils(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	proposals := createTestProposals()
	config := createTestConfig()

	// Create multiple sessions
	session1, err := NewSession("Session 1", proposals, config, "test.csv")
	require.NoError(t, err)
	session1.SetStorageDirectory(tempDir)
	err = session1.Save()
	require.NoError(t, err)

	session2, err := NewSession("Session 2", proposals, config, "test.csv")
	require.NoError(t, err)
	session2.SetStorageDirectory(tempDir)
	err = session2.Save()
	require.NoError(t, err)

	t.Run("List sessions", func(t *testing.T) {
		sessions, err := ListSessions(tempDir)
		require.NoError(t, err)
		assert.Len(t, sessions, 2)
		assert.Contains(t, sessions, session1.ID)
		assert.Contains(t, sessions, session2.ID)
	})

	t.Run("Get session info", func(t *testing.T) {
		info, err := GetSessionInfo(session1.ID, tempDir)
		require.NoError(t, err)
		assert.Equal(t, session1.ID, info.ID)
		assert.Equal(t, session1.Name, info.Name)
		assert.Equal(t, StatusCreated, info.Status)
		assert.Equal(t, len(proposals), info.ProposalCount)
		assert.Greater(t, info.FileSize, int64(0))
	})

	t.Run("Validate session file", func(t *testing.T) {
		err := ValidateSessionFile(session1.ID, tempDir)
		assert.NoError(t, err)

		err = ValidateSessionFile("nonexistent", tempDir)
		assert.Error(t, err)
	})

	t.Run("Delete session", func(t *testing.T) {
		err := DeleteSession(session2.ID, tempDir)
		require.NoError(t, err)

		// Verify session is gone
		_, err = LoadSession(session2.ID, tempDir)
		assert.Error(t, err)

		sessions, err := ListSessions(tempDir)
		require.NoError(t, err)
		assert.Len(t, sessions, 1)
		assert.NotContains(t, sessions, session2.ID)
	})
}

// TestSessionDetectionContract tests the SessionDetector interface
// This validates the contract specified in session-contract.md
func TestSessionDetectionContract(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	err := os.MkdirAll(sessionsDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name          string
		sessionName   string
		createSession bool
		expectedMode  SessionMode
		expectError   bool
		description   string
	}{
		{
			name:          "new session - StartMode",
			sessionName:   "NewTestSession",
			createSession: false,
			expectedMode:  StartMode,
			expectError:   false,
			description:   "Should return StartMode when no matching session exists",
		},
		{
			name:          "existing session - ResumeMode",
			sessionName:   "ExistingSession",
			createSession: true,
			expectedMode:  ResumeMode,
			expectError:   false,
			description:   "Should return ResumeMode when matching session exists",
		},
		{
			name:        "invalid session name",
			sessionName: "Invalid/Name",
			expectError: true,
			description: "Should fail with invalid filesystem characters",
		},
		{
			name:        "empty session name",
			sessionName: "",
			expectError: true,
			description: "Should fail with empty session name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test session if needed
			if tt.createSession {
				err := createTestSessionFile(sessionsDir, tt.sessionName)
				require.NoError(t, err)
			}

			// This will fail until we implement SessionDetector
			// Following TDD approach - write failing tests first
			detector := NewSessionDetector(sessionsDir)
			mode, err := detector.DetectMode(tt.sessionName)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedMode, mode, tt.description)
			}

			// Cleanup test session file
			if tt.createSession {
				cleanupTestSessionFile(sessionsDir, tt.sessionName)
			}
		})
	}
}

// TestSessionFileValidation tests session file validation
func TestSessionFileValidation(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	err := os.MkdirAll(sessionsDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name        string
		sessionData string
		expectError bool
		description string
	}{
		{
			name:        "valid session file",
			sessionData: `{"id":"session_Test_12345","name":"Test","created_at":"2025-10-02T17:38:56Z","config":{"comparison_mode":"pairwise"}}`,
			expectError: false,
			description: "Should validate correct JSON session file",
		},
		{
			name:        "invalid JSON",
			sessionData: `{"id":"session_Test_12345","name":"Test"`,
			expectError: true,
			description: "Should fail with malformed JSON",
		},
		{
			name:        "missing required fields",
			sessionData: `{"name":"Test"}`,
			expectError: true,
			description: "Should fail with missing required fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test session file with specific content
			sessionFile := filepath.Join(sessionsDir, "session_TestValidation_12345.json")
			err := os.WriteFile(sessionFile, []byte(tt.sessionData), 0644)
			require.NoError(t, err)
			defer os.Remove(sessionFile)

			// This will fail until we implement session validation
			// Following TDD approach - write failing tests first
			detector := NewSessionDetector(sessionsDir)
			err = detector.ValidateSession(sessionFile)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// Helper functions for session detection testing

// SessionMode and SessionDetector are now implemented in session.go (T010 and T011 completed)

// createTestSessionFile creates a test session file for testing
func createTestSessionFile(sessionsDir, sessionName string) error {
	sessionData := map[string]any{
		"id":         fmt.Sprintf("session_%s_%d_testid", sessionName, time.Now().Unix()),
		"name":       sessionName,
		"created_at": time.Now().Format(time.RFC3339),
		"config": map[string]any{
			"comparison_mode": "pairwise",
			"initial_rating":  1500.0,
			"target_accepted": 10,
		},
		"proposals": []map[string]any{
			{"id": 1, "title": "Test Proposal", "rating": 1500.0},
		},
	}

	filename := fmt.Sprintf("session_%s_%d_testid.json", sessionName, time.Now().Unix())
	sessionPath := filepath.Join(sessionsDir, filename)

	file, err := os.Create(sessionPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(sessionData)
}

// cleanupTestSessionFile removes test session files
func cleanupTestSessionFile(sessionsDir, sessionName string) {
	matches, _ := filepath.Glob(filepath.Join(sessionsDir, fmt.Sprintf("session_%s_*", sessionName)))
	for _, match := range matches {
		os.Remove(match)
	}
}

// TestSessionDetectorComprehensive provides comprehensive test coverage for SessionDetector
func TestSessionDetectorComprehensive(t *testing.T) {
	t.Run("NewSessionDetector", func(t *testing.T) {
		sessionsDir := "/test/sessions"
		detector := NewSessionDetector(sessionsDir)

		assert.NotNil(t, detector)
		assert.Equal(t, sessionsDir, detector.sessionsDir)
	})

	t.Run("DetectMode_EdgeCases", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")
		detector := NewSessionDetector(sessionsDir)

		tests := []struct {
			name            string
			sessionName     string
			setupFunc       func() error
			expectedMode    SessionMode
			expectError     bool
			expectedErrType error
		}{
			{
				name:            "empty session name",
				sessionName:     "",
				expectedMode:    StartMode,
				expectError:     true,
				expectedErrType: ErrSessionNameInvalid,
			},
			{
				name:            "invalid characters slash",
				sessionName:     "test/session",
				expectedMode:    StartMode,
				expectError:     true,
				expectedErrType: ErrSessionNameInvalid,
			},
			{
				name:            "invalid characters backslash",
				sessionName:     `test\session`,
				expectedMode:    StartMode,
				expectError:     true,
				expectedErrType: ErrSessionNameInvalid,
			},
			{
				name:            "reserved name Windows",
				sessionName:     "CON",
				expectedMode:    StartMode,
				expectError:     true,
				expectedErrType: ErrSessionNameInvalid,
			},
			{
				name:        "valid new session",
				sessionName: "ValidNewSession",
				setupFunc: func() error {
					return os.MkdirAll(sessionsDir, 0755)
				},
				expectedMode: StartMode,
				expectError:  false,
			},
			{
				name:        "existing valid session",
				sessionName: "ExistingValidSession",
				setupFunc: func() error {
					if err := os.MkdirAll(sessionsDir, 0755); err != nil {
						return err
					}
					return createTestSessionFile(sessionsDir, "ExistingValidSession")
				},
				expectedMode: ResumeMode,
				expectError:  false,
			},
			{
				name:        "corrupted session file",
				sessionName: "CorruptedSession",
				setupFunc: func() error {
					if err := os.MkdirAll(sessionsDir, 0755); err != nil {
						return err
					}
					sessionFile := filepath.Join(sessionsDir, "session_CorruptedSession_12345.json")
					return os.WriteFile(sessionFile, []byte("invalid json"), 0644)
				},
				expectedMode:    StartMode,
				expectError:     true,
				expectedErrType: ErrSessionCorrupted,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.setupFunc != nil {
					err := tt.setupFunc()
					require.NoError(t, err)
				}

				mode, err := detector.DetectMode(tt.sessionName)

				if tt.expectError {
					assert.Error(t, err)
					if tt.expectedErrType != nil {
						assert.ErrorIs(t, err, tt.expectedErrType)
					}
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expectedMode, mode)
				}

				// Cleanup
				cleanupTestSessionFile(sessionsDir, tt.sessionName)
			})
		}
	})

	t.Run("FindSessionFile", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")
		err := os.MkdirAll(sessionsDir, 0755)
		require.NoError(t, err)

		detector := NewSessionDetector(sessionsDir)

		// Test empty session name
		_, err = detector.FindSessionFile("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")

		// Test non-existent session
		sessionFile, err := detector.FindSessionFile("NonExistent")
		assert.NoError(t, err)
		assert.Empty(t, sessionFile)

		// Test existing session
		err = createTestSessionFile(sessionsDir, "ExistingForFind")
		require.NoError(t, err)
		defer cleanupTestSessionFile(sessionsDir, "ExistingForFind")

		sessionFile, err = detector.FindSessionFile("ExistingForFind")
		assert.NoError(t, err)
		assert.NotEmpty(t, sessionFile)
		assert.Contains(t, sessionFile, "ExistingForFind")
	})

	t.Run("ValidateSession", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")
		err := os.MkdirAll(sessionsDir, 0755)
		require.NoError(t, err)

		detector := NewSessionDetector(sessionsDir)

		// Test empty path
		err = detector.ValidateSession("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")

		// Test non-existent file
		err = detector.ValidateSession(filepath.Join(sessionsDir, "nonexistent.json"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Test invalid JSON
		invalidFile := filepath.Join(sessionsDir, "invalid.json")
		err = os.WriteFile(invalidFile, []byte("invalid json"), 0644)
		require.NoError(t, err)
		defer os.Remove(invalidFile)

		err = detector.ValidateSession(invalidFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")

		// Test missing required fields
		incompleteFile := filepath.Join(sessionsDir, "incomplete.json")
		incompleteData := `{"name":"Test"}`
		err = os.WriteFile(incompleteFile, []byte(incompleteData), 0644)
		require.NoError(t, err)
		defer os.Remove(incompleteFile)

		err = detector.ValidateSession(incompleteFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required field")

		// Test valid session file
		err = createTestSessionFile(sessionsDir, "ValidForValidation")
		require.NoError(t, err)
		defer cleanupTestSessionFile(sessionsDir, "ValidForValidation")

		matches, err := filepath.Glob(filepath.Join(sessionsDir, "session_ValidForValidation_*"))
		require.NoError(t, err)
		require.Len(t, matches, 1)

		err = detector.ValidateSession(matches[0])
		assert.NoError(t, err)
	})

	t.Run("SessionsDirectory_Management", func(t *testing.T) {
		tempDir := t.TempDir()
		nonExistentDir := filepath.Join(tempDir, "nonexistent", "sessions")

		detector := NewSessionDetector(nonExistentDir)

		// Should create directory and detect mode for new session
		mode, err := detector.DetectMode("TestSession")
		assert.NoError(t, err)
		assert.Equal(t, StartMode, mode)

		// Verify directory was created
		info, err := os.Stat(nonExistentDir)
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}
