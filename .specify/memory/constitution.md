<!--
Sync Impact Report:
Version: v1.0.0 (initial constitution)
Added principles:
- I. Code Quality Excellence
- II. Test-First Development (NON-NEGOTIABLE)
- III. User Experience Consistency
- IV. Performance Requirements
- V. Minimal Workflow Design
Added sections:
- Quality Standards
- Development Workflow
Templates requiring updates: ✅ validated - all templates already align with TDD and quality principles
Follow-up TODOs: None - all placeholders filled
-->

# Confelo Constitution

## Core Principles

### I. Code Quality Excellence

Every code change MUST maintain high quality standards through automated enforcement.
Code MUST pass linting, formatting, and static analysis before merge. Documentation
MUST be updated when public interfaces change. Clean, readable code is preferred over
clever solutions. Refactoring MUST be continuous to prevent technical debt accumulation.

### II. Test-First Development (NON-NEGOTIABLE)

TDD is mandatory: Tests written → Tests fail → Implementation → Tests pass.
Red-Green-Refactor cycle MUST be strictly enforced. No production code without
failing tests first. Contract tests MUST validate all API boundaries. Integration
tests MUST cover critical user workflows. Test coverage below 90% blocks deployment.

### III. User Experience Consistency

User interfaces MUST follow consistent patterns across all features. Error messages
MUST be clear, actionable, and user-friendly. Performance feedback (loading states,
progress indicators) MUST be provided for operations >200ms. Accessibility standards
MUST be met for all interactive elements. UX changes require usability validation.

### IV. Performance Requirements

Response times MUST be <200ms p95 for interactive operations. Resource usage MUST
be monitored and optimized continuously. Database queries MUST be analyzed for
efficiency. Memory leaks MUST be prevented through proper cleanup. Performance
regressions block deployment and require immediate remediation.

### V. Minimal Workflow Design

Features MUST justify their value in improved usability or accuracy. Complexity
MUST be justified with clear benefits over simpler alternatives. User workflows
MUST require minimum steps to accomplish goals. UI elements MUST serve clear
purposes - no decorative complexity. Every added feature MUST pass the "essential" test.

## Quality Standards

Code quality enforcement through automated tools and gates. Static analysis,
dependency scanning, and security checks run on every commit. Code review
required for all changes with focus on maintainability and performance.
Documentation updates mandatory for API changes, with examples and usage patterns.

## Development Workflow

Specification-driven development workflow using clarify → plan → tasks → implement cycle.
Constitution compliance verified at each phase gate. Test-first approach with
failing tests before implementation. Performance validation required before
feature completion. User experience review mandatory for all UI changes.

## Governance

This constitution supersedes all other development practices and coding standards.
Amendments require documented justification, team approval, and migration plan for
existing code. All code reviews MUST verify constitutional compliance. Complexity
MUST be justified against simpler alternatives. Performance and UX requirements
are non-negotiable and block deployment if violated.

**Version**: v1.0.0 | **Ratified**: 2025-09-28 | **Last Amended**: 2025-09-28
