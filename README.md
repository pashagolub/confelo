# Confelo - Conference Talk Ranking System

A privacy-first terminal user interface application for ranking conference talk proposals using the Elo rating system. Features intelligent matchup selection, convergence detection, and comprehensive audit trails.

## Features

- **Privacy-First**: All data processing happens locally - no external network dependencies
- **Elo Rating System**: Uses proven Chess Elo mathematics for reliable ranking
- **Intelligent Comparisons**: Smart matchup selection reduces total comparison time
- **Multi-Proposal Support**: Handle pairwise, trio, and quartet comparisons efficiently
- **Convergence Detection**: Automatic stopping criteria based on rating stability
- **Audit Trails**: Complete comparison history for transparency and reproducibility
- **Flexible Input/Output**: CSV import/export with configurable formats
- **Cross-Platform**: Works on Linux, macOS, and Windows terminal environments

## Quick Start

### Prerequisites

- Go 1.21 or later
- Terminal with color support (recommended)

### Installation

```bash
# Clone the repository
git clone https://github.com/pashagolub/confelo.git
cd confelo

# Build the application
go build -o confelo ./cmd/confelo

# Run with sample data
./confelo compare --input testdata/proposals.csv
```

### Basic Usage

1. **Prepare your data**: Create a CSV file with conference proposals
   - Required columns: `id`, `title`, `speaker`
   - Optional: `abstract`, `track`, custom metadata columns

2. **Start ranking session**:

   ```bash
   ./confelo compare --input proposals.csv
   ```

3. **Make comparisons**: Use the interactive TUI to compare proposals
   - Arrow keys to navigate
   - Enter to select/confirm choices
   - Tab to switch between comparison modes
   - Esc to return to main menu

4. **Export results**:

   ```bash
   ./confelo export --session latest --output ranked_proposals.csv
   ```

## Development

### Project Structure

```sh
cmd/confelo/          # CLI application entry point
pkg/
├── elo/             # Core Elo rating engine
├── data/            # Data models and persistence
├── tui/             # Terminal user interface
└── journal/         # Audit logging and export
testdata/            # Sample data for testing
```

### Build and Test

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run linting
golangci-lint run

# Build for current platform
go build ./cmd/confelo

# Cross-compilation examples
GOOS=linux GOARCH=amd64 go build ./cmd/confelo
GOOS=darwin GOARCH=amd64 go build ./cmd/confelo
GOOS=windows GOARCH=amd64 go build ./cmd/confelo
```

### Quality Standards

This project follows constitutional principles for code quality:

- **Test Coverage**: >90% for core logic, >70% for UI components
- **Code Quality**: Zero golangci-lint violations
- **Performance**: <200ms p95 response time, <100MB memory usage
- **Documentation**: All public APIs documented
- **Cross-Platform**: Tested on Linux, macOS, Windows

## Configuration

### CLI Options

```bash
confelo compare [flags]
  --input string     Input CSV file path
  --config string    Configuration file path (optional)
  --k-factor int     Elo K-factor (default: 32)
  --initial int      Initial rating (default: 1500)

confelo export [flags]
  --session string   Session ID or 'latest'
  --output string    Output CSV file path
  --format string    Export format: csv, json, yaml (default: csv)
```

### Configuration File

Create `.confelo.yml` in your working directory:

```yaml
elo:
  k_factor: 32
  initial_rating: 1500
  min_rating: 0
  max_rating: 3000

csv:
  id_column: "id"
  title_column: "title"
  speaker_column: "speaker"
  abstract_column: "abstract"

convergence:
  stability_threshold: 5.0
  min_comparisons: 10
  max_comparisons: 1000
```

## Algorithm Details

### Elo Rating System

Uses standard Chess Elo mathematics:

- Expected score: `E_A = 1 / (1 + 10^((R_B - R_A) / 400))`
- Rating update: `R'_A = R_A + K * (S_A - E_A)`

### Multi-Proposal Comparisons

- **Trio**: Decomposes into 3 pairwise games (A vs B, B vs C, C vs A)
- **Quartet**: Decomposes into 6 pairwise games (all combinations)
- Maintains mathematical consistency with pairwise Elo calculations

### Convergence Detection

Automatic stopping based on multiple criteria:

- Rating stability (changes <5 points per comparison)
- Ranking stability (top N unchanged for 3+ rounds)
- Coverage completeness (minimum comparisons per proposal)
- Variance thresholds (rating change variance approaches zero)

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Follow TDD: Write tests first, then implementation
4. Ensure all quality gates pass (`go test ./...` and `golangci-lint run`)
5. Commit changes (`git commit -m 'Add amazing feature'`)
6. Push to branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Elo rating system invented by Arpad Elo for chess rankings
- Inspired by conference program committee workflows
- Built with Go's excellent standard library and terminal UI ecosystem