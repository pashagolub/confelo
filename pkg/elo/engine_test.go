package elo

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test configuration constants
const (
	defaultInitialRating = 1200.0
	defaultKFactor       = 32
	defaultMinRating     = 0.0
	defaultMaxRating     = 3000.0
	tolerance            = 0.0001 // Floating point comparison tolerance
)

// Helper function to create a default engine for testing
func createTestEngine() *Engine {
	config := Config{
		InitialRating: defaultInitialRating,
		KFactor:       defaultKFactor,
		MinRating:     defaultMinRating,
		MaxRating:     defaultMaxRating,
	}
	engine, _ := NewEngine(config)
	return engine
}

// Helper function to create a rating
func createRating(id string, score float64, games int) Rating {
	engine := createTestEngine()
	return Rating{
		ID:         id,
		Score:      score,
		Games:      games,
		Confidence: engine.calculateConfidence(games),
	}
}

func TestNewEngine(t *testing.T) {
	t.Run("valid configuration creates engine", func(t *testing.T) {
		config := Config{
			InitialRating: 1500.0,
			KFactor:       32,
			MinRating:     0.0,
			MaxRating:     3000.0,
		}

		engine, err := NewEngine(config)
		require.NoError(t, err)
		require.NotNil(t, engine)

		assert.Equal(t, 1500.0, engine.InitialRating)
		assert.Equal(t, 32, engine.KFactor)
		assert.Equal(t, 0.0, engine.MinRating)
		assert.Equal(t, 3000.0, engine.MaxRating)
	})

	t.Run("invalid K-factor returns error", func(t *testing.T) {
		config := Config{
			InitialRating: 1500.0,
			KFactor:       0, // Invalid
			MinRating:     0.0,
			MaxRating:     3000.0,
		}

		engine, err := NewEngine(config)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidKFactor, err)
		assert.Nil(t, engine)
	})

	t.Run("invalid bounds returns error", func(t *testing.T) {
		config := Config{
			InitialRating: 1500.0,
			KFactor:       32,
			MinRating:     3000.0, // Min > Max
			MaxRating:     0.0,
		}

		engine, err := NewEngine(config)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidBounds, err)
		assert.Nil(t, engine)
	})

	t.Run("NaN initial rating returns error", func(t *testing.T) {
		config := Config{
			InitialRating: math.NaN(),
			KFactor:       32,
			MinRating:     0.0,
			MaxRating:     3000.0,
		}

		engine, err := NewEngine(config)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidRating, err)
		assert.Nil(t, engine)
	})
}

func TestCalculateExpectedScore(t *testing.T) {
	engine := createTestEngine()

	testCases := []struct {
		name     string
		ratingA  float64
		ratingB  float64
		expected float64
	}{
		{"equal ratings", 1200.0, 1200.0, 0.5},
		{"A higher than B by 400", 1600.0, 1200.0, 0.9090909090909091},
		{"A lower than B by 400", 800.0, 1200.0, 0.09090909090909091},
		{"A higher than B by 200", 1400.0, 1200.0, 0.7597469733656174},
		{"A lower than B by 200", 1000.0, 1200.0, 0.24025302663438258},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.calculateExpectedScore(tc.ratingA, tc.ratingB)
			assert.InDelta(t, tc.expected, result, tolerance, "Expected score calculation incorrect")

			// Test symmetry: E_A + E_B should equal 1.0
			expectedB := engine.calculateExpectedScore(tc.ratingB, tc.ratingA)
			assert.InDelta(t, 1.0, result+expectedB, tolerance, "Expected scores should sum to 1.0")
		})
	}
}

