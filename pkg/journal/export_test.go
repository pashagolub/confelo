package journal

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExporter_CSVExport tests CSV export functionality
func TestExporter_CSVExport(t *testing.T) {
	tests := []struct {
		name           string
		session        *Session
		options        ExportOptions
		expectedHeader []string
		expectedRows   int
		wantErr        bool
	}{
		{
			name: "basic CSV export with original format preservation",
			session: &Session{
				ID:   "test-session-1",
				Name: "Test Session",
				Proposals: []Proposal{
					{
						ID:       "p1",
						Title:    "First Proposal",
						Abstract: "Abstract 1",
						Speaker:  "Speaker 1",
						Score:    1650.0,
						Metadata: map[string]string{
							"category": "tech",
							"track":    "ai",
						},
					},
					{
						ID:       "p2",
						Title:    "Second Proposal",
						Abstract: "Abstract 2",
						Speaker:  "Speaker 2",
						Score:    1450.0,
						Metadata: map[string]string{
							"category": "business",
							"track":    "strategy",
						},
					},
				},
			},
			options: ExportOptions{
				Format:       FormatCSV,
				IncludeStats: false,
			},
			expectedHeader: []string{"id", "title", "abstract", "speaker", "score", "category", "track"},
			expectedRows:   2,
			wantErr:        false,
		},
		{
			name: "CSV export with statistics included",
			session: &Session{
				ID:   "test-session-2",
				Name: "Test Session 2",
				Proposals: []Proposal{
					{
						ID:      "p1",
						Title:   "First Proposal",
						Speaker: "Speaker 1",
						Score:   1650.0,
					},
				},
				ConvergenceMetrics: &ConvergenceMetrics{
					ConvergenceScore:   0.95,
					RankingStability:   0.88,
					CoveragePercentage: 100.0,
				},
			},
			options: ExportOptions{
				Format:       FormatCSV,
				IncludeStats: true,
			},
			expectedHeader: []string{"id", "title", "speaker", "score", "rank", "confidence_score"},
			expectedRows:   1,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := NewExporter()

			var output strings.Builder
			err := exporter.ExportCSV(tt.session, &output, tt.options)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Parse the CSV output
			reader := csv.NewReader(strings.NewReader(output.String()))
			records, err := reader.ReadAll()
			require.NoError(t, err)

			// Verify header
			assert.Equal(t, tt.expectedHeader, records[0])

			// Verify row count (header + data rows)
			assert.Equal(t, tt.expectedRows+1, len(records))

			// Verify data is sorted by score (descending)
			if len(records) > 2 {
				for i := 1; i < len(records)-1; i++ {
					currentScore := parseFloat(t, records[i][4]) // score column
					nextScore := parseFloat(t, records[i+1][4])
					assert.GreaterOrEqual(t, currentScore, nextScore, "CSV should be sorted by score descending")
				}
			}
		})
	}
}

