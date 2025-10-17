package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorage_NewFileStorage(t *testing.T) {
	tests := []struct {
		name      string
		backupDir string
		want      string
	}{
		{
			name:      "default backup directory",
			backupDir: "",
			want:      "backups",
		},
		{
			name:      "custom backup directory",
			backupDir: "custom/backup",
			want:      "custom/backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFileStorage(tt.backupDir)
			assert.Equal(t, tt.want, fs.backupDir)
			assert.Equal(t, 5, fs.maxBackups)
			assert.True(t, fs.atomicWrites)
		})
	}
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

			fs := NewFileStorage(filepath.Join(tempDir, "backups"))
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
	fs := NewFileStorage(filepath.Join(tempDir, "backups"))

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

func TestFileStorage_ExportProposalsToCSV(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewFileStorage(filepath.Join(tempDir, "backups"))

	proposals := []Proposal{
		{
			ID:            "PROP001",
			Title:         "Go Testing",
			Speaker:       "Jane Doe",
			Abstract:      "Testing patterns",
			Score:         1650.0,
			OriginalScore: func() *float64 { f := 1500.0; return &f }(),
			Metadata: map[string]string{
				"id":       "PROP001",
				"title":    "Go Testing",
				"speaker":  "Jane Doe",
				"abstract": "Testing patterns",
				"score":    "1500",
				"track":    "Backend",
			},
		},
		{
			ID:            "PROP002",
			Title:         "Microservices",
			Speaker:       "John Smith",
			Abstract:      "Scaling systems",
			Score:         1450.0,
			OriginalScore: func() *float64 { f := 1500.0; return &f }(),
			Metadata: map[string]string{
				"id":       "PROP002",
				"title":    "Microservices",
				"speaker":  "John Smith",
				"abstract": "Scaling systems",
				"score":    "1500",
				"track":    "Backend",
			},
		},
	}

	t.Run("export with new rating column", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "export.csv")
		csvConfig := DefaultCSVConfig()
		exportConfig := ExportConfig{
			Format:        "csv",
			SortBy:        "rating",
			SortOrder:     "desc",
			RoundDecimals: 0,
		}

		err := fs.ExportProposalsToCSV(proposals, outputPath, csvConfig, exportConfig)
		require.NoError(t, err)

		// Verify file was created and contains expected content
		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "final_rating")
		assert.Contains(t, contentStr, "1650") // Higher rating should be first due to desc sort
		assert.Contains(t, contentStr, "1450")

		// Verify header is present
		lines := strings.Split(strings.TrimSpace(contentStr), "\n")
		assert.GreaterOrEqual(t, len(lines), 3) // Header + 2 data rows
	})

	t.Run("export with different sort order", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "export_title.csv")
		csvConfig := DefaultCSVConfig()
		exportConfig := ExportConfig{
			Format:    "csv",
			SortBy:    "title",
			SortOrder: "asc",
		}

		err := fs.ExportProposalsToCSV(proposals, outputPath, csvConfig, exportConfig)
		require.NoError(t, err)

		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)

		// "Go Testing" should come before "Microservices" alphabetically
		contentStr := string(content)
		goIndex := strings.Index(contentStr, "Go Testing")
		microIndex := strings.Index(contentStr, "Microservices")
		assert.Less(t, goIndex, microIndex)
	})
}

func TestFileStorage_SaveSession(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewFileStorage(filepath.Join(tempDir, "backups"))

	session := &Session{
		ID:        "test-session",
		Name:      "Test Session",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
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

		assert.Equal(t, session.ID, loadedSession.ID)
		assert.Equal(t, session.Name, loadedSession.Name)
		assert.Len(t, loadedSession.Proposals, 1)
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
	fs := NewFileStorage(filepath.Join(tempDir, "backups"))

	session := &Session{
		ID:        "test-session",
		Name:      "Test Session",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
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
				loadedSession.Close()
			}
		}()

		assert.Equal(t, session.ID, loadedSession.ID)
		assert.Equal(t, session.Name, loadedSession.Name)
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

	t.Run("session with no ID", func(t *testing.T) {
		invalidPath := filepath.Join(tempDir, "no_id.json")
		invalidSession := map[string]any{
			"name": "Invalid Session",
		}
		data, _ := json.Marshal(invalidSession)
		err := os.WriteFile(invalidPath, data, 0644)
		require.NoError(t, err)

		_, err = fs.LoadSession(invalidPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session has no ID")
	})
}

func TestFileStorage_BackupOperations(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewFileStorage(filepath.Join(tempDir, "backups"))

	// Create a test file
	testFile := filepath.Join(tempDir, "test.json")
	testContent := `{"test": "data"}`
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	t.Run("create backup", func(t *testing.T) {
		backupPath, err := fs.CreateBackup(testFile)
		require.NoError(t, err)

		assert.FileExists(t, backupPath)

		// Verify backup content matches original
		backupContent, err := os.ReadFile(backupPath)
		require.NoError(t, err)
		assert.Equal(t, testContent, string(backupContent))
	})

	t.Run("backup nonexistent file", func(t *testing.T) {
		_, err := fs.CreateBackup("nonexistent.json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source file does not exist")
	})

	t.Run("recover from backup", func(t *testing.T) {
		// Create backup first
		_, err := fs.CreateBackup(testFile)
		require.NoError(t, err)

		// Modify original file
		err = os.WriteFile(testFile, []byte(`{"modified": "data"}`), 0644)
		require.NoError(t, err)

		// Recover from backup
		err = fs.RecoverFromBackup(testFile)
		require.NoError(t, err)

		// Verify original content is restored
		content, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, testContent, string(content))
	})

	t.Run("rotate backups", func(t *testing.T) {
		// Create multiple backups
		for i := 0; i < 10; i++ {
			_, err := fs.CreateBackup(testFile)
			require.NoError(t, err)
			time.Sleep(1 * time.Millisecond) // Ensure different timestamps
		}

		// Rotate to keep only 3 backups
		err := fs.RotateBackups(testFile, 3)
		require.NoError(t, err)

		// Verify only 3 backups remain
		backupPattern := fs.getBackupPath(testFile, "*")
		matches, err := filepath.Glob(backupPattern)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(matches), 3)
	})
}

func TestFileStorage_ConcurrentOperations(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewFileStorage(filepath.Join(tempDir, "backups"))

	session := &Session{
		ID:   "concurrent-test",
		Name: "Concurrent Session",
	}

	t.Run("concurrent saves", func(t *testing.T) {
		sessionPath := filepath.Join(tempDir, "concurrent.json")

		// Launch multiple goroutines to save concurrently
		done := make(chan error, 5)
		for i := 0; i < 5; i++ {
			go func(id int) {
				// Create a new session instead of copying to avoid lock value copy
				testSession := &Session{
					ID:                   fmt.Sprintf("session-%d", id),
					Name:                 session.Name,
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