func TestCalculatePairwise(t *testing.T) {
	engine := createTestEngine()

	t.Run("equal ratings - winner beats loser", func(t *testing.T) {
		winner := createRating("PROP001", 1200.0, 5)
		loser := createRating("PROP002", 1200.0, 3)

		newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
		require.NoError(t, err)

		// With equal ratings and K=32, winner should gain 16, loser should lose 16
		assert.InDelta(t, 1216.0, newWinner.Score, tolerance)
		assert.InDelta(t, 1184.0, newLoser.Score, tolerance)

		// Game counts should increment
		assert.Equal(t, 6, newWinner.Games)
		assert.Equal(t, 4, newLoser.Games)

		// IDs should be preserved
		assert.Equal(t, "PROP001", newWinner.ID)
		assert.Equal(t, "PROP002", newLoser.ID)
	})

	t.Run("higher rated winner beats lower rated loser", func(t *testing.T) {
		winner := createRating("PROP007", 1850.0, 10)
		loser := createRating("PROP020", 950.0, 8)

		newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
		require.NoError(t, err)

		// Calculate expected scores to verify our math
		expectedWinner := engine.calculateExpectedScore(winner.Score, loser.Score)
		expectedLoser := engine.calculateExpectedScore(loser.Score, winner.Score)

		// Expected gains/losses
		expectedWinnerGain := float64(engine.KFactor) * (1.0 - expectedWinner)
		expectedLoserLoss := float64(engine.KFactor) * (0.0 - expectedLoser)

		assert.InDelta(t, winner.Score+expectedWinnerGain, newWinner.Score, tolerance)
		assert.InDelta(t, loser.Score+expectedLoserLoss, newLoser.Score, tolerance)

		// Verify rating conservation: winner gains exactly what loser loses
		winnerGain := newWinner.Score - winner.Score
		loserLoss := loser.Score - newLoser.Score
		assert.InDelta(t, winnerGain, loserLoss, tolerance,
			"Winner should gain exactly what loser loses (rating conservation)")
	})

	t.Run("rating conservation", func(t *testing.T) {
		winner := createRating("PROP001", 1400.0, 5)
		loser := createRating("PROP002", 1200.0, 3)

		newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
		require.NoError(t, err)

		// Total rating change should be zero (conservation)
		oldTotal := winner.Score + loser.Score
		newTotal := newWinner.Score + newLoser.Score
		assert.InDelta(t, oldTotal, newTotal, tolerance, "Rating points should be conserved")
	})

	t.Run("bounds enforcement", func(t *testing.T) {
		// Test minimum bound
		winner := createRating("PROP001", 50.0, 1)
		loser := createRating("PROP002", 0.0, 1) // At minimum

		_, newLoser, err := engine.CalculatePairwise(winner, loser)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, newLoser.Score, engine.MinRating)
		assert.LessOrEqual(t, newLoser.Score, engine.MaxRating)

		// Test maximum bound
		winner2 := createRating("PROP003", 3000.0, 1) // At maximum
		loser2 := createRating("PROP004", 2950.0, 1)

		newWinner2, _, err := engine.CalculatePairwise(winner2, loser2)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, newWinner2.Score, engine.MinRating)
		assert.LessOrEqual(t, newWinner2.Score, engine.MaxRating)
	})

	t.Run("invalid ratings return errors", func(t *testing.T) {
		validRating := createRating("PROP001", 1200.0, 5)
		invalidRating := createRating("PROP002", math.NaN(), 3)

		_, _, err := engine.CalculatePairwise(validRating, invalidRating)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidRating, err)

		_, _, err = engine.CalculatePairwise(invalidRating, validRating)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidRating, err)
	})
}

func TestCalculateConfidence(t *testing.T) {
	engine := createTestEngine()

	testCases := []struct {
		games    int
		expected float64
	}{
		{0, 0.0},
		{5, 0.25},
		{10, 0.5},
		{15, 0.75},
		{20, 1.0},
		{25, 1.0}, // Caps at 1.0
		{100, 1.0},
	}

	for _, tc := range testCases {
		t.Run("games", func(t *testing.T) {
			result := engine.calculateConfidence(tc.games)
			assert.InDelta(t, tc.expected, result, tolerance)
		})
	}
}

func TestScaleRating(t *testing.T) {
	engine := createTestEngine()

	t.Run("scale to 0-10 range", func(t *testing.T) {
		// Test various ratings on 0-10 scale
		testCases := []struct {
			rating   float64
			expected float64
		}{
			{engine.MinRating, 0.0},                          // Minimum maps to 0
			{engine.MaxRating, 10.0},                         // Maximum maps to 10
			{engine.InitialRating, 4.0},                      // 1200 on 0-3000 scale = 0.4, scaled to 4.0
			{(engine.MinRating + engine.MaxRating) / 2, 5.0}, // Midpoint maps to 5
		}

		for _, tc := range testCases {
			result := engine.ScaleRating(tc.rating, 0.0, 10.0)
			assert.InDelta(t, tc.expected, result, tolerance)
		}
	})

	t.Run("scale to arbitrary range", func(t *testing.T) {
		rating := 1500.0
		scaled := engine.ScaleRating(rating, 100.0, 200.0)

		// 1500 is at position 0.5 on 0-3000 scale, should be at 150 on 100-200 scale
		expected := 150.0
		assert.InDelta(t, expected, scaled, tolerance)
	})

	t.Run("invalid rating returns minimum", func(t *testing.T) {
		result := engine.ScaleRating(math.NaN(), 0.0, 10.0)
		assert.Equal(t, 0.0, result)
	})

	t.Run("out of bounds rating gets clamped", func(t *testing.T) {
		// Rating above maximum
		result := engine.ScaleRating(5000.0, 0.0, 10.0)
		assert.InDelta(t, 10.0, result, tolerance)

		// Rating below minimum
		result = engine.ScaleRating(-1000.0, 0.0, 10.0)
		assert.InDelta(t, 0.0, result, tolerance)
	})
}

