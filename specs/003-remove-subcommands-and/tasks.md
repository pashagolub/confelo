# Tasks: Remove Subcommands and Implement Automatic Mode Detection

**Input**: Design documents from `/specs/003-remove-subcommands-and/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/

## Execution Flow (main)

```
1. Load plan.md from feature directory
   → Extract: Go 1.25+, tview (TUI), jessevdk/go-flags (CLI), JSON/CSV storage
2. Load design documents:
   → data-model.md: CLIOptions, SessionMode, SessionDetector entities
   → contracts/: cli-contract.md, session-contract.md
   → quickstart.md: 3 test scenarios (new session, resume session, error handling)
3. Generate tasks by category:
   → Setup: project dependencies, linting, test structure
   → Tests: contract tests, integration tests (TDD approach)
   → Core: CLI parsing, session detection, mode logic
   → Integration: TUI initialization, session management
   → Polish: unit tests, error handling, documentation
4. Apply task rules:
   → Different files = mark [P] for parallel
   → Same file = sequential (no [P])
   → Tests before implementation (TDD)
5. Tasks ordered by dependencies: Setup → Tests → Core → Integration → Polish
```

## Format: `[ID] [P?] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Phase 3.1: Setup

- [X] T001 Update Go module dependencies for jessevdk/go-flags and tview in `go.mod`
- [X] T002 [P] Configure linting rules for Go 1.25+ features in project root
- [X] T003 [P] Create test data directory structure for new test scenarios

## Phase 3.2: Contract Tests (TDD)

- [X] T004 [P] Implement CLI contract test in `cmd/confelo/main_test.go` for automatic mode detection
- [X] T005 [P] Implement session detection contract test in `pkg/data/session_test.go` for StartMode/ResumeMode logic
- [X] T006 Create integration test file `pkg/data/integration_test.go` for quickstart scenarios

## Phase 3.3: Core Implementation

- [X] T007 Remove subcommand parsing logic from `cmd/confelo/main.go`
- [X] T008 Update CLI options struct in `pkg/data/cli.go` to remove subcommand fields
- [X] T009 [P] Remove config file support entirely from `pkg/data/config.go` (delete file)
- [X] T010 Implement SessionMode enum and constants in `pkg/data/session.go`
- [X] T011 Implement SessionDetector service in `pkg/data/session.go` with DetectMode method
- [X] T012 Update main CLI entry point in `cmd/confelo/main.go` for automatic mode detection
- [X] T013 [P] Update CLI test suite in `pkg/data/cli_test.go` for CLI-only configuration

## Phase 3.4: Integration

- [X] T014 Update TUI initialization in `pkg/tui/app.go` to handle new CLI-only entry modes
- [X] T015 [P] Update session management in `pkg/data/session.go` to support mode-based initialization
- [X] T016 Add error handling for mode detection failures in `cmd/confelo/main.go`

## Phase 3.5: Polish

- [X] T017 [P] Add comprehensive unit tests for SessionDetector in `pkg/data/session_test.go`
- [X] T018 [P] Add unit tests for updated CLI parsing in `pkg/data/cli_test.go`
- [X] T019 [P] Add integration tests for quickstart scenarios in `pkg/data/integration_test.go`
- [X] T020 Update application help text and usage messages in `pkg/data/cli.go`
- [X] T021 [P] Update existing session tests in `pkg/data/session_test.go` for new behavior
- [X] T022 Performance validation: ensure <200ms startup time with new logic

## Dependencies

### Critical Path

```
T001 → T004,T005 → T007,T008 → T010,T011 → T012 → T014,T015 → T016
```

### Parallel Opportunities

```
Phase 3.2: T004 ∥ T005 ∥ T006
Phase 3.3: T009 ∥ T013 (different files)
Phase 3.4: T014 ∥ T015 (different concerns)
Phase 3.5: T017 ∥ T018 ∥ T019 ∥ T021 (different test files)
```

## Task Agent Commands (Parallel Examples)

### Setup Phase

```bash
# Run these in parallel:
task T001 &  # Update dependencies
task T002 &  # Configure linting
task T003 &  # Create test structure
wait
```

### Contract Tests Phase

```bash
# TDD - write failing tests first:
task T004 &  # CLI contract test
task T005 &  # Session contract test
wait
task T006    # Integration test (depends on both)
```

### Core Implementation Phase

```bash
# Sequential for main.go changes:
task T007    # Remove subcommands from main.go
task T008    # Update CLI options
# Parallel for separate concerns:
task T009 &  # Remove config file (separate file)
task T013 &  # Update CLI tests (separate file)
wait
# Sequential for session.go:
task T010    # Add SessionMode to session.go
task T011    # Add SessionDetector to session.go
task T012    # Update main.go (depends on T010,T011)
```

## Validation Checklist

- [x] All contract files (cli-contract.md, session-contract.md) have corresponding test tasks
- [x] All entities (CLIOptions, SessionMode, SessionDetector) have implementation tasks
- [x] All quickstart scenarios have integration test coverage
- [x] TDD approach: tests before implementation
- [x] Parallel tasks target different files or independent concerns
- [x] Dependencies properly ordered (setup → tests → core → integration → polish)
- [x] Performance requirement (<200ms startup) included in polish phase

## Notes

- **Config file removal**: Task T009 completely removes `pkg/data/config.go` - this is intentional per spec
- **CLI-only approach**: No environment variables or config file support remains
- **Session detection**: New logic automatically determines Start/Resume mode without user input
- **TUI integration**: Existing screens (export, list) remain functional with new entry logic
- **Error handling**: Enhanced error messages for mode detection failures
