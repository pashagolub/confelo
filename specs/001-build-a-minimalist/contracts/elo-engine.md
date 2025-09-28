# Elo Rating Engine Contract

**Package**: `pkg/elo`  
**Description**: Core Elo rating calculations for multi-proposal comparisons

## Public API

### Types

#### `Engine`

Core Elo rating engine with configurable parameters.

```go
type Engine struct {
    InitialRating float64
    KFactor       int
    MinRating     float64
    MaxRating     float64
}
```

**Methods**:

##### `NewEngine(config EloConfig) *Engine`

Creates a new Elo rating engine with specified configuration.

**Parameters**:

- `config EloConfig`: Engine configuration parameters

**Returns**: Configured Engine instance

**Example**:

```go
config := EloConfig{
    InitialRating: 1500.0,
    KFactor: 32,
    MinRating: 0.0,
    MaxRating: 3000.0,
}
engine := NewEngine(config)
```

##### `CalculatePairwise(winner, loser Rating) (newWinner, newLoser Rating)`

Calculates new ratings for pairwise comparison.

**Parameters**:

- `winner Rating`: Current rating of winning proposal
- `loser Rating`: Current rating of losing proposal

**Returns**: Updated ratings for both proposals

**Algorithm**: Standard Elo formula with expected scores based on rating difference

**Example**:

```go
winner := Rating{ID: "prop1", Score: 1600.0}
loser := Rating{ID: "prop2", Score: 1400.0}
newWinner, newLoser := engine.CalculatePairwise(winner, loser)
```

##### `CalculateMultiway(rankings []Rating) []Rating`

Calculates new ratings for multi-proposal comparison (trio/quartet).

**Parameters**:

- `rankings []Rating`: Proposals ordered by performance (best to worst)

**Returns**: Updated ratings for all proposals

**Algorithm**: Converts multi-way comparison into series of pairwise comparisons with position-based weighting

**Example**:

```go
rankings := []Rating{
    {ID: "prop1", Score: 1500.0}, // 1st place
    {ID: "prop2", Score: 1450.0}, // 2nd place  
    {ID: "prop3", Score: 1550.0}, // 3rd place
}
updated := engine.CalculateMultiway(rankings)
```

##### `ScaleRating(rating float64, outputMin, outputMax float64) float64`

Converts internal Elo rating to specified output scale.

**Parameters**:

- `rating float64`: Internal Elo rating
- `outputMin float64`: Minimum value of output scale
- `outputMax float64`: Maximum value of output scale

**Returns**: Scaled rating value

**Example**:

```go
// Convert 1600 Elo to 0-10 scale
scaled := engine.ScaleRating(1600.0, 0.0, 10.0)
// Result: approximately 7.3
```

#### `Rating`

Represents a proposal's rating information.

```go
type Rating struct {
    ID         string
    Score      float64
    Confidence float64
    Games      int
}
```

**Fields**:

- `ID`: Unique proposal identifier
- `Score`: Current Elo rating
- `Confidence`: Statistical confidence (0.0-1.0)
- `Games`: Number of comparisons participated in

#### `ComparisonResult`

Result of a rating calculation with audit information.

```go
type ComparisonResult struct {
    Updates    []RatingUpdate
    Method     ComparisonMethod
    Timestamp  time.Time
    Duration   time.Duration
}
```

**Fields**:

- `Updates`: Rating changes for each affected proposal
- `Method`: Type of comparison performed
- `Timestamp`: When calculation was performed
- `Duration`: Time taken for calculation

#### `RatingUpdate`

Individual rating change record.

```go
type RatingUpdate struct {
    ProposalID string
    OldRating  float64
    NewRating  float64
    Delta      float64
    KFactor    int
}
```

#### `ComparisonMethod`

Enumeration of supported comparison types.

```go
type ComparisonMethod string

const (
    Pairwise ComparisonMethod = "pairwise"
    Trio     ComparisonMethod = "trio"
    Quartet  ComparisonMethod = "quartet"
)
```

## Mathematical Specifications

### Pairwise Elo Calculation

**Expected Score Formula**:

```
E_A = 1 / (1 + 10^((R_B - R_A) / 400))
E_B = 1 / (1 + 10^((R_A - R_B) / 400))
```

**Rating Update Formula**:

