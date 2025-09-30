package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions
func setupTestAuditTrail(t *testing.T) (*AuditTrail, string) {
	t.Helper()

	// Create temporary directory
	tempDir := t.TempDir()
	sessionID := "test_session_123"

	audit, err := NewAuditTrail(sessionID, tempDir)
	require.NoError(t, err)
	require.NotNil(t, audit)

	return audit, tempDir
}

func TestNewAuditTrail(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		expectError bool
	}{
		{
			name:        "valid session ID",
			sessionID:   "valid_session_123",
			expectError: false,
		},
		{
			name:        "empty session ID",
			sessionID:   "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			audit, err := NewAuditTrail(tt.sessionID, tempDir)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, audit)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, audit)
				assert.Equal(t, tt.sessionID, audit.sessionID)
				assert.True(t, audit.isInitialized)

				// Cleanup
				audit.Close()
			}
		})
	}
}

func TestAuditTrail_LogComparison(t *testing.T) {
	audit, _ := setupTestAuditTrail(t)
	defer audit.Close()

	tests := []struct {
		name      string
		eventType AuditEventType
		data      ComparisonAuditData
	}{
		{
			name:      "comparison started",
			eventType: EventComparisonStarted,
			data: ComparisonAuditData{
				ComparisonID: "comp_123",
				ProposalIDs:  []string{"prop_1", "prop_2"},
				Method:       "pairwise",
			},
		},
		{
			name:      "comparison completed with winner",
			eventType: EventComparisonCompleted,
			data: ComparisonAuditData{
				ComparisonID: "comp_123",
				ProposalIDs:  []string{"prop_1", "prop_2"},
				Method:       "pairwise",
				WinnerID:     "prop_1",
				Duration:     "2.5s",
			},
		},
		{
			name:      "comparison skipped",
			eventType: EventComparisonSkipped,
			data: ComparisonAuditData{
				ComparisonID: "comp_124",
				ProposalIDs:  []string{"prop_3", "prop_4"},
				Method:       "pairwise",
				SkipReason:   "conflict of interest",
			},
		},
		{
			name:      "trio comparison",
			eventType: EventComparisonCompleted,
			data: ComparisonAuditData{
				ComparisonID: "comp_125",
				ProposalIDs:  []string{"prop_1", "prop_2", "prop_3"},
				Method:       "trio",
				Rankings:     []string{"prop_2", "prop_1", "prop_3"},
				Duration:     "4.2s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := audit.LogComparison(tt.eventType, tt.data)
			assert.NoError(t, err)
		})
	}

	// Verify sequence increments
	assert.Equal(t, uint64(4), audit.GetSequence())
}

func TestAuditTrail_LogRatingUpdate(t *testing.T) {
	audit, _ := setupTestAuditTrail(t)
	defer audit.Close()

	data := RatingAuditData{
		ComparisonID: "comp_123",
		ProposalID:   "prop_1",
		OldRating:    1500.0,
		NewRating:    1516.0,
		RatingDelta:  16.0,
		KFactor:      32,
	}

	err := audit.LogRatingUpdate(data)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), audit.GetSequence())
}

func TestAuditTrail_LogSessionEvent(t *testing.T) {
	audit, _ := setupTestAuditTrail(t)
	defer audit.Close()

	metadata := map[string]interface{}{
		"session_name":   "Test Conference 2024",
		"proposal_count": 42,
		"config": map[string]interface{}{
			"initial_rating": 1500.0,
			"k_factor":       32,
		},
	}

	err := audit.LogSessionEvent(EventSessionCreated, metadata)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), audit.GetSequence())
}

