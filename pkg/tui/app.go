// Package tui provides Terminal User Interface functionality for conference talk ranking.
// It implements the main TUI application structure with screen management, keyboard shortcuts,
// and help system following the established terminal UI patterns.
package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/pashagolub/confelo/pkg/data"
)

// ScreenType represents different screens in the TUI application
type ScreenType int

const (
	ScreenComparison ScreenType = iota
	ScreenRanking
	ScreenHelp
)

// String returns the string representation of ScreenType
func (s ScreenType) String() string {
	switch s {
	case ScreenComparison:
		return "comparison"
	case ScreenRanking:
		return "ranking"
	case ScreenHelp:
		return "help"
	default:
		return "unknown"
	}
}

// Screen interface defines the contract for all TUI screens
type Screen interface {
	// GetPrimitive returns the tview.Primitive for this screen
	GetPrimitive() tview.Primitive

	// OnEnter is called when the screen becomes active
	OnEnter(app interface{}) error

	// OnExit is called when leaving the screen
	OnExit(app interface{}) error

	// GetTitle returns the screen title for display
	GetTitle() string

	// GetHelpText returns help text specific to this screen
	GetHelpText() []string
}

// AppState represents the current application state
type AppState struct {
	mu             sync.RWMutex
	session        *data.Session
	storage        data.Storage
	config         *data.SessionConfig
	currentScreen  ScreenType
	previousScreen ScreenType
	isRunning      bool
}

// App represents the main TUI application
type App struct {
	tviewApp *tview.Application
	pages    *tview.Pages
	header   *tview.TextView
	footer   *tview.TextView
	state    *AppState
	screens  map[ScreenType]Screen
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
}

// KeyBinding represents a keyboard shortcut
type KeyBinding struct {
	Key         tcell.Key
	Rune        rune
	Description string
	Handler     func(app *App) error
}

// Global key bindings available across all screens
var globalKeyBindings = []KeyBinding{
	{Key: tcell.KeyF1, Description: "Show help", Handler: (*App).ShowHelp},
	{Key: tcell.KeyEsc, Description: "Go back/Exit", Handler: (*App).GoBack},
	{Key: tcell.KeyCtrlR, Description: "Show rankings", Handler: (*App).ShowRanking},
}

// NewApp creates a new TUI application instance
func NewApp(config *data.SessionConfig, storage data.Storage) (*App, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		tviewApp: tview.NewApplication(),
		pages:    tview.NewPages(),
		header:   tview.NewTextView(),
		footer:   tview.NewTextView(),
		state: &AppState{
			config:        config,
			storage:       storage,
			currentScreen: ScreenComparison,
			isRunning:     false,
		},
		screens: make(map[ScreenType]Screen),
		ctx:     ctx,
		cancel:  cancel,
	}

	if err := app.setupUI(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup UI: %w", err)
	}

	return app, nil
}

// setupUI initializes the UI components and layout
func (a *App) setupUI() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Configure header
	a.header.SetBorder(true).
		SetTitle("Conference Talk Ranking System").
		SetTitleAlign(tview.AlignCenter).
		SetBackgroundColor(tcell.ColorDarkBlue)
	a.header.SetTextColor(tcell.ColorWhite)

	// Configure footer with help text
	a.footer.SetBorder(true).
		SetTitle("Keyboard Shortcuts").
		SetTitleAlign(tview.AlignCenter).
		SetBackgroundColor(tcell.ColorDarkGreen)
	a.footer.SetTextColor(tcell.ColorWhite)

	a.updateFooter()

	// Create main layout
	mainLayout := tview.NewFlex().SetDirection(tview.FlexRow)

	// Add header (fixed height)
	mainLayout.AddItem(a.header, 3, 0, false)

	// Add pages container (flexible)
	mainLayout.AddItem(a.pages, 0, 1, true)

	// Add footer (fixed height)
	mainLayout.AddItem(a.footer, 3, 0, false)

	// Set up global input capture
	mainLayout.SetInputCapture(a.handleGlobalInput)

	// Set the main layout as root
	a.tviewApp.SetRoot(mainLayout, true)

	// Configure application settings
	a.tviewApp.EnableMouse(true)
	a.tviewApp.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		a.updateHeader()
		return false
	})

	return nil
}

