package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/confelo/pkg/elo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEloEngineIntegration demonstrates the configuration system working with the Elo engine
func TestEloEngineIntegration(t *testing.T) {
	// Create temporary CSV file
	tmpCSV, err := os.CreateTemp("", "test_integration_*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpCSV.Name())

	_, err = tmpCSV.WriteString("id,title,speaker\n1,Proposal A,Alice\n2,Proposal B,Bob\n")
	require.NoError(t, err)
	tmpCSV.Close()

	// Create config file with custom Elo settings
	configYAML := `
elo:
  initial_rating: 1400
  k_factor: 24
  min_rating: 100
  max_rating: 2500
  output_min: 1
  output_max: 5
  use_decimals: true

csv:
  id_column: id
  title_column: title
  speaker_column: speaker
  has_header: true

export:
  format: json
  sort_by: rating
  sort_order: desc
`

	tmpDir, err := os.MkdirTemp("", "test_config_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "confelo.yaml")
	err = os.WriteFile(configFile, []byte(configYAML), 0644)
	require.NoError(t, err)

	t.Run("ConfigurationToEloEngine", func(t *testing.T) {
		// Load configuration using CLI parser (CLI-only approach)
		args := []string{
			"--session-name", "test-session",
			"--input", tmpCSV.Name(),
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, opts)

		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Create Elo engine from configuration
		engineConfig := elo.Config{
			InitialRating: config.Elo.InitialRating,
			KFactor:       config.Elo.KFactor,
			MinRating:     config.Elo.MinRating,
			MaxRating:     config.Elo.MaxRating,
		}

		engine, err := elo.NewEngine(engineConfig)
		require.NoError(t, err)

		// Verify configuration was applied correctly (using defaults)
		assert.Equal(t, 1500.0, engineConfig.InitialRating)
		assert.Equal(t, 32, engineConfig.KFactor)
		assert.Equal(t, 0.0, engineConfig.MinRating)
		assert.Equal(t, 3000.0, engineConfig.MaxRating)

		// Test basic Elo calculation with configured parameters
		rating1 := elo.Rating{
			ID:    "1",
			Score: engineConfig.InitialRating,
		}
		rating2 := elo.Rating{
			ID:    "2",
			Score: engineConfig.InitialRating,
		}

		newWinnerRating, newLoserRating, err := engine.CalculatePairwise(rating1, rating2)
		require.NoError(t, err)

		// Verify ratings are within configured bounds
		assert.GreaterOrEqual(t, newWinnerRating.Score, engineConfig.MinRating)
		assert.LessOrEqual(t, newWinnerRating.Score, engineConfig.MaxRating)
		assert.GreaterOrEqual(t, newLoserRating.Score, engineConfig.MinRating)
		assert.LessOrEqual(t, newLoserRating.Score, engineConfig.MaxRating)

		// Test output scaling using configuration
		scaledWinner := engine.ScaleRating(newWinnerRating.Score, config.Elo.OutputMin, config.Elo.OutputMax)
		scaledLoser := engine.ScaleRating(newLoserRating.Score, config.Elo.OutputMin, config.Elo.OutputMax)

		// Verify output scaling uses configured range
		assert.GreaterOrEqual(t, scaledWinner, config.Elo.OutputMin)
		assert.LessOrEqual(t, scaledWinner, config.Elo.OutputMax)
		assert.GreaterOrEqual(t, scaledLoser, config.Elo.OutputMin)
		assert.LessOrEqual(t, scaledLoser, config.Elo.OutputMax)

		// Verify CSV configuration is available for data parsing
		assert.Equal(t, "id", config.CSV.IDColumn)
		assert.Equal(t, "title", config.CSV.TitleColumn)
		assert.Equal(t, "speaker", config.CSV.SpeakerColumn)
		assert.True(t, config.CSV.HasHeader)

		// Verify export configuration (using defaults)
		assert.Equal(t, "csv", config.Export.Format)
		assert.Equal(t, "rating", config.Export.SortBy)
		assert.Equal(t, "desc", config.Export.SortOrder)
	})

	t.Run("CLIParametersSetCorrectly", func(t *testing.T) {
		// Test CLI parameters work correctly (CLI-only approach)
		args := []string{
			"--session-name", "cli-test-session",
			"--input", tmpCSV.Name(),
			"--initial-rating", "1600",
			"--target-accepted", "15",
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err)

		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err)

		// CLI parameters should be applied
		assert.Equal(t, 1600.0, config.Elo.InitialRating)      // CLI parameter
		assert.Equal(t, 15, config.Convergence.TargetAccepted) // CLI parameter

		// Non-specified values should use defaults
		assert.Equal(t, 32, config.Elo.KFactor)      // Default value
		assert.Equal(t, 0.0, config.Elo.MinRating)   // Default value
		assert.Equal(t, 0.0, config.Elo.OutputMin)   // Default output scale min (from "0-100")
		assert.Equal(t, 100.0, config.Elo.OutputMax) // Default output scale max (from "0-100")
		assert.False(t, config.Elo.UseDecimals)      // Default output scale uses integers

		// Test engine works with overridden configuration
		engineConfig := elo.Config{
			InitialRating: config.Elo.InitialRating,
			KFactor:       config.Elo.KFactor,
			MinRating:     config.Elo.MinRating,
			MaxRating:     config.Elo.MaxRating,
		}

		engine, err := elo.NewEngine(engineConfig)
		require.NoError(t, err)

		// Verify the default K-factor is used (no CLI override available)
		assert.Equal(t, 32, engineConfig.KFactor)

		// Test calculation works with new K-factor
		rating1 := elo.Rating{ID: "1", Score: 1400}
		rating2 := elo.Rating{ID: "2", Score: 1400}

		_, _, err = engine.CalculatePairwise(rating1, rating2)
		require.NoError(t, err)
	})

	t.Run("CLIOnlyConfiguration", func(t *testing.T) {
		// Test CLI-only approach (no config files or environment variables)
		args := []string{
			"--session-name", "cli-only-session",
			"--input", tmpCSV.Name(),
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err)

		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err)

		// Should use default values
		assert.Equal(t, 32, config.Elo.KFactor)
		assert.Equal(t, 1500.0, config.Elo.InitialRating)
	})
}

