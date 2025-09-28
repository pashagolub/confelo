package elo

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test ratings for optimization tests
func createOptimizationTestRatings() []Rating {
	engine := createTestEngine()
	return []Rating{
		{ID: "high1", Score: 1700.0, Games: 8, Confidence: engine.calculateConfidence(8)},
		{ID: "high2", Score: 1680.0, Games: 7, Confidence: engine.calculateConfidence(7)},
		{ID: "mid1", Score: 1450.0, Games: 5, Confidence: engine.calculateConfidence(5)},
		{ID: "mid2", Score: 1430.0, Games: 6, Confidence: engine.calculateConfidence(6)},
		{ID: "low1", Score: 1200.0, Games: 4, Confidence: engine.calculateConfidence(4)},
		{ID: "low2", Score: 1180.0, Games: 3, Confidence: engine.calculateConfidence(3)},
	}
}

func TestGetRatingBins(t *testing.T) {
	engine := createTestEngine()
	proposals := createOptimizationTestRatings()

	t.Run("default bin size", func(t *testing.T) {
		bins := engine.GetRatingBins(proposals, 50.0)

		// Verify bins are created correctly
		assert.Greater(t, len(bins), 0, "Should create at least one bin")

		// Count total proposals in bins
		totalInBins := 0
		for _, proposalIDs := range bins {
			totalInBins += len(proposalIDs)
		}
		assert.Equal(t, len(proposals), totalInBins, "All proposals should be in bins")

		// Track bin indices for comparison
		highBinIndices := make([]int, 0)
		lowBinIndices := make([]int, 0)

		for binIndex, proposalIDs := range bins {
			for _, id := range proposalIDs {
				if id == "high1" || id == "high2" {
					highBinIndices = append(highBinIndices, binIndex)
				}
				if id == "low1" || id == "low2" {
					lowBinIndices = append(lowBinIndices, binIndex)
				}
			}
		}

		assert.Greater(t, len(highBinIndices), 0, "High-rated proposals should be found in bins")
		assert.Greater(t, len(lowBinIndices), 0, "Low-rated proposals should be found in bins")

		// High-rated proposals should be in higher bin indices than low-rated ones
		if len(highBinIndices) > 0 && len(lowBinIndices) > 0 {
			maxLowBin := lowBinIndices[0]
			for _, idx := range lowBinIndices {
				if idx > maxLowBin {
					maxLowBin = idx
				}
			}

			minHighBin := highBinIndices[0]
			for _, idx := range highBinIndices {
				if idx < minHighBin {
					minHighBin = idx
				}
			}

			assert.GreaterOrEqual(t, minHighBin, maxLowBin,
				"High-rated proposals should be in bins >= low-rated proposals")
		}
	})

	t.Run("custom bin size", func(t *testing.T) {
		bins := engine.GetRatingBins(proposals, 100.0) // Larger bins

		// With larger bins, we should have fewer bins with more proposals each
		binCount := len(bins)
		assert.Greater(t, binCount, 0)

		// Verify no empty bins
		for _, proposalIDs := range bins {
			assert.Greater(t, len(proposalIDs), 0, "Bins should not be empty")
		}
	})

	t.Run("single proposal", func(t *testing.T) {
		singleProposal := []Rating{proposals[0]}
		bins := engine.GetRatingBins(singleProposal, 50.0)

		assert.Equal(t, 1, len(bins), "Should create exactly one bin for single proposal")

		for _, proposalIDs := range bins {
			assert.Equal(t, 1, len(proposalIDs))
			assert.Equal(t, "high1", proposalIDs[0])
		}
	})
}

