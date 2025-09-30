// Package components provides reusable TUI components for conference talk ranking.
// This file contains comprehensive tests for the proposal carousel component.
package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pashagolub/confelo/pkg/data"
)

// Test data helpers

func createTestProposal(id, title, speaker, abstract string, score float64, conflictTags []string) data.Proposal {
	now := time.Now()
	return data.Proposal{
		ID:           id,
		Title:        title,
		Speaker:      speaker,
		Abstract:     abstract,
		Score:        score,
		ConflictTags: conflictTags,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata:     make(map[string]string),
	}
}

func createTestProposals() []data.Proposal {
	return []data.Proposal{
		createTestProposal("1", "Introduction to Go", "John Doe", "A beginner-friendly talk about Go programming.", 1500.0, nil),
		createTestProposal("2", "Advanced Testing", "Jane Smith", "Deep dive into testing strategies and best practices.", 1600.0, []string{"testing"}),
		createTestProposal("3", "Microservices Architecture", "Bob Johnson", "Building scalable microservices with Go and Docker.", 1400.0, nil),
		createTestProposal("4", "Performance Optimization", "Alice Brown", "Techniques for optimizing Go applications for high performance.", 1550.0, []string{"performance"}),
	}
}

func createLongAbstractProposal() data.Proposal {
	longAbstract := "This is a very long abstract that should be truncated in compact mode. " +
		"It contains multiple sentences and goes on for quite a while to test the " +
		"truncation functionality. The abstract should be cut off at 300 characters " +
		"in compact mode and show the full text in expanded mode. This allows users " +
		"to quickly scan proposals while still having access to full details when needed."

	return createTestProposal("long", "Long Abstract Test", "Test Speaker", longAbstract, 1500.0, nil)
}

// Constructor tests

func TestNewCarousel(t *testing.T) {
	carousel := NewCarousel()

	assert.NotNil(t, carousel)
	assert.NotNil(t, carousel.container)
	assert.NotNil(t, carousel.currentCard)
	assert.NotNil(t, carousel.navIndicator)
	assert.Equal(t, 0, carousel.currentIndex)
	assert.Equal(t, 0, carousel.totalItems)
	assert.True(t, carousel.showNumbers)
	assert.Equal(t, tcell.ColorYellow, carousel.highlightColor)
	assert.Equal(t, tcell.ColorWhite, carousel.normalColor)
	assert.True(t, carousel.showNavigation)
	assert.False(t, carousel.expandedView)
}

func TestNewCarouselWithConfig(t *testing.T) {
	config := CarouselConfig{
		ShowNumbers:    false,
		HighlightColor: tcell.ColorRed,
		NormalColor:    tcell.ColorBlue,
		ShowNavigation: false,
		ExpandedView:   true,
	}

	carousel := NewCarouselWithConfig(config)

	assert.False(t, carousel.showNumbers)
	assert.Equal(t, tcell.ColorRed, carousel.highlightColor)
	assert.Equal(t, tcell.ColorBlue, carousel.normalColor)
	assert.False(t, carousel.showNavigation)
	assert.True(t, carousel.expandedView)
}

// Data management tests

func TestSetProposals(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()

	carousel.SetProposals(proposals)

	assert.Equal(t, len(proposals), carousel.totalItems)
	assert.Equal(t, 0, carousel.currentIndex)
	assert.True(t, carousel.HasProposals())
}

func TestSetProposalsEmpty(t *testing.T) {
	carousel := NewCarousel()
	carousel.SetProposals([]data.Proposal{})

	assert.Equal(t, 0, carousel.totalItems)
	assert.Equal(t, -1, carousel.currentIndex)
	assert.False(t, carousel.HasProposals())
}

func TestGetCurrentProposal(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	// Test first proposal
	current := carousel.GetCurrentProposal()
	assert.Equal(t, proposals[0].ID, current.ID)
	assert.Equal(t, proposals[0].Title, current.Title)

	// Test after navigation
	carousel.Next()
	current = carousel.GetCurrentProposal()
	assert.Equal(t, proposals[1].ID, current.ID)
}

func TestGetCurrentProposalEmpty(t *testing.T) {
	carousel := NewCarousel()
	current := carousel.GetCurrentProposal()

	assert.Equal(t, "", current.ID)
	assert.Equal(t, "", current.Title)
}

func TestGetCurrentIndex(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	assert.Equal(t, 0, carousel.GetCurrentIndex())

	carousel.Next()
	assert.Equal(t, 1, carousel.GetCurrentIndex())

	carousel.Previous()
	assert.Equal(t, 0, carousel.GetCurrentIndex())
}

