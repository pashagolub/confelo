# Research: Conference Talk Ranking Application

**Feature**: 001-build-a-minimalist  
**Date**: 2025-09-28  
**Status**: Complete

## Technical Decisions

### TUI Library Selection: tview vs bubbletea

**Decision**: Recommend **tview** for initial implementation  
**Rationale**:

- More traditional component-based architecture aligns with carousel requirements
- Built-in widgets (Table, List, Form) reduce custom development
- Mature library with stable API
- Better suited for data-heavy interfaces like proposal comparison
- Direct terminal manipulation for responsive UI

**Alternatives considered**:

- bubbletea: More modern Elm-like architecture but requires more custom component development
- Survey/promptui: Too simplistic for multi-panel interface requirements
- Custom termbox/tcell: Unnecessary complexity for MVP

### Elo Rating Implementation

**Decision**: Standard Chess Elo with pairwise decomposition for multi-proposal comparisons  
**Rationale**:

- Use proven Chess Elo mathematics for accuracy and transparency
- Multi-proposal comparisons (trios, quartets) decompose into individual pairwise games
- Trio: A vs B, B vs C, C vs A (3 games total with win/loss/draw results)
- Quartet: A vs B, A vs C, A vs D, B vs C, B vs D, C vs D (6 games total)
- Each game updates ratings independently using standard Elo formulas
- Custom scaling layer converts final ratings to desired output range (e.g., 0-10)

**Alternatives considered**:

- Complex multi-way ranking algorithms: Unnecessary when pairwise decomposition works
- Swiss tournament system: Overly complex for simple ranking needs
- Position-based weighting: Less transparent than pure pairwise games

### Configuration Management

**Decision**: jessevdk/go-flags with YAML configuration files  
**Rationale**:

- go-flags provides clean CLI argument parsing
- YAML configuration for CSV column mapping and rating scales
- No heavy dependencies, aligns with minimalist principles
- Supports both command-line and file-based configuration

**Alternatives considered**:

- Standard flag package: Limited validation and help generation
- Viper: Too heavyweight for simple configuration needs
- Pure YAML only: Poor UX for one-off usage

### Data Persistence Strategy

**Decision**: CSV input, JSON internal state, structured audit logs  
**Rationale**:

- CSV input matches common proposal export formats
- JSON for session state provides structured data with Go marshal support
- Separate audit log format for transparency and debugging
- All local storage maintains privacy requirements

**Alternatives considered**:

- SQLite: Overkill for simple ranking data
- Binary formats: Poor transparency and debugging
- Plain text logs: Harder to parse for analysis

## Performance Considerations

### Memory Management

- Proposals loaded fully in memory (acceptable for 50-200 items)
- Lazy loading for large abstracts/descriptions if needed
- Garbage collection optimization for frequent UI updates

### Response Time Optimization

- Pre-compute next comparison candidates during current comparison
- Cache rendered TUI components to avoid re-rendering
- Asynchronous auto-save to avoid UI blocking

### Scalability Limits

- Current design targets 50-200 proposals (conference typical)
- Memory usage linear with proposal count
- Pairwise decomposition scales well: trio = 3 games, quartet = 6 games
- Efficient comparison strategy reduces total sessions needed vs pure pairwise

## Integration Patterns

### CSV Column Flexibility

- Configurable column mapping (id, score, comments, position-based extras)
- Robust parsing with error handling for malformed data
- Preview mode for CSV validation before processing

### Export Format Standardization

- Maintain original CSV structure with updated scores
- Add metadata header with ranking methodology
- Support multiple output formats (CSV, JSON, plain text rankings)

## Security and Privacy

### Data Isolation

- No network communication whatsoever
- All processing in local memory and filesystem
- Audit logs contain no personally identifiable information beyond proposals

### Conflict of Interest Handling

- Configurable exclusion rules based on proposal metadata
- Manual skip functionality during comparisons
- Transparent logging of excluded comparisons

## Development Workflow

### Testing Strategy

- Unit tests for Elo calculation accuracy
- Integration tests for full ranking sessions
- Property-based testing for Elo invariants (transitivity, consistency)
- Manual testing scenarios with real conference data

### Quality Gates

- golangci-lint for code quality
- 90% test coverage for core elo and data packages
- Performance benchmarks for large proposal sets
- UX validation with actual conference organizers

## Dependencies Analysis

### Core Dependencies

- `github.com/rivo/tview`: TUI framework (~50KB)
- `github.com/jessevdk/go-flags`: CLI parsing (~30KB)
- Standard library only for CSV, JSON, file operations

### Development Dependencies

- `github.com/stretchr/testify`: Enhanced testing assertions
- `github.com/golangci/golangci-lint`: Code quality analysis

**Total binary size estimate**: <10MB static binary (excellent for distribution)

## Risk Assessment

### Technical Risks

- **Very Low**: Elo mathematical complexity (standard Chess Elo with pairwise decomposition)
- **Low**: TUI performance with large datasets
- **Medium**: CSV parsing edge cases and malformed data

### Mitigation Strategies

- Use proven Chess Elo implementation libraries or algorithms
- Comprehensive unit testing for pairwise game calculations
- Performance benchmarking during development
- Robust input validation and error handling

## Algorithmic Optimizations

### Intelligent Matchup Selection

**Decision**: Rating-based binning with strategic pairing

**Strategy**:

- Create rating bins (50-point ranges) after each comparison round
- Prioritize matchups within same or adjacent bins for meaningful comparisons
- Occasional cross-bin matches for calibration (10-20% of total comparisons)
- Avoid pairing proposals with >200 rating point differences
- Track comparison history to prevent redundant matchups

**Benefits**:

- More efficient convergence to stable rankings
- Avoids wasted comparisons between clear favorites and underdogs
- Maximizes information gain per comparison session

### Convergence Detection and Stopping Criteria

**Decision**: Multi-metric convergence detection for early termination

**Primary Criteria**:

- Rating stability: Stop when average rating change <5 points per comparison
- Ranking stability: Top N proposals unchanged for 3+ consecutive rounds
- Variance tracking: Stop when rating change variance approaches zero

**Secondary Criteria**:

- Minimum coverage: Each proposal participates in 5-10 comparisons
- Time-based limits: Maximum session duration or comparison count
- Manual override: Reviewer satisfaction with current rankings

**Implementation**:

- Real-time convergence monitoring during comparison sessions
- Progress indicators showing stability metrics
- Automatic suggestions to stop when criteria met
- Export capabilities at any point during process

### Progress Measurement

**Decision**: Multi-dimensional progress tracking

**Metrics**:

- **Convergence Rate**: Standard deviation of recent rating changes
- **Ranking Stability**: Percentage of top-10 unchanged in last N rounds
- **Coverage Completeness**: Percentage of meaningful pairs compared
- **Confidence Scores**: Statistical confidence in current rankings per proposal

**Visualization**:

- Real-time progress bars during TUI sessions
- Convergence graphs in export reports
- Estimated completion time based on current convergence rate
- Quality indicators for each proposal's rating confidence
