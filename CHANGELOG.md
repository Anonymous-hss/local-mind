# Changelog

All notable changes to LocalMind will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-01-29

### Added
- **Core Engine**
  - Orchestrator with task routing and cancellation management
  - Completion engine with Ollama integration and caching
  - Suggestion engine for code refactoring proposals
  - Agent engine for multi-file task execution with rollback
  - AST parser with multi-language support (Go, TypeScript, JavaScript, Python)
  - Repository memory with architecture inference and convention detection

- **VS Code Extension**
  - Inline code completion with debouncing
  - Agent task planning and execution commands
  - Step-by-step approval workflow
  - Rollback support for agent changes
  - Status bar integration

- **IPC Protocol**
  - Length-prefixed JSON messaging
  - Request/response tracking with cancellation
  - Timeout enforcement (150ms/300ms/3s budgets)

- **Quality & Stability**
  - Golden tests for deterministic verification
  - Performance benchmarks with latency budgets
  - Panic recovery middleware
  - Graceful shutdown handlers
  - Cross-platform packaging (Windows, macOS, Linux)

### Security
- User approval required for file modifications
- Action logging for audit trail
- No autonomous loops - all operations are step-by-step

## [Unreleased]

### Planned
- Tree-sitter CGO support for enhanced parsing
- GitHub Actions CI/CD pipeline
- Marketplace publishing
