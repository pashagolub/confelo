package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorage_NewFileStorage(t *testing.T) {
	fs := NewFileStorage()
	assert.NotNil(t, fs)
	assert.True(t, fs.atomicWrites)
}

func TestFileStorage_LoadProposalsFromCSV(t *testing.T) {
	// Create temporary directory for test files
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		csvContent  string
		config      CSVConfig
		wantCount   int
		wantErrors  int
		wantSkipped int
		checkFirst  func(*testing.T, []Proposal)
	}{
		{
			name: "valid CSV with header",
			csvContent: `id,title,speaker,abstract,score
PROP001,"Go Testing Best Practices","Jane Doe","Learn advanced testing patterns",1600
PROP002,"Microservices at Scale","John Smith","Building resilient systems",1450`,
			config:      DefaultCSVConfig(),
			wantCount:   2,
			wantErrors:  0,
			wantSkipped: 0,
			checkFirst: func(t *testing.T, proposals []Proposal) {
				assert.Equal(t, "PROP001", proposals[0].ID)
				assert.Equal(t, "Go Testing Best Practices", proposals[0].Title)
				assert.Equal(t, "Jane Doe", proposals[0].Speaker)
				assert.Equal(t, 1600.0, proposals[0].Score)
				assert.NotNil(t, proposals[0].OriginalScore)
				assert.Equal(t, 1600.0, *proposals[0].OriginalScore)
			},
		},
		{
			name: "CSV without score column",
			csvContent: `id,title,speaker
PROP001,"Go Testing","Jane Doe"
PROP002,"Microservices","John Smith"`,
			config:      DefaultCSVConfig(),
			wantCount:   2,
			wantErrors:  0,
			wantSkipped: 0,
			checkFirst: func(t *testing.T, proposals []Proposal) {
				assert.Equal(t, 1500.0, proposals[0].Score) // Default score
			},
		},
		{
			name: "CSV with conflict tags",
			csvContent: `id,title,speaker,conflicts
PROP001,"Go Testing","Jane Doe","tag1,tag2"
PROP002,"Microservices","John Smith","tag3"`,
			config: CSVConfig{
				IDColumn:       "id",
				TitleColumn:    "title",
				SpeakerColumn:  "speaker",
				ConflictColumn: "conflicts",
				HasHeader:      true,
				Delimiter:      ",",
			},
			wantCount:   2,
			wantErrors:  0,
			wantSkipped: 0,
			checkFirst: func(t *testing.T, proposals []Proposal) {
				assert.Equal(t, []string{"tag1", "tag2"}, proposals[0].ConflictTags)
				assert.Equal(t, []string{"tag3"}, proposals[1].ConflictTags)
			},
		},
		{
			name: "CSV with empty rows and missing data",
			csvContent: `id,title,speaker
PROP001,"Valid Title","Jane Doe"
,,
,"Missing ID","John Smith"
PROP003,"Another Valid","Alice Johnson"`,
			config:      DefaultCSVConfig(),
			wantCount:   2,
			wantErrors:  1, // Missing ID error
			wantSkipped: 1, // Empty row
			checkFirst: func(t *testing.T, proposals []Proposal) {
				assert.Equal(t, "PROP001", proposals[0].ID)
				assert.Equal(t, "PROP003", proposals[1].ID)
			},
		},
		{
			name: "CSV with invalid score",
			csvContent: `id,title,score
PROP001,"Valid Title",1500
PROP002,"Invalid Score","not-a-number"`,
			config:     DefaultCSVConfig(),
			wantCount:  2, // Both proposals loaded, invalid score uses default
			wantErrors: 0, // No errors - invalid scores are handled gracefully
			checkFirst: func(t *testing.T, proposals []Proposal) {
				assert.Equal(t, "PROP001", proposals[0].ID)
				assert.Equal(t, 1500.0, proposals[0].Score) // Parsed score
				assert.Equal(t, "PROP002", proposals[1].ID)
				assert.Equal(t, 1500.0, proposals[1].Score) // Default score used for invalid
			},
		},
		{
			name: "CSV with different delimiter",
			csvContent: `id;title;speaker
PROP001;"Go Testing";"Jane Doe"
PROP002;"Microservices";"John Smith"`,
			config: CSVConfig{
				IDColumn:      "id",
				TitleColumn:   "title",
				SpeakerColumn: "speaker",
				HasHeader:     true,
				Delimiter:     ";",
			},
			wantCount:   2,
			wantErrors:  0,
			wantSkipped: 0,
			checkFirst: func(t *testing.T, proposals []Proposal) {
				assert.Equal(t, "PROP001", proposals[0].ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test CSV file
			csvPath := filepath.Join(tempDir, tt.name+".csv")
			err := os.WriteFile(csvPath, []byte(tt.csvContent), 0644)
			require.NoError(t, err)

			fs := NewFileStorage()
			result, err := fs.LoadProposalsFromCSV(csvPath, tt.config)

			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(result.Proposals))
			assert.Equal(t, tt.wantErrors, len(result.ParseErrors))
			assert.Equal(t, tt.wantSkipped, len(result.SkippedRows))
			assert.Equal(t, tt.wantCount, result.SuccessfulRows)

			if tt.checkFirst != nil && len(result.Proposals) > 0 {
				tt.checkFirst(t, result.Proposals)
			}

			// Verify metadata preservation
			if len(result.Proposals) > 0 {
				assert.NotEmpty(t, result.Proposals[0].Metadata)
			}
		})
	}
}