```
R'_A = R_A + K * (S_A - E_A)
R'_B = R_B + K * (S_B - E_B)
```

Where:

- `R_A, R_B`: Current ratings
- `R'_A, R'_B`: New ratings  
- `E_A, E_B`: Expected scores
- `S_A, S_B`: Actual scores (1 for winner, 0 for loser)
- `K`: K-factor (rating change sensitivity)

### Multi-way Conversion Algorithm

For N proposals ranked 1st to Nth:

1. **Generate pairwise comparisons**: Each proposal "plays against" all lower-ranked proposals
2. **Apply position weights**: Higher positions receive stronger victory scores
3. **Calculate expected vs actual performance**: Based on relative rating differences
4. **Update ratings**: Sum of all pairwise rating changes

**Position Weighting**:

- 1st place: Defeats all others with weight 1.0
- 2nd place: Defeats 3rd+ with weight 0.8  
- 3rd place: Defeats 4th+ with weight 0.6
- Etc.

### Confidence Calculation

**Formula**:

```
Confidence = min(1.0, games / 20.0)
```

Where:

- `games`: Number of comparisons proposal has participated in
- 20: Minimum games for full confidence (configurable)

### Rating Bounds

**Enforcement**:

- Ratings clamped to [MinRating, MaxRating] range after each update
- Prevents runaway inflation or deflation
- Maintains meaningful rating differences

## Error Handling

### Input Validation

**Invalid Ratings**:

- Negative scores
- NaN or infinite values
- Scores outside configured bounds

**Invalid Comparisons**:

- Empty proposal lists
- Duplicate proposal IDs
- Missing required fields

**Error Types**:

```go
var (
    ErrInvalidRating     = errors.New("rating value is invalid")
    ErrEmptyComparison   = errors.New("comparison contains no proposals")
    ErrDuplicateProposal = errors.New("proposal appears multiple times")
    ErrInvalidKFactor    = errors.New("K-factor must be positive")
    ErrInvalidBounds     = errors.New("min rating must be less than max rating")
)
```

## Performance Specifications

### Time Complexity

- Pairwise calculation: O(1)
- Multi-way calculation: O(N²) where N is number of proposals in comparison
- Rating scaling: O(1)

### Memory Usage

- Constant memory per calculation
- No allocation for pairwise comparisons
- Linear temporary allocation for multi-way comparisons

### Benchmark Targets

- Pairwise comparison: <1ms
- 4-way comparison: <5ms
- 100 proposal session: <1s total calculation time

## Testing Contract

### Unit Test Coverage

**Mathematical Correctness**:

- Standard Elo formulas produce expected results
- Multi-way decomposition matches manual pairwise calculations
- Rating bounds are enforced correctly
- Scaling functions preserve relative ordering

**Property-Based Tests**:

- Rating sum conservation (total rating points remain constant)
- Transitivity (A > B > C implies A > C after sufficient comparisons)
- Convergence (ratings stabilize with repeated comparisons)
- Symmetry (swapping identical ratings produces no change)

**Edge Cases**:

- Extreme rating differences (3000 vs 0)
- Minimum and maximum K-factors
- Single proposal "comparisons"
- All proposals tied

### Integration Test Requirements

**Real-world Scenarios**:

- Tournament simulation with known outcomes
- Conference data with actual reviewer preferences
- Large-scale stress testing (1000+ proposals)
- Performance benchmarking under load

### Mock Data Specifications

**Test Proposals**:

```go
var TestProposals = []Rating{
    {ID: "high", Score: 2000.0, Confidence: 1.0, Games: 50},
    {ID: "medium", Score: 1500.0, Confidence: 0.8, Games: 40},
    {ID: "low", Score: 1000.0, Confidence: 0.6, Games: 30},
    {ID: "untested", Score: 1500.0, Confidence: 0.0, Games: 0},
}
```

**Expected Outcomes**:

- High vs Low: High wins, minimal rating change due to expected outcome
- Low vs High: High wins, larger rating changes due to rating difference
- Medium vs Medium: Ratings change by exactly K-factor/2

## Configuration Validation

### Valid Ranges

- `InitialRating`: [0.0, 10000.0]
- `KFactor`: [1, 100]
- `MinRating`: [0.0, InitialRating)
- `MaxRating`: (InitialRating, 10000.0]

### Default Values

