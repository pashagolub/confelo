# Research: Remove Subcommands and Implement Automatic Mode Detection

**Date**: October 2, 2025  
**Feature**: 003-remove-subcommands-and  

## Technology Decisions

### CLI Library Strategy

**Decision**: Retain jessevdk/go-flags but simplify to global options only  
**Rationale**:

- Current codebase already uses go-flags effectively
- Removing subcommands reduces complexity significantly  
- Global flags provide sufficient configuration surface
- No need to rewrite existing flag parsing logic

**Alternatives considered**:

- Standard flag package: Would require rewriting all existing flag handling
- cobra: Overkill for simplified CLI interface
- Custom parsing: Unnecessary complexity

### Configuration Strategy

**Decision**: Eliminate all configuration file support - CLI parameters only  
**Rationale**:

- User explicitly requested removal of confelo.yaml and config file traces
- Simplifies deployment (no config file dependencies)
- Reduces complexity and potential configuration conflicts
- Forces explicit parameter specification (clearer intent)
- Eliminates config file parsing, validation, and error handling code

**Alternatives considered**:

- Keep minimal config file support: Rejected per user requirements
- Environment variables only: Less transparent than explicit CLI flags
- Hybrid approach: Adds unnecessary complexity

### Session Detection Algorithm

**Decision**: Implement exact case-sensitive filename matching in sessions/ directory  
**Rationale**:

- Clarification specified exact case-sensitive matching only
- Simple filesystem scan is performant for expected session volumes
- No need for database or indexing at current scale
- Consistent with existing session storage approach

**Alternatives considered**:

- Case-insensitive matching: Rejected per clarification
- Fuzzy matching: Added complexity without clear user benefit
- Database storage: Overkill for local CLI tool

### Mode Detection Logic

**Decision**: Check session existence at startup, before TUI initialization  
**Rationale**:

- Early validation prevents UI mode switches
- Clear error reporting before interactive mode starts
- Follows existing pattern of CLI validation
- Enables fail-fast behavior for invalid configurations

**Alternatives considered**:

- Runtime mode switching: Adds UI complexity
- Lazy detection: Delays error feedback
- TUI-based validation: Inconsistent with CLI error patterns

### Error Handling Strategy

**Decision**: Maintain existing structured error system with exit codes  
**Rationale**:

- Current CLIError system is well-designed
- JSON error formatting supports automation
- Exit codes enable proper shell integration
- No need to change working error patterns

**Alternatives considered**:

- Simplified error handling: Would reduce script integration capabilities
- Panic-based errors: Not suitable for CLI applications
- TUI error dialogs: Inconsistent with CLI expectations

### TUI Integration Points

**Decision**: Use existing export and list TUI screens (no new screens needed)  
**Rationale**:

- Export and list functionality already implemented in TUI
- Maintains interactive workflow consistency
- Leverages existing tview framework and screens
- Follows established TUI patterns in codebase
- Removes need for CLI subcommand complexity

**Alternatives considered**:

- Keep as separate CLI commands: Violates requirement to remove subcommands
- Create new screens: Unnecessary since functionality already exists
- Modal dialogs: Inconsistent with existing TUI patterns

## Implementation Patterns

### CLI Parameter Processing

**Pattern**: Single struct with jessevdk/go-flags annotations, no subcommands  
**Justification**: Simplifies parsing, eliminates command routing logic

### Configuration Management

**Pattern**: Use Go struct defaults and CLI flag defaults only (no config files)  
**Justification**: Eliminates config file parsing complexity, predictable behavior

### Filesystem Operations

**Pattern**: Use existing session storage patterns from pkg/data/session.go  
**Justification**: Consistent with current codebase, tested patterns

### Configuration Validation

**Pattern**: Validate at CLI parsing time, fail fast with clear messages  
**Justification**: Early error detection, follows CLI best practices

### TUI Screen Architecture  

**Pattern**: Follow existing screen pattern in pkg/tui/screens/  
**Justification**: Consistent with existing codebase structure and tview usage

## Risk Mitigation

### Backward Compatibility

**Risk**: Existing scripts using subcommands will break  
**Mitigation**: Clear migration guide in quickstart.md, semantic version bump

### Session Corruption Handling

**Risk**: User declines to delete corrupted session file  
**Mitigation**: Graceful exit with clear error message, suggest manual cleanup

### Filesystem Permissions

**Risk**: Cannot create sessions directory  
**Mitigation**: Clear error message with permission requirements, fail gracefully

## Testing Strategy

### Unit Tests

- Session detection logic with various filename patterns
- CLI flag parsing with new simplified structure  
- Error handling for edge cases (missing input, corrupted files)

### Integration Tests

- End-to-end startup flow for new sessions
- End-to-end startup flow for existing sessions
- TUI integration for export/list functionality

### Performance Validation

- Startup time measurement (target: <200ms)
- Session directory scan performance with large numbers of files
- Memory usage profiling during mode detection