// TestExporter_JSONExport tests JSON export functionality
func TestExporter_JSONExport(t *testing.T) {
	session := &Session{
		ID:   "test-session-json",
		Name: "JSON Test Session",
		Proposals: []Proposal{
			{
				ID:        "p1",
				Title:     "First Proposal",
				Score:     1650.0,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		CompletedComparisons: []Comparison{
			{
				ID:          "c1",
				SessionID:   "test-session-json",
				ProposalIDs: []string{"p1", "p2"},
				WinnerID:    "p1",
				Method:      "pairwise",
				Timestamp:   time.Now(),
			},
		},
	}

	tests := []struct {
		name    string
		options ExportOptions
		wantErr bool
	}{
		{
			name: "basic JSON export",
			options: ExportOptions{
				Format: FormatJSON,
			},
			wantErr: false,
		},
		{
			name: "JSON export with audit trail",
			options: ExportOptions{
				Format:       FormatJSON,
				IncludeAudit: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := NewExporter()

			var output strings.Builder
			err := exporter.ExportJSON(session, &output, tt.options)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify valid JSON
			var result map[string]interface{}
			err = json.Unmarshal([]byte(output.String()), &result)
			require.NoError(t, err)

			// Verify required fields
			assert.Contains(t, result, "session_id")
			assert.Contains(t, result, "rankings")
			assert.Equal(t, session.ID, result["session_id"])

			if tt.options.IncludeAudit {
				assert.Contains(t, result, "comparisons")
			}
		})
	}
}

// TestExporter_RankingReport tests ranking report generation
func TestExporter_RankingReport(t *testing.T) {
	session := &Session{
		ID:   "test-ranking-report",
		Name: "Ranking Report Test",
		Proposals: []Proposal{
			{
				ID:      "p1",
				Title:   "Top Proposal",
				Speaker: "Speaker 1",
				Score:   1750.0,
			},
			{
				ID:      "p2",
				Title:   "Second Proposal",
				Speaker: "Speaker 2",
				Score:   1650.0,
			},
			{
				ID:      "p3",
				Title:   "Third Proposal",
				Speaker: "Speaker 3",
				Score:   1550.0,
			},
		},
		ConvergenceMetrics: &ConvergenceMetrics{
			ConvergenceScore:   0.92,
			RankingStability:   0.85,
			CoveragePercentage: 100.0,
		},
		CompletedComparisons: make([]Comparison, 10), // Simulate 10 comparisons
	}

	exporter := NewExporter()

	var output strings.Builder
	err := exporter.ExportRankingReport(session, &output, ExportOptions{})
	require.NoError(t, err)

	report := output.String()

	// Verify report contains expected sections
	assert.Contains(t, report, "Conference Talk Ranking Report")
	assert.Contains(t, report, session.Name)
	assert.Contains(t, report, "Top Proposal") // First ranked proposal
	assert.Contains(t, report, "Convergence Score: 92.0%")
	assert.Contains(t, report, "Total Comparisons: 10")

	// Verify ranking order in report
	topIndex := strings.Index(report, "Top Proposal")
	secondIndex := strings.Index(report, "Second Proposal")
	thirdIndex := strings.Index(report, "Third Proposal")

	assert.True(t, topIndex < secondIndex, "Top proposal should appear before second")
	assert.True(t, secondIndex < thirdIndex, "Second proposal should appear before third")
}

// TestExporter_FileOperations tests file-based export operations
func TestExporter_FileOperations(t *testing.T) {
	tmpDir := t.TempDir()

	session := &Session{
		ID:   "test-file-ops",
		Name: "File Operations Test",
		Proposals: []Proposal{
			{
				ID:    "p1",
				Title: "Test Proposal",
				Score: 1600.0,
			},
		},
	}

	tests := []struct {
		name     string
		filename string
		format   ExportFormat
		wantErr  bool
	}{
		{
			name:     "export to CSV file",
			filename: "rankings.csv",
			format:   FormatCSV,
			wantErr:  false,
		},
		{
			name:     "export to JSON file",
			filename: "rankings.json",
			format:   FormatJSON,
			wantErr:  false,
		},
		{
			name:     "export to text report",
			filename: "report.txt",
			format:   FormatText,
			wantErr:  false,
		},
		{
			name:     "invalid directory",
			filename: "NUL/invalid/rankings.csv", // NUL is invalid on Windows
			format:   FormatCSV,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := NewExporter()

			var filePath string
			if !tt.wantErr {
				filePath = filepath.Join(tmpDir, tt.filename)
			} else {
				filePath = tt.filename
			}

			err := exporter.ExportToFile(session, filePath, ExportOptions{Format: tt.format})

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify file was created and has content
			info, err := os.Stat(filePath)
			require.NoError(t, err)
			assert.Greater(t, info.Size(), int64(0), "Export file should not be empty")

			// Verify file content format
			content, err := os.ReadFile(filePath)
			require.NoError(t, err)

			switch tt.format {
			case FormatCSV:
				assert.Contains(t, string(content), "id,title")
			case FormatJSON:
				var jsonData map[string]interface{}
				err = json.Unmarshal(content, &jsonData)
				assert.NoError(t, err, "Should be valid JSON")
			case FormatText:
				assert.Contains(t, string(content), "Conference Talk Ranking Report")
			}
		})
	}
}

// TestExporter_LargeDataset tests export performance with large datasets
func TestExporter_LargeDataset(t *testing.T) {
	// Create session with 200 proposals (constitutional limit)
	proposals := make([]Proposal, 200)
	for i := 0; i < 200; i++ {
		proposals[i] = Proposal{
			ID:      fmt.Sprintf("p%d", i),
			Title:   fmt.Sprintf("Proposal %d", i),
			Speaker: fmt.Sprintf("Speaker %d", i),
			Score:   1500.0 + float64(i), // Varied scores
			Metadata: map[string]string{
				"category": fmt.Sprintf("cat%d", i%5),
				"track":    fmt.Sprintf("track%d", i%3),
			},
		}
	}

	session := &Session{
		ID:        "large-dataset-test",
		Name:      "Large Dataset Test",
		Proposals: proposals,
	}

	exporter := NewExporter()

	// Test CSV export performance
	start := time.Now()
	var output strings.Builder
	err := exporter.ExportCSV(session, &output, ExportOptions{Format: FormatCSV})
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, duration, time.Second, "Large dataset export should complete within 1 second")

	// Verify all proposals are exported
	lines := strings.Split(output.String(), "\n")
	assert.Equal(t, 202, len(lines), "Should have header + 200 data rows + final newline")
}

// TestExporter_CustomTemplates tests custom export template functionality
func TestExporter_CustomTemplates(t *testing.T) {
	session := &Session{
		ID:   "template-test",
		Name: "Template Test",
		Proposals: []Proposal{
			{
				ID:      "p1",
				Title:   "Template Proposal",
				Speaker: "Template Speaker",
				Score:   1700.0,
			},
		},
	}

	template := ExportTemplate{
		Name:         "Custom Template",
		Description:  "Test template",
		HeaderFormat: "Custom Export Report\nGenerated: {{.Timestamp}}\n",
		RowFormat:    "{{.Rank}}. {{.Title}} by {{.Speaker}} (Score: {{.Score}})\n",
		FooterFormat: "\nTotal Proposals: {{.TotalProposals}}\n",
	}

	exporter := NewExporter()

	var output strings.Builder
	err := exporter.ExportWithTemplate(session, &output, template, ExportOptions{})
	require.NoError(t, err)

	result := output.String()

	// Verify template formatting
	assert.Contains(t, result, "Custom Export Report")
	assert.Contains(t, result, "1. Template Proposal by Template Speaker")
	assert.Contains(t, result, "Total Proposals: 1")
}

// Helper function to parse float from string in tests
func parseFloat(t *testing.T, s string) float64 {
	t.Helper()
	val, err := strconv.ParseFloat(s, 64)
	require.NoError(t, err)
	return val
}
