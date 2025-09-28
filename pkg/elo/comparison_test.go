package elo

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test ratings
func createTestRating(id string, score float64, games int) Rating {
	engine := createTestEngine()
	return Rating{
		ID:         id,
		Score:      score,
		Games:      games,
		Confidence: engine.calculateConfidence(games),
	}
}

func TestGetExpectedGameCount(t *testing.T) {
	testCases := []struct {
		proposals int
		expected  int
	}{
		{0, 0},
		{1, 0},
		{2, 1},  // Pairwise: 1 game
		{3, 3},  // Trio: 3 games (A-B, A-C, B-C)
		{4, 6},  // Quartet: 6 games (A-B, A-C, A-D, B-C, B-D, C-D)
		{5, 10}, // Beyond quartet but mathematically valid
	}

	for _, tc := range testCases {
		t.Run("proposals", func(t *testing.T) {
			actual := GetExpectedGameCount(tc.proposals)
			assert.Equal(t, tc.expected, actual,
				"Wrong game count for %d proposals", tc.proposals)
		})
	}
}

func TestNewMultiWayComparison(t *testing.T) {
	engine := createTestEngine()

	t.Run("valid trio", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", 1600.0, 5),
			createTestRating("B", 1400.0, 3),
			createTestRating("C", 1200.0, 2),
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		require.NoError(t, err)
		require.NotNil(t, comparison)

		assert.Equal(t, Trio, comparison.Method)
		assert.Len(t, comparison.Proposals, 3)
		assert.Equal(t, engine, comparison.Engine)
		assert.NotNil(t, comparison.TotalChanges)
	})

	t.Run("valid quartet", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", 1800.0, 10),
			createTestRating("B", 1600.0, 8),
			createTestRating("C", 1400.0, 6),
			createTestRating("D", 1200.0, 4),
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		require.NoError(t, err)
		require.NotNil(t, comparison)

		assert.Equal(t, Quartet, comparison.Method)
		assert.Len(t, comparison.Proposals, 4)
	})

	t.Run("pairwise handled correctly", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", 1600.0, 5),
			createTestRating("B", 1400.0, 3),
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		require.NoError(t, err)

		assert.Equal(t, Pairwise, comparison.Method)
		assert.Len(t, comparison.Proposals, 2)
	})

	t.Run("too few proposals", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", 1600.0, 5),
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		assert.Error(t, err)
		assert.Equal(t, ErrTooFewProposals, err)
		assert.Nil(t, comparison)
	})

	t.Run("too many proposals", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", 1800.0, 10),
			createTestRating("B", 1600.0, 8),
			createTestRating("C", 1400.0, 6),
			createTestRating("D", 1200.0, 4),
			createTestRating("E", 1000.0, 2),
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		assert.Error(t, err)
		assert.Equal(t, ErrTooManyProposals, err)
		assert.Nil(t, comparison)
	})

	t.Run("duplicate proposal IDs", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", 1600.0, 5),
			createTestRating("A", 1400.0, 3), // Duplicate ID
			createTestRating("C", 1200.0, 2),
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		assert.Error(t, err)
		assert.Equal(t, ErrDuplicateProposal, err)
		assert.Nil(t, comparison)
	})

	t.Run("invalid rating", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", math.NaN(), 5), // Invalid rating
			createTestRating("B", 1400.0, 3),
			createTestRating("C", 1200.0, 2),
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid rating for proposal A")
		assert.Nil(t, comparison)
	})
}

func TestCalculatePositionWeight(t *testing.T) {
	testCases := []struct {
		higherPos int
		lowerPos  int
		expected  float64
	}{
		{0, 1, 1.0}, // 1st vs 2nd
		{0, 2, 1.0}, // 1st vs 3rd
		{0, 3, 1.0}, // 1st vs 4th
		{1, 2, 0.8}, // 2nd vs 3rd
		{1, 3, 0.8}, // 2nd vs 4th
		{2, 3, 0.6}, // 3rd vs 4th
	}

	for _, tc := range testCases {
		t.Run("position weight", func(t *testing.T) {
			actual := calculatePositionWeight(tc.higherPos, tc.lowerPos)
			assert.InDelta(t, tc.expected, actual, 0.001,
				"Wrong weight for positions %d vs %d", tc.higherPos, tc.lowerPos)
		})
	}
}

