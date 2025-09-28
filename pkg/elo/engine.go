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

// GetRatingBins groups proposals into rating bins for strategic matchup selection
func (e *Engine) GetRatingBins(proposals []Rating, binSize float64) map[int][]string {
	if binSize <= 0 {
		binSize = 50.0 // Default bin size
	}

	bins := make(map[int][]string)

	for _, proposal := range proposals {
		// Calculate which bin this rating belongs to
		binIndex := int(math.Floor((proposal.Score - e.MinRating) / binSize))

		// Ensure the bin exists
		if bins[binIndex] == nil {
			bins[binIndex] = make([]string, 0)
		}

		bins[binIndex] = append(bins[binIndex], proposal.ID)
	}

	return bins
}

// GetOptimalMatchup returns the most informative next comparison
func (e *Engine) GetOptimalMatchup(proposals []Rating, history *ComparisonHistory, config OptimizationConfig) *Matchup {
	if len(proposals) < 2 {
		return nil
	}

	// Create rating bins
	bins := e.GetRatingBins(proposals, config.BinSize)

	// Create proposal lookup map
	proposalMap := make(map[string]Rating)
	for _, proposal := range proposals {
		proposalMap[proposal.ID] = proposal
	}

	bestMatchup := &Matchup{
		Priority:    0,
		Information: -1.0,
	}

	// Look for optimal matchups following priority order:
	// 1. Within-bin matchups (80-85%)
	// 2. Adjacent-bin matchups (10-15%)
	// 3. Cross-bin calibration (5-15%)

	// Track all potential matchups
	var candidates []Matchup

	// Within-bin and adjacent-bin matchups
	for binIndex, proposalIDs := range bins {
		// Within-bin matchups
		for i := range proposalIDs {
			for j := i + 1; j < len(proposalIDs); j++ {
				proposalA := proposalMap[proposalIDs[i]]
				proposalB := proposalMap[proposalIDs[j]]

				matchup := e.evaluateMatchup(proposalA, proposalB, history)
				candidates = append(candidates, matchup)
			}
		}

		// Adjacent-bin matchups
		adjacentBin := bins[binIndex+1]
		if adjacentBin != nil {
			for _, idA := range proposalIDs {
				for _, idB := range adjacentBin {
					proposalA := proposalMap[idA]
					proposalB := proposalMap[idB]

					matchup := e.evaluateMatchup(proposalA, proposalB, history)
					// Slightly lower priority for adjacent bins
					matchup.Priority = max(1, matchup.Priority-1)
					candidates = append(candidates, matchup)
				}
			}
		}
	}

	// Occasional cross-bin calibration
	if len(history.Comparisons)%10 < int(config.CrossBinRate*10) {
		// Add some cross-bin matchups for calibration
		binIndices := make([]int, 0, len(bins))
		for binIndex := range bins {
			binIndices = append(binIndices, binIndex)
		}

		for i := 0; i < len(binIndices) && i < 3; i++ {
			for j := i + 2; j < len(binIndices) && j < i+5; j++ {
				binA := bins[binIndices[i]]
				binB := bins[binIndices[j]]

				if len(binA) > 0 && len(binB) > 0 {
					// Take one representative from each bin
					proposalA := proposalMap[binA[0]]
					proposalB := proposalMap[binB[0]]

					matchup := e.evaluateMatchup(proposalA, proposalB, history)
					matchup.Priority = max(1, matchup.Priority-1) // Lower priority for cross-bin
					candidates = append(candidates, matchup)
				}
			}
		}
	}

	// Find the best matchup
	for _, candidate := range candidates {
		if candidate.Information > bestMatchup.Information {
			*bestMatchup = candidate
		}
	}

	if bestMatchup.Information < 0 {
		return nil // No valid matchup found
	}

	return bestMatchup
}

// evaluateMatchup calculates the quality metrics for a potential matchup
func (e *Engine) evaluateMatchup(proposalA, proposalB Rating, history *ComparisonHistory) Matchup {
	ratingDiff := math.Abs(proposalA.Score - proposalB.Score)
	gamesPlayed := 0

	if history != nil {
		gamesPlayed = history.GetPairComparisonCount(proposalA.ID, proposalB.ID)
	}

	information := calculateInformationGain(proposalA.Score, proposalB.Score, gamesPlayed)
	priority := calculatePriority(information)
	expectedClose := ratingDiff <= 50.0

	return Matchup{
		ProposalA:     proposalA.ID,
		ProposalB:     proposalB.ID,
		ExpectedClose: expectedClose,
		Priority:      priority,
		Information:   information,
	}
}

// CheckConvergence evaluates multiple stopping criteria and recommends session termination
func (e *Engine) CheckConvergence(
	proposals []Rating,
	history *ComparisonHistory,
	config OptimizationConfig,
) *ConvergenceStatus {
	if history == nil || len(history.Comparisons) == 0 {
		return &ConvergenceStatus{
			ShouldStop:        false,
			Confidence:        0.0,
			RemainingEstimate: config.MinCoverage * len(proposals),
			CriteriaMet:       map[string]bool{},
			Metrics:           &ConvergenceMetrics{},
		}
	}

	metrics := e.calculateConvergenceMetrics(proposals, history, config)
	criteriaMet := make(map[string]bool)

	// Criterion 1: Rating Stability - average rating change <5 points per comparison
	criteriaMet["rating_stability"] = metrics.AvgRatingChange < config.StabilityThreshold

	// Criterion 2: Ranking Stability - top N positions unchanged for stability window
	criteriaMet["ranking_stability"] = metrics.RankingStability >= 0.8 // 80% stability

	// Criterion 3: Variance Threshold - rating change variance approaches zero
	criteriaMet["variance_threshold"] = metrics.RatingVariance < (config.StabilityThreshold * 0.5)

	// Criterion 4: Minimum Coverage - each proposal in minimum comparisons
	criteriaMet["minimum_coverage"] = metrics.CoveragePercentage >= float64(config.MinCoverage)

	// Count met criteria
	metCount := 0
	for _, met := range criteriaMet {
		if met {
			metCount++
		}
	}

	// Recommend stopping if 3 out of 4 criteria are met
	shouldStop := metCount >= 3

	// Calculate confidence based on criteria satisfaction
	confidence := float64(metCount) / 4.0

	// Estimate remaining comparisons
	remaining := 0
	if !shouldStop {
		remaining = e.estimateRemainingComparisons(proposals, history, config)
	}

	return &ConvergenceStatus{
		ShouldStop:        shouldStop,
		Confidence:        confidence,
		RemainingEstimate: remaining,
		CriteriaMet:       criteriaMet,
		Metrics:           metrics,
	}
}

// calculateConvergenceMetrics computes detailed convergence analysis
func (e *Engine) calculateConvergenceMetrics(
	proposals []Rating,
	history *ComparisonHistory,
	config OptimizationConfig,
) *ConvergenceMetrics {
	recentComparisons := history.GetRecentComparisons(config.ConvergenceWindow)

	// Calculate average rating change
	var totalRatingChange float64
	var totalChanges int

	for _, comparison := range recentComparisons {
		for _, update := range comparison.Updates {
			totalRatingChange += math.Abs(update.Delta)
			totalChanges++
		}
	}

	avgRatingChange := 0.0
	if totalChanges > 0 {
		avgRatingChange = totalRatingChange / float64(totalChanges)
	}

	// Calculate rating variance
	var varianceSum float64
	for _, comparison := range recentComparisons {
		for _, update := range comparison.Updates {
			diff := math.Abs(update.Delta) - avgRatingChange
			varianceSum += diff * diff
		}
	}

	ratingVariance := 0.0
	if totalChanges > 1 {
		ratingVariance = varianceSum / float64(totalChanges-1)
	}

	// Calculate ranking stability
	rankingStability := e.calculateRankingStability(proposals, history, config)

	// Calculate coverage percentage
	coveragePercentage := e.calculateCoveragePercentage(proposals, history, config)

	return &ConvergenceMetrics{
		AvgRatingChange:    avgRatingChange,
		RatingVariance:     ratingVariance,
		RankingStability:   rankingStability,
		CoveragePercentage: coveragePercentage,
		RecentComparisons:  len(recentComparisons),
	}
}

// calculateRankingStability determines how stable the top-N rankings are
func (e *Engine) calculateRankingStability(
	proposals []Rating,
	history *ComparisonHistory,
	config OptimizationConfig,
) float64 {
	if len(history.Comparisons) < config.StabilityWindow {
		return 0.0
	}

	// Get current top-N proposals
	currentTopN := e.getTopNProposals(proposals, config.TopNForStability)

	// Look back through stability window and check consistency
	stableCount := 0
	checkPoints := min(config.StabilityWindow, len(history.Comparisons))

	for i := len(history.Comparisons) - checkPoints; i < len(history.Comparisons); i++ {
		// Reconstruct ratings at this point in time
		historicalRatings := e.reconstructRatingsAtComparison(proposals, history, i)
		historicalTopN := e.getTopNProposals(historicalRatings, config.TopNForStability)

		// Check how many top-N positions are the same
		matches := 0
		for j, currentID := range currentTopN {
			if j < len(historicalTopN) && historicalTopN[j] == currentID {
				matches++
			}
		}

		if matches >= int(float64(config.TopNForStability)*0.8) { // 80% match threshold
			stableCount++
		}
	}

	return float64(stableCount) / float64(checkPoints)
}