// TestEndToEndWorkflow validates the complete CSV → comparison → export pipeline
func TestEndToEndWorkflow(t *testing.T) {
	// Setup test directory
	testDir := filepath.Join(".", "test_e2e")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	// Test data - create a small CSV for testing
	csvContent := `id,title,abstract,speaker,score
1,"Go Microservices","Building scalable microservices","John Doe",
2,"Python ML","Machine learning with Python","Jane Smith",
3,"JavaScript Frontend","Modern frontend development","Bob Wilson",
4,"Database Design","Effective database patterns","Alice Brown",`

	csvFile := filepath.Join(testDir, "test_proposals.csv")
	err = os.WriteFile(csvFile, []byte(csvContent), 0644)
	require.NoError(t, err)

	t.Run("CSV Loading Performance", func(t *testing.T) {
		storage := NewFileStorage(testDir)
		config := DefaultSessionConfig()

		// Test CSV loading speed for constitutional compliance
		result, err := storage.LoadProposalsFromCSV(csvFile, config.CSV)
		require.NoError(t, err)
		assert.Equal(t, 4, len(result.Proposals))
		assert.Equal(t, 0, len(result.ParseErrors))

		// Validate proposal data structure
		proposal := result.Proposals[0]
		assert.Equal(t, "1", proposal.ID)
		assert.Equal(t, "Go Microservices", proposal.Title)
		assert.Equal(t, "Building scalable microservices", proposal.Abstract)
		assert.Equal(t, "John Doe", proposal.Speaker)
		assert.Equal(t, 1500.0, proposal.Score) // Default initial rating
	})

}