func TestAuditTrail_Query(t *testing.T) {
	audit, _ := setupTestAuditTrail(t)
	defer audit.Close()

	// Create test data
	comparisonData1 := ComparisonAuditData{
		ComparisonID: "comp_123",
		ProposalIDs:  []string{"prop_1", "prop_2"},
		Method:       "pairwise",
		WinnerID:     "prop_1",
	}

	comparisonData2 := ComparisonAuditData{
		ComparisonID: "comp_124",
		ProposalIDs:  []string{"prop_2", "prop_3"},
		Method:       "pairwise",
		WinnerID:     "prop_3",
	}

	ratingData := RatingAuditData{
		ComparisonID: "comp_123",
		ProposalID:   "prop_1",
		OldRating:    1500.0,
		NewRating:    1516.0,
		RatingDelta:  16.0,
		KFactor:      32,
	}

	// Log test entries
	require.NoError(t, audit.LogComparison(EventComparisonCompleted, comparisonData1))
	require.NoError(t, audit.LogComparison(EventComparisonCompleted, comparisonData2))
	require.NoError(t, audit.LogRatingUpdate(ratingData))

	t.Run("query all entries", func(t *testing.T) {
		result, err := audit.Query(QueryOptions{})
		assert.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
		assert.Len(t, result.Entries, 3)
		assert.False(t, result.HasMore)
	})

	t.Run("filter by event type", func(t *testing.T) {
		result, err := audit.Query(QueryOptions{
			EventTypes: []AuditEventType{EventComparisonCompleted},
		})
		assert.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount)
		assert.Len(t, result.Entries, 2)
	})

	t.Run("filter by comparison ID", func(t *testing.T) {
		result, err := audit.Query(QueryOptions{
			ComparisonID: "comp_123",
		})
		assert.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount) // comparison event + rating update
		assert.Len(t, result.Entries, 2)
	})

	t.Run("filter by proposal ID", func(t *testing.T) {
		result, err := audit.Query(QueryOptions{
			ProposalID: "prop_1",
		})
		assert.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount) // comparison event + rating update
		assert.Len(t, result.Entries, 2)
	})

	t.Run("limit and offset", func(t *testing.T) {
		result, err := audit.Query(QueryOptions{
			Limit:  1,
			Offset: 1,
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
		assert.Len(t, result.Entries, 1)
		assert.True(t, result.HasMore)
	})

	t.Run("time range filter", func(t *testing.T) {
		now := time.Now()
		past := now.Add(-1 * time.Hour)
		future := now.Add(1 * time.Hour)

		result, err := audit.Query(QueryOptions{
			StartTime: &past,
			EndTime:   &future,
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
	})
}

func TestAuditTrail_GetComparisonHistory(t *testing.T) {
	audit, _ := setupTestAuditTrail(t)
	defer audit.Close()

	comparisonID := "comp_123"

	// Log comparison and related rating update
	comparisonData := ComparisonAuditData{
		ComparisonID: comparisonID,
		ProposalIDs:  []string{"prop_1", "prop_2"},
		Method:       "pairwise",
		WinnerID:     "prop_1",
	}

	ratingData := RatingAuditData{
		ComparisonID: comparisonID,
		ProposalID:   "prop_1",
		OldRating:    1500.0,
		NewRating:    1516.0,
		RatingDelta:  16.0,
		KFactor:      32,
	}

	require.NoError(t, audit.LogComparison(EventComparisonCompleted, comparisonData))
	require.NoError(t, audit.LogRatingUpdate(ratingData))

	// Test retrieval
	history, err := audit.GetComparisonHistory(comparisonID)
	assert.NoError(t, err)
	assert.Len(t, history, 2)
}

func TestAuditTrail_GetProposalHistory(t *testing.T) {
	audit, _ := setupTestAuditTrail(t)
	defer audit.Close()

	proposalID := "prop_1"

	// Log multiple events involving the proposal
	comparisonData1 := ComparisonAuditData{
		ComparisonID: "comp_123",
		ProposalIDs:  []string{proposalID, "prop_2"},
		Method:       "pairwise",
		WinnerID:     proposalID,
	}

	comparisonData2 := ComparisonAuditData{
		ComparisonID: "comp_124",
		ProposalIDs:  []string{proposalID, "prop_3"},
		Method:       "pairwise",
		WinnerID:     "prop_3",
	}

	ratingData := RatingAuditData{
		ComparisonID: "comp_123",
		ProposalID:   proposalID,
		OldRating:    1500.0,
		NewRating:    1516.0,
		RatingDelta:  16.0,
		KFactor:      32,
	}

	require.NoError(t, audit.LogComparison(EventComparisonCompleted, comparisonData1))
	require.NoError(t, audit.LogComparison(EventComparisonCompleted, comparisonData2))
	require.NoError(t, audit.LogRatingUpdate(ratingData))

	// Test retrieval
	history, err := audit.GetProposalHistory(proposalID)
	assert.NoError(t, err)
	assert.Len(t, history, 3) // 2 comparisons + 1 rating update
}

func TestAuditTrail_VerifyIntegrity(t *testing.T) {
	audit, tempDir := setupTestAuditTrail(t)

	// Log some entries
	comparisonData := ComparisonAuditData{
		ComparisonID: "comp_123",
		ProposalIDs:  []string{"prop_1", "prop_2"},
		Method:       "pairwise",
		WinnerID:     "prop_1",
	}

	require.NoError(t, audit.LogComparison(EventComparisonCompleted, comparisonData))
	require.NoError(t, audit.LogSessionEvent(EventSessionCreated, map[string]interface{}{"test": "data"}))

	audit.Close()

	t.Run("valid log passes integrity check", func(t *testing.T) {
		// Reopen and verify
		reopened, err := NewAuditTrail("test_session_123", tempDir)
		require.NoError(t, err)
		defer reopened.Close()

		err = reopened.VerifyIntegrity()
		assert.NoError(t, err)
	})

	t.Run("tampered log fails integrity check", func(t *testing.T) {
		// Tamper with the log file
		logPath := filepath.Join(tempDir, "audit_test_session_123.jsonl")

		// Read original content
		originalContent, err := os.ReadFile(logPath)
		require.NoError(t, err)

		// Tamper with content (change a hash)
		tamperedContent := []byte("tampered content\n")
		err = os.WriteFile(logPath, tamperedContent, 0644)
		require.NoError(t, err)

		// Try to verify - should fail
		tampered, err := NewAuditTrail("test_session_123", tempDir)
		if err == nil {
			// If it opens, verification should fail
			err = tampered.VerifyIntegrity()
			assert.Error(t, err)
			tampered.Close()
		} else {
			// Should fail to open due to validation
			assert.Error(t, err)
		}

		// Restore original content for cleanup
		os.WriteFile(logPath, originalContent, 0644)
	})
}

func TestAuditTrail_GetStatistics(t *testing.T) {
	audit, _ := setupTestAuditTrail(t)
	defer audit.Close()

	// Log various events
	require.NoError(t, audit.LogComparison(EventComparisonStarted, ComparisonAuditData{
		ComparisonID: "comp_123",
		ProposalIDs:  []string{"prop_1", "prop_2"},
		Method:       "pairwise",
	}))

	require.NoError(t, audit.LogComparison(EventComparisonCompleted, ComparisonAuditData{
		ComparisonID: "comp_123",
		ProposalIDs:  []string{"prop_1", "prop_2"},
		Method:       "pairwise",
		WinnerID:     "prop_1",
	}))

	require.NoError(t, audit.LogRatingUpdate(RatingAuditData{
		ComparisonID: "comp_123",
		ProposalID:   "prop_1",
		OldRating:    1500.0,
		NewRating:    1516.0,
		RatingDelta:  16.0,
		KFactor:      32,
	}))

	stats, err := audit.GetStatistics()
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, "test_session_123", stats.SessionID)
	assert.Equal(t, 3, stats.TotalEntries)
	assert.Equal(t, 1, stats.EventCounts[EventComparisonStarted])
	assert.Equal(t, 1, stats.EventCounts[EventComparisonCompleted])
	assert.Equal(t, 1, stats.EventCounts[EventRatingUpdated])
	assert.NotNil(t, stats.FirstEntry)
	assert.NotNil(t, stats.LastEntry)
}

func TestAuditTrail_PersistenceAndRecovery(t *testing.T) {
	tempDir := t.TempDir()
	sessionID := "persist_test_123"

	// Create initial audit trail and log some entries
	audit1, err := NewAuditTrail(sessionID, tempDir)
	require.NoError(t, err)

	comparisonData := ComparisonAuditData{
		ComparisonID: "comp_123",
		ProposalIDs:  []string{"prop_1", "prop_2"},
		Method:       "pairwise",
		WinnerID:     "prop_1",
	}

	require.NoError(t, audit1.LogComparison(EventComparisonCompleted, comparisonData))
	require.NoError(t, audit1.LogSessionEvent(EventSessionCreated, map[string]interface{}{"test": "data"}))

	firstSequence := audit1.GetSequence()
	audit1.Close()

	// Reopen and verify state is recovered
	audit2, err := NewAuditTrail(sessionID, tempDir)
	require.NoError(t, err)
	defer audit2.Close()

	assert.Equal(t, firstSequence, audit2.GetSequence())

	// Verify we can query previous entries
	result, err := audit2.Query(QueryOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 2, result.TotalCount)

	// Log new entry to verify continued operation
	require.NoError(t, audit2.LogRatingUpdate(RatingAuditData{
		ComparisonID: "comp_123",
		ProposalID:   "prop_1",
		OldRating:    1500.0,
		NewRating:    1516.0,
		RatingDelta:  16.0,
		KFactor:      32,
	}))

	assert.Equal(t, firstSequence+1, audit2.GetSequence())
}

func TestAuditTrail_ConcurrentAccess(t *testing.T) {
	audit, _ := setupTestAuditTrail(t)
	defer audit.Close()

	// Test concurrent logging (simplified test - full concurrency testing would be more complex)
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 10; i++ {
			data := ComparisonAuditData{
				ComparisonID: fmt.Sprintf("comp_%d", i),
				ProposalIDs:  []string{"prop_1", "prop_2"},
				Method:       "pairwise",
			}
			audit.LogComparison(EventComparisonStarted, data)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			data := RatingAuditData{
				ComparisonID: fmt.Sprintf("comp_%d", i),
				ProposalID:   "prop_1",
				OldRating:    1500.0,
				NewRating:    1500.0 + float64(i),
				RatingDelta:  float64(i),
				KFactor:      32,
			}
			audit.LogRatingUpdate(data)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify all entries were logged
	result, err := audit.Query(QueryOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 20, result.TotalCount)

	// Verify integrity
	assert.NoError(t, audit.VerifyIntegrity())
}

// Benchmark tests for performance validation
func BenchmarkAuditTrail_LogComparison(b *testing.B) {
	tempDir := b.TempDir()
	audit, err := NewAuditTrail("benchmark_session", tempDir)
	if err != nil {
		b.Fatal(err)
	}
	defer audit.Close()

	data := ComparisonAuditData{
		ComparisonID: "comp_benchmark",
		ProposalIDs:  []string{"prop_1", "prop_2"},
		Method:       "pairwise",
		WinnerID:     "prop_1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.ComparisonID = fmt.Sprintf("comp_%d", i)
		err := audit.LogComparison(EventComparisonCompleted, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditTrail_LogRatingUpdate(b *testing.B) {
	tempDir := b.TempDir()
	audit, err := NewAuditTrail("benchmark_session", tempDir)
	if err != nil {
		b.Fatal(err)
	}
	defer audit.Close()

	data := RatingAuditData{
		ComparisonID: "comp_benchmark",
		ProposalID:   "prop_1",
		OldRating:    1500.0,
		NewRating:    1516.0,
		RatingDelta:  16.0,
		KFactor:      32,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.ComparisonID = fmt.Sprintf("comp_%d", i)
		err := audit.LogRatingUpdate(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditTrail_Query(b *testing.B) {
	tempDir := b.TempDir()
	audit, err := NewAuditTrail("benchmark_session", tempDir)
	if err != nil {
		b.Fatal(err)
	}
	defer audit.Close()

	// Populate with test data
	for i := 0; i < 1000; i++ {
		data := ComparisonAuditData{
			ComparisonID: fmt.Sprintf("comp_%d", i),
			ProposalIDs:  []string{"prop_1", "prop_2"},
			Method:       "pairwise",
			WinnerID:     "prop_1",
		}
		audit.LogComparison(EventComparisonCompleted, data)
	}

	options := QueryOptions{
		EventTypes: []AuditEventType{EventComparisonCompleted},
		Limit:      100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := audit.Query(options)
		if err != nil {
			b.Fatal(err)
		}
	}
}
