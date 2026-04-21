# ADR-001: Foundation & Constraints Lock

**Status:** Accepted  
**Date:** 2026-01-15  
**Decision Makers:** LocalMind Founder

---

## Context

LocalMind is a local-first coding assistant. Before writing any feature code, we must lock down architectural rules to prevent future tech debt and ensure the product remains true to its core principles.

---

## Decision

### 1. Core Principles (Non-Negotiable)

| # | Principle | Implication |
|---|-----------|-------------|
| 1 | **Local-First Only** | No cloud API calls for core features |
| 2 | **No Background Indexing** | No always-on repo scanning |
| 3 | **Latency > Intelligence** | Prefer fast small models over slow large ones |
| 4 | **Structure > Brute Context** | Use AST/structured memory, not massive context windows |
| 5 | **Predictability > Creativity** | Deterministic, reliable output |
| 6 | **Escalate Only When Required** | Use smallest capable model |

### 2. Architecture Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│                    VS Code Extension                         │
│                    (TypeScript)                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ Completion  │  │ Suggestion  │  │    Agent Commands   │  │
│  │  Provider   │  │  Provider   │  │                     │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
│         │                │                     │             │
│         └────────────────┼─────────────────────┘             │
│                          │                                   │
│                   ┌──────▼──────┐                           │
│                   │  IPC Client │ ← STDIO (length-prefixed) │
└───────────────────┴──────┬──────┴────────────────────────────┘
                           │
                           │ Named Pipes / STDIO
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                      Core Engine                             │
│                         (Go)                                 │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                    Orchestrator                          ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  ││
│  │  │   Latency   │  │    Task     │  │   Cancellation  │  ││
│  │  │  Scheduler  │  │   Router    │  │     Manager     │  ││
│  │  └─────────────┘  └─────────────┘  └─────────────────┘  ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────────┐  │
│  │  Completion   │ │  Suggestion   │ │      Agent        │  │
│  │    Engine     │ │    Engine     │ │     Engine        │  │
│  │  (isolated)   │ │               │ │                   │  │
│  └───────┬───────┘ └───────┬───────┘ └─────────┬─────────┘  │
│          │                 │                   │             │
│  ┌───────▼─────────────────▼───────────────────▼───────────┐│
│  │                   Model Router                           ││
│  │         (Single Base Model + LoRA Adapters)              ││
│  └──────────────────────────┬───────────────────────────────┘│
│                             │                                │
│  ┌──────────────────────────▼───────────────────────────────┐│
│  │                  Ollama Client                            ││
│  └───────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐ │
│  │  AST Engine     │  │   Repo Memory   │  │   Storage    │ │
│  │  (Tree-sitter)  │  │   (SQLite)      │  │   (SQLite)   │ │
│  └─────────────────┘  └─────────────────┘  └──────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

### 3. Module Isolation Rules

| Rule | Rationale |
|------|-----------|
| Completion engine must be isolated | Cannot be blocked by agent work |
| Extension cannot call Ollama directly | All model access through core engine |
| Core engine must be runnable standalone | Enables testing without VS Code |
| No shared mutable state between engines | Prevents race conditions |

### 4. Latency Budgets

| Feature | Target | Hard Cap | Enforcement |
|---------|--------|----------|-------------|
| Autocomplete | 100ms | **150ms** | Cancel + return partial |
| Suggestion | 500ms | **1000ms** | Return with LATENCY_EXCEEDED warning |
| Agent | N/A | User-defined | Configurable per task |
| Ping | 10ms | 100ms | Log warning only |

**Enforcement:** Latency budgets are enforced by the Orchestrator's Latency Scheduler. Requests exceeding hard caps are terminated.

### 5. Tech Stack

| Layer | Technology | Rationale |
|-------|------------|-----------|
| Extension | TypeScript | Native VS Code API |
| Core Engine | **Go** | Fast, simple, good concurrency |
| Model Runtime | Ollama | No need to re-implement inference |
| IPC | STDIO + Named Pipes | Zero network overhead |
| Storage | SQLite + JSON | Human-editable, Git-friendly |
| AST | Tree-sitter | Multi-language, fast, deterministic |