// calculateCoveragePercentage determines what percentage of meaningful comparisons have been made
func (e *Engine) calculateCoveragePercentage(
	proposals []Rating,
	history *ComparisonHistory,
	config OptimizationConfig,
) float64 {
	if len(proposals) == 0 {
		return 0.0
	}

	// Count how many proposals have sufficient comparisons
	sufficientCount := 0

	for _, proposal := range proposals {
		comparisonCount := 0

		// Count comparisons this proposal has participated in
		for proposalID, count := range history.PairHistory {
			if contains(proposalID, proposal.ID) {
				comparisonCount += count
			}
		}

		// Also count from rating history
		if len(history.RatingHistory[proposal.ID]) >= config.MinCoverage {
			sufficientCount++
		}
	}

	return float64(sufficientCount) / float64(len(proposals))
}

// estimateRemainingComparisons calculates how many more comparisons are likely needed
func (e *Engine) estimateRemainingComparisons(
	proposals []Rating,
	history *ComparisonHistory,
	config OptimizationConfig,
) int {
	if len(proposals) == 0 {
		return 0
	}

	// Count proposals that still need more comparisons
	needMoreComparisons := 0

	for _, proposal := range proposals {
		comparisonCount := len(history.RatingHistory[proposal.ID])
		if comparisonCount < config.MinCoverage {
			needMoreComparisons += config.MinCoverage - comparisonCount
		}
	}

	// Add some buffer for convergence
	return needMoreComparisons + (needMoreComparisons / 4)
}

// getTopNProposals returns the IDs of the top N proposals by rating
func (e *Engine) getTopNProposals(proposals []Rating, n int) []string {
	if n > len(proposals) {
		n = len(proposals)
	}

	// Create a copy and sort by rating (descending)
	sorted := make([]Rating, len(proposals))
	copy(sorted, proposals)

	// Simple bubble sort for small N
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].Score < sorted[j+1].Score {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = sorted[i].ID
	}

	return result
}

// reconstructRatingsAtComparison rebuilds the rating state at a specific point in history
func (e *Engine) reconstructRatingsAtComparison(
	currentProposals []Rating,
	history *ComparisonHistory,
	comparisonIndex int,
) []Rating {
	// For simplicity, use current ratings as approximation
	// In a full implementation, this would replay the rating history
	return currentProposals
}

// Helper functions

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetProgressMetrics returns real-time progress indicators for TUI display
func (e *Engine) GetProgressMetrics(
	proposals []Rating,
	history *ComparisonHistory,
	config OptimizationConfig,
) *ProgressMetrics {
	if history == nil {
		return &ProgressMetrics{
			TotalComparisons:   0,
			CoverageComplete:   0.0,
			ConvergenceRate:    0.0,
			EstimatedRemaining: len(proposals) * config.MinCoverage,
			TopNStable:         0,
			ConfidenceScores:   make(map[string]float64),
		}
	}

	totalComparisons := len(history.Comparisons)

	// Calculate coverage completion
	coverageComplete := e.calculateCoveragePercentage(proposals, history, config)

	// Calculate convergence rate (trend of rating changes)
	convergenceRate := e.calculateConvergenceRate(history, config)

	// Estimate remaining comparisons
	estimatedRemaining := e.estimateRemainingComparisons(proposals, history, config)

	// Calculate consecutive stable top positions
	topNStable := e.calculateConsecutiveStablePositions(proposals, history, config)

	// Calculate confidence scores per proposal
	confidenceScores := e.calculateConfidenceScores(proposals, history, config)

	return &ProgressMetrics{
		TotalComparisons:   totalComparisons,
		CoverageComplete:   coverageComplete,
		ConvergenceRate:    convergenceRate,
		EstimatedRemaining: estimatedRemaining,
		TopNStable:         topNStable,
		ConfidenceScores:   confidenceScores,
	}
}

