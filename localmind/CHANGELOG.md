# Change Log

All notable changes to the "LocalMind" extension will be documented in this file.

## [0.1.0] - 2026-02-20

### Added
- **Local-First Architecture**: Core engine in Go, Extension in TypeScript.
- **Ollama Integration**: Support for local LLMs (Llama 3, CodeLlama, etc.).
- **RAG System**: Semantic search with `rag/` package (Embedder, VectorStore, Retriever).
- **Multi-File Intelligence**: `multifile/` package for dependency-aware refactoring.
- **Context Budgeting**: Smart context selection strategy.
- **Safety Features**: "Safe Mode" and output sanitization.

### Changed
- **Performance**: Optimized latency budgets (<100ms autocomplete).
- **Structure**: Split into `packages/core` and `packages/extension`.

### Fixed
- Various stability improvements and bug fixes during development.