### 6. Model Strategy

```
┌─────────────────────────────────────────────┐
│           Single Base Model                  │
│         (e.g., qwen2.5-coder:7b)            │
├─────────────────────────────────────────────┤
│  LoRA Adapters (loaded on demand):          │
│  ├── completion-adapter (~50MB)             │
│  ├── suggestion-adapter (~50MB)             │
│  └── agent-adapter (~100MB)                 │
└─────────────────────────────────────────────┘
```

- **ONE base model** reduces storage footprint
- **Adapters are MBs, not GBs**
- **Dynamic loading** based on task type

### 7. Storage Footprint

| Item | Max Size |
|------|----------|
| Base model | ~4GB (single model) |
| Adapters | ~200MB total |
| Repo memory (per workspace) | <10MB |
| SQLite storage | <50MB |

### 8. Completion ≠ Repo Memory (Critical Rule)

> [!IMPORTANT]
> The **Completion Engine** and **Repo Memory** are **completely separate systems**.

| Completion Engine | Repo Memory |
|-------------------|-------------|
| Uses only prefix/suffix context | Uses structured architectural data |
| No database access | SQLite-backed |
| Zero latency penalty | Loaded on demand |
| ~200-400 tokens max | Full repo understanding |

**Why this matters:**
- Completion must be <150ms — cannot wait for repo scans
- Repo memory is expensive to build — should not block typing
- Mixing them creates unpredictable latency

**Enforcement:**
- Completion engine has **no import path** to repo memory
- Separate Go packages with no shared dependencies
- CI check: `go list -f '{{.Imports}}' ./internal/completion` must not include memory packages

### 9. Failure & Degradation Policy

| Failure Type | Behavior | User Feedback |
|--------------|----------|---------------|
| Ollama unavailable | All AI features disabled | Toast: "LocalMind: Ollama not running" |
| Model not loaded | Graceful skip | Inline: "Model loading..." |
| Completion timeout | Return empty | Silent (no disruption) |
| Suggestion timeout | Return partial | Warning badge |
| Agent failure | Rollback all changes | Error modal + diff |
| Core engine crash | Auto-restart (max 3) | Toast: "Restarting..." |
| IPC connection lost | Queue requests (5s) | Status bar indicator |

**Degradation Priority:**
1. Never crash VS Code
2. Never corrupt user files
3. Never block typing
4. Inform user only when actionable

### 10. Versioning & Compatibility Rules

**Protocol Version:** `1.0.0` (semver)

| Change Type | Version Bump | Compatibility |
|-------------|--------------|---------------|
| New optional field | PATCH | Backward compatible |
| New request type | MINOR | Old clients ignore |
| Breaking change | MAJOR | Requires migration |

**Version Negotiation:**
```json
// First message from extension
{ "type": "handshake", "protocolVersion": "1.0.0", "extensionVersion": "0.0.1" }

// Core response
{ "type": "handshake_ack", "protocolVersion": "1.0.0", "coreVersion": "0.0.1", "compatible": true }
```

**Rules:**
- Extension must check `compatible` before proceeding
- Core must reject requests if protocol version mismatch
- Unknown fields are **ignored**, not rejected (forward compatibility)

