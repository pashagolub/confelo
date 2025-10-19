// Package screens provides TUI screen implementations for conference talk ranking.
// This file implements the comparison screen interface where users perform
// pairwise and multi-way comparisons between proposals.
package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/pashagolub/confelo/pkg/data"
	"github.com/pashagolub/confelo/pkg/elo"
)

// ComparisonScreen implements the comparison interface for ranking proposals
type ComparisonScreen struct {
	// UI components
	container      *tview.Flex
	leftPanel      *tview.Flex
	rightPanel     *tview.Flex
	proposalsPanel *tview.Flex
	proposalCards  []*tview.TextView // Dynamic array for 2-4 proposals
	controlPanel   *tview.TextView
	progressBar    *tview.TextView
	statusBar      *tview.TextView

	// Comparison state
	currentProposals []data.Proposal
	comparisonMethod data.ComparisonMethod
	selectedWinner   string
	rankings         []string       // Final ranking order (1st, 2nd, 3rd, 4th)
	proposalRanks    map[string]int // Maps proposal ID to assigned rank (1-4)
	isRanking        bool
	currentRank      int // Next rank to assign (1-4)

	// App reference - we'll use any and cast as needed
	app any
}

// NewComparisonScreen creates a new comparison screen instance
func NewComparisonScreen() *ComparisonScreen {
	cs := &ComparisonScreen{
		container:        tview.NewFlex(),
		leftPanel:        tview.NewFlex(),
		rightPanel:       tview.NewFlex(),
		proposalsPanel:   tview.NewFlex(),
		proposalCards:    make([]*tview.TextView, 0, 4), // Start empty, max 4
		controlPanel:     tview.NewTextView(),
		progressBar:      tview.NewTextView(),
		statusBar:        tview.NewTextView(),
		comparisonMethod: data.MethodPairwise,
	}

	cs.setupUI()
	return cs
}

// setupUI initializes the comparison screen layout
func (cs *ComparisonScreen) setupUI() {
	// Configure main container as horizontal split
	cs.container.SetDirection(tview.FlexColumn)

	// Setup left panel for proposal display
	cs.leftPanel.SetDirection(tview.FlexRow).
		SetBorder(true).
		SetTitle("Proposals for Comparison").
		SetBorderColor(tcell.ColorBlue)

	// Setup right panel for controls
	cs.rightPanel.SetDirection(tview.FlexRow).
		SetBorder(true).
		SetTitle("Comparison Controls").
		SetBorderColor(tcell.ColorGreen)

	// Configure proposal carousel
	// Carousel is configured through its own methods

	// Proposal cards will be created dynamically based on comparison method
	// This allows support for pairwise (2), trio (3), or quartet (4) comparisons

	// Configure control panel - use TextView methods correctly
	cs.controlPanel.
		SetBorder(true).
		SetTitle("Instructions")
	cs.controlPanel.SetWordWrap(true)
	cs.controlPanel.SetDynamicColors(true)

	// Configure progress bar
	cs.progressBar.
		SetBorder(true).
		SetTitle("Progress")
	cs.progressBar.SetDynamicColors(true)

	// Configure status bar
	cs.statusBar.
		SetBorder(true).
		SetTitle("Status")
	cs.statusBar.SetDynamicColors(true)

	// Proposals panel will be populated dynamically when comparison starts

	// Layout left panel: proposals panel takes all space
	cs.leftPanel.AddItem(cs.proposalsPanel, 0, 1, false)

	// Layout right panel: controls, progress, status
	cs.rightPanel.
		AddItem(cs.controlPanel, 0, 2, false).
		AddItem(cs.progressBar, 6, 0, false).
		AddItem(cs.statusBar, 3, 0, false)

	// Main container: left panel (70%) and right panel (30%)
	cs.container.
		AddItem(cs.leftPanel, 0, 75, true).
		AddItem(cs.rightPanel, 0, 25, false)

	// Set up input handling
	cs.container.SetInputCapture(cs.handleInput)

	// Initialize with default instructions
	cs.updateInstructions()
}

// setupProposalCards creates the required number of proposal cards based on comparison method
func (cs *ComparisonScreen) setupProposalCards(count int) {
	// Clear existing proposal cards from the layout
	cs.proposalsPanel.Clear()

	// Create new proposal cards
	cs.proposalCards = make([]*tview.TextView, count)

	for i := 0; i < count; i++ {
		card := tview.NewTextView()
		card.SetBorder(true)
		card.SetTitle(fmt.Sprintf("Proposal %d", i+1))
		card.SetWordWrap(true)
		card.SetDynamicColors(true)

		cs.proposalCards[i] = card
		cs.proposalsPanel.AddItem(card, 0, 1, false)
	}
}

