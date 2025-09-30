// Package tui provides Terminal User Interface screens for conference talk ranking.
// This file implements the help screen that displays keyboard shortcuts and usage instructions.
package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HelpScreen provides help and keyboard shortcut information
type HelpScreen struct {
	root     *tview.Flex
	textView *tview.TextView
	app      *App
}

// NewHelpScreen creates a new help screen
func NewHelpScreen() *HelpScreen {
	hs := &HelpScreen{
		root:     tview.NewFlex(),
		textView: tview.NewTextView(),
	}

	hs.setupLayout()
	return hs
}

// GetPrimitive returns the root primitive for this screen
func (hs *HelpScreen) GetPrimitive() tview.Primitive {
	return hs.root
}

// OnEnter is called when the help screen becomes active
func (hs *HelpScreen) OnEnter(app *App) error {
	hs.app = app
	hs.updateContent()
	return nil
}

// OnExit is called when leaving the help screen
func (hs *HelpScreen) OnExit(app *App) error {
	return nil
}

// GetTitle returns the screen title
func (hs *HelpScreen) GetTitle() string {
	return "Help"
}

// GetHelpText returns help text for this screen
func (hs *HelpScreen) GetHelpText() []string {
	return []string{
		"Press ESC or q to go back",
		"Use arrow keys to scroll",
	}
}

// setupLayout configures the help screen layout
func (hs *HelpScreen) setupLayout() {
	// Configure text view
	hs.textView.
		SetBorder(true).
		SetTitle("Help - Conference Talk Ranking System").
		SetTitleAlign(tview.AlignCenter)

	hs.textView.SetWrap(true).
		SetDynamicColors(true).
		SetScrollable(true)

	// Set up key bindings for the text view
	hs.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			if hs.app != nil {
				go hs.app.GoBack()
			}
			return nil
		}

		switch event.Rune() {
		case 'q', 'Q':
			if hs.app != nil {
				go hs.app.GoBack()
			}
			return nil
		}

		return event
	})

	// Add to layout
	hs.root.AddItem(hs.textView, 0, 1, true)
}

// updateContent updates the help screen content
func (hs *HelpScreen) updateContent() {
	var content strings.Builder

	// Application overview
	content.WriteString("[yellow]Conference Talk Ranking System[-]\n\n")
	content.WriteString("This application helps you rank conference talk proposals using the Elo rating system.\n")
	content.WriteString("The system provides pairwise and multi-way comparisons to converge on reliable rankings.\n\n")

	// Global keyboard shortcuts
	content.WriteString("[green]Global Keyboard Shortcuts[-]\n")
	content.WriteString("═════════════════════════════\n")
	for _, binding := range globalKeyBindings {
		keyText := ""
		if binding.Key != tcell.KeyRune {
			keyText = tcell.KeyNames[binding.Key]
		} else {
			keyText = string(binding.Rune)
		}
		content.WriteString("[white]")
		content.WriteString(keyText)
		content.WriteString("[-]  - ")
		content.WriteString(binding.Description)
		content.WriteString("\n")
	}

	// Screen navigation
	content.WriteString("\n[green]Screen Navigation[-]\n")
	content.WriteString("═══════════════════\n")
	content.WriteString("[white]Setup[-]      - Configure CSV input and Elo parameters\n")
	content.WriteString("[white]Comparison[-] - Perform proposal comparisons\n")
	content.WriteString("[white]Ranking[-]    - View current rankings and export results\n")
	content.WriteString("[white]Help[-]       - This help screen\n")

	// Workflow
	content.WriteString("\n[green]Typical Workflow[-]\n")
	content.WriteString("══════════════════\n")
	content.WriteString("1. Start with [white]Setup[-] screen to load your proposal CSV file\n")
	content.WriteString("2. Configure Elo parameters (K-factor, initial ratings, etc.)\n")
	content.WriteString("3. Move to [white]Comparison[-] screen to perform rankings\n")
	content.WriteString("4. Use [white]Ranking[-] screen to view results and export final rankings\n")

	// Tips
	content.WriteString("\n[green]Tips[-]\n")
	content.WriteString("════\n")
	content.WriteString("• The system tracks convergence automatically\n")
	content.WriteString("• All comparisons are logged for audit purposes\n")
	content.WriteString("• You can pause and resume sessions at any time\n")
	content.WriteString("• Export preserves your original CSV format with added rankings\n")
	content.WriteString("• Use trio/quartet comparisons for faster convergence with many proposals\n")

	// Version and about
	content.WriteString("\n[green]About[-]\n")
	content.WriteString("═════\n")
	content.WriteString("Conference Talk Ranking System v1.0\n")
	content.WriteString("Built with Go, tview, and mathematical rigor\n")
	content.WriteString("Privacy-first: All data stays on your machine\n")

	hs.textView.SetText(content.String())
}