```go
var DefaultConfig = EloConfig{
    InitialRating: 1500.0,
    KFactor:       32,
    MinRating:     0.0,
    MaxRating:     3000.0,
}
```

### Validation Rules

- MinRating < InitialRating < MaxRating
- KFactor must be positive integer
- All values must be finite numbers

## Algorithmic Optimization Extensions

### Intelligent Matchup Selection

#### `GetOptimalMatchup() *Matchup`

Returns the most informative next comparison based on rating distributions.

**Algorithm**:

1. Create rating bins (50-point ranges)
2. Prioritize within-bin or adjacent-bin matchups
3. Track comparison history to avoid redundancy
4. Occasional cross-bin calibration (10-20% of comparisons)

**Return Type**:

```go
type Matchup struct {
    ProposalA      string
    ProposalB      string
    ExpectedClose  bool     // true if ratings within 50 points
    Priority       int      // 1-5, higher = more informative
    Information    float64  // expected information gain
}
```

#### `GetRatingBins(binSize float64) map[int][]string`

Groups proposals into rating bins for strategic matchup selection.

**Parameters**:

- `binSize`: Rating range per bin (recommended: 50.0)

**Returns**: Map of bin index to proposal IDs

### Convergence Detection

#### `CheckConvergence() *ConvergenceStatus`

Evaluates multiple stopping criteria and recommends session termination.

**Stopping Criteria**:

1. **Rating Stability**: Average rating change <5 points per comparison
2. **Ranking Stability**: Top N positions unchanged for 3+ rounds  
3. **Variance Threshold**: Rating change variance approaches zero
4. **Minimum Coverage**: Each proposal in 5-10 comparisons

**Return Type**:

```go
type ConvergenceStatus struct {
    ShouldStop           bool
    Confidence           float64           // 0.0-1.0
    RemainingEstimate    int              // estimated comparisons needed
    CriteriaMet          map[string]bool  // which criteria are satisfied
    Metrics              *ConvergenceMetrics
}

type ConvergenceMetrics struct {
    AvgRatingChange      float64
    RatingVariance       float64  
    RankingStability     float64  // % of top-N unchanged
    CoveragePercentage   float64  // % of meaningful pairs compared
    RecentComparisons    int      // comparisons in stability window
}
```

### Progress Measurement

#### `GetProgressMetrics() *ProgressMetrics`

Returns real-time progress indicators for TUI display.

**Return Type**:

```go
type ProgressMetrics struct {
    TotalComparisons      int
    CoverageComplete      float64  // 0.0-1.0
    ConvergenceRate       float64  // change per comparison
    EstimatedRemaining    int      // comparisons to convergence
    TopNStable           int      // consecutive stable top positions
    ConfidenceScores     map[string]float64  // per proposal
}
```

#### `UpdateComparisonHistory(comparison ComparisonResult)`

Tracks comparison results for convergence analysis.

**Parameters**:

- `comparison`: Result of latest rating calculation

**Side Effects**: Updates internal metrics used by convergence detection

### Enhanced Rating Types

#### Extended `Rating` struct

```go
type Rating struct {
    ID              string
    Score           float64
    Confidence      float64
    Games           int
    RecentChanges   []float64  // last N rating changes
    BinIndex        int        // current rating bin
    LastCompared    time.Time  // for matchup history
}
```

#### `ComparisonHistory`

Tracks pairing history to avoid redundant matchups.

```go
type ComparisonHistory struct {
    Pairs           map[string]int     // "propA:propB" -> count
    LastRound       map[string]time.Time
    AvoidanceWindow time.Duration      // minimum time between repeat pairings
}
```

## Optimization Performance Targets

### Algorithm Efficiency

- **Matchup Selection**: O(N log N) where N is proposal count
- **Convergence Check**: O(1) with sliding window metrics
- **Progress Calculation**: O(N) for confidence score updates
- **Bin Management**: O(N) for rating redistribution

### Memory Overhead

- **Comparison History**: O(N²) worst case, O(N) typical
- **Convergence Metrics**: O(1) with fixed-size sliding windows  
- **Progress Tracking**: O(N) for per-proposal statistics

### Accuracy Targets

- **Convergence Detection**: 95% accuracy in predicting stable rankings
- **Information Gain**: 20% reduction in comparisons needed vs random pairing
- **Confidence Scoring**: Correlation >0.9 with actual ranking stability