// updateProposalDisplay updates the display of all proposal cards
func (cs *ComparisonScreen) updateProposalDisplay() {
	if len(cs.currentProposals) == 0 {
		// Clear all cards
		for i, card := range cs.proposalCards {
			card.SetText(fmt.Sprintf("[dim]No proposal %d available[-]", i+1))
		}
		return
	}

	// Ensure we have the right number of proposal cards
	if len(cs.proposalCards) != len(cs.currentProposals) {
		cs.setupProposalCards(len(cs.currentProposals))
	}

	// Display each proposal in its corresponding card
	for i, proposal := range cs.currentProposals {
		if i < len(cs.proposalCards) {
			content := cs.formatProposalContent(proposal)
			cs.proposalCards[i].SetText(content)
		}
	}
}

// formatProposalContent creates formatted text for a proposal
func (cs *ComparisonScreen) formatProposalContent(proposal data.Proposal) string {
	var content strings.Builder

	// Title
	content.WriteString(fmt.Sprintf("[white::b]%s[white::-]\n\n", proposal.Title))

	// Speaker (if available)
	if proposal.Speaker != "" {
		content.WriteString(fmt.Sprintf("[yellow]Speaker:[-] %s\n\n", proposal.Speaker))
	}

	// Abstract (if available)
	if proposal.Abstract != "" {
		content.WriteString(fmt.Sprintf("[green]Abstract:[-]\n%s\n\n", proposal.Abstract))
	}

	// Current rating
	content.WriteString(fmt.Sprintf("[blue]Current Rating:[-] %.0f", proposal.Score))

	return content.String()
}

// GetPrimitive returns the main container primitive
func (cs *ComparisonScreen) GetPrimitive() tview.Primitive {
	return cs.container
}

// OnEnter is called when the screen becomes active
func (cs *ComparisonScreen) OnEnter(app any) error {
	cs.app = app

	// Set comparison method from config
	if appWithConfig, ok := app.(interface{ GetConfig() *data.SessionConfig }); ok {
		config := appWithConfig.GetConfig()
		if config != nil && config.UI.ComparisonMode != "" {
			switch config.UI.ComparisonMode {
			case "pairwise":
				cs.comparisonMethod = data.MethodPairwise
			case "trio":
				cs.comparisonMethod = data.MethodTrio
			case "quartet":
				cs.comparisonMethod = data.MethodQuartet
			default:
				cs.comparisonMethod = data.MethodPairwise // fallback
			}
		}
	}

	// Load proposals for comparison
	if err := cs.loadNextComparison(); err != nil {
		return fmt.Errorf("failed to load comparison: %w", err)
	}

	cs.updateDisplay()
	return nil
}

// OnExit is called when leaving the screen
func (cs *ComparisonScreen) OnExit(app any) error {
	// Save any pending comparison state if needed
	return nil
}

// GetTitle returns the screen title
func (cs *ComparisonScreen) GetTitle() string {
	return "Comparison"
}

// handleInput processes keyboard input for the comparison screen
func (cs *ComparisonScreen) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyDown:
		// Allow scrolling within proposal content
		return event
	}

	switch event.Rune() {
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case '1', '2', '3', '4':
		// For trio/quartet mode, automatically use ranking
		if cs.comparisonMethod == data.MethodTrio || cs.comparisonMethod == data.MethodQuartet {
			if !cs.isRanking {
				cs.startRanking()
			}
			cs.handleRankingInput(event.Rune())
			return nil
		}
		// For pairwise mode, check if we're in ranking mode first
		if cs.handleRankingInput(event.Rune()) {
			return nil
		}
		cs.selectWinner(string(event.Rune()))
		return nil
	case 's', 'n':
		cs.nextComparison()
		return nil
	case 'p':
		cs.setComparisonMode(data.MethodPairwise)
		return nil
	case 't':
		cs.setComparisonMode(data.MethodTrio)
		return nil
	case 'q':
		cs.setComparisonMode(data.MethodQuartet)
		return nil
	case '\r', '\n': // Enter key
		if cs.handleRankingInput(event.Rune()) {
			return nil
		}
	case 27: // Escape key
		if cs.handleRankingInput(event.Rune()) {
			return nil
		}
	}

	return event
}

// loadNextComparison loads the next set of proposals for comparison
func (cs *ComparisonScreen) loadNextComparison() error {
	session := cs.getSession()
	if session == nil {
		return fmt.Errorf("no active session")
	}

	// Check convergence criteria before loading next comparison
	config := cs.getConfig()
	if config != nil && config.Convergence.EnableEarlyStopping {
		completed := len(session.CompletedComparisons)

		// Check if we've reached minimum comparisons and convergence is achieved
		if completed >= config.Convergence.MinComparisons {
			if cs.checkTopTConvergence() {
				return fmt.Errorf("ranking has converged - top %d proposals are stable", config.Convergence.TargetAccepted)
			}
		}

		// Hard limit check
		if completed >= config.Convergence.MaxComparisons {
			return fmt.Errorf("maximum comparison limit reached (%d)", config.Convergence.MaxComparisons)
		}
	}

	proposals := session.Proposals
	if len(proposals) < 2 {
		return fmt.Errorf("not enough proposals for comparison")
	}

	// Select proposals based on comparison method
	count := 2
	switch cs.comparisonMethod {
	case data.MethodTrio:
		count = 3
	case data.MethodQuartet:
		count = 4
	}

	if len(proposals) < count {
		count = len(proposals)
		cs.comparisonMethod = data.MethodPairwise
	}

	// Find next uncompared pair/group of proposals
	nextProposals := cs.findNextComparison(proposals, count, session.CompletedComparisons)
	if nextProposals == nil {
		return fmt.Errorf("no more comparisons available")
	}

	cs.currentProposals = nextProposals

	// Update proposal display
	cs.updateProposalDisplay()
	cs.updateDisplay()

	return nil
}

// findNextComparison finds the next set of proposals that haven't been compared yet
func (cs *ComparisonScreen) findNextComparison(proposals []data.Proposal, count int, completed []data.Comparison) []data.Proposal {
	// Create a set of completed comparisons for quick lookup
	completedSet := make(map[string]bool)
	for _, comp := range completed {
		// Create a sorted key from proposal IDs
		key := cs.createComparisonKey(comp.ProposalIDs)
		completedSet[key] = true
	}

	// For pairwise comparisons, find next uncompleted pair
	if count == 2 {
		for i := 0; i < len(proposals); i++ {
			for j := i + 1; j < len(proposals); j++ {
				proposalIDs := []string{proposals[i].ID, proposals[j].ID}
				key := cs.createComparisonKey(proposalIDs)

				if !completedSet[key] {
					return []data.Proposal{proposals[i], proposals[j]}
				}
			}
		}
	}

	// For multi-way comparisons (trio, quartet), implement similar logic
	// For now, just use a simple round-robin approach
	if count > 2 {
		// Simple implementation: take next sequential group
		startIdx := len(completed) % (len(proposals) - count + 1)
		if startIdx+count <= len(proposals) {
			result := make([]data.Proposal, count)
			copy(result, proposals[startIdx:startIdx+count])
			return result
		}
	}

	return nil // No more comparisons available
}

// createComparisonKey creates a consistent key for comparison lookup
func (cs *ComparisonScreen) createComparisonKey(proposalIDs []string) string {
	// Sort IDs to ensure consistent key regardless of order
	sortedIDs := make([]string, len(proposalIDs))
	copy(sortedIDs, proposalIDs)

	// Simple sort for small arrays
	for i := 0; i < len(sortedIDs); i++ {
		for j := i + 1; j < len(sortedIDs); j++ {
			if sortedIDs[i] > sortedIDs[j] {
				sortedIDs[i], sortedIDs[j] = sortedIDs[j], sortedIDs[i]
			}
		}
	}

	return strings.Join(sortedIDs, ",")
}

// updateDisplay refreshes the UI state (carousel handles its own display)
func (cs *ComparisonScreen) updateDisplay() {
	// Update control panel components
	cs.updateInstructions()
	cs.updateProgress()
	cs.updateStatus()
}

// selectWinner handles winner selection in comparison
func (cs *ComparisonScreen) selectWinner(winner string) {
	winnerIndex, err := strconv.Atoi(winner)
	if err != nil || winnerIndex < 1 || winnerIndex > len(cs.currentProposals) {
		return // Invalid selection, ignore
	}

	cs.selectedWinner = cs.currentProposals[winnerIndex-1].ID

	// Execute comparison and update ratings
	if err := cs.executeComparison(); err != nil {
		// For now just skip to next comparison if there's an error
	}

	// Automatically load next comparison
	cs.nextComparison()
}

// startRanking initiates multi-way ranking mode
func (cs *ComparisonScreen) startRanking() {
	if len(cs.currentProposals) < 2 {
		return // Not enough proposals for ranking
	}

	cs.isRanking = true
	cs.currentRank = 1
	cs.proposalRanks = make(map[string]int)
	cs.rankings = make([]string, len(cs.currentProposals))

	cs.updateInstructions()
	cs.updateDisplay()
}

// handleRankingInput processes ranking input when in ranking mode
func (cs *ComparisonScreen) handleRankingInput(key rune) bool {
	if !cs.isRanking {
		return false
	}

	switch key {
	case '1', '2', '3', '4':
		// Numbers assign the current rank to a specific proposal
		proposalIndex, _ := strconv.Atoi(string(key))
		if proposalIndex >= 1 && proposalIndex <= len(cs.currentProposals) {
			cs.assignRankToProposal(proposalIndex-1, cs.currentRank)
		}
		return true
	case '\r', '\n': // Enter key to confirm ranking (only if all ranks assigned)
		if len(cs.proposalRanks) == len(cs.currentProposals) {
			cs.confirmRanking()
		}
		return true
	case 27: // Escape key to cancel ranking
		cs.cancelRanking()
		return true
	case 'u': // 'u' for undo last ranking assignment
		cs.undoLastRank()
		return true
	}

	return false
}

