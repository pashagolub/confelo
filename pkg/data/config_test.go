package data

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigs(t *testing.T) {
	t.Run("DefaultSessionConfig", func(t *testing.T) {
		config := DefaultSessionConfig()

		// Verify all sub-configs are populated
		assert.NotZero(t, config.CSV)
		assert.NotZero(t, config.Elo)
		assert.NotZero(t, config.UI)
		assert.NotZero(t, config.Export)

		// Validate the entire config
		assert.NoError(t, config.Validate())
	})

	t.Run("DefaultCSVConfig", func(t *testing.T) {
		config := DefaultCSVConfig()

		assert.Equal(t, "id", config.IDColumn)
		assert.Equal(t, "title", config.TitleColumn)
		assert.Equal(t, "abstract", config.AbstractColumn)
		assert.Equal(t, "speaker", config.SpeakerColumn)
		assert.Equal(t, ",", config.Delimiter)
		assert.True(t, config.HasHeader)

		assert.NoError(t, config.Validate())
	})

	t.Run("DefaultEloConfig", func(t *testing.T) {
		config := DefaultEloConfig()

		assert.Equal(t, 1500.0, config.InitialRating)
		assert.Equal(t, 32, config.KFactor)
		assert.Equal(t, 0.0, config.MinRating)
		assert.Equal(t, 3000.0, config.MaxRating)
		assert.Equal(t, 0.0, config.OutputMin)
		assert.Equal(t, 10.0, config.OutputMax)
		assert.True(t, config.UseDecimals)

		assert.NoError(t, config.Validate())
	})

	t.Run("DefaultUIConfig", func(t *testing.T) {
		config := DefaultUIConfig()

		assert.Equal(t, "pairwise", config.ComparisonMode)
		assert.True(t, config.ShowProgress)
		assert.True(t, config.ShowConfidence)
		assert.True(t, config.AutoSave)
		assert.Equal(t, 5*time.Minute, config.AutoSaveInterval)

		assert.NoError(t, config.Validate())
	})

	t.Run("DefaultExportConfig", func(t *testing.T) {
		config := DefaultExportConfig()

		assert.Equal(t, "csv", config.Format)
		assert.True(t, config.IncludeMetadata)
		assert.Equal(t, "rating", config.SortBy)
		assert.Equal(t, "desc", config.SortOrder)
		assert.True(t, config.ScaleOutput)
		assert.Equal(t, 2, config.RoundDecimals)

		assert.NoError(t, config.Validate())
	})
}

