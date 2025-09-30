// Package screens provides TUI screen implementations for conference talk ranking.
// This file implements the comparison screen interface where users perform
// pairwise and multi-way comparisons between proposals.
package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/pashagolub/confelo/pkg/data"
	"github.com/pashagolub/confelo/pkg/elo"
	"github.com/pashagolub/confelo/pkg/tui/components"
)

// ComparisonScreen implements the comparison interface for ranking proposals
type ComparisonScreen struct {
	// UI components
	container        *tview.Flex
	leftPanel        *tview.Flex
	rightPanel       *tview.Flex
	proposalCarousel *components.Carousel
	controlPanel     *tview.TextView
	progressBar      *tview.TextView
	statusBar        *tview.TextView

	// Comparison state
	currentProposals []data.Proposal
	comparisonMethod data.ComparisonMethod
	selectedWinner   string
	rankings         []string
	isRanking        bool

	// App reference - we'll use interface{} and cast as needed
	app interface{}
}

// NewComparisonScreen creates a new comparison screen instance
func NewComparisonScreen() *ComparisonScreen {
	cs := &ComparisonScreen{
		container:        tview.NewFlex(),
		leftPanel:        tview.NewFlex(),
		rightPanel:       tview.NewFlex(),
		proposalCarousel: components.NewCarousel(),
		controlPanel:     tview.NewTextView(),
		progressBar:      tview.NewTextView(),
		statusBar:        tview.NewTextView(),
		comparisonMethod: data.MethodPairwise,
	}

	cs.setupUI()
	cs.setupCarouselCallbacks()
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

	// Configure control panel - use TextView methods correctly
	cs.controlPanel.
		SetBorder(true).
		SetTitle("Instructions")
	cs.controlPanel.SetWordWrap(true)

	// Configure progress bar
	cs.progressBar.
		SetBorder(true).
		SetTitle("Progress")

	// Configure status bar
	cs.statusBar.
		SetBorder(true).
		SetTitle("Status")

	// Layout left panel: proposal carousel takes most space
	cs.leftPanel.AddItem(cs.proposalCarousel.GetPrimitive(), 0, 1, false)

	// Layout right panel: controls, progress, status
	cs.rightPanel.
		AddItem(cs.controlPanel, 0, 2, false).
		AddItem(cs.progressBar, 6, 0, false).
		AddItem(cs.statusBar, 3, 0, false)

	// Main container: left panel (70%) and right panel (30%)
	cs.container.
		AddItem(cs.leftPanel, 0, 7, true).
		AddItem(cs.rightPanel, 0, 3, false)

	// Set up input handling
	cs.container.SetInputCapture(cs.handleInput)

	// Initialize with default instructions
	cs.updateInstructions()
}

// setupCarouselCallbacks configures callbacks for the proposal carousel
func (cs *ComparisonScreen) setupCarouselCallbacks() {
	// Navigation callback to update display when user navigates
	cs.proposalCarousel.SetOnNavigate(func(index int, proposal data.Proposal) {
		cs.updateDisplay()
	})

	// Selection callback for proposal selection
	cs.proposalCarousel.SetOnSelect(func(index int, proposal data.Proposal) {
		// In comparison mode, selection could trigger comparison selection
		cs.selectProposalForComparison(index + 1) // Convert to 1-based index
	})
}

// GetPrimitive returns the main container primitive
func (cs *ComparisonScreen) GetPrimitive() tview.Primitive {
	return cs.container
}

// OnEnter is called when the screen becomes active
func (cs *ComparisonScreen) OnEnter(app interface{}) error {
	cs.app = app

	// Load proposals for comparison
	if err := cs.loadNextComparison(); err != nil {
		return fmt.Errorf("failed to load comparison: %w", err)
	}

	cs.updateDisplay()
	return nil
}

// OnExit is called when leaving the screen
func (cs *ComparisonScreen) OnExit(app interface{}) error {
	// Save any pending comparison state if needed
	return nil
}

// GetTitle returns the screen title
func (cs *ComparisonScreen) GetTitle() string {
	return "Comparison"
}

// GetHelpText returns help text specific to this screen
func (cs *ComparisonScreen) GetHelpText() []string {
	return []string{
		"Navigation:",
		"  ← → / h l    Navigate between proposals",
		"  ↑ ↓ / j k    Scroll proposal content",
		"",
		"Comparison:",
		"  1-4          Select winner (pairwise: 1-2, trio: 1-3, quartet: 1-4)",
		"  r            Rank all proposals (drag-drop style)",
		"  s            Skip this comparison",
		"  n            Next comparison",
		"",
		"Mode:",
		"  p            Switch to pairwise mode",
		"  t            Switch to trio mode",
		"  q            Switch to quartet mode",
		"",
		"Navigation:",
		"  Tab          Go to ranking screen",
		"  Esc          Go back to setup",
		"  F1/?         Show help",
	}
}

