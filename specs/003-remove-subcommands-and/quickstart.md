# Quickstart: Remove Subcommands and Implement Automatic Mode Detection

**Date**: October 2, 2025  
**Feature**: 003-remove-subcommands-and  
**Time to Complete**: ~15 minutes

## Overview

This quickstart validates the simplified CLI interface with automatic mode detection. You'll test both new session creation and existing session resumption without using subcommands.

## Prerequisites

- Go 1.25+ installed
- confelo application built with new changes
- Sample CSV file with proposals
- Terminal access

## Test Scenarios

### Scenario 1: Start New Session

**Objective**: Verify automatic detection of new session and TUI launch

1. **Prepare test data**

   ```bash
   # Use existing test data
   cp testdata/proposals.csv test-proposals.csv
   ```

2. **Run with new session name**

   ```bash
   ./confelo --session-name "QuickstartTest" --input test-proposals.csv
   ```

3. **Expected behavior**:
   - Application detects new session (Start mode)
   - Validates CSV input automatically
   - Creates session file in `sessions/` directory
   - Launches TUI in ranking mode
   - Shows proposal comparison interface

4. **Validation**:
   - [ ] No subcommand required
   - [ ] TUI launches successfully  
   - [ ] Session file created: `sessions/session_QuickstartTest_*.json`
   - [ ] Proposals loaded correctly
   - [ ] Comparison interface functional

5. **Test export functionality**:
   - Within TUI, navigate to export screen
   - Export results to file
   - Verify export file created

6. **Exit application** (Ctrl+C or quit command)

### Scenario 2: Resume Existing Session  

**Objective**: Verify automatic detection of existing session

1. **Resume the session created in Scenario 1**

   ```bash
   ./confelo --session-name "QuickstartTest"
   ```

2. **Expected behavior**:
   - Application detects existing session (Resume mode)
   - Ignores `--input` parameter if provided
   - Loads existing session data
   - Launches TUI with previous state
   - Shows continuation of ranking progress

3. **Validation**:
   - [ ] No subcommand required
   - [ ] Previous comparisons preserved
   - [ ] Ranking state maintained
   - [ ] TUI shows correct progress
   - [ ] Can continue making comparisons

4. **Test list functionality**:
   - Within TUI, navigate to session list screen
   - Verify current session appears in list
   - Check session metadata display

### Scenario 3: Error Handling

**Objective**: Verify error conditions and user guidance

1. **Missing session name**

   ```bash
   ./confelo --input test-proposals.csv
   ```

   - [ ] Clear error message displayed
   - [ ] Usage help shown
   - [ ] Exit code 2 returned

2. **New session without input**

   ```bash
   ./confelo --session-name "NoInputTest"
   ```

   - [ ] Error: "Input file required for new sessions"
   - [ ] Usage help shown  
   - [ ] Exit code 2 returned

3. **Invalid input file**

   ```bash
   ./confelo --session-name "BadInputTest" --input nonexistent.csv
   ```

   - [ ] File not found error
   - [ ] Clear error message
   - [ ] Exit code 1 returned

4. **Corrupted session file**

   ```bash
   # Manually corrupt a session file
   echo "invalid json" > sessions/session_CorruptTest_*.json
   ./confelo --session-name "CorruptTest"
   ```

   - [ ] Corruption detected
   - [ ] User prompted to delete file
   - [ ] Graceful handling based on user choice

### Scenario 4: Configuration Options

**Objective**: Verify configuration parameters work with simplified CLI

1. **Custom configuration**

   ```bash
   ./confelo --session-name "ConfigTest" --input test-proposals.csv \
     --comparison-mode trio --initial-rating 1600 --target-accepted 5
   ```

2. **Validation**:
   - [ ] Configuration applied correctly
   - [ ] TUI reflects custom settings
   - [ ] Session file contains correct config

3. **Help and version**

   ```bash
   ./confelo --help
   ./confelo --version
   ```

   - [ ] Help shows simplified usage (no subcommands)
   - [ ] Version information displayed

## Migration Validation

### Old vs New Commands

Test that previous subcommand workflows are replaced:

1. **Previous start command**

   ```bash
   # OLD: confelo start --input file.csv --session-name name
   # NEW: 
   ./confelo --input test-proposals.csv --session-name "MigrationTest"
   ```

2. **Previous resume command**  

   ```bash
   # OLD: confelo resume --session-id name
   # NEW:
   ./confelo --session-name "MigrationTest"
   ```

3. **Export/List via TUI**
   - [ ] Export functionality available in TUI
   - [ ] List functionality available in TUI  
   - [ ] No CLI subcommands needed

## Performance Validation

### Startup Time

1. **Measure startup time for new session**

   ```bash
   time ./confelo --session-name "PerfTest" --input test-proposals.csv
   ```

   - [ ] Startup completes in <200ms
   - [ ] Mode detection adds minimal overhead

2. **Measure startup time for resume**  

   ```bash
   time ./confelo --session-name "PerfTest"
   ```

   - [ ] Resume completes in <200ms
   - [ ] Session loading is fast

## Cleanup

```bash
# Remove test sessions
rm sessions/session_QuickstartTest_*.json
rm sessions/session_NoInputTest_*.json  
rm sessions/session_BadInputTest_*.json
rm sessions/session_CorruptTest_*.json
rm sessions/session_ConfigTest_*.json
rm sessions/session_MigrationTest_*.json
rm sessions/session_PerfTest_*.json

# Remove test files
rm test-proposals.csv
```

## Success Criteria

All validation checkboxes above should be checked ✓:

- [ ] New session detection works automatically
- [ ] Resume session detection works automatically  
- [ ] Error handling provides clear guidance
- [ ] Configuration options function correctly
- [ ] Performance targets met (<200ms startup)
- [ ] TUI export/list functionality works
- [ ] No subcommands required for any operation
- [ ] Migration from old commands is straightforward

## Troubleshooting

### Common Issues

1. **"Session name required" error**
   - Ensure `--session-name` parameter is provided

2. **"Input file required" error**  
   - Provide `--input` parameter for new sessions
   - Check if session name already exists (use different name for new session)

3. **TUI not launching**
   - Check terminal supports TUI (not in non-interactive shell)
   - Verify session files have correct permissions

4. **Performance issues**
   - Check sessions directory size (too many files)
   - Verify file system performance
   - Monitor memory usage during startup

### Getting Help

- Run `./confelo --help` for usage information
- Check session files in `sessions/` directory for debugging
- Use `--verbose` flag for detailed logging
- Verify Go version compatibility (1.25+ required)
