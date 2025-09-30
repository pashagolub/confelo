package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pashagolub/confelo/pkg/data"
)

// TestNewSetupScreen verifies setup screen initialization
func TestNewSetupScreen(t *testing.T) {
	screen := NewSetupScreen()

	assert.NotNil(t, screen)
	assert.NotNil(t, screen.container)
	assert.NotNil(t, screen.leftPanel)
	assert.NotNil(t, screen.rightPanel)
	assert.NotNil(t, screen.filePanel)
	assert.NotNil(t, screen.configPanel)
	assert.NotNil(t, screen.previewPanel)
	assert.NotNil(t, screen.statusBar)
	assert.NotNil(t, screen.actionButtons)

	// Check default configuration
	defaultConfig := data.DefaultSessionConfig()
	assert.Equal(t, defaultConfig.CSV.IDColumn, screen.config.CSV.IDColumn)
	assert.Equal(t, defaultConfig.CSV.TitleColumn, screen.config.CSV.TitleColumn)
	assert.Equal(t, defaultConfig.CSV.HasHeader, screen.config.CSV.HasHeader)
	assert.Equal(t, defaultConfig.CSV.Delimiter, screen.config.CSV.Delimiter)

	// Check initial state
	assert.Empty(t, screen.selectedFile)
	assert.False(t, screen.isValid)
}

// TestGetPrimitive verifies the screen interface implementation
func TestGetPrimitive(t *testing.T) {
	screen := NewSetupScreen()
	primitive := screen.GetPrimitive()

	assert.NotNil(t, primitive)
	assert.IsType(t, &tview.Flex{}, primitive)
}

// TestOnEnterExit verifies screen lifecycle methods
func TestOnEnterExit(t *testing.T) {
	screen := NewSetupScreen()

	// Test OnEnter
	mockApp := &MockApp{}
	err := screen.OnEnter(mockApp)
	assert.NoError(t, err)
	assert.Equal(t, mockApp, screen.app)

	// Test OnExit
	err = screen.OnExit(mockApp)
	assert.NoError(t, err)
}

// TestGetTitle verifies screen title
func TestGetTitle(t *testing.T) {
	screen := NewSetupScreen()
	title := screen.GetTitle()

	assert.Equal(t, "Setup & Configuration", title)
}

// TestGetHelpText verifies help text content
func TestGetHelpText(t *testing.T) {
	screen := NewSetupScreen()
	helpText := screen.GetHelpText()

	assert.NotEmpty(t, helpText)
	assert.Contains(t, strings.Join(helpText, " "), "File Selection")
	assert.Contains(t, strings.Join(helpText, " "), "Configuration")
	assert.Contains(t, strings.Join(helpText, " "), "Required Steps")
}

// TestConfigurationMethods verifies public configuration methods
func TestConfigurationMethods(t *testing.T) {
	screen := NewSetupScreen()

	// Test initial state
	config := screen.GetConfig()
	assert.Equal(t, data.DefaultSessionConfig(), config)
	assert.Empty(t, screen.GetSelectedFile())
	assert.False(t, screen.IsValid())

	// Test SetConfig
	customConfig := data.DefaultSessionConfig()
	customConfig.CSV.IDColumn = "proposal_id"
	customConfig.Elo.InitialRating = 1600

	screen.SetConfig(customConfig)
	updatedConfig := screen.GetConfig()
	assert.Equal(t, "proposal_id", updatedConfig.CSV.IDColumn)
	assert.Equal(t, float64(1600), updatedConfig.Elo.InitialRating)

	// Test SetSelectedFile
	testFile := "/path/to/test.csv"
	screen.SetSelectedFile(testFile)
	assert.Equal(t, testFile, screen.GetSelectedFile())
}

