// Package components provides reusable TUI components for conference talk ranking.
// This file implements a proposal carousel component for smooth navigation
// between proposals with detail expansion and content overflow handling.
package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/pashagolub/confelo/pkg/data"
)

// Carousel displays a navigable set of proposals with smooth transitions
type Carousel struct {
	// UI components
	container    *tview.Flex
	currentCard  *tview.TextView
	navIndicator *tview.TextView

	// Data and state
	proposals    []data.Proposal
	currentIndex int
	totalItems   int

	// Display configuration
	showNumbers    bool
	highlightColor tcell.Color
	normalColor    tcell.Color
	showNavigation bool
	expandedView   bool

	// Navigation callbacks
	onNavigate  func(index int, proposal data.Proposal)
	onSelect    func(index int, proposal data.Proposal)
	keyHandlers map[tcell.Key]func() bool
}

// CarouselConfig holds configuration options for the carousel
type CarouselConfig struct {
	ShowNumbers    bool
	HighlightColor tcell.Color
	NormalColor    tcell.Color
	ShowNavigation bool
	ExpandedView   bool
	OnNavigate     func(index int, proposal data.Proposal)
	OnSelect       func(index int, proposal data.Proposal)
}

// NewCarousel creates a new proposal carousel with default configuration
func NewCarousel() *Carousel {
	c := &Carousel{
		container:      tview.NewFlex(),
		currentCard:    tview.NewTextView(),
		navIndicator:   tview.NewTextView(),
		currentIndex:   0,
		showNumbers:    true,
		highlightColor: tcell.ColorYellow,
		normalColor:    tcell.ColorWhite,
		showNavigation: true,
		expandedView:   false,
		keyHandlers:    make(map[tcell.Key]func() bool),
	}

	c.setupUI()
	c.setupKeyHandlers()
	return c
}

// NewCarouselWithConfig creates a carousel with custom configuration
func NewCarouselWithConfig(config CarouselConfig) *Carousel {
	c := NewCarousel()

	c.showNumbers = config.ShowNumbers
	c.highlightColor = config.HighlightColor
	c.normalColor = config.NormalColor
	c.showNavigation = config.ShowNavigation
	c.expandedView = config.ExpandedView
	c.onNavigate = config.OnNavigate
	c.onSelect = config.OnSelect

	c.setupUI()
	return c
}

// setupUI initializes the carousel layout
func (c *Carousel) setupUI() {
	// Configure main container as vertical layout
	c.container.SetDirection(tview.FlexRow)

	// Configure current card display
	c.currentCard.SetBorder(true)
	c.currentCard.SetWordWrap(true)
	c.currentCard.SetScrollable(true)

	// Configure navigation indicator
	c.navIndicator.
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// Layout: main card takes most space, navigation indicator at bottom
	c.container.
		AddItem(c.currentCard, 0, 1, true)

	if c.showNavigation {
		c.container.AddItem(c.navIndicator, 1, 0, false)
	}

	// Set up input handling
	c.container.SetInputCapture(c.handleInput)
}

// setupKeyHandlers initializes default key bindings
func (c *Carousel) setupKeyHandlers() {
	// Navigation keys
	c.keyHandlers[tcell.KeyLeft] = func() bool {
		return c.Previous()
	}
	c.keyHandlers[tcell.KeyRight] = func() bool {
		return c.Next()
	}
	c.keyHandlers[tcell.KeyHome] = func() bool {
		return c.First()
	}
	c.keyHandlers[tcell.KeyEnd] = func() bool {
		return c.Last()
	}

	// Selection key
	c.keyHandlers[tcell.KeyEnter] = func() bool {
		if c.onSelect != nil && c.HasProposals() {
			c.onSelect(c.currentIndex, c.GetCurrentProposal())
		}
		return true
	}

	// View toggle
	c.keyHandlers[tcell.KeyTab] = func() bool {
		c.ToggleExpandedView()
		return true
	}
}

// SetProposals updates the carousel with new proposals
func (c *Carousel) SetProposals(proposals []data.Proposal) {
	c.proposals = make([]data.Proposal, len(proposals))
	copy(c.proposals, proposals)
	c.totalItems = len(proposals)

	// Reset to first item if we have proposals
	if c.totalItems > 0 {
		c.currentIndex = 0
	} else {
		c.currentIndex = -1
	}

	c.updateDisplay()
}

