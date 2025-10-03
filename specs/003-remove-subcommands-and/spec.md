# Feature Specification: Remove Subcommands and Implement Automatic Mode Detection

**Feature Branch**: `003-remove-subcommands-and`  
**Created**: October 2, 2025  
**Status**: Draft  
**Input**: User description: "A previous implementation added commands Start, Resume, Export, List, Validate, but it is unnecessary, since all configuration is handled at startup via command-line parameters. Validation is handled during startup. If --session-name parameter is new, then app should immediately switch to Start mode. If --session-name was used before and there is save session, app should proceed in Resume mode. Export and List are done inside the app with UI. The goal is to remove commands and everything connected to them, while ensuring that application will operate in a proper mode automatically."

---

## Clarifications

### Session 2025-10-02

- Q: When multiple session files could match the same session name (e.g., due to case sensitivity or partial matching), how should the system resolve this? → A: Exact case-sensitive match only
- Q: When a session file is corrupted or unreadable, what should the application do? → A: Prompt user to delete corrupted file
- Q: When --session-name is provided but --input is missing for a new session, how should the system respond? → A: Show error and exit with usage help
- Q: When the sessions directory doesn't exist, what should the application do? → A: Create directory automatically and proceed

## User Scenarios & Testing

### Primary User Story

As a conference organizer, I want to launch the confelo application with a simple command line that automatically determines whether I'm starting a new ranking session or resuming an existing one, so that I don't need to remember different subcommands for different scenarios.

### Acceptance Scenarios

1. **Given** I have a CSV file with proposals and want to start ranking, **When** I run `confelo --input proposals.csv --session-name "DevConf2025"` and no session with that name exists, **Then** the application starts a new ranking session in interactive mode

2. **Given** I previously started a session named "DevConf2025", **When** I run `confelo --session-name "DevConf2025"` (with or without --input), **Then** the application resumes the existing session in interactive mode

3. **Given** I provide an invalid CSV file, **When** the application starts, **Then** validation occurs automatically during startup and appropriate error messages are displayed

4. **Given** I'm working within the application, **When** I need to export results or list sessions, **Then** these functions are available through the user interface

### Edge Cases

- What happens when user declines to delete a corrupted session file?
- How does the system handle invalid session names with special characters?
- What occurs when session name contains invalid filesystem characters?
- How does the application handle filesystem permission errors when creating sessions directory?

## Requirements

### Functional Requirements

- **FR-001**: System MUST automatically detect whether a session name refers to a new or existing session using exact case-sensitive matching
- **FR-002**: System MUST start in "Start mode" when session name is new and valid input file is provided
- **FR-003**: System MUST start in "Resume mode" when session name matches an existing saved session
- **FR-004**: System MUST perform CSV validation automatically during startup for new sessions
- **FR-005**: System MUST eliminate all subcommands (start, resume, export, list, validate) from the CLI interface
- **FR-006**: System MUST eliminate batch mode and always run in interactive mode
- **FR-007**: System MUST automatically create sessions directory if it doesn't exist
- **FR-008**: System MUST preserve all existing command-line configuration options (comparison-mode, initial-rating, target-accepted, etc.) except batch mode
- **FR-009**: System MUST require --input parameter for new sessions and show error with usage help when missing
- **FR-010**: System MUST eliminate all configuration file support and use only command-line parameters for configuration
- **FR-011**: System MUST provide clear error messages when required parameters are missing or invalid
- **FR-013**: System MUST prompt user to delete corrupted session files and handle user response appropriately
- **FR-012**: System MUST generate session IDs consistently with existing format for new sessions

### Key Entities

- **Session**: Represents a ranking session with unique name, ID, proposals, and state information
- **Command Line Interface**: Simplified interface accepting global options and automatic mode detection
- **Configuration**: Session settings that can be specified only via CLI flags (no config files or environment variables)

---

## Review & Acceptance Checklist

### Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

### Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous  
- [x] Success criteria are measurable
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified
