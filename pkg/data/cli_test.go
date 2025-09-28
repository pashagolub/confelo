package data

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCLI(t *testing.T) {
	// Create temporary CSV file for tests
	tmpCSV, err := os.CreateTemp("", "test_*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpCSV.Name())

	_, err = tmpCSV.WriteString("id,title\n1,Test Proposal\n")
	require.NoError(t, err)
	tmpCSV.Close()

	t.Run("BasicArguments", func(t *testing.T) {
		args := []string{
			"--csv", tmpCSV.Name(),
			"--id-column", "proposal_id",
			"--title-column", "proposal_title",
			"--k-factor", "24",
			"--mode", "trio",
			"--format", "json",
		}

		config, opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, config)
		require.NotNil(t, opts)

		// Verify CLI overrides
		assert.Equal(t, "proposal_id", config.CSV.IDColumn)
		assert.Equal(t, "proposal_title", config.CSV.TitleColumn)
		assert.Equal(t, 24, config.Elo.KFactor)
		assert.Equal(t, "trio", config.UI.ComparisonMode)
		assert.Equal(t, "json", config.Export.Format)

		// Verify CLI options
		assert.Equal(t, tmpCSV.Name(), opts.CSVFile)
	})

	t.Run("RequiredCSVFile", func(t *testing.T) {
		args := []string{}

		config, _, err := ParseCLI(args)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("ConfigFileDisabled", func(t *testing.T) {
		args := []string{
			"--no-config",
			"--csv", tmpCSV.Name(),
		}

		config, opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Should use defaults when config is disabled
		assert.Equal(t, "id", config.CSV.IDColumn)
		assert.Equal(t, 1500.0, config.Elo.InitialRating)
		assert.True(t, opts.NoConfig)
	})

	t.Run("BooleanFlags", func(t *testing.T) {
		args := []string{
			"--csv", tmpCSV.Name(),
			"--no-header",
			"--no-progress",
			"--no-confidence",
			"--no-auto-save",
			"--no-metadata",
			"--no-scaling",
			"--use-decimals",
		}

		config, opts, err := ParseCLI(args)
		require.NoError(t, err)

		// Verify boolean overrides
		assert.False(t, config.CSV.HasHeader)
		assert.False(t, config.UI.ShowProgress)
		assert.False(t, config.UI.ShowConfidence)
		assert.False(t, config.UI.AutoSave)
		assert.False(t, config.Export.IncludeMetadata)
		assert.False(t, config.Export.ScaleOutput)
		assert.True(t, config.Elo.UseDecimals)

		// Verify CLI options
		assert.True(t, opts.NoHeader)
		assert.True(t, opts.NoProgress)
		assert.True(t, opts.UseDecimals)
	})

	t.Run("NumericArguments", func(t *testing.T) {
		args := []string{
			"--csv", tmpCSV.Name(),
			"--initial-rating", "1400",
			"--k-factor", "16",
			"--min-rating", "100",
			"--max-rating", "2800",
			"--output-min", "1",
			"--output-max", "5",
			"--round-decimals", "1",
			"--auto-save-interval", "10m",
		}

		config, _, err := ParseCLI(args)
		require.NoError(t, err)

		// Verify numeric overrides
		assert.Equal(t, 1400.0, config.Elo.InitialRating)
		assert.Equal(t, 16, config.Elo.KFactor)
		assert.Equal(t, 100.0, config.Elo.MinRating)
		assert.Equal(t, 2800.0, config.Elo.MaxRating)
		assert.Equal(t, 1.0, config.Elo.OutputMin)
		assert.Equal(t, 5.0, config.Elo.OutputMax)
		assert.Equal(t, 1, config.Export.RoundDecimals)
		assert.Equal(t, 10*time.Minute, config.UI.AutoSaveInterval)
	})

	t.Run("InvalidConfiguration", func(t *testing.T) {
		args := []string{
			"--csv", tmpCSV.Name(),
			"--k-factor", "0", // Invalid K-factor
		}

		config, _, err := ParseCLI(args)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "invalid configuration")
	})

	t.Run("VersionFlag", func(t *testing.T) {
		args := []string{"--version"}

		config, opts, err := ParseCLI(args)
		assert.NoError(t, err)
		assert.Nil(t, config)
		assert.NotNil(t, opts)
		assert.True(t, opts.Version)
	})

	t.Run("HelpFlag", func(t *testing.T) {
		args := []string{"--help"}

		config, opts, err := ParseCLI(args)
		require.Error(t, err) // Help returns error (consistent with go-flags)
		assert.Nil(t, config)
		assert.NotNil(t, opts)

		// Check error type is help
		if flagsErr, ok := err.(*flags.Error); ok {
			assert.Equal(t, flags.ErrHelp, flagsErr.Type)
		}
	})
}

func TestCLIWithConfigFile(t *testing.T) {
	// Create temporary CSV file
	tmpCSV, err := os.CreateTemp("", "test_*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpCSV.Name())

	_, err = tmpCSV.WriteString("id,title\n1,Test\n")
	require.NoError(t, err)
	tmpCSV.Close()

	// Create temporary config file
	configYAML := `
csv:
  id_column: config_id
  title_column: config_title
  delimiter: ";"

elo:
  initial_rating: 1600
  k_factor: 20

ui:
  comparison_mode: quartet
  auto_save: false

export:
  format: yaml
  sort_order: asc
`

	tmpConfig, err := os.CreateTemp("", "test_config_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpConfig.Name())

	_, err = tmpConfig.WriteString(configYAML)
	require.NoError(t, err)
	tmpConfig.Close()

	t.Run("ConfigFileAndCLIOverrides", func(t *testing.T) {
		args := []string{
			"--config", tmpConfig.Name(),
			"--csv", tmpCSV.Name(),
			"--k-factor", "48", // CLI override (non-default value)
			"--format", "json", // CLI override
		}

		config, opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Values from config file
		assert.Equal(t, "config_id", config.CSV.IDColumn)
		assert.Equal(t, "config_title", config.CSV.TitleColumn)
		assert.Equal(t, ";", config.CSV.Delimiter)
		assert.Equal(t, 1600.0, config.Elo.InitialRating)
		assert.Equal(t, "quartet", config.UI.ComparisonMode)
		assert.False(t, config.UI.AutoSave)
		assert.Equal(t, "asc", config.Export.SortOrder)

		// CLI overrides
		assert.Equal(t, 48, config.Elo.KFactor)       // CLI override
		assert.Equal(t, "json", config.Export.Format) // CLI override

		// Verify CLI options
		assert.Equal(t, tmpConfig.Name(), opts.ConfigFile)
	})

	t.Run("NonexistentConfigFile", func(t *testing.T) {
		args := []string{
			"--config", "nonexistent.yaml",
			"--csv", tmpCSV.Name(),
		}

		config, opts, err := ParseCLI(args)
		require.NoError(t, err) // Should use defaults, not error
		require.NotNil(t, config)
		require.NotNil(t, opts)

		// Should use default values when config file not found
		assert.Equal(t, "id", config.CSV.IDColumn)
		assert.Equal(t, 1500.0, config.Elo.InitialRating)
		assert.Equal(t, "nonexistent.yaml", opts.ConfigFile)
	})
}

func TestValidateCLIOptions(t *testing.T) {
	// Create temporary CSV file
	tmpCSV, err := os.CreateTemp("", "test_*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpCSV.Name())
	tmpCSV.Close()

	// Create temporary directory for output tests
	tmpDir, err := os.MkdirTemp("", "test_output_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("ValidOptions", func(t *testing.T) {
		opts := &CLIOptions{
			CSVFile:    tmpCSV.Name(),
			OutputFile: filepath.Join(tmpDir, "output.csv"),
		}

		assert.NoError(t, ValidateCLIOptions(opts))
	})

	t.Run("MissingCSVFile", func(t *testing.T) {
		opts := &CLIOptions{}

		err := ValidateCLIOptions(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CSV file path is required")
	})

	t.Run("NonexistentCSVFile", func(t *testing.T) {
		opts := &CLIOptions{
			CSVFile: "nonexistent.csv",
		}

		err := ValidateCLIOptions(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CSV file not found")
	})

	t.Run("InvalidOutputDirectory", func(t *testing.T) {
		opts := &CLIOptions{
			CSVFile:    tmpCSV.Name(),
			OutputFile: "/nonexistent/directory/output.csv",
		}

		err := ValidateCLIOptions(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output directory does not exist")
	})

	t.Run("ValidOutputFile", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "valid_output.csv")
		opts := &CLIOptions{
			CSVFile:    tmpCSV.Name(),
			OutputFile: outputPath,
		}

		assert.NoError(t, ValidateCLIOptions(opts))
	})
}

func TestApplyCLIOverrides(t *testing.T) {
	t.Run("OnlyOverridesNonDefaults", func(t *testing.T) {
		config := DefaultSessionConfig()
		originalInitialRating := config.Elo.InitialRating

		opts := &CLIOptions{
			InitialRating: 1500, // Same as default
			KFactor:       24,   // Different from default
		}

		applyCLIOverrides(&config, opts)

		// Should not override with default values
		assert.Equal(t, originalInitialRating, config.Elo.InitialRating)

		// Should override with non-default values
		assert.Equal(t, 24, config.Elo.KFactor)
	})

	t.Run("BooleanOverrides", func(t *testing.T) {
		config := DefaultSessionConfig()

		// Set some initial values
		config.CSV.HasHeader = true
		config.UI.ShowProgress = true
		config.UI.AutoSave = true
		config.Export.IncludeMetadata = true

		opts := &CLIOptions{
			NoHeader:    true,
			NoProgress:  true,
			NoAutoSave:  true,
			NoMetadata:  true,
			UseDecimals: true,
		}

		applyCLIOverrides(&config, opts)

		// Boolean flags should invert the values
		assert.False(t, config.CSV.HasHeader)
		assert.False(t, config.UI.ShowProgress)
		assert.False(t, config.UI.AutoSave)
		assert.False(t, config.Export.IncludeMetadata)
		assert.True(t, config.Elo.UseDecimals)
	})
}

func TestGetConfigSearchPaths(t *testing.T) {
	paths := GetConfigSearchPaths("confelo.yaml")

	// Should include current directory
	assert.Contains(t, paths, "confelo.yaml")

	// Should include user config paths (if home directory exists)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		expectedPaths := []string{
			filepath.Join(homeDir, ".config", "confelo", "confelo.yaml"),
			filepath.Join(homeDir, ".confelo", "confelo.yaml"),
		}

		for _, expectedPath := range expectedPaths {
			assert.Contains(t, paths, expectedPath)
		}
	}

	// Should include system config path
	assert.Contains(t, paths, filepath.Join("/etc", "confelo", "confelo.yaml"))
}

func TestCreateDefaultConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_config_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "subdir", "confelo.yaml")

	err = CreateDefaultConfig(configPath)
	require.NoError(t, err)

	// Verify file was created
	assert.FileExists(t, configPath)

	// Verify it can be loaded
	config, err := LoadFromFile(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Verify it contains default values
	assert.Equal(t, "id", config.CSV.IDColumn)
	assert.Equal(t, 1500.0, config.Elo.InitialRating)
}

func TestCheckWritable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_writable_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("WritableFile", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "writable.txt")
		assert.NoError(t, checkWritable(testFile))
	})

	t.Run("NonexistentDirectory", func(t *testing.T) {
		testFile := filepath.Join("/nonexistent", "file.txt")
		assert.Error(t, checkWritable(testFile))
	})
}

