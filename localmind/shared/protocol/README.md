# LocalMind IPC Protocol

This directory contains the shared message protocol schema used for communication between the VS Code extension and the Core Engine.

## Files

- **messages.schema.json** - JSON Schema defining all request/response types
- See [../../docs/protocol/ipc-protocol.md](../../docs/protocol/ipc-protocol.md) for human-readable documentation

## Message Flow

```
VS Code Extension                    Core Engine
      │                                   │
      │  ─────── Request (JSON) ───────►  │
      │                                   │
      │  ◄────── Response (JSON) ───────  │
      │                                   │
```

## Transport

Messages are length-prefixed (4-byte big-endian uint32) and sent over STDIO.