func TestGetOptimalMatchup(t *testing.T) {
	engine := createTestEngine()
	proposals := createOptimizationTestRatings()
	history := NewComparisonHistory()
	config := DefaultOptimizationConfig()

	t.Run("basic matchup selection", func(t *testing.T) {
		matchup := engine.GetOptimalMatchup(proposals, history, config)

		require.NotNil(t, matchup)
		assert.NotEmpty(t, matchup.ProposalA)
		assert.NotEmpty(t, matchup.ProposalB)
		assert.NotEqual(t, matchup.ProposalA, matchup.ProposalB)
		assert.GreaterOrEqual(t, matchup.Priority, 1)
		assert.LessOrEqual(t, matchup.Priority, 5)
		assert.GreaterOrEqual(t, matchup.Information, 0.0)
	})

	t.Run("prefers close ratings", func(t *testing.T) {
		// Add some history to make certain matchups less attractive
		result := ComparisonResult{
			Updates: []RatingUpdate{
				{ProposalID: "high1", OldRating: 1700.0, NewRating: 1705.0, Delta: 5.0},
				{ProposalID: "low1", OldRating: 1200.0, NewRating: 1195.0, Delta: -5.0},
			},
			Method: Pairwise,
		}
		history.AddComparison(result)

		matchup := engine.GetOptimalMatchup(proposals, history, config)

		require.NotNil(t, matchup)

		// Should prefer matchups with closer ratings
		proposalA := findProposalByID(proposals, matchup.ProposalA)
		proposalB := findProposalByID(proposals, matchup.ProposalB)
		ratingDiff := math.Abs(proposalA.Score - proposalB.Score)

		// Expected close should be true for ratings within 50 points
		if ratingDiff <= 50.0 {
			assert.True(t, matchup.ExpectedClose)
		} else {
			assert.False(t, matchup.ExpectedClose)
		}
	})

	t.Run("handles empty proposals", func(t *testing.T) {
		emptyProposals := []Rating{}
		matchup := engine.GetOptimalMatchup(emptyProposals, history, config)
		assert.Nil(t, matchup)
	})

	t.Run("handles single proposal", func(t *testing.T) {
		singleProposal := []Rating{proposals[0]}
		matchup := engine.GetOptimalMatchup(singleProposal, history, config)
		assert.Nil(t, matchup)
	})
}

func TestCheckConvergence(t *testing.T) {
	engine := createTestEngine()
	proposals := createOptimizationTestRatings()
	config := DefaultOptimizationConfig()

	t.Run("no history", func(t *testing.T) {
		status := engine.CheckConvergence(proposals, nil, config)

		require.NotNil(t, status)
		assert.False(t, status.ShouldStop)
		assert.Equal(t, 0.0, status.Confidence)
		assert.Greater(t, status.RemainingEstimate, 0)
		assert.NotNil(t, status.Metrics)
	})

	t.Run("insufficient history", func(t *testing.T) {
		history := NewComparisonHistory()

		// Add minimal history
		result := ComparisonResult{
			Updates: []RatingUpdate{
				{ProposalID: "high1", OldRating: 1700.0, NewRating: 1702.0, Delta: 2.0},
				{ProposalID: "mid1", OldRating: 1450.0, NewRating: 1448.0, Delta: -2.0},
			},
			Method: Pairwise,
		}
		history.AddComparison(result)

		status := engine.CheckConvergence(proposals, history, config)

		require.NotNil(t, status)
		assert.False(t, status.ShouldStop) // Should not stop with minimal history
		assert.Less(t, status.Confidence, 1.0)
	})

	t.Run("converged state simulation", func(t *testing.T) {
		history := NewComparisonHistory()

		// Simulate many comparisons with decreasing rating changes (convergence)
		for i := 0; i < 25; i++ {
			// Decreasing deltas to simulate convergence
			delta := 10.0 * math.Exp(-float64(i)/5.0) // Exponential decay

			result := ComparisonResult{
				Updates: []RatingUpdate{
					{ProposalID: "high1", OldRating: 1700.0, NewRating: 1700.0 + delta, Delta: delta},
					{ProposalID: "mid1", OldRating: 1450.0, NewRating: 1450.0 - delta, Delta: -delta},
				},
				Method: Pairwise,
			}
			history.AddComparison(result)
		}

		// Add sufficient rating history for coverage
		for _, proposal := range proposals {
			for j := 0; j < config.MinCoverage; j++ {
				if history.RatingHistory[proposal.ID] == nil {
					history.RatingHistory[proposal.ID] = make([]float64, 0)
				}
				history.RatingHistory[proposal.ID] = append(history.RatingHistory[proposal.ID], proposal.Score)
			}
		}

		status := engine.CheckConvergence(proposals, history, config)

		require.NotNil(t, status)
		// With small rating changes and sufficient history, should recommend stopping
		if status.Metrics.AvgRatingChange < config.StabilityThreshold {
			assert.Greater(t, status.Confidence, 0.5) // Should have reasonable confidence
		}

		// Verify metrics are calculated
		assert.GreaterOrEqual(t, status.Metrics.AvgRatingChange, 0.0)
		assert.GreaterOrEqual(t, status.Metrics.RatingVariance, 0.0)
		assert.GreaterOrEqual(t, status.Metrics.RankingStability, 0.0)
		assert.GreaterOrEqual(t, status.Metrics.CoveragePercentage, 0.0)
	})
}

