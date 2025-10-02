# Feature Specification: Remove SetupScreen Component

**Feature Branch**: `002-remove-setupscreen-component`  
**Created**: October 2, 2025  
**Status**: Draft  
**Input**: User description: "A previous implementation added a SetupScreen component, but it is unnecessary, since all configuration is handled at startup via command-line parameters. The goal is to remove SetupScreen and everything connected to it, while ensuring that the normal workflow of the application is unaffected."

## User Scenarios & Testing

### Primary User Story

As a conference organizer using confelo, I want the application to start directly with the comparison screen when launched with command-line parameters, without requiring manual configuration through a SetupScreen, so that I can get to ranking talks more efficiently.

### Acceptance Scenarios

1. **Given** I have started confelo with required parameters like `--input proposals.csv`, **When** the application launches, **Then** it should immediately show the comparison screen without requiring setup steps.
2. **Given** I have launched confelo with valid command parameters, **When** I navigate through the application, **Then** I should not see any setup screen or configuration interface.
3. **Given** I have launched confelo with incomplete parameters, **When** the application detects missing required configuration, **Then** it should provide a helpful error message rather than showing a setup screen.

### Edge Cases

- What happens when a user tries to access the setup screen through keyboard shortcuts or navigation? The application should not have any pathways to a nonexistent screen.
- How does the system handle configuration changes that were previously managed through the setup screen? All configuration should be solely through command-line parameters or configuration files.

## Requirements

### Functional Requirements

- **FR-001**: System MUST remove the SetupScreen component and all related code.
- **FR-002**: System MUST continue to start directly with the comparison screen when valid parameters are provided.
- **FR-003**: System MUST ensure all configuration previously handled by SetupScreen is fully supported via command-line parameters.
- **FR-004**: System MUST maintain consistent functionality without the setup screen.
- **FR-005**: System MUST provide clear error messages for missing or invalid parameters.
- **FR-006**: System MUST update all screen navigation logic to remove references to the setup screen.

### Key Entities

- **Screen Navigation**: How the application manages transitions between screens, which must be updated to remove the setup screen.
- **App Initialization**: The startup process that will now skip the setup screen entirely.
- **CLI Parameters**: The command-line parameters that replace the setup screen's configuration options.

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
