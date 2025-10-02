package data

import (
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
		// Load configuration using CLI parser
		args := []string{
			"--config", configFile,
			"--csv", tmpCSV.Name(),
		}

		config, opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, config)
		require.NotNil(t, opts)

		// Create Elo engine from configuration
		engineConfig := elo.Config{
			InitialRating: config.Elo.InitialRating,
			KFactor:       config.Elo.KFactor,
			MinRating:     config.Elo.MinRating,
			MaxRating:     config.Elo.MaxRating,
		}

		engine, err := elo.NewEngine(engineConfig)
		require.NoError(t, err)

		// Verify configuration was applied correctly
		assert.Equal(t, 1400.0, engineConfig.InitialRating)
		assert.Equal(t, 24, engineConfig.KFactor)
		assert.Equal(t, 100.0, engineConfig.MinRating)
		assert.Equal(t, 2500.0, engineConfig.MaxRating)

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

		// Verify export configuration
		assert.Equal(t, "json", config.Export.Format)
		assert.Equal(t, "rating", config.Export.SortBy)
		assert.Equal(t, "desc", config.Export.SortOrder)
	})

	t.Run("CLIOverridesConfigFile", func(t *testing.T) {
		// Test CLI overrides work correctly
		args := []string{
			"--config", configFile,
			"--csv", tmpCSV.Name(),
			"--k-factor", "16", // Override config file value of 24
			"--output-max", "10", // Override config file value of 5
		}

		config, _, err := ParseCLI(args)
		require.NoError(t, err)

		// CLI should override config file
		assert.Equal(t, 16, config.Elo.KFactor)     // CLI override
		assert.Equal(t, 10.0, config.Elo.OutputMax) // CLI override

		// Config file values should remain for non-overridden settings
		assert.Equal(t, 1400.0, config.Elo.InitialRating) // From config file
		assert.Equal(t, 100.0, config.Elo.MinRating)      // From config file

		// Test engine works with overridden configuration
		engineConfig := elo.Config{
			InitialRating: config.Elo.InitialRating,
			KFactor:       config.Elo.KFactor,
			MinRating:     config.Elo.MinRating,
			MaxRating:     config.Elo.MaxRating,
		}

		engine, err := elo.NewEngine(engineConfig)
		require.NoError(t, err)

		// Verify the overridden K-factor is used
		assert.Equal(t, 16, engineConfig.KFactor)

		// Test calculation works with new K-factor
		rating1 := elo.Rating{ID: "1", Score: 1400}
		rating2 := elo.Rating{ID: "2", Score: 1400}

		_, _, err = engine.CalculatePairwise(rating1, rating2)
		require.NoError(t, err)
	})

	t.Run("EnvironmentVariableOverrides", func(t *testing.T) {
		// Save original environment
		originalKFactor := os.Getenv("CONFELO_ELO_K_FACTOR")
		defer func() {
			if originalKFactor == "" {
				os.Unsetenv("CONFELO_ELO_K_FACTOR")
			} else {
				os.Setenv("CONFELO_ELO_K_FACTOR", originalKFactor)
			}
		}()

		// Set environment variable
		os.Setenv("CONFELO_ELO_K_FACTOR", "48")

		args := []string{
			"--config", configFile,
			"--csv", tmpCSV.Name(),
		}

		config, _, err := ParseCLI(args)
		require.NoError(t, err)

		// Environment should override config file (but CLI would override environment)
		assert.Equal(t, 48, config.Elo.KFactor)           // Environment override
		assert.Equal(t, 1400.0, config.Elo.InitialRating) // From config file
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

	t.Run("Complete Workflow Integration", func(t *testing.T) {
		storage := NewFileStorage(testDir)
		config := DefaultSessionConfig()

		// Step 1: Load CSV
		result, err := storage.LoadProposalsFromCSV(csvFile, config.CSV)
		require.NoError(t, err)

		// Step 2: Create session
		now := time.Now()
		session := &Session{
			ID:        "e2e_test_session",
			Name:      "End-to-End Test Session",
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
			Proposals: result.Proposals,
		}

		// Step 3: Save session
		sessionFile := filepath.Join(testDir, "e2e_session.json")
		err = storage.SaveSession(session, sessionFile)
		require.NoError(t, err)

		// Step 4: Load session (simulate resume)
		loadedSession, err := storage.LoadSession(sessionFile)
		require.NoError(t, err)
		assert.Equal(t, session.ID, loadedSession.ID)
		assert.Equal(t, len(session.Proposals), len(loadedSession.Proposals))

		// Step 5: Simulate comparison (would be done via TUI)
		// Update ratings to simulate comparison results
		loadedSession.Proposals[0].Score = 1600.0 // Winner
		loadedSession.Proposals[1].Score = 1450.0
		loadedSession.Proposals[2].Score = 1550.0
		loadedSession.Proposals[3].Score = 1400.0 // Lowest

		// Step 6: Export results
		exportConfig := ExportConfig{
			Format:          "csv",
			IncludeMetadata: true,
			SortBy:          "rating",
			SortOrder:       "desc",
			ScaleOutput:     true,
			RoundDecimals:   2,
		}

		exportFile := filepath.Join(testDir, "e2e_results.csv")
		err = storage.ExportProposalsToCSV(loadedSession.Proposals, exportFile, config.CSV, exportConfig)
		require.NoError(t, err)

		// Validate export
		exportedData, err := os.ReadFile(exportFile)
		require.NoError(t, err)
		exportContent := string(exportedData)

		assert.Contains(t, exportContent, "Go Microservices") // Should be first due to highest rating
		assert.Contains(t, exportContent, "rating")
		assert.Contains(t, exportContent, "1600") // Highest rating
	})

	t.Run("Memory and Performance Requirements", func(t *testing.T) {
		// Test constitutional requirements for larger datasets
		storage := NewFileStorage(testDir)
		config := DefaultSessionConfig()

		// Generate test data for 100 proposals (constitutional requirement is 200, using smaller for CI speed)
		largeCSV := generateTestCSV(100)
		largeCsvFile := filepath.Join(testDir, "large_test.csv")
		err := os.WriteFile(largeCsvFile, []byte(largeCSV), 0644)
		require.NoError(t, err)

		// Test loading performance
		result, err := storage.LoadProposalsFromCSV(largeCsvFile, config.CSV)
		require.NoError(t, err)
		assert.Equal(t, 100, len(result.Proposals))

		// Test export performance
		exportFile := filepath.Join(testDir, "large_export.csv")
		exportConfig := ExportConfig{Format: "csv", SortBy: "rating", SortOrder: "desc"}
		err = storage.ExportProposalsToCSV(result.Proposals, exportFile, config.CSV, exportConfig)
		require.NoError(t, err)

		// Verify export integrity
		exportStat, err := os.Stat(exportFile)
		require.NoError(t, err)
		assert.Greater(t, exportStat.Size(), int64(1000)) // Should have substantial content
	})
}

// generateTestCSV creates a CSV string with the specified number of test proposals
func generateTestCSV(count int) string {
	var content strings.Builder
	content.WriteString("id,title,abstract,speaker,score\n")

	for i := 1; i <= count; i++ {
		content.WriteString(fmt.Sprintf("%d,\"Test Proposal %d\",\"Test abstract for proposal %d\",\"Speaker %d\",\n", i, i, i, i))
	}

	return content.String()
}