// RegisterScreen registers a screen with the application
func (a *App) RegisterScreen(screenType ScreenType, screen Screen) error {
	if screen == nil {
		return fmt.Errorf("screen cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.screens[screenType] = screen
	a.pages.AddPage(screenType.String(), screen.GetPrimitive(), true, false)

	return nil
}

// NavigateTo switches to the specified screen
func (a *App) NavigateTo(screenType ScreenType) error {
	a.state.mu.Lock()

	screen, exists := a.screens[screenType]
	if !exists {
		a.state.mu.Unlock()
		return fmt.Errorf("screen %s not registered", screenType.String())
	}

	// Get current screen for exit
	currentScreen, hasCurrentScreen := a.screens[a.state.currentScreen]
	previousScreen := a.state.currentScreen

	a.state.mu.Unlock()

	// Exit current screen (without lock to avoid deadlock)
	if hasCurrentScreen {
		if err := currentScreen.OnExit(a); err != nil {
			return fmt.Errorf("failed to exit screen %s: %w", previousScreen.String(), err)
		}
	}

	// Update state (with lock)
	a.state.mu.Lock()
	a.state.previousScreen = a.state.currentScreen
	a.state.currentScreen = screenType
	a.state.mu.Unlock()

	// Enter new screen (without lock to avoid deadlock)
	if err := screen.OnEnter(a); err != nil {
		// Restore previous screen on error
		a.state.mu.Lock()
		a.state.currentScreen = a.state.previousScreen
		a.state.mu.Unlock()
		return fmt.Errorf("failed to enter screen %s: %w", screenType.String(), err)
	}

	// Show the page
	a.pages.SwitchToPage(screenType.String())

	return nil
}

// GoBack navigates to the previous screen or exits if at the first screen
func (a *App) GoBack() error {
	a.state.mu.RLock()
	current := a.state.currentScreen
	previous := a.state.previousScreen
	a.state.mu.RUnlock()

	// If we're at comparison screen (which is now the first screen), exit the application
	if current == ScreenComparison {
		return a.Exit()
	}

	// Navigate to previous screen
	return a.NavigateTo(previous)
}

// ShowHelp displays the help screen
func (a *App) ShowHelp() error {
	return a.NavigateTo(ScreenHelp)
}

// ShowRanking displays the ranking screen
func (a *App) ShowRanking() error {
	return a.NavigateTo(ScreenRanking)
}

// Exit stops the application
func (a *App) Exit() error {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()

	a.state.isRunning = false
	a.cancel()
	a.tviewApp.Stop()

	return nil
}

// Run starts the TUI application
func (a *App) Run() error {
	a.state.mu.Lock()
	a.state.isRunning = true
	a.state.mu.Unlock()

	// Always start with the comparison screen
	// The configuration is now handled via command-line parameters
	if err := a.NavigateTo(ScreenComparison); err != nil {
		return fmt.Errorf("failed to navigate to comparison screen: %w", err)
	}

	// Run the application
	return a.tviewApp.Run()
}

// Stop gracefully stops the application
func (a *App) Stop() {
	a.state.mu.RLock()
	running := a.state.isRunning
	a.state.mu.RUnlock()

	if running {
		a.Exit()
	}
}

// GetState returns a copy of the current application state
func (a *App) GetState() *AppState {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()

	return &AppState{
		session:        a.state.session,
		storage:        a.state.storage,
		config:         a.state.config,
		currentScreen:  a.state.currentScreen,
		previousScreen: a.state.previousScreen,
		isRunning:      a.state.isRunning,
	}
}

// SetSession updates the current session
func (a *App) SetSession(session *data.Session) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	a.state.session = session
}

// GetSession returns the current session
func (a *App) GetSession() *data.Session {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return a.state.session
}

// GetProposals returns all proposals from the current session
func (a *App) GetProposals() ([]data.Proposal, error) {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()

	if a.state.session == nil {
		return nil, fmt.Errorf("no active session")
	}

	return a.state.session.Proposals, nil
}

// GetStorage returns the storage interface
func (a *App) GetStorage() data.Storage {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return a.state.storage
}

// GetConfig returns the current configuration
func (a *App) GetConfig() *data.SessionConfig {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return a.state.config
}

// GetTViewApp returns the underlying tview application for advanced usage
func (a *App) GetTViewApp() *tview.Application {
	return a.tviewApp
}

// GetComparisonCount returns how many comparisons a specific proposal has participated in
func (a *App) GetComparisonCount(proposalID string) int {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()

	if a.state.session == nil {
		return 0
	}

	count := 0
	for _, comparison := range a.state.session.CompletedComparisons {
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

// handleGlobalInput handles global keyboard shortcuts
func (a *App) handleGlobalInput(event *tcell.EventKey) *tcell.EventKey {
	for _, binding := range globalKeyBindings {
		if (binding.Key != tcell.KeyRune && event.Key() == binding.Key) ||
			(binding.Key == tcell.KeyRune && event.Rune() == binding.Rune) {

			// Execute handler in a goroutine to prevent blocking
			go func(handler func(*App) error) {
				if err := handler(a); err != nil {
					// Log error or show notification
					// For now, we'll just continue
					_ = err
				}
			}(binding.Handler)

			return nil // Consume the event
		}
	}

	return event // Let other handlers process the event
}

// updateHeader updates the header text with current screen information
func (a *App) updateHeader() {
	a.state.mu.RLock()
	currentScreen := a.state.currentScreen
	session := a.state.session
	a.state.mu.RUnlock()

	screen, exists := a.screens[currentScreen]
	if !exists {
		return
	}

	title := screen.GetTitle()
	sessionInfo := ""

	if session != nil {
		sessionInfo = fmt.Sprintf(" | Session: %s (%s)", session.Name, session.Status)
	}

	headerText := fmt.Sprintf("Screen: %s%s", title, sessionInfo)
	a.header.SetText(headerText)
}

// updateFooter updates the footer with current key bindings
func (a *App) updateFooter() {
	helpText := ""
	for i, binding := range globalKeyBindings {
		if i > 0 {
			helpText += " | "
		}

		keyText := ""
		if binding.Key != tcell.KeyRune {
			keyText = tcell.KeyNames[binding.Key]
		} else {
			keyText = string(binding.Rune)
		}

		helpText += fmt.Sprintf("%s: %s", keyText, binding.Description)
	}

	a.footer.SetText(helpText)
}

// IsRunning returns whether the application is currently running
func (a *App) IsRunning() bool {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return a.state.isRunning
}

// GetCurrentScreen returns the current screen type
func (a *App) GetCurrentScreen() ScreenType {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return a.state.currentScreen
}

// LoadCsvAndStartSession loads a CSV file, creates a session, and navigates to comparison
func (a *App) LoadCsvAndStartSession(csvPath string, config data.SessionConfig) error {
	// Load proposals from CSV
	parseResult, err := a.state.storage.LoadProposalsFromCSV(csvPath, config.CSV)
	if err != nil {
		return fmt.Errorf("failed to load CSV: %w", err)
	}

	// Create new session with loaded proposals
	session := &data.Session{
		ID:        fmt.Sprintf("session_%d", time.Now().Unix()),
		Name:      fmt.Sprintf("Session %s", time.Now().Format("2006-01-02 15:04")),
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Proposals: parseResult.Proposals,
	}

	// Update app state
	a.SetSession(session)

	// Update configuration
	a.state.mu.Lock()
	a.state.config = &config
	a.state.mu.Unlock()

	// Navigate to comparison screen
	return a.NavigateTo(ScreenComparison)
}