func TestHasProposals(t *testing.T) {
	carousel := NewCarousel()

	// Empty carousel
	assert.False(t, carousel.HasProposals())

	// With proposals
	proposals := createTestProposals()
	carousel.SetProposals(proposals)
	assert.True(t, carousel.HasProposals())

	// After clearing
	carousel.SetProposals([]data.Proposal{})
	assert.False(t, carousel.HasProposals())
}

// Navigation tests

func TestNext(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	// Test normal progression
	assert.Equal(t, 0, carousel.currentIndex)

	success := carousel.Next()
	assert.True(t, success)
	assert.Equal(t, 1, carousel.currentIndex)

	success = carousel.Next()
	assert.True(t, success)
	assert.Equal(t, 2, carousel.currentIndex)

	// Test wraparound at end
	carousel.NavigateTo(3) // Last item
	success = carousel.Next()
	assert.True(t, success)
	assert.Equal(t, 0, carousel.currentIndex) // Should wrap to first
}

func TestNextEmpty(t *testing.T) {
	carousel := NewCarousel()
	success := carousel.Next()
	assert.False(t, success)
}

func TestPrevious(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	// Test normal regression
	carousel.NavigateTo(2)
	assert.Equal(t, 2, carousel.currentIndex)

	success := carousel.Previous()
	assert.True(t, success)
	assert.Equal(t, 1, carousel.currentIndex)

	// Test wraparound at beginning
	carousel.NavigateTo(0) // First item
	success = carousel.Previous()
	assert.True(t, success)
	assert.Equal(t, 3, carousel.currentIndex) // Should wrap to last
}

func TestPreviousEmpty(t *testing.T) {
	carousel := NewCarousel()
	success := carousel.Previous()
	assert.False(t, success)
}

func TestFirst(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	carousel.NavigateTo(2)
	assert.Equal(t, 2, carousel.currentIndex)

	success := carousel.First()
	assert.True(t, success)
	assert.Equal(t, 0, carousel.currentIndex)
}

func TestFirstEmpty(t *testing.T) {
	carousel := NewCarousel()
	success := carousel.First()
	assert.False(t, success)
}

func TestLast(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	assert.Equal(t, 0, carousel.currentIndex)

	success := carousel.Last()
	assert.True(t, success)
	assert.Equal(t, 3, carousel.currentIndex) // Last item (0-indexed)
}

func TestLastEmpty(t *testing.T) {
	carousel := NewCarousel()
	success := carousel.Last()
	assert.False(t, success)
}

func TestNavigateTo(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	// Test valid navigation
	success := carousel.NavigateTo(2)
	assert.True(t, success)
	assert.Equal(t, 2, carousel.currentIndex)

	// Test invalid navigation
	success = carousel.NavigateTo(-1)
	assert.False(t, success)
	assert.Equal(t, 2, carousel.currentIndex) // Should not change

	success = carousel.NavigateTo(10)
	assert.False(t, success)
	assert.Equal(t, 2, carousel.currentIndex) // Should not change
}

func TestNavigateToEmpty(t *testing.T) {
	carousel := NewCarousel()
	success := carousel.NavigateTo(0)
	assert.False(t, success)
}

// Display configuration tests

func TestSetHighlightColor(t *testing.T) {
	carousel := NewCarousel()
	carousel.SetHighlightColor(tcell.ColorRed)
	assert.Equal(t, tcell.ColorRed, carousel.highlightColor)
}

func TestSetNormalColor(t *testing.T) {
	carousel := NewCarousel()
	carousel.SetNormalColor(tcell.ColorBlue)
	assert.Equal(t, tcell.ColorBlue, carousel.normalColor)
}

func TestToggleExpandedView(t *testing.T) {
	carousel := NewCarousel()
	assert.False(t, carousel.expandedView)

	carousel.ToggleExpandedView()
	assert.True(t, carousel.expandedView)

	carousel.ToggleExpandedView()
	assert.False(t, carousel.expandedView)
}

func TestSetExpandedView(t *testing.T) {
	carousel := NewCarousel()
	assert.False(t, carousel.expandedView)

	carousel.SetExpandedView(true)
	assert.True(t, carousel.expandedView)

	carousel.SetExpandedView(false)
	assert.False(t, carousel.expandedView)
}

// Callback tests

