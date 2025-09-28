
# Implementation Plan: Conference Talk Ranking Application

**Branch**: `001-build-a-minimalist` | **Date**: 2025-09-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-build-a-minimalist/spec.md`

## Execution Flow (/plan command scope)

```
1. Load feature spec from Input path
   → If not found: ERROR "No feature spec at {path}"
2. Fill Technical Context (scan for NEEDS CLARIFICATION)
   → Detect Project Type from file system structure or context (web=frontend+backend, mobile=app+api)
   → Set Structure Decision based on project type
3. Fill the Constitution Check section based on the content of the constitution document.
4. Evaluate Constitution Check section below
   → If violations exist: Document in Complexity Tracking
   → If no justification possible: ERROR "Simplify approach first"
   → Update Progress Tracking: Initial Constitution Check
5. Execute Phase 0 → research.md
   → If NEEDS CLARIFICATION remain: ERROR "Resolve unknowns"
6. Execute Phase 1 → contracts, data-model.md, quickstart.md, agent-specific template file (e.g., `CLAUDE.md` for Claude Code, `.github/copilot-instructions.md` for GitHub Copilot, `GEMINI.md` for Gemini CLI, `QWEN.md` for Qwen Code or `AGENTS.md` for opencode).
7. Re-evaluate Constitution Check section
   → If new violations: Refactor design, return to Phase 1
   → Update Progress Tracking: Post-Design Constitution Check
