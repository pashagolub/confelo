// Package screens provides TUI screen implementations for conference talk ranking.
// This file implements the setup and configuration screen where users select CSV files,
// configure Elo parameters, and validate settings before starting a ranking session.
package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/pashagolub/confelo/pkg/data"
)

// SetupScreen implements the initial configuration interface
type SetupScreen struct {
	// UI components
	container     *tview.Flex
	leftPanel     *tview.Flex
	rightPanel    *tview.Flex
	filePanel     *tview.Form
	configPanel   *tview.Form
	previewPanel  *tview.TextView
	statusBar     *tview.TextView
	actionButtons *tview.Flex

	// Configuration state
	config       data.SessionConfig
	selectedFile string
	isValid      bool

	// App reference - we'll use interface{} and cast as needed
	app interface{}
}

// NewSetupScreen creates a new setup screen instance
func NewSetupScreen() *SetupScreen {
	ss := &SetupScreen{
		container:     tview.NewFlex(),
		leftPanel:     tview.NewFlex(),
		rightPanel:    tview.NewFlex(),
		filePanel:     tview.NewForm(),
		configPanel:   tview.NewForm(),
		previewPanel:  tview.NewTextView(),
		statusBar:     tview.NewTextView(),
		actionButtons: tview.NewFlex(),
		config:        data.DefaultSessionConfig(),
		isValid:       false,
	}

	ss.setupUI()
	return ss
}

// setupUI initializes the setup screen layout
func (ss *SetupScreen) setupUI() {
	// Configure main container as horizontal split
	ss.container.SetDirection(tview.FlexColumn)

	// Setup left panel for file selection and CSV config
	ss.leftPanel.SetDirection(tview.FlexRow).
		SetBorder(true).
		SetTitle("File and CSV Configuration").
		SetBorderColor(tcell.ColorBlue)

	// Setup right panel for Elo config and preview
	ss.rightPanel.SetDirection(tview.FlexRow).
		SetBorder(true).
		SetTitle("Elo Configuration and Preview").
		SetBorderColor(tcell.ColorGreen)

	// Setup file selection panel
	ss.setupFilePanel()

	// Setup configuration panel
	ss.setupConfigPanel()

	// Setup preview panel
	ss.setupPreviewPanel()

	// Setup action buttons
	ss.setupActionButtons()

	// Setup status bar
	ss.setupStatusBar()

	// Assemble the layout
	ss.leftPanel.
		AddItem(ss.filePanel, 0, 1, true).
		AddItem(ss.configPanel, 0, 1, false)

	ss.rightPanel.
		AddItem(ss.previewPanel, 0, 2, false).
		AddItem(ss.actionButtons, 3, 0, false)

	ss.container.
		AddItem(ss.leftPanel, 0, 1, true).
		AddItem(ss.rightPanel, 0, 1, false).
		AddItem(ss.statusBar, 1, 0, false)
}

// setupFilePanel creates the file selection interface
func (ss *SetupScreen) setupFilePanel() {
	ss.filePanel.
		SetBorder(true).
		SetTitle("CSV File Selection").
		SetBorderColor(tcell.ColorBlue)

	// File path input
	ss.filePanel.AddInputField("File Path", "", 50, nil, ss.onFilePathChanged)

	// Browse button (simulated with dropdown showing common paths)
	workDir, _ := os.Getwd()
	commonPaths := []string{
		workDir,
		filepath.Join(workDir, "testdata"),
		filepath.Join(workDir, "data"),
		"Browse...",
	}

	ss.filePanel.AddDropDown("Quick Select", commonPaths, 0, ss.onQuickSelectChanged)

	// CSV format options
	ss.filePanel.AddCheckbox("Has Header Row", ss.config.CSV.HasHeader, ss.onHeaderChanged)

	delimiters := []string{",", ";", "\t", "|"}
	delimiterIndex := 0
	for i, d := range delimiters {
		if d == ss.config.CSV.Delimiter {
			delimiterIndex = i
			break
		}
	}
	ss.filePanel.AddDropDown("Delimiter", delimiters, delimiterIndex, ss.onDelimiterChanged)
}