func TestNavigationCallback(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	var callbackIndex int
	var callbackProposal data.Proposal
	callbackCalled := false

	carousel.SetOnNavigate(func(index int, proposal data.Proposal) {
		callbackIndex = index
		callbackProposal = proposal
		callbackCalled = true
	})

	carousel.Next()

	assert.True(t, callbackCalled)
	assert.Equal(t, 1, callbackIndex)
	assert.Equal(t, proposals[1].ID, callbackProposal.ID)
}

func TestSelectionCallback(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	var callbackIndex int
	var callbackProposal data.Proposal
	callbackCalled := false

	carousel.SetOnSelect(func(index int, proposal data.Proposal) {
		callbackIndex = index
		callbackProposal = proposal
		callbackCalled = true
	})

	// Simulate selection
	if carousel.onSelect != nil {
		carousel.onSelect(carousel.currentIndex, carousel.GetCurrentProposal())
	}

	assert.True(t, callbackCalled)
	assert.Equal(t, 0, callbackIndex)
	assert.Equal(t, proposals[0].ID, callbackProposal.ID)
}

// Content formatting tests

func TestFormatProposalContentBasic(t *testing.T) {
	carousel := NewCarousel()
	proposal := createTestProposal("1", "Test Title", "Test Speaker", "Test abstract", 1500.0, nil)

	content := carousel.formatProposalContent(proposal)

	assert.Contains(t, content, "Test Title")
	assert.Contains(t, content, "Test Speaker")
	assert.Contains(t, content, "Test abstract")
	assert.Contains(t, content, "1500")
}

func TestFormatProposalContentWithConflicts(t *testing.T) {
	carousel := NewCarousel()
	proposal := createTestProposal("1", "Test Title", "Test Speaker", "Test abstract", 1500.0, []string{"conflict1", "conflict2"})

	content := carousel.formatProposalContent(proposal)

	assert.Contains(t, content, "conflict1, conflict2")
}

func TestFormatProposalContentTruncation(t *testing.T) {
	carousel := NewCarousel()
	proposal := createLongAbstractProposal()

	// Test compact mode (should truncate)
	carousel.SetExpandedView(false)
	content := carousel.formatProposalContent(proposal)
	assert.Contains(t, content, "...")
	assert.Contains(t, content, "Press Tab for full view")

	// Test expanded mode (should show full content)
	carousel.SetExpandedView(true)
	content = carousel.formatProposalContent(proposal)
	assert.NotContains(t, content, "...")
	assert.NotContains(t, content, "Press Tab for full view")
}

func TestFormatProposalContentWithOriginalScore(t *testing.T) {
	carousel := NewCarousel()
	original := 1400.0
	proposal := createTestProposal("1", "Test Title", "Test Speaker", "Test abstract", 1500.0, nil)
	proposal.OriginalScore = &original

	content := carousel.formatProposalContent(proposal)

	assert.Contains(t, content, "1500")
	assert.Contains(t, content, "(was 1400)")
}

func TestFormatProposalContentWithMetadata(t *testing.T) {
	carousel := NewCarousel()
	proposal := createTestProposal("1", "Test Title", "Test Speaker", "Test abstract", 1500.0, nil)
	proposal.Metadata["category"] = "backend"
	proposal.Metadata["level"] = "intermediate"

	// Should not show metadata in compact mode
	carousel.SetExpandedView(false)
	content := carousel.formatProposalContent(proposal)
	assert.NotContains(t, content, "Additional Information")
	assert.NotContains(t, content, "category")

	// Should show metadata in expanded mode
	carousel.SetExpandedView(true)
	content = carousel.formatProposalContent(proposal)
	assert.Contains(t, content, "Additional Information")
	assert.Contains(t, content, "category: backend")
	assert.Contains(t, content, "level: intermediate")
}

// Key handler tests

func TestAddRemoveKeyHandler(t *testing.T) {
	carousel := NewCarousel()
	handlerCalled := false

	// Add custom key handler
	carousel.AddKeyHandler(tcell.KeyF1, func() bool {
		handlerCalled = true
		return true
	})

	// Simulate key event
	event := tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone)
	result := carousel.handleInput(event)

	assert.True(t, handlerCalled)
	assert.Nil(t, result) // Event should be consumed

	// Remove key handler
	carousel.RemoveKeyHandler(tcell.KeyF1)
	handlerCalled = false

	// Simulate same key event
	result = carousel.handleInput(event)
	assert.False(t, handlerCalled)
	assert.NotNil(t, result) // Event should pass through
}