// GetCurrentProposal returns the currently displayed proposal
func (c *Carousel) GetCurrentProposal() data.Proposal {
	if !c.HasProposals() {
		return data.Proposal{}
	}
	return c.proposals[c.currentIndex]
}

// GetCurrentIndex returns the current carousel position
func (c *Carousel) GetCurrentIndex() int {
	return c.currentIndex
}

// HasProposals returns true if the carousel has any proposals
func (c *Carousel) HasProposals() bool {
	return c.totalItems > 0 && c.currentIndex >= 0 && c.currentIndex < c.totalItems
}

// Navigation methods

// Next moves to the next proposal
func (c *Carousel) Next() bool {
	if !c.HasProposals() {
		return false
	}

	newIndex := (c.currentIndex + 1) % c.totalItems
	return c.NavigateTo(newIndex)
}

// Previous moves to the previous proposal
func (c *Carousel) Previous() bool {
	if !c.HasProposals() {
		return false
	}

	newIndex := c.currentIndex - 1
	if newIndex < 0 {
		newIndex = c.totalItems - 1
	}
	return c.NavigateTo(newIndex)
}

// First moves to the first proposal
func (c *Carousel) First() bool {
	if !c.HasProposals() {
		return false
	}
	return c.NavigateTo(0)
}

// Last moves to the last proposal
func (c *Carousel) Last() bool {
	if !c.HasProposals() {
		return false
	}
	return c.NavigateTo(c.totalItems - 1)
}

// NavigateTo moves to a specific proposal index
func (c *Carousel) NavigateTo(index int) bool {
	if !c.HasProposals() || index < 0 || index >= c.totalItems {
		return false
	}

	c.currentIndex = index
	c.updateDisplay()

	// Call navigation callback
	if c.onNavigate != nil {
		c.onNavigate(c.currentIndex, c.GetCurrentProposal())
	}

	return true
}

// Display configuration methods

// SetHighlightColor sets the color for the active proposal
func (c *Carousel) SetHighlightColor(color tcell.Color) {
	c.highlightColor = color
	c.updateDisplay()
}

// SetNormalColor sets the color for inactive proposals
func (c *Carousel) SetNormalColor(color tcell.Color) {
	c.normalColor = color
	c.updateDisplay()
}

// ToggleExpandedView switches between compact and expanded display modes
func (c *Carousel) ToggleExpandedView() {
	c.expandedView = !c.expandedView
	c.updateDisplay()
}

// SetExpandedView sets the expanded view mode
func (c *Carousel) SetExpandedView(expanded bool) {
	c.expandedView = expanded
	c.updateDisplay()
}

// Callback configuration

// SetOnNavigate sets the callback for navigation events
func (c *Carousel) SetOnNavigate(callback func(index int, proposal data.Proposal)) {
	c.onNavigate = callback
}

// SetOnSelect sets the callback for selection events
func (c *Carousel) SetOnSelect(callback func(index int, proposal data.Proposal)) {
	c.onSelect = callback
}

// AddKeyHandler adds a custom key handler
func (c *Carousel) AddKeyHandler(key tcell.Key, handler func() bool) {
	c.keyHandlers[key] = handler
}

// RemoveKeyHandler removes a key handler
func (c *Carousel) RemoveKeyHandler(key tcell.Key) {
	delete(c.keyHandlers, key)
}

// GetPrimitive returns the main container for integration with tview
func (c *Carousel) GetPrimitive() tview.Primitive {
	return c.container
}

// handleInput processes keyboard input for the carousel
func (c *Carousel) handleInput(event *tcell.EventKey) *tcell.EventKey {
	key := event.Key()

	// Check for custom key handlers
	if handler, exists := c.keyHandlers[key]; exists {
		if handler() {
			return nil // Event consumed
		}
	}

	// Handle rune keys for number navigation
	if key == tcell.KeyRune {
		ch := event.Rune()
		if ch >= '1' && ch <= '9' {
			// Navigate to numbered position
			pos := int(ch - '1')
			if pos < c.totalItems {
				c.NavigateTo(pos)
			}
			return nil // Always consume number keys, even if invalid
		}
	}

	return event // Pass through unhandled events
}

// updateDisplay refreshes the carousel display
func (c *Carousel) updateDisplay() {
	if !c.HasProposals() {
		c.currentCard.SetText("[dim]No proposals available[-]")
		c.currentCard.SetTitle("Carousel")
		c.currentCard.SetBorderColor(c.normalColor)

		if c.showNavigation {
			c.navIndicator.SetText("[dim]0 / 0[-]")
		}
		return
	}

	proposal := c.GetCurrentProposal()

	// Format proposal content
	content := c.formatProposalContent(proposal)
	c.currentCard.SetText(content)

	// Update title
	title := "Proposal"
	if c.showNumbers {
		title = fmt.Sprintf("Proposal %d", c.currentIndex+1)
	}
	c.currentCard.SetTitle(title)
	c.currentCard.SetBorderColor(c.highlightColor)
	c.currentCard.SetTitleColor(c.highlightColor)

	// Update navigation indicator
	if c.showNavigation {
		c.updateNavigationIndicator()
	}
}

// formatProposalContent creates formatted text for a proposal
func (c *Carousel) formatProposalContent(proposal data.Proposal) string {
	var content strings.Builder

	// Title
	content.WriteString(fmt.Sprintf("[white::b]%s[white::-]\n\n", proposal.Title))

	// Speaker (if available)
	if proposal.Speaker != "" {
		content.WriteString(fmt.Sprintf("[yellow]Speaker:[-] %s\n\n", proposal.Speaker))
	}

	// Abstract (with length handling)
	if proposal.Abstract != "" {
		abstract := proposal.Abstract

		// In compact mode, truncate long abstracts
		if !c.expandedView && len(abstract) > 300 {
			abstract = abstract[:297] + "..."
			content.WriteString(fmt.Sprintf("[green]Abstract:[-]\n%s\n\n", abstract))
			content.WriteString("[dim]Press Tab for full view[-]\n\n")
		} else {
			content.WriteString(fmt.Sprintf("[green]Abstract:[-]\n%s\n\n", abstract))
		}
	}

	// Current rating
	content.WriteString(fmt.Sprintf("[blue]Current Rating:[-] %.0f", proposal.Score))

	// Original score (if different)
	if proposal.OriginalScore != nil && *proposal.OriginalScore != proposal.Score {
		content.WriteString(fmt.Sprintf(" (was %.0f)", *proposal.OriginalScore))
	}

	// Conflict tags (if any)
	if len(proposal.ConflictTags) > 0 {
		content.WriteString(fmt.Sprintf("\n\n[red]Conflicts:[-] %s", strings.Join(proposal.ConflictTags, ", ")))
	}

	// Metadata (in expanded view)
	if c.expandedView && len(proposal.Metadata) > 0 {
		content.WriteString("\n\n[cyan]Additional Information:[-]")
		for key, value := range proposal.Metadata {
			if value != "" {
				content.WriteString(fmt.Sprintf("\n  %s: %s", key, value))
			}
		}
	}

	return content.String()
}

// updateNavigationIndicator updates the navigation display
func (c *Carousel) updateNavigationIndicator() {
	if !c.HasProposals() {
		c.navIndicator.SetText("[dim]0 / 0[-]")
		return
	}

	// Create navigation indicator with visual dots
	indicator := fmt.Sprintf("[white]%d / %d[-]", c.currentIndex+1, c.totalItems)

	// Add visual indicators for small sets
	if c.totalItems <= 10 {
		dots := make([]string, c.totalItems)
		for i := 0; i < c.totalItems; i++ {
			if i == c.currentIndex {
				dots[i] = "[yellow]●[-]"
			} else {
				dots[i] = "[dim]○[-]"
			}
		}
		indicator = fmt.Sprintf("%s  %s", indicator, strings.Join(dots, " "))
	}

	// Add navigation hints
	if c.totalItems > 1 {
		indicator += "\n[dim]← → Navigate  Tab: Expand  Enter: Select[-]"
	}

	c.navIndicator.SetText(indicator)
}
