// Package tui provides Terminal User Interface functionality for conference talk ranking.
// This file contains comprehensive tests for the TUI framework functionality.
package tui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pashagolub/confelo/pkg/data"
)

// mockStorage is a mock implementation of data.Storage for testing
type mockStorage struct {
	sessions  map[string]*data.Session
	proposals map[string][]data.Proposal
	loadError error
	saveError error
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		sessions:  make(map[string]*data.Session),
		proposals: make(map[string][]data.Proposal),
	}
}

func (m *mockStorage) LoadProposalsFromCSV(filename string, config data.CSVConfig) (*data.CSVParseResult, error) {
	if m.loadError != nil {
		return nil, m.loadError
	}

	proposals, exists := m.proposals[filename]
	if !exists {
		proposals = []data.Proposal{}
	}

	return &data.CSVParseResult{
		Proposals:      proposals,
		TotalRows:      len(proposals),
		SuccessfulRows: len(proposals),
		SkippedRows:    []int{},
		ParseErrors:    []data.CSVParseError{},
	}, nil
}

func (m *mockStorage) ExportProposalsToCSV(proposals []data.Proposal, filename string, config data.CSVConfig, exportConfig data.ExportConfig) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.proposals[filename] = proposals
	return nil
}

func (m *mockStorage) SaveSession(session *data.Session, filename string) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.sessions[filename] = session
	return nil
}

func (m *mockStorage) LoadSession(filename string) (*data.Session, error) {
	if m.loadError != nil {
		return nil, m.loadError
	}

	session, exists := m.sessions[filename]
	if !exists {
		return nil, data.ErrSessionNotFound
	}

	return session, nil
}

func (m *mockStorage) CreateBackup(filename string) (string, error) {
	if m.saveError != nil {
		return "", m.saveError
	}
	return filename + ".bak", nil
}

func (m *mockStorage) RecoverFromBackup(filename string) error {
	return m.loadError
}

func (m *mockStorage) RotateBackups(basePath string, maxBackups int) error {
	return m.saveError
}

// mockScreen is a mock implementation of Screen interface for testing
type mockScreen struct {
	title       string
	helpText    []string
	primitive   *testPrimitive
	onEnterFunc func(app interface{}) error
	onExitFunc  func(app interface{}) error
}

type testPrimitive struct {
	// Simple test primitive that implements tview.Primitive
}

func (tp *testPrimitive) Draw(screen tcell.Screen)        {}
func (tp *testPrimitive) GetRect() (int, int, int, int)   { return 0, 0, 0, 0 }
func (tp *testPrimitive) SetRect(x, y, width, height int) {}
func (tp *testPrimitive) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return nil
}
func (tp *testPrimitive) Focus(delegate func(p tview.Primitive)) {}
func (tp *testPrimitive) Blur()                                  {}
func (tp *testPrimitive) HasFocus() bool                         { return false }
func (tp *testPrimitive) PasteHandler() func(pastedText string, setFocus func(p tview.Primitive)) {
	return nil
}
func (tp *testPrimitive) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return nil
}

func newMockScreen(title string) *mockScreen {
	return &mockScreen{
		title:     title,
		helpText:  []string{"Test help text"},
		primitive: &testPrimitive{},
	}
}

func (ms *mockScreen) GetPrimitive() tview.Primitive {
	return ms.primitive
}

func (ms *mockScreen) OnEnter(app interface{}) error {
	if ms.onEnterFunc != nil {
		return ms.onEnterFunc(app)
	}
	return nil
}

func (ms *mockScreen) OnExit(app interface{}) error {
	if ms.onExitFunc != nil {
		return ms.onExitFunc(app)
	}
	return nil
}

func (ms *mockScreen) GetTitle() string {
	return ms.title
}

func (ms *mockScreen) GetHelpText() []string {
	return ms.helpText
}

// createTestConfig returns a valid test configuration
func createTestConfig() *data.SessionConfig {
	return &data.SessionConfig{
		CSV: data.CSVConfig{
			Delimiter:      ",",
			HasHeader:      true,
			TitleColumn:    "Title",
			SpeakerColumn:  "Speaker",
			IDColumn:       "ID",
			AbstractColumn: "Abstract",
		},
		Elo: data.EloConfig{
			KFactor:       32,
			InitialRating: 1500.0,
			MinRating:     800.0,
			MaxRating:     2200.0,
		},
		UI: data.UIConfig{
			ComparisonMode: "pairwise",
			ShowProgress:   true,
		},
	}
}

func TestNewApp(t *testing.T) {
	tests := []struct {
		name        string
		config      *data.SessionConfig
		storage     data.Storage
		expectError bool
	}{
		{
			name:        "valid configuration",
			config:      createTestConfig(),
			storage:     newMockStorage(),
			expectError: false,
		},
		{
			name:        "nil config",
			config:      nil,
			storage:     newMockStorage(),
			expectError: true,
		},
		{
			name:        "nil storage",
			config:      createTestConfig(),
			storage:     nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := NewApp(tt.config, tt.storage)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, app)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, app)
				assert.NotNil(t, app.tviewApp)
				assert.NotNil(t, app.pages)
				assert.NotNil(t, app.state)
				assert.Equal(t, ScreenComparison, app.state.currentScreen)
			}
		})
	}
}

func TestAppScreenRegistration(t *testing.T) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(t, err)

	// Test registering a valid screen
	mockScreen := newMockScreen("Test Screen")
	err = app.RegisterScreen(ScreenComparison, mockScreen)
	assert.NoError(t, err)

	// Verify screen is registered
	assert.Contains(t, app.screens, ScreenComparison)

	// Test registering nil screen
	err = app.RegisterScreen(ScreenRanking, nil)
	assert.Error(t, err)
}

