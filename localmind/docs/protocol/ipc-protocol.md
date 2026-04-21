# LocalMind IPC Protocol

## Overview

Communication between the VS Code extension and the Core Engine uses **length-prefixed JSON** over **STDIO**.

## Transport Layer

### Why STDIO?
- Zero network overhead
- No port conflicts
- Simple process lifecycle management
- Cross-platform compatible

### Message Framing

Each message is framed as:

```
[4-byte length (big-endian uint32)][JSON payload]
```

Example:
```
\x00\x00\x00\x1c{"id":"abc","type":"ping",...}
```

## Message Structure

### Request (Extension → Core)

```json
{
  "id": "uuid-v4",
  "timestamp": 1705312800000,
  "type": "ping | completion | suggestion | agent | cancel",
  "payload": { ... }
}
```

### Response (Core → Extension)

```json
{
  "id": "uuid-v4",
  "timestamp": 1705312800050,
  "requestId": "original-request-uuid",
  "type": "pong | result | stream | error | cancelled",
  "payload": { ... }
}
```

## Request Types

### `ping`
Health check and Ollama status.

**Request:**
```json
{ "id": "...", "timestamp": ..., "type": "ping" }
```

**Response:**
```json
{
  "id": "...", "requestId": "...", "type": "pong",
  "payload": { "version": "0.0.1", "ollamaStatus": "connected" }
}
```

---

### `completion`
Ultra-fast autocomplete (latency budget: <150ms).

**Request:**
```json
{
  "id": "...", "timestamp": ..., "type": "completion",
  "payload": {
    "prefix": "func main() {\n    fmt.",
    "suffix": "\n}",
    "language": "go",
    "filePath": "cmd/main.go",
    "maxTokens": 50
  }
}
```

**Response:**
```json
{
  "id": "...", "requestId": "...", "type": "result",
  "payload": {
    "content": "Println(\"Hello, World!\")",
    "latencyMs": 87,
    "model": "qwen2.5-coder:1.5b"
  }
}
```

---

### `suggestion`
Code improvement suggestions (latency budget: <1s).

**Request:**
```json
{
  "id": "...", "timestamp": ..., "type": "suggestion",
  "payload": {
    "code": "for i := 0; i < len(arr); i++ { ... }",
    "context": "// Processing user data...",
    "language": "go",
    "suggestionType": "refactor"
  }
}
```

---

### `agent`
Multi-file task execution (task-dependent latency).

**Request:**
```json
{
  "id": "...", "timestamp": ..., "type": "agent",
  "payload": {
    "task": "Add error handling to all database calls",
    "files": ["internal/db/user.go", "internal/db/order.go"],
    "workspaceRoot": "/path/to/project"
  }
}
```

---

### `cancel`
Cancel an in-flight request.

**Request:**
```json
{
  "id": "...", "timestamp": ..., "type": "cancel",
  "payload": { "requestId": "request-to-cancel-uuid" }
}
```

**Response:**
```json
{ "id": "...", "requestId": "...", "type": "cancelled" }
```

## Streaming Responses

For longer outputs (suggestions, agent), responses may be streamed:

```json
{ "type": "stream", "payload": { "chunk": "func ", "done": false } }
{ "type": "stream", "payload": { "chunk": "hello()", "done": false } }
{ "type": "stream", "payload": { "chunk": " { ... }", "done": true } }
```

## Error Handling

### Error Codes

| Code | Description |
|------|-------------|
| `INTERNAL_ERROR` | Unexpected error in core engine |
| `TIMEOUT` | Request exceeded latency budget |
| `CANCELLED` | Request was cancelled |
| `INVALID_REQUEST` | Malformed request |
| `MODEL_UNAVAILABLE` | Ollama model not loaded |
| `CONTEXT_TOO_LARGE` | Input exceeds token limit |
| `LATENCY_EXCEEDED` | Soft warning, result still returned |

### Error Response

```json
{
  "id": "...", "requestId": "...", "type": "error",
  "payload": {
    "code": "TIMEOUT",
    "message": "Completion exceeded 150ms budget",
    "details": { "actualMs": 203 }
  }
}
```

## Latency Budgets

| Request Type | Target | Hard Cap | Action on Exceed |
|-------------|--------|----------|------------------|
| `completion` | 100ms | 150ms | Cancel + return partial |
| `suggestion` | 500ms | 1000ms | Return with warning |
| `agent` | N/A | Task-defined | User-configurable |
| `ping` | 10ms | 100ms | Log warning |

## Cancellation Flow

1. Extension sends `cancel` request with `requestId`
2. Core engine attempts to abort in-flight work
3. Core responds with `cancelled` for the original request
4. Any partial results are discarded

## Implementation Notes

### Extension (TypeScript)
- Use `child_process.spawn()` with `stdio: ['pipe', 'pipe', 'pipe']`
- Parse length prefix, then JSON
- Track pending requests with timeout callbacks

### Core (Go)
- Read length prefix from stdin
- Dispatch to appropriate handler
- Write length-prefixed response to stdout
- Use goroutines with context for cancellation