// setupConfigPanel creates the Elo configuration interface
func (ss *SetupScreen) setupConfigPanel() {
	ss.configPanel.
		SetBorder(true).
		SetTitle("Column Mapping").
		SetBorderColor(tcell.ColorYellow)

	// Column mapping inputs
	ss.configPanel.AddInputField("ID Column", ss.config.CSV.IDColumn, 20, nil, ss.onIDColumnChanged)
	ss.configPanel.AddInputField("Title Column", ss.config.CSV.TitleColumn, 20, nil, ss.onTitleColumnChanged)
	ss.configPanel.AddInputField("Abstract Column", ss.config.CSV.AbstractColumn, 20, nil, ss.onAbstractColumnChanged)
	ss.configPanel.AddInputField("Speaker Column", ss.config.CSV.SpeakerColumn, 20, nil, ss.onSpeakerColumnChanged)
	ss.configPanel.AddInputField("Score Column", ss.config.CSV.ScoreColumn, 20, nil, ss.onScoreColumnChanged)
}

// setupPreviewPanel creates the configuration preview display
func (ss *SetupScreen) setupPreviewPanel() {
	ss.previewPanel.
		SetBorder(true).
		SetTitle("Configuration Preview").
		SetBorderColor(tcell.ColorRed)

	ss.updatePreview()
}

// setupActionButtons creates the action button panel
func (ss *SetupScreen) setupActionButtons() {
	ss.actionButtons.SetDirection(tview.FlexColumn)

	// Test Configuration button
	testBtn := tview.NewButton("Test Config").
		SetSelectedFunc(ss.onTestConfig)
	testBtn.SetBorder(true).SetBorderColor(tcell.ColorOrange)

	// Start Session button
	startBtn := tview.NewButton("Start Session").
		SetSelectedFunc(ss.onStartSession)
	startBtn.SetBorder(true).SetBorderColor(tcell.ColorGreen)

	// Reset button
	resetBtn := tview.NewButton("Reset").
		SetSelectedFunc(ss.onReset)
	resetBtn.SetBorder(true).SetBorderColor(tcell.ColorRed)

	ss.actionButtons.
		AddItem(testBtn, 0, 1, false).
		AddItem(startBtn, 0, 1, false).
		AddItem(resetBtn, 0, 1, false)
}

// setupStatusBar creates the status display
func (ss *SetupScreen) setupStatusBar() {
	ss.statusBar.
		SetBorder(true).
		SetTitle("Status").
		SetBorderColor(tcell.ColorWhite)

	ss.updateStatus("Ready to configure. Select a CSV file to begin.")
}

// Event handlers

// onFilePathChanged handles file path input changes
func (ss *SetupScreen) onFilePathChanged(text string) {
	ss.selectedFile = text
	ss.validateFile()
	ss.updatePreview()
}

// onQuickSelectChanged handles quick select dropdown changes
func (ss *SetupScreen) onQuickSelectChanged(option string, index int) {
	if option == "Browse..." {
		// In a real implementation, this would open a file browser
		ss.updateStatus("File browser not implemented. Enter path manually.")
		return
	}

	if index >= 0 {
		// Look for CSV files in the selected directory
		files, err := filepath.Glob(filepath.Join(option, "*.csv"))
		if err == nil && len(files) > 0 {
			ss.selectedFile = files[0] // Select first CSV file found
			// Update the file path input field
			ss.filePanel.GetFormItemByLabel("File Path").(*tview.InputField).SetText(ss.selectedFile)
			ss.validateFile()
		}
	}
	ss.updatePreview()
}

// onHeaderChanged handles header checkbox changes
func (ss *SetupScreen) onHeaderChanged(checked bool) {
	ss.config.CSV.HasHeader = checked
	ss.updatePreview()
}

// onDelimiterChanged handles delimiter selection changes
func (ss *SetupScreen) onDelimiterChanged(option string, index int) {
	ss.config.CSV.Delimiter = option
	ss.updatePreview()
}

// onIDColumnChanged handles ID column name changes
func (ss *SetupScreen) onIDColumnChanged(text string) {
	ss.config.CSV.IDColumn = text
	ss.updatePreview()
}

// onTitleColumnChanged handles title column name changes
func (ss *SetupScreen) onTitleColumnChanged(text string) {
	ss.config.CSV.TitleColumn = text
	ss.updatePreview()
}

