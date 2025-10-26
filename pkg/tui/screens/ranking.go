// Package screens provides TUI screen implementations for conference talk ranking.
// This file implements the ranking display screen where users view current rankings,
// filter results, and initiate export operations.
package screens

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/pashagolub/confelo/pkg/data"
)

// SortOrder represents the sorting direction for rankings
type SortOrder int

const (
	SortAsc SortOrder = iota
	SortDesc
)

// SortField represents the field to sort rankings by
type SortField int

const (
	SortByRank SortField = iota
	SortByScore
	SortByExportScore
	SortByTitle
	SortBySpeaker
	SortByConfidence
)

// FilterCriteria holds the current filtering settings
type FilterCriteria struct {
	SearchText    string   // Text to search in title/speaker/abstract
	MinScore      float64  // Minimum Elo score
	MaxScore      float64  // Maximum Elo score
	MinConfidence float64  // Minimum confidence level
	ConflictTags  []string // Filter by conflict tags
}

// RankingScreen implements the ranking display interface
type RankingScreen struct {
	// UI components
	container  *tview.Flex
	mainLayout *tview.Flex

	// Main ranking display
	rankingTable *tview.Table

	// Control panels
	statusBar *tview.TextView
	// Current state
	proposals   []data.Proposal
	sortField   SortField
	sortOrder   SortOrder
	selectedRow int

	// Export state
	exportInProgress bool

	// App reference
	app any
}

// NewRankingScreen creates a new ranking screen instance
func NewRankingScreen() *RankingScreen {
	rs := &RankingScreen{
		container:    tview.NewFlex(),
		mainLayout:   tview.NewFlex(),
		rankingTable: tview.NewTable(),
		statusBar:    tview.NewTextView(),

		sortField:   SortByRank,
		sortOrder:   SortAsc,
		selectedRow: 0,
	}

	rs.setupUI()
	rs.setupKeyBindings()

	return rs
}

// GetPrimitive returns the main primitive for the ranking screen
func (rs *RankingScreen) GetPrimitive() tview.Primitive {
	return rs.container
}

// OnEnter is called when the ranking screen becomes active
func (rs *RankingScreen) OnEnter(app any) error {
	rs.app = app

	// Load proposals from the current session
	if err := rs.loadProposals(); err != nil {
		return fmt.Errorf("failed to load proposals: %w", err)
	}

	// Apply current filter and sort
	rs.sortProposals()
	rs.updateDisplay()

	return nil
}

// OnExit is called when leaving the ranking screen
func (rs *RankingScreen) OnExit(app any) error {
	return nil
}

// GetTitle returns the screen title
func (rs *RankingScreen) GetTitle() string {
	return fmt.Sprintf("Rankings (%d proposals)", len(rs.proposals))
}

// setupUI initializes the user interface layout
func (rs *RankingScreen) setupUI() {
	// Configure main ranking table
	rs.rankingTable.SetBorder(true).
		SetTitle(" Rankings ").
		SetTitleAlign(tview.AlignCenter)
	rs.rankingTable.SetSelectable(true, false)
	rs.rankingTable.SetFixed(1, 1).SetEvaluateAllRows(true)

	// Setup table headers
	rs.setupTableHeaders()

	// Configure status bar
	rs.statusBar.SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText("[blue]Ready - Use arrow keys to navigate, 'S' to sort[-]")

	rs.mainLayout.SetDirection(tview.FlexColumn).
		AddItem(rs.rankingTable, 0, 3, true)

	rs.container.SetDirection(tview.FlexRow).
		AddItem(rs.mainLayout, 0, 1, true).
		AddItem(rs.statusBar, 1, 1, false)
}

// setupTableHeaders configures the ranking table headers
func (rs *RankingScreen) setupTableHeaders() {
	headers := []string{"Rank", "Elo", "Score", "Confidence", "Title", "Speaker"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false)

		rs.rankingTable.SetCell(0, col, cell)
	}
}

// setupKeyBindings configures keyboard shortcuts
func (rs *RankingScreen) setupKeyBindings() {
	rs.rankingTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 's', 'S':
			rs.cycleSortField()
			return nil
		case 'o', 'O':
			rs.toggleSortOrder()
			return nil
		}

		return event
	})
}