// confirmRanking finalizes the ranking and processes the multi-way comparison
func (cs *ComparisonScreen) confirmRanking() {
	if !cs.isRanking || len(cs.proposalRanks) != len(cs.currentProposals) {
		return
	}

	// Build final rankings array
	cs.buildRankingsArray()

	// Execute multi-way comparison based on ranking
	if err := cs.executeMultiWayComparison(); err != nil {
		// Handle error - for now just continue
	}

	cs.isRanking = false
	cs.rankings = nil
	cs.proposalRanks = nil
	cs.currentRank = 1
	cs.nextComparison()
}

// cancelRanking cancels the current ranking and returns to comparison mode
func (cs *ComparisonScreen) cancelRanking() {
	cs.isRanking = false
	cs.rankings = nil
	cs.proposalRanks = nil
	cs.currentRank = 1
	cs.updateInstructions()
	cs.updateDisplay()
}

// assignRankToProposal assigns the current rank to a specific proposal
func (cs *ComparisonScreen) assignRankToProposal(proposalIndex, rank int) {
	if proposalIndex < 0 || proposalIndex >= len(cs.currentProposals) {
		return
	}

	proposalID := cs.currentProposals[proposalIndex].ID

	// Check if this proposal already has a rank
	if existingRank, exists := cs.proposalRanks[proposalID]; exists {
		// Remove from existing rank and shift others down
		for id, r := range cs.proposalRanks {
			if r > existingRank {
				cs.proposalRanks[id] = r - 1
			}
		}
		cs.currentRank--
	}

	// Assign the current rank to this proposal
	cs.proposalRanks[proposalID] = rank

	// Advance to next rank if all current ranks are assigned
	if rank == cs.currentRank {
		cs.currentRank++
	}

	// Build rankings array
	cs.buildRankingsArray()
	cs.updateDisplay()
}

// undoLastRank removes the last assigned rank
func (cs *ComparisonScreen) undoLastRank() {
	if len(cs.proposalRanks) == 0 {
		return
	}

	// Find the highest rank and remove it
	highestRank := 0
	var proposalToRemove string

	for proposalID, rank := range cs.proposalRanks {
		if rank > highestRank {
			highestRank = rank
			proposalToRemove = proposalID
		}
	}

	if proposalToRemove != "" {
		delete(cs.proposalRanks, proposalToRemove)
		cs.currentRank = highestRank
		cs.buildRankingsArray()
		cs.updateDisplay()
	}
}

// buildRankingsArray constructs the rankings array from proposalRanks map
func (cs *ComparisonScreen) buildRankingsArray() {
	cs.rankings = make([]string, len(cs.currentProposals))

	// Fill rankings array based on assigned ranks
	for proposalID, rank := range cs.proposalRanks {
		if rank >= 1 && rank <= len(cs.rankings) {
			cs.rankings[rank-1] = proposalID
		}
	}
}

// executeMultiWayComparison processes a multi-way ranking result
func (cs *ComparisonScreen) executeMultiWayComparison() error {
	session := cs.getSession()
	if session == nil {
		return fmt.Errorf("no active session")
	}

	// For now, implement a simple approach:
	// Award points based on ranking position and update ratings accordingly
	rankPoints := make(map[string]float64)
	totalProposals := len(cs.rankings)

	for i, proposalID := range cs.rankings {
		// Higher rank (lower index) gets more points
		points := float64(totalProposals - i)
		rankPoints[proposalID] = points
	}

	// Update ratings based on relative performance
	avgPoints := float64(totalProposals+1) / 2.0
	for proposalID, points := range rankPoints {
		ratingChange := (points - avgPoints) * 8.0 // Scale factor for rating change

		// Find and update the proposal in session
		for i := range session.Proposals {
			if session.Proposals[i].ID == proposalID {
				session.Proposals[i].Score += ratingChange

				// Ensure rating stays within bounds
				if session.Proposals[i].Score < 0 {
					session.Proposals[i].Score = 0
				} else if session.Proposals[i].Score > 3000 {
					session.Proposals[i].Score = 3000
				}

				session.Proposals[i].UpdatedAt = time.Now()
				break
			}
		}
	}

	// Record the comparison
	comparison := data.Comparison{
		ID:          cs.generateComparisonID(),
		SessionID:   session.ID,
		ProposalIDs: cs.getProposalIDs(),
		WinnerID:    cs.rankings[0], // First in ranking is winner
		Rankings:    cs.rankings,
		Method:      cs.comparisonMethod,
		Timestamp:   time.Now(),
	}

	session.CompletedComparisons = append(session.CompletedComparisons, comparison)

	// Save session back to app
	if app, ok := cs.app.(interface{ SetSession(*data.Session) }); ok {
		app.SetSession(session)
	}

	return nil
}

