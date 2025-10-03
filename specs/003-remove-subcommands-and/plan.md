
# Implementation Plan: Remove Subcommands and Implement Automatic Mode Detection

**Branch**: `003-remove-subcommands-and` | **Date**: October 2, 2025 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-remove-subcommands-and/spec.md`

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

Remove all subcommands (start, resume, export, list, validate) and implement automatic mode detection based on session name existence. Eliminate all configuration file support - use only command-line parameters. When --session-name is new, start new session; when existing, resume. Always run in interactive mode. Validation occurs at startup.

## Technical Context

**Language/Version**: Go 1.25+ (using modern features like `any` type alias)  
**Primary Dependencies**: tview (TUI), jessevdk/go-flags (CLI), standard library (CSV/JSON)  
**Storage**: JSON session files in `sessions/` directory, CSV input files  
**Testing**: Go standard testing package, testify for assertions  
**Target Platform**: Cross-platform CLI (Windows, Linux, macOS)
**Project Type**: single (CLI application with TUI)  
**Performance Goals**: <200ms startup time, <100ms UI responsiveness  
**Constraints**: <50MB memory usage, filesystem-based storage only, CLI-only configuration  
**Scale/Scope**: Single-user tool, ~1000 proposals max, local filesystem only

**User Requirements Integration**: Follow KISS principle, avoid bloated code, use modern Go 1.25 features including `any` instead of `any`. Configuration must be done only with command-line parameters - remove all traces of confelo.yaml and config file support.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Code Quality**: Go fmt, go vet, staticcheck already in place; removing subcommands and config files simplifies codebase significantly
- [x] **Test-First**: TDD approach planned - write failing tests for new CLI behavior first
- [x] **UX Consistency**: Simplified CLI interface improves consistency; error messages will follow existing patterns
- [x] **Performance**: <200ms startup specified; removing subcommand parsing and config file loading reduces complexity and improves performance
- [x] **Minimal Design**: **PERFECT ALIGNMENT** - This feature removes unnecessary complexity (5 subcommands + config file system) for simpler CLI-only approach
- [x] **Quality Standards**: 90% test coverage requirement acknowledged; existing tests will be updated
- [x] **Workflow Integration**: Fits clarify→plan→tasks→implement cycle; no constitutional violations detected

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
    ├── main.go           # CLI entry point - MAJOR CHANGES: remove subcommands, remove config file support
    └── main_test.go      # CLI tests - UPDATE: test new automatic mode detection

pkg/
├── data/
│   ├── cli.go           # CLI configuration - MAJOR UPDATE: CLI-only config, no file loading
│   ├── cli_test.go      # CLI config tests - UPDATE: test new CLI-only structure  
│   ├── config.go        # App configuration - REMOVE: eliminate config file support entirely
│   ├── session.go       # Session management - UPDATE: add mode detection logic
│   └── session_test.go  # Session tests - UPDATE: test auto mode detection
├── elo/                 # Elo rating engine - NO CHANGES
└── tui/                 # Terminal UI - UPDATE: handle new entry modes
    ├── app.go           # Main TUI app - UPDATE: handle CLI-only initialization
    ├── screens/         # UI screens - NO CHANGES (export/list already exist)
    └── components/      # UI components - NO CHANGES

sessions/                # Session storage - NO CHANGES
testdata/               # Test data - NO CHANGES
```

**Structure Decision**: Single Go project structure maintained. Primary changes in `cmd/confelo/main.go` (remove subcommands, remove config file support) and `pkg/data/` (eliminate config.go file entirely, update cli.go for CLI-only configuration). TUI requires minimal changes since export/list screens already exist.

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
- Generate tasks from CLI contract, session contract, and data model
- CLI restructuring → remove subcommands, eliminate config file support [P]
- Session detection → new mode detection logic [P] 
- TUI updates → handle CLI-only initialization (no new screens needed)
- Test updates → CLI behavior tests, session detection tests
- Integration tests → end-to-end workflows from quickstart.md

**Ordering Strategy**:

- TDD order: Write failing tests first, then implementation
- Dependency order: CLI parsing → Session detection → TUI integration
- Mark [P] for parallel execution (independent files)
- Core changes before TUI enhancements
- Tests before corresponding implementation

**Specific Task Categories**:

1. **CLI Restructuring (6-8 tasks)**
   - Remove subcommand structs and handlers
   - Remove all config file loading logic (loadConfiguration, etc.)
   - Create unified CLI options structure  
   - Update argument parsing logic for global flags only
   - Implement mode detection integration
   - Remove confelo.yaml references

2. **Session Detection (4-5 tasks)**
   - Create SessionDetector interface and implementation
   - Add session file validation logic
   - Implement error handling for corrupted sessions
   - Add directory creation logic

3. **TUI Updates (2-3 tasks)**
   - Update TUI initialization for CLI-only parameters
   - Replace config parameter passing with CLI options
   - Test existing TUI functionality with simplified CLI

4. **Testing (8-10 tasks)**
   - CLI behavior tests for new simplified interface
   - Session detection unit tests
   - Integration tests for start/resume flows
   - TUI integration tests with CLI-only flow
   - Performance validation tests
   - Error handling scenario tests

5. **Code Cleanup (3-4 tasks)**
   - Remove pkg/data/config.go file entirely
   - Remove config file parsing from main.go
   - Update all function signatures to use CLI options instead of config
   - Remove confelo.yaml from documentation and examples

**Estimated Output**: 23-30 numbered, ordered tasks in tasks.md following TDD principles with emphasis on removing config file complexity

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
- [x] Complexity deviations documented (None - feature reduces complexity significantly)

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*