func TestHandleInputNavigation(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	// Test right arrow (next)
	event := tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	result := carousel.handleInput(event)
	assert.Nil(t, result)
	assert.Equal(t, 1, carousel.currentIndex)

	// Test left arrow (previous)
	event = tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	result = carousel.handleInput(event)
	assert.Nil(t, result)
	assert.Equal(t, 0, carousel.currentIndex)

	// Test home (first)
	carousel.NavigateTo(2)
	event = tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
	result = carousel.handleInput(event)
	assert.Nil(t, result)
	assert.Equal(t, 0, carousel.currentIndex)

	// Test end (last)
	event = tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
	result = carousel.handleInput(event)
	assert.Nil(t, result)
	assert.Equal(t, 3, carousel.currentIndex)
}

func TestHandleInputNumbers(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	// Test number navigation
	event := tcell.NewEventKey(tcell.KeyRune, '3', tcell.ModNone)
	result := carousel.handleInput(event)
	assert.Nil(t, result)
	assert.Equal(t, 2, carousel.currentIndex) // '3' maps to index 2 (0-based)

	// Test invalid number
	event = tcell.NewEventKey(tcell.KeyRune, '9', tcell.ModNone)
	result = carousel.handleInput(event)
	assert.Nil(t, result)
	assert.Equal(t, 2, carousel.currentIndex) // Should not change
}

func TestHandleInputTab(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()
	carousel.SetProposals(proposals)

	assert.False(t, carousel.expandedView)

	// Test tab toggles expanded view
	event := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	result := carousel.handleInput(event)
	assert.Nil(t, result)
	assert.True(t, carousel.expandedView)
}

// Integration tests

func TestCarouselLifecycle(t *testing.T) {
	carousel := NewCarousel()
	proposals := createTestProposals()

	// Start empty
	assert.False(t, carousel.HasProposals())
	assert.False(t, carousel.Next())

	// Load proposals
	carousel.SetProposals(proposals)
	assert.True(t, carousel.HasProposals())
	assert.Equal(t, "Introduction to Go", carousel.GetCurrentProposal().Title)

	// Navigate through all proposals
	for i := 1; i < len(proposals); i++ {
		assert.True(t, carousel.Next())
		assert.Equal(t, proposals[i].Title, carousel.GetCurrentProposal().Title)
	}

	// Wrap around
	assert.True(t, carousel.Next())
	assert.Equal(t, proposals[0].Title, carousel.GetCurrentProposal().Title)

	// Clear proposals
	carousel.SetProposals([]data.Proposal{})
	assert.False(t, carousel.HasProposals())
}

func TestCarouselPrimitiveIntegration(t *testing.T) {
	carousel := NewCarousel()
	primitive := carousel.GetPrimitive()

	require.NotNil(t, primitive)
	assert.Equal(t, carousel.container, primitive)
}

// Performance and edge case tests

func TestCarouselWithSingleProposal(t *testing.T) {
	carousel := NewCarousel()
	proposal := createTestProposal("single", "Single Proposal", "Speaker", "Abstract", 1500.0, nil)
	carousel.SetProposals([]data.Proposal{proposal})

	assert.True(t, carousel.HasProposals())
	assert.Equal(t, 0, carousel.currentIndex)

	// Navigation should wrap properly
	assert.True(t, carousel.Next())
	assert.Equal(t, 0, carousel.currentIndex) // Should stay at 0

	assert.True(t, carousel.Previous())
	assert.Equal(t, 0, carousel.currentIndex) // Should stay at 0

	assert.True(t, carousel.First())
	assert.Equal(t, 0, carousel.currentIndex)

	assert.True(t, carousel.Last())
	assert.Equal(t, 0, carousel.currentIndex)
}

func TestCarouselWithManyProposals(t *testing.T) {
	carousel := NewCarousel()

	// Create many proposals
	var proposals []data.Proposal
	for i := 0; i < 50; i++ {
		proposals = append(proposals, createTestProposal(
			fmt.Sprintf("id%d", i),
			fmt.Sprintf("Proposal %d", i),
			fmt.Sprintf("Speaker %d", i),
			fmt.Sprintf("Abstract for proposal %d", i),
			1500.0+float64(i),
			nil,
		))
	}

	carousel.SetProposals(proposals)
	assert.Equal(t, 50, carousel.totalItems)

	// Test navigation to various positions
	assert.True(t, carousel.NavigateTo(25))
	assert.Equal(t, 25, carousel.currentIndex)

	assert.True(t, carousel.Last())
	assert.Equal(t, 49, carousel.currentIndex)

	assert.True(t, carousel.First())
	assert.Equal(t, 0, carousel.currentIndex)
}