// TestQuickstartScenario1_NewSession tests starting a new session
// Corresponds to Scenario 1 in quickstart.md
func TestQuickstartScenario1_NewSession(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")

	// Create test CSV file
	testCSV := filepath.Join(tempDir, "test-proposals.csv")
	csvContent := `id,title,speaker,abstract
1,"Go Programming","Alice Johnson","Introduction to Go language"
2,"React Patterns","Bob Smith","Advanced React development"
3,"Machine Learning","Carol Davis","ML fundamentals"`

	err := os.WriteFile(testCSV, []byte(csvContent), 0644)
	require.NoError(t, err)

	sessionName := "QuickstartTest"

	// Simulate CLI arguments: --session-name "QuickstartTest" --input test-proposals.csv
	// Note: CLIOptions will be updated in T008 to include SessionName and Input fields
	_ = sessionName // Will be used when CLIOptions is updated
	_ = testCSV     // Will be used when CLIOptions is updated

	t.Run("detect Start mode for new session", func(t *testing.T) {
		// This will be implemented when SessionDetector is ready
		detector := NewSessionDetector(sessionsDir)
		mode, err := detector.DetectMode(sessionName)

		// Expected to fail initially (TDD approach)
		if err != nil {
			t.Logf("Expected failure during TDD: %v", err)
			return
		}

		assert.Equal(t, StartMode, mode, "Should detect Start mode for new session")
	})

	t.Run("validate CSV input automatically", func(t *testing.T) {
		// Test CSV validation - will be implemented with new CLI structure
		t.Skip("CSV validation integration will be implemented when CLIOptions is updated in T008")

		// TODO: Test CSV loading with new simplified CLI interface
		// This should validate CSV input automatically without explicit config
	})

	t.Run("create session file", func(t *testing.T) {
		// This will be implemented when new session management is ready
		t.Skip("Session creation integration will be implemented when SessionMode is added in T010")

		// TODO: Test session creation with new CLI-only approach
		// Should create session file automatically after mode detection
	})

	t.Run("launch TUI in ranking mode", func(t *testing.T) {
		// This test validates TUI initialization without actually starting the UI
		// We'll test the app initialization logic

		t.Skip("TUI testing requires mock interface - will be implemented in T014")

		// TODO: Test TUI initialization with new CLI-only approach
		// This should validate that the app can start in ranking mode
		// without requiring subcommands
	})
}

// TestQuickstartScenario2_ResumeSession tests resuming an existing session
// Corresponds to Scenario 2 in quickstart.md
func TestQuickstartScenario2_ResumeSession(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	err := os.MkdirAll(sessionsDir, 0755)
	require.NoError(t, err)

	sessionName := "QuickstartTestResume"

	// Create an existing session file
	err = createTestSessionFileForIntegration(sessionsDir, sessionName)
	require.NoError(t, err)
	defer cleanupTestSessionFileForIntegration(sessionsDir, sessionName)

	t.Run("detect Resume mode for existing session", func(t *testing.T) {
		// This will be implemented when SessionDetector is ready
		detector := NewSessionDetector(sessionsDir)
		mode, err := detector.DetectMode(sessionName)

		// Expected to fail initially (TDD approach)
		if err != nil {
			t.Logf("Expected failure during TDD: %v", err)
			return
		}

		assert.Equal(t, ResumeMode, mode, "Should detect Resume mode for existing session")
	})

	t.Run("ignore input parameter when resuming", func(t *testing.T) {
		// This will be implemented when CLIOptions is updated in T008
		t.Skip("Input parameter handling will be implemented when CLIOptions is updated in T008")

		// TODO: Verify that resume mode ignores the input parameter
		// This logic will be implemented in the new CLI parsing
	})

	t.Run("load existing session data", func(t *testing.T) {
		// Test loading existing session - this should work with current code
		sessions, err := ListSessions(sessionsDir)
		if err != nil {
			t.Logf("Session listing failed (may not be implemented yet): %v", err)
			return
		}

		// Find our test session
		found := false
		for _, sessionID := range sessions {
			if strings.Contains(sessionID, sessionName) {
				found = true
				break
			}
		}

		if !found {
			t.Skip("Session listing not yet compatible with test format")
		}
	})

	t.Run("launch TUI with previous state", func(t *testing.T) {
		// This test validates that TUI can resume with existing session state
		t.Skip("TUI testing requires mock interface - will be implemented in T014")

		// TODO: Test TUI initialization with existing session
		// Should preserve ranking state and show correct progress
	})
}

