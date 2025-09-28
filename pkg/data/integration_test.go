package data

import (
	"os"
	"path/filepath"
	"testing"

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