8. Plan Phase 2 → Describe task generation approach (DO NOT create tasks.md)
9. STOP - Ready for /tasks command
```

**IMPORTANT**: The /plan command STOPS at step 7. Phases 2-4 are executed by other commands:

- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary

Privacy-first terminal application for ranking conference talk proposals using Elo rating system. Go-based implementation with modular architecture: Core Elo Engine for calculations, TUI Layer using tview/bubbletea for interface, Data Layer with CSV input/JSON storage, Matchup Journal for audit trails, and flexible Configuration system. Supports pairwise and multi-proposal comparisons with local-only data processing, conflict-of-interest handling, and transparent ranking export.

## Technical Context

**Language/Version**: Go 1.21+  
**Primary Dependencies**: tview or bubbletea (TUI), jessevdk/go-flags (CLI), standard library for CSV/JSON  
**Storage**: Local files - CSV input, JSON/YAML for internal state, audit logs  
**Testing**: Go standard testing, testify for enhanced assertions  
**Target Platform**: Cross-platform terminal environments (Linux, macOS, Windows)
**Project Type**: single - standalone CLI application  
**Performance Goals**: <200ms response time, <100MB memory usage, handle 50-200 proposals efficiently  
**Constraints**: <200ms p95, <100MB memory, offline-capable, zero external network dependencies  
**Scale/Scope**: Medium conference pools (50-200 proposals), single reviewer sessions, audit trail storage

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Code Quality**: Go linting (golangci-lint), gofmt formatting, static analysis planned
- [x] **Test-First**: TDD approach with unit tests for Elo engine, integration tests for TUI  
- [x] **UX Consistency**: Terminal UI patterns, consistent key bindings, clear error messages
- [x] **Performance**: <200ms p95 response time, <100MB memory targets specified
- [x] **Minimal Design**: Modular architecture justified, multi-comparison complexity needed for usability
- [x] **Quality Standards**: 90% test coverage requirement for core Elo logic and data handling
- [x] **Workflow Integration**: Fits clarify→plan→tasks→implement cycle
- [ ] **Workflow Integration**: Fits clarify→plan→tasks→implement cycle

## Project Structure

### Documentation (this feature)

```
specs/[###-feature]/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
```

### Source Code (repository root)
```
cmd/
└── confelo/
    ├── main.go                 # Application entry point
    └── main_test.go            # CLI integration tests
pkg/
├── elo/
│   ├── engine.go              # Core Elo rating calculations
│   ├── engine_test.go         # Elo algorithm unit tests
│   ├── comparison.go          # Multi-proposal comparison logic
│   └── comparison_test.go     # Multi-proposal tests
├── data/
│   ├── proposal.go            # Proposal data model
│   ├── proposal_test.go       # Proposal validation tests
│   ├── session.go             # Ranking session management
│   ├── session_test.go        # Session management tests
│   ├── storage.go             # CSV/JSON persistence
│   ├── storage_test.go        # Storage integration tests
│   ├── config.go              # Configuration handling
│   └── config_test.go         # Configuration tests
├── tui/
│   ├── app.go                 # Main TUI application
│   ├── app_test.go            # TUI integration tests
│   ├── screens/
│   │   ├── comparison.go      # Comparison interface
│   │   ├── comparison_test.go # Comparison screen tests
│   │   ├── ranking.go         # Results display
│   │   ├── ranking_test.go    # Ranking screen tests
│   │   ├── setup.go           # Initial configuration
│   │   └── setup_test.go      # Setup screen tests
│   └── components/
│       ├── carousel.go        # Proposal carousel display
│       ├── carousel_test.go   # Carousel component tests
│       ├── progress.go        # Progress indicators
│       └── progress_test.go   # Progress component tests
└── journal/
    ├── audit.go               # Matchup logging
    ├── audit_test.go          # Audit trail tests
    ├── export.go              # Results export
    └── export_test.go         # Export functionality tests
testdata/
├── proposals.csv              # Sample proposal data
└── expected_results.json      # Expected test outputs
go.mod
go.sum
README.md
LICENSE
```

**Structure Decision**: Single Go project structure selected. Modular package organization with `cmd/` for executables, `pkg/` for core logic (elo, data, tui, journal), and `tests/` for integration testing. This approach supports the constitutional principles of simplicity, testability, and maintainability while enabling clear separation of concerns between Elo calculations, TUI interface, and data management.

## Phase 0: Outline & Research

1. **Extract unknowns from Technical Context** above:
   - For each NEEDS CLARIFICATION → research task
   - For each dependency → best practices task
   - For each integration → patterns task

2. **Generate and dispatch research agents**:

   ```
   For each unknown in Technical Context:
     Task: "Research {unknown} for {feature context}"
   For each technology choice:
     Task: "Find best practices for {tech} in {domain}"
   ```

3. **Consolidate findings** in `research.md` using format:
   - Decision: [what was chosen]
   - Rationale: [why chosen]
   - Alternatives considered: [what else evaluated]

**Output**: research.md with all NEEDS CLARIFICATION resolved

## Phase 1: Design & Contracts

*Prerequisites: research.md complete*

1. **Extract entities from feature spec** → `data-model.md`:
   - Entity name, fields, relationships
   - Validation rules from requirements
   - State transitions if applicable

2. **Generate API contracts** from functional requirements:
   - For each user action → endpoint
   - Use standard REST/GraphQL patterns
   - Output OpenAPI/GraphQL schema to `/contracts/`

3. **Generate contract tests** from contracts:
   - One test file per endpoint
   - Assert request/response schemas
   - Tests must fail (no implementation yet)

4. **Extract test scenarios** from user stories:
   - Each story → integration test scenario
   - Quickstart test = story validation steps

5. **Update agent file incrementally** (O(1) operation):
   - Run `.specify/scripts/powershell/update-agent-context.ps1 -AgentType copilot`
     **IMPORTANT**: Execute it exactly as specified above. Do not add or remove any arguments.
   - If exists: Add only NEW tech from current plan
   - Preserve manual additions between markers
   - Update recent changes (keep last 3)
   - Keep under 150 lines for token efficiency
   - Output to repository root

**Output**: data-model.md, /contracts/*, failing tests, quickstart.md, agent-specific file

## Phase 2: Task Planning Approach

*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:

- Load `.specify/templates/tasks-template.md` as base
- Generate Go-specific tasks from Phase 1 design docs (contracts, data model, quickstart)
- CLI interface contract → CLI handler and flag parsing tests [P]
- Elo engine contract → core calculation and multi-way algorithm tests [P]
- Data model entities → struct definitions and validation tests [P]
- Quickstart scenarios → integration test scenarios [P]
- TUI components → interface and interaction tests
- Configuration system → parsing and validation logic
- Implementation tasks following Go project structure (pkg/, cmd/)

**Ordering Strategy**:

- Setup: Go module initialization, dependency management, linting tools
- Tests first: Unit tests for elo package, data package, integration tests
- Core implementation: Elo calculations, data structures, configuration
- TUI layer: Interface components, screen flow, user interactions  
- Integration: CLI parsing, session management, export functionality
- Quality gates: Performance tests, coverage validation, manual testing
- Mark [P] for parallel execution (different packages/files)

**Estimated Output**: 20-25 numbered tasks optimized for Go development workflow

**Key Go-Specific Considerations**:

- Package-based task organization (pkg/elo, pkg/data, pkg/tui, cmd/)
- Interface-driven design for testability and modularity
- Concurrent-safe session management
- Cross-platform terminal compatibility testing
- Static binary compilation and distribution

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation

*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)  
**Phase 4**: Implementation (execute tasks.md following constitutional principles)  
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |

## Progress Tracking

*This checklist is updated during execution flow*

**Phase Status**:

- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [x] Phase 2: Task planning complete (/plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:

- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented (none required)

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*