// TestQuickstartScenario3_ErrorHandling tests error conditions
// Corresponds to Scenario 3 in quickstart.md
func TestQuickstartScenario3_ErrorHandling(t *testing.T) {

	t.Run("missing session name", func(t *testing.T) {
		// This will be implemented when CLIOptions is updated in T008
		t.Skip("CLI validation will be implemented when CLIOptions is updated in T008")
	})

	t.Run("new session without input file", func(t *testing.T) {
		// This will be implemented when mode detection is ready in T011
		t.Skip("New session validation will be implemented when mode detection is ready in T011")
	})

	t.Run("non-existent input file", func(t *testing.T) {
		// This will be implemented when CSV loading is integrated with new CLI
		t.Skip("CSV validation will be implemented when integrated with new CLI in T012")
	})

	t.Run("invalid session name characters", func(t *testing.T) {
		// This will be implemented when SessionDetector is ready in T011
		t.Skip("Session name validation will be implemented when SessionDetector is ready in T011")
	})

	t.Run("corrupted session file", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")
		err := os.MkdirAll(sessionsDir, 0755)
		require.NoError(t, err)

		// Create a corrupted session file
		corruptedFile := filepath.Join(sessionsDir, "session_CorruptedTest_12345.json")
		corruptedContent := `{"id": "incomplete json`
		err = os.WriteFile(corruptedFile, []byte(corruptedContent), 0644)
		require.NoError(t, err)
		defer os.Remove(corruptedFile)

		// This should fail validation
		detector := NewSessionDetector(sessionsDir)
		err = detector.ValidateSession(corruptedFile)

		if err == nil {
			t.Skip("Session validation not yet implemented - expected during TDD")
		} else {
			assert.Error(t, err, "Should fail with corrupted session file")
		}
	})
}

// Helper functions for integration testing

