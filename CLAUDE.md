# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OpenPact is a secure, minimal framework for running your own AI assistant. It's a Go monorepo with a Vue 3 admin UI and Docusaurus docs site. The AI assistant connects to Discord/Slack/Telegram, runs LLM agents in-process via [stackllm](https://github.com/stack-bound/stackllm), and exposes capabilities to those agents through MCP (Model Context Protocol) tools with a security-first design.

## Build & Development Commands

```bash
# Go backend
make build                  # Build binary to ./openpact
make test                   # Run all Go tests (171 tests)
make coverage               # Generate HTML coverage report
make fmt                    # Format Go code
make lint                   # Lint (requires golangci-lint)
make run                    # Build and run locally
make docker                 # Build Docker image

# Run a single Go test
go test -v -run TestName ./internal/packagename/

# Node.js — this project uses nvm. Always run `nvm use` before Node operations.
nvm use

# Admin UI (Vue 3 + Vite)
cd admin-ui && npm install && npm run build   # Build (output: admin-ui/dist/)
cd admin-ui && npm run dev                     # Dev server with API proxy to :8888

# Documentation (Docusaurus)
cd docs && yarn install && yarn start          # Dev server
cd docs && yarn build                          # Production build
```

**Important:** The admin UI must be built (`admin-ui/dist/` must exist) before running Go tests, because `admin-ui/embed.go` uses `//go:embed all:dist`. Build the UI first with `cd admin-ui && npm ci && npm run build`.

## Architecture

**Data flow:**
```
Discord/Slack/Telegram → Orchestrator → engine.Stack (stackllm agent, in-process) → reply
Admin UI chat          → /api/engine/chat (stackllm web.ManagedHandler, SSE)       → reply
```