// nextComparison loads the next comparison set
func (cs *ComparisonScreen) nextComparison() {
	cs.selectedWinner = ""
	cs.rankings = nil
	cs.isRanking = false

	if err := cs.loadNextComparison(); err != nil {
		// No more comparisons available - show completion message
		cs.showCompletionMessage()
		return
	}

	cs.updateDisplay()
}

// showCompletionMessage displays completion status and provides navigation options
func (cs *ComparisonScreen) showCompletionMessage() {
	session := cs.getSession()
	config := cs.getConfig()

	var completionTitle string
	var completionReason string

	if config != nil && config.Convergence.EnableEarlyStopping && cs.checkTopTConvergence() {
		completionTitle = "🎯 Ranking Converged!"
		completionReason = fmt.Sprintf("Top-%d proposals have stabilized.\nNo need for additional comparisons!", config.Convergence.TargetAccepted)
	} else {
		completionTitle = "🎉 All Comparisons Complete!"
		completionReason = "All possible comparisons have been finished."
	}

	// Show completion message in the first card (there should be at least one)
	if len(cs.proposalCards) > 0 {
		completionText := fmt.Sprintf("[green::b]%s[white::-]\n\n%s\n\n[yellow]Options:[-]\n  [blue]Ctrl+R[-] - View Rankings\n  [blue]Esc[-] - Return to main menu\n  [blue]Ctrl+C[-] - Exit application",
			completionTitle, completionReason)
		cs.proposalCards[0].SetText(completionText)
	}

	// Show session statistics in the second card if available
	if len(cs.proposalCards) > 1 && session != nil {
		totalComparisons := len(session.CompletedComparisons)
		totalProposals := len(session.Proposals)

		statsText := fmt.Sprintf("[blue::b]Session Statistics[white::-]\n\n[yellow]Comparisons:[-] %d\n[yellow]Proposals:[-] %d\n[yellow]Mode:[-] %s",
			totalComparisons, totalProposals, cs.comparisonMethod)

		if config != nil && config.Convergence.EnableEarlyStopping {
			statsText += fmt.Sprintf("\n[yellow]Target Accepted:[-] %d", config.Convergence.TargetAccepted)
		}

		statsText += "\n\n[green]Ready for export and analysis![-]"
		cs.proposalCards[1].SetText(statsText)
	}

	// Clear remaining cards if any
	for i := 2; i < len(cs.proposalCards); i++ {
		cs.proposalCards[i].SetText("")
	}
}

// setComparisonMode changes the comparison method
func (cs *ComparisonScreen) setComparisonMode(method data.ComparisonMethod) {
	cs.comparisonMethod = method
	cs.loadNextComparison()
	cs.updateDisplay()
}

// executeComparison processes the comparison and updates ratings
func (cs *ComparisonScreen) executeComparison() error {
	session := cs.getSession()
	if session == nil {
		return fmt.Errorf("no active session")
	}

	// Create a simple Elo engine for basic calculations
	engine := &elo.Engine{
		InitialRating: 1500.0,
		KFactor:       32,
		MinRating:     0.0,
		MaxRating:     3000.0,
	}

	startTime := time.Now()

	// For pairwise comparison, just do a simple rating swap
	if cs.comparisonMethod == data.MethodPairwise && len(cs.currentProposals) == 2 {
		winnerIdx := 0
		loserIdx := 1
		if cs.selectedWinner == cs.currentProposals[1].ID {
			winnerIdx = 1
			loserIdx = 0
		}

		winner := elo.Rating{
			ID:    cs.currentProposals[winnerIdx].ID,
			Score: cs.currentProposals[winnerIdx].Score,
		}
		loser := elo.Rating{
			ID:    cs.currentProposals[loserIdx].ID,
			Score: cs.currentProposals[loserIdx].Score,
		}

		newWinner, newLoser, err := engine.CalculatePairwise(winner, loser)
		if err != nil {
			return err
		}

		// Update session with new ratings
		for i := range session.Proposals {
			switch session.Proposals[i].ID {
			case newWinner.ID:
				session.Proposals[i].Score = newWinner.Score
				session.Proposals[i].UpdatedAt = time.Now()
			case newLoser.ID:
				session.Proposals[i].Score = newLoser.Score
				session.Proposals[i].UpdatedAt = time.Now()
			}
		}
	}

	// Record completed comparison
	comparison := data.Comparison{
		ID:          cs.generateComparisonID(),
		SessionID:   session.ID,
		ProposalIDs: cs.getProposalIDs(),
		WinnerID:    cs.selectedWinner,
		Rankings:    cs.rankings,
		Method:      cs.comparisonMethod,
		Timestamp:   time.Now(),
		Duration:    time.Since(startTime),
	}

	session.CompletedComparisons = append(session.CompletedComparisons, comparison)

	// Save session back to app if possible
	if app, ok := cs.app.(interface{ SetSession(*data.Session) }); ok {
		app.SetSession(session)
	}

	return nil
}

