// Package elo provides Elo rating calculations for tournament and comparison systems.
// It implements the standard Chess Elo rating algorithm with support for pairwise
// comparisons, configurable parameters, and mathematical validation.
package elo

import (
	"errors"
	"math"
	"time"
)

// Error types for validation
var (
	ErrInvalidRating     = errors.New("rating value is invalid")
	ErrEmptyComparison   = errors.New("comparison contains no proposals")
	ErrDuplicateProposal = errors.New("proposal appears multiple times")
	ErrInvalidKFactor    = errors.New("k-factor must be positive")
	ErrInvalidBounds     = errors.New("min rating must be less than max rating")
)

// ComparisonMethod represents the type of comparison performed
type ComparisonMethod string

// Supported comparison methods
const (
	Pairwise ComparisonMethod = "pairwise" // Two-proposal comparison
	Trio     ComparisonMethod = "trio"     // Three-proposal ranking
	Quartet  ComparisonMethod = "quartet"  // Four-proposal ranking
)

// Rating represents a proposal's rating information
type Rating struct {
	ID         string  // Unique proposal identifier
	Score      float64 // Current Elo rating
	Confidence float64 // Statistical confidence (0.0-1.0)
	Games      int     // Number of comparisons participated in
}

// RatingUpdate represents an individual rating change record
type RatingUpdate struct {
	ProposalID string  // Proposal being updated
	OldRating  float64 // Rating before comparison
	NewRating  float64 // Rating after comparison
	Delta      float64 // Change in rating (NewRating - OldRating)
	KFactor    int     // K-factor used for this update
}

// ComparisonResult represents the result of a rating calculation with audit information
type ComparisonResult struct {
	Updates   []RatingUpdate   // Rating changes for each affected proposal
	Method    ComparisonMethod // Type of comparison performed
	Timestamp time.Time        // When calculation was performed
	Duration  time.Duration    // Time taken for calculation
}

// Config holds configuration parameters for the Elo engine
type Config struct {
	InitialRating float64 // Default rating for new proposals
	KFactor       int     // K-factor for rating sensitivity
	MinRating     float64 // Minimum allowed rating
	MaxRating     float64 // Maximum allowed rating
}

// Engine is the core Elo rating engine with configurable parameters
type Engine struct {
	InitialRating float64 // Default rating for new proposals
	KFactor       int     // K-factor for rating change sensitivity
	MinRating     float64 // Minimum allowed rating
	MaxRating     float64 // Maximum allowed rating
}

// NewEngine creates a new Elo rating engine with specified configuration
func NewEngine(config Config) (*Engine, error) {
	// Validate configuration
	if config.KFactor <= 0 {
		return nil, ErrInvalidKFactor
	}
	if config.MinRating >= config.MaxRating {
		return nil, ErrInvalidBounds
	}
	if math.IsNaN(config.InitialRating) || math.IsInf(config.InitialRating, 0) {
		return nil, ErrInvalidRating
	}

	return &Engine{
		InitialRating: config.InitialRating,
		KFactor:       config.KFactor,
		MinRating:     config.MinRating,
		MaxRating:     config.MaxRating,
	}, nil
}

// validateRating checks if a rating value is valid
func (e *Engine) validateRating(rating float64) error {
	if math.IsNaN(rating) || math.IsInf(rating, 0) {
		return ErrInvalidRating
	}
	return nil
}

// clampRating ensures a rating stays within configured bounds
func (e *Engine) clampRating(rating float64) float64 {
	if rating < e.MinRating {
		return e.MinRating
	}
	if rating > e.MaxRating {
		return e.MaxRating
	}
	return rating
}

// calculateExpectedScore computes the expected score for player A vs player B
func (e *Engine) calculateExpectedScore(ratingA, ratingB float64) float64 {
	return 1.0 / (1.0 + math.Pow(10.0, (ratingB-ratingA)/400.0))
}

// CalculatePairwise calculates new ratings for pairwise comparison
// winner: Rating of the winning proposal
// loser: Rating of the losing proposal
// Returns updated ratings for both proposals
func (e *Engine) CalculatePairwise(winner, loser Rating) (Rating, Rating, error) {
	// Validate input ratings
	if err := e.validateRating(winner.Score); err != nil {
		return Rating{}, Rating{}, err
	}
	if err := e.validateRating(loser.Score); err != nil {
		return Rating{}, Rating{}, err
	}

	// Calculate expected scores
	expectedWinner := e.calculateExpectedScore(winner.Score, loser.Score)
	expectedLoser := e.calculateExpectedScore(loser.Score, winner.Score)

	// Actual scores: winner gets 1, loser gets 0
	actualWinner := 1.0
	actualLoser := 0.0

	// Calculate rating changes
	winnerDelta := float64(e.KFactor) * (actualWinner - expectedWinner)
	loserDelta := float64(e.KFactor) * (actualLoser - expectedLoser)

	// Apply rating changes and clamp to bounds
	newWinnerScore := e.clampRating(winner.Score + winnerDelta)
	newLoserScore := e.clampRating(loser.Score + loserDelta)

	// Update game counts and confidence
	newWinner := Rating{
		ID:         winner.ID,
		Score:      newWinnerScore,
		Games:      winner.Games + 1,
		Confidence: e.calculateConfidence(winner.Games + 1),
	}

	newLoser := Rating{
		ID:         loser.ID,
		Score:      newLoserScore,
		Games:      loser.Games + 1,
		Confidence: e.calculateConfidence(loser.Games + 1),
	}

	return newWinner, newLoser, nil
}

// calculateConfidence computes confidence level based on number of games played
func (e *Engine) calculateConfidence(games int) float64 {
	// Confidence reaches 1.0 after 20 games
	confidence := float64(games) / 20.0
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

// ScaleRating converts internal Elo rating to specified output scale
// rating: Internal Elo rating
// outputMin: Minimum value of output scale
// outputMax: Maximum value of output scale
// Returns: Scaled rating value
func (e *Engine) ScaleRating(rating float64, outputMin, outputMax float64) float64 {
	// Validate input
	if err := e.validateRating(rating); err != nil {
		return outputMin // Return minimum on invalid input
	}

	// Calculate scale factor
	inputRange := e.MaxRating - e.MinRating
	outputRange := outputMax - outputMin

	// Clamp rating to engine bounds first
	clampedRating := e.clampRating(rating)

	// Linear scaling from engine range to output range
	normalized := (clampedRating - e.MinRating) / inputRange
	scaled := outputMin + (normalized * outputRange)

	return scaled
}

// CalculatePairwiseWithResult performs pairwise calculation and returns detailed result
func (e *Engine) CalculatePairwiseWithResult(winner, loser Rating) (ComparisonResult, error) {
	start := time.Now()

	newWinner, newLoser, err := e.CalculatePairwise(winner, loser)
	if err != nil {
		return ComparisonResult{}, err
	}

	// Create rating updates
	updates := []RatingUpdate{
		{
			ProposalID: winner.ID,
			OldRating:  winner.Score,
			NewRating:  newWinner.Score,
			Delta:      newWinner.Score - winner.Score,
			KFactor:    e.KFactor,
		},
		{
			ProposalID: loser.ID,
			OldRating:  loser.Score,
			NewRating:  newLoser.Score,
			Delta:      newLoser.Score - loser.Score,
			KFactor:    e.KFactor,
		},
	}

	result := ComparisonResult{
		Updates:   updates,
		Method:    Pairwise,
		Timestamp: start,
		Duration:  time.Since(start),
	}

	return result, nil
}
