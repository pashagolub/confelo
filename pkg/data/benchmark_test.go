// Package data provides benchmark tests for constitutional compliance validation.
// These tests ensure the application meets performance requirements for memory usage,
// response times, and throughput as specified in the constitutional requirements.
package data

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// BenchmarkCSVLoading tests CSV loading performance for constitutional compliance
func BenchmarkCSVLoading(b *testing.B) {
	// Create test directory
	testDir := filepath.Join(".", "benchmark_test")
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	// Generate test data sets of different sizes
	testCases := []struct {
		name      string
		size      int
		maxTimeMs int64
	}{
		{"Small_10_proposals", 10, 50},
		{"Medium_50_proposals", 50, 200},
		{"Large_200_proposals", 200, 1000}, // Constitutional requirement: <1s for 200 proposals
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Generate CSV data
			csvData := generateBenchmarkCSV(tc.size)
			csvFile := filepath.Join(testDir, fmt.Sprintf("benchmark_%d.csv", tc.size))
			err := os.WriteFile(csvFile, []byte(csvData), 0644)
			if err != nil {
				b.Fatal(err)
			}

			storage := NewFileStorage()
			config := DefaultSessionConfig()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()
				result, err := storage.LoadProposalsFromCSV(csvFile, config.CSV)
				duration := time.Since(start)

				if err != nil {
					b.Fatal(err)
				}
				if len(result.Proposals) != tc.size {
					b.Fatalf("Expected %d proposals, got %d", tc.size, len(result.Proposals))
				}
				if duration.Milliseconds() > tc.maxTimeMs {
					b.Fatalf("Loading took %dms, expected <%dms", duration.Milliseconds(), tc.maxTimeMs)
				}
			}
		})
	}
}

// BenchmarkMemoryUsage tests memory usage for constitutional compliance
func BenchmarkMemoryUsage(b *testing.B) {
	testDir := filepath.Join(".", "benchmark_memory")
	_ = os.MkdirAll(testDir, 0755)
	defer func() { _ = os.RemoveAll(testDir) }()

	b.Run("Memory_Usage_200_Proposals", func(b *testing.B) {
		// Constitutional requirement: <100MB for 200 proposals
		const maxMemoryMB = 100

		// Generate test data
		csvData := generateBenchmarkCSV(200)
		csvFile := filepath.Join(testDir, "memory_test.csv")
		err := os.WriteFile(csvFile, []byte(csvData), 0644)
		if err != nil {
			b.Fatal(err)
		}

		storage := NewFileStorage()
		config := DefaultSessionConfig()

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// Force garbage collection and get baseline memory
			runtime.GC()
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)
			baselineAlloc := m1.Alloc

			// Load proposals
			result, err := storage.LoadProposalsFromCSV(csvFile, config.CSV)
			if err != nil {
				b.Fatal(err)
			}

			// Create session to simulate full application state
			s := &Session{
				Name:      fmt.Sprintf("memory_test_%d", i),
				Status:    "active",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Proposals: result.Proposals,
			}

			_, _ = fmt.Fprintln(io.Discard, s)

			// Get memory after loading
			runtime.GC()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)
			usedMemoryBytes := m2.Alloc - baselineAlloc
			usedMemoryMB := usedMemoryBytes / (1024 * 1024)

			if usedMemoryMB > maxMemoryMB {
				b.Fatalf("Memory usage %dMB exceeds constitutional limit of %dMB", usedMemoryMB, maxMemoryMB)
			}

			// Report memory usage
			b.ReportMetric(float64(usedMemoryMB), "MB")

			// Clean up
			result = nil
		}
	})
}