func TestAppNavigation(t *testing.T) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(t, err)

	// Register test screens
	comparisonScreen := newMockScreen("Comparison")

	// SetupScreen has been removed

	err = app.RegisterScreen(ScreenComparison, comparisonScreen)
	require.NoError(t, err)

	// Test navigation to registered screen
	err = app.NavigateTo(ScreenComparison)
	assert.NoError(t, err)
	assert.Equal(t, ScreenComparison, app.GetCurrentScreen())

	// Test navigation to unregistered screen
	err = app.NavigateTo(ScreenRanking)
	assert.Error(t, err)

	// Test going back - this should now set to ScreenComparison
	err = app.GoBack()
	assert.NoError(t, err)
	assert.Equal(t, ScreenComparison, app.GetCurrentScreen())
}

func TestAppState(t *testing.T) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(t, err)

	// Test initial state
	state := app.GetState()
	assert.NotNil(t, state)
	assert.Equal(t, ScreenComparison, state.currentScreen)
	assert.False(t, state.isRunning)

	// Test session management
	session := &data.Session{
		Name:   "Test Session",
		Status: data.StatusActive,
	}

	app.SetSession(session)
	retrievedSession := app.GetSession()
	assert.Equal(t, session, retrievedSession)

	// Test config and storage access
	assert.Equal(t, config, app.GetConfig())
	assert.Equal(t, storage, app.GetStorage())
}

func TestAppKeyBindings(t *testing.T) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(t, err)

	// Register test screens including help
	comparisonScreen := newMockScreen("Comparison")

	err = app.RegisterScreen(ScreenComparison, comparisonScreen)
	require.NoError(t, err)

	// Set initial screen
	err = app.NavigateTo(ScreenComparison)
	require.NoError(t, err)

	// Test help key binding (F1)
	event := tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone)
	result := app.handleGlobalInput(event)
	assert.Nil(t, result) // Event should be consumed

	// Give time for goroutine to execute
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, ScreenHelp, app.GetCurrentScreen())

	// Test manual exit instead of relying on Ctrl+C key binding
	app.state.isRunning = true
	assert.True(t, app.IsRunning())

	// Call Exit directly
	err = app.Exit()
	assert.NoError(t, err)
	assert.False(t, app.IsRunning())
}

func TestAppScreenCallbacks(t *testing.T) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(t, err)

	// Create mock screen with callbacks
	enterCalled := false
	exitCalled := false

	mockScreen := newMockScreen("Test")
	mockScreen.onEnterFunc = func(app interface{}) error {
		enterCalled = true
		return nil
	}
	mockScreen.onExitFunc = func(app interface{}) error {
		exitCalled = true
		return nil
	}

	err = app.RegisterScreen(ScreenComparison, mockScreen)
	require.NoError(t, err)

	// Navigate to screen - should call OnEnter
	err = app.NavigateTo(ScreenComparison)
	assert.NoError(t, err)
	assert.True(t, enterCalled)

	// Navigate away - should call OnExit
	helpScreen := newMockScreen("Help")
	err = app.RegisterScreen(ScreenHelp, helpScreen)
	require.NoError(t, err)

	err = app.NavigateTo(ScreenHelp)
	assert.NoError(t, err)
	assert.True(t, exitCalled)
}

func TestAppErrorHandling(t *testing.T) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(t, err)

	// Test screen with OnEnter error
	mockScreen := newMockScreen("Error Screen")
	mockScreen.onEnterFunc = func(app interface{}) error {
		return assert.AnError
	}

	err = app.RegisterScreen(ScreenComparison, mockScreen)
	require.NoError(t, err)

	// Navigation should fail and maintain current screen
	originalScreen := app.GetCurrentScreen()
	err = app.NavigateTo(ScreenComparison)
	assert.Error(t, err)
	assert.Equal(t, originalScreen, app.GetCurrentScreen())
}

func TestAppConcurrency(t *testing.T) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(t, err)

	// Test concurrent access to state
	done := make(chan bool)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = app.GetState()
			_ = app.GetCurrentScreen()
			_ = app.IsRunning()
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		session := &data.Session{
			Name:   "Concurrent Session",
			Status: data.StatusActive,
		}
		for i := 0; i < 100; i++ {
			app.SetSession(session)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify state is still consistent
	assert.NotNil(t, app.GetState())
}

func TestScreenTypeString(t *testing.T) {
	tests := []struct {
		screen   ScreenType
		expected string
	}{
		// ScreenSetup is removed
		{ScreenComparison, "comparison"},
		{ScreenRanking, "ranking"},
		{ScreenHelp, "help"},
		{ScreenType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.screen.String())
		})
	}
}

func TestAppCleanup(t *testing.T) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(t, err)

	// Start the app state
	app.state.isRunning = true

	// Test cleanup
	app.Stop()
	assert.False(t, app.IsRunning())

	// Multiple stops should be safe
	app.Stop()
	assert.False(t, app.IsRunning())
}

// Benchmark tests
func BenchmarkAppNavigation(b *testing.B) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(b, err)

	// Register screens
	comparisonScreen := newMockScreen("Comparison")
	rankingScreen := newMockScreen("Ranking")

	_ = app.RegisterScreen(ScreenComparison, comparisonScreen)
	_ = app.RegisterScreen(ScreenRanking, rankingScreen)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = app.NavigateTo(ScreenComparison)
		_ = app.NavigateTo(ScreenRanking)
	}
}

func BenchmarkAppStateAccess(b *testing.B) {
	config := createTestConfig()
	storage := newMockStorage()

	app, err := NewApp(config, storage)
	require.NoError(b, err)

	session := &data.Session{
		Name:   "Benchmark Session",
		Status: data.StatusActive,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		app.SetSession(session)
		_ = app.GetSession()
		_ = app.GetState()
	}
}
