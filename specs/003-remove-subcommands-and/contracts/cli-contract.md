# CLI Contract: Simplified Interface

**Date**: October 2, 2025  
**Feature**: 003-remove-subcommands-and  

## Command Line Interface

### Syntax

```
confelo [OPTIONS]
```

### Required Options

| Flag | Type | Description | Validation |
|------|------|-------------|------------|
| `--session-name` | string | Session identifier | Required, filesystem-safe characters |

### Optional Configuration

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--input` | string | - | CSV input file (required for new sessions) |
| `--comparison-mode` | string | "pairwise" | Comparison type: pairwise/trio/quartet |
| `--initial-rating` | float64 | 1500.0 | Starting Elo rating |
| `--output-scale` | string | "0-100" | Output scale format |
| `--target-accepted` | int | 10 | Number of talks to accept |

| `--verbose` | bool | false | Enable verbose logging |

### Global Options

| Flag | Description |
|------|-------------|
| `--help, -h` | Show usage help |
| `--version` | Show version information |

## Behavior Contract

### Automatic Mode Detection

1. **New Session** (`Start` mode):
   - Condition: `--session-name` does not match existing session
   - Requirement: `--input` parameter must be provided  
   - Action: Create new session, initialize from CSV
   - UI: Launch TUI in ranking mode

2. **Existing Session** (`Resume` mode):
   - Condition: `--session-name` matches existing session file
   - Requirement: `--input` parameter ignored if provided
   - Action: Load existing session data
   - UI: Launch TUI in ranking mode

### Error Conditions

| Condition | Exit Code | Message | Action |
|-----------|-----------|---------|---------|
| Missing `--session-name` | 2 | "Session name required" | Show usage help |
| New session without `--input` | 2 | "Input file required for new sessions" | Show usage help |
| Invalid input file | 1 | "Cannot read input file: {path}" | Exit with error |
| Corrupted session file | 3 | "Session file corrupted: {path}" | Prompt to delete |
| Permissions error | 1 | "Cannot access sessions directory" | Exit with error |
| Invalid session name | 2 | "Invalid session name: {name}" | Show usage help |

### Success Flow

```
CLI Parse → Mode Detection → Session Init → TUI Launch → Interactive Mode
```

### Configuration Approach

**CLI-Only Configuration**: All configuration must be specified via command-line flags

1. Command line flags (explicit values)
2. Go struct defaults (embedded in code)

**No Support For**:

- Configuration files (confelo.yaml removed)
- Environment variables  
- Runtime configuration changes

## Examples

### Start New Session

```bash
confelo --session-name "DevConf2025" --input proposals.csv
```

### Resume Existing Session  

```bash
confelo --session-name "DevConf2025"
```

### With Custom Configuration

```bash
confelo --session-name "DevConf2025" --input proposals.csv --comparison-mode trio --target-accepted 20
```

### Show Help

```bash
confelo --help
```

## Migration from Subcommands

| Old Command | New Command |
|-------------|-------------|
| `confelo start --input file.csv --session-name name` | `confelo --input file.csv --session-name name` |
| `confelo resume --session-id name` | `confelo --session-name name` |
| `confelo export --session-id name` | Use TUI export screen |
| `confelo list` | Use TUI list screen |
| `confelo validate --input file.csv` | Automatic at startup |