**Key packages in `internal/`:**
- **engine/** — Composes stackllm primitives into a single `Stack` (`profile.Manager`, `session.SQLiteStore`, `tools.Registry`, `web.ManagedHandler`). No interface, no HTTP hop — the agent runs in-process.
- **orchestrator/** — Central coordinator. Builds the Stack, routes chat-provider messages through an `agent.Agent`, injects SOUL/USER/MEMORY as a system-role message, persists per-channel session IDs.
- **mcp/** — MCP tool registry. The server object is used as a catalogue only; `engine.RegisterMCPTools` copies every registered tool into a native stackllm `tools.Registry` via a thin adapter. An optional stdio-mode server remains for future external MCP clients.
- **admin/** — Web server for the admin UI. JWT auth, setup wizard, script/secret/schedule stores, and the mount of `stack.Handler` under `/api/engine/` so the browser can drive provider login, model selection, and SSE chat directly.
- **starlark/** — Sandboxed Starlark script execution with built-in modules (http, json, time, secrets). Secrets are injected at runtime and redacted from output before returning to the AI.
- **config/** — YAML + env var loader. The `engine:` block now has just a single optional `db_path` override; all LLM credentials and the default model live in stackllm's own stores under `secure/data/`.
- **context/** — Loads SOUL.md, USER.md, MEMORY.md from `ai-data/` for AI context injection.
- **chat/** — Abstract chat-provider interface (`ChatProvider`) shared by the Discord/Slack/Telegram adapters.
- **scheduler/** — Cron-based Starlark and agent job runner. `RunAgent` on the orchestrator is the agent-job entrypoint.

**Three entry points in `cmd/`:**
- `cmd/openpact/` — Main binary (`openpact start` runs orchestrator + admin UI + engine stack).
- `cmd/admin/` — Standalone admin server (dev convenience — spins up a minimal stack with workspace + memory tools only).
- `cmd/mcp-server/` — Standalone stdio MCP server for external clients that want direct access to the tool registry.

**Admin UI (`admin-ui/`):** Vue 3 + Naive UI. Built with Vite, embedded in the Go binary via `//go:embed`. Key views:
- `/engine` (EngineView) — provider login + default-model picker. Drives `/api/engine/*`.
- `/sessions` (SessionsView) — admin chat over `POST /api/engine/chat` SSE, laid out per the YummyAdmin Chat/* components.
- `/settings/advanced` (AdvancedSettingsView) — logging, rate limiter, health address, Starlark limits. Backed by `/api/config/advanced`.
- `/integrations` (IntegrationsView) — calendar feeds + Obsidian vault. Backed by `/api/config/integrations`.
- `/secrets` — Starlark secret store + read-only view of authenticated LLM providers.
- Setup wizard (`/setup`) — 3 steps: account → profile → provider login + default model.

## Engine (stackllm, in-process)

`internal/engine/stackllm.go` assembles:
1. `auth.FileStore` at `<workspace>/secure/data/stackllm_auth.json` (0600) — provider tokens.
2. `config.Store` at `<workspace>/secure/data/stackllm_config.json` — default provider/model, Ollama base URL, recent models.
3. `session.SQLiteStore` at `<workspace>/secure/data/stackllm.db` (override with `engine.db_path`) — message history, artifacts, last-usage cache.
4. `tools.Registry` built from the MCP `Server.ListTools()` output via `engine.RegisterMCPTools` (each MCP handler runs through `mcpToolAdapter.Call`).
5. `web.NewManagedHandler` — the HTTP surface the admin UI mounts.

Providers available via the managed handler: OpenAI (API key or Codex-flow "Sign in with ChatGPT"), GitHub Copilot (device flow), Google Gemini (API key), Ollama (base URL). Anthropic is intentionally not surfaced — third-party harness use is no longer permitted.

The orchestrator calls `stack.Manager.Default` + `LoadProviderForModel` + `agent.New(provider, WithTools(stack.Tools))` on each chat message. System prompt (SOUL/USER/MEMORY) is prepended to the message history as a `RoleSystem` message the first time a session runs. Per-session mutexes serialize concurrent messages in the same channel.

Session management: each `(provider, channelID)` maps to one stackllm session UUID (persisted in `<DataDir>/channel_sessions.json`). stackllm's SQLite store owns the message history; the admin UI can interact with any session via `GET /api/engine/sessions/{id}`.

## Workspace Directory Structure

The workspace uses a security-first split between system and AI data:

```
/workspace/
├── secure/                     # SYSTEM-ONLY — AI has ZERO access
│   ├── config.yaml             # Bootstrap config (engine.db_path, admin bind, Starlark limits)
│   └── data/                   # All system state
│       ├── jwt_secret
│       ├── users.json
│       ├── approvals.json
│       ├── secrets.json        # Starlark secrets (separate from LLM credentials)
│       ├── chat_providers.json
│       ├── channel_sessions.json
│       ├── channel_modes.json
│       ├── setup_state.json
│       ├── advanced_settings.json
│       ├── integrations.json
│       ├── stackllm_auth.json  # LLM provider tokens (OpenAI/Gemini/Copilot/Ollama)
│       ├── stackllm_config.json # Default model + recent models
│       └── stackllm.db         # Conversation history (pure-Go SQLite)
├── ai-data/                    # AI-ACCESSIBLE — MCP tools scope here
│   ├── SOUL.md
│   ├── USER.md
│   ├── MEMORY.md
│   ├── memory/                 # Daily memory files
│   ├── scripts/                # Starlark scripts
│   └── skills/                 # Skills directory
```

Key path methods on `WorkspaceConfig`:
- `SecureDir()` → `<workspace>/secure`
- `AIDataDir()` → `<workspace>/ai-data`
- `DataDir()` → `<workspace>/secure/data`
- `ScriptsDir()` → `<workspace>/ai-data/scripts`

## Critical: Admin Route Registration — DUAL HANDLER RULE

**There are TWO handler methods in `internal/admin/` that BOTH need API routes registered:**

1. **`Handler()`** in `router.go` — API-only mode (standalone admin server)
2. **`HandlerWithUI()`** in `embed.go` — API + embedded SPA mode (production/orchestrator)

**When adding new API routes, you MUST register them in BOTH methods.** If a route is only added to `Handler()` but not `HandlerWithUI()`, the route will work in API-only mode but silently fail in production — the SPA catch-all (`mux.Handle("/", spaHandler)`) will serve `index.html` instead of the API response, causing "Failed to load" errors on the frontend.

This has been a repeated source of bugs. Always check both methods when adding or modifying routes.

## Key Design Decisions

- **Single binary** — stackllm runs in-process (pure-Go SQLite via `modernc.org/sqlite`). No Node, no opencode, no supervisor loop. CGO-free static build.
- **Admin UI is the config surface** — provider credentials, default model, logging, rate limits, Starlark limits, calendar feeds, and vault config are all set through the web UI. `config.yaml` carries only bootstrap defaults.
- **Security boundary at the tool registry** — the agent can only call tools that were registered by `mcp.RegisterAllTools`. Workspace tools are scoped to `ai-data/`; secrets are injected into Starlark at runtime and redacted from output.
- **Physical security split** — `secure/` (0700) holds system data; `ai-data/` (0755) is AI-accessible. Derived from workspace path; no env var.
- **Single-user containers** — the old two-user Docker model is gone along with OpenCode. The binary runs as one unprivileged user.
- **Go standard library for HTTP** — `net/http` directly, no web framework.

## Configuration

`secure/config.yaml` is the bootstrap file. LLM provider credentials and the default model live in stackllm's own stores (`stackllm_auth.json` + `stackllm_config.json` under `secure/data/`) and are managed through the admin UI at `/engine`. Advanced knobs (logging, rate limit, Starlark limits) are in `advanced_settings.json` via `/settings/advanced`; integrations (calendars, vault) in `integrations.json` via `/integrations`.

Key env vars (bootstrap only):
- `WORKSPACE_PATH` — workspace root (default `/workspace`).
- `CONFIG_PATH` — path to `config.yaml` (default `<workspace>/secure/config.yaml`).
- `ADMIN_BIND` — admin UI bind address (default from config.yaml).
- `DISCORD_TOKEN`, `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN`, `TELEGRAM_BOT_TOKEN` — chat-provider tokens (also settable via admin UI).
- `GITHUB_TOKEN` — GitHub MCP tool auth.

## Admin UI Theme Reference — MANDATORY RULES

The admin UI is based on the [YummyAdmin](https://github.com/nicevoice/yummy-admin) theme (Naive UI + Vue 3). The original theme source is at `ai/theme/YummyAdmin/src/`. The full AI reference document is at `ai/theme/theme-instructions.md`.

### STRICT RULES — DO NOT VIOLATE THESE

**1. NEVER invent CSS values.** Every CSS property value (heights, margins, padding, calc expressions, border-radius, colors) in layout and styling MUST come directly from the theme source files. If a theme file says `height: calc(100vh - 30px)`, use EXACTLY `calc(100vh - 30px)` — do not "adjust" it, round it, or substitute your own calculation. You are not smarter than the theme author. The theme is battle-tested; your custom values are not.

**2. ALWAYS read the theme source FIRST.** Before writing or modifying ANY admin UI component, you MUST:
   - Find the closest matching component in `ai/theme/YummyAdmin/src/`
   - Read ALL related theme files (component, layout, styles) completely
   - Copy the theme's HTML structure, CSS classes, SCSS, and `<style>` blocks verbatim
   - Only then adapt the `<script>` logic for our data model

**3. NEVER add CSS properties the theme doesn't have.** If the theme's `.main-content` doesn't have `display: flex; flex-direction: column`, do NOT add it. If the theme's `.message-input` only has `background: transparent; border: none; &:focus { outline: none }`, do NOT add `color`, `font-size`, `width`, or `:disabled` styles. Copy what exists. Nothing more.

**4. NEVER change spacing or layout values.** If the theme uses `my-2`, use `my-2` — not `my-1`. If the theme uses `p-3`, use `p-3` — not `p-4 md:p-6`. These values are deliberate design choices that affect the entire layout chain.

**5. Use the theme's exact height calc values.** Key reference values from the theme (do NOT change these):
   - `main.scss` → `.main-content { height: calc(100vh - 1.3rem); }`
   - `ChatApp.vue` → `.chat-layout { height: calc(100vh - 30px); }`
   - `ChatApp.vue` → `.chat-sidebar { height: calc(100vh - 150px); }`
   - `ChatMessages.vue` → `.messages-box { height: calc(100% - 51px); }`
   - `default.vue` → `.main-content` div uses class `my-2`

**6. When something looks broken, the fix is to match the theme more closely** — not to invent a new workaround. If heights are wrong, compare every single CSS value against the theme source. The theme already works; divergence from it is always the bug.

### General conventions
- Page width is controlled at the layout level (`AppLayout.vue`), not per-page — individual pages should NOT set their own `max-width`
- Uses UnoCSS (with `presetUno`, `presetAttributify`, `presetWind`) for utility classes
- Dark mode via `dark:` UnoCSS variants and CSS variables in `admin-ui/src/styles/main.scss`
- Component library: Naive UI (`n-` prefixed components)
- The Chat page (`SessionsView.vue`) maps to theme's `components/Apps/Chat/` — ChatApp.vue, ChatMessages.vue, ChatList.vue, MessageItem.vue. These are the source of truth for all chat layout and styling.

## Go Module

Module path: `github.com/open-pact/openpact`, Go 1.25. Key dependencies:
- `github.com/stack-bound/stackllm@v0.4.0` — in-process LLM engine, tool registry, session store, web.ManagedHandler.
- `modernc.org/sqlite` — pure-Go SQLite driver (transitive via stackllm). Keeps the build CGO-free.
- `github.com/gorilla/websocket`, `github.com/robfig/cron/v3`, `go.starlark.net`, chat provider SDKs.
