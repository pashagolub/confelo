package data

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStartupPerformance validates that startup time meets the <200ms requirement
func TestStartupPerformance(t *testing.T) {
	t.Run("CLIParsingPerformance", func(t *testing.T) {
		start := time.Now()

		args := []string{
			"--session-name", "PerfTest",
			"--input", "test.csv",
			"--comparison-mode", "trio",
			"--initial-rating", "1600",
			"--target-accepted", "15",
			"--verbose",
		}

		_, err := ParseCLI(args)
		require.NoError(t, err)

		elapsed := time.Since(start)

		// CLI parsing should be very fast - well under 200ms
		assert.Less(t, elapsed.Milliseconds(), int64(50),
			"CLI parsing should be <50ms, got %v", elapsed)
	})

	t.Run("ModeDetectionPerformance", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")
		err := os.MkdirAll(sessionsDir, 0755)
		require.NoError(t, err)

		detector := NewSessionDetector(sessionsDir)

		start := time.Now()

		// Test mode detection for new session
		_, err = detector.DetectMode("NewPerfTest")
		require.NoError(t, err)

		elapsed := time.Since(start)

		// Mode detection should be very fast
		assert.Less(t, elapsed.Milliseconds(), int64(50),
			"Mode detection should be <50ms, got %v", elapsed)
	})

	t.Run("ConfigCreationPerformance", func(t *testing.T) {
		opts := &CLIOptions{
			SessionName:    "PerfTest",
			ComparisonMode: "trio",
			InitialRating:  1600.0,
			OutputScale:    "0-100",
			TargetAccepted: 15,
		}

		start := time.Now()

		_, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err)

		elapsed := time.Since(start)

		// Config creation should be very fast
		assert.Less(t, elapsed.Milliseconds(), int64(10),
			"Config creation should be <10ms, got %v", elapsed)
	})

	t.Run("OverallStartupSimulation", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")

		// Create test CSV
		testCSV := filepath.Join(tempDir, "test.csv")
		csvData := "id,title,speaker\n1,Test A,Speaker A\n2,Test B,Speaker B\n"
		err := os.WriteFile(testCSV, []byte(csvData), 0644)
		require.NoError(t, err)

		start := time.Now()

		// Simulate complete startup process for new session
		args := []string{
			"--session-name", "StartupPerfTest",
			"--input", testCSV,
		}

		// 1. Parse CLI
		opts, err := ParseCLI(args)
		require.NoError(t, err)

		// 2. Detect mode
		detector := NewSessionDetector(sessionsDir)
		mode, err := detector.DetectMode(opts.SessionName)
		require.NoError(t, err)
		assert.Equal(t, StartMode, mode)

		// 3. Create config
		config, err := CreateSessionConfigFromCLI(opts)
		require.NoError(t, err)

		// 4. Load CSV (for new session)
		storage := &FileStorage{}
		_, err = storage.LoadProposalsFromCSV(testCSV, config.CSV)
		require.NoError(t, err)

		elapsed := time.Since(start)

		// Overall startup should meet the <200ms requirement
		assert.Less(t, elapsed.Milliseconds(), int64(200),
			"Overall startup should be <200ms, got %v", elapsed)

		t.Logf("Startup performance: %v (target: <200ms)", elapsed)
	})

	t.Run("ResumeSessionPerformance", func(t *testing.T) {
		tempDir := t.TempDir()
		sessionsDir := filepath.Join(tempDir, "sessions")
		err := os.MkdirAll(sessionsDir, 0755)
		require.NoError(t, err)

		// Create existing session file
		sessionName := "ResumePerfTest"
		err = createTestSessionFileForIntegration(sessionsDir, sessionName)
		require.NoError(t, err)
		defer cleanupTestSessionFileForIntegration(sessionsDir, sessionName)

		start := time.Now()

		// Simulate resume session startup process
		args := []string{
			"--session-name", sessionName,
		}

		// 1. Parse CLI
		opts, err := ParseCLI(args)
		require.NoError(t, err)

		// 2. Detect mode
		detector := NewSessionDetector(sessionsDir)
		mode, err := detector.DetectMode(opts.SessionName)
		require.NoError(t, err)
		assert.Equal(t, ResumeMode, mode)

		// 3. Find and load session
		sessionFile, err := detector.FindSessionFile(opts.SessionName)
		require.NoError(t, err)

		storage := &FileStorage{}
		_, err = storage.LoadSession(sessionFile)
		require.NoError(t, err)

		elapsed := time.Since(start)

		// Resume session should also meet the <200ms requirement
		assert.Less(t, elapsed.Milliseconds(), int64(200),
			"Resume session should be <200ms, got %v", elapsed)

		t.Logf("Resume performance: %v (target: <200ms)", elapsed)
	})
}
