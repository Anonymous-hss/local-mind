# LocalMind VS Code Extension

The VS Code extension provides the user interface for LocalMind.

## Structure

```
extension/
├── src/
│   ├── extension.ts      # Entry point
│   ├── core/             # Core engine management
│   │   └── client.ts     # IPC client
│   ├── providers/        # VS Code providers
│   │   ├── completion.ts # Inline completion
│   │   └── commands.ts   # Command handlers
│   └── utils/            # Utilities
├── package.json
└── tsconfig.json
```

## Development

```bash
# Install dependencies
npm install

# Compile
npm run compile

# Watch mode
npm run watch

# Run extension in VS Code
# Press F5 in VS Code
```

## IPC Communication

The extension spawns the core engine as a child process and communicates via STDIO using length-prefixed JSON messages.

See [../../shared/protocol/](../../shared/protocol/) for message schemas.
