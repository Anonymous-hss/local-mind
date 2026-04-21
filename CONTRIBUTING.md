# Contributing to LocalMind

Welcome! We are excited that you are interested in contributing to LocalMind. This document will guide you through the process of setting up your development environment and submitting contributions.

## Code of Conduct
We expect all community members to interact with respect and professionalism.

## Getting Started

### Prerequisites
- [Go](https://golang.org/doc/install) (1.21 or later)
- [Node.js](https://nodejs.org/en/download/) (v18 or later)
- [npm](https://www.npmjs.com/get-npm)
- [Visual Studio Code](https://code.visualstudio.com/)
- [Ollama](https://ollama.ai/) installed and running locally

### Development Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/localmind/localmind.git
   cd localmind
   ```

2. **Install Extension Dependencies:**
   ```bash
   cd packages/extension
   npm install
   ```

3. **Compile the Core Engine:**
   ```bash
   cd ../../packages/core
   go build ./cmd/localmind
   ```

4. **Run the Extension:**
   - Open the workspace in VS Code.
   - Press `F5` to open a new VS Code window with the extension loaded in development mode.

## Architecture Overview
- **Core Engine (Go):** Located in `packages/core`. This handles communication with Ollama, AST parsing, and context building.
- **VS Code Extension (TypeScript):** Located in `packages/extension`. This provides the UI and IDE integration, communicating with the core engine via IPC over standard I/O.

## Submitting Pull Requests
1. Fork the repository.
2. Create a new branch for your feature or bugfix (`git checkout -b feature/your-feature-name`).
3. Make your changes and ensure all tests pass (`go test ./...`).
4. Commit your changes with clear, descriptive commit messages.
5. Push your branch to your fork.
6. Open a Pull Request from your branch to the `main` branch of this repository.

## Upgrading to Pro
LocalMind offers a Pro version with advanced features like multi-file agents, task planning, repo memory, error recovery, and strategy learning. The community edition acts as the foundational layer. If you are looking to contribute to those premium features, please note they live in a separate, private repository.
