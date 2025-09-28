# Quickstart Guide: Conference Talk Ranking Application

**Feature**: 001-build-a-minimalist  
**Date**: 2025-09-28  
**Purpose**: End-to-end validation scenarios for manual testing

## Prerequisites

- Go 1.21+ installed
- Sample CSV file with conference talk proposals
- Terminal environment with 80x24 minimum size

## Test Scenario 1: Basic Pairwise Ranking

**Objective**: Verify core functionality with simple pairwise comparisons

### Setup

1. Create test data file `test_proposals.csv`:

```csv
id,title,speaker,abstract
prop001,Go Concurrency Patterns,Alice Johnson,Advanced patterns for goroutines and channels
prop002,Microservices with Kubernetes,Bob Smith,Scaling applications with container orchestration  
prop003,Machine Learning in Go,Carol Williams,Building ML pipelines with native Go libraries
prop004,API Design Best Practices,David Brown,RESTful API patterns and GraphQL integration
```

2. Create configuration file `test_config.yaml`:

```yaml
csv:
  id_column: "id"
  title_column: "title"
  speaker_column: "speaker" 
  abstract_column: "abstract"
  has_header: true

elo:
  initial_rating: 1000.0
  k_factor: 40
  output_min: 1.0
  output_max: 10.0
  use_decimals: true

ui:
  comparison_mode: "pairwise"
  show_progress: true
  auto_save: true
```

### Execution Steps

1. **Start new session**:

   ```bash
   confelo start -i test_proposals.csv -c test_config.yaml --session-name "QuickTest1"
   ```

2. **Verify setup screen**:
   - [ ] Shows 4 proposals loaded
   - [ ] Displays correct configuration summary
   - [ ] Shows estimated comparisons needed (~6 for complete ranking)

3. **Perform comparisons** (press Enter to confirm each):
   - Compare "Go Concurrency" vs "Microservices": Select Go Concurrency
   - Compare "Machine Learning" vs "API Design": Select Machine Learning  
   - Compare "Go Concurrency" vs "Machine Learning": Select Machine Learning
   - Compare "Microservices" vs "API Design": Select API Design
   - Compare "Go Concurrency" vs "API Design": Select Go Concurrency
   - Compare "Microservices" vs "Machine Learning": Select Machine Learning

4. **View current rankings** (press 'r'):
   - [ ] Machine Learning should be ranked #1 (highest score)
   - [ ] Go Concurrency should be ranked #2
   - [ ] API Design should be ranked #3
   - [ ] Microservices should be ranked #4 (lowest score)
   - [ ] Scores should be in 1.0-10.0 range with decimals

5. **Save and export** (press 's' then 'e'):
   - [ ] Session saves successfully
   - [ ] Export generates `rankings_QuickTest1.csv`
   - [ ] Output file contains original data with updated scores

**Expected Results**:

- Rankings reflect comparison choices
- Elo ratings converted to 1-10 scale correctly
- All operations complete within 200ms response time
- Memory usage stays under 50MB

## Test Scenario 2: Multi-proposal Comparison

**Objective**: Validate trio/quartet comparison functionality

### Setup

Use same test data as Scenario 1, but modify config:

```yaml
ui:
  comparison_mode: "trio"
```

### Execution Steps

1. **Start session with trio mode**:

   ```bash
   confelo start -i test_proposals.csv -c test_config.yaml --session-name "QuickTest2" --comparison-mode trio
   ```

2. **Perform trio comparisons**:
   - Compare "Go Concurrency", "Microservices", "Machine Learning"
     - Game 1: Go Concurrency vs Microservices → Winner: Go Concurrency
     - Game 2: Microservices vs Machine Learning → Winner: Machine Learning  
     - Game 3: Machine Learning vs Go Concurrency → Winner: Machine Learning
   - Compare "Go Concurrency", "API Design", "Machine Learning"
     - Game 1: Go Concurrency vs API Design → Winner: API Design
     - Game 2: API Design vs Machine Learning → Winner: Machine Learning
     - Game 3: Machine Learning vs Go Concurrency → Winner: Machine Learning

3. **Verify ranking calculation**:
   - [ ] Each individual game updates Elo ratings using standard Chess Elo formulas
   - [ ] Final rankings reflect cumulative game results (Machine Learning should be highest)
   - [ ] Comparison history shows individual pairwise games within trio format

**Expected Results**:

- Machine Learning maintains top ranking (won most individual games)
- More games per comparison session (trio = 3 games vs 1 pairwise game)
- Individual game results follow standard Elo calculations
- Final rankings reflect cumulative win/loss record across all games

## Test Scenario 3: Session Resume and Export

**Objective**: Test persistence and recovery functionality

### Execution Steps

1. **Start session and make partial progress**:

   ```bash
   confelo start -i test_proposals.csv --session-name "ResumeTest"
   ```

   - Perform 2-3 comparisons
   - Press 's' to save
   - Press 'q' to quit

