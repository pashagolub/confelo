# CLI Interface Contract

**Command**: `confelo`  
**Description**: Conference talk ranking application with Elo rating system

## Command Structure

```bash
confelo [global-options] <command> [command-options] [arguments]
```

## Global Options

| Option | Short | Type | Description | Default |
|--------|-------|------|-------------|---------|
| `--config` | `-c` | string | Configuration file path | `./config.yaml` |
| `--verbose` | `-v` | bool | Enable verbose logging | `false` |
| `--help` | `-h` | bool | Show help message | `false` |
| `--version` | | bool | Show version information | `false` |

## Commands

### `start` - Start a new ranking session

```bash
confelo start --input <csv-file> [options]
```

**Required Arguments**:

- `--input`, `-i`: Path to CSV file containing proposals

**Options**:

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `--session-name` | string | Name for the ranking session | Auto-generated |
| `--comparison-mode` | string | Comparison type (pairwise/trio/quartet) | `pairwise` |
| `--initial-rating` | float | Starting Elo rating | `1500.0` |
| `--output-scale` | string | Output scale format (e.g., "0-10", "1-5") | `0-100` |

**Examples**:

```bash
# Basic usage
confelo start -i proposals.csv

# Custom session with trio comparisons
confelo start -i talks.csv --session-name "PyCon2025" --comparison-mode trio

# Custom rating scale
confelo start -i submissions.csv --output-scale "1-10"
```

**Exit Codes**:

- `0`: Success
- `1`: File not found or invalid CSV
- `2`: Invalid configuration
- `3`: Session creation failed

### `resume` - Resume an existing session

```bash
confelo resume <session-id> [options]
```

**Arguments**:

- `session-id`: Identifier of existing session to resume

**Options**:

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `--comparison-mode` | string | Override comparison mode | Session default |

**Examples**:

```bash
# Resume by session ID
confelo resume session_20250928_143021

# Resume with different comparison mode
confelo resume session_20250928_143021 --comparison-mode quartet
```

**Exit Codes**:

- `0`: Success
- `1`: Session not found
- `2`: Session corrupted or invalid
- `3`: Resume failed

### `export` - Export ranking results

```bash
confelo export <session-id> [options]
```

**Arguments**:

- `session-id`: Session to export results from

**Options**:

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `--output`, `-o` | string | Output file path | `rankings_<session-id>.csv` |
| `--format` | string | Export format (csv/json/text) | `csv` |
| `--include-stats` | bool | Include rating statistics | `false` |
| `--include-audit` | bool | Include comparison history | `false` |

**Examples**:

```bash
# Basic export
confelo export session_20250928_143021

# Export with statistics
confelo export session_20250928_143021 -o results.csv --include-stats

# JSON format with audit trail
confelo export session_20250928_143021 --format json --include-audit
```

**Exit Codes**:

- `0`: Success
- `1`: Session not found
- `2`: Export failed
- `3`: Invalid output path

### `list` - List available sessions

```bash
confelo list [options]
```

**Options**:

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `--format` | string | Output format (table/json/csv) | `table` |
| `--status` | string | Filter by status (active/complete/all) | `all` |

**Output Format** (table):

```
SESSION ID               NAME        STATUS    PROPOSALS  COMPARISONS  CREATED
session_20250928_143021  PyCon2025   Active    45         23          2025-09-28 14:30
session_20250927_091234  DevConf     Complete  32         31          2025-09-27 09:12
```

**Exit Codes**:

- `0`: Success
- `1`: No sessions found
- `2`: List operation failed

### `validate` - Validate CSV input file

```bash
confelo validate --input <csv-file> [options]
```

**Required Arguments**:

- `--input`, `-i`: Path to CSV file to validate

**Options**:

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `--config` | string | Configuration file for column mapping | Auto-detect |
| `--preview` | int | Number of rows to preview | `5` |

**Output**: Validation report with:

- File statistics (rows, columns)
- Column mapping detection
- Data quality issues
- Preview of parsed data

**Exit Codes**:

- `0`: Valid CSV file
- `1`: File not found
- `2`: Invalid CSV format
- `3`: Missing required columns

## Configuration File Format

**File**: `config.yaml` (YAML format)

```yaml
csv:
  id_column: "id"
  title_column: "title"  
  abstract_column: "abstract"
  speaker_column: "speaker"
  score_column: "score"
  comment_column: "comments"
  has_header: true
  delimiter: ","

elo:
  initial_rating: 1500.0
  k_factor: 32
  min_rating: 0.0
  max_rating: 3000.0
  output_min: 0.0
  output_max: 100.0
  use_decimals: true

ui:
  comparison_mode: "pairwise"
  show_progress: true
  show_confidence: true
  auto_save: true
  auto_save_interval: "30s"

export:
  default_format: "csv"
  include_metadata: true
```

## Error Handling

### Standard Error Format (JSON)

```json
{
  "error": {
    "code": "INVALID_CSV_FORMAT",
    "message": "CSV file has malformed data on line 15",
    "details": {
      "line": 15,
      "column": "title",
      "value": "Missing closing quote"
    },
    "suggestions": [
      "Check for unescaped quotes in title field",
      "Use --preview option to inspect data"
    ]
  }
}
```

### Common Error Codes

| Code | Description | Resolution |
|------|-------------|------------|
| `FILE_NOT_FOUND` | Input file doesn't exist | Check file path and permissions |
| `INVALID_CSV_FORMAT` | Malformed CSV data | Validate CSV with external tool |
| `MISSING_REQUIRED_COLUMNS` | Required columns not found | Update config or CSV headers |
| `SESSION_NOT_FOUND` | Session ID doesn't exist | Use `list` command to see available sessions |
| `INVALID_CONFIGURATION` | Config file has errors | Validate YAML syntax and values |
| `INSUFFICIENT_PROPOSALS` | Less than 2 proposals loaded | Add more proposals to CSV |
| `EXPORT_FAILED` | Cannot write output file | Check output directory permissions |

## Interactive Mode Behavior

When launched with `start` or `resume`, the application enters interactive TUI mode:

### Key Bindings

| Key | Action | Context |
|-----|--------|---------|
| `←/→` | Navigate between proposals | Comparison screen |
| `Enter` | Select proposal | Comparison screen |
| `Space` | Skip comparison | Comparison screen |
| `r` | Show current rankings | Any screen |
| `s` | Save session | Any screen |
| `q` | Quit application | Any screen |
| `?` | Show help | Any screen |

### Screen Flow

1. **Setup Screen**: Confirm configuration, preview proposals
2. **Comparison Screen**: Main evaluation interface with carousel
3. **Progress Screen**: Current rankings and statistics
4. **Export Screen**: Final results and export options

### Auto-save Behavior

- Saves session state every 30 seconds (configurable)
- Saves immediately after each comparison
- Creates backup before major operations
- Atomic writes to prevent corruption