// Helper methods

// getSession gets the current session from the app
func (cs *ComparisonScreen) getSession() *data.Session {
	if cs.app == nil {
		return nil
	}

	// Use type assertion to get the app and call GetSession
	if app, ok := cs.app.(interface{ GetSession() *data.Session }); ok {
		return app.GetSession()
	}

	// Fallback: return a dummy session for testing
	return &data.Session{
		ID:   "test",
		Name: "Test Session",
		Proposals: []data.Proposal{
			{
				ID:      "prop1",
				Title:   "Example Proposal 1",
				Speaker: "Speaker 1",
				Score:   1500.0,
			},
			{
				ID:      "prop2",
				Title:   "Example Proposal 2",
				Speaker: "Speaker 2",
				Score:   1500.0,
			},
		},
	}
}

// getConfig gets the current configuration from the app
func (cs *ComparisonScreen) getConfig() *data.SessionConfig {
	if cs.app == nil {
		return nil
	}

	// Use type assertion to get the app and call GetConfig
	if app, ok := cs.app.(interface{ GetConfig() *data.SessionConfig }); ok {
		return app.GetConfig()
	}

	return nil
}

// checkTopTConvergence checks if the top-T proposals have stabilized
func (cs *ComparisonScreen) checkTopTConvergence() bool {
	session := cs.getSession()
	config := cs.getConfig()

	if session == nil || config == nil || !config.Convergence.EnableEarlyStopping {
		return false
	}

	// Need minimum comparisons before checking convergence
	if len(session.CompletedComparisons) < config.Convergence.MinComparisons {
		return false
	}

	// Get current proposals and sort by score to get rankings
	proposals := session.GetProposals()
	if len(proposals) < config.Convergence.TargetAccepted {
		return false
	}

	// Sort proposals by score (descending) to get current rankings
	sortedProposals := make([]data.Proposal, len(proposals))
	copy(sortedProposals, proposals)

	// Sort by score - higher scores rank better
	for i := 0; i < len(sortedProposals)-1; i++ {
		for j := i + 1; j < len(sortedProposals); j++ {
			if sortedProposals[j].Score > sortedProposals[i].Score {
				sortedProposals[i], sortedProposals[j] = sortedProposals[j], sortedProposals[i]
			}
		}
	}

	// Get top-T proposals
	topT := make([]string, config.Convergence.TargetAccepted)
	for i := 0; i < config.Convergence.TargetAccepted; i++ {
		topT[i] = sortedProposals[i].ID
	}

	// Simple convergence check: if we have enough comparisons and clear rating separation
	totalComparisons := len(session.CompletedComparisons)

	// Enhanced convergence criteria for better confidence:
	// 1. We have at least 2x the minimum comparisons
	// 2. The top-T proposals have a clear rating gap from the rest
	// 3. Each top-T proposal has participated in enough individual comparisons for confidence
	if totalComparisons >= config.Convergence.MinComparisons*2 {
		// Check rating gap between T-th and (T+1)-th proposal
		if config.Convergence.TargetAccepted < len(sortedProposals) {
			topTScore := sortedProposals[config.Convergence.TargetAccepted-1].Score
			nextScore := sortedProposals[config.Convergence.TargetAccepted].Score
			ratingGap := topTScore - nextScore

			// Require significant rating gap
			if ratingGap > config.Convergence.StabilityThreshold {
				// Additional check: ensure top-T proposals have enough individual comparisons
				// This ensures confidence levels are reasonable (aim for ~70%+ confidence)
				// With trio mode, we want each top proposal to participate in 4-5 comparisons
				// Using logarithmic confidence: 100 * (1 - e^(-count/5))
				// 4 comparisons → ~55% confidence, 5 comparisons → ~63% confidence
				minIndividualComparisons := 4 // Increased for better confidence levels
				if cs.comparisonMethod == data.MethodQuartet {
					minIndividualComparisons = 3 // Quartet is more efficient, can use lower minimum
				}
				for i := 0; i < config.Convergence.TargetAccepted; i++ {
					proposalID := sortedProposals[i].ID
					individualCount := cs.getProposalComparisonCount(proposalID, session)
					if individualCount < minIndividualComparisons {
						// Top proposal doesn't have enough individual comparisons yet
						// This ensures confidence will be ≥55% (trio) or ≥45% (quartet)
						return false
					}
				}
				// All conditions met: rating gap + individual participation
				return true
			}
		}
	}

	// Alternative: if we've done many comparisons, assume convergence
	theoreticalMax := len(proposals) * (len(proposals) - 1) / 2
	switch cs.comparisonMethod {
	case data.MethodTrio:
		theoreticalMax = (theoreticalMax + 2) / 3
	case data.MethodQuartet:
		theoreticalMax = (theoreticalMax + 5) / 6
	}

	// If we've done more than 80% of theoretical max, consider converged
	if totalComparisons > int(float64(theoreticalMax)*0.8) {
		return true
	}

	return false
}