// loadProposals loads proposals from the current session
func (rs *RankingScreen) loadProposals() error {
	// Try to get proposals from the app's session
	if appInterface, ok := rs.app.(interface {
		GetProposals() ([]data.Proposal, error)
	}); ok {
		proposals, err := appInterface.GetProposals()
		if err != nil {
			return fmt.Errorf("failed to load proposals from session: %w", err)
		}
		rs.proposals = proposals
		return nil
	}

	// Fallback: try to access app state directly if available
	if appInterface, ok := rs.app.(interface{ GetState() any }); ok {
		state := appInterface.GetState()
		if appState, ok := state.(interface{ GetSession() any }); ok {
			session := appState.GetSession()
			if sessionWithProposals, ok := session.(interface{ GetProposals() []data.Proposal }); ok {
				rs.proposals = sessionWithProposals.GetProposals()
				return nil
			}
		}
	}

	return fmt.Errorf("unable to access proposals from app")
}

// calculateConfidence estimates confidence based on number of comparisons
func (rs *RankingScreen) calculateConfidence(proposal data.Proposal) float64 {
	// Try to get comparison count from the app's session data
	if appInterface, ok := rs.app.(interface{ GetComparisonCount(proposalID string) int }); ok {
		count := appInterface.GetComparisonCount(proposal.ID)

		// Get total number of proposals to adjust confidence scaling
		totalProposals := len(rs.proposals)

		// Base confidence on number of comparisons, adjusted for dataset size
		// For small datasets, fewer comparisons are needed for good confidence
		// For large datasets, more comparisons are needed
		var targetComparisons float64
		switch {
		case totalProposals <= 5:
			// Small dataset: 2-3 comparisons is reasonable
			targetComparisons = 2.5
		case totalProposals <= 20:
			// Medium dataset: 4-5 comparisons
			targetComparisons = 4.5
		case totalProposals <= 50:
			// Larger dataset: 6-8 comparisons
			targetComparisons = 7.0
		default:
			// Very large dataset: 10+ comparisons
			targetComparisons = 10.0
		}

		// Calculate confidence using adjusted scale
		// confidence = 100 * (1 - e^(-count/target))
		// This gives:
		// - For target=2.5: 1 comp≈33%, 2≈55%, 3≈70%, 5≈86%
		// - For target=4.5: 3 comp≈48%, 5≈67%, 7≈78%, 10≈89%
		baseConfidence := 100.0 * (1.0 - math.Exp(-float64(count)/targetComparisons))

		// Apply smaller penalties for small datasets
		if totalProposals <= 5 {
			// For very small datasets, be less strict
			if count < 2 {
				baseConfidence *= 0.85 // Only 15% penalty
			}
		} else {
			// For larger datasets, apply standard penalties
			if count < 3 {
				baseConfidence *= 0.7
			}
		}

		// Check if this proposal has the same score as others nearby (within ±1 rating)
		// This indicates a tie situation with high uncertainty
		if rs.hasSimilarScores(proposal, 1.0) {
			// For small datasets with ties, only small penalty
			if totalProposals <= 5 && count >= 2 {
				baseConfidence *= 0.9 // Only 10% penalty if we have some comparisons
			} else if count < 5 {
				baseConfidence *= 0.8 // Larger penalty for larger datasets or few comparisons
			}
		}

		return math.Min(baseConfidence, 100.0)
	}

	// Fallback: estimate confidence based on score deviation from default
	// This assumes proposals that have been compared more will have scores farther from default
	deviation := math.Abs(proposal.Score - 1500.0)
	confidence := math.Min(1.0, deviation/300.0) * 100.0

	// Add some randomness to make it look more realistic for testing
	baseConfidence := 30.0 + (confidence * 0.7)

	// Check for tied scores in fallback mode too
	if rs.hasSimilarScores(proposal, 1.0) {
		baseConfidence *= 0.8
	}

	return math.Min(baseConfidence, 100.0)
}

// hasSimilarScores checks if there are other proposals with similar scores
func (rs *RankingScreen) hasSimilarScores(proposal data.Proposal, threshold float64) bool {
	similarCount := 0
	for _, other := range rs.proposals {
		if other.ID == proposal.ID {
			continue
		}
		if math.Abs(other.Score-proposal.Score) <= threshold {
			similarCount++
		}
	}
	// If 2+ other proposals have similar scores, consider it a tie
	return similarCount >= 2
}