func TestTrioComparison(t *testing.T) {
	engine := createTestEngine()

	t.Run("basic trio comparison", func(t *testing.T) {
		// Create trio with clear rating differences
		rankings := []Rating{
			createTestRating("FIRST", 1600.0, 5),  // 1st place
			createTestRating("SECOND", 1400.0, 3), // 2nd place
			createTestRating("THIRD", 1200.0, 2),  // 3rd place
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		require.NoError(t, err)

		updatedRatings, result, err := comparison.Execute()
		require.NoError(t, err)

		// Verify basic structure
		assert.Len(t, updatedRatings, 3)
		assert.Len(t, result.Updates, 3)
		assert.Equal(t, Trio, result.Method)
		assert.True(t, result.Duration > 0)

		// Verify exactly 3 pairwise games generated
		assert.Len(t, comparison.Games, 3, "Trio should generate exactly 3 pairwise games")

		// Verify games are correct combinations
		gamesSeen := make(map[string]bool)
		for _, game := range comparison.Games {
			gameKey := game.Winner.ID + "-" + game.Loser.ID
			gamesSeen[gameKey] = true
		}

		expectedGames := []string{"FIRST-SECOND", "FIRST-THIRD", "SECOND-THIRD"}
		for _, expectedGame := range expectedGames {
			assert.True(t, gamesSeen[expectedGame],
				"Missing expected game: %s", expectedGame)
		}

		// Verify position weights
		for _, game := range comparison.Games {
			switch game.Winner.ID + "-" + game.Loser.ID {
			case "FIRST-SECOND", "FIRST-THIRD":
				assert.InDelta(t, 1.0, game.Weight, 0.001,
					"1st place games should have weight 1.0")
			case "SECOND-THIRD":
				assert.InDelta(t, 0.8, game.Weight, 0.001,
					"2nd place vs 3rd should have weight 0.8")
			}
		}

		// Verify rating conservation
		err = comparison.ValidateRatingConservation()
		assert.NoError(t, err, "Rating conservation should be maintained")
	})

	t.Run("trio with equal ratings", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", 1400.0, 5),
			createTestRating("B", 1400.0, 5),
			createTestRating("C", 1400.0, 5),
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		require.NoError(t, err)

		updatedRatings, _, err := comparison.Execute()
		require.NoError(t, err)

		// With equal ratings, 1st place should gain, 3rd should lose
		var firstRating, thirdRating float64
		for _, rating := range updatedRatings {
			switch rating.ID {
			case "A": // 1st place
				firstRating = rating.Score
			case "C": // 3rd place (last in ranking)
				thirdRating = rating.Score
			}
		}

		assert.Greater(t, firstRating, 1400.0, "1st place should gain rating")
		assert.Less(t, thirdRating, 1400.0, "3rd place should lose rating")

		err = comparison.ValidateRatingConservation()
		assert.NoError(t, err)
	})
}

func TestQuartetComparison(t *testing.T) {
	engine := createTestEngine()

	t.Run("basic quartet comparison", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("FIRST", 1800.0, 10),
			createTestRating("SECOND", 1600.0, 8),
			createTestRating("THIRD", 1400.0, 6),
			createTestRating("FOURTH", 1200.0, 4),
		}

		comparison, err := engine.NewMultiWayComparison(rankings)
		require.NoError(t, err)

		updatedRatings, result, err := comparison.Execute()
		require.NoError(t, err)

		// Verify basic structure
		assert.Len(t, updatedRatings, 4)
		assert.Len(t, result.Updates, 4)
		assert.Equal(t, Quartet, result.Method)

		// Verify exactly 6 pairwise games
		assert.Len(t, comparison.Games, 6, "Quartet should generate exactly 6 pairwise games")

		// Verify all combinations exist
		expectedGames := map[string]bool{
			"FIRST-SECOND": false, "FIRST-THIRD": false, "FIRST-FOURTH": false,
			"SECOND-THIRD": false, "SECOND-FOURTH": false, "THIRD-FOURTH": false,
		}

		for _, game := range comparison.Games {
			gameKey := game.Winner.ID + "-" + game.Loser.ID
			if _, exists := expectedGames[gameKey]; exists {
				expectedGames[gameKey] = true
			}
		}

		for gameKey, found := range expectedGames {
			assert.True(t, found, "Missing expected game: %s", gameKey)
		}

		// Verify rating conservation
		err = comparison.ValidateRatingConservation()
		assert.NoError(t, err)

		// Verify position weights
		for _, game := range comparison.Games {
			gameKey := game.Winner.ID + "-" + game.Loser.ID
			switch gameKey {
			case "FIRST-SECOND", "FIRST-THIRD", "FIRST-FOURTH":
				assert.InDelta(t, 1.0, game.Weight, 0.001)
			case "SECOND-THIRD", "SECOND-FOURTH":
				assert.InDelta(t, 0.8, game.Weight, 0.001)
			case "THIRD-FOURTH":
				assert.InDelta(t, 0.6, game.Weight, 0.001)
			}
		}
	})
}

