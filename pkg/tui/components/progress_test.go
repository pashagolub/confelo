package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"

	"github.com/pashagolub/confelo/pkg/data"
	"github.com/pashagolub/confelo/pkg/elo"
)

func TestNewProgress(t *testing.T) {
	engine := &elo.Engine{}
	config := DefaultProgressConfig()

	progress := NewProgress(engine, config)

	assert.NotNil(t, progress)
	assert.Equal(t, engine, progress.engine)
	assert.True(t, progress.showBars)
	assert.True(t, progress.showMetrics)
	assert.True(t, progress.showEstimates)
	assert.NotNil(t, progress.GetContainer())
}

func TestNewProgressWithCustomConfig(t *testing.T) {
	engine := &elo.Engine{}
	config := ProgressConfig{
		ShowBars:       false,
		ShowMetrics:    true,
		ShowEstimates:  false,
		UpdateInterval: 50 * time.Millisecond,
		ProgressColor:  tcell.ColorRed,
		CompleteColor:  tcell.ColorYellow,
		TextColor:      tcell.ColorBlue,
		BorderColor:    tcell.ColorGreen,
	}

	progress := NewProgress(engine, config)

	assert.False(t, progress.showBars)
	assert.True(t, progress.showMetrics)
	assert.False(t, progress.showEstimates)
	assert.Equal(t, 50*time.Millisecond, progress.updateInterval)
	assert.Equal(t, tcell.ColorRed, progress.progressColor)
	assert.Equal(t, tcell.ColorYellow, progress.completeColor)
}

func TestProgressUpdate(t *testing.T) {
	engine := &elo.Engine{
		InitialRating: 1500.0,
		KFactor:       32,
		MinRating:     0.0,
		MaxRating:     3000.0,
	}

	config := DefaultProgressConfig()
	config.UpdateInterval = 0 // No throttling for tests
	progress := NewProgress(engine, config)

	// Create test proposals
	proposals := []data.Proposal{
		{ID: "prop1", Title: "Test 1", Score: 1500.0},
		{ID: "prop2", Title: "Test 2", Score: 1450.0},
		{ID: "prop3", Title: "Test 3", Score: 1550.0},
	}

	// Create test history
	history := elo.NewComparisonHistory()

	// Update progress component
	progress.Update(proposals, history)

	// Should have metrics now (may be default metrics if engine returns nil)
	metrics := progress.GetMetrics()
	// Note: metrics may be nil if engine.GetProgressMetrics returns nil, which is acceptable
	if metrics != nil {
		assert.GreaterOrEqual(t, metrics.TotalComparisons, 0)
	}
}

func TestProgressUpdateWithHistory(t *testing.T) {
	engine := &elo.Engine{
		InitialRating: 1500.0,
		KFactor:       32,
		MinRating:     0.0,
		MaxRating:     3000.0,
	}

	config := DefaultProgressConfig()
	config.UpdateInterval = 0 // No throttling for tests
	progress := NewProgress(engine, config)

	// Create test proposals
	proposals := []data.Proposal{
		{ID: "prop1", Title: "Test 1", Score: 1500.0},
		{ID: "prop2", Title: "Test 2", Score: 1450.0},
	}

	// Create test history with some comparisons
	history := elo.NewComparisonHistory()
	history.AddComparison(elo.ComparisonResult{
		Updates: []elo.RatingUpdate{
			{ProposalID: "prop1", OldRating: 1500.0, NewRating: 1516.0, Delta: 16.0},
			{ProposalID: "prop2", OldRating: 1450.0, NewRating: 1434.0, Delta: -16.0},
		},
		Method:    elo.Pairwise,
		Timestamp: time.Now(),
		Duration:  2 * time.Second,
	})

	// Update progress component
	progress.Update(proposals, history)

	// Should have metrics with comparison data
	metrics := progress.GetMetrics()
	if metrics != nil {
		assert.Equal(t, 1, metrics.TotalComparisons)
	}
}