// calculateExportScore converts an Elo score to the export scale
// Uses the actual min/max Elo scores from current proposals for better scaling
func (rs *RankingScreen) calculateExportScore(eloScore float64) float64 {
	// Get config for output scale settings
	var outputMin, outputMax float64
	var useDecimals bool

	if appInterface, ok := rs.app.(interface{ GetConfig() *data.SessionConfig }); ok {
		config := appInterface.GetConfig()
		if config != nil {
			outputMin = config.Elo.OutputMin
			outputMax = config.Elo.OutputMax
			useDecimals = config.Elo.UseDecimals
		}
	} else {
		// Use defaults if config not available
		defaultConfig := data.DefaultEloConfig()
		outputMin = defaultConfig.OutputMin
		outputMax = defaultConfig.OutputMax
		useDecimals = defaultConfig.UseDecimals
	}

	// Calculate actual min/max from current proposals
	if len(rs.proposals) == 0 {
		return outputMin // No proposals, return minimum
	}

	minElo := rs.proposals[0].Score
	maxElo := rs.proposals[0].Score

	for _, proposal := range rs.proposals {
		if proposal.Score < minElo {
			minElo = proposal.Score
		}
		if proposal.Score > maxElo {
			maxElo = proposal.Score
		}
	}

	// Handle edge case where all proposals have the same score
	if minElo == maxElo {
		// Return middle of output scale
		exportScore := outputMin + (outputMax-outputMin)/2.0
		if !useDecimals {
			return float64(int(exportScore + 0.5))
		}
		return exportScore
	}

	// Linear scaling from actual [minElo, maxElo] to [outputMin, outputMax]
	eloRange := maxElo - minElo
	outputRange := outputMax - outputMin

	// Clamp to actual range
	clampedScore := eloScore
	if clampedScore < minElo {
		clampedScore = minElo
	}
	if clampedScore > maxElo {
		clampedScore = maxElo
	}

	normalized := (clampedScore - minElo) / eloRange
	exportScore := outputMin + (normalized * outputRange)

	// Round to integer if UseDecimals is false
	if !useDecimals {
		return float64(int(exportScore + 0.5))
	}

	return exportScore
}

// formatExportScore formats the export score based on the UseDecimals setting
func (rs *RankingScreen) formatExportScore(exportScore float64) string {
	// Try to get the config from the app to check UseDecimals
	useDecimals := true // default
	if appInterface, ok := rs.app.(interface{ GetConfig() *data.SessionConfig }); ok {
		config := appInterface.GetConfig()
		if config != nil {
			useDecimals = config.Elo.UseDecimals
		}
	}

	if useDecimals {
		return fmt.Sprintf("%.1f", exportScore)
	}
	return fmt.Sprintf("%d", int(exportScore))
}

// sortProposals sorts the filtered proposals by the current sort criteria
func (rs *RankingScreen) sortProposals() {
	sort.Slice(rs.proposals, func(i, j int) bool {
		var result bool

		switch rs.sortField {
		case SortByRank:
			// For ranking, higher scores should come first (rank 1 = highest score)
			result = rs.proposals[i].Score > rs.proposals[j].Score
		case SortByScore:
			// For score sorting, higher scores should come first by default
			result = rs.proposals[i].Score > rs.proposals[j].Score
		case SortByExportScore:
			// For export score sorting, higher export scores should come first
			exportI := rs.calculateExportScore(rs.proposals[i].Score)
			exportJ := rs.calculateExportScore(rs.proposals[j].Score)
			result = exportI > exportJ
		case SortByTitle:
			result = strings.Compare(rs.proposals[i].Title, rs.proposals[j].Title) < 0
		case SortBySpeaker:
			result = strings.Compare(rs.proposals[i].Speaker, rs.proposals[j].Speaker) < 0
		case SortByConfidence:
			confI := rs.calculateConfidence(rs.proposals[i])
			confJ := rs.calculateConfidence(rs.proposals[j])
			result = confI > confJ
		}

		if rs.sortOrder == SortDesc {
			result = !result
		}

		return result
	})
}

