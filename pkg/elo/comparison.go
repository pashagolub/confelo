package elo

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Additional error types for multi-way comparisons
var (
	ErrTooFewProposals  = errors.New("multi-way comparison requires at least 2 proposals")
	ErrTooManyProposals = errors.New("multi-way comparison supports at most 4 proposals")
	ErrInvalidRanking   = errors.New("ranking must contain all proposal IDs exactly once")
)

// PairwiseGame represents a single pairwise game within a multi-way comparison
type PairwiseGame struct {
	Winner       Rating  // Winner of this pairwise game
	Loser        Rating  // Loser of this pairwise game
	Weight       float64 // Position-based weight for this game (0.6-1.0)
	ExpectedWin  float64 // Expected probability of winner winning
	ActualScore  float64 // Actual score (1 for win, 0 for loss)
	RatingChange float64 // Rating change for winner (negative of loser's change)
}

// MultiWayComparison handles trio and quartet ranking comparisons
type MultiWayComparison struct {
	Engine       *Engine            // Elo engine for calculations
	Proposals    []Rating           // Proposals being compared (ranked 1st to last)
	Method       ComparisonMethod   // Trio or Quartet
	Games        []PairwiseGame     // Generated pairwise games
	TotalChanges map[string]float64 // Total rating change per proposal
}

// NewMultiWayComparison creates a new multi-way comparison from ranked proposals
// rankings: Proposals ordered from best (1st place) to worst (last place)
func (e *Engine) NewMultiWayComparison(rankings []Rating) (*MultiWayComparison, error) {
	if len(rankings) < 2 {
		return nil, ErrTooFewProposals
	}
	if len(rankings) > 4 {
		return nil, ErrTooManyProposals
	}

	// Validate unique proposal IDs
	seen := make(map[string]bool)
	for _, rating := range rankings {
		if seen[rating.ID] {
			return nil, ErrDuplicateProposal
		}
		seen[rating.ID] = true

		// Validate rating values
		if err := e.validateRating(rating.Score); err != nil {
			return nil, fmt.Errorf("invalid rating for proposal %s: %w", rating.ID, err)
		}
	}

	// Determine method based on number of proposals
	var method ComparisonMethod
	switch len(rankings) {
	case 2:
		method = Pairwise
	case 3:
		method = Trio
	case 4:
		method = Quartet
	}

	return &MultiWayComparison{
		Engine:       e,
		Proposals:    rankings,
		Method:       method,
		Games:        []PairwiseGame{},
		TotalChanges: make(map[string]float64),
	}, nil
}

// calculatePositionWeight determines the weight for a game based on position difference
// higherPos: Position of higher-ranked proposal (0-based, 0 = 1st place)
// lowerPos: Position of lower-ranked proposal (0-based)
func calculatePositionWeight(higherPos, lowerPos int) float64 {
	// Base weight starts at 1.0 and decreases by 0.2 per position
	// 1st vs others: 1.0
	// 2nd vs 3rd+: 0.8
	// 3rd vs 4th: 0.6
	baseWeight := 1.0
	positionPenalty := float64(higherPos) * 0.2
	weight := baseWeight - positionPenalty

	// Ensure minimum weight of 0.6
	if weight < 0.6 {
		weight = 0.6
	}

	return weight
}

// generatePairwiseGames creates all pairwise games for the multi-way comparison
func (mw *MultiWayComparison) generatePairwiseGames() error {
	n := len(mw.Proposals)
	mw.Games = []PairwiseGame{}

	// Generate all pairwise combinations where higher-ranked beats lower-ranked
	for i := range n {
		for j := i + 1; j < n; j++ {
			winner := mw.Proposals[i] // Higher ranked (lower index)
			loser := mw.Proposals[j]  // Lower ranked (higher index)

			// Calculate position weight
			weight := calculatePositionWeight(i, j)

			// Calculate expected score for winner
			expectedWin := mw.Engine.calculateExpectedScore(winner.Score, loser.Score)

			game := PairwiseGame{
				Winner:      winner,
				Loser:       loser,
				Weight:      weight,
				ExpectedWin: expectedWin,
				ActualScore: weight, // Winner gets weighted score (not just 1.0)
			}

			mw.Games = append(mw.Games, game)
		}
	}

	return nil
}