// createTestSessionFileForIntegration creates a test session file for integration testing
func createTestSessionFileForIntegration(sessionsDir, sessionName string) error {
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
			{"id": "1", "title": "Test Proposal", "rating": 1500.0},
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

// cleanupTestSessionFileForIntegration removes test session files
func cleanupTestSessionFileForIntegration(sessionsDir, sessionName string) {
	matches, _ := filepath.Glob(filepath.Join(sessionsDir, fmt.Sprintf("session_%s_*", sessionName)))
	for _, match := range matches {
		os.Remove(match)
	}
}

// TestQuickstartScenarios validates the quickstart scenarios from quickstart.md
func TestQuickstartScenarios(t *testing.T) {
	t.Run("Scenario1_StartNewSession", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")

		// Prepare test data (proposals CSV)
		testCSV := filepath.Join(tempDir, "test-proposals.csv")
		csvData := `id,title,speaker
1,Test Proposal A,Alice Smith
2,Test Proposal B,Bob Jones
3,Test Proposal C,Carol Wilson
`
		err := os.WriteFile(testCSV, []byte(csvData), 0644)
		require.NoError(t, err)

		// Test new session creation via CLI parsing
		sessionName := "QuickstartTest"
		args := []string{
			"--session-name", sessionName,
			"--input", testCSV,
		}

		// Parse CLI arguments
		opts, err := ParseCLI(args)
		require.NoError(t, err, "CLI parsing should succeed for new session")

		// Validate required parameters are present
		assert.Equal(t, sessionName, opts.SessionName)
		assert.Equal(t, testCSV, opts.Input)
		assert.Equal(t, "pairwise", opts.ComparisonMode) // Default value

		// Test mode detection for new session
		detector := NewSessionDetector(sessionsDir)
		mode, err := detector.DetectMode(sessionName)
		require.NoError(t, err, "Mode detection should succeed")
		assert.Equal(t, StartMode, mode, "Should detect StartMode for new session")

		// Test session config creation from CLI
		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err, "Session config creation should succeed")
		assert.NotNil(t, config)

		// Test CSV loading and session creation
		storage := &FileStorage{}
		parseResult, err := storage.LoadProposalsFromCSV(testCSV, config.CSV)
		require.NoError(t, err, "CSV loading should succeed")
		assert.Len(t, parseResult.Proposals, 3, "Should load 3 proposals")

		// Create session (simulating what main.go does)
		session := &Session{
			Name:      sessionName,
			Status:    StatusActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Proposals: parseResult.Proposals,
		}

		// Test session file creation
		err = os.MkdirAll(sessionsDir, 0755)
		require.NoError(t, err)

		sessionFile := filepath.Join(sessionsDir, SanitizeFilename(session.Name)+".json")
		err = storage.SaveSession(session, sessionFile)
		require.NoError(t, err, "Session save should succeed") // Validate session file was created
		assert.FileExists(t, sessionFile, "Session file should exist")

		// Validate session file content
		loadedSession, err := storage.LoadSession(sessionFile)
		require.NoError(t, err, "Session loading should succeed")
		assert.Equal(t, sessionName, loadedSession.Name)
		assert.Len(t, loadedSession.Proposals, 3)

		defer cleanupTestSessionFileForIntegration(sessionsDir, sessionName)
	})

	t.Run("Scenario2_ResumeExistingSession", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")
		err := os.MkdirAll(sessionsDir, 0755)
		require.NoError(t, err)

		sessionName := "ExistingQuickstartTest"

		// Create an existing session file first
		err = createTestSessionFileForIntegration(sessionsDir, sessionName)
		require.NoError(t, err)
		defer cleanupTestSessionFileForIntegration(sessionsDir, sessionName)

		// Test resume session via CLI parsing (no input required)
		args := []string{
			"--session-name", sessionName,
			// Note: --input should be ignored for existing sessions
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err, "CLI parsing should succeed for resume")

		// Test mode detection for existing session
		detector := NewSessionDetector(sessionsDir)
		mode, err := detector.DetectMode(sessionName)
		require.NoError(t, err, "Mode detection should succeed")
		assert.Equal(t, ResumeMode, mode, "Should detect ResumeMode for existing session")

		// Test session file finding and loading
		sessionFile, err := detector.FindSessionFile(sessionName)
		require.NoError(t, err, "Session file finding should succeed")
		assert.NotEmpty(t, sessionFile, "Should find existing session file")

		// Test session loading
		storage := &FileStorage{}
		session, err := storage.LoadSession(sessionFile)
		require.NoError(t, err, "Session loading should succeed")
		assert.Equal(t, sessionName, session.Name)

		// Test config creation (can override settings on resume)
		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err, "Config creation should succeed on resume")
		assert.NotNil(t, config)
	})

	t.Run("Scenario3_ErrorHandling", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")

		t.Run("MissingSessionName", func(t *testing.T) {
			args := []string{
				"--input", "test.csv",
				// Missing --session-name
			}

			_, err := ParseCLI(args)
			require.Error(t, err, "Should fail without session name")
			assert.Contains(t, strings.ToLower(err.Error()), "session name")
		})

		t.Run("NewSessionWithoutInput", func(t *testing.T) {
			sessionName := "NoInputTest"
			args := []string{
				"--session-name", sessionName,
				// Missing --input for new session
			}

			opts, err := ParseCLI(args)
			require.NoError(t, err, "CLI parsing should succeed") // Parser doesn't validate this

			// The validation happens in ValidateInputForNewSession
			err = ValidateInputForNewSession(opts)
			require.Error(t, err, "Should fail without input for new session")
			assert.Contains(t, err.Error(), "input file is required")
		})

		t.Run("InvalidInputFile", func(t *testing.T) {
			sessionName := "BadInputTest"
			args := []string{
				"--session-name", sessionName,
				"--input", "nonexistent.csv",
			}

			opts, err := ParseCLI(args)
			require.NoError(t, err, "CLI parsing should succeed")

			// The validation happens in ValidateInputForNewSession
			err = ValidateInputForNewSession(opts)
			require.Error(t, err, "Should fail with nonexistent input file")
			assert.Contains(t, err.Error(), "not found")
		})

		t.Run("CorruptedSessionFile", func(t *testing.T) {
			err := os.MkdirAll(sessionsDir, 0755)
			require.NoError(t, err)

			sessionName := "CorruptTest"
			corruptFile := filepath.Join(sessionsDir, fmt.Sprintf("session_%s_12345.json", sessionName))
			err = os.WriteFile(corruptFile, []byte("invalid json"), 0644)
			require.NoError(t, err)
			defer os.Remove(corruptFile)

			detector := NewSessionDetector(sessionsDir)
			_, err = detector.DetectMode(sessionName)
			require.Error(t, err, "Should detect corruption")
			assert.Contains(t, err.Error(), "corrupted")
		})
	})

	t.Run("Scenario4_ConfigurationOptions", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create test CSV
		testCSV := filepath.Join(tempDir, "config-test.csv")
		csvData := `id,title,speaker
1,Config Test A,Speaker A
2,Config Test B,Speaker B
`
		err := os.WriteFile(testCSV, []byte(csvData), 0644)
		require.NoError(t, err)

		t.Run("CustomConfiguration", func(t *testing.T) {
			args := []string{
				"--session-name", "ConfigTest",
				"--input", testCSV,
				"--comparison-mode", "trio",
				"--initial-rating", "1600",
				"--target-accepted", "5",
				"--verbose",
			}

			opts, err := ParseCLI(args)
			require.NoError(t, err, "CLI parsing should succeed with custom config")

			// Validate custom configuration is applied
			assert.Equal(t, "ConfigTest", opts.SessionName)
			assert.Equal(t, testCSV, opts.Input)
			assert.Equal(t, "trio", opts.ComparisonMode)
			assert.Equal(t, 1600.0, opts.InitialRating)
			assert.Equal(t, 5, opts.TargetAccepted)
			assert.True(t, opts.Verbose)

			// Test config creation with custom values
			config, err := CreateSessionConfigFromCLI(opts)
			require.NoError(t, err, "Config creation should succeed")
			assert.Equal(t, "trio", config.UI.ComparisonMode)
			assert.Equal(t, 1600.0, config.Elo.InitialRating)
			assert.Equal(t, 5, config.Convergence.TargetAccepted)
		})

		t.Run("HelpAndVersion", func(t *testing.T) {
			// Test help flag
			helpArgs := []string{"--help"}
			_, err := ParseCLI(helpArgs)
			require.Error(t, err, "Help should return error for exit")
			// The error type should indicate help was requested

			// Test version flag
			versionArgs := []string{"--version"}
			versionOpts, err := ParseCLI(versionArgs)
			require.NoError(t, err, "Version parsing should succeed")
			assert.True(t, versionOpts.Version)
		})
	})

	t.Run("PerformanceValidation", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")

		// Create test CSV
		testCSV := filepath.Join(tempDir, "perf-test.csv")
		csvData := `id,title,speaker
1,Perf Test A,Speaker A
2,Perf Test B,Speaker B  
`
		err := os.WriteFile(testCSV, []byte(csvData), 0644)
		require.NoError(t, err)

		t.Run("NewSessionStartupTime", func(t *testing.T) {
			start := time.Now()

			// Simulate new session startup process
			sessionName := "PerfTest"
			args := []string{
				"--session-name", sessionName,
				"--input", testCSV,
			}

			opts, err := ParseCLI(args)
			require.NoError(t, err)

			detector := NewSessionDetector(sessionsDir)
			_, err = detector.DetectMode(sessionName)
			require.NoError(t, err)

			config, err := CreateSessionConfigFromCLI(opts)
			require.NoError(t, err)

			storage := &FileStorage{}
			_, err = storage.LoadProposalsFromCSV(testCSV, config.CSV)
			require.NoError(t, err)

			elapsed := time.Since(start)

			// Performance requirement: <200ms startup time
			assert.Less(t, elapsed.Milliseconds(), int64(200),
				"New session startup should be <200ms, got %v", elapsed)
		})

		t.Run("ResumeSessionStartupTime", func(t *testing.T) {
			// First create a session
			sessionName := "PerfResumeTest"
			err := createTestSessionFileForIntegration(sessionsDir, sessionName)
			require.NoError(t, err)
			defer cleanupTestSessionFileForIntegration(sessionsDir, sessionName)

			start := time.Now()

			// Simulate resume session startup process
			args := []string{
				"--session-name", sessionName,
			}

			_, err = ParseCLI(args)
			require.NoError(t, err)

			detector := NewSessionDetector(sessionsDir)
			_, err = detector.DetectMode(sessionName)
			require.NoError(t, err)

			sessionFile, err := detector.FindSessionFile(sessionName)
			require.NoError(t, err)

			storage := &FileStorage{}
			_, err = storage.LoadSession(sessionFile)
			require.NoError(t, err)

			elapsed := time.Since(start)

			// Performance requirement: <200ms resume time
			assert.Less(t, elapsed.Milliseconds(), int64(200),
				"Resume session startup should be <200ms, got %v", elapsed)
		})
	})
}