// updateDisplay refreshes the ranking table with current data
func (rs *RankingScreen) updateDisplay() {
	// Clear existing content (except headers)
	rs.rankingTable.Clear()
	rs.setupTableHeaders()

	// Add proposal rows
	for row, proposal := range rs.proposals {
		rs.addProposalRow(row+1, proposal) // +1 for header row
	}

	// Update status
	rs.updateStatusBar()

	// Ensure valid selection
	if rs.selectedRow >= len(rs.proposals) {
		rs.selectedRow = len(rs.proposals) - 1
	}
	if rs.selectedRow < 0 {
		rs.selectedRow = 0
	}

	if len(rs.proposals) > 0 {
		rs.rankingTable.Select(rs.selectedRow+1, 0) // +1 for header
	}
}

// addProposalRow adds a single proposal row to the table
func (rs *RankingScreen) addProposalRow(row int, proposal data.Proposal) {
	// Rank (1-based)
	rs.rankingTable.SetCell(row, 0,
		tview.NewTableCell(strconv.Itoa(row)).
			SetAlign(tview.AlignCenter).
			SetTextColor(tcell.ColorWhite))

	// Score (formatted to 1 decimal place)
	scoreText := fmt.Sprintf("%.1f", proposal.Score)
	scoreColor := rs.getScoreColor(proposal.Score)
	rs.rankingTable.SetCell(row, 1,
		tview.NewTableCell(scoreText).
			SetAlign(tview.AlignCenter).
			SetTextColor(scoreColor))

	// Export Score - convert Elo score to output scale
	exportScore := rs.calculateExportScore(proposal.Score)
	exportText := rs.formatExportScore(exportScore)
	rs.rankingTable.SetCell(row, 2,
		tview.NewTableCell(exportText).
			SetAlign(tview.AlignCenter).
			SetTextColor(scoreColor)) // Use same color as regular score

	// Confidence indicator
	confidence := rs.calculateConfidence(proposal)
	confidenceText := fmt.Sprintf("%.0f%%", confidence)
	confidenceColor := rs.getConfidenceColor(confidence)
	rs.rankingTable.SetCell(row, 3,
		tview.NewTableCell(confidenceText).
			SetAlign(tview.AlignCenter).
			SetTextColor(confidenceColor))

	// Title (truncated if too long)
	rs.rankingTable.SetCell(row, 4,
		tview.NewTableCell(proposal.Title).
			SetAlign(tview.AlignLeft).
			SetTextColor(tcell.ColorWhite))

	// Speaker
	rs.rankingTable.SetCell(row, 5,
		tview.NewTableCell(proposal.Speaker).
			SetAlign(tview.AlignLeft).
			SetTextColor(tcell.ColorLightBlue))
}

// getScoreColor returns appropriate color for a score value
func (rs *RankingScreen) getScoreColor(score float64) tcell.Color {
	if score >= 1600 {
		return tcell.ColorGreen
	} else if score >= 1400 {
		return tcell.ColorYellow
	} else {
		return tcell.ColorRed
	}
}

// getConfidenceColor returns appropriate color for confidence level
func (rs *RankingScreen) getConfidenceColor(confidence float64) tcell.Color {
	if confidence >= 75 {
		return tcell.ColorGreen
	} else if confidence >= 50 {
		return tcell.ColorYellow
	} else {
		return tcell.ColorRed
	}
}

// updateStatusBar updates the status bar with current information
func (rs *RankingScreen) updateStatusBar() {
	sortFieldName := []string{"Rank", "Score", "Export", "Title", "Speaker", "Confidence"}[rs.sortField]
	sortOrderName := map[SortOrder]string{SortAsc: "↑", SortDesc: "↓"}[rs.sortOrder]

	status := fmt.Sprintf("[blue]S: Sort (%s) | O: Order (%s) | Use arrow keys to navigate[-]",
		sortFieldName, sortOrderName)
	rs.statusBar.SetText(status)
}

// cycleSortField cycles through available sort fields
func (rs *RankingScreen) cycleSortField() {
	rs.sortField = SortField((int(rs.sortField) + 1) % 6)
	rs.sortProposals()
	rs.updateDisplay()
}

// toggleSortOrder toggles between ascending and descending sort
func (rs *RankingScreen) toggleSortOrder() {
	if rs.sortOrder == SortAsc {
		rs.sortOrder = SortDesc
	} else {
		rs.sortOrder = SortAsc
	}
	rs.sortProposals()
	rs.updateDisplay()
}
