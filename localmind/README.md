# LocalMind

**A Local-First, High-Performance Coding Agent powered by Ollama.**

> *"The smallest intelligence at the right moment."*

LocalMind is a privacy-focused, speed-oriented coding assistant that runs entirely on your machine. It leverages local LLMs via [Ollama](https://ollama.ai) to provide intelligent code completion, refactoring, and explanations without sending your code to the cloud.

![Banner](https://github.com/localmind/localmind/raw/main/packages/extension/images/banner.png)

## Features

- **🚀 Local-First Architecture**: Zero data egress. Your code never leaves your machine.
- **⚡ High Performance**: Optimized for latency.
  - Autocomplete: <100ms
  - Suggestions: <500ms
- **🧠 Context-Aware Intelligence**:
  - **Budget-Based Context**: Intelligently selects relevant context (cursor, files, git diffs) to fit model limits.
  - **RAG (Semantic Search)**: Embeddings-based retrieval for repo-wide knowledge.
  - **Multi-File Refactoring**: Understands dependencies and plans atomic changes across files.
- **🛡️ Safety & Control**:
  - **Safe Mode**: Requires confirmation for destructive actions.
  - **Sanitization**: Shell output is sanitized; secrets are redacted.
  - **Human-in-the-Loop**: Inspect and approve every plan before execution.

## Requirements

- **VS Code** 1.85.0 or higher
- **[Ollama](https://ollama.ai)** installed and running locally
- A model pulled in Ollama (e.g., `llama3`, `codellama`, `mistral`)

## Installation

1. Install the extension from the VS Code Marketplace.
2. Ensure Ollama is running (`ollama serve`).
3. Open a project in VS Code.
4. Run `LocalMind: Ping Core Engine` from the Command Palette to verify connection.

## Configuration

| Setting | Description | Default |
|---|---|---|
| `localmind.ollamaUrl` | URL of the Ollama API | `http://localhost:11434` |
| `localmind.model` | Model to use for generation | `llama3` |
| `localmind.contextBudget` | Max context size in characters | `12000` |
| `localmind.safeMode` | Safety level (`safe`, `warn`, `dev`) | `warn` |

## Architecture

LocalMind is built with a split architecture for maximum performance and stability:

- **Extension (TypeScript)**: UI, Editor integration, IPC client.
- **Core Engine (Go)**: Heavy lifting, LLM interaction, RAG, Analysis.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.
