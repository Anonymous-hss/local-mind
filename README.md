# LocalMind ⚡

LocalMind is an open-source, local-first AI coding assistant that runs entirely on your machine using [Ollama](https://ollama.ai/). It brings code completion and intelligent suggestions directly to your IDE without sending your code to the cloud.

## Features (Community Edition)

- **Local Code Completion:** Fast, privacy-preserving autocomplete powered by local LLMs (e.g., `qwen2.5-coder`).
- **Inline Suggestions:** Refactor, optimize, explain, and fix code without leaving your editor.
- **Zero Cloud Dependency:** Your code stays on your machine. No APIs, no subscriptions.

## Installation

1. **Install Ollama:** Follow the instructions at [ollama.ai](https://ollama.ai) to install and run Ollama.
2. **Pull a Model:** We recommend starting with a fast, capable coder model:
   ```bash
   ollama pull qwen2.5-coder:1.5b
   ```
3. **Install Extension:** Install the LocalMind extension from the VS Code Marketplace.

## Usage

Once installed, start editing code. LocalMind will automatically connect to your local Ollama instance and provide completions as you type. You can also use inline actions (CodeLens) to refactor or explain functions.

## LocalMind Pro 🚀

Looking for more power? **LocalMind Pro** offers a premium suite of features for advanced agentic coding:

- **Multi-File Agent:** Plan and execute complex tasks across multiple files.
- **Repository Memory:** Auto-infers your tech stack, architecture, and coding conventions.
- **Self-Healing & Recovery:** AI critic reviews plans, and automatic recovery fixes logic errors.
- **Operator Dashboard:** Track task history, success rates, strategy learning, and scoring metrics.

*(LocalMind Pro is a separate private repository/service built on top of this open-source core.)*

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) to get started with local development.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
