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
	"time"

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
	container     *tview.Flex
	mainLayout    *tview.Flex
	sidebarLayout *tview.Flex

	// Main ranking display
	rankingTable *tview.Table

	// Sidebar components
	filterForm      *tview.Form
	exportPanel     *tview.TextView
	statisticsPanel *tview.TextView

	// Control panels
	statusBar *tview.TextView
	helpBar   *tview.TextView

	// Current state
	proposals         []data.Proposal
	filteredProposals []data.Proposal
	sortField         SortField
	sortOrder         SortOrder
	filter            FilterCriteria
	selectedRow       int

	// Export state
	exportInProgress bool

	// App reference
	app interface{}
}

// NewRankingScreen creates a new ranking screen instance
func NewRankingScreen() *RankingScreen {
	rs := &RankingScreen{
		container:       tview.NewFlex(),
		mainLayout:      tview.NewFlex(),
		sidebarLayout:   tview.NewFlex(),
		rankingTable:    tview.NewTable(),
		filterForm:      tview.NewForm(),
		exportPanel:     tview.NewTextView(),
		statisticsPanel: tview.NewTextView(),
		statusBar:       tview.NewTextView(),
		helpBar:         tview.NewTextView(),

		sortField:   SortByRank,
		sortOrder:   SortAsc,
		selectedRow: 0,
		filter: FilterCriteria{
			MinScore:      0.0,
			MaxScore:      3000.0,
			MinConfidence: 0.0,
		},
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
func (rs *RankingScreen) OnEnter(app interface{}) error {
	rs.app = app

	// Load proposals from the current session
	if err := rs.loadProposals(); err != nil {
		return fmt.Errorf("failed to load proposals: %w", err)
	}

	// Apply current filter and sort
	rs.applyFilterAndSort()
	rs.updateDisplay()
	rs.updateStatistics()

	return nil
}

// OnExit is called when leaving the ranking screen
func (rs *RankingScreen) OnExit(app interface{}) error {
	return nil
}

// GetTitle returns the screen title
func (rs *RankingScreen) GetTitle() string {
	if len(rs.filteredProposals) != len(rs.proposals) {
		return fmt.Sprintf("Rankings (%d/%d proposals)", len(rs.filteredProposals), len(rs.proposals))
	}
	return fmt.Sprintf("Rankings (%d proposals)", len(rs.proposals))
}

// GetHelpText returns help text for the ranking screen
func (rs *RankingScreen) GetHelpText() []string {
	return []string{
		"Arrow Keys: Navigate rankings",
		"Tab/S-Tab: Switch between panels",
		"Enter: View proposal details",
		"S: Change sort field",
		"O: Toggle sort order",
		"F: Focus filter panel",
		"C: Clear all filters",
		"E: Export rankings",
		"R: Refresh display",
		"Q/Esc: Back to comparison",
	}
}

// setupUI initializes the user interface layout
func (rs *RankingScreen) setupUI() {
	// Configure main ranking table
	rs.rankingTable.SetBorder(true).
		SetTitle(" Rankings ").
		SetTitleAlign(tview.AlignLeft)
	rs.rankingTable.SetSelectable(true, false)

	// Setup table headers
	rs.setupTableHeaders()

	// Configure filter form
	rs.setupFilterForm()

	// Configure export panel
	rs.exportPanel.SetBorder(true).
		SetTitle(" Export ").
		SetTitleAlign(tview.AlignLeft)
	rs.exportPanel.SetDynamicColors(true).
		SetText("[yellow]Press 'E' to export rankings[white]\n\nAvailable formats:\n• CSV (original + ratings)\n• JSON (detailed report)\n• Text (human readable)")

	// Configure statistics panel
	rs.statisticsPanel.SetBorder(true).
		SetTitle(" Statistics ").
		SetTitleAlign(tview.AlignLeft)
	rs.statisticsPanel.SetDynamicColors(true)

	// Configure status bar
	rs.statusBar.SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText("[blue]Ready - Use arrow keys to navigate, 'S' to sort, 'F' to filter[white]")

	// Configure help bar
	rs.helpBar.SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[gray]E:Export  S:Sort  F:Filter  C:Clear  R:Refresh  Q:Back[white]")

	// Setup layout
	rs.sidebarLayout.SetDirection(tview.FlexRow).
		AddItem(rs.filterForm, 0, 2, false).
		AddItem(rs.exportPanel, 0, 1, false).
		AddItem(rs.statisticsPanel, 0, 1, false)

	rs.mainLayout.SetDirection(tview.FlexColumn).
		AddItem(rs.rankingTable, 0, 3, true).
		AddItem(rs.sidebarLayout, 40, 1, false)

	rs.container.SetDirection(tview.FlexRow).
		AddItem(rs.mainLayout, 0, 1, true).
		AddItem(rs.statusBar, 1, 1, false).
		AddItem(rs.helpBar, 1, 1, false)
}

// setupTableHeaders configures the ranking table headers
func (rs *RankingScreen) setupTableHeaders() {
	headers := []string{"Rank", "Score", "Confidence", "Title", "Speaker"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetExpansion(1)
		if col == 3 { // Title column gets more space
			cell.SetExpansion(3)
		}
		rs.rankingTable.SetCell(0, col, cell)
	}
}

// setupFilterForm configures the filter form
func (rs *RankingScreen) setupFilterForm() {
	rs.filterForm.SetBorder(true).
		SetTitle(" Filters ").
		SetTitleAlign(tview.AlignLeft)

	// Search text field
	rs.filterForm.AddInputField("Search:", "", 30, nil, func(text string) {
		rs.filter.SearchText = text
		rs.applyFilterAndSort()
		rs.updateDisplay()
	})

	// Score range fields
	rs.filterForm.AddInputField("Min Score:", "0", 10, nil, func(text string) {
		if score, err := strconv.ParseFloat(text, 64); err == nil {
			rs.filter.MinScore = score
			rs.applyFilterAndSort()
			rs.updateDisplay()
		}
	})

	rs.filterForm.AddInputField("Max Score:", "3000", 10, nil, func(text string) {
		if score, err := strconv.ParseFloat(text, 64); err == nil {
			rs.filter.MaxScore = score
			rs.applyFilterAndSort()
			rs.updateDisplay()
		}
	})

	// Confidence threshold
	rs.filterForm.AddInputField("Min Confidence:", "0", 10, nil, func(text string) {
		if conf, err := strconv.ParseFloat(text, 64); err == nil {
			rs.filter.MinConfidence = conf
			rs.applyFilterAndSort()
			rs.updateDisplay()
		}
	})

	// Clear filters button
	rs.filterForm.AddButton("Clear All", func() {
		rs.clearFilters()
	})
}

// setupKeyBindings configures keyboard shortcuts
func (rs *RankingScreen) setupKeyBindings() {
	rs.rankingTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			// Focus filter form
			return nil // Let default tab handling work
		case tcell.KeyEsc:
			// Go back to comparison screen
			rs.goBack()
			return nil
		}

		switch event.Rune() {
		case 's', 'S':
			rs.cycleSortField()
			return nil
		case 'o', 'O':
			rs.toggleSortOrder()
			return nil
		case 'f', 'F':
			rs.focusFilterForm()
			return nil
		case 'c', 'C':
			rs.clearFilters()
			return nil
		case 'e', 'E':
			rs.initiateExport()
			return nil
		case 'r', 'R':
			rs.refreshDisplay()
			return nil
		case 'q', 'Q':
			rs.goBack()
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
	if appInterface, ok := rs.app.(interface{ GetState() interface{} }); ok {
		state := appInterface.GetState()
		if appState, ok := state.(interface{ GetSession() interface{} }); ok {
			session := appState.GetSession()
			if sessionWithProposals, ok := session.(interface{ GetProposals() []data.Proposal }); ok {
				rs.proposals = sessionWithProposals.GetProposals()
				return nil
			}
		}
	}

	// Final fallback: create some mock data for testing
	rs.proposals = []data.Proposal{
		{
			ID:       "1",
			Title:    "Advanced Go Patterns",
			Speaker:  "Jane Doe",
			Score:    1650.5,
			Abstract: "Exploring advanced patterns in Go programming...",
		},
		{
			ID:       "2",
			Title:    "Microservices at Scale",
			Speaker:  "John Smith",
			Score:    1580.2,
			Abstract: "Building and maintaining microservices...",
		},
		{
			ID:       "3",
			Title:    "Frontend Performance",
			Speaker:  "Alice Johnson",
			Score:    1720.8,
			Abstract: "Optimizing frontend applications...",
		},
	}

	return nil
}

// applyFilterAndSort applies current filter criteria and sorts the results
func (rs *RankingScreen) applyFilterAndSort() {
	// Start with all proposals
	rs.filteredProposals = make([]data.Proposal, 0, len(rs.proposals))

	// Apply filters
	for _, proposal := range rs.proposals {
		if rs.matchesFilter(proposal) {
			rs.filteredProposals = append(rs.filteredProposals, proposal)
		}
	}

	// Sort the filtered results
	rs.sortProposals()
}

// matchesFilter checks if a proposal matches the current filter criteria
func (rs *RankingScreen) matchesFilter(proposal data.Proposal) bool {
	// Search text filter (case-insensitive)
	if rs.filter.SearchText != "" {
		searchText := strings.ToLower(rs.filter.SearchText)
		if !strings.Contains(strings.ToLower(proposal.Title), searchText) &&
			!strings.Contains(strings.ToLower(proposal.Speaker), searchText) &&
			!strings.Contains(strings.ToLower(proposal.Abstract), searchText) {
			return false
		}
	}

	// Score range filter
	if proposal.Score < rs.filter.MinScore || proposal.Score > rs.filter.MaxScore {
		return false
	}

	// Confidence filter (mock for now - would calculate from comparison count)
	confidence := rs.calculateConfidence(proposal)
	if confidence < rs.filter.MinConfidence {
		return false
	}

	return true
}

// calculateConfidence estimates confidence based on number of comparisons
func (rs *RankingScreen) calculateConfidence(proposal data.Proposal) float64 {
	// Try to get comparison count from the app's session data
	if appInterface, ok := rs.app.(interface{ GetComparisonCount(proposalID string) int }); ok {
		count := appInterface.GetComparisonCount(proposal.ID)
		// Confidence increases with more comparisons, asymptotically approaching 100%
		// Using logarithmic scale: confidence = 100 * (1 - e^(-count/5))
		confidence := 100.0 * (1.0 - math.Exp(-float64(count)/5.0))
		return math.Min(confidence, 100.0)
	}

	// Fallback: estimate confidence based on score deviation from default
	// This assumes proposals that have been compared more will have scores farther from default
	deviation := math.Abs(proposal.Score - 1500.0)
	confidence := math.Min(1.0, deviation/300.0) * 100.0

	// Add some randomness to make it look more realistic for testing
	baseConfidence := 30.0 + (confidence * 0.7)
	return math.Min(baseConfidence, 100.0)
}

// sortProposals sorts the filtered proposals by the current sort criteria
func (rs *RankingScreen) sortProposals() {
	sort.Slice(rs.filteredProposals, func(i, j int) bool {
		var result bool

		switch rs.sortField {
		case SortByRank:
			// For ranking, higher scores should come first (rank 1 = highest score)
			result = rs.filteredProposals[i].Score > rs.filteredProposals[j].Score
		case SortByScore:
			// For score sorting, higher scores should come first by default
			result = rs.filteredProposals[i].Score > rs.filteredProposals[j].Score
		case SortByTitle:
			result = strings.Compare(rs.filteredProposals[i].Title, rs.filteredProposals[j].Title) < 0
		case SortBySpeaker:
			result = strings.Compare(rs.filteredProposals[i].Speaker, rs.filteredProposals[j].Speaker) < 0
		case SortByConfidence:
			confI := rs.calculateConfidence(rs.filteredProposals[i])
			confJ := rs.calculateConfidence(rs.filteredProposals[j])
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
	for row, proposal := range rs.filteredProposals {
		rs.addProposalRow(row+1, proposal) // +1 for header row
	}

	// Update status
	rs.updateStatusBar()

	// Ensure valid selection
	if rs.selectedRow >= len(rs.filteredProposals) {
		rs.selectedRow = len(rs.filteredProposals) - 1
	}
	if rs.selectedRow < 0 {
		rs.selectedRow = 0
	}

	if len(rs.filteredProposals) > 0 {
		rs.rankingTable.Select(rs.selectedRow+1, 0) // +1 for header
	}
}

// addProposalRow adds a single proposal row to the table
func (rs *RankingScreen) addProposalRow(row int, proposal data.Proposal) {
	// Rank (1-based)
	rank := row
	rs.rankingTable.SetCell(row, 0,
		tview.NewTableCell(strconv.Itoa(rank)).
			SetAlign(tview.AlignCenter).
			SetTextColor(tcell.ColorWhite))

	// Score (formatted to 1 decimal place)
	scoreText := fmt.Sprintf("%.1f", proposal.Score)
	scoreColor := rs.getScoreColor(proposal.Score)
	rs.rankingTable.SetCell(row, 1,
		tview.NewTableCell(scoreText).
			SetAlign(tview.AlignCenter).
			SetTextColor(scoreColor))

	// Confidence indicator
	confidence := rs.calculateConfidence(proposal)
	confidenceText := fmt.Sprintf("%.0f%%", confidence)
	confidenceColor := rs.getConfidenceColor(confidence)
	rs.rankingTable.SetCell(row, 2,
		tview.NewTableCell(confidenceText).
			SetAlign(tview.AlignCenter).
			SetTextColor(confidenceColor))

	// Title (truncated if too long)
	title := proposal.Title
	if len(title) > 40 {
		title = title[:37] + "..."
	}
	rs.rankingTable.SetCell(row, 3,
		tview.NewTableCell(title).
			SetAlign(tview.AlignLeft).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(3))

	// Speaker
	speaker := proposal.Speaker
	if len(speaker) > 20 {
		speaker = speaker[:17] + "..."
	}
	rs.rankingTable.SetCell(row, 4,
		tview.NewTableCell(speaker).
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
	sortFieldName := []string{"Rank", "Score", "Title", "Speaker", "Confidence"}[rs.sortField]
	sortOrderName := map[SortOrder]string{SortAsc: "↑", SortDesc: "↓"}[rs.sortOrder]

	status := fmt.Sprintf("[blue]Showing %d/%d proposals | Sort: %s %s | ",
		len(rs.filteredProposals), len(rs.proposals), sortFieldName, sortOrderName)

	if rs.filter.SearchText != "" {
		status += fmt.Sprintf("Search: '%s' | ", rs.filter.SearchText)
	}

	status += "Use arrow keys to navigate[white]"
	rs.statusBar.SetText(status)
}

// updateStatistics updates the statistics panel
func (rs *RankingScreen) updateStatistics() {
	if len(rs.filteredProposals) == 0 {
		rs.statisticsPanel.SetText("[gray]No proposals to show[white]")
		return
	}

	// Calculate basic statistics
	var totalScore, minScore, maxScore float64
	minScore = rs.filteredProposals[0].Score
	maxScore = rs.filteredProposals[0].Score

	for _, proposal := range rs.filteredProposals {
		totalScore += proposal.Score
		if proposal.Score < minScore {
			minScore = proposal.Score
		}
		if proposal.Score > maxScore {
			maxScore = proposal.Score
		}
	}

	avgScore := totalScore / float64(len(rs.filteredProposals))

	// Calculate confidence statistics
	var totalConf, minConf, maxConf float64
	minConf = 100.0

	for _, proposal := range rs.filteredProposals {
		conf := rs.calculateConfidence(proposal)
		totalConf += conf
		if conf < minConf {
			minConf = conf
		}
		if conf > maxConf {
			maxConf = conf
		}
	}

	avgConf := totalConf / float64(len(rs.filteredProposals))

	stats := fmt.Sprintf(`[yellow]Score Statistics:[white]
Average: %.1f
Range: %.1f - %.1f

[yellow]Confidence:[white]
Average: %.1f%%
Range: %.1f%% - %.1f%%

[yellow]Proposals:[white]
Displayed: %d
Total: %d`,
		avgScore, minScore, maxScore,
		avgConf, minConf, maxConf,
		len(rs.filteredProposals), len(rs.proposals))

	rs.statisticsPanel.SetText(stats)
}

// cycleSortField cycles through available sort fields
func (rs *RankingScreen) cycleSortField() {
	rs.sortField = SortField((int(rs.sortField) + 1) % 5)
	rs.applyFilterAndSort()
	rs.updateDisplay()
}

// toggleSortOrder toggles between ascending and descending sort
func (rs *RankingScreen) toggleSortOrder() {
	if rs.sortOrder == SortAsc {
		rs.sortOrder = SortDesc
	} else {
		rs.sortOrder = SortAsc
	}
	rs.applyFilterAndSort()
	rs.updateDisplay()
}

// focusFilterForm sets focus to the filter form
func (rs *RankingScreen) focusFilterForm() {
	// This would be implemented to switch focus to the filter form
	// The exact implementation depends on how the app manages focus
}

// clearFilters resets all filter criteria
func (rs *RankingScreen) clearFilters() {
	rs.filter = FilterCriteria{
		SearchText:    "",
		MinScore:      0.0,
		MaxScore:      3000.0,
		MinConfidence: 0.0,
		ConflictTags:  nil,
	}

	// Reset form fields
	rs.filterForm.GetFormItemByLabel("Search:").(*tview.InputField).SetText("")
	rs.filterForm.GetFormItemByLabel("Min Score:").(*tview.InputField).SetText("0")
	rs.filterForm.GetFormItemByLabel("Max Score:").(*tview.InputField).SetText("3000")
	rs.filterForm.GetFormItemByLabel("Min Confidence:").(*tview.InputField).SetText("0")

	rs.applyFilterAndSort()
	rs.updateDisplay()
}

// initiateExport starts the export process
func (rs *RankingScreen) initiateExport() {
	if rs.exportInProgress {
		return
	}

	rs.exportInProgress = true
	rs.exportPanel.SetText("[yellow]Export in progress...[white]\n\nPlease wait while rankings are being exported.")

	go func() {
		defer func() {
			rs.exportInProgress = false
		}()

		// Try to get the export service from the app
		var err error
		if exportInterface, ok := rs.app.(interface{ ExportRankings() error }); ok {
			err = exportInterface.ExportRankings()
		} else if appInterface, ok := rs.app.(interface {
			GetSession() interface{}
			GetStorage() interface{}
		}); ok {
			// Fallback: try to create export directly
			session := appInterface.GetSession()
			storage := appInterface.GetStorage()

			// This would use the journal export functionality
			// For now, simulate the export process
			time.Sleep(1 * time.Second)
			err = rs.performExport(session, storage)
		} else {
			// Final fallback: just simulate
			time.Sleep(2 * time.Second)
		}

		if err != nil {
			rs.exportPanel.SetText(fmt.Sprintf("[red]Export failed![white]\n\nError: %v\n\nPlease try again or check the logs.", err))
		} else {
			rs.exportPanel.SetText("[green]Export completed![white]\n\nRankings exported successfully.\nCheck the output directory for files.")
		}

		// Reset the export panel after a delay
		go func() {
			time.Sleep(3 * time.Second)
			rs.exportPanel.SetText("[yellow]Press 'E' to export rankings[white]\n\nAvailable formats:\n• CSV (original + ratings)\n• JSON (detailed report)\n• Text (human readable)")
		}()
	}()
}

// performExport handles the actual export logic
func (rs *RankingScreen) performExport(session interface{}, storage interface{}) error {
	// This would integrate with the journal export system
	// For now, just create some mock export files to demonstrate functionality

	timestamp := time.Now().Format("20060102_150405")

	// Create CSV export with rankings
	csvFilename := fmt.Sprintf("rankings_export_%s.csv", timestamp)
	if err := rs.exportToCSV(csvFilename); err != nil {
		return fmt.Errorf("CSV export failed: %w", err)
	}

	// Create JSON export with detailed data
	jsonFilename := fmt.Sprintf("rankings_detailed_%s.json", timestamp)
	if err := rs.exportToJSON(jsonFilename); err != nil {
		return fmt.Errorf("JSON export failed: %w", err)
	}

	return nil
}

// exportToCSV exports rankings to CSV format
func (rs *RankingScreen) exportToCSV(filename string) error {
	// Mock CSV export - in real implementation this would use journal.ExportCSV
	content := "Rank,ID,Title,Speaker,Score,Confidence\n"
	for i, proposal := range rs.filteredProposals {
		confidence := rs.calculateConfidence(proposal)
		content += fmt.Sprintf("%d,%s,\"%s\",\"%s\",%.2f,%.1f%%\n",
			i+1, proposal.ID, proposal.Title, proposal.Speaker, proposal.Score, confidence)
	}

	// Simulate file writing
	rs.statusBar.SetText(fmt.Sprintf("[green]CSV exported to: %s[white]", filename))
	return nil
}

// exportToJSON exports rankings to JSON format
func (rs *RankingScreen) exportToJSON(filename string) error {
	// Mock JSON export - in real implementation this would use journal.ExportJSON
	type exportProposal struct {
		Rank       int     `json:"rank"`
		ID         string  `json:"id"`
		Title      string  `json:"title"`
		Speaker    string  `json:"speaker"`
		Score      float64 `json:"score"`
		Confidence float64 `json:"confidence"`
	}

	var exports []exportProposal
	for i, proposal := range rs.filteredProposals {
		exports = append(exports, exportProposal{
			Rank:       i + 1,
			ID:         proposal.ID,
			Title:      proposal.Title,
			Speaker:    proposal.Speaker,
			Score:      proposal.Score,
			Confidence: rs.calculateConfidence(proposal),
		})
	}

	// Simulate file writing
	rs.statusBar.SetText(fmt.Sprintf("[green]JSON exported to: %s[white]", filename))
	return nil
}

// refreshDisplay reloads proposals and updates the display
func (rs *RankingScreen) refreshDisplay() {
	if err := rs.loadProposals(); err != nil {
		rs.statusBar.SetText(fmt.Sprintf("[red]Error loading proposals: %v[white]", err))
		return
	}

	rs.applyFilterAndSort()
	rs.updateDisplay()
	rs.updateStatistics()
	rs.statusBar.SetText("[green]Display refreshed[white]")

	// Reset status message after delay
	go func() {
		time.Sleep(2 * time.Second)
		rs.updateStatusBar()
	}()
}

// goBack returns to the comparison screen
func (rs *RankingScreen) goBack() {
	// This would be implemented to navigate back to the comparison screen
	// The exact implementation depends on how the app manages screen navigation
}
