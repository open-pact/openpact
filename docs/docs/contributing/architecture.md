---
title: Architecture
sidebar_position: 2
---

# Architecture Overview

OpenPact is a Go monorepo with a Vue 3 admin UI and a Docusaurus docs site. The runtime is a single static binary: the LLM engine, the tool registry, the orchestrator, the admin web server, and the SQLite session store all live in the same process.

## Component Diagram

```
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│   Discord    │   │   Telegram   │   │    Slack     │
│   Adapter    │   │   Adapter    │   │   Adapter    │
└──────┬───────┘   └──────┬───────┘   └──────┬───────┘
       │                  │                  │
       ▼                  ▼                  ▼
┌──────────────────────────────────────────────────────────────┐
│                    chat.Provider interface                    │
│                       (internal/chat)                         │
└────────────────────────────────┬─────────────────────────────┘
                                 ▼
┌──────────────────────────────────────────────────────────────┐
│                       Orchestrator                            │
│                  (internal/orchestrator)                      │
│  - Per-channel session map (op_channel_sessions)              │
│  - Per-session mutex                                          │
│  - SOUL/USER/MEMORY system-prompt injection                   │
│  - Builds agent.Agent per turn                                │
└────────────┬─────────────────────────────┬───────────────────┘
             │ in-process                  │ in-process
             ▼                             ▼
┌────────────────────────┐    ┌──────────────────────────────┐
│   stackllm engine      │    │       MCP tool catalogue     │
│   (internal/engine)    │    │       (internal/mcp)         │
│                        │    │                              │
│  Stack:                │    │  - mcp.RegisterAllTools()    │
│   - profile.Manager    │◄──►│  - HTTP /api/engine/* mount  │
│   - session.SQLiteStore│    │  - Stdio MCP server (opt.)   │
│   - tools.Registry     │    └──────────────────────────────┘
│   - web.ManagedHandler │              ▲
└────────────────────────┘              │ Tools registered once at boot
                                        │ via engine.RegisterMCPTools
                              ┌─────────┴───────────────┐
                              ▼                         ▼
            ┌─────────────────────────┐   ┌─────────────────────────┐
            │     Workspace tools     │   │    Integration tools    │
            │                         │   │                         │
            │  - workspace_read       │   │  - calendar_read        │
            │  - workspace_write      │   │  - vault_*              │
            │  - workspace_list       │   │  - github_*             │
            │  - memory_read/write    │   │  - web_fetch            │
            │  - script_*             │   │  - discord_send         │
            └─────────────────────────┘   └─────────────────────────┘
```

## Data Flow

### Chat-platform message

```
Discord/Slack/Telegram → chat.Provider → Orchestrator → engine.Stack
                                                        ├─ profile.Manager.Default
                                                        ├─ LoadProviderForModel
                                                        └─ agent.New(...).Run(ctx, msgs)
                                                            ↓
                                                       <-chan agent.Event
                                                            ↓
                                              accumulateChatResponse
                                                            ↓
                                                Reply via chat.Provider
```

### Admin UI chat

```
Browser → POST /api/engine/chat (SSE)
                ↓
        web.ManagedHandler (mounted under /api/engine/)
                ↓
        Same agent.Agent loop, in-process
                ↓
        SSE: event: block_delta / done / error
```

There is no HTTP hop between OpenPact and the LLM engine. The agent is a Go function call inside the same binary.

## Core Packages

### `cmd/openpact`

The user-facing binary. `openpact start` boots the orchestrator, the engine stack, and the admin UI. There is no separate admin binary in production — `cmd/admin` exists only for dev workflows that want the UI without chat providers.

### `cmd/mcp-server`

A standalone stdio MCP server. Optional. Useful if you want an external MCP client (e.g. another LLM, Claude Desktop) to call OpenPact tools directly. Not used at runtime by the main binary.

### `internal/orchestrator`

Central coordinator. Owns the `*engine.Stack`, the per-channel session map (`op_channel_sessions`), and the per-session mutex map (`sessionLocks`). On each incoming chat message:

1. Acquire the per-session mutex.
2. Load (or create) the stackllm session by the `(provider, channel)` key.
3. Prepend `RoleSystem` (SOUL + USER + MEMORY) on the first turn of a fresh session.
4. Build `agent.Agent` with `agent.WithTools(stack.Tools)`.
5. Consume `agent.Run(ctx, messages)` events; accumulate text via `accumulateChatResponse`.
6. Emit reply on the originating chat provider; mirror it into the typing/edit semantics of that platform.

`accumulateChatResponse` (with regression tests in `internal/orchestrator/accumulate_test.go`) handles the awkward case of multiple text blocks around a tool call (`"Looking that up..."` → tool_use → `"Here's what I found."`) — every block is preserved.

### `internal/engine`

Composes stackllm primitives into a single `Stack`:

```go
type Stack struct {
    Manager   *profile.Manager       // Provider auth, default model, recent models
    Sessions  session.SessionStore   // SQLite-backed; pure-Go via modernc.org/sqlite
    Tools     *tools.Registry        // Native Go tools registered at boot
    Handler   http.Handler           // web.ManagedHandler — mounted under /api/engine/
    // SystemPrompt accessor pair (RWMutex-gated) used by the orchestrator
}
```

There is no `Engine` interface. The orchestrator and admin server use `*Stack` directly: `Manager` to choose a provider, `Sessions` for persistence, `agent.New(provider, agent.WithTools(stack.Tools))` per turn, and `Handler` for the admin UI's HTTP surface.

`engine.RegisterMCPTools` walks `mcp.Server.ListTools()` and registers each tool in `tools.Registry` via `mcpToolAdapter` — the adapter translates stackllm's JSON-args calling convention to MCP's `(ctx, argsMap)` shape. Schemas pass through verbatim (MCP `InputSchema` is the same JSON-Schema shape stackllm consumes).

### `internal/mcp`

MCP tool registry and (optional) stdio server. Tool implementations:

- **Workspace** — `workspace_read`, `workspace_write`, `workspace_list`. Path-validated against `<workspace>/ai-data/`.
- **Memory** — `memory_read`, `memory_write`. Hard-coded paths only.
- **Communication** — `discord_send`.
- **Integrations** — `calendar_read`, `vault_*`, `github_*`, `web_fetch`.
- **Scripts** — `script_run`, `script_exec`, `script_list`, `script_reload`.

There is no MCP HTTP/JSON-RPC server. The agent is in-process; no external transport is needed.

### `internal/admin`

