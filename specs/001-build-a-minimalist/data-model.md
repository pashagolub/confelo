# Data Model: Conference Talk Ranking Application

**Feature**: 001-build-a-minimalist  
**Date**: 2025-09-28  
**Status**: Complete

## Core Entities

### Proposal

Represents a single conference talk submission.

**Attributes**:

- ID: Unique identifier (string, from CSV)
- Title: Talk title (string)
- Abstract: Full description (string, optional)
- Speaker: Presenter information (string)
- Score: Current Elo rating (float64)
- OriginalScore: Initial rating from CSV (float64, optional)
- Metadata: Additional CSV columns (map[string]string)
- ConflictTags: Identifiers for conflict-of-interest exclusion ([]string)

**Validation Rules**:

- ID must be unique within session
- Title required, non-empty
- Score initialized to configured default if not provided
- Metadata preserves all original CSV columns for export

**State Transitions**:

- Unrated → Rated (after first comparison)
- Rated → Updated (after subsequent comparisons)

### Session

Manages the complete ranking workflow and persistent state.

**Attributes**:

- ID: Unique session identifier (string)
- Name: Human-readable session name (string)
- Proposals: Collection of proposals ([]Proposal)
- Config: Session configuration (SessionConfig)
- CurrentComparison: Active comparison state (ComparisonState, optional)
- CompletedComparisons: Historical comparisons ([]Comparison)
- CreatedAt: Session creation timestamp (time.Time)
- UpdatedAt: Last modification timestamp (time.Time)

**Validation Rules**:

- Must contain at least 2 proposals
- Session name required for save/load operations
- Timestamps automatically managed

**State Transitions**:

- Created → Active (first comparison started)
- Active → Paused (session saved)
- Paused → Active (session resumed)  
- Active → Complete (all desired comparisons finished)

### Comparison

Records a single evaluation event between proposals.

**Attributes**:

- ID: Unique comparison identifier (string)
- SessionID: Parent session reference (string)
- ProposalIDs: Proposals being compared ([]string)
- WinnerID: Selected best proposal ID (string, optional for skipped)
- Rankings: Full ranking order for multi-proposal ([]string, optional)
- Method: Comparison type (string: "pairwise", "trio", "quartet")
- Timestamp: When comparison was made (time.Time)
- Duration: Time spent on comparison (time.Duration)
- Skipped: Whether comparison was skipped (bool)
- SkipReason: Why comparison was skipped (string, optional)

**Validation Rules**:

- ProposalIDs must reference existing proposals
- WinnerID must be in ProposalIDs if not skipped
- Rankings must contain all ProposalIDs if provided
- Method must match number of proposals

**Relationships**:

- Belongs to one Session
- References multiple Proposals
- Creates EloUpdates for rating changes

### EloUpdate

Records rating changes from a single comparison.

**Attributes**:

- ID: Unique update identifier (string)
- ComparisonID: Parent comparison (string)
- ProposalID: Affected proposal (string)
- OldRating: Rating before comparison (float64)
- NewRating: Rating after comparison (float64)
- RatingDelta: Change amount (NewRating - OldRating) (float64)
- KFactor: K-factor used for this calculation (int)

**Relationships**:

- Belongs to one Comparison
- Updates one Proposal

### ConvergenceMetrics

Tracks session progress and convergence indicators.

**Attributes**:

- SessionID: Parent session identifier (string)
- TotalComparisons: Number of comparisons performed (int)
- AvgRatingChange: Rolling average of rating changes (float64)
- RatingVariance: Variance in recent rating changes (float64)
- RankingStability: Percentage of stable top-N positions (float64)
- CoveragePercentage: Percentage of meaningful pairs compared (float64)
- ConvergenceScore: Overall convergence indicator 0-1 (float64)
- LastCalculated: When metrics were last updated (time.Time)

**Relationships**:

- Belongs to one Session
- Updated after each Comparison

### MatchupHistory

Tracks comparison pairings to optimize future matchup selection.

**Attributes**:

- SessionID: Parent session identifier (string)
- ProposalA: First proposal ID (string)
- ProposalB: Second proposal ID (string)
- ComparisonCount: Times this pair has been compared (int)
- LastCompared: Most recent comparison timestamp (time.Time)
- RatingDifferenceHistory: Rating gaps at each comparison ([]float64)
- InformationGain: Measured impact on ranking stability (float64)

**Relationships**:

- Belongs to one Session
- References two Proposals

### RatingBin

Groups proposals by rating ranges for strategic matchup selection.