// getProposalComparisonCount counts how many comparisons a proposal has participated in
func (cs *ComparisonScreen) getProposalComparisonCount(proposalID string, session *data.Session) int {
	if session == nil {
		return 0
	}

	count := 0
	for _, comparison := range session.CompletedComparisons {
		// Check if this proposal was involved in this comparison
		for _, id := range comparison.ProposalIDs {
			if id == proposalID {
				count++
				break // Only count once per comparison
			}
		}
	}

	return count
}

// generateComparisonID creates a unique ID for a comparison
func (cs *ComparisonScreen) generateComparisonID() string {
	return fmt.Sprintf("comp_%d", time.Now().UnixNano())
}

// getProposalIDs returns the IDs of current proposals
func (cs *ComparisonScreen) getProposalIDs() []string {
	ids := make([]string, len(cs.currentProposals))
	for i, proposal := range cs.currentProposals {
		ids[i] = proposal.ID
	}
	return ids
}

// updateInstructions updates the control panel with current instructions
func (cs *ComparisonScreen) updateInstructions() {
	var instructions strings.Builder

	instructions.WriteString("[yellow]Current Mode:[-] ")
	instructions.WriteString(string(cs.comparisonMethod))
	instructions.WriteString("\n\n")

	if cs.isRanking {
		instructions.WriteString(fmt.Sprintf("[green]Ranking Mode: Assigning Rank %d[-]\n", cs.currentRank))
		instructions.WriteString("Press the number of the proposal to assign this rank:\n")
		for i := range cs.currentProposals {
			// Show which proposals already have ranks
			proposalID := cs.currentProposals[i].ID
			if rank, hasRank := cs.proposalRanks[proposalID]; hasRank {
				instructions.WriteString(fmt.Sprintf("  %d - Proposal %d [dim](Rank %d)[-]\n", i+1, i+1, rank))
			} else {
				instructions.WriteString(fmt.Sprintf("  %d - Proposal %d\n", i+1, i+1))
			}
		}
		instructions.WriteString("\n[yellow]u[-] - Undo last | [yellow]Enter[-] - Confirm (when all ranked)")
	} else {
		// Different instructions based on comparison method
		if cs.comparisonMethod == data.MethodTrio || cs.comparisonMethod == data.MethodQuartet {
			instructions.WriteString("[white]Rank all proposals from best (1) to worst:[-]\n")
			for i := range cs.currentProposals {
				instructions.WriteString(fmt.Sprintf("  %d - Proposal %d\n", i+1, i+1))
			}
		} else {
			instructions.WriteString("[white]Select the best proposal:[-]\n")
			for i := range cs.currentProposals {
				instructions.WriteString(fmt.Sprintf("  %d - Proposal %d\n", i+1, i+1))
			}
			instructions.WriteString("\n[blue]Or press 'r' to rank all[-]")
		}
	}

	cs.controlPanel.SetText(instructions.String())
}

// calculateExpectedComparisons estimates realistic number of comparisons needed for convergence
func (cs *ComparisonScreen) calculateExpectedComparisons(totalProposals int, config *data.SessionConfig) int {
	if config == nil {
		return totalProposals * 2 // Fallback: ~2 comparisons per proposal
	}

	// Calculate based on minimum comparisons per proposal and comparison method efficiency
	minPerProposal := cs.calculateMinComparisonsForConfidence(totalProposals)

	// Estimate total comparisons based on method
	switch cs.comparisonMethod {
	case data.MethodTrio:
		// Trio covers 3 pairwise games per comparison
		// Need minPerProposal comparisons per proposal, but trios cover 3 proposals each
		return (totalProposals * minPerProposal) / 2 // Divide by 2 for overlap efficiency
	case data.MethodQuartet:
		// Quartet covers 6 pairwise games per comparison
		// Most efficient method
		return (totalProposals * minPerProposal) / 3 // Divide by 3 for higher overlap
	default:
		// Pairwise: need minPerProposal comparisons per proposal
		return totalProposals * minPerProposal / 2 // Divide by 2 since each comparison involves 2 proposals
	}
}