Admin UI backend. JWT auth (HS256), short-lived access tokens, refresh-cookie rotation. The setup wizard guards onboarding (`RequireSetupMiddleware`). `Handler()` and `HandlerWithUI()` both call a shared `registerAPIRoutes(mux)` so the [dual-handler rule](#dual-handler-rule) is enforced at the function-composition level.

The mounted `/api/engine/*` subtree is a `http.StripPrefix`-wrapped `web.ManagedHandler` that handles provider login, model selection, and SSE chat. It sits behind `withEngineAuth` — once setup is complete, that's just `withAuth`; during step 3 of the wizard a narrow whitelist (`isSetupEngineEndpoint`) lets the user sign in to a provider before they have a JWT.

### `internal/storage/*`

Package-per-domain persistence over the shared SQLite DB:

- `users` — admin accounts (bcrypt).
- `approvals` — Starlark script approval metadata.
- `secrets` — AES-256-GCM ciphertext keyed on `data_encryption_key`.
- `chatproviders` — Discord/Slack/Telegram enablement, allow lists, tokens.
- `schedules` — cron schedules (script + agent).
- `kv` — scoped scalar settings (`advanced_settings.*`, `integrations.*`, `setup_state.*`).
- `calendars` — ordered list of iCal feeds.
- `channels` — `(provider, channel_id) → session UUID` and detail-mode mappings.

OpenPact's tables are prefixed `op_*`; stackllm's are `stackllm_*`. They share the same DB file.

### `internal/starlark`

Sandboxed Starlark interpreter. No filesystem access. HTTP-only via the built-in `http` module. Built-in modules: `http`, `json`, `time`, `secrets`. Secrets are injected at runtime and redacted from output before returning to the agent.

### `internal/config`

Bootstrap-only YAML loader. The file carries `workspace.path`, `engine.db_path`, `admin.bind`, `admin.allowlist`, `starlark.max_execution_ms`, `starlark.max_memory_mb`, and Discord defaults — nothing else. Every runtime-mutable setting lives in `op_*` tables.

### `internal/context`

Loads `SOUL.md`, `USER.md`, `MEMORY.md` from `<workspace>/ai-data/` for system-prompt injection.

### `internal/chat`

Generic `chat.Provider` interface plus `MessageHandler` and `CommandHandler` callbacks. Implemented by the Discord, Slack, and Telegram adapters.

### `internal/providers/{discord,slack,telegram}`

Chat-platform adapters. Discord uses websockets; Telegram uses long polling; Slack uses Socket Mode. Each implements `chat.Provider`.

### `internal/health`

Health endpoints: `/health` (detailed), `/healthz` (Kubernetes-style), `/ready` (readiness), `/metrics` (Prometheus). Bind address configurable via `/settings/advanced`.

### `internal/logging`

Structured logging via `slog`. Text or JSON format; levels: debug/info/warn/error. Configurable via `/settings/advanced`.

### `internal/ratelimit`

Token-bucket rate limiter. Configurable rate/burst via `/settings/advanced`.

### `internal/scheduler`

Cron-based job runner for Starlark scripts and agent jobs. Calls `Orchestrator.RunAgent(ctx, prompt)` for agent jobs.

## Security Architecture

### Tool Registry as the Boundary

The agent can only call tools that were registered in `tools.Registry` at boot. There is no shell, no `eval`, no arbitrary file IO. Adding a capability requires editing Go code and rebuilding.

Each tool enforces its own scoping:

- `workspace_*` validates paths against `<workspace>/ai-data/`.
- `vault_*` is constrained to the configured vault path.
- `web_fetch` is HTTP/HTTPS-only, response size capped.
- `script_run` requires admin approval (or `admin.allowlist`).
- Chat-send tools respect provider allow lists.

### Filesystem Permissions

`secure/` is mode `0700`; `secure/data/*` files are `0600`. Backup if a tool implementation has a path-traversal bug — the OS refuses the read.

### Secret Handling

- Provider tokens → `secure/data/stackllm_auth.json` (mode `0600`), owned by stackllm.
- Starlark secrets → `op_secrets`, AES-256-GCM at rest, decrypted in-memory only.
- Output sanitisation: Starlark output is scanned for any literal secret value and replaced with `[REDACTED:NAME]` before reaching the agent.

### Dual Handler Rule {#dual-handler-rule}

`/api/*` routes are registered in a single `registerAPIRoutes(mux)` method that both `Handler()` (admin-only mode) and `HandlerWithUI()` (production with embedded SPA) call. Adding a route to one but not the other is no longer syntactically possible.

## Extension Points

### Adding a new tool

1. Implement the MCP tool function in `internal/mcp/<tool>.go`.
2. Register it in `internal/mcp/register.go` (`RegisterAllTools`).
3. Done — `engine.RegisterMCPTools` will pick it up at boot.

### Adding a new chat provider

1. Implement `chat.Provider` in `internal/providers/<name>/`.
2. Add storage hooks in `internal/storage/chatproviders/`.
3. Wire it into `internal/orchestrator/orchestrator.go`'s provider start/stop.
4. Add a UI page under `admin-ui/src/views/Providers/`.

### Adding a Starlark built-in

1. Add the function in `internal/starlark/builtin_*.go`.
2. Register it in the appropriate module.
3. Document it in `docs/starlark/built-in-functions.md`.

### Adding admin UI views

1. Create the Vue SFC under `admin-ui/src/views/`.
2. Register the route in `admin-ui/src/main.js`.
3. Add a sidebar entry in `admin-ui/src/components/layout/SidebarMenu.vue`.
4. Match the YummyAdmin theme verbatim — never invent CSS values. The theme source is at `ai/theme/YummyAdmin/src/`.
