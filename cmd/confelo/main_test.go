// Package main provides integration tests for the confelo CLI application.
// It tests all subcommands, error handling, and argument validation to ensure
// the CLI interface contract is properly implemented.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pashagolub/confelo/pkg/data"
)

// TestMain sets up and tears down test environment
func TestMain(m *testing.M) {
	// Create test directory
	testDir := "test_cli_sessions"
	os.Mkdir(testDir, 0755)

	// Change to test directory
	oldDir, _ := os.Getwd()
	os.Chdir(testDir)

	// Run tests
	code := m.Run()

	// Cleanup
	os.Chdir(oldDir)
	os.RemoveAll(testDir)

	os.Exit(code)
}

// TestStartCommand tests the start subcommand functionality
func TestStartCommand(t *testing.T) {
	// Create test CSV file
	testCSV := "test_proposals.csv"
	csvContent := `id,title,speaker,abstract
1,"Go Programming","Alice Johnson","Introduction to Go language"
2,"React Patterns","Bob Smith","Advanced React development"
3,"Machine Learning","Carol Davis","ML fundamentals"`

	err := os.WriteFile(testCSV, []byte(csvContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}
	defer os.Remove(testCSV)

	tests := []struct {
		name         string
		cmd          *StartCommand
		expectError  bool
		expectedCode ErrorCode
		checkOutput  func(t *testing.T, err error)
	}{
		{
			name: "valid start command batch mode",
			cmd: &StartCommand{
				Input:       testCSV,
				Batch:       true,
				SessionName: "Test Session",
				Global:      &GlobalOptions{},
			},
			expectError: false,
		},
		{
			name: "missing input file",
			cmd: &StartCommand{
				Batch:  true,
				Global: &GlobalOptions{},
			},
			expectError:  true,
			expectedCode: ExitFileError,
		},
		{
			name: "non-existent input file",
			cmd: &StartCommand{
				Input:  "nonexistent.csv",
				Batch:  true,
				Global: &GlobalOptions{},
			},
			expectError:  true,
			expectedCode: ExitFileError,
		},
		{
			name: "custom configuration overrides",
			cmd: &StartCommand{
				Input:          testCSV,
				Batch:          true,
				InitialRating:  1200,
				ComparisonMode: "trio",
				Global:         &GlobalOptions{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Execute([]string{})

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if cliErr, ok := err.(*CLIError); ok {
					if tt.expectedCode != 0 && cliErr.Code != tt.expectedCode {
						t.Errorf("Expected error code %d, got %d", tt.expectedCode, cliErr.Code)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, err)
			}
		})
	}
}

// TestValidateCommand tests the validate subcommand functionality
func TestValidateCommand(t *testing.T) {
	// Create test CSV files
	validCSV := "valid_test.csv"
	validContent := `id,title,speaker
1,"Test Title","Test Speaker"
2,"Another Title","Another Speaker"`

	invalidCSV := "invalid_test.csv"
	invalidContent := `id,title,speaker
1,"Unclosed quote,"Test Speaker"`

	err := os.WriteFile(validCSV, []byte(validContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid test CSV: %v", err)
	}
	defer os.Remove(validCSV)

	err = os.WriteFile(invalidCSV, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid test CSV: %v", err)
	}
	defer os.Remove(invalidCSV)

	tests := []struct {
		name         string
		cmd          *ValidateCommand
		expectError  bool
		expectedCode ErrorCode
	}{
		{
			name: "valid CSV file",
			cmd: &ValidateCommand{
				Input: validCSV,
			},
			expectError: false,
		},
		{
			name: "non-existent file",
			cmd: &ValidateCommand{
				Input: "nonexistent.csv",
			},
			expectError:  true,
			expectedCode: ExitFileError,
		},
		{
			name: "valid CSV with preview",
			cmd: &ValidateCommand{
				Input:   validCSV,
				Preview: 1,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Execute([]string{})

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if cliErr, ok := err.(*CLIError); ok {
					if tt.expectedCode != 0 && cliErr.Code != tt.expectedCode {
						t.Errorf("Expected error code %d, got %d", tt.expectedCode, cliErr.Code)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestListCommand tests the list subcommand functionality
func TestListCommand(t *testing.T) {
	// Create a test session first
	testCSV := "list_test.csv"
	csvContent := `id,title,speaker
1,"Test","Speaker"`

	err := os.WriteFile(testCSV, []byte(csvContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}
	defer os.Remove(testCSV)

	// Create a session to list
	startCmd := &StartCommand{
		Input:       testCSV,
		SessionName: "List Test Session",
		Batch:       true,
		Global:      &GlobalOptions{},
	}
	err = startCmd.Execute([]string{})
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	tests := []struct {
		name        string
		cmd         *ListCommand
		expectError bool
		checkOutput func(t *testing.T, err error)
	}{
		{
			name: "default table format",
			cmd:  &ListCommand{},
		},
		{
			name: "json format",
			cmd:  &ListCommand{Format: "json"},
		},
		{
			name: "csv format",
			cmd:  &ListCommand{Format: "csv"},
		},
		{
			name: "filter by status",
			cmd:  &ListCommand{Status: "active"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Execute([]string{})

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, err)
			}
		})
	}
}

// TestExportCommand tests the export subcommand functionality
func TestExportCommand(t *testing.T) {
	// Create a test session first
	testCSV := "export_test.csv"
	csvContent := `id,title,speaker,abstract
1,"Export Test","Test Speaker","Test abstract"`

	err := os.WriteFile(testCSV, []byte(csvContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}
	defer os.Remove(testCSV)

	// Create a session to export
	startCmd := &StartCommand{
		Input:       testCSV,
		SessionName: "Export Test Session",
		Batch:       true,
		Global:      &GlobalOptions{},
	}
	err = startCmd.Execute([]string{})
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// Find the session ID
	listCmd := &ListCommand{Format: "json"}
	err = listCmd.Execute([]string{})
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	// Get a session ID from the sessions directory
	sessions, err := filepath.Glob("sessions/*.json")
	if err != nil || len(sessions) == 0 {
		t.Fatalf("No sessions found for export test")
	}

	sessionFile := filepath.Base(sessions[0])
	sessionID := strings.TrimSuffix(sessionFile, ".json")

	tests := []struct {
		name         string
		cmd          *ExportCommand
		expectError  bool
		expectedCode ErrorCode
		checkFile    string
	}{
		{
			name: "csv export",
			cmd: &ExportCommand{
				SessionID: sessionID,
				Output:    "test_export.csv",
				Global:    &GlobalOptions{},
			},
			checkFile: "test_export.csv",
		},
		{
			name: "json export",
			cmd: &ExportCommand{
				SessionID: sessionID,
				Output:    "test_export.json",
				Format:    "json",
				Global:    &GlobalOptions{},
			},
			checkFile: "test_export.json",
		},
		{
			name: "non-existent session",
			cmd: &ExportCommand{
				SessionID: "nonexistent_session",
				Global:    &GlobalOptions{},
			},
			expectError:  true,
			expectedCode: ExitSessionError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Execute([]string{})

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if cliErr, ok := err.(*CLIError); ok {
					if tt.expectedCode != 0 && cliErr.Code != tt.expectedCode {
						t.Errorf("Expected error code %d, got %d", tt.expectedCode, cliErr.Code)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Check if output file was created
				if tt.checkFile != "" {
					if _, err := os.Stat(tt.checkFile); os.IsNotExist(err) {
						t.Errorf("Expected output file %s was not created", tt.checkFile)
					} else {
						os.Remove(tt.checkFile) // Cleanup
					}
				}
			}
		})
	}
}

// TestResumeCommand tests the resume subcommand functionality
func TestResumeCommand(t *testing.T) {
	// Create a test session first
	testCSV := "resume_test.csv"
	csvContent := `id,title,speaker
1,"Resume Test","Test Speaker"`

	err := os.WriteFile(testCSV, []byte(csvContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}
	defer os.Remove(testCSV)

	// Create a session to resume
	startCmd := &StartCommand{
		Input:       testCSV,
		SessionName: "Resume Test Session",
		Batch:       true,
		Global:      &GlobalOptions{},
	}
	err = startCmd.Execute([]string{})
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// Get a session ID
	sessions, err := filepath.Glob("sessions/*.json")
	if err != nil || len(sessions) == 0 {
		t.Fatalf("No sessions found for resume test")
	}

	sessionFile := filepath.Base(sessions[0])
	sessionID := strings.TrimSuffix(sessionFile, ".json")

	tests := []struct {
		name         string
		cmd          *ResumeCommand
		expectError  bool
		expectedCode ErrorCode
	}{
		{
			name: "resume existing session batch",
			cmd: &ResumeCommand{
				SessionID: sessionID,
				Batch:     true,
				Global:    &GlobalOptions{},
			},
			expectError: false,
		},
		{
			name: "resume non-existent session",
			cmd: &ResumeCommand{
				SessionID: "nonexistent",
				Batch:     true,
				Global:    &GlobalOptions{},
			},
			expectError:  true,
			expectedCode: ExitSessionError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Execute([]string{})

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if cliErr, ok := err.(*CLIError); ok {
					if tt.expectedCode != 0 && cliErr.Code != tt.expectedCode {
						t.Errorf("Expected error code %d, got %d", tt.expectedCode, cliErr.Code)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestErrorHandling tests error handling and JSON error output
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		error     *CLIError
		checkJSON func(t *testing.T, jsonStr string)
	}{
		{
			name: "file not found error",
			error: &CLIError{
				Code:    ExitFileError,
				Message: "File not found: test.csv",
				Details: map[string]interface{}{
					"file": "test.csv",
				},
				Suggestions: []string{
					"Check file path and name",
					"Ensure file exists",
				},
			},
			checkJSON: func(t *testing.T, jsonStr string) {
				var errorObj map[string]interface{}
				err := json.Unmarshal([]byte(jsonStr), &errorObj)
				if err != nil {
					t.Errorf("Failed to parse error JSON: %v", err)
					return
				}

				errorData := errorObj["error"].(map[string]interface{})
				if errorData["code"].(float64) != float64(ExitFileError) {
					t.Errorf("Expected error code %d, got %v", ExitFileError, errorData["code"])
				}

				if !strings.Contains(errorData["message"].(string), "File not found") {
					t.Errorf("Expected message to contain 'File not found'")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonStr := formatErrorJSON(tt.error)
			if tt.checkJSON != nil {
				tt.checkJSON(t, jsonStr)
			}
		})
	}
}

// TestConfigurationOverrides tests CLI flag configuration overrides
func TestConfigurationOverrides(t *testing.T) {
	testCSV := "config_test.csv"
	csvContent := `id,title,speaker
1,"Config Test","Test Speaker"`

	err := os.WriteFile(testCSV, []byte(csvContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}
	defer os.Remove(testCSV)

	// Test initial rating override
	cmd := &StartCommand{
		Input:         testCSV,
		InitialRating: 1200.0,
		OutputScale:   "1-10",
		Batch:         true,
	}

	// Load default config
	config := data.DefaultSessionConfig()

	// Apply overrides
	applyStartConfigOverrides(&config, cmd)

	// Check that overrides were applied
	if config.Elo.InitialRating != 1200.0 {
		t.Errorf("Expected initial rating override to 1200.0, got %f", config.Elo.InitialRating)
	}

	if config.Elo.OutputMin != 1.0 || config.Elo.OutputMax != 10.0 {
		t.Errorf("Expected output scale override to 1-10, got %f-%f", config.Elo.OutputMin, config.Elo.OutputMax)
	}
}

// TestVersionCommand tests version information display
func TestVersionCommand(t *testing.T) {
	// Test version through validate command (simplest path)
	cmd := &ValidateCommand{
		Version: true,
	}

	err := cmd.Execute([]string{})
	// Version command should not return an error when displaying version
	if err != nil {
		t.Errorf("Version command should not return error, got: %v", err)
	}
}
