package elo

import (
	"math"
	"time"
)

// Matchup represents an optimal comparison between two proposals
type Matchup struct {
	ProposalA     string  // ID of first proposal
	ProposalB     string  // ID of second proposal
	ExpectedClose bool    // true if ratings within 50 points
	Priority      int     // 1-5, higher = more informative
	Information   float64 // expected information gain
}

// ConvergenceStatus evaluates multiple stopping criteria
type ConvergenceStatus struct {
	ShouldStop        bool                // recommendation to stop session
	Confidence        float64             // 0.0-1.0 confidence in current rankings
	RemainingEstimate int                 // estimated comparisons needed
	CriteriaMet       map[string]bool     // which criteria are satisfied
	Metrics           *ConvergenceMetrics // detailed convergence metrics
}

// ConvergenceMetrics provides detailed convergence analysis
type ConvergenceMetrics struct {
	AvgRatingChange    float64 // average rating change per comparison
	RatingVariance     float64 // variance of rating changes
	RankingStability   float64 // % of top-N unchanged (0.0-1.0)
	CoveragePercentage float64 // % of meaningful pairs compared (0.0-1.0)
	RecentComparisons  int     // comparisons in stability window
}

// ProgressMetrics provides real-time progress indicators
type ProgressMetrics struct {
	TotalComparisons   int                // total comparisons completed
	CoverageComplete   float64            // 0.0-1.0 coverage of proposal pairs
	ConvergenceRate    float64            // rating change per comparison (trend)
	EstimatedRemaining int                // comparisons to convergence
	TopNStable         int                // consecutive stable top positions
	ConfidenceScores   map[string]float64 // confidence per proposal (0.0-1.0)
}

// ComparisonHistory tracks comparison results for convergence analysis
type ComparisonHistory struct {
	Comparisons   []ComparisonResult   // ordered history of comparisons
	RatingHistory map[string][]float64 // rating progression per proposal
	PairHistory   map[string]int       // count of comparisons per pair
	RecentWindow  int                  // size of recent comparison window
	StartTime     time.Time            // session start time
}

// RatingBin represents a group of proposals within a rating range
type RatingBin struct {
	MinRating   float64  // minimum rating for this bin
	MaxRating   float64  // maximum rating for this bin
	ProposalIDs []string // proposals in this bin
	BinIndex    int      // bin number (0-based)
}

// OptimizationConfig holds configuration for optimization algorithms
type OptimizationConfig struct {
	BinSize            float64 // rating range per bin (default: 50.0)
	StabilityThreshold float64 // max rating change for stability (default: 5.0)
	StabilityWindow    int     // comparisons to check for stability (default: 10)
	MinCoverage        int     // minimum comparisons per proposal (default: 5)
	MaxCoverage        int     // maximum comparisons per proposal (default: 10)
	TopNForStability   int     // top N positions to track for ranking stability (default: 5)
	CrossBinRate       float64 // rate of cross-bin calibration (default: 0.15)
	ConvergenceWindow  int     // window for convergence analysis (default: 20)
}

// DefaultOptimizationConfig returns recommended optimization settings
func DefaultOptimizationConfig() OptimizationConfig {
	return OptimizationConfig{
		BinSize:            50.0,
		StabilityThreshold: 5.0,
		StabilityWindow:    10,
		MinCoverage:        5,
		MaxCoverage:        10,
		TopNForStability:   5,
		CrossBinRate:       0.15,
		ConvergenceWindow:  20,
	}
}

// NewComparisonHistory creates a new comparison history tracker
func NewComparisonHistory() *ComparisonHistory {
	return &ComparisonHistory{
		Comparisons:   make([]ComparisonResult, 0),
		RatingHistory: make(map[string][]float64),
		PairHistory:   make(map[string]int),
		RecentWindow:  20, // Default window size
		StartTime:     time.Now(),
	}
}

// AddComparison records a new comparison result
func (ch *ComparisonHistory) AddComparison(result ComparisonResult) {
	ch.Comparisons = append(ch.Comparisons, result)

	// Update rating history for each updated proposal
	for _, update := range result.Updates {
		if ch.RatingHistory[update.ProposalID] == nil {
			ch.RatingHistory[update.ProposalID] = make([]float64, 0)
		}
		ch.RatingHistory[update.ProposalID] = append(
			ch.RatingHistory[update.ProposalID],
			update.NewRating,
		)
	}

	// Update pair history (for pairwise comparisons)
	if result.Method == Pairwise && len(result.Updates) == 2 {
		pair := createPairKey(result.Updates[0].ProposalID, result.Updates[1].ProposalID)
		ch.PairHistory[pair]++
	}

	// For multi-way comparisons, update all pair combinations
	if result.Method == Trio || result.Method == Quartet {
		updates := result.Updates
		for i := 0; i < len(updates); i++ {
			for j := i + 1; j < len(updates); j++ {
				pair := createPairKey(updates[i].ProposalID, updates[j].ProposalID)
				ch.PairHistory[pair]++
			}
		}
	}
}

// GetRecentComparisons returns the most recent N comparisons
func (ch *ComparisonHistory) GetRecentComparisons(n int) []ComparisonResult {
	if n <= 0 {
		n = ch.RecentWindow
	}

	start := len(ch.Comparisons) - n
	if start < 0 {
		start = 0
	}

	return ch.Comparisons[start:]
}

// GetRatingProgression returns the rating history for a specific proposal
func (ch *ComparisonHistory) GetRatingProgression(proposalID string) []float64 {
	return ch.RatingHistory[proposalID]
}

// GetPairComparisonCount returns how many times two proposals have been compared
func (ch *ComparisonHistory) GetPairComparisonCount(proposalA, proposalB string) int {
	pair := createPairKey(proposalA, proposalB)
	return ch.PairHistory[pair]
}

// createPairKey creates a consistent key for proposal pairs
func createPairKey(proposalA, proposalB string) string {
	// Ensure consistent ordering to avoid A-B vs B-A duplicates
	if proposalA < proposalB {
		return proposalA + ":" + proposalB
	}
	return proposalB + ":" + proposalA
}

// calculateInformationGain estimates the expected information gain from a matchup
func calculateInformationGain(ratingA, ratingB float64, gamesPlayed int) float64 {
	// Information gain is higher when:
	// 1. Ratings are close (more uncertainty)
	// 2. Fewer previous games between these proposals
	// 3. Ratings are in the "interesting" middle range

	ratingDiff := math.Abs(ratingA - ratingB)

	// Closeness factor: higher when ratings are close
	closenessFactor := math.Exp(-ratingDiff / 100.0)

	// Novelty factor: higher when fewer previous games
	noveltyFactor := math.Exp(-float64(gamesPlayed) / 3.0)

	// Average rating factor: slightly favor middle ratings over extremes
	avgRating := (ratingA + ratingB) / 2.0
	middleFactor := 1.0 - math.Abs(avgRating-1400.0)/1000.0
	if middleFactor < 0.5 {
		middleFactor = 0.5
	}

	return closenessFactor * noveltyFactor * middleFactor
}

// calculatePriority determines the priority level (1-5) for a matchup
func calculatePriority(informationGain float64) int {
	// Convert information gain to priority scale
	if informationGain >= 0.8 {
		return 5 // Highest priority
	} else if informationGain >= 0.6 {
		return 4
	} else if informationGain >= 0.4 {
		return 3
	} else if informationGain >= 0.2 {
		return 2
	}
	return 1 // Lowest priority
}
