package elo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test data structures for expected results
type ExpectedResults struct {
	Description   string       `json:"description"`
	InitialRating float64      `json:"initial_rating"`
	KFactor       int          `json:"k_factor"`
	TestCases     []TestCase   `json:"test_cases"`
	EdgeCases     []TestCase   `json:"edge_cases"`
	RatingBounds  RatingBounds `json:"validation_rules"`
}

type TestCase struct {
	Description     string                  `json:"description"`
	ProposalA       ProposalInfo            `json:"proposal_a"`
	ProposalB       ProposalInfo            `json:"proposal_b"`
	Winner          string                  `json:"winner"`
	ExpectedResults map[string]RatingChange `json:"expected_results"`
}

type ProposalInfo struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Speaker       string  `json:"speaker,omitempty"`
	Abstract      string  `json:"abstract,omitempty"`
	InitialRating float64 `json:"initial_rating"`
}

type RatingChange struct {
	NewRating    float64 `json:"new_rating"`
	RatingChange float64 `json:"rating_change"`
}

type RatingBounds struct {
	RatingBounds struct {
		Minimum float64 `json:"minimum"`
		Maximum float64 `json:"maximum"`
	} `json:"rating_bounds"`
	KFactorValidation struct {
		Standard     int   `json:"standard"`
		Alternatives []int `json:"alternatives"`
	} `json:"k_factor_validation"`
	RequiredFields []string `json:"required_fields"`
	OptionalFields []string `json:"optional_fields"`
}

func TestStandardChessEloExamples(t *testing.T) {
	// Test against well-known Chess Elo examples
	engine := createTestEngine()

	t.Run("standard chess example 1", func(t *testing.T) {
		// Example from Chess.com Elo explanation
		// Player A (1600) beats Player B (1400)
		winner := createRating("A", 1600.0, 0)
		loser := createRating("B", 1400.0, 0)

		newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
		require.NoError(t, err)

		// Expected: Winner gains ~11, Loser loses ~11
		// E_A = 1 / (1 + 10^((1400-1600)/400)) = 1 / (1 + 10^(-0.5)) = 1 / (1 + 0.316) = 0.76
		// Winner gains: 32 * (1 - 0.76) = 32 * 0.24 = 7.68 ≈ 8
		expectedWinnerGain := 7.68
		expectedLoserLoss := 7.68

		assert.InDelta(t, expectedWinnerGain, newWinner.Score-winner.Score, 0.1)
		assert.InDelta(t, expectedLoserLoss, loser.Score-newLoser.Score, 0.1)
	})

	t.Run("standard chess example 2", func(t *testing.T) {
		// Lower rated player beats higher rated player (upset)
		// Player A (1400) beats Player B (1600)
		winner := createRating("A", 1400.0, 0)
		loser := createRating("B", 1600.0, 0)

		newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
		require.NoError(t, err)

		// Expected: Winner gains ~24, Loser loses ~24
		// E_A = 1 / (1 + 10^((1600-1400)/400)) = 1 / (1 + 10^(0.5)) = 1 / (1 + 3.16) = 0.24
		// Winner gains: 32 * (1 - 0.24) = 32 * 0.76 = 24.32 ≈ 24
		expectedWinnerGain := 24.32
		expectedLoserLoss := 24.32

		assert.InDelta(t, expectedWinnerGain, newWinner.Score-winner.Score, 0.1)
		assert.InDelta(t, expectedLoserLoss, loser.Score-newLoser.Score, 0.1)
	})

	t.Run("equal ratings example", func(t *testing.T) {
		// Equal rated players
		winner := createRating("A", 1500.0, 0)
		loser := createRating("B", 1500.0, 0)

		newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
		require.NoError(t, err)

		// Expected: Both players change by K/2 = 16
		expectedChange := 16.0

		assert.InDelta(t, expectedChange, newWinner.Score-winner.Score, 0.1)
		assert.InDelta(t, expectedChange, loser.Score-newLoser.Score, 0.1)
	})
}