### 11. Security & Trust Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│                    TRUSTED ZONE                              │
│  ┌─────────────────┐      ┌─────────────────────────────┐   │
│  │   VS Code       │ IPC  │      Core Engine            │   │
│  │   Extension     │◄────►│  (runs as same user)        │   │
│  └─────────────────┘      └──────────────┬──────────────┘   │
│                                          │                   │
│                                   localhost only             │
│                                          │                   │
│                           ┌──────────────▼──────────────┐   │
│                           │        Ollama               │   │
│                           │   (localhost:11434)         │   │
│                           └─────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ✕ NO EXTERNAL NETWORK
```

**Trust Rules:**
| Rule | Enforcement |
|------|-------------|
| No outbound network calls | Firewall rules in CI tests |
| No telemetry without consent | Compile-time flag |
| No code leaves machine | No cloud APIs |
| Ollama must be localhost | Hardcoded check |
| File access scoped to workspace | Path validation |

**File System Boundaries:**
- Core can only read/write within workspace root
- No access to `~/.ssh`, `~/.aws`, or sensitive directories
- Explicit allowlist for config paths

### 12. Observability Rules

**What we log (always):**
| Event | Data | Purpose |
|-------|------|---------|
| Request start | type, id, timestamp | Latency tracking |
| Request end | id, duration_ms, status | Performance |
| Engine errors | error_code, message | Debugging |
| Model load/unload | model_name, duration | Resource tracking |

**What we NEVER log:**
- User code content
- File paths (except in debug mode)
- Prompt contents
- Model outputs

**Log Levels:**
| Level | When Used |
|-------|-----------|
| ERROR | Failures requiring user attention |
| WARN | Degraded performance, timeout |
| INFO | Lifecycle events (start, stop) |
| DEBUG | Detailed tracing (opt-in only) |

**Metrics (local only):**
- `completion_latency_ms` histogram
- `suggestion_latency_ms` histogram
- `model_load_time_ms` gauge
- `active_requests` gauge

**Storage:** Logs rotate daily, max 10MB retained.

### 13. Explicit Non-Goals

The following are **explicitly out of scope** for LocalMind:

| Non-Goal | Rationale |
|----------|-----------|
| Cloud-based inference | Violates local-first principle |
| Multi-user collaboration | Adds complexity, not core value |
| Chat interface | Focus on inline assistance |
| Code execution/REPL | Security risk, not core value |
| Git operations | User has existing tools |
| Project scaffolding | Other tools do this better |
| Language-specific IDE features | VS Code already provides |
| Training/fine-tuning UI | Out of scope for v1 |
| Browser-based interface | VS Code only for now |

**Why document non-goals?**
- Prevents feature creep
- Focuses development effort
- Sets clear user expectations

### 14. Configuration Policy

**Configuration Layers (priority order):**
1. **Command flags** (highest) — runtime overrides
2. **Workspace settings** — `.vscode/settings.json`
3. **User settings** — VS Code user preferences
4. **Defaults** — hardcoded sane defaults

**Configurable Settings:**

| Setting | Default | Range | Hot-reload |
|---------|---------|-------|------------|
| `localmind.ollamaUrl` | `http://localhost:11434` | URL | ✓ |
| `localmind.completionEnabled` | `true` | bool | ✓ |
| `localmind.completionModel` | `qwen2.5-coder:1.5b` | string | ✓ |
| `localmind.suggestionModel` | `qwen2.5-coder:7b` | string | ✓ |
| `localmind.maxCompletionTokens` | `50` | 10-100 | ✓ |
| `localmind.debugLogging` | `false` | bool | ✓ |

**Not Configurable (hardcoded):**
| Setting | Value | Reason |
|---------|-------|--------|
| Completion hard cap | 150ms | Core UX guarantee |
| IPC protocol | STDIO | Architectural decision |
| Ollama host | localhost | Security |

### 15. Testing Guarantees

**Test Pyramid:**
```
         ▲
        /│\        E2E Tests (few, slow)
       / │ \       - Full VS Code + Core + Ollama
      /  │  \
     /   │   \     Integration Tests (moderate)
    /    │    \    - IPC roundtrips, mock Ollama
   /     │     \
  /      │      \  Unit Tests (many, fast)
 /       │       \ - Pure functions, no I/O
─────────┴─────────
```

**Mandatory Coverage:**
| Area | Min Coverage | Test Type |
|------|--------------|-----------|
| IPC protocol parsing | 100% | Unit |
| Latency enforcement | 100% | Unit + Integration |
| Error handling | 90% | Unit |
| Cancellation | 100% | Integration |
| Overall | 80% | Combined |