// calculateMinComparisonsForConfidence determines minimum comparisons per proposal for good confidence
func (cs *ComparisonScreen) calculateMinComparisonsForConfidence(totalProposals int) int {
	// For small datasets, we need relatively more comparisons for confidence
	// For large datasets, we can get by with fewer per proposal

	switch {
	case totalProposals <= 5:
		// Very small dataset: need each proposal compared multiple times
		switch cs.comparisonMethod {
		case data.MethodQuartet:
			return 2 // 2 quartets means ~4 pairwise comparisons
		case data.MethodTrio:
			return 3 // 3 trios means ~4.5 pairwise comparisons
		default:
			return 4 // 4 pairwise comparisons
		}
	case totalProposals <= 20:
		// Small dataset: moderate number of comparisons
		switch cs.comparisonMethod {
		case data.MethodQuartet:
			return 3
		case data.MethodTrio:
			return 4
		default:
			return 5
		}
	case totalProposals <= 50:
		// Medium dataset
		switch cs.comparisonMethod {
		case data.MethodQuartet:
			return 4
		case data.MethodTrio:
			return 5
		default:
			return 6
		}
	default:
		// Large dataset: can use fewer per proposal
		switch cs.comparisonMethod {
		case data.MethodQuartet:
			return 5
		case data.MethodTrio:
			return 6
		default:
			return 8
		}
	}
}

// updateProgress updates the progress display with convergence-aware metrics
func (cs *ComparisonScreen) updateProgress() {
	session := cs.getSession()
	if session == nil {
		return
	}

	completed := len(session.CompletedComparisons)
	totalProposals := len(session.Proposals)

	// Get convergence configuration
	config := cs.getConfig()
	var convergenceStatus string

	// Check if we have convergence configuration and early stopping is enabled
	if config.Convergence.EnableEarlyStopping {
		// Check top-T convergence instead of all comparisons
		if cs.checkTopTConvergence() {
			convergenceStatus = fmt.Sprintf(" [green]Top-%d Stable![-]", config.Convergence.TargetAccepted)
		}
	}

	// Use convergence-aware display
	var progress string
	if config.Convergence.EnableEarlyStopping {
		// Calculate realistic expected comparisons based on dataset size and method
		// This gives a much better progress estimate than using MaxComparisons limit
		expectedComparisons := cs.calculateExpectedComparisons(totalProposals, config)

		// Progress toward expected convergence point (not hard max limit)
		convergencePercent := float64(completed) / float64(expectedComparisons) * 100
		if convergencePercent > 100 {
			convergencePercent = 100
		}

		// Calculate stability: what % of ALL proposals (not just top-T) meet criteria
		stabilityProgress := 0.0
		if completed >= config.Convergence.MinComparisons {
			// Count how many proposals meet the stability criteria
			sortedProposals := make([]data.Proposal, len(session.Proposals))
			copy(sortedProposals, session.Proposals)
			sort.Slice(sortedProposals, func(i, j int) bool {
				return sortedProposals[i].Score > sortedProposals[j].Score
			})

			stableCount := 0
			// Use the smaller of TargetAccepted or actual proposal count
			// This prevents showing 67% when we only have 3 proposals
			targetTop := min(config.Convergence.TargetAccepted, len(sortedProposals))

			// Adjust minimum comparisons based on dataset size and method
			minIndividualComparisons := cs.calculateMinComparisonsForConfidence(totalProposals)

			for i := range targetTop {
				proposalID := sortedProposals[i].ID
				if cs.getProposalComparisonCount(proposalID, session) >= minIndividualComparisons {
					stableCount++
				}
			}

			stabilityProgress = float64(stableCount) / float64(targetTop) * 100
		}

		progress = fmt.Sprintf("Moves: %d\nProgress: %.0f%%\nStability: %.0f%%%s",
			completed, convergencePercent, stabilityProgress, convergenceStatus)
	} else {
		// Traditional completion percentage - calculate theoretical maximum
		var maxComparisons int
		switch cs.comparisonMethod {
		case data.MethodPairwise:
			maxComparisons = totalProposals * (totalProposals - 1) / 2
		case data.MethodTrio:
			totalPairwise := totalProposals * (totalProposals - 1) / 2
			maxComparisons = (totalPairwise + 2) / 3 // Round up division
		case data.MethodQuartet:
			totalPairwise := totalProposals * (totalProposals - 1) / 2
			maxComparisons = (totalPairwise + 5) / 6 // Round up division
		default:
			maxComparisons = totalProposals * (totalProposals - 1) / 2
		}

		percentage := float64(completed) / float64(maxComparisons) * 100
		if percentage > 100 {
			percentage = 100
		}
		progress = fmt.Sprintf("Comparisons: %d/%d (%.1f%%)",
			completed, maxComparisons, percentage)
	}

	cs.progressBar.SetText(progress)
}

// updateStatus updates the status display
func (cs *ComparisonScreen) updateStatus() {
	session := cs.getSession()
	totalProposals := 0
	if session != nil {
		totalProposals = len(session.Proposals)
	}

	// With side-by-side display, show both proposals in status
	status := fmt.Sprintf("Proposals: %d | Current: %d",
		totalProposals, len(cs.currentProposals))

	if cs.selectedWinner != "" {
		status += " | Winner selected"
	}

	cs.statusBar.SetText(status)
}