func TestEngineCalculateMultiway(t *testing.T) {
	engine := createTestEngine()

	t.Run("convenience method works", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", 1600.0, 5),
			createTestRating("B", 1400.0, 3),
			createTestRating("C", 1200.0, 2),
		}

		updatedRatings, result, err := engine.CalculateMultiway(rankings)
		require.NoError(t, err)

		assert.Len(t, updatedRatings, 3)
		assert.Equal(t, Trio, result.Method)
		assert.Len(t, result.Updates, 3)
	})
}

func TestMultiWayEdgeCases(t *testing.T) {
	engine := createTestEngine()

	t.Run("extreme rating differences", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("HIGH", 2500.0, 20), // Very high rating
			createTestRating("LOW", 500.0, 5),    // Very low rating
		}

		updatedRatings, _, err := engine.CalculateMultiway(rankings)
		require.NoError(t, err)

		// High-rated player should gain very little
		// Low-rated player should lose very little
		var highNew, lowNew float64
		for _, rating := range updatedRatings {
			switch rating.ID {
			case "HIGH":
				highNew = rating.Score
			case "LOW":
				lowNew = rating.Score
			}
		}

		assert.InDelta(t, 2500.0, highNew, 5.0, "High-rated player should change very little")
		assert.InDelta(t, 500.0, lowNew, 5.0, "Low-rated player should change very little")
	})

	t.Run("rating bounds enforcement", func(t *testing.T) {
		// Create engine with tight bounds
		config := Config{
			InitialRating: 1200.0,
			KFactor:       32,
			MinRating:     1000.0,
			MaxRating:     1500.0,
		}
		boundedEngine, err := NewEngine(config)
		require.NoError(t, err)

		rankings := []Rating{
			{ID: "HIGH", Score: 1499.0, Games: 5, Confidence: 0.25}, // Near max
			{ID: "LOW", Score: 1001.0, Games: 3, Confidence: 0.15},  // Near min
		}

		updatedRatings, _, err := boundedEngine.CalculateMultiway(rankings)
		require.NoError(t, err)

		for _, rating := range updatedRatings {
			assert.GreaterOrEqual(t, rating.Score, boundedEngine.MinRating)
			assert.LessOrEqual(t, rating.Score, boundedEngine.MaxRating)
		}
	})

	t.Run("game count updates correctly", func(t *testing.T) {
		rankings := []Rating{
			createTestRating("A", 1600.0, 5),
			createTestRating("B", 1400.0, 3),
			createTestRating("C", 1200.0, 2),
		}

		updatedRatings, _, err := engine.CalculateMultiway(rankings)
		require.NoError(t, err)

		// In a trio, each proposal plays 2 games (vs each of the other 2)
		for i, rating := range updatedRatings {
			originalGames := rankings[i].Games
			expectedNewGames := originalGames + 2 // Each plays 2 games in a trio
			assert.Equal(t, expectedNewGames, rating.Games,
				"Proposal %s should have correct game count", rating.ID)
		}
	})
}

// Benchmark tests for performance validation
func BenchmarkTrioComparison(b *testing.B) {
	engine := createTestEngine()
	rankings := []Rating{
		createTestRating("A", 1600.0, 5),
		createTestRating("B", 1400.0, 3),
		createTestRating("C", 1200.0, 2),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = engine.CalculateMultiway(rankings)
	}
}

func BenchmarkQuartetComparison(b *testing.B) {
	engine := createTestEngine()
	rankings := []Rating{
		createTestRating("A", 1800.0, 10),
		createTestRating("B", 1600.0, 8),
		createTestRating("C", 1400.0, 6),
		createTestRating("D", 1200.0, 4),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = engine.CalculateMultiway(rankings)
	}
}