// calculateConvergenceRate determines the trend of rating changes over time
func (e *Engine) calculateConvergenceRate(history *ComparisonHistory, config OptimizationConfig) float64 {
	recentComparisons := history.GetRecentComparisons(config.ConvergenceWindow)

	if len(recentComparisons) < 2 {
		return 1.0 // High rate when insufficient data
	}

	// Calculate moving average of rating changes
	windowSize := min(10, len(recentComparisons))
	changes := make([]float64, len(recentComparisons))

	for i, comparison := range recentComparisons {
		totalChange := 0.0
		for _, update := range comparison.Updates {
			totalChange += math.Abs(update.Delta)
		}
		changes[i] = totalChange / float64(len(comparison.Updates))
	}

	// Calculate trend: recent average vs earlier average
	if len(changes) < windowSize*2 {
		// Not enough data for trend analysis
		sum := 0.0
		for _, change := range changes {
			sum += change
		}
		return sum / float64(len(changes))
	}

	// Recent window
	recentSum := 0.0
	for i := len(changes) - windowSize; i < len(changes); i++ {
		recentSum += changes[i]
	}
	recentAvg := recentSum / float64(windowSize)

	// Earlier window
	earlierSum := 0.0
	for i := len(changes) - windowSize*2; i < len(changes)-windowSize; i++ {
		earlierSum += changes[i]
	}
	earlierAvg := earlierSum / float64(windowSize)

	// Return the rate of change (lower is better for convergence)
	if earlierAvg == 0 {
		return recentAvg
	}

	return recentAvg / earlierAvg
}

// calculateConsecutiveStablePositions counts how many top positions have been stable
func (e *Engine) calculateConsecutiveStablePositions(
	proposals []Rating,
	history *ComparisonHistory,
	config OptimizationConfig,
) int {
	if len(history.Comparisons) < config.StabilityWindow {
		return 0
	}

	currentTopN := e.getTopNProposals(proposals, config.TopNForStability)

	// Check stability over the stability window
	stablePositions := 0

	// Look back through recent comparisons
	checkPoints := min(config.StabilityWindow, len(history.Comparisons))

	for position := 0; position < len(currentTopN); position++ {
		positionStable := true
		currentID := currentTopN[position]

		// Check if this position has been stable
		for i := len(history.Comparisons) - checkPoints; i < len(history.Comparisons); i++ {
			historicalRatings := e.reconstructRatingsAtComparison(proposals, history, i)
			historicalTopN := e.getTopNProposals(historicalRatings, config.TopNForStability)

			if position >= len(historicalTopN) || historicalTopN[position] != currentID {
				positionStable = false
				break
			}
		}

		if positionStable {
			stablePositions++
		} else {
			// Stop counting at first unstable position (consecutive requirement)
			break
		}
	}

	return stablePositions
}

// calculateConfidenceScores determines confidence level for each proposal's ranking
func (e *Engine) calculateConfidenceScores(
	proposals []Rating,
	history *ComparisonHistory,
	config OptimizationConfig,
) map[string]float64 {
	confidenceScores := make(map[string]float64)

	for _, proposal := range proposals {
		// Base confidence on number of games played and rating stability
		gamesPlayed := len(history.RatingHistory[proposal.ID])

		// Games-based confidence (more games = higher confidence)
		gamesConfidence := math.Min(1.0, float64(gamesPlayed)/float64(config.MaxCoverage))

		// Rating stability confidence
		ratingProgression := history.GetRatingProgression(proposal.ID)
		stabilityConfidence := e.calculateRatingStabilityConfidence(ratingProgression, config)

		// Combined confidence
		confidence := (gamesConfidence + stabilityConfidence) / 2.0

		// Ensure confidence is between 0 and 1
		confidence = math.Max(0.0, math.Min(1.0, confidence))

		confidenceScores[proposal.ID] = confidence
	}

	return confidenceScores
}

// calculateRatingStabilityConfidence measures how stable a proposal's rating has been
func (e *Engine) calculateRatingStabilityConfidence(ratingProgression []float64, config OptimizationConfig) float64 {
	if len(ratingProgression) < 3 {
		return 0.5 // Medium confidence with insufficient data
	}

	// Calculate variance in recent ratings
	recentWindow := min(config.StabilityWindow, len(ratingProgression))
	recentRatings := ratingProgression[len(ratingProgression)-recentWindow:]

	// Calculate mean
	sum := 0.0
	for _, rating := range recentRatings {
		sum += rating
	}
	mean := sum / float64(len(recentRatings))

	// Calculate variance
	varianceSum := 0.0
	for _, rating := range recentRatings {
		diff := rating - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(recentRatings))

	// Convert variance to confidence (lower variance = higher confidence)
	// Use threshold-based mapping
	maxVariance := config.StabilityThreshold * config.StabilityThreshold
	confidence := 1.0 - (variance / maxVariance)

	return math.Max(0.0, math.Min(1.0, confidence))
}

// UpdateComparisonHistory is a convenience method to update history and trigger analysis
func (e *Engine) UpdateComparisonHistory(history *ComparisonHistory, result ComparisonResult) {
	if history != nil {
		history.AddComparison(result)
	}
}

// contains checks if a colon-separated pair contains a specific ID
func contains(pair, id string) bool {
	return pair == id+":"+id ||
		(len(pair) > len(id) &&
			(pair[:len(id)+1] == id+":" ||
				pair[len(pair)-len(id)-1:] == ":"+id))
}
