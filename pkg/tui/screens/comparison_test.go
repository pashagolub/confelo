// Package screens provides TUI screen implementations for conference talk ranking.
// This file contains unit tests for the comparison screen functionality.
package screens

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"

	"github.com/pashagolub/confelo/pkg/data"
)

// MockApp implements the app interface for testing
type MockApp struct {
	session *data.Session
	calls   []string
}

func (m *MockApp) GetSession() *data.Session {
	m.calls = append(m.calls, "GetSession")
	return m.session
}

func (m *MockApp) SetSession(session *data.Session) {
	m.calls = append(m.calls, "SetSession")
	m.session = session
}

func (m *MockApp) NavigateTo(screen int) error {
	m.calls = append(m.calls, "NavigateTo")
	return nil
}

// Test data setup
func createTestSession() *data.Session {
	return &data.Session{
		Name: "test-session",
		Proposals: []data.Proposal{
			{
				ID:       "prop1",
				Title:    "Implementing Microservices with Go",
				Speaker:  "John Doe",
				Abstract: "A comprehensive guide to building scalable microservices using Go programming language.",
				Score:    1500.0,
			},
			{
				ID:       "prop2",
				Title:    "Advanced React Patterns",
				Speaker:  "Jane Smith",
				Abstract: "Exploring advanced React patterns for building maintainable applications.",
				Score:    1520.0,
			},
			{
				ID:       "prop3",
				Title:    "Database Optimization Strategies",
				Speaker:  "Bob Wilson",
				Abstract: "Techniques for optimizing database performance in high-load applications.",
				Score:    1480.0,
			},
			{
				ID:       "prop4",
				Title:    "DevOps Best Practices",
				Speaker:  "Alice Johnson",
				Abstract: "Essential DevOps practices for modern software development teams.",
				Score:    1510.0,
			},
		},
		CompletedComparisons: []data.Comparison{},
	}
}

func TestNewComparisonScreen(t *testing.T) {
	screen := NewComparisonScreen()

	assert.NotNil(t, screen)
	assert.NotNil(t, screen.container)
	assert.NotNil(t, screen.leftPanel)
	assert.NotNil(t, screen.rightPanel)
	assert.NotNil(t, screen.proposalsPanel)
	assert.NotNil(t, screen.controlPanel)
	assert.NotNil(t, screen.progressBar)
	assert.NotNil(t, screen.statusBar)
	assert.Equal(t, data.MethodPairwise, screen.comparisonMethod)
}

func TestComparisonScreen_GetPrimitive(t *testing.T) {
	screen := NewComparisonScreen()
	primitive := screen.GetPrimitive()

	assert.NotNil(t, primitive)
	assert.IsType(t, (*tview.Flex)(nil), primitive)
	assert.Equal(t, screen.container, primitive)
}

func TestComparisonScreen_GetTitle(t *testing.T) {
	screen := NewComparisonScreen()
	title := screen.GetTitle()

	assert.Equal(t, "Comparison", title)
}

func TestComparisonScreen_OnEnter(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}

	err := screen.OnEnter(mockApp)

	assert.NoError(t, err)
	assert.Equal(t, mockApp, screen.app)
	assert.NotEmpty(t, screen.currentProposals)
	assert.Equal(t, 2, len(screen.currentProposals)) // Pairwise by default
	assert.Contains(t, mockApp.calls, "GetSession")
}

func TestComparisonScreen_OnEnterWithoutSession(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: nil}

	err := screen.OnEnter(mockApp)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

func TestComparisonScreen_OnEnterInsufficientProposals(t *testing.T) {
	screen := NewComparisonScreen()
	session := &data.Session{
		Name:      "test",
		Proposals: []data.Proposal{{ID: "single", Title: "Only One", Score: 1500}},
	}
	mockApp := &MockApp{session: session}

	err := screen.OnEnter(mockApp)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough proposals")
}

func TestComparisonScreen_OnExit(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}

	err := screen.OnExit(mockApp)

	assert.NoError(t, err)
}

func TestComparisonScreen_SetComparisonMode(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	// Test trio mode
	screen.setComparisonMode(data.MethodTrio)
	assert.Equal(t, data.MethodTrio, screen.comparisonMethod)
	assert.Equal(t, 3, len(screen.currentProposals))

	// Test quartet mode
	screen.setComparisonMode(data.MethodQuartet)
	assert.Equal(t, data.MethodQuartet, screen.comparisonMethod)
	assert.Equal(t, 4, len(screen.currentProposals))

	// Test back to pairwise
	screen.setComparisonMode(data.MethodPairwise)
	assert.Equal(t, data.MethodPairwise, screen.comparisonMethod)
	assert.Equal(t, 2, len(screen.currentProposals))
}

func TestComparisonScreen_SetComparisonModeInsufficientProposals(t *testing.T) {
	screen := NewComparisonScreen()
	session := &data.Session{
		Name: "test",
		Proposals: []data.Proposal{
			{ID: "prop1", Title: "First", Score: 1500},
			{ID: "prop2", Title: "Second", Score: 1500},
		},
	}
	mockApp := &MockApp{session: session}
	screen.OnEnter(mockApp)

	// Try to set trio mode with only 2 proposals
	screen.setComparisonMode(data.MethodTrio)
	// Should fallback to pairwise
	assert.Equal(t, data.MethodPairwise, screen.comparisonMethod)
	assert.Equal(t, 2, len(screen.currentProposals))
}

func TestComparisonScreen_SelectWinner(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	initialComparisons := len(mockApp.session.CompletedComparisons)

	// Select first proposal as winner
	screen.selectWinner("1")

	// Should have recorded a comparison
	assert.Greater(t, len(mockApp.session.CompletedComparisons), initialComparisons)
	assert.Contains(t, mockApp.calls, "SetSession")
}

func TestComparisonScreen_SelectWinnerInvalidInput(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	initialComparisons := len(mockApp.session.CompletedComparisons)

	// Select invalid winner
	screen.selectWinner("0") // Invalid
	screen.selectWinner("5") // Out of range
	screen.selectWinner("a") // Non-numeric

	// Should not have recorded any comparisons
	assert.Equal(t, initialComparisons, len(mockApp.session.CompletedComparisons))
}

func TestComparisonScreen_StartRanking(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)
	screen.setComparisonMode(data.MethodTrio) // Need at least 3 proposals

	screen.startRanking()

	assert.True(t, screen.isRanking)
	assert.Equal(t, len(screen.currentProposals), len(screen.rankings))
	assert.NotEmpty(t, screen.rankings)
}

func TestComparisonScreen_StartRankingInsufficientProposals(t *testing.T) {
	screen := NewComparisonScreen()
	screen.currentProposals = []data.Proposal{{ID: "single", Title: "Only One", Score: 1500}}

	screen.startRanking()

	// Should not enter ranking mode with only 1 proposal
	assert.False(t, screen.isRanking)
	assert.Empty(t, screen.rankings)
}

func TestComparisonScreen_NextComparison(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	// Set some state
	screen.selectedWinner = "prop1"
	screen.isRanking = true
	screen.rankings = []string{"prop1", "prop2"}

	screen.nextComparison()

	// Should reset state
	assert.Equal(t, "", screen.selectedWinner)
	assert.False(t, screen.isRanking)
	assert.Empty(t, screen.rankings)
}

func TestComparisonScreen_ExecuteComparison(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)
	screen.selectedWinner = screen.currentProposals[0].ID

	err := screen.executeComparison()

	assert.NoError(t, err)
	assert.NotEmpty(t, mockApp.session.CompletedComparisons)

	// Check that a comparison was recorded
	lastComparison := mockApp.session.CompletedComparisons[len(mockApp.session.CompletedComparisons)-1]
	assert.Equal(t, screen.selectedWinner, lastComparison.WinnerID)
	assert.Equal(t, data.MethodPairwise, lastComparison.Method)
	assert.NotEmpty(t, lastComparison.ProposalIDs)
	assert.NotZero(t, lastComparison.Timestamp)
	assert.Contains(t, mockApp.calls, "SetSession")
}

func TestComparisonScreen_ExecuteComparisonNoSession(t *testing.T) {
	screen := NewComparisonScreen()
	// No app set
	screen.app = nil

	err := screen.executeComparison()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

func TestComparisonScreen_HandleInputNavigation(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	// Test left arrow key
	event := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	result := screen.handleInput(event)
	assert.Nil(t, result) // Should consume the event

	// Test right arrow key
	event = tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	result = screen.handleInput(event)
	assert.Nil(t, result)

	// Test h key (left)
	event = tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone)
	result = screen.handleInput(event)
	assert.Nil(t, result)

	// Test l key (right)
	event = tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone)
	result = screen.handleInput(event)
	assert.Nil(t, result)
}

func TestComparisonScreen_HandleInputWinnerSelection(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	initialComparisons := len(mockApp.session.CompletedComparisons)

	// Test winner selection
	event := tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone)
	result := screen.handleInput(event)
	assert.Nil(t, result)

	// Should have executed comparison
	assert.Greater(t, len(mockApp.session.CompletedComparisons), initialComparisons)
}

func TestComparisonScreen_HandleInputModeSwitch(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	// Test pairwise mode
	event := tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone)
	result := screen.handleInput(event)
	assert.Nil(t, result)
	assert.Equal(t, data.MethodPairwise, screen.comparisonMethod)

	// Test trio mode
	event = tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone)
	result = screen.handleInput(event)
	assert.Nil(t, result)
	assert.Equal(t, data.MethodTrio, screen.comparisonMethod)

	// Test quartet mode
	event = tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone)
	result = screen.handleInput(event)
	assert.Nil(t, result)
	assert.Equal(t, data.MethodQuartet, screen.comparisonMethod)
}

func TestComparisonScreen_HandleInputRanking(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)
	screen.setComparisonMode(data.MethodTrio) // Need 3+ proposals for ranking

	// Test ranking mode
	event := tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone)
	result := screen.handleInput(event)
	assert.Nil(t, result)
	assert.True(t, screen.isRanking)
}

func TestComparisonScreen_HandleInputSkipAndNext(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	// Test skip
	event := tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)
	result := screen.handleInput(event)
	assert.Nil(t, result)

	// Test next
	event = tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone)
	result = screen.handleInput(event)
	assert.Nil(t, result)
}

func TestComparisonScreen_HandleInputPassThrough(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	// Test keys that should pass through (like up/down for scrolling)
	event := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	result := screen.handleInput(event)
	assert.Equal(t, event, result)

	event = tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	result = screen.handleInput(event)
	assert.Equal(t, event, result)

	// Test j/k keys (should convert to up/down)
	event = tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone)
	result = screen.handleInput(event)
	assert.NotNil(t, result)
	assert.Equal(t, tcell.KeyDown, result.Key())

	event = tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone)
	result = screen.handleInput(event)
	assert.NotNil(t, result)
	assert.Equal(t, tcell.KeyUp, result.Key())
}

// TestComparisonScreen_CreateProposalCard is obsolete - createProposalCard method removed
// The proposal cards are now created dynamically in the updateDisplay method
/*
func TestComparisonScreen_CreateProposalCard(t *testing.T) {
	screen := NewComparisonScreen()
	proposal := data.Proposal{
		ID:           "test-prop",
		Title:        "Test Proposal",
		Speaker:      "Test Speaker",
		Abstract:     "This is a test abstract.",
		Score:        1550.0,
		ConflictTags: []string{"tag1", "tag2"},
	}

	card := screen.createProposalCard(proposal, 1)

	assert.NotNil(t, card)
	assert.IsType(t, (*tview.TextView)(nil), card)

	// Check that the card content includes key information
	text := card.GetText(false)
	assert.Contains(t, text, "Test Proposal")
	assert.Contains(t, text, "Test Speaker")
	assert.Contains(t, text, "This is a test abstract.")
	assert.Contains(t, text, "1550")
	assert.Contains(t, text, "tag1, tag2")
}
*/

func TestComparisonScreen_UpdateInstructions(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	// Test normal mode instructions
	screen.updateInstructions()
	text := screen.controlPanel.GetText(false)
	assert.Contains(t, text, "pairwise")
	assert.Contains(t, text, "Select the best proposal:")

	// Test ranking mode instructions
	screen.setComparisonMode(data.MethodTrio)
	screen.startRanking()
	screen.updateInstructions()
	text = screen.controlPanel.GetText(false)
	assert.Contains(t, text, "Ranking Mode:")
}

func TestComparisonScreen_UpdateProgress(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	screen.updateProgress()
	text := screen.progressBar.GetText(false)
	assert.Contains(t, text, "Comparisons:")
	assert.Contains(t, text, "%")
}

func TestComparisonScreen_UpdateStatus(t *testing.T) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	screen.updateStatus()
	text := screen.statusBar.GetText(false)
	assert.Contains(t, text, "Proposals:")
	assert.Contains(t, text, "Current:")

	// Test with winner selected
	screen.selectedWinner = "prop1"
	screen.updateStatus()
	text = screen.statusBar.GetText(false)
	assert.Contains(t, text, "Winner selected")
}

func TestComparisonScreen_GenerateComparisonID(t *testing.T) {
	screen := NewComparisonScreen()

	id1 := screen.generateComparisonID()
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	id2 := screen.generateComparisonID()

	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "comp_")
	assert.Contains(t, id2, "comp_")
}

func TestComparisonScreen_GetProposalIDs(t *testing.T) {
	screen := NewComparisonScreen()
	screen.currentProposals = []data.Proposal{
		{ID: "prop1", Title: "First"},
		{ID: "prop2", Title: "Second"},
		{ID: "prop3", Title: "Third"},
	}

	ids := screen.getProposalIDs()

	assert.Equal(t, []string{"prop1", "prop2", "prop3"}, ids)
}

func TestComparisonScreen_GetProposalIDsEmpty(t *testing.T) {
	screen := NewComparisonScreen()
	screen.currentProposals = []data.Proposal{}

	ids := screen.getProposalIDs()

	assert.Empty(t, ids)
}

// Benchmark tests
// BenchmarkComparisonScreen_CreateProposalCard is obsolete - method removed
/*
func BenchmarkComparisonScreen_CreateProposalCard(b *testing.B) {
	screen := NewComparisonScreen()
	proposal := data.Proposal{
		ID:       "test-prop",
		Title:    "Test Proposal",
		Speaker:  "Test Speaker",
		Abstract: "This is a test abstract for benchmarking purposes. It contains multiple sentences and various formatting to test the performance of card creation.",
		Score:    1550.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		screen.createProposalCard(proposal, 1)
	}
}
*/

func BenchmarkComparisonScreen_UpdateDisplay(b *testing.B) {
	screen := NewComparisonScreen()
	mockApp := &MockApp{session: createTestSession()}
	screen.OnEnter(mockApp)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		screen.updateDisplay()
	}
}