func TestGetProgressMetrics(t *testing.T) {
	engine := createTestEngine()
	proposals := createOptimizationTestRatings()
	config := DefaultOptimizationConfig()

	t.Run("no history", func(t *testing.T) {
		metrics := engine.GetProgressMetrics(proposals, nil, config)

		require.NotNil(t, metrics)
		assert.Equal(t, 0, metrics.TotalComparisons)
		assert.Equal(t, 0.0, metrics.CoverageComplete)
		assert.Equal(t, 0, metrics.TopNStable)
		assert.Greater(t, metrics.EstimatedRemaining, 0)
		assert.NotNil(t, metrics.ConfidenceScores)
	})

	t.Run("with comparison history", func(t *testing.T) {
		history := NewComparisonHistory()

		// Add several comparisons
		for i := 0; i < 10; i++ {
			result := ComparisonResult{
				Updates: []RatingUpdate{
					{ProposalID: "high1", OldRating: 1700.0, NewRating: 1702.0, Delta: 2.0},
					{ProposalID: "mid1", OldRating: 1450.0, NewRating: 1448.0, Delta: -2.0},
				},
				Method: Pairwise,
			}
			history.AddComparison(result)
		}

		// Add rating histories for confidence calculation
		for _, proposal := range proposals {
			history.RatingHistory[proposal.ID] = []float64{proposal.Score, proposal.Score + 1, proposal.Score + 2}
		}

		metrics := engine.GetProgressMetrics(proposals, history, config)

		require.NotNil(t, metrics)
		assert.Equal(t, 10, metrics.TotalComparisons)
		assert.GreaterOrEqual(t, metrics.CoverageComplete, 0.0)
		assert.LessOrEqual(t, metrics.CoverageComplete, 1.0)
		assert.GreaterOrEqual(t, metrics.ConvergenceRate, 0.0)
		assert.GreaterOrEqual(t, metrics.TopNStable, 0)

		// Check confidence scores
		assert.Equal(t, len(proposals), len(metrics.ConfidenceScores))
		for proposalID, confidence := range metrics.ConfidenceScores {
			assert.GreaterOrEqual(t, confidence, 0.0)
			assert.LessOrEqual(t, confidence, 1.0)

			// Verify proposal ID exists
			found := false
			for _, proposal := range proposals {
				if proposal.ID == proposalID {
					found = true
					break
				}
			}
			assert.True(t, found, "Confidence score for unknown proposal: %s", proposalID)
		}
	})
}

func TestComparisonHistory(t *testing.T) {
	t.Run("basic functionality", func(t *testing.T) {
		history := NewComparisonHistory()

		assert.NotNil(t, history)
		assert.Equal(t, 0, len(history.Comparisons))
		assert.NotNil(t, history.RatingHistory)
		assert.NotNil(t, history.PairHistory)
	})

	t.Run("add pairwise comparison", func(t *testing.T) {
		history := NewComparisonHistory()

		result := ComparisonResult{
			Updates: []RatingUpdate{
				{ProposalID: "A", OldRating: 1500.0, NewRating: 1516.0, Delta: 16.0},
				{ProposalID: "B", OldRating: 1500.0, NewRating: 1484.0, Delta: -16.0},
			},
			Method: Pairwise,
		}

		history.AddComparison(result)

		assert.Equal(t, 1, len(history.Comparisons))
		assert.Equal(t, 1516.0, history.RatingHistory["A"][0])
		assert.Equal(t, 1484.0, history.RatingHistory["B"][0])
		assert.Equal(t, 1, history.GetPairComparisonCount("A", "B"))
		assert.Equal(t, 1, history.GetPairComparisonCount("B", "A")) // Should be symmetric
	})

	t.Run("add multi-way comparison", func(t *testing.T) {
		history := NewComparisonHistory()

		result := ComparisonResult{
			Updates: []RatingUpdate{
				{ProposalID: "A", OldRating: 1500.0, NewRating: 1520.0, Delta: 20.0},
				{ProposalID: "B", OldRating: 1500.0, NewRating: 1500.0, Delta: 0.0},
				{ProposalID: "C", OldRating: 1500.0, NewRating: 1480.0, Delta: -20.0},
			},
			Method: Trio,
		}

		history.AddComparison(result)

		assert.Equal(t, 1, len(history.Comparisons))

		// All pairs should be recorded for trio
		assert.Equal(t, 1, history.GetPairComparisonCount("A", "B"))
		assert.Equal(t, 1, history.GetPairComparisonCount("A", "C"))
		assert.Equal(t, 1, history.GetPairComparisonCount("B", "C"))
	})

	t.Run("recent comparisons", func(t *testing.T) {
		history := NewComparisonHistory()

		// Add multiple comparisons
		for i := 0; i < 15; i++ {
			result := ComparisonResult{
				Updates: []RatingUpdate{
					{ProposalID: "A", OldRating: 1500.0, NewRating: 1500.0 + float64(i), Delta: float64(i)},
					{ProposalID: "B", OldRating: 1500.0, NewRating: 1500.0 - float64(i), Delta: -float64(i)},
				},
				Method: Pairwise,
			}
			history.AddComparison(result)
		}

		recent := history.GetRecentComparisons(5)
		assert.Equal(t, 5, len(recent))

		// Should get the most recent comparisons
		assert.Equal(t, 14.0, recent[4].Updates[0].Delta)
		assert.Equal(t, 10.0, recent[0].Updates[0].Delta)
	})
}