func TestCSVConfigValidation(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := CSVConfig{
			IDColumn:    "proposal_id",
			TitleColumn: "title",
			Delimiter:   ",",
		}

		assert.NoError(t, config.Validate())
	})

	t.Run("MissingIDColumn", func(t *testing.T) {
		config := CSVConfig{
			TitleColumn: "title",
			Delimiter:   ",",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "id_column is required")
	})

	t.Run("MissingTitleColumn", func(t *testing.T) {
		config := CSVConfig{
			IDColumn:  "id",
			Delimiter: ",",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "title_column is required")
	})

	t.Run("DuplicateColumns", func(t *testing.T) {
		config := CSVConfig{
			IDColumn:       "same_column",
			TitleColumn:    "same_column",
			AbstractColumn: "different",
			Delimiter:      ",",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate column name")
	})

	t.Run("EmptyDelimiter", func(t *testing.T) {
		config := CSVConfig{
			IDColumn:    "id",
			TitleColumn: "title",
			Delimiter:   "",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "delimiter cannot be empty")
	})

	t.Run("InvalidDelimiter", func(t *testing.T) {
		config := CSVConfig{
			IDColumn:    "id",
			TitleColumn: "title",
			Delimiter:   "x",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a common CSV separator")
	})

	t.Run("ValidDelimiters", func(t *testing.T) {
		validDelimiters := []string{",", ";", "\t", "|"}

		for _, delimiter := range validDelimiters {
			config := CSVConfig{
				IDColumn:    "id",
				TitleColumn: "title",
				Delimiter:   delimiter,
			}

			assert.NoError(t, config.Validate(), "Delimiter '%s' should be valid", delimiter)
		}
	})
}

func TestEloConfigValidation(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := EloConfig{
			InitialRating: 1500,
			KFactor:       32,
			MinRating:     0,
			MaxRating:     3000,
			OutputMin:     0,
			OutputMax:     10,
		}

		assert.NoError(t, config.Validate())
	})

	t.Run("InvalidKFactor", func(t *testing.T) {
		config := EloConfig{
			InitialRating: 1500,
			KFactor:       0,
			MinRating:     0,
			MaxRating:     3000,
			OutputMin:     0,
			OutputMax:     10,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "k_factor must be positive")
	})

	t.Run("HighKFactor", func(t *testing.T) {
		config := EloConfig{
			InitialRating: 1500,
			KFactor:       150,
			MinRating:     0,
			MaxRating:     3000,
			OutputMin:     0,
			OutputMax:     10,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unusually high")
	})

	t.Run("InvalidRatingBounds", func(t *testing.T) {
		config := EloConfig{
			InitialRating: 1500,
			KFactor:       32,
			MinRating:     2000, // Higher than max
			MaxRating:     1000,
			OutputMin:     0,
			OutputMax:     10,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "min_rating")
		assert.Contains(t, err.Error(), "must be less than max_rating")
	})

	t.Run("InitialRatingOutOfBounds", func(t *testing.T) {
		config := EloConfig{
			InitialRating: 500, // Below min
			KFactor:       32,
			MinRating:     1000,
			MaxRating:     3000,
			OutputMin:     0,
			OutputMax:     10,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "initial_rating")
		assert.Contains(t, err.Error(), "must be between")
	})

	t.Run("InvalidOutputBounds", func(t *testing.T) {
		config := EloConfig{
			InitialRating: 1500,
			KFactor:       32,
			MinRating:     0,
			MaxRating:     3000,
			OutputMin:     10, // Higher than max
			OutputMax:     5,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output_min")
		assert.Contains(t, err.Error(), "must be less than output_max")
	})
}

func TestUIConfigValidation(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := UIConfig{
			ComparisonMode:   "pairwise",
			AutoSave:         true,
			AutoSaveInterval: 5 * time.Minute,
		}

		assert.NoError(t, config.Validate())
	})

	t.Run("InvalidComparisonMode", func(t *testing.T) {
		config := UIConfig{
			ComparisonMode: "invalid",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "comparison_mode")
		assert.Contains(t, err.Error(), "must be one of: pairwise, trio, quartet")
	})

	t.Run("ValidComparisonModes", func(t *testing.T) {
		validModes := []string{"pairwise", "trio", "quartet"}

		for _, mode := range validModes {
			config := UIConfig{
				ComparisonMode: mode,
			}

			assert.NoError(t, config.Validate(), "Mode '%s' should be valid", mode)
		}
	})

	t.Run("AutoSaveIntervalZero", func(t *testing.T) {
		config := UIConfig{
			ComparisonMode:   "pairwise",
			AutoSave:         true,
			AutoSaveInterval: 0,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auto_save_interval must be positive")
	})

	t.Run("AutoSaveDisabled", func(t *testing.T) {
		config := UIConfig{
			ComparisonMode:   "pairwise",
			AutoSave:         false,
			AutoSaveInterval: 0, // Should be OK when auto-save is disabled
		}

		assert.NoError(t, config.Validate())
	})

	t.Run("ExcessiveAutoSaveInterval", func(t *testing.T) {
		config := UIConfig{
			ComparisonMode:   "pairwise",
			AutoSave:         true,
			AutoSaveInterval: 48 * time.Hour,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unusually long")
	})
}

func TestExportConfigValidation(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := ExportConfig{
			Format:        "csv",
			SortBy:        "rating",
			SortOrder:     "desc",
			RoundDecimals: 2,
		}

		assert.NoError(t, config.Validate())
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		config := ExportConfig{
			Format:    "xml",
			SortBy:    "rating",
			SortOrder: "desc",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "format")
		assert.Contains(t, err.Error(), "must be one of: csv, json, yaml")
	})

	t.Run("InvalidSortBy", func(t *testing.T) {
		config := ExportConfig{
			Format:    "csv",
			SortBy:    "invalid",
			SortOrder: "desc",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sort_by")
		assert.Contains(t, err.Error(), "must be one of: rating, title, speaker, id")
	})

	t.Run("InvalidSortOrder", func(t *testing.T) {
		config := ExportConfig{
			Format:    "csv",
			SortBy:    "rating",
			SortOrder: "invalid",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sort_order")
		assert.Contains(t, err.Error(), "'asc' or 'desc'")
	})

	t.Run("InvalidDecimals", func(t *testing.T) {
		config := ExportConfig{
			Format:        "csv",
			SortBy:        "rating",
			SortOrder:     "desc",
			RoundDecimals: -1,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "round_decimals")
		assert.Contains(t, err.Error(), "must be between 0 and 10")

		config.RoundDecimals = 15
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "round_decimals")
	})
}