// handleInput processes keyboard input for the comparison screen
func (cs *ComparisonScreen) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyLeft, tcell.KeyRight:
		cs.navigateProposals(event.Key() == tcell.KeyRight)
		return nil
	case tcell.KeyUp, tcell.KeyDown:
		// Allow scrolling within proposal content
		return event
	case tcell.KeyTab:
		// Switch to ranking screen - implement later when navigation is available
		return nil
	}

	switch event.Rune() {
	case 'h':
		cs.navigateProposals(false) // left
		return nil
	case 'l':
		cs.navigateProposals(true) // right
		return nil
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case '1', '2', '3', '4':
		cs.selectWinner(string(event.Rune()))
		return nil
	case 'r':
		cs.startRanking()
		return nil
	case 's':
		cs.skipComparison()
		return nil
	case 'n':
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
	}

	return event
}

// loadNextComparison loads the next set of proposals for comparison
func (cs *ComparisonScreen) loadNextComparison() error {
	session := cs.getSession()
	if session == nil {
		return fmt.Errorf("no active session")
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

	// Simple selection for now - take first N proposals
	cs.currentProposals = make([]data.Proposal, count)
	copy(cs.currentProposals, proposals[:count])

	// Load proposals into the carousel
	cs.proposalCarousel.SetProposals(cs.currentProposals)

	return nil
}

// updateDisplay refreshes the UI state (carousel handles its own display)
func (cs *ComparisonScreen) updateDisplay() {
	// Update control panel components
	cs.updateInstructions()
	cs.updateProgress()
	cs.updateStatus()
}

// selectProposalForComparison handles proposal selection in comparison mode
func (cs *ComparisonScreen) selectProposalForComparison(number int) {
	if number < 1 || number > len(cs.currentProposals) {
		return
	}

	cs.selectedWinner = cs.currentProposals[number-1].ID
	cs.updateDisplay()
}

// navigateProposals handles left/right navigation between proposals
func (cs *ComparisonScreen) navigateProposals(right bool) {
	if right {
		cs.proposalCarousel.Next()
	} else {
		cs.proposalCarousel.Previous()
	}
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
	if len(cs.currentProposals) < 3 {
		return // Not enough proposals for ranking
	}

	cs.isRanking = true
	cs.rankings = make([]string, len(cs.currentProposals))

	// Initialize with current order
	for i, proposal := range cs.currentProposals {
		cs.rankings[i] = proposal.ID
	}

	cs.updateInstructions()
}

// skipComparison skips the current comparison
func (cs *ComparisonScreen) skipComparison() {
	cs.nextComparison()
}

// nextComparison loads the next comparison set
func (cs *ComparisonScreen) nextComparison() {
	cs.selectedWinner = ""
	cs.rankings = nil
	cs.isRanking = false

	if err := cs.loadNextComparison(); err != nil {
		// No more comparisons available
		return
	}

	cs.updateDisplay()
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
			if session.Proposals[i].ID == newWinner.ID {
				session.Proposals[i].Score = newWinner.Score
				session.Proposals[i].UpdatedAt = time.Now()
			} else if session.Proposals[i].ID == newLoser.ID {
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

	instructions.WriteString("[yellow::b]Current Mode:[-] ")
	instructions.WriteString(string(cs.comparisonMethod))
	instructions.WriteString("\n\n")

	if cs.isRanking {
		instructions.WriteString("[green]Ranking Mode Active[-]\n")
		instructions.WriteString("Set the order from best (1) to worst\n")
		instructions.WriteString("Press Enter when done\n")
	} else {
		instructions.WriteString("[white]Select the best proposal:[-]\n")
		for i := range cs.currentProposals {
			instructions.WriteString(fmt.Sprintf("  %d - Proposal %d\n", i+1, i+1))
		}
		instructions.WriteString("\n[blue]Or press 'r' to rank all[-]")
	}

	cs.controlPanel.SetText(instructions.String())
}

// updateProgress updates the progress display
func (cs *ComparisonScreen) updateProgress() {
	session := cs.getSession()
	if session == nil {
		return
	}

	completed := len(session.CompletedComparisons)
	// Rough estimate of total comparisons needed
	totalProposals := len(session.Proposals)
	total := totalProposals * (totalProposals - 1) / 2 // All pairs

	progress := fmt.Sprintf("Comparisons: %d/%d (%.1f%%)",
		completed, total, float64(completed)/float64(total)*100)

	cs.progressBar.SetText(progress)
}

// updateStatus updates the status display
func (cs *ComparisonScreen) updateStatus() {
	session := cs.getSession()
	totalProposals := 0
	if session != nil {
		totalProposals = len(session.Proposals)
	}

	currentIndex := cs.proposalCarousel.GetCurrentIndex()
	status := fmt.Sprintf("Proposals: %d | Current: %d/%d",
		totalProposals, currentIndex+1, len(cs.currentProposals))

	if cs.selectedWinner != "" {
		status += " | Winner selected"
	}

	cs.statusBar.SetText(status)
}
