# LocalMind Core Engine

The core engine is written in Go and handles:

- **Orchestration** - Task routing, latency control, cancellation
- **Model Runtime** - Ollama client, adapter loading
- **Context Management** - Live context, repo memory, change context
- **Code Intelligence** - AST parsing, dependency graph

## Structure

```
core/
├── cmd/
│   └── localmind/     # Main binary entry point
├── internal/
│   ├── orchestrator/  # Core orchestration logic
│   ├── completion/    # Completion engine
│   ├── suggestion/    # Suggestion engine
│   ├── agent/         # Agent engine
│   ├── context/       # Context management
│   ├── memory/        # Repo memory
│   ├── ast/           # AST parsing (Tree-sitter)
│   ├── model/         # Ollama client
│   └── ipc/           # IPC message handling
├── pkg/
│   └── protocol/      # Shared protocol types
├── go.mod
└── go.sum
```

## Building

```bash
go build -o bin/localmind ./cmd/localmind
```

## IPC Communication

The core engine communicates with the VS Code extension via STDIO using length-prefixed JSON messages.

See [../../shared/protocol/](../../shared/protocol/) for message schemas.