// onAbstractColumnChanged handles abstract column name changes
func (ss *SetupScreen) onAbstractColumnChanged(text string) {
	ss.config.CSV.AbstractColumn = text
	ss.updatePreview()
}

// onSpeakerColumnChanged handles speaker column name changes
func (ss *SetupScreen) onSpeakerColumnChanged(text string) {
	ss.config.CSV.SpeakerColumn = text
	ss.updatePreview()
}

// onScoreColumnChanged handles score column name changes
func (ss *SetupScreen) onScoreColumnChanged(text string) {
	ss.config.CSV.ScoreColumn = text
	ss.updatePreview()
}

// onTestConfig handles configuration testing
func (ss *SetupScreen) onTestConfig() {
	if ss.selectedFile == "" {
		ss.updateStatus("Error: No file selected")
		return
	}

	// Test file reading and CSV parsing
	storage := data.NewFileStorage("./tmp")
	result, err := storage.LoadProposalsFromCSV(ss.selectedFile, ss.config.CSV)
	if err != nil {
		ss.updateStatus(fmt.Sprintf("Error: %v", err))
		return
	}

	ss.isValid = true
	ss.updateStatus(fmt.Sprintf("Success: Loaded %d proposals", len(result.Proposals)))
	ss.updatePreview()
}

// onStartSession handles session start
func (ss *SetupScreen) onStartSession() {
	if !ss.isValid {
		ss.updateStatus("Error: Configuration not validated. Test configuration first.")
		return
	}

	if ss.selectedFile == "" {
		ss.updateStatus("Error: No file selected")
		return
	}

	// Signal to main app to start comparison session
	ss.updateStatus("Starting session...")

	// Cast app to TUI App and start session
	if ss.app != nil {
		// We need to add an import and cast properly
		if tuiApp, ok := ss.app.(interface {
			LoadCsvAndStartSession(string, data.SessionConfig) error
		}); ok {
			err := tuiApp.LoadCsvAndStartSession(ss.selectedFile, ss.config)
			if err != nil {
				ss.updateStatus(fmt.Sprintf("Error starting session: %v", err))
				return
			}
			ss.updateStatus("Session started successfully")
		} else {
			ss.updateStatus("Error: Invalid app instance")
		}
	}
}

// onReset handles configuration reset
func (ss *SetupScreen) onReset() {
	ss.config = data.DefaultSessionConfig()
	ss.selectedFile = ""
	ss.isValid = false

	// Reset form fields
	ss.filePanel.GetFormItemByLabel("File Path").(*tview.InputField).SetText("")
	ss.filePanel.GetFormItemByLabel("Has Header Row").(*tview.Checkbox).SetChecked(ss.config.CSV.HasHeader)

	ss.configPanel.GetFormItemByLabel("ID Column").(*tview.InputField).SetText(ss.config.CSV.IDColumn)
	ss.configPanel.GetFormItemByLabel("Title Column").(*tview.InputField).SetText(ss.config.CSV.TitleColumn)
	ss.configPanel.GetFormItemByLabel("Abstract Column").(*tview.InputField).SetText(ss.config.CSV.AbstractColumn)
	ss.configPanel.GetFormItemByLabel("Speaker Column").(*tview.InputField).SetText(ss.config.CSV.SpeakerColumn)
	ss.configPanel.GetFormItemByLabel("Score Column").(*tview.InputField).SetText(ss.config.CSV.ScoreColumn)

	ss.updateStatus("Configuration reset to defaults")
	ss.updatePreview()
}

// Helper methods

// validateFile checks if the selected file exists and is readable
func (ss *SetupScreen) validateFile() {
	if ss.selectedFile == "" {
		return
	}

	_, err := os.Stat(ss.selectedFile)
	if err != nil {
		ss.updateStatus(fmt.Sprintf("File error: %v", err))
		ss.isValid = false
		return
	}

	ss.updateStatus("File found. Test configuration to validate CSV format.")
}

