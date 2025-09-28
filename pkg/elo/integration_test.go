package elo

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func loadExpectedResults(t *testing.T) ExpectedResults {
	// Get the path to testdata/expected_results.json
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Navigate up to find the testdata directory (handling both pkg/elo and root contexts)
	testDataPath := ""
	for i := 0; i < 3; i++ { // Try up to 3 levels up
		candidate := filepath.Join(wd, "testdata", "expected_results.json")
		if _, err := os.Stat(candidate); err == nil {
			testDataPath = candidate
			break
		}
		wd = filepath.Dir(wd)
	}

	require.NotEmpty(t, testDataPath, "Could not find testdata/expected_results.json")

	file, err := os.Open(testDataPath)
	require.NoError(t, err, "Failed to open expected_results.json")
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Logf("Failed to close file: %v", closeErr)
		}
	}()

	data, err := io.ReadAll(file)
	require.NoError(t, err, "Failed to read expected_results.json")

	var results ExpectedResults
	err = json.Unmarshal(data, &results)
	require.NoError(t, err, "Failed to parse expected_results.json")

	return results
}

func TestAgainstExpectedResults(t *testing.T) {
	expected := loadExpectedResults(t)

	// Create engine with expected configuration
	config := Config{
		InitialRating: expected.InitialRating,
		KFactor:       expected.KFactor,
		MinRating:     expected.RatingBounds.RatingBounds.Minimum,
		MaxRating:     expected.RatingBounds.RatingBounds.Maximum,
	}
	engine, err := NewEngine(config)
	require.NoError(t, err)

	t.Run("main test cases", func(t *testing.T) {
		for i, testCase := range expected.TestCases {
			t.Run(fmt.Sprintf("test_case_%d_%s", i+1, testCase.Description), func(t *testing.T) {
				// Skip draw test case for now (not implemented)
				if testCase.Winner == "draw" {
					t.Skip("Draw functionality not yet implemented")
					return
				}

				// Create ratings
				ratingA := Rating{
					ID:         testCase.ProposalA.ID,
					Score:      testCase.ProposalA.InitialRating,
					Games:      0,
					Confidence: 0.0,
				}
				ratingB := Rating{
					ID:         testCase.ProposalB.ID,
					Score:      testCase.ProposalB.InitialRating,
					Games:      0,
					Confidence: 0.0,
				}

				// Determine winner and loser
				var winner, loser Rating
				if testCase.Winner == testCase.ProposalA.ID {
					winner, loser = ratingA, ratingB
				} else {
					winner, loser = ratingB, ratingA
				}

				// Calculate new ratings
				newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
				require.NoError(t, err)

				// Verify against expected results
				expectedWinner := testCase.ExpectedResults[winner.ID]
				expectedLoser := testCase.ExpectedResults[loser.ID]

				tolerance := 1.0 // Allow 1 point tolerance for rounding

				assert.InDelta(t, expectedWinner.NewRating, newWinner.Score, tolerance,
					"Winner rating mismatch for %s", winner.ID)
				assert.InDelta(t, expectedWinner.RatingChange, newWinner.Score-winner.Score, tolerance,
					"Winner rating change mismatch for %s", winner.ID)

				assert.InDelta(t, expectedLoser.NewRating, newLoser.Score, tolerance,
					"Loser rating mismatch for %s", loser.ID)
				assert.InDelta(t, expectedLoser.RatingChange, newLoser.Score-loser.Score, tolerance,
					"Loser rating change mismatch for %s", loser.ID)
			})
		}
	})

	t.Run("edge cases", func(t *testing.T) {
		for i, testCase := range expected.EdgeCases {
			t.Run(fmt.Sprintf("edge_case_%d_%s", i+1, testCase.Description), func(t *testing.T) {
				// Create ratings
				ratingA := Rating{
					ID:         testCase.ProposalA.ID,
					Score:      testCase.ProposalA.InitialRating,
					Games:      0,
					Confidence: 0.0,
				}
				ratingB := Rating{
					ID:         testCase.ProposalB.ID,
					Score:      testCase.ProposalB.InitialRating,
					Games:      0,
					Confidence: 0.0,
				}

				// Determine winner and loser
				var winner, loser Rating
				if testCase.Winner == testCase.ProposalA.ID {
					winner, loser = ratingA, ratingB
				} else {
					winner, loser = ratingB, ratingA
				}

				// Calculate new ratings
				newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
				require.NoError(t, err)

				// Verify against expected results
				expectedWinner := testCase.ExpectedResults[winner.ID]
				expectedLoser := testCase.ExpectedResults[loser.ID]

				tolerance := 1.0 // Allow 1 point tolerance for rounding

				assert.InDelta(t, expectedWinner.NewRating, newWinner.Score, tolerance,
					"Winner rating mismatch for %s", winner.ID)
				assert.InDelta(t, expectedLoser.NewRating, newLoser.Score, tolerance,
					"Loser rating mismatch for %s", loser.ID)
			})
		}
	})
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

func TestConfigurationValidation(t *testing.T) {
	expected := loadExpectedResults(t)

	t.Run("validate K-factor alternatives", func(t *testing.T) {
		for _, kFactor := range expected.RatingBounds.KFactorValidation.Alternatives {
			config := Config{
				InitialRating: expected.InitialRating,
				KFactor:       kFactor,
				MinRating:     expected.RatingBounds.RatingBounds.Minimum,
				MaxRating:     expected.RatingBounds.RatingBounds.Maximum,
			}
			engine, err := NewEngine(config)
			require.NoError(t, err, "Should accept K-factor %d", kFactor)
			assert.Equal(t, kFactor, engine.KFactor)
		}
	})

	t.Run("validate rating bounds", func(t *testing.T) {
		config := Config{
			InitialRating: expected.InitialRating,
			KFactor:       expected.KFactor,
			MinRating:     expected.RatingBounds.RatingBounds.Minimum,
			MaxRating:     expected.RatingBounds.RatingBounds.Maximum,
		}
		engine, err := NewEngine(config)
		require.NoError(t, err)

		assert.Equal(t, expected.RatingBounds.RatingBounds.Minimum, engine.MinRating)
		assert.Equal(t, expected.RatingBounds.RatingBounds.Maximum, engine.MaxRating)
	})
}