// calculateRatingChanges computes rating changes for all proposals
func (mw *MultiWayComparison) calculateRatingChanges() error {
	// Initialize total changes
	for _, proposal := range mw.Proposals {
		mw.TotalChanges[proposal.ID] = 0.0
	}

	// Process each pairwise game
	for i := range mw.Games {
		game := &mw.Games[i]
		winner := game.Winner
		loser := game.Loser

		// For rating conservation, calculate the unweighted change first
		// then scale by weight while maintaining zero-sum property
		expectedWinnerScore := game.ExpectedWin
		expectedLoserScore := 1.0 - expectedWinnerScore

		// Calculate base change (as if it was a normal 1.0 vs 0.0 game)
		baseWinnerChange := float64(mw.Engine.KFactor) * (1.0 - expectedWinnerScore)
		baseLoserChange := float64(mw.Engine.KFactor) * (0.0 - expectedLoserScore)

		// Apply weight scaling while preserving zero-sum
		winnerChange := baseWinnerChange * game.Weight
		loserChange := baseLoserChange * game.Weight

		game.RatingChange = winnerChange

		// Accumulate changes
		mw.TotalChanges[winner.ID] += winnerChange
		mw.TotalChanges[loser.ID] += loserChange
	}

	return nil
}

// Execute performs the multi-way comparison and returns updated ratings
func (mw *MultiWayComparison) Execute() ([]Rating, ComparisonResult, error) {
	start := time.Now()

	// Generate pairwise games
	if err := mw.generatePairwiseGames(); err != nil {
		return nil, ComparisonResult{}, err
	}

	// Calculate rating changes
	if err := mw.calculateRatingChanges(); err != nil {
		return nil, ComparisonResult{}, err
	}

	// Apply changes and create updated ratings
	updatedRatings := make([]Rating, len(mw.Proposals))
	updates := make([]RatingUpdate, 0, len(mw.Proposals))

	// Count how many games each proposal participates in
	gameCount := make(map[string]int)
	for _, game := range mw.Games {
		gameCount[game.Winner.ID]++
		gameCount[game.Loser.ID]++
	}

	for i, proposal := range mw.Proposals {
		oldScore := proposal.Score
		change := mw.TotalChanges[proposal.ID]
		newScore := mw.Engine.clampRating(oldScore + change)

		gamesPlayed := gameCount[proposal.ID]
		updatedRatings[i] = Rating{
			ID:         proposal.ID,
			Score:      newScore,
			Games:      proposal.Games + gamesPlayed,
			Confidence: mw.Engine.calculateConfidence(proposal.Games + gamesPlayed),
		}

		updates = append(updates, RatingUpdate{
			ProposalID: proposal.ID,
			OldRating:  oldScore,
			NewRating:  newScore,
			Delta:      change,
			KFactor:    mw.Engine.KFactor,
		})
	}

	duration := time.Since(start)
	if duration == 0 {
		duration = 1 * time.Nanosecond // Ensure non-zero duration
	}

	result := ComparisonResult{
		Updates:   updates,
		Method:    mw.Method,
		Timestamp: start,
		Duration:  duration,
	}

	return updatedRatings, result, nil
}

// CalculateMultiway is a convenience method on Engine for multi-way comparisons
func (e *Engine) CalculateMultiway(rankings []Rating) ([]Rating, ComparisonResult, error) {
	comparison, err := e.NewMultiWayComparison(rankings)
	if err != nil {
		return nil, ComparisonResult{}, err
	}

	return comparison.Execute()
}

// GetExpectedGameCount returns the number of pairwise games for a given number of proposals
func GetExpectedGameCount(proposalCount int) int {
	if proposalCount < 2 {
		return 0
	}
	// Combinations: n choose 2 = n! / (2! * (n-2)!) = n * (n-1) / 2
	return proposalCount * (proposalCount - 1) / 2
}

// ValidateRatingConservation checks that total rating points are conserved
func (mw *MultiWayComparison) ValidateRatingConservation() error {
	totalChange := 0.0
	for _, change := range mw.TotalChanges {
		totalChange += change
	}

	// Allow small floating-point tolerance
	tolerance := 1e-10
	if math.Abs(totalChange) > tolerance {
		return fmt.Errorf("rating conservation violated: total change = %f", totalChange)
	}

	return nil
}