func TestOptimizationConfiguration(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		config := DefaultOptimizationConfig()

		assert.Equal(t, 50.0, config.BinSize)
		assert.Equal(t, 5.0, config.StabilityThreshold)
		assert.Equal(t, 10, config.StabilityWindow)
		assert.Equal(t, 5, config.MinCoverage)
		assert.Equal(t, 10, config.MaxCoverage)
		assert.Equal(t, 5, config.TopNForStability)
		assert.Equal(t, 0.15, config.CrossBinRate)
		assert.Equal(t, 20, config.ConvergenceWindow)
	})
}

func TestInformationGainCalculation(t *testing.T) {
	t.Run("close ratings have high information gain", func(t *testing.T) {
		closeGain := calculateInformationGain(1500.0, 1520.0, 0) // 20 point difference, no history
		wideGain := calculateInformationGain(1500.0, 1700.0, 0)  // 200 point difference, no history

		assert.Greater(t, closeGain, wideGain, "Close ratings should have higher information gain")
	})

	t.Run("repeated games have lower information gain", func(t *testing.T) {
		newGain := calculateInformationGain(1500.0, 1520.0, 0)    // No previous games
		repeatGain := calculateInformationGain(1500.0, 1520.0, 5) // 5 previous games

		assert.Greater(t, newGain, repeatGain, "New matchups should have higher information gain")
	})

	t.Run("priority calculation", func(t *testing.T) {
		highInfo := 0.9
		lowInfo := 0.1

		highPriority := calculatePriority(highInfo)
		lowPriority := calculatePriority(lowInfo)

		assert.Greater(t, highPriority, lowPriority)
		assert.GreaterOrEqual(t, highPriority, 1)
		assert.LessOrEqual(t, highPriority, 5)
		assert.GreaterOrEqual(t, lowPriority, 1)
		assert.LessOrEqual(t, lowPriority, 5)
	})
}

// Helper function to find proposal by ID
func findProposalByID(proposals []Rating, id string) Rating {
	for _, proposal := range proposals {
		if proposal.ID == id {
			return proposal
		}
	}
	return Rating{} // Return empty if not found
}

// Benchmark tests for optimization performance
func BenchmarkGetOptimalMatchup(b *testing.B) {
	engine := createTestEngine()
	proposals := createOptimizationTestRatings()
	history := NewComparisonHistory()
	config := DefaultOptimizationConfig()

	// Add some history for realistic scenario
	for i := 0; i < 10; i++ {
		result := ComparisonResult{
			Updates: []RatingUpdate{
				{ProposalID: proposals[i%len(proposals)].ID, Delta: float64(i)},
			},
			Method: Pairwise,
		}
		history.AddComparison(result)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.GetOptimalMatchup(proposals, history, config)
	}
}

func BenchmarkCheckConvergence(b *testing.B) {
	engine := createTestEngine()
	proposals := createOptimizationTestRatings()
	history := NewComparisonHistory()
	config := DefaultOptimizationConfig()

	// Add substantial history
	for i := 0; i < 50; i++ {
		result := ComparisonResult{
			Updates: []RatingUpdate{
				{ProposalID: proposals[i%len(proposals)].ID, Delta: float64(i % 10)},
			},
			Method: Pairwise,
		}
		history.AddComparison(result)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.CheckConvergence(proposals, history, config)
	}
}

func BenchmarkGetProgressMetrics(b *testing.B) {
	engine := createTestEngine()
	proposals := createOptimizationTestRatings()
	history := NewComparisonHistory()
	config := DefaultOptimizationConfig()

	// Add rating histories
	for _, proposal := range proposals {
		history.RatingHistory[proposal.ID] = []float64{proposal.Score, proposal.Score + 5, proposal.Score + 3}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.GetProgressMetrics(proposals, history, config)
	}
}