// TestFileValidation tests file validation logic
func TestFileValidation(t *testing.T) {
	screen := NewSetupScreen()

	// Test with non-existent file
	screen.selectedFile = "/non/existent/file.csv"
	screen.validateFile()
	assert.False(t, screen.isValid)

	// Test with existing file (create a temporary file)
	tempDir, err := os.MkdirTemp("", "confelo_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "test.csv")
	err = os.WriteFile(tempFile, []byte("id,title,speaker\n1,Test Talk,John Doe"), 0644)
	require.NoError(t, err)

	screen.selectedFile = tempFile
	screen.validateFile()
	// File exists but not validated yet (need to run test config)
	assert.False(t, screen.isValid)
}

// TestFilePathChanged tests file path change handling
func TestFilePathChanged(t *testing.T) {
	screen := NewSetupScreen()

	// Test with empty path
	screen.onFilePathChanged("")
	assert.Empty(t, screen.selectedFile)

	// Test with valid path
	testPath := "/path/to/proposals.csv"
	screen.onFilePathChanged(testPath)
	assert.Equal(t, testPath, screen.selectedFile)
}

// TestCSVConfigurationChanges tests CSV configuration handlers
func TestCSVConfigurationChanges(t *testing.T) {
	screen := NewSetupScreen()

	// Test header checkbox
	screen.onHeaderChanged(false)
	assert.False(t, screen.config.CSV.HasHeader)

	screen.onHeaderChanged(true)
	assert.True(t, screen.config.CSV.HasHeader)

	// Test delimiter change
	screen.onDelimiterChanged(";", 1)
	assert.Equal(t, ";", screen.config.CSV.Delimiter)

	// Test column name changes
	screen.onIDColumnChanged("proposal_id")
	assert.Equal(t, "proposal_id", screen.config.CSV.IDColumn)

	screen.onTitleColumnChanged("talk_title")
	assert.Equal(t, "talk_title", screen.config.CSV.TitleColumn)

	screen.onAbstractColumnChanged("description")
	assert.Equal(t, "description", screen.config.CSV.AbstractColumn)

	screen.onSpeakerColumnChanged("presenter")
	assert.Equal(t, "presenter", screen.config.CSV.SpeakerColumn)

	screen.onScoreColumnChanged("rating")
	assert.Equal(t, "rating", screen.config.CSV.ScoreColumn)
}

// TestQuickSelectChanged tests quick file selection
func TestQuickSelectChanged(t *testing.T) {
	screen := NewSetupScreen()

	// Test "Browse..." option
	screen.onQuickSelectChanged("Browse...", 3)
	// Should not change selected file
	assert.Empty(t, screen.selectedFile)

	// Test directory selection (create temporary directory with CSV)
	tempDir, err := os.MkdirTemp("", "confelo_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	csvFile := filepath.Join(tempDir, "test.csv")
	err = os.WriteFile(csvFile, []byte("id,title,speaker\n1,Test,John"), 0644)
	require.NoError(t, err)

	screen.onQuickSelectChanged(tempDir, 0)
	assert.Equal(t, csvFile, screen.selectedFile)
}

// TestConfigurationReset tests the reset functionality
func TestConfigurationReset(t *testing.T) {
	screen := NewSetupScreen()

	// Modify configuration
	screen.selectedFile = "/some/file.csv"
	screen.isValid = true
	screen.config.CSV.IDColumn = "custom_id"
	screen.config.CSV.HasHeader = false

	// Reset configuration
	screen.onReset()

	// Verify reset state
	assert.Empty(t, screen.selectedFile)
	assert.False(t, screen.isValid)

	defaultConfig := data.DefaultSessionConfig()
	assert.Equal(t, defaultConfig.CSV.IDColumn, screen.config.CSV.IDColumn)
	assert.Equal(t, defaultConfig.CSV.HasHeader, screen.config.CSV.HasHeader)
}

// TestPreviewUpdate tests the configuration preview functionality
func TestPreviewUpdate(t *testing.T) {
	screen := NewSetupScreen()

	// Set some configuration
	screen.selectedFile = "/path/to/test.csv"
	screen.config.CSV.IDColumn = "proposal_id"
	screen.config.CSV.HasHeader = true
	screen.config.CSV.Delimiter = ";"

	// Update preview
	screen.updatePreview()

	// Verify preview text contains expected information
	previewText := screen.previewPanel.GetText(false)
	assert.Contains(t, previewText, "/path/to/test.csv")
	assert.Contains(t, previewText, "proposal_id")
	assert.Contains(t, previewText, "true")
	assert.Contains(t, previewText, "\";\"")
}

// TestStatusUpdate tests status bar updates
func TestStatusUpdate(t *testing.T) {
	screen := NewSetupScreen()

	testMessage := "Test status message"
	screen.updateStatus(testMessage)

	statusText := screen.statusBar.GetText(false)
	assert.Contains(t, statusText, testMessage)
}

// TestSetInputCapture tests input capture functionality
func TestSetInputCapture(t *testing.T) {
	screen := NewSetupScreen()

	// Create a test capture function
	captureFunc := func(event *tcell.EventKey) *tcell.EventKey {
		return event
	}

	// Should not panic
	assert.NotPanics(t, func() {
		screen.SetInputCapture(captureFunc)
	})
}

// TestConfigurationValidationWithRealCSV tests the test configuration functionality
func TestConfigurationValidationWithRealCSV(t *testing.T) {
	screen := NewSetupScreen()

	// Create a temporary CSV file with proper format
	tempDir, err := os.MkdirTemp("", "confelo_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	csvContent := `id,title,speaker,abstract
1,"Introduction to Go","John Doe","A great talk about Go programming"
2,"Advanced TUI","Jane Smith","Building terminal user interfaces"
3,"Testing Strategies","Bob Johnson","How to write effective tests"`

	csvFile := filepath.Join(tempDir, "proposals.csv")
	err = os.WriteFile(csvFile, []byte(csvContent), 0644)
	require.NoError(t, err)

	// Set the file and configuration
	screen.selectedFile = csvFile
	screen.config = data.DefaultSessionConfig()

	// Test configuration validation
	screen.onTestConfig()

	// Should be valid now
	assert.True(t, screen.isValid)
}

// TestConfigurationValidationWithInvalidFile tests error handling
func TestConfigurationValidationWithInvalidFile(t *testing.T) {
	screen := NewSetupScreen()

	// Test with no file selected
	screen.selectedFile = ""
	screen.onTestConfig()
	assert.False(t, screen.isValid)

	// Test with non-existent file
	screen.selectedFile = "/non/existent/file.csv"
	screen.onTestConfig()
	assert.False(t, screen.isValid)
}

// TestStartSessionValidation tests session start validation
func TestStartSessionValidation(t *testing.T) {
	screen := NewSetupScreen()

	// Test starting session without validation
	screen.onStartSession()
	// Should not proceed without validation

	// Test starting session without file
	screen.isValid = true
	screen.selectedFile = ""
	screen.onStartSession()
	// Should not proceed without file

	// Test starting session with valid configuration
	screen.isValid = true
	screen.selectedFile = "/path/to/test.csv"

	// Mock app for testing
	mockApp := &MockApp{}
	screen.app = mockApp

	// Should succeed (in real implementation, would call app methods)
	assert.NotPanics(t, func() {
		screen.onStartSession()
	})
}

// Additional integration tests

// TestScreenIntegrationWithTView tests integration with tview components
func TestScreenIntegrationWithTView(t *testing.T) {
	screen := NewSetupScreen()

	// Test that all UI components are properly configured
	assert.NotNil(t, screen.container)
	assert.NotNil(t, screen.leftPanel)
	assert.NotNil(t, screen.rightPanel)

	// Test that forms have expected items
	assert.Greater(t, screen.filePanel.GetFormItemCount(), 0)
	assert.Greater(t, screen.configPanel.GetFormItemCount(), 0)

	// Test that panels are properly sized and arranged
	container := screen.GetPrimitive().(*tview.Flex)
	assert.NotNil(t, container)
}

// TestFormFieldAccess tests accessing form fields programmatically
func TestFormFieldAccess(t *testing.T) {
	screen := NewSetupScreen()

	// Test accessing file path field
	filePathField := screen.filePanel.GetFormItemByLabel("File Path")
	assert.NotNil(t, filePathField)
	assert.IsType(t, &tview.InputField{}, filePathField)

	// Test accessing header checkbox
	headerField := screen.filePanel.GetFormItemByLabel("Has Header Row")
	assert.NotNil(t, headerField)
	assert.IsType(t, &tview.Checkbox{}, headerField)

	// Test accessing column mapping fields
	idField := screen.configPanel.GetFormItemByLabel("ID Column")
	assert.NotNil(t, idField)
	assert.IsType(t, &tview.InputField{}, idField)

	titleField := screen.configPanel.GetFormItemByLabel("Title Column")
	assert.NotNil(t, titleField)
	assert.IsType(t, &tview.InputField{}, titleField)
}

// TestDefaultValues tests that default configuration values are properly set
func TestDefaultValues(t *testing.T) {
	screen := NewSetupScreen()
	defaultConfig := data.DefaultSessionConfig()

	// Verify CSV configuration defaults
	assert.Equal(t, defaultConfig.CSV.IDColumn, screen.config.CSV.IDColumn)
	assert.Equal(t, defaultConfig.CSV.TitleColumn, screen.config.CSV.TitleColumn)
	assert.Equal(t, defaultConfig.CSV.AbstractColumn, screen.config.CSV.AbstractColumn)
	assert.Equal(t, defaultConfig.CSV.SpeakerColumn, screen.config.CSV.SpeakerColumn)
	assert.Equal(t, defaultConfig.CSV.ScoreColumn, screen.config.CSV.ScoreColumn)
	assert.Equal(t, defaultConfig.CSV.HasHeader, screen.config.CSV.HasHeader)
	assert.Equal(t, defaultConfig.CSV.Delimiter, screen.config.CSV.Delimiter)

	// Verify Elo configuration defaults
	assert.Equal(t, defaultConfig.Elo.InitialRating, screen.config.Elo.InitialRating)
	assert.Equal(t, defaultConfig.Elo.KFactor, screen.config.Elo.KFactor)
	assert.Equal(t, defaultConfig.Elo.MinRating, screen.config.Elo.MinRating)
	assert.Equal(t, defaultConfig.Elo.MaxRating, screen.config.Elo.MaxRating)
}