func TestCalculatePairwiseWithResult(t *testing.T) {
	engine := createTestEngine()

	winner := createRating("PROP001", 1200.0, 5)
	loser := createRating("PROP002", 1200.0, 3)

	result, err := engine.CalculatePairwiseWithResult(winner, loser)
	require.NoError(t, err)

	// Check result structure
	assert.Equal(t, Pairwise, result.Method)
	assert.Len(t, result.Updates, 2)
	assert.True(t, result.Duration >= 0)
	assert.True(t, result.Timestamp.Before(time.Now().Add(time.Minute)))

	// Check rating updates
	winnerUpdate := result.Updates[0]
	assert.Equal(t, "PROP001", winnerUpdate.ProposalID)
	assert.Equal(t, 1200.0, winnerUpdate.OldRating)
	assert.InDelta(t, 1216.0, winnerUpdate.NewRating, tolerance)
	assert.InDelta(t, 16.0, winnerUpdate.Delta, tolerance)
	assert.Equal(t, defaultKFactor, winnerUpdate.KFactor)

	loserUpdate := result.Updates[1]
	assert.Equal(t, "PROP002", loserUpdate.ProposalID)
	assert.Equal(t, 1200.0, loserUpdate.OldRating)
	assert.InDelta(t, 1184.0, loserUpdate.NewRating, tolerance)
	assert.InDelta(t, -16.0, loserUpdate.Delta, tolerance)
	assert.Equal(t, defaultKFactor, loserUpdate.KFactor)
}

// Property-based tests
func TestPropertyConservation(t *testing.T) {
	engine := createTestEngine()

	// Test that rating points are always conserved in pairwise comparisons
	testCases := []struct {
		winnerRating float64
		loserRating  float64
	}{
		{1200.0, 1200.0},
		{1800.0, 1000.0},
		{800.0, 1600.0},
		{2500.0, 500.0},
		{1234.5, 1876.3},
	}

	for _, tc := range testCases {
		t.Run("conservation property", func(t *testing.T) {
			winner := createRating("W", tc.winnerRating, 5)
			loser := createRating("L", tc.loserRating, 3)

			newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
			require.NoError(t, err)

			oldTotal := winner.Score + loser.Score
			newTotal := newWinner.Score + newLoser.Score
			assert.InDelta(t, oldTotal, newTotal, tolerance,
				"Rating conservation violated for ratings %.2f vs %.2f", tc.winnerRating, tc.loserRating)
		})
	}
}

func TestPropertySymmetry(t *testing.T) {
	engine := createTestEngine()

	// Test that expected scores are symmetric (E_A + E_B = 1.0)
	ratings := []float64{800.0, 1000.0, 1200.0, 1400.0, 1600.0, 1800.0, 2000.0}

	for _, ratingA := range ratings {
		for _, ratingB := range ratings {
			t.Run("symmetry property", func(t *testing.T) {
				expectedA := engine.calculateExpectedScore(ratingA, ratingB)
				expectedB := engine.calculateExpectedScore(ratingB, ratingA)

				sum := expectedA + expectedB
				assert.InDelta(t, 1.0, sum, tolerance,
					"Expected score symmetry violated for ratings %.2f vs %.2f", ratingA, ratingB)
			})
		}
	}
}

// Benchmark tests for performance requirements
func BenchmarkCalculatePairwise(b *testing.B) {
	engine := createTestEngine()
	winner := createRating("PROP001", 1400.0, 10)
	loser := createRating("PROP002", 1200.0, 8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = engine.CalculatePairwise(winner, loser)
	}
}

func BenchmarkCalculateExpectedScore(b *testing.B) {
	engine := createTestEngine()
	ratingA := 1400.0
	ratingB := 1200.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.calculateExpectedScore(ratingA, ratingB)
	}
}

func BenchmarkScaleRating(b *testing.B) {
	engine := createTestEngine()
	rating := 1400.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.ScaleRating(rating, 0.0, 10.0)
	}
}