**Golden Tests (snapshot tests):**
- Fixed input prompts → expected outputs
- Fail CI on unexpected diff
- Located in `packages/core/testdata/golden/`

**Performance Tests (required before release):**
| Metric | Pass Threshold | Fail Action |
|--------|----------------|-------------|
| Completion p50 | <80ms | Block release |
| Completion p99 | <150ms | Block release |
| Cold start | <2s | Warning |
| Memory usage | <500MB | Warning |

**Test Environments:**
- CI: Ubuntu, macOS, Windows
- Mock Ollama responses (no real model in CI)

---

## Constraints Locked

The following constraints are **frozen** and cannot be changed without a new ADR:

1. ❌ No cloud dependencies for core features
2. ❌ No background full-repo indexing
3. ❌ No direct Ollama calls from extension
4. ❌ No shared state between completion and agent engines
5. ✓ Completion engine must meet <150ms hard cap
6. ✓ Core engine must be testable without VS Code
7. ✓ All IPC uses length-prefixed JSON over STDIO

---

## Consequences

### Positive
- Clear boundaries prevent scope creep
- Latency budgets keep UX fast
- Modular design enables future changes
- Local-first ensures privacy

### Negative
- Cannot add cloud features easily later
- Limited by local compute power
- Adapter strategy requires upfront model work

---

### 16. AST & Code Intelligence Layer

**Decision:** Use Tree-sitter via go-tree-sitter (CGO bindings) for AST parsing.

#### Rationale
- Tree-sitter is industry standard (GitHub, Neovim, Zed)
- Multi-language support with consistent API
- Incremental parsing for performance
- Error recovery for broken code

#### CGO Decision
**Approved with acknowledged risks:**

| Risk | Mitigation |
|------|------------|
| Cross-platform builds | Pre-built binaries via GitHub Actions matrix |
| Contributor friction | Docker-based dev environment option |
| CGO leaking | Isolate all CGO in `internal/ast/` package |

#### Build Requirements
```bash
# Windows: requires mingw-w64
# macOS: requires Xcode command line tools
# Linux: requires gcc
CGO_ENABLED=1 go build ./...
```

#### Supported Languages (Phase 1)
| Language | Grammar Package | Priority |
|----------|-----------------|----------|
| Go | tree-sitter-go | High |
| TypeScript | tree-sitter-typescript | High |
| JavaScript | tree-sitter-javascript | High |
| Python | tree-sitter-python | Medium |

#### Performance Requirements
| Operation | Target | Hard Cap |
|-----------|--------|----------|
| Parse single file | <50ms | 100ms |
| Parse 100 files | <2s | 5s |
| Symbol lookup | <1ms | 5ms |

#### Non-Negotiables
- **Deterministic output** — same input = same AST
- **No LLM calls** — pure structural analysis
- **Graceful degradation** — broken code returns partial results

---

### 17. Repo Native Memory

**Decision:** Schema-first memory with confidence scoring and human-editable overrides.

#### Storage
| Type | Format | Purpose |
|------|--------|---------|
| Runtime | SQLite | Fast queries |
| Human-editable | `.localmind/memory.json` | User overrides |

#### Confidence Levels
| Level | Range | Meaning |
|-------|-------|---------|
| Detected | 0.9-1.0 | Explicit evidence found |
| Inferred | 0.6-0.9 | Pattern matching |
| Unknown | 0.0-0.3 | Insufficient data |

#### Key Principles
- **Schema-enforced** — All memory has types
- **Confidence scoring** — Never blind assumptions
- **User overrides** — Always respected
- **Incremental updates** — No full rescans

---

## References

- [PDR.md](file:///a:/Local%20Mind/Docs/PDR.md) - Product Design Requirements
- [Tech Req.md](file:///a:/Local%20Mind/Docs/Tech%20Req.md) - Technical Requirements
- [Dev Plan.md](file:///a:/Local%20Mind/Docs/Dev%20Plan.md) - Development Plan
- [IPC Protocol](../protocol/ipc-protocol.md) - Message protocol specification