// BenchmarkSessionOperations tests session management performance
func BenchmarkSessionOperations(b *testing.B) {
	testDir := filepath.Join(".", "session_benchmark")
	_ = os.MkdirAll(testDir, 0755)
	defer func() { _ = os.RemoveAll(testDir) }()

	b.Run("Session_Save_Load_Performance", func(b *testing.B) {
		// Constitutional requirement: Session operations should be fast
		storage := NewFileStorage()

		// Create test session with substantial data
		proposals := make([]Proposal, 50)
		for i := 0; i < 50; i++ {
			proposals[i] = Proposal{
				ID:        fmt.Sprintf("bench_prop_%d", i+1),
				Title:     fmt.Sprintf("Benchmark Proposal %d", i+1),
				Abstract:  fmt.Sprintf("Detailed abstract for benchmark proposal %d", i+1),
				Speaker:   fmt.Sprintf("Speaker %d", i+1),
				Score:     1500.0 + float64(i*5),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
		}

		session := &Session{
			Name:      "benchmark_session",
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Proposals: proposals,
		}

		sessionFile := filepath.Join(testDir, "benchmark_session.json")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Test save performance
			start := time.Now()
			err := storage.SaveSession(session, sessionFile)
			saveTime := time.Since(start)

			if err != nil {
				b.Fatal(err)
			}
			if saveTime.Milliseconds() > 100 { // Should be very fast
				b.Fatalf("Session save took %dms, expected <100ms", saveTime.Milliseconds())
			}

			// Test load performance
			start = time.Now()
			loadedSession, err := storage.LoadSession(sessionFile)
			loadTime := time.Since(start)

			if err != nil {
				b.Fatal(err)
			}
			if loadTime.Milliseconds() > 100 { // Should be very fast
				b.Fatalf("Session load took %dms, expected <100ms", loadTime.Milliseconds())
			}

			if len(loadedSession.Proposals) != len(session.Proposals) {
				b.Fatal("Session load integrity check failed")
			}
		}
	})
}

// generateBenchmarkCSV creates test CSV data for benchmarking
func generateBenchmarkCSV(count int) string {
	var builder strings.Builder
	builder.WriteString("id,title,abstract,speaker,score\n")

	for i := 1; i <= count; i++ {
		abstract := fmt.Sprintf("This is a comprehensive abstract for proposal %d that contains substantial text to simulate real-world proposal content with detailed descriptions, technical details, and learning outcomes that would be typical in conference submissions.", i)

		builder.WriteString(fmt.Sprintf("%d,\"Benchmark Proposal %d: Advanced Technical Topic\",\"%s\",\"Dr. Speaker %d\",\n", i, i, abstract, i))
	}

	return builder.String()
}

// TestConstitutionalRequirements runs quick validation tests for constitutional compliance
func TestConstitutionalRequirements(t *testing.T) {

	t.Run("Cross Platform Compatibility", func(t *testing.T) {
		// Test that the application handles cross-platform file paths and line endings
		testDir := filepath.Join(".", "cross_platform_test")
		_ = os.MkdirAll(testDir, 0755)
		defer func() { _ = os.RemoveAll(testDir) }()

		storage := NewFileStorage()
		config := DefaultSessionConfig()

		// Test with different line endings (Windows CRLF)
		csvWithCRLF := "id,title,abstract,speaker,score\r\n1,\"Test Proposal\",\"Test abstract\",\"Test Speaker\",\r\n"
		csvFile := filepath.Join(testDir, "crlf_test.csv")
		err := os.WriteFile(csvFile, []byte(csvWithCRLF), 0644)
		if err != nil {
			t.Fatal(err)
		}

		result, err := storage.LoadProposalsFromCSV(csvFile, config.CSV)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Proposals) != 1 {
			t.Errorf("Expected 1 proposal, got %d (CRLF handling failed)", len(result.Proposals))
		}

		// Test nested directory creation (cross-platform paths)
		nestedDir := filepath.Join(testDir, "level1", "level2")
		nestedStorage := NewFileStorage() // Should create nested directories

		session := &Session{
			Name:      "cross_platform_session",
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Proposals: result.Proposals,
		}

		nestedFile := filepath.Join(nestedDir, "nested_session.json")
		err = nestedStorage.SaveSession(session, nestedFile)
		if err != nil {
			t.Fatal(err)
		}

		// Verify file was created in correct location
		if _, err := os.Stat(nestedFile); os.IsNotExist(err) {
			t.Error("Nested file creation failed - cross-platform path handling issue")
		}
	})
}
