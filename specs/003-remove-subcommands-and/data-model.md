# Data Model: Remove Subcommands and Implement Automatic Mode Detection

**Date**: October 2, 2025  
**Feature**: 003-remove-subcommands-and  

## Core Entities

### CLIOptions (Modified)

Simplified CLI structure without subcommands.

**Fields**:

- `Verbose` (bool): Enable verbose logging  
- `Version` (bool): Show version information
- `Help` (bool): Show help message
- `Input` (string): Path to CSV file containing proposals
- `SessionName` (string): Name for the ranking session  
- `ComparisonMode` (string): Comparison type (pairwise/trio/quartet)
- `InitialRating` (float64): Starting Elo rating
- `OutputScale` (string): Output scale format
- `TargetAccepted` (int): Number of talks to be accepted

**Configuration Approach**:

- All configuration via CLI flags only (no config files)
- Defaults embedded in Go struct field tags
- No environment variable support
- No runtime configuration changes

**Validation Rules**:

- `SessionName` is required
- `Input` is required for new sessions (when session doesn't exist)
- `ComparisonMode` must be valid enum value
- `InitialRating` must be positive
- `TargetAccepted` must be positive
- `OutputScale` consists of two numbers separated by a hyphen. If integers, the scale is finite (e.g. "0-10"). If floats, the scale uses 2 decimal digits (e.g. "1.01-9.99").

**State Transitions**:

- Parse CLI args → Validate required fields → Detect mode → Initialize session

### SessionMode (New)

Represents the detected operational mode.

**Values**:

- `Start`: New session mode (session name not found)
- `Resume`: Existing session mode (session name found)

**Validation Rules**:

- Start mode requires `Input` parameter
- Resume mode ignores `Input` parameter (uses stored session data)

### SessionDetector (New)

Service for detecting session existence and mode determination.

**Fields**:

- `sessionsDir` (string): Directory containing session files
- `sessionName` (string): Target session name

**Methods**:

- `DetectMode() (SessionMode, error)`: Determine Start/Resume mode
- `FindSessionFile(name string) (string, error)`: Locate session file by name  
- `ValidateSessionFile(path string) error`: Check session file integrity

**Validation Rules**:

- Session name must be valid filesystem name
- Sessions directory must exist or be creatable
- Session files must be readable and valid JSON

**State Transitions**:

- Initialize → Scan sessions directory → Match session name → Return mode

### CLIError (Enhanced)

Extended error handling for new failure modes.

**New Error Codes**:

- `ExitModeDetectionError`: Session mode detection failed
- `ExitSessionCorruption`: Session file corrupted, user action needed

**Enhanced Fields**:

- Added suggestions for mode detection failures
- Added session cleanup instructions for corruption errors

## Modified Entities

### Session (pkg/data/session.go)

**New Methods**:

- `DetectMode(sessionName string) (SessionMode, error)`: Core mode detection
- `IsSessionCorrupted(path string) bool`: Validate session file integrity

**Modified Behavior**:

- Session creation now triggered automatically in Start mode
- Session loading now triggered automatically in Resume mode
- Error handling enhanced for mode detection failures

### TUIApp (pkg/tui/app.go)

**New Initialization Modes**:

- `NewStartMode(options CLIOptions, input string)`: Initialize for new session with CLI parameters
- `NewResumeMode(options CLIOptions, sessionID string)`: Initialize for existing session with CLI parameters

**Updated Integration**:

- Export screen (already exists - no changes needed)
- Session list screen (already exists - no changes needed)

## Entity Relationships

```
CLIOptions → SessionDetector → SessionMode
     ↓             ↓               ↓
   Session ←→ TUIApp.Initialize(mode)
     ↓
  Export/List (via existing TUI screens)
```

## Data Flow

### New Session Flow

1. Parse CLI: Extract `sessionName`, `input`, and all CLI options
2. Mode Detection: `SessionDetector.DetectMode()` → `Start`
3. Validation: Ensure `input` parameter provided
4. Session Creation: Initialize new session with CSV data and CLI options
5. TUI Launch: Start interactive ranking

### Resume Session Flow  

1. Parse CLI: Extract `sessionName` and all CLI options
2. Mode Detection: `SessionDetector.DetectMode()` → `Resume`
3. Session Loading: Load existing session data
4. TUI Launch: Resume interactive ranking

### Error Handling Flow

1. Corrupted Session: Prompt user to delete file
2. Missing Input: Show usage help and exit
3. Invalid Session Name: Clear error message
4. Permissions Error: Graceful failure with instructions

## Validation Rules Summary

### CLI Level

- Session name required and valid
- Input required for new sessions only
- All numeric values positive
- Enum values within allowed sets

### Session Level  

- Exact case-sensitive session name matching
- Session files must be valid JSON
- Sessions directory must exist or be creatable
- File permissions must allow read/write

### TUI Level

- Mode must be determined before TUI initialization  
- Export/list functions (already implemented) remain available during interactive mode
- Error states handled with appropriate user feedback