func TestFileStorage_LoadProposalsFromCSV_Errors(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewFileStorage()

	t.Run("file not found", func(t *testing.T) {
		_, err := fs.LoadProposalsFromCSV("nonexistent.csv", DefaultCSVConfig())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot open CSV file")
	})

	t.Run("missing required ID column", func(t *testing.T) {
		csvPath := filepath.Join(tempDir, "no_id.csv")
		content := `title,speaker
"Test Title","Test Speaker"`
		err := os.WriteFile(csvPath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = fs.LoadProposalsFromCSV(csvPath, DefaultCSVConfig())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required ID column")
	})

	t.Run("missing required title column", func(t *testing.T) {
		csvPath := filepath.Join(tempDir, "no_title.csv")
		content := `id,speaker
PROP001,"Test Speaker"`
		err := os.WriteFile(csvPath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = fs.LoadProposalsFromCSV(csvPath, DefaultCSVConfig())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required title column")
	})
}

func TestFileStorage_SaveSession(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewFileStorage()

	// Create a test CSV file (CSV is source of truth)
	csvPath := filepath.Join(tempDir, "test_proposals.csv")
	csvContent := `id,title,speaker
PROP001,Test Proposal,Test Speaker`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	session := &Session{
		Name:         "test-session",
		InputCSVPath: csvPath, // CSV path is required
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Config:       DefaultSessionConfig(),
		Proposals: []Proposal{
			{
				ID:       "PROP001",
				Title:    "Test Proposal",
				Score:    1500.0,
				Metadata: map[string]string{"id": "PROP001", "title": "Test Proposal"},
			},
		},
	}

	t.Run("atomic save", func(t *testing.T) {
		sessionPath := filepath.Join(tempDir, "session.json")

		err := fs.SaveSession(session, sessionPath)
		require.NoError(t, err)

		// Verify file exists and contains valid JSON
		content, err := os.ReadFile(sessionPath)
		require.NoError(t, err)

		var loadedSession Session
		err = json.Unmarshal(content, &loadedSession)
		require.NoError(t, err)

		assert.Equal(t, session.Name, loadedSession.Name)
		// Proposals are not serialized anymore - they're reloaded from CSV
		// Instead, ProposalScores are saved
		assert.Len(t, loadedSession.ProposalScores, 1)
		assert.Contains(t, loadedSession.ProposalScores, "PROP001")
	})

	t.Run("non-atomic save", func(t *testing.T) {
		fs.SetAtomicWrites(false)
		sessionPath := filepath.Join(tempDir, "session_direct.json")

		err := fs.SaveSession(session, sessionPath)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(sessionPath)
		assert.NoError(t, err)
	})

	t.Run("nil session error", func(t *testing.T) {
		err := fs.SaveSession(nil, "test.json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session cannot be nil")
	})
}

func TestFileStorage_LoadSession(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewFileStorage()

	// Create a test CSV file (CSV is source of truth)
	csvPath := filepath.Join(tempDir, "test_proposals.csv")
	csvContent := `id,title,speaker
PROP001,Test Proposal,Test Speaker`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	session := &Session{
		Name:         "test-session",
		InputCSVPath: csvPath, // CSV path is required
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Config:       DefaultSessionConfig(),
	}

	t.Run("load valid session", func(t *testing.T) {
		sessionPath := filepath.Join(tempDir, "valid_session.json")

		// Save session first
		err := fs.SaveSession(session, sessionPath)
		require.NoError(t, err)

		// Load session
		loadedSession, err := fs.LoadSession(sessionPath)
		require.NoError(t, err)

		// Clean up audit trail to prevent file lock issues
		defer func() {
			if loadedSession != nil {
				_ = loadedSession.Close()
			}
		}()

		assert.Equal(t, session.Name, loadedSession.Name)
		assert.Equal(t, csvPath, loadedSession.InputCSVPath)
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := fs.LoadSession("nonexistent.json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session file does not exist")
	})

	t.Run("corrupted JSON", func(t *testing.T) {
		corruptedPath := filepath.Join(tempDir, "corrupted.json")
		err := os.WriteFile(corruptedPath, []byte("{invalid json"), 0644)
		require.NoError(t, err)

		_, err = fs.LoadSession(corruptedPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "corrupted session file")
	})
}

func TestFileStorage_ConcurrentOperations(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewFileStorage()

	session := &Session{
		Name: "concurrent-test",
	}

	t.Run("concurrent saves", func(t *testing.T) {
		sessionPath := filepath.Join(tempDir, "concurrent.json")

		// Launch multiple goroutines to save concurrently
		done := make(chan error, 5)
		for i := 0; i < 5; i++ {
			go func(id int) {
				// Create a new session instead of copying to avoid lock value copy
				testSession := &Session{
					Name:                 fmt.Sprintf("session-%d", id),
					Status:               session.Status,
					CreatedAt:            session.CreatedAt,
					UpdatedAt:            time.Now(),
					Proposals:            session.Proposals,
					CompletedComparisons: session.CompletedComparisons,
					ConvergenceMetrics:   session.ConvergenceMetrics,
					MatchupHistory:       session.MatchupHistory,
					RatingBins:           session.RatingBins,
				}
				done <- fs.SaveSession(testSession, fmt.Sprintf("%s-%d", sessionPath, id))
			}(i)
		}

		// Wait for all operations to complete
		for i := 0; i < 5; i++ {
			err := <-done
			assert.NoError(t, err)
		}
	})
}
