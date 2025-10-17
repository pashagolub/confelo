package data

import (
	"os"
	"testing"

	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCLI(t *testing.T) {
	// Create temporary CSV file for tests
	tmpCSV, err := os.CreateTemp("", "test_*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpCSV.Name())

	_, err = tmpCSV.WriteString("id,title,speaker\n1,Test Proposal,John Doe\n")
	require.NoError(t, err)
	tmpCSV.Close()

	t.Run("ValidNewSession", func(t *testing.T) {
		args := []string{
			"--session-name", "TestSession",
			"--input", tmpCSV.Name(),
			"--comparison-mode", "trio",
			"--initial-rating", "1600",
			"--output-scale", "1-10",
			"--target-accepted", "15",
			"--verbose",
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, opts)

		// Verify CLI options
		assert.Equal(t, "TestSession", opts.SessionName)
		assert.Equal(t, tmpCSV.Name(), opts.Input)
		assert.Equal(t, "trio", opts.ComparisonMode)
		assert.Equal(t, 1600.0, opts.InitialRating)
		assert.Equal(t, "1-10", opts.OutputScale)
		assert.Equal(t, 15, opts.TargetAccepted)
		assert.True(t, opts.Verbose)
		assert.False(t, opts.Version)
	})

	t.Run("ValidResumeSession", func(t *testing.T) {
		args := []string{
			"--session-name", "ExistingSession",
			"--comparison-mode", "pairwise",
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, opts)

		// Verify CLI options
		assert.Equal(t, "ExistingSession", opts.SessionName)
		assert.Equal(t, "", opts.Input) // No input required for resume
		assert.Equal(t, "pairwise", opts.ComparisonMode)
		assert.Equal(t, 1500.0, opts.InitialRating) // Default value
		assert.Equal(t, "0-100", opts.OutputScale)  // Default value
		assert.Equal(t, 10, opts.TargetAccepted)    // Default value
		assert.False(t, opts.Verbose)
		assert.False(t, opts.Version)
	})

	t.Run("MissingSessionName", func(t *testing.T) {
		args := []string{
			"--input", tmpCSV.Name(),
		}

		_, err := ParseCLI(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session-name")
	})

	t.Run("VersionFlag", func(t *testing.T) {
		args := []string{
			"--version",
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err) // Version flag doesn't require session name
		assert.True(t, opts.Version)
	})

	t.Run("HelpFlag", func(t *testing.T) {
		args := []string{
			"--help",
		}

		_, err := ParseCLI(args)
		require.Error(t, err)
		flagsErr, ok := err.(*flags.Error)
		require.True(t, ok)
		assert.Equal(t, flags.ErrHelp, flagsErr.Type)
	})

	t.Run("InvalidComparisonMode", func(t *testing.T) {
		args := []string{
			"--session-name", "TestSession",
			"--comparison-mode", "invalid",
		}

		_, err := ParseCLI(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid comparison mode")
	})

	t.Run("InvalidOutputScale", func(t *testing.T) {
		args := []string{
			"--session-name", "TestSession",
			"--output-scale", "invalid",
		}

		_, err := ParseCLI(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid output scale")
	})

	t.Run("DefaultValues", func(t *testing.T) {
		args := []string{
			"--session-name", "TestSession",
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err)

		// Check default values
		assert.Equal(t, "pairwise", opts.ComparisonMode)
		assert.Equal(t, 1500.0, opts.InitialRating)
		assert.Equal(t, "0-100", opts.OutputScale)
		assert.Equal(t, 10, opts.TargetAccepted)
		assert.False(t, opts.Verbose)
		assert.False(t, opts.Version)
	})

	t.Run("UnexpectedArguments", func(t *testing.T) {
		args := []string{
			"--session-name", "TestSession",
			"extra", "arguments",
		}

		_, err := ParseCLI(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected arguments")
	})
}

func TestValidateInputForNewSession(t *testing.T) {
	// Create temporary CSV file
	tmpCSV, err := os.CreateTemp("", "test_*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpCSV.Name())
	tmpCSV.Close()

	t.Run("ValidInput", func(t *testing.T) {
		opts := &CLIOptions{
			Input: tmpCSV.Name(),
		}

		err := ValidateInputForNewSession(opts)
		assert.NoError(t, err)
	})

	t.Run("MissingInput", func(t *testing.T) {
		opts := &CLIOptions{}

		err := ValidateInputForNewSession(opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "input file is required")
	})

	t.Run("NonexistentFile", func(t *testing.T) {
		opts := &CLIOptions{
			Input: "nonexistent.csv",
		}

		err := ValidateInputForNewSession(opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "input file not found")
	})
}

func TestValidateSessionName(t *testing.T) {
	t.Run("ValidName", func(t *testing.T) {
		err := ValidateSessionName("ValidSessionName")
		assert.NoError(t, err)
	})

	t.Run("EmptyName", func(t *testing.T) {
		err := ValidateSessionName("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session name cannot be empty")
	})

	t.Run("InvalidCharacters", func(t *testing.T) {
		invalidNames := []string{
			"session<name>",
			"session:name",
			"session\"name",
			"session/name",
			"session\\name",
			"session|name",
			"session?name",
			"session*name",
		}

		for _, name := range invalidNames {
			err := ValidateSessionName(name)
			require.Error(t, err, "Expected error for name: %s", name)
			assert.Contains(t, err.Error(), "invalid characters")
		}
	})
}

func TestCreateSessionConfigFromCLI(t *testing.T) {
	t.Run("BasicConfiguration", func(t *testing.T) {
		opts := &CLIOptions{
			SessionName:    "TestSession",
			ComparisonMode: "pairwise",
			InitialRating:  1500.0,
			OutputScale:    "0-100",
			TargetAccepted: 10,
		}

		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "pairwise", config.UI.ComparisonMode)
		assert.Equal(t, 1500.0, config.Elo.InitialRating)
		assert.Equal(t, 10, config.Convergence.TargetAccepted)
	})

	t.Run("CustomConfiguration", func(t *testing.T) {
		opts := &CLIOptions{
			SessionName:    "CustomSession",
			ComparisonMode: "trio",
			InitialRating:  1600.0,
			OutputScale:    "1-10",
			TargetAccepted: 20,
		}

		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "trio", config.UI.ComparisonMode)
		assert.Equal(t, 1600.0, config.Elo.InitialRating)
		assert.Equal(t, 20, config.Convergence.TargetAccepted)
	})
}

func TestShowHelp(t *testing.T) {
	// This is mainly for coverage; we can't easily test the output
	ShowHelp("confelo")
	// If it doesn't panic, it's working
}

// TestCLIOnlyConfiguration verifies that the CLI-only approach works without config file dependencies
func TestCLIOnlyConfiguration(t *testing.T) {
	t.Run("NoConfigFileRequired", func(t *testing.T) {
		// Test that CLI can work in a directory with no config files
		tempDir := t.TempDir()
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)

		err = os.Chdir(tempDir)
		require.NoError(t, err)

		// Create a temporary CSV file for input
		tmpCSV, err := os.CreateTemp(tempDir, "test_*.csv")
		require.NoError(t, err)
		defer os.Remove(tmpCSV.Name())

		_, err = tmpCSV.WriteString("id,title,speaker\n1,Test Proposal,John Doe\n")
		require.NoError(t, err)
		tmpCSV.Close()

		// Verify CLI parsing works without any config files present
		args := []string{
			"--session-name", "TestSessionNoCfg",
			"--input", tmpCSV.Name(),
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, opts)

		// Verify defaults are applied correctly (no config file needed)
		assert.Equal(t, "TestSessionNoCfg", opts.SessionName)
		assert.Equal(t, tmpCSV.Name(), opts.Input)
		assert.Equal(t, "pairwise", opts.ComparisonMode)
		assert.Equal(t, 1500.0, opts.InitialRating)
		assert.Equal(t, "0-100", opts.OutputScale)
		assert.Equal(t, 10, opts.TargetAccepted)

		// Verify session config can be created from CLI options only
		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, "pairwise", config.UI.ComparisonMode)
		assert.Equal(t, 1500.0, config.Elo.InitialRating)
	})

	t.Run("AllParametersFromCLI", func(t *testing.T) {
		// Test that all configuration can be specified via CLI flags
		tmpCSV, err := os.CreateTemp("", "test_*.csv")
		require.NoError(t, err)
		defer os.Remove(tmpCSV.Name())
		tmpCSV.Close()

		args := []string{
			"--session-name", "FullCLIConfig",
			"--input", tmpCSV.Name(),
			"--comparison-mode", "quartet",
			"--initial-rating", "1600.5",
			"--output-scale", "1-10",
			"--target-accepted", "25",
			"--verbose",
		}

		opts, err := ParseCLI(args)
		require.NoError(t, err)
		require.NotNil(t, opts)

		// Verify all CLI parameters are correctly parsed
		assert.Equal(t, "FullCLIConfig", opts.SessionName)
		assert.Equal(t, tmpCSV.Name(), opts.Input)
		assert.Equal(t, "quartet", opts.ComparisonMode)
		assert.Equal(t, 1600.5, opts.InitialRating)
		assert.Equal(t, "1-10", opts.OutputScale)
		assert.Equal(t, 25, opts.TargetAccepted)
		assert.True(t, opts.Verbose)

		// Verify configuration can be created with all custom values
		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err)
		assert.Equal(t, "quartet", config.UI.ComparisonMode)
		assert.Equal(t, 1600.5, config.Elo.InitialRating)
		assert.Equal(t, 25, config.Convergence.TargetAccepted)
	})

	t.Run("NoEnvironmentVariableSupport", func(t *testing.T) {
		// Test that environment variables are not used (CLI-only approach)
		os.Setenv("CONFELO_SESSION_NAME", "FromEnv")
		os.Setenv("CONFELO_COMPARISON_MODE", "trio")
		defer os.Unsetenv("CONFELO_SESSION_NAME")
		defer os.Unsetenv("CONFELO_COMPARISON_MODE")

		// Parse CLI args without session-name (should fail, not use env var)
		args := []string{
			"--input", "test.csv",
		}

		_, err := ParseCLI(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session name is required")
	})
}

func TestApplyOutputScale(t *testing.T) {
	t.Run("IntegerScale0To100", func(t *testing.T) {
		config := DefaultSessionConfig()
		err := applyOutputScale(&config, "0-100")
		require.NoError(t, err)

		assert.Equal(t, 0.0, config.Elo.OutputMin)
		assert.Equal(t, 100.0, config.Elo.OutputMax)
		assert.False(t, config.Elo.UseDecimals)
	})

	t.Run("DecimalScale1To5", func(t *testing.T) {
		config := DefaultSessionConfig()
		err := applyOutputScale(&config, "1.0-5.0")
		require.NoError(t, err)

		assert.Equal(t, 1.0, config.Elo.OutputMin)
		assert.Equal(t, 5.0, config.Elo.OutputMax)
		assert.True(t, config.Elo.UseDecimals)
	})

	t.Run("MixedDecimalScale", func(t *testing.T) {
		config := DefaultSessionConfig()
		err := applyOutputScale(&config, "0.0-10")
		require.NoError(t, err)

		assert.Equal(t, 0.0, config.Elo.OutputMin)
		assert.Equal(t, 10.0, config.Elo.OutputMax)
		assert.True(t, config.Elo.UseDecimals) // Has decimals in one value
	})

	t.Run("EmptyScaleUsesDefaults", func(t *testing.T) {
		config := DefaultSessionConfig()
		originalMin := config.Elo.OutputMin
		originalMax := config.Elo.OutputMax

		err := applyOutputScale(&config, "")
		require.NoError(t, err)

		// Should not change defaults
		assert.Equal(t, originalMin, config.Elo.OutputMin)
		assert.Equal(t, originalMax, config.Elo.OutputMax)
	})

	t.Run("InvalidFormatMissingDash", func(t *testing.T) {
		config := DefaultSessionConfig()
		err := applyOutputScale(&config, "100")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scale format")
	})

	t.Run("InvalidFormatNonNumeric", func(t *testing.T) {
		config := DefaultSessionConfig()
		err := applyOutputScale(&config, "zero-hundred")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid minimum value")
	})

	t.Run("InvalidRangeMinGreaterThanMax", func(t *testing.T) {
		config := DefaultSessionConfig()
		err := applyOutputScale(&config, "100-0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be less than")
	})

	t.Run("InvalidRangeEqualValues", func(t *testing.T) {
		config := DefaultSessionConfig()
		err := applyOutputScale(&config, "50-50")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be less than")
	})
}

func TestEloConfig_CalculateExportScore(t *testing.T) {
	t.Run("LinearScaling0To100", func(t *testing.T) {
		config := EloConfig{
			MinRating:   0.0,
			MaxRating:   3000.0,
			OutputMin:   0.0,
			OutputMax:   100.0,
			UseDecimals: false,
		}

		// Test various Elo scores
		assert.Equal(t, 0.0, config.CalculateExportScore(0.0))      // Min rating
		assert.Equal(t, 50.0, config.CalculateExportScore(1500.0))  // Mid rating
		assert.Equal(t, 100.0, config.CalculateExportScore(3000.0)) // Max rating
		assert.Equal(t, 25.0, config.CalculateExportScore(750.0))   // Quarter
		assert.Equal(t, 75.0, config.CalculateExportScore(2250.0))  // Three quarters
	})

	t.Run("LinearScaling1To5Decimals", func(t *testing.T) {
		config := EloConfig{
			MinRating:   0.0,
			MaxRating:   3000.0,
			OutputMin:   1.0,
			OutputMax:   5.0,
			UseDecimals: true,
		}

		// Test various Elo scores
		assert.InDelta(t, 1.0, config.CalculateExportScore(0.0), 0.01)    // Min rating
		assert.InDelta(t, 3.0, config.CalculateExportScore(1500.0), 0.01) // Mid rating
		assert.InDelta(t, 5.0, config.CalculateExportScore(3000.0), 0.01) // Max rating
		assert.InDelta(t, 2.0, config.CalculateExportScore(750.0), 0.01)  // Quarter
		assert.InDelta(t, 4.0, config.CalculateExportScore(2250.0), 0.01) // Three quarters
	})

	t.Run("ClampingBelowMinRating", func(t *testing.T) {
		config := EloConfig{
			MinRating:   0.0,
			MaxRating:   3000.0,
			OutputMin:   0.0,
			OutputMax:   100.0,
			UseDecimals: false,
		}

		// Scores below MinRating should clamp to OutputMin
		assert.Equal(t, 0.0, config.CalculateExportScore(-100.0))
		assert.Equal(t, 0.0, config.CalculateExportScore(-1000.0))
	})

	t.Run("ClampingAboveMaxRating", func(t *testing.T) {
		config := EloConfig{
			MinRating:   0.0,
			MaxRating:   3000.0,
			OutputMin:   0.0,
			OutputMax:   100.0,
			UseDecimals: false,
		}

		// Scores above MaxRating should clamp to OutputMax
		assert.Equal(t, 100.0, config.CalculateExportScore(3500.0))
		assert.Equal(t, 100.0, config.CalculateExportScore(10000.0))
	})

	t.Run("IntegerRoundingWhenUseDecimalsFalse", func(t *testing.T) {
		config := EloConfig{
			MinRating:   0.0,
			MaxRating:   3000.0,
			OutputMin:   0.0,
			OutputMax:   100.0,
			UseDecimals: false,
		}

		// Test rounding behavior
		assert.Equal(t, 33.0, config.CalculateExportScore(1000.0)) // Should round 33.33 to 33
		assert.Equal(t, 67.0, config.CalculateExportScore(2000.0)) // Should round 66.67 to 67
	})

	t.Run("DecimalPrecisionWhenUseDecimalsTrue", func(t *testing.T) {
		config := EloConfig{
			MinRating:   0.0,
			MaxRating:   3000.0,
			OutputMin:   0.0,
			OutputMax:   100.0,
			UseDecimals: true,
		}

		// Test decimal precision
		result := config.CalculateExportScore(1000.0)
		assert.InDelta(t, 33.333, result, 0.01) // Should keep decimals
	})
}