**Attributes**:

- SessionID: Parent session identifier (string)
- BinIndex: Numeric bin identifier (int)
- MinRating: Lower bound of rating range (float64)
- MaxRating: Upper bound of rating range (float64)
- ProposalIDs: Proposals currently in this bin ([]string)
- LastUpdated: When bin assignments were recalculated (time.Time)

**Relationships**:

- Belongs to one Session
- Contains multiple Proposals

### SessionConfig

Configuration settings for a ranking session.

**Attributes**:

- CSVConfig: Input CSV column mapping (CSVConfig)
- EloConfig: Rating calculation settings (EloConfig)
- UIConfig: Interface preferences (UIConfig)
- ExportConfig: Output format settings (ExportConfig)

**Validation Rules**:

- All nested configs must be valid
- Required CSV columns must be mapped

### CSVConfig

Defines how to parse input CSV files.

**Attributes**:

- IDColumn: Column name for proposal ID (string)
- TitleColumn: Column name for title (string)
- AbstractColumn: Column name for abstract (string, optional)
- SpeakerColumn: Column name for speaker (string, optional)
- ScoreColumn: Column name for existing score (string, optional)
- CommentColumn: Column name for reviewer comments (string, optional)
- ConflictColumn: Column name for conflict tags (string, optional)
- HasHeader: Whether CSV has header row (bool)
- Delimiter: CSV field separator (rune, default comma)

**Validation Rules**:

- IDColumn and TitleColumn required
- Column names must be unique
- Delimiter must be valid CSV separator

### EloConfig

Settings for Elo rating calculations.

**Attributes**:

- InitialRating: Starting rating for new proposals (float64, default 1500)
- KFactor: Rating change sensitivity (int, default 32)
- MinRating: Minimum allowed rating (float64, default 0)
- MaxRating: Maximum allowed rating (float64, default 3000)
- OutputMin: Minimum output scale value (float64)
- OutputMax: Maximum output scale value (float64)
- UseDecimals: Whether output uses decimal places (bool)

**Validation Rules**:

- InitialRating between MinRating and MaxRating
- KFactor must be positive
- OutputMin < OutputMax
- MinRating < MaxRating

### UIConfig

Terminal interface preferences.

**Attributes**:

- ComparisonMode: Default comparison type (string: "pairwise", "trio", "quartet")
- ShowProgress: Display progress indicators (bool)
- ShowConfidence: Display rating confidence (bool)
- AutoSave: Automatically save session (bool)
- AutoSaveInterval: Save frequency (time.Duration)

**Validation Rules**:

- ComparisonMode must be valid type
- AutoSaveInterval must be positive if AutoSave enabled

## Data Relationships

```text
Session 1---* Proposal
Session 1---* Comparison  
Session 1---1 ConvergenceMetrics
Session 1---* MatchupHistory
Session 1---* RatingBin
Comparison *---* Proposal (through ProposalIDs)
Comparison 1---* EloUpdate
EloUpdate *---1 Proposal
Session 1---1 SessionConfig
MatchupHistory *---2 Proposal (ProposalA, ProposalB)
RatingBin *---* Proposal (through ProposalIDs)
```

## Storage Strategy

### JSON Session Files

- Sessions serialized to JSON for persistence
- File naming: `session_<id>_<timestamp>.json`
- Atomic writes to prevent corruption
- Backup rotation for safety

### CSV Input/Output

- Input: Parse according to CSVConfig
- Output: Maintain original structure with updated scores
- Export: Generate clean ranking lists

### Audit Logs

- Append-only log of all comparisons
- JSON Lines format for easy parsing
- Separate file per session: `audit_<session_id>.jsonl`
- Never modified, only appended

## Concurrency Considerations

### Thread Safety

- Single-threaded TUI operations (no concurrent access needed)
- File operations use proper locking
- Session state modifications are atomic

### Data Consistency

- All rating updates applied as complete transactions
- Session saves include all related data
- Audit logs written immediately after each comparison

## Performance Optimizations

### Memory Usage

- Proposals loaded fully (acceptable for target scale)
- Lazy loading for large abstracts if needed
- Periodic garbage collection hints

### Query Patterns

- Pre-sorted proposal lists for ranking display
- Cached comparison candidate selection
- Efficient conflict-of-interest filtering

## Migration Strategy

### Backward Compatibility

- Session format versioning
- Graceful degradation for missing fields
- Migration utilities for format changes

### Data Import

- Support multiple CSV formats
- Validation and error reporting
- Preview mode before full import
