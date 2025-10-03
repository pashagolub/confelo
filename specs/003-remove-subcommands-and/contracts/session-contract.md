# Session Management Contract

**Date**: October 2, 2025  
**Feature**: 003-remove-subcommands-and  

## Session Detection API

### Interface

```go
type SessionDetector interface {
    DetectMode(sessionName string) (SessionMode, error)
    FindSessionFile(sessionName string) (string, error)
    ValidateSession(sessionPath string) error
}

type SessionMode int

const (
    StartMode SessionMode = iota
    ResumeMode
)
```

### Detection Logic

#### Input Parameters

- `sessionName` (string): User-provided session identifier
- `sessionsDir` (string): Directory containing session files (default: "sessions/")

#### Algorithm

1. **Directory Validation**
   - Check if sessions directory exists
   - Create directory if missing (with user permissions)
   - Return error if creation fails

2. **File Matching**  
   - Scan directory for files matching pattern: `session_{sessionName}_*.json`
   - Use exact case-sensitive string matching
   - Return first match found (if any)

3. **Mode Determination**
   - If match found → `ResumeMode`
   - If no match found → `StartMode`

#### Session File Validation

```go
func ValidateSession(sessionPath string) error {
    // 1. Check file exists and is readable
    // 2. Validate JSON structure
    // 3. Check required fields present
    // 4. Validate data integrity
}
```

### Error Handling Contract

| Error Type | Condition | Recovery Action |
|------------|-----------|-----------------|
| `ErrSessionsDirectoryNotFound` | Directory missing, creation failed | Exit with permissions error |
| `ErrSessionCorrupted` | JSON invalid or incomplete | Prompt user to delete file |
| `ErrSessionPermissions` | File exists but not readable | Exit with permissions error |
| `ErrSessionNameInvalid` | Contains invalid filesystem chars | Exit with validation error |

### Session File Format

#### Expected Structure

```json
{
  "id": "session_DevConf2025_20251002_173856_26a0a25c",
  "name": "DevConf2025", 
  "created_at": "2025-10-02T17:38:56Z",
  "config": {
    "comparison_mode": "pairwise",
    "initial_rating": 1500.0,
    "target_accepted": 10
  },
  "proposals": [...],
  "comparisons": [...],
  "state": "active"
}
```

#### Validation Rules

- `id` field must be present and non-empty
- `name` field must match requested session name exactly
- `config` section must be valid configuration object
- `proposals` array must contain at least 1 element
- `state` must be valid enum value

## TUI Integration Contract

### Mode Initialization

```go
type TUIInitializer interface {
    InitializeStartMode(options CLIOptions, csvPath string) error
    InitializeResumeMode(options CLIOptions, sessionID string) error
}
```

### Screen Integration

### State Management

- TUI maintains session state during runtime
- Auto-save session data after each comparison
- Handle graceful shutdown with session persistence
- Provide session statistics and progress tracking

## Configuration Contract

### CLI-Only Configuration

**Approach**: All configuration specified via CLI flags only

1. CLI flags (explicit values)
2. Go struct defaults (embedded in code)

**Eliminated Features**:

- Configuration files (confelo.yaml removed)
- Environment variables
- Runtime configuration changes

**Default Sessions Directory**: "sessions/" (hardcoded, not configurable)

## Testing Contract

### Unit Tests

- Session detection with various filename patterns
- Mode determination logic
- Error handling for corrupted files
- Directory creation and permissions

### Integration Tests  

- End-to-end CLI parsing to TUI launch
- Session persistence across app restarts
- Export/list functionality via TUI
- CLI-only configuration validation

### Performance Requirements

- Mode detection: <50ms for directories with <1000 sessions  
- Session loading: <200ms for sessions with <10000 comparisons
- TUI initialization: <100ms after mode detection