func TestYAMLLoading(t *testing.T) {
	t.Run("LoadValidYAML", func(t *testing.T) {
		yamlContent := `
csv:
  id_column: proposal_id
  title_column: proposal_title
  abstract_column: abstract
  delimiter: ","
  has_header: true

elo:
  initial_rating: 1400
  k_factor: 24
  min_rating: 0
  max_rating: 2800
  output_min: 1
  output_max: 5
  use_decimals: false

ui:
  comparison_mode: trio
  show_progress: true
  auto_save: false

export:
  format: json
  sort_by: title
  sort_order: asc
  round_decimals: 1
`

		// Create temporary file
		tmpFile, err := os.CreateTemp("", "test_config_*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(yamlContent)
		require.NoError(t, err)
		tmpFile.Close()

		// Load configuration
		config, err := LoadFromFile(tmpFile.Name())
		require.NoError(t, err)
		assert.NotNil(t, config)

		// Verify values were loaded correctly
		assert.Equal(t, "proposal_id", config.CSV.IDColumn)
		assert.Equal(t, "proposal_title", config.CSV.TitleColumn)
		assert.Equal(t, 1400.0, config.Elo.InitialRating)
		assert.Equal(t, 24, config.Elo.KFactor)
		assert.Equal(t, "trio", config.UI.ComparisonMode)
		assert.False(t, config.UI.AutoSave)
		assert.Equal(t, "json", config.Export.Format)
		assert.Equal(t, "asc", config.Export.SortOrder)
	})

	t.Run("LoadPartialYAML", func(t *testing.T) {
		yamlContent := `
csv:
  id_column: custom_id
  title_column: custom_title

elo:
  k_factor: 16
`

		tmpFile, err := os.CreateTemp("", "test_partial_*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(yamlContent)
		require.NoError(t, err)
		tmpFile.Close()

		config, err := LoadFromFile(tmpFile.Name())
		require.NoError(t, err)

		// Verify custom values
		assert.Equal(t, "custom_id", config.CSV.IDColumn)
		assert.Equal(t, "custom_title", config.CSV.TitleColumn)
		assert.Equal(t, 16, config.Elo.KFactor)

		// Verify defaults were applied
		assert.Equal(t, ",", config.CSV.Delimiter)            // Default
		assert.Equal(t, 1500.0, config.Elo.InitialRating)     // Default
		assert.Equal(t, "pairwise", config.UI.ComparisonMode) // Default
	})

	t.Run("LoadNonexistentFile", func(t *testing.T) {
		config, err := LoadFromFile("nonexistent.yaml")
		assert.Nil(t, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration file not found")
	})

	t.Run("LoadInvalidYAML", func(t *testing.T) {
		invalidYAML := `
csv:
  id_column: test
  invalid_yaml: [unclosed array
`

		tmpFile, err := os.CreateTemp("", "test_invalid_*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(invalidYAML)
		require.NoError(t, err)
		tmpFile.Close()

		config, err := LoadFromFile(tmpFile.Name())
		assert.Nil(t, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse configuration file")
	})
}

func TestEnvironmentOverrides(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"CONFELO_CSV_ID_COLUMN",
		"CONFELO_CSV_TITLE_COLUMN",
		"CONFELO_CSV_HAS_HEADER",
		"CONFELO_ELO_INITIAL_RATING",
		"CONFELO_ELO_K_FACTOR",
		"CONFELO_UI_COMPARISON_MODE",
		"CONFELO_UI_AUTO_SAVE",
		"CONFELO_EXPORT_FORMAT",
	}

	for _, envVar := range envVars {
		originalEnv[envVar] = os.Getenv(envVar)
	}

	// Clean up after test
	defer func() {
		for _, envVar := range envVars {
			if val, exists := originalEnv[envVar]; exists {
				os.Setenv(envVar, val)
			} else {
				os.Unsetenv(envVar)
			}
		}
	}()

	t.Run("EnvironmentOverrides", func(t *testing.T) {
		// Set environment variables
		os.Setenv("CONFELO_CSV_ID_COLUMN", "env_id")
		os.Setenv("CONFELO_CSV_TITLE_COLUMN", "env_title")
		os.Setenv("CONFELO_CSV_HAS_HEADER", "false")
		os.Setenv("CONFELO_ELO_INITIAL_RATING", "1600")
		os.Setenv("CONFELO_ELO_K_FACTOR", "48")
		os.Setenv("CONFELO_UI_COMPARISON_MODE", "quartet")
		os.Setenv("CONFELO_UI_AUTO_SAVE", "false")
		os.Setenv("CONFELO_EXPORT_FORMAT", "yaml")

		config, err := LoadWithEnvironment("")
		require.NoError(t, err)

		// Verify environment overrides
		assert.Equal(t, "env_id", config.CSV.IDColumn)
		assert.Equal(t, "env_title", config.CSV.TitleColumn)
		assert.False(t, config.CSV.HasHeader)
		assert.Equal(t, 1600.0, config.Elo.InitialRating)
		assert.Equal(t, 48, config.Elo.KFactor)
		assert.Equal(t, "quartet", config.UI.ComparisonMode)
		assert.False(t, config.UI.AutoSave)
		assert.Equal(t, "yaml", config.Export.Format)

		// Verify validation still works
		assert.NoError(t, config.Validate())
	})

	t.Run("InvalidEnvironmentValues", func(t *testing.T) {
		os.Setenv("CONFELO_ELO_INITIAL_RATING", "invalid_number")
		os.Setenv("CONFELO_CSV_HAS_HEADER", "not_boolean")

		// Should still work, invalid values are ignored
		config, err := LoadWithEnvironment("")
		require.NoError(t, err)

		// Should use defaults when parsing fails
		assert.Equal(t, 1500.0, config.Elo.InitialRating) // Default
		assert.True(t, config.CSV.HasHeader)              // Default
	})
}

func TestSaveToFile(t *testing.T) {
	t.Run("SaveValidConfig", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.CSV.IDColumn = "test_id"
		config.Elo.KFactor = 16

		tmpFile, err := os.CreateTemp("", "test_save_*.yaml")
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		err = config.SaveToFile(tmpFile.Name())
		require.NoError(t, err)

		// Verify file was written and can be read back
		loadedConfig, err := LoadFromFile(tmpFile.Name())
		require.NoError(t, err)

		assert.Equal(t, "test_id", loadedConfig.CSV.IDColumn)
		assert.Equal(t, 16, loadedConfig.Elo.KFactor)
	})
}

func TestConfigurationIntegration(t *testing.T) {
	t.Run("FullWorkflow", func(t *testing.T) {
		// Create config with custom values
		config := DefaultSessionConfig()
		config.CSV.IDColumn = "proposal_id"
		config.CSV.TitleColumn = "talk_title"
		config.Elo.InitialRating = 1400
		config.UI.ComparisonMode = "trio"
		config.Export.Format = "json"

		// Save to file
		tmpFile, err := os.CreateTemp("", "test_workflow_*.yaml")
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		err = config.SaveToFile(tmpFile.Name())
		require.NoError(t, err)

		// Set environment override
		os.Setenv("CONFELO_ELO_K_FACTOR", "24")
		defer os.Unsetenv("CONFELO_ELO_K_FACTOR")

		// Load with environment
		loadedConfig, err := LoadWithEnvironment(tmpFile.Name())
		require.NoError(t, err)

		// Verify file values were loaded
		assert.Equal(t, "proposal_id", loadedConfig.CSV.IDColumn)
		assert.Equal(t, "talk_title", loadedConfig.CSV.TitleColumn)
		assert.Equal(t, 1400.0, loadedConfig.Elo.InitialRating)
		assert.Equal(t, "trio", loadedConfig.UI.ComparisonMode)
		assert.Equal(t, "json", loadedConfig.Export.Format)

		// Verify environment override was applied
		assert.Equal(t, 24, loadedConfig.Elo.KFactor)

		// Verify defaults for unspecified values
		assert.Equal(t, ",", loadedConfig.CSV.Delimiter)
		assert.True(t, loadedConfig.UI.ShowProgress)

		// Verify configuration is valid
		assert.NoError(t, loadedConfig.Validate())
	})
}