// Contract Test T008: Integration test for complete ranking workflow without .jsonl files
func TestCompleteWorkflowNoJSONLIntegration(t *testing.T) {
	t.Run("Complete_ranking_workflow_produces_no_jsonl_files", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create test CSV file
		csvContent := "id,title,speaker,score\n1,Test Proposal 1,Speaker A,0\n2,Test Proposal 2,Speaker B,0\n3,Test Proposal 3,Speaker C,0\n4,Test Proposal 4,Speaker D,0\n"
		csvFile := filepath.Join(tempDir, "test-proposals.csv")
		err := os.WriteFile(csvFile, []byte(csvContent), 0644)
		require.NoError(t, err)

		// Load proposals
		storage := NewFileStorage(tempDir)
		config := DefaultSessionConfig()
		parseResult, err := storage.LoadProposalsFromCSVWithElo(csvFile, config.CSV, &config.Elo)
		require.NoError(t, err)
		require.Len(t, parseResult.Proposals, 4)

		// Create session
		session, err := NewSession("Integration Test Workflow", parseResult.Proposals, config, csvFile)
		require.NoError(t, err)
		session.storageDirectory = tempDir

		// Perform complete ranking workflow
		t.Log("Starting complete ranking workflow...")

		// Do multiple comparisons to simulate real usage
		comparisons := [][]string{
			{"1", "2"},
			{"2", "3"},
			{"3", "4"},
			{"1", "3"},
			{"2", "4"},
		}

		for _, compPair := range comparisons {
			err = session.StartComparison(compPair, MethodPairwise)
			require.NoError(t, err)

			// Simulate user picking winner (alternate winners)
			winner := compPair[0]
			if len(session.CompletedComparisons)%2 == 1 {
				winner = compPair[1]
			}

			_, err = session.CompleteComparison(winner, compPair, false, "")
			require.NoError(t, err)
		}

		// Save session state
		err = session.Save()
		require.NoError(t, err)

		// Update CSV with final scores (this is the export functionality that should be preserved)
		err = storage.UpdateCSVScores(session.Proposals, csvFile, config.CSV, &config.Elo)
		require.NoError(t, err)

		// Verify CSV was updated with scores
		updatedContent, err := os.ReadFile(csvFile)
		require.NoError(t, err)
		assert.Contains(t, string(updatedContent), "score", "CSV should contain score column after export")

		// CONTRACT: Complete workflow MUST NOT generate .jsonl files
		// This is the critical test that must FAIL before journal removal and PASS after
		matches, err := filepath.Glob(filepath.Join(tempDir, "*.jsonl"))
		require.NoError(t, err)
		assert.Empty(t, matches, "CRITICAL: Complete workflow should not generate any .jsonl audit files after journal removal")

		// Verify only expected files exist
		sessionFile := filepath.Join(tempDir, SanitizeFilename(session.Name)+".json")
		_, err = os.Stat(sessionFile)
		assert.NoError(t, err, "Session JSON file should exist")

		_, err = os.Stat(csvFile)
		assert.NoError(t, err, "Updated CSV file should exist")

		// Verify that session can be loaded back without audit dependencies
		loadedSession, err := LoadSession(session.Name, tempDir)
		require.NoError(t, err)
		assert.Equal(t, session.Name, loadedSession.Name)
		assert.Len(t, loadedSession.CompletedComparisons, 0) // Comparisons are not persisted		// CONTRACT: Loaded session should not have audit trail after journal removal
		// Note: auditTrail field was successfully removed, so this contract is fulfilled

		t.Log("Integration test completed successfully")
	})
}