2. **Resume session**:

   ```bash
   confelo resume ResumeTest
   ```

   - [ ] Session loads with previous comparisons intact
   - [ ] Current rankings reflect previous decisions
   - [ ] Can continue making comparisons seamlessly

3. **Export in multiple formats**:

   ```bash
   confelo export ResumeTest --format csv -o results.csv
   confelo export ResumeTest --format json -o results.json --include-stats
   confelo export ResumeTest --format text -o results.txt
   ```

   - [ ] CSV maintains original structure with scores
   - [ ] JSON includes detailed statistics and metadata
   - [ ] Text format provides clean ranking list

## Test Scenario 4: Conflict of Interest Handling

**Objective**: Verify COI exclusion functionality

### Setup

Create COI test data `coi_proposals.csv`:

```csv
id,title,speaker,abstract,conflict_tag
prop001,Go Concurrency,Alice Johnson,Advanced patterns,reviewer_company
prop002,Microservices,Bob Smith,Container orchestration,
prop003,Machine Learning,Carol Williams,ML pipelines,reviewer_company  
prop004,API Design,David Brown,RESTful patterns,
```

Configuration with COI handling:

```yaml
csv:
  conflict_column: "conflict_tag"

# COI exclusion would be configured separately
```

### Execution Steps

1. **Configure reviewer COI**:
   - Set reviewer to exclude "reviewer_company" tagged proposals
   - Start ranking session

2. **Verify exclusion behavior**:
   - [ ] Proposals with matching conflict tags are not presented together
   - [ ] Can still rank non-conflicted proposals
   - [ ] Conflicted proposals receive default/interpolated ratings

## Test Scenario 5: Large Dataset Performance

**Objective**: Validate performance with realistic dataset size

### Setup

Generate larger test dataset (50-100 proposals) using script or tool

### Execution Steps

1. **Load large dataset**:

   ```bash
   confelo start -i large_proposals.csv --session-name "PerfTest"
   ```

2. **Monitor performance**:
   - [ ] Initial load completes within 5 seconds
   - [ ] Each comparison responds within 200ms
   - [ ] Memory usage stays under 100MB
   - [ ] Auto-save operations don't block UI

3. **Complete substantial ranking**:
   - Perform 20-30 comparisons across dataset
   - [ ] Rankings converge to reasonable order
   - [ ] Application remains responsive throughout
   - [ ] Session save/resume works with large data

## Test Scenario 6: Error Handling

**Objective**: Verify graceful handling of error conditions

### Error Cases to Test

1. **Invalid CSV file**:

   ```bash
   confelo start -i nonexistent.csv
   ```

   - [ ] Clear error message about file not found
   - [ ] Suggests validation command

2. **Malformed CSV data**:
   Create CSV with missing quotes, extra commas, etc.
   - [ ] Validation identifies specific parsing errors
   - [ ] Provides line numbers and suggestions

3. **Insufficient proposals**:
   CSV with only 1 proposal
   - [ ] Error message about minimum requirements
   - [ ] Graceful exit without crash

4. **Invalid configuration**:
   Config with negative K-factor, invalid column names
   - [ ] Validation catches configuration errors
   - [ ] Specific error messages for each issue

5. **Corrupted session file**:
   Manually corrupt a saved session JSON
   - [ ] Graceful fallback or error recovery
   - [ ] Option to start fresh or restore backup

## Validation Checklist

After completing all scenarios:

### Functional Requirements

- [ ] All CSV formats parse correctly
- [ ] Standard Chess Elo calculations produce consistent results  
- [ ] Multi-proposal comparisons decompose into correct pairwise games
- [ ] Individual game results (win/loss/draw) update ratings appropriately
- [ ] Session persistence is reliable
- [ ] Export formats are correct and complete
- [ ] COI handling functions properly

### Performance Requirements  

- [ ] Startup time <5 seconds for 100 proposals
- [ ] UI response time <200ms consistently
- [ ] Memory usage <100MB for typical datasets
- [ ] File I/O operations are atomic and safe

### User Experience Requirements

- [ ] Interface is intuitive and distraction-free
- [ ] Error messages are clear and actionable
- [ ] Progress indicators work correctly
- [ ] Keyboard navigation is complete and logical

### Quality Requirements

- [ ] No crashes or data corruption under normal use
- [ ] Graceful handling of all error conditions
- [ ] Consistent results across multiple runs
- [ ] Clean code organization and documentation

## Acceptance Criteria

**PASS** if all scenarios complete successfully with:

- No functional defects
- Performance within specified limits  
- User experience meets constitutional requirements
- Error handling is comprehensive and user-friendly

**FAIL** if any critical functionality is missing or severely degraded

## Post-Testing Actions

1. **Document any issues found**:
   - Create bug reports for defects
   - Note performance concerns
   - Record UX improvement suggestions

2. **Update configuration if needed**:
   - Adjust default parameters based on testing
   - Refine error messages based on user feedback

3. **Prepare for production**:
   - Clean up test artifacts
   - Finalize documentation
   - Package for distribution