// updatePreview refreshes the configuration preview display
func (ss *SetupScreen) updatePreview() {
	var preview strings.Builder

	preview.WriteString("[yellow]File Configuration:[white]\n")
	preview.WriteString(fmt.Sprintf("File: %s\n", ss.selectedFile))
	preview.WriteString(fmt.Sprintf("Has Header: %t\n", ss.config.CSV.HasHeader))
	preview.WriteString(fmt.Sprintf("Delimiter: %q\n\n", ss.config.CSV.Delimiter))

	preview.WriteString("[yellow]Column Mapping:[white]\n")
	preview.WriteString(fmt.Sprintf("ID Column: %s\n", ss.config.CSV.IDColumn))
	preview.WriteString(fmt.Sprintf("Title Column: %s\n", ss.config.CSV.TitleColumn))
	preview.WriteString(fmt.Sprintf("Abstract Column: %s\n", ss.config.CSV.AbstractColumn))
	preview.WriteString(fmt.Sprintf("Speaker Column: %s\n", ss.config.CSV.SpeakerColumn))
	preview.WriteString(fmt.Sprintf("Score Column: %s\n\n", ss.config.CSV.ScoreColumn))

	preview.WriteString("[yellow]Elo Configuration:[white]\n")
	preview.WriteString(fmt.Sprintf("Initial Rating: %.0f\n", ss.config.Elo.InitialRating))
	preview.WriteString(fmt.Sprintf("K-Factor: %d\n", ss.config.Elo.KFactor))
	preview.WriteString(fmt.Sprintf("Min Rating: %.0f\n", ss.config.Elo.MinRating))
	preview.WriteString(fmt.Sprintf("Max Rating: %.0f\n\n", ss.config.Elo.MaxRating))

	if ss.isValid {
		preview.WriteString("[green]✓ Configuration validated successfully[white]\n")
	} else {
		preview.WriteString("[red]⚠ Configuration not yet validated[white]\n")
	}

	ss.previewPanel.SetText(preview.String())
}

// updateStatus updates the status bar message
func (ss *SetupScreen) updateStatus(message string) {
	ss.statusBar.SetText(message)
}

// Screen interface implementation

// GetPrimitive returns the root primitive for this screen
func (ss *SetupScreen) GetPrimitive() tview.Primitive {
	return ss.container
}

// OnEnter is called when the screen becomes active
func (ss *SetupScreen) OnEnter(app interface{}) error {
	ss.app = app
	ss.updateStatus("Setup screen active. Configure CSV file and parameters.")
	return nil
}

// OnExit is called when the screen is deactivated
func (ss *SetupScreen) OnExit(app interface{}) error {
	return nil
}

// SetInputCapture sets the input capture function for keyboard shortcuts
func (ss *SetupScreen) SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) {
	ss.container.SetInputCapture(capture)
}

// GetTitle returns the screen title for navigation
func (ss *SetupScreen) GetTitle() string {
	return "Setup & Configuration"
}

// GetHelpText returns help text specific to this screen
func (ss *SetupScreen) GetHelpText() []string {
	return []string{
		"File Selection:",
		"  Tab / Shift+Tab  Navigate between form fields",
		"  Enter           Select dropdown options",
		"  Ctrl+C / Esc    Exit application",
		"",
		"Configuration:",
		"  Test Config     Validate CSV file and settings",
		"  Start Session   Begin ranking with current configuration",
		"  Reset           Restore default settings",
		"",
		"Required Steps:",
		"  1. Select or enter CSV file path",
		"  2. Configure column mapping if needed",
		"  3. Test configuration to validate",
		"  4. Start session to begin ranking",
	}
}

// Public methods for external access

// GetConfig returns the current configuration
func (ss *SetupScreen) GetConfig() data.SessionConfig {
	return ss.config
}

// GetSelectedFile returns the currently selected file path
func (ss *SetupScreen) GetSelectedFile() string {
	return ss.selectedFile
}

// IsValid returns whether the current configuration is validated
func (ss *SetupScreen) IsValid() bool {
	return ss.isValid
}

// SetConfig updates the configuration (useful for loading saved settings)
func (ss *SetupScreen) SetConfig(config data.SessionConfig) {
	ss.config = config
	ss.updatePreview()
}

// SetSelectedFile sets the file path programmatically
func (ss *SetupScreen) SetSelectedFile(path string) {
	ss.selectedFile = path
	if ss.filePanel != nil {
		ss.filePanel.GetFormItemByLabel("File Path").(*tview.InputField).SetText(path)
	}
	ss.validateFile()
	ss.updatePreview()
}
