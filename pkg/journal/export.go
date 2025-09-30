// Package journal provides audit trail and export functionality for the conference
// talk ranking application. It implements ranking export with multiple formats,
// confidence scores, and original CSV format preservation for transparent results.
package journal

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	texttemplate "text/template"
	"time"
)

// Session types needed for export (to avoid import cycle)
type Session struct {
	ID                   string
	Name                 string
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Proposals            []Proposal
	CompletedComparisons []Comparison
	ConvergenceMetrics   *ConvergenceMetrics
}

type Proposal struct {
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

type Comparison struct {
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

type ConvergenceMetrics struct {
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

// ExportFormat represents the format for exporting results
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
	FormatText ExportFormat = "text"
)

// ExportOptions configures export behavior
type ExportOptions struct {
	Format       ExportFormat `json:"format"`        // Export format (csv, json, text)
	IncludeStats bool         `json:"include_stats"` // Include confidence scores and stats
	IncludeAudit bool         `json:"include_audit"` // Include comparison history
	CustomFields []string     `json:"custom_fields"` // Additional fields to include
}

// ExportTemplate defines custom export formatting
type ExportTemplate struct {
	Name         string `json:"name"`        // Template name
	Description  string `json:"description"` // Template description
	HeaderFormat string `json:"header"`      // Header template
	RowFormat    string `json:"row"`         // Row template
	FooterFormat string `json:"footer"`      // Footer template
}

// RankingExport represents the complete export data structure
type RankingExport struct {
	SessionID       string                 `json:"session_id"`
	SessionName     string                 `json:"session_name"`
	ExportedAt      time.Time              `json:"exported_at"`
	Rankings        []RankedProposal       `json:"rankings"`
	Statistics      *ExportStatistics      `json:"statistics,omitempty"`
	AuditTrail      []Comparison           `json:"audit_trail,omitempty"`
	Comparisons     []Comparison           `json:"comparisons,omitempty"`
	ConvergenceInfo *ConvergenceInfo       `json:"convergence_info,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// RankedProposal represents a proposal with ranking information
type RankedProposal struct {
	Rank            int               `json:"rank"`
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Abstract        string            `json:"abstract,omitempty"`
	Speaker         string            `json:"speaker,omitempty"`
	Score           float64           `json:"score"`
	OriginalScore   *float64          `json:"original_score,omitempty"`
	ConfidenceScore float64           `json:"confidence_score"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	ConflictTags    []string          `json:"conflict_tags,omitempty"`
}

// ExportStatistics provides summary statistics
type ExportStatistics struct {
	TotalProposals    int     `json:"total_proposals"`
	TotalComparisons  int     `json:"total_comparisons"`
	AverageScore      float64 `json:"average_score"`
	ScoreRange        float64 `json:"score_range"`
	StandardDeviation float64 `json:"standard_deviation"`
	CompletionTime    string  `json:"completion_time"`
	SessionDuration   string  `json:"session_duration"`
}

// ConvergenceInfo provides convergence analysis
type ConvergenceInfo struct {
	IsConverged        bool    `json:"is_converged"`
	ConvergenceScore   float64 `json:"convergence_score"`
	RankingStability   float64 `json:"ranking_stability"`
	CoveragePercentage float64 `json:"coverage_percentage"`
	AvgRatingChange    float64 `json:"avg_rating_change"`
	RatingVariance     float64 `json:"rating_variance"`
	RecommendStop      bool    `json:"recommend_stop"`
}

// Exporter handles ranking export operations
type Exporter struct {
	// Internal state can be added here if needed
}

// NewExporter creates a new exporter instance
func NewExporter() *Exporter {
	return &Exporter{}
}

// ExportToFile exports session results to a file with the specified format
func (e *Exporter) ExportToFile(session *Session, filePath string, options ExportOptions) error {
	// Try to create directory if needed
	dir := filepath.Dir(filePath)
	if dir != "." && dir != filepath.Dir(".") {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create temporary file for atomic writes
	tempFile := filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		file.Close()
		if err != nil {
			os.Remove(tempFile) // Clean up on error
		}
	}()

	// Export to the temporary file
	switch options.Format {
	case FormatCSV:
		err = e.ExportCSV(session, file, options)
	case FormatJSON:
		err = e.ExportJSON(session, file, options)
	case FormatText:
		err = e.ExportRankingReport(session, file, options)
	default:
		err = fmt.Errorf("unsupported export format: %s", options.Format)
	}

	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Close file before rename
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Atomically replace the target file
	if err := os.Rename(tempFile, filePath); err != nil {
		os.Remove(tempFile) // Clean up
		return fmt.Errorf("failed to replace target file: %w", err)
	}

	return nil
}

// ExportCSV exports rankings in CSV format, preserving original structure
func (e *Exporter) ExportCSV(session *Session, writer io.Writer, options ExportOptions) error {
	proposals := e.getSortedProposals(session)
	if len(proposals) == 0 {
		return fmt.Errorf("no proposals to export")
	}

	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Build header from first proposal's metadata and standard fields
	var headers []string
	stdFields := []string{"id", "title", "abstract", "speaker", "score"}

	// Add standard fields that exist
	for _, field := range stdFields {
		switch field {
		case "abstract":
			if proposals[0].Abstract != "" || hasAnyAbstract(proposals) {
				headers = append(headers, field)
			}
		case "speaker":
			if proposals[0].Speaker != "" || hasAnySpeaker(proposals) {
				headers = append(headers, field)
			}
		default:
			headers = append(headers, field)
		}
	}

	// Add metadata fields (preserve original CSV columns)
	metadataKeys := getMetadataKeys(proposals)
	sort.Strings(metadataKeys) // Consistent ordering
	headers = append(headers, metadataKeys...)

	// Add statistical fields if requested
	if options.IncludeStats {
		headers = append(headers, "rank", "confidence_score")
	}

	// Write header
	if err := csvWriter.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Calculate confidence scores if needed
	var confidenceScores map[string]float64
	if options.IncludeStats {
		confidenceScores = e.calculateConfidenceScores(session)
	}

	// Write data rows
	for rank, proposal := range proposals {
		var record []string

		// Standard fields
		for _, header := range headers {
			switch header {
			case "id":
				record = append(record, proposal.ID)
			case "title":
				record = append(record, proposal.Title)
			case "abstract":
				record = append(record, proposal.Abstract)
			case "speaker":
				record = append(record, proposal.Speaker)
			case "score":
				record = append(record, formatFloat(proposal.Score))
			case "rank":
				record = append(record, strconv.Itoa(rank+1))
			case "confidence_score":
				if score, ok := confidenceScores[proposal.ID]; ok {
					record = append(record, formatFloat(score))
				} else {
					record = append(record, "0.0")
				}

			default:
				// Check if it's a metadata field
				if value, ok := proposal.Metadata[header]; ok {
					record = append(record, value)
				} else {
					record = append(record, "")
				}
			}
		}

		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record for proposal %s: %w", proposal.ID, err)
		}
	}

	return nil
}

// ExportJSON exports rankings in JSON format with comprehensive data
func (e *Exporter) ExportJSON(session *Session, writer io.Writer, options ExportOptions) error {
	export := &RankingExport{
		SessionID:   session.ID,
		SessionName: session.Name,
		ExportedAt:  time.Now(),
		Rankings:    e.buildRankedProposals(session, options.IncludeStats),
		Metadata: map[string]interface{}{
			"session_status": session.Status,
			"created_at":     session.CreatedAt,
			"updated_at":     session.UpdatedAt,
			"export_options": options,
		},
	}

	// Add statistics if requested
	if options.IncludeStats {
		export.Statistics = e.calculateExportStatistics(session)
		export.ConvergenceInfo = e.buildConvergenceInfo(session)
	}

	// Add audit trail if requested
	if options.IncludeAudit {
		export.AuditTrail = session.CompletedComparisons
		export.Comparisons = session.CompletedComparisons
	}

	// Encode to JSON with formatting
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(export); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// ExportRankingReport generates a human-readable text report
func (e *Exporter) ExportRankingReport(session *Session, writer io.Writer, options ExportOptions) error {
	proposals := e.getSortedProposals(session)
	statistics := e.calculateExportStatistics(session)
	convergence := e.buildConvergenceInfo(session)

	// Write report header
	fmt.Fprintf(writer, "Conference Talk Ranking Report\n")
	fmt.Fprintf(writer, "=====================================\n\n")
	fmt.Fprintf(writer, "Session: %s\n", session.Name)
	fmt.Fprintf(writer, "Session ID: %s\n", session.ID)
	fmt.Fprintf(writer, "Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(writer, "Status: %s\n\n", session.Status)

	// Statistics section (always show in text reports unless explicitly disabled)
	if statistics != nil {
		fmt.Fprintf(writer, "Session Statistics\n")
		fmt.Fprintf(writer, "------------------\n")
		fmt.Fprintf(writer, "Total Proposals: %d\n", statistics.TotalProposals)
		fmt.Fprintf(writer, "Total Comparisons: %d\n", statistics.TotalComparisons)
		fmt.Fprintf(writer, "Average Score: %.1f\n", statistics.AverageScore)
		fmt.Fprintf(writer, "Score Range: %.1f\n", statistics.ScoreRange)
		fmt.Fprintf(writer, "Standard Deviation: %.1f\n", statistics.StandardDeviation)

		if convergence != nil {
			fmt.Fprintf(writer, "\nConvergence Analysis\n")
			fmt.Fprintf(writer, "-------------------\n")
			fmt.Fprintf(writer, "Convergence Score: %.1f%%\n", convergence.ConvergenceScore*100)
			fmt.Fprintf(writer, "Ranking Stability: %.1f%%\n", convergence.RankingStability*100)
			fmt.Fprintf(writer, "Coverage: %.1f%%\n", convergence.CoveragePercentage)
			fmt.Fprintf(writer, "Avg Rating Change: %.2f\n", convergence.AvgRatingChange)
			fmt.Fprintf(writer, "Rating Variance: %.2f\n", convergence.RatingVariance)
			if convergence.RecommendStop {
				fmt.Fprintf(writer, "Status: ✓ Converged - Rankings are stable\n")
			} else {
				fmt.Fprintf(writer, "Status: ⚠ Not fully converged - More comparisons recommended\n")
			}
		}
		fmt.Fprintf(writer, "\n")
	}

	// Rankings section
	fmt.Fprintf(writer, "Final Rankings\n")
	fmt.Fprintf(writer, "==============\n\n")

	confidenceScores := e.calculateConfidenceScores(session)

	for i, proposal := range proposals {
		rank := i + 1
		fmt.Fprintf(writer, "%d. %s\n", rank, proposal.Title)

		if proposal.Speaker != "" {
			fmt.Fprintf(writer, "   Speaker: %s\n", proposal.Speaker)
		}

		fmt.Fprintf(writer, "   Score: %.1f", proposal.Score)

		if confidence, ok := confidenceScores[proposal.ID]; ok {
			fmt.Fprintf(writer, " | Confidence: %.0f%%", confidence*100)
		}
		fmt.Fprintf(writer, "\n")

		if proposal.Abstract != "" && len(proposal.Abstract) < 200 {
			fmt.Fprintf(writer, "   %s\n", proposal.Abstract)
		} else if proposal.Abstract != "" {
			fmt.Fprintf(writer, "   %s...\n", proposal.Abstract[:197])
		}

		fmt.Fprintf(writer, "\n")
	}

	// Audit information if requested
	if options.IncludeAudit {
		comparisons := session.CompletedComparisons
		fmt.Fprintf(writer, "Comparison History\n")
		fmt.Fprintf(writer, "==================\n\n")
		fmt.Fprintf(writer, "Total Comparisons: %d\n\n", len(comparisons))

		// Show recent comparisons
		recentCount := 10
		if len(comparisons) < recentCount {
			recentCount = len(comparisons)
		}

		if recentCount > 0 {
			fmt.Fprintf(writer, "Recent Comparisons:\n")
			for i := len(comparisons) - recentCount; i < len(comparisons); i++ {
				comp := comparisons[i]
				fmt.Fprintf(writer, "- %s: %s vs %s",
					comp.Timestamp.Format("15:04:05"),
					comp.ProposalIDs[0],
					comp.ProposalIDs[1])
				if comp.WinnerID != "" {
					fmt.Fprintf(writer, " → Winner: %s", comp.WinnerID)
				} else {
					fmt.Fprintf(writer, " → Skipped")
				}
				fmt.Fprintf(writer, "\n")
			}
		}
	}

	return nil
}

// ExportWithTemplate exports using a custom template
func (e *Exporter) ExportWithTemplate(session *Session, writer io.Writer, template ExportTemplate, options ExportOptions) error {
	proposals := e.getSortedProposals(session)

	// Template data
	data := struct {
		Session        *Session
		Proposals      []Proposal
		TotalProposals int
		Timestamp      string
		Rankings       []RankedProposal
	}{
		Session:        session,
		Proposals:      proposals,
		TotalProposals: len(proposals),
		Timestamp:      time.Now().Format("2006-01-02 15:04:05"),
		Rankings:       e.buildRankedProposals(session, options.IncludeStats),
	}

	// Parse and execute header template
	if template.HeaderFormat != "" {
		tmpl, err := texttemplate.New("header").Parse(template.HeaderFormat)
		if err != nil {
			return fmt.Errorf("failed to parse header template: %w", err)
		}
		if err := tmpl.Execute(writer, data); err != nil {
			return fmt.Errorf("failed to execute header template: %w", err)
		}
	}

	// Execute row template for each proposal
	if template.RowFormat != "" {
		tmpl, err := texttemplate.New("row").Parse(template.RowFormat)
		if err != nil {
			return fmt.Errorf("failed to parse row template: %w", err)
		}

		for i, ranking := range data.Rankings {
			rowData := struct {
				RankedProposal
				Index int
			}{
				RankedProposal: ranking,
				Index:          i,
			}

			if err := tmpl.Execute(writer, rowData); err != nil {
				return fmt.Errorf("failed to execute row template for proposal %s: %w", ranking.ID, err)
			}
		}
	}

	// Parse and execute footer template
	if template.FooterFormat != "" {
		tmpl, err := texttemplate.New("footer").Parse(template.FooterFormat)
		if err != nil {
			return fmt.Errorf("failed to parse footer template: %w", err)
		}
		if err := tmpl.Execute(writer, data); err != nil {
			return fmt.Errorf("failed to execute footer template: %w", err)
		}
	}

	return nil
}

// Helper functions

// getSortedProposals returns proposals sorted by score (descending)
func (e *Exporter) getSortedProposals(session *Session) []Proposal {
	proposals := make([]Proposal, len(session.Proposals))
	copy(proposals, session.Proposals)

	sort.Slice(proposals, func(i, j int) bool {
		return proposals[i].Score > proposals[j].Score
	})

	return proposals
}

// buildRankedProposals creates ranked proposal objects with optional statistics
func (e *Exporter) buildRankedProposals(session *Session, includeStats bool) []RankedProposal {
	proposals := e.getSortedProposals(session)
	ranked := make([]RankedProposal, len(proposals))

	var confidenceScores map[string]float64
	if includeStats {
		confidenceScores = e.calculateConfidenceScores(session)
	}

	for i, proposal := range proposals {
		ranked[i] = RankedProposal{
			Rank:          i + 1,
			ID:            proposal.ID,
			Title:         proposal.Title,
			Abstract:      proposal.Abstract,
			Speaker:       proposal.Speaker,
			Score:         proposal.Score,
			OriginalScore: proposal.OriginalScore,
			Metadata:      proposal.Metadata,
			ConflictTags:  proposal.ConflictTags,
		}

		if includeStats {
			if score, ok := confidenceScores[proposal.ID]; ok {
				ranked[i].ConfidenceScore = score
			}
		}
	}

	return ranked
}

// calculateConfidenceScores estimates ranking confidence for each proposal
func (e *Exporter) calculateConfidenceScores(session *Session) map[string]float64 {
	scores := make(map[string]float64)

	// Simple heuristic: confidence based on number of comparisons and score stability
	comparisons := session.CompletedComparisons
	participationCount := make(map[string]int)

	// Count how many comparisons each proposal participated in
	for _, comp := range comparisons {
		for _, proposalID := range comp.ProposalIDs {
			participationCount[proposalID]++
		}
	}

	// Base confidence on participation and convergence
	baseConfidence := 0.5
	if session.ConvergenceMetrics != nil {
		baseConfidence = session.ConvergenceMetrics.ConvergenceScore
	}

	for _, proposal := range session.Proposals {
		participations := participationCount[proposal.ID]

		// More comparisons = higher confidence, up to a limit
		participationFactor := float64(participations) / 10.0 // Normalize to ~10 comparisons
		if participationFactor > 1.0 {
			participationFactor = 1.0
		}

		// Combine base convergence with participation
		confidence := (baseConfidence + participationFactor) / 2.0
		if confidence > 1.0 {
			confidence = 1.0
		}

		scores[proposal.ID] = confidence
	}

	return scores
}

// calculateExportStatistics computes summary statistics for the session
func (e *Exporter) calculateExportStatistics(session *Session) *ExportStatistics {
	if len(session.Proposals) == 0 {
		return nil
	}

	scores := make([]float64, len(session.Proposals))
	var sum, min, max float64

	for i, proposal := range session.Proposals {
		score := proposal.Score
		scores[i] = score
		sum += score

		if i == 0 || score < min {
			min = score
		}
		if i == 0 || score > max {
			max = score
		}
	}

	average := sum / float64(len(scores))

	// Calculate standard deviation
	var variance float64
	for _, score := range scores {
		diff := score - average
		variance += diff * diff
	}
	variance /= float64(len(scores))
	stdDev := math.Sqrt(variance)

	// Session duration
	duration := session.UpdatedAt.Sub(session.CreatedAt)

	return &ExportStatistics{
		TotalProposals:    len(session.Proposals),
		TotalComparisons:  len(session.CompletedComparisons),
		AverageScore:      average,
		ScoreRange:        max - min,
		StandardDeviation: stdDev,
		CompletionTime:    session.UpdatedAt.Format("2006-01-02 15:04:05"),
		SessionDuration:   formatDuration(duration),
	}
}

// buildConvergenceInfo creates convergence information from session metrics
func (e *Exporter) buildConvergenceInfo(session *Session) *ConvergenceInfo {
	if session.ConvergenceMetrics == nil {
		return nil
	}

	metrics := session.ConvergenceMetrics

	// Simple heuristic for convergence recommendation
	recommendStop := metrics.ConvergenceScore > 0.8 &&
		metrics.RatingVariance < 5.0 &&
		metrics.CoveragePercentage > 50.0

	return &ConvergenceInfo{
		IsConverged:        recommendStop,
		ConvergenceScore:   metrics.ConvergenceScore,
		RankingStability:   metrics.RankingStability,
		CoveragePercentage: metrics.CoveragePercentage,
		AvgRatingChange:    metrics.AvgRatingChange,
		RatingVariance:     metrics.RatingVariance,
		RecommendStop:      recommendStop,
	}
}

// Utility functions

// hasAnyAbstract checks if any proposal has an abstract
func hasAnyAbstract(proposals []Proposal) bool {
	for _, p := range proposals {
		if p.Abstract != "" {
			return true
		}
	}
	return false
}

// hasAnySpeaker checks if any proposal has a speaker
func hasAnySpeaker(proposals []Proposal) bool {
	for _, p := range proposals {
		if p.Speaker != "" {
			return true
		}
	}
	return false
}

// getMetadataKeys returns all unique metadata keys across proposals
func getMetadataKeys(proposals []Proposal) []string {
	keySet := make(map[string]bool)
	for _, proposal := range proposals {
		for key := range proposal.Metadata {
			keySet[key] = true
		}
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}

	return keys
}

// formatFloat formats a float with appropriate precision
func formatFloat(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

// formatDuration formats a duration for human reading
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