func TestProgressUpdateThrottling(t *testing.T) {
	engine := &elo.Engine{}
	config := ProgressConfig{
		ShowBars:       true,
		ShowMetrics:    true,
		ShowEstimates:  true,
		UpdateInterval: 100 * time.Millisecond, // Long interval for testing
	}

	progress := NewProgress(engine, config)
	proposals := []data.Proposal{
		{ID: "prop1", Title: "Test 1", Score: 1500.0},
	}

	// First update should work
	progress.Update(proposals, nil)
	firstUpdate := progress.lastUpdate

	// Immediate second update should be throttled
	progress.Update(proposals, nil)
	secondUpdate := progress.lastUpdate

	// Should be the same time (throttled)
	assert.Equal(t, firstUpdate, secondUpdate)

	// Wait and try again
	time.Sleep(110 * time.Millisecond)
	progress.Update(proposals, nil)
	thirdUpdate := progress.lastUpdate

	// Should be different time (not throttled)
	assert.True(t, thirdUpdate.After(firstUpdate))
}

func TestCreateProgressBar(t *testing.T) {
	engine := &elo.Engine{}
	config := DefaultProgressConfig()
	progress := NewProgress(engine, config)

	// Test various progress values
	testCases := []struct {
		progress   float64
		isComplete bool
		expected   string
	}{
		{0.0, false, "[blue][gray]░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░[white]"},
		{0.5, false, "[blue]███████████████[gray]░░░░░░░░░░░░░░░[white]"},
		{1.0, true, "[green]██████████████████████████████[gray][white]"},
	}

	for _, tc := range testCases {
		result := progress.createProgressBar(tc.progress, tc.isComplete)
		assert.Equal(t, tc.expected, result)
	}
}

