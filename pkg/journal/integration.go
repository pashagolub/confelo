// Package journal provides integration functions for the data package to use export functionality
package journal

import "time"

// ConvertDataSession converts a data.Session to journal.Session for export
func ConvertDataSession(id, name, status string, createdAt, updatedAt time.Time,
	proposals []ProposalData, comparisons []ComparisonData, metrics *ConvergenceData) *Session {

	// Convert proposals
	sessionProposals := make([]Proposal, len(proposals))
	for i, p := range proposals {
		sessionProposals[i] = Proposal{
			ID:            p.ID,
			Title:         p.Title,
			Abstract:      p.Abstract,
			Speaker:       p.Speaker,
			Score:         p.Score,
			OriginalScore: p.OriginalScore,
			Metadata:      p.Metadata,
			ConflictTags:  p.ConflictTags,
			CreatedAt:     p.CreatedAt,
			UpdatedAt:     p.UpdatedAt,
		}
	}

	// Convert comparisons
	sessionComparisons := make([]Comparison, len(comparisons))
	for i, c := range comparisons {
		sessionComparisons[i] = Comparison{
			ID:          c.ID,
			SessionID:   c.SessionID,
			ProposalIDs: c.ProposalIDs,
			WinnerID:    c.WinnerID,
			Rankings:    c.Rankings,
			Method:      c.Method,
			Timestamp:   c.Timestamp,
			Duration:    c.Duration,
			Skipped:     c.Skipped,
			SkipReason:  c.SkipReason,
		}
	}

	// Convert convergence metrics
	var sessionMetrics *ConvergenceMetrics
	if metrics != nil {
		sessionMetrics = &ConvergenceMetrics{
			SessionID:           metrics.SessionID,
			TotalComparisons:    metrics.TotalComparisons,
			AvgRatingChange:     metrics.AvgRatingChange,
			RatingVariance:      metrics.RatingVariance,
			RankingStability:    metrics.RankingStability,
			CoveragePercentage:  metrics.CoveragePercentage,
			ConvergenceScore:    metrics.ConvergenceScore,
			LastCalculated:      metrics.LastCalculated,
			RecentRatingChanges: metrics.RecentRatingChanges,
		}
	}

	return &Session{
		ID:                   id,
		Name:                 name,
		Status:               status,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
		Proposals:            sessionProposals,
		CompletedComparisons: sessionComparisons,
		ConvergenceMetrics:   sessionMetrics,
	}
}

// Data transfer structs that match the data package types
type ProposalData struct {
	ID            string
	Title         string
	Abstract      string
	Speaker       string
	Score         float64
	OriginalScore *float64
	Metadata      map[string]string
	ConflictTags  []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ComparisonData struct {
	ID          string
	SessionID   string
	ProposalIDs []string
	WinnerID    string
	Rankings    []string
	Method      string
	Timestamp   time.Time
	Duration    time.Duration
	Skipped     bool
	SkipReason  string
}

type ConvergenceData struct {
	SessionID           string
	TotalComparisons    int
	AvgRatingChange     float64
	RatingVariance      float64
	RankingStability    float64
	CoveragePercentage  float64
	ConvergenceScore    float64
	LastCalculated      time.Time
	RecentRatingChanges []float64
}