func TestCLIIntegrationWithEnvironment(t *testing.T) {
	// Save original environment
	originalVars := map[string]string{
		"CONFELO_ELO_K_FACTOR":       os.Getenv("CONFELO_ELO_K_FACTOR"),
		"CONFELO_UI_COMPARISON_MODE": os.Getenv("CONFELO_UI_COMPARISON_MODE"),
	}

	defer func() {
		for key, val := range originalVars {
			if val == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, val)
			}
		}
	}()

	// Create temporary CSV file
	tmpCSV, err := os.CreateTemp("", "test_*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpCSV.Name())
	tmpCSV.Close()

	t.Run("CLIEnvironmentConfigPrecedence", func(t *testing.T) {
		// Set environment variables
		os.Setenv("CONFELO_ELO_K_FACTOR", "16")         // Environment
		os.Setenv("CONFELO_UI_COMPARISON_MODE", "trio") // Environment

		args := []string{
			"--no-config", // Skip config file
			"--csv", tmpCSV.Name(),
			"--k-factor", "48", // CLI override (highest precedence)
		}

		config, opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, config)

		// CLI should override environment
		assert.Equal(t, 48, config.Elo.KFactor) // CLI wins

		// Environment should be applied when no CLI override
		assert.Equal(t, "trio", config.UI.ComparisonMode) // Environment wins

		assert.True(t, opts.NoConfig)
	})
}