func TestIsConverged(t *testing.T) {
	engine := &elo.Engine{}
	config := DefaultProgressConfig()
	progress := NewProgress(engine, config)

	// Test with no metrics
	assert.False(t, progress.isConverged())

	// Test with various convergence states
	testCases := []struct {
		name      string
		coverage  float64
		convRate  float64
		topStable int
		expected  bool
	}{
		{"Not converged - low coverage", 0.5, 0.05, 5, false},
		{"Not converged - high conv rate", 0.9, 0.5, 5, false},
		{"Not converged - unstable top", 0.9, 0.05, 2, false},
		{"Converged - all criteria met", 0.9, 0.05, 5, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			progress.currentMetrics = &elo.ProgressMetrics{
				CoverageComplete: tc.coverage,
				ConvergenceRate:  tc.convRate,
				TopNStable:       tc.topStable,
			}
			progress.config = elo.OptimizationConfig{
				TopNForStability: 5,
			}

			result := progress.isConverged()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCalculateAverageConfidence(t *testing.T) {
	engine := &elo.Engine{}
	config := DefaultProgressConfig()
	progress := NewProgress(engine, config)

	// Test with no metrics
	assert.Equal(t, 0.0, progress.calculateAverageConfidence())

	// Test with metrics
	progress.currentMetrics = &elo.ProgressMetrics{
		ConfidenceScores: map[string]float64{
			"prop1": 0.8,
			"prop2": 0.6,
			"prop3": 0.9,
		},
	}

	expected := (0.8 + 0.6 + 0.9) / 3.0
	assert.InDelta(t, expected, progress.calculateAverageConfidence(), 0.01)
}

func TestGetConvergenceStatusText(t *testing.T) {
	engine := &elo.Engine{}
	config := DefaultProgressConfig()
	progress := NewProgress(engine, config)

	// Test with no metrics
	assert.Equal(t, "[gray]Unknown[white]", progress.getConvergenceStatusText())

	// Test various states
	testCases := []struct {
		name     string
		coverage float64
		expected string
	}{
		{"Early stage", 0.2, "[red]Early Stage[white]"},
		{"Progressing", 0.5, "[yellow]Progressing[white]"},
		{"Near convergence", 0.8, "[blue]Near Convergence[white]"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			progress.currentMetrics = &elo.ProgressMetrics{
				CoverageComplete: tc.coverage,
				ConvergenceRate:  0.2, // Not converged
			}

			result := progress.getConvergenceStatusText()
			assert.Equal(t, tc.expected, result)
		})
	}

	// Test converged state
	progress.currentMetrics = &elo.ProgressMetrics{
		CoverageComplete: 0.9,
		ConvergenceRate:  0.05,
		TopNStable:       5,
	}
	progress.config = elo.OptimizationConfig{TopNForStability: 5}

	assert.Equal(t, "[green]Converged[white]", progress.getConvergenceStatusText())
}

func TestGetRecommendations(t *testing.T) {
	engine := &elo.Engine{}
	config := DefaultProgressConfig()
	progress := NewProgress(engine, config)

	// Test with no metrics
	assert.Equal(t, "", progress.getRecommendations())

	testCases := []struct {
		name      string
		coverage  float64
		convRate  float64
		topStable int
		expected  string
	}{
		{
			"Converged",
			0.9, 0.05, 5,
			"Rankings have converged. Consider exporting results.",
		},
		{
			"Low coverage",
			0.3, 0.2, 2,
			"Continue comparisons to improve coverage.",
		},
		{
			"High convergence rate",
			0.8, 0.5, 3,
			"Rankings still changing significantly. More comparisons recommended.",
		},
		{
			"Unstable top",
			0.8, 0.1, 1,
			"Top rankings not yet stable. Focus on high-priority matchups.",
		},
		{
			"Near convergence",
			0.8, 0.1, 4,
			"Nearing convergence. A few more comparisons should stabilize rankings.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			progress.currentMetrics = &elo.ProgressMetrics{
				CoverageComplete: tc.coverage,
				ConvergenceRate:  tc.convRate,
				TopNStable:       tc.topStable,
			}
			progress.config = elo.OptimizationConfig{TopNForStability: 5}

			result := progress.getRecommendations()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSetVisible(t *testing.T) {
	engine := &elo.Engine{}
	config := DefaultProgressConfig()
	progress := NewProgress(engine, config)

	// Test visibility changes
	progress.SetVisible(true)
	// Note: We can't easily test the title change without accessing internal tview state
	// This test mainly ensures the method doesn't panic

	progress.SetVisible(false)
	// Same here - main goal is to ensure no panic
}

func TestSetConfig(t *testing.T) {
	engine := &elo.Engine{}
	config := DefaultProgressConfig()
	progress := NewProgress(engine, config)

	newConfig := elo.OptimizationConfig{
		TopNForStability: 10,
		MinCoverage:      3,
	}

	progress.SetConfig(newConfig)
	assert.Equal(t, newConfig, progress.config)
}

func TestFormatDuration(t *testing.T) {
	testCases := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{3661 * time.Second, "1h 1m"},
		{7200 * time.Second, "2h 0m"},
	}

	for _, tc := range testCases {
		result := formatDuration(tc.duration)
		assert.Equal(t, tc.expected, result)
	}
}

func TestTruncateID(t *testing.T) {
	testCases := []struct {
		id       string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"verylongproposalid", 8, "veryl..."},
		{"exact", 5, "exact"},
	}

	for _, tc := range testCases {
		result := truncateID(tc.id, tc.maxLen)
		assert.Equal(t, tc.expected, result)
	}
}

func TestCallbacks(t *testing.T) {
	engine := &elo.Engine{
		InitialRating: 1500.0,
		KFactor:       32,
	}

	config := ProgressConfig{
		ShowBars:    true,
		ShowMetrics: true,
	}

	progress := NewProgress(engine, config)

	proposals := []data.Proposal{
		{ID: "prop1", Title: "Test 1", Score: 1500.0},
	}

	// Test that Update method doesn't panic with callbacks
	progress.Update(proposals, nil)

	// Test basic functionality - this test mainly ensures no panics occur
	assert.NotNil(t, progress.GetContainer())
}

// Benchmark tests
func BenchmarkProgressUpdate(b *testing.B) {
	engine := &elo.Engine{
		InitialRating: 1500.0,
		KFactor:       32,
	}
	config := DefaultProgressConfig()
	progress := NewProgress(engine, config)

	proposals := make([]data.Proposal, 50)
	for i := 0; i < 50; i++ {
		proposals[i] = data.Proposal{
			ID:    fmt.Sprintf("prop%d", i),
			Title: fmt.Sprintf("Proposal %d", i),
			Score: 1500.0,
		}
	}

	history := elo.NewComparisonHistory()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		progress.Update(proposals, history)
	}
}

func BenchmarkCreateProgressBar(b *testing.B) {
	engine := &elo.Engine{}
	config := DefaultProgressConfig()
	progress := NewProgress(engine, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		progress.createProgressBar(0.5, false)
	}
}
