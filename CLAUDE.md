# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OpenPact is a secure, minimal framework for running your own AI assistant. It's a Go monorepo with a Vue 3 admin UI and Docusaurus docs site. The AI assistant connects to Discord, uses an engine abstraction (OpenCode), and exposes capabilities through MCP (Model Context Protocol) tools with a security-first design.

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

**Important:** The admin UI must be built (`admin-ui/dist/` must exist) before running Go tests, because `internal/admin/embed.go` uses `//go:embed all:admin-ui/dist`. Tests will fail without it. The embed directive expects the dist directory at `internal/admin/admin-ui/dist` (there's a symlink or copy step needed).

## Architecture

**Data flow:**
```
Discord msg  → Orchestrator → POST /session/:id/message → opencode serve → AI response → reply
Discord /cmd → Orchestrator → session management (create/list/switch)
Admin UI     → Admin API    → Orchestrator (SessionAPI) → opencode serve
```

**Key packages in `internal/`:**
- **orchestrator/** — Central coordinator. Manages component lifecycle, routes Discord messages to the AI engine, injects context (SOUL/USER/MEMORY docs)
- **mcp/** — MCP server implementing JSON-RPC 2.0 over stdin/stdout pipes. This is the security boundary — the AI can only use explicitly registered tools (~20 tools across workspace, memory, Discord, calendar, vault, web, GitHub, scripts)
- **engine/** — Abstraction layer for AI coding agents. Communicates with OpenCode via its HTTP server API (see below)
- **admin/** — Web server for the admin UI. JWT auth, session management, script approval workflow. File-based JSON storage (no database). Embeds the Vue SPA via `//go:embed`
- **discord/** — Discord bot with user/channel allowlists and bidirectional messaging
- **starlark/** — Sandboxed Starlark script execution with built-in modules (http, json, time, secrets). Secrets are injected at runtime and redacted from output before returning to the AI
- **config/** — YAML + env var configuration loading
- **context/** — Loads SOUL.md, USER.md, MEMORY.md from `ai-data/` for AI context injection
- **health/** — Health checks and Prometheus metrics
- **ratelimit/** — Token bucket rate limiter
- **logging/** — Structured logging with configurable levels

**Two entry points in `cmd/`:**
- `cmd/openpact/` — Main orchestrator binary
- `cmd/admin/` — Standalone admin server

**Admin UI (`admin-ui/`):** Vue 3 + Naive UI component library. Built with Vite. The compiled output is embedded into the Go binary. During development, the Vite dev server proxies `/api` requests to `localhost:8888`.

## OpenCode Engine Integration

The engine (`internal/engine/opencode.go`) is a pure HTTP client that connects to an externally-managed [OpenCode](https://opencode.ai) `opencode serve` instance. In Docker, the entrypoint launches OpenCode as `openpact-ai` with a monitored restart loop; the Go engine just connects and talks HTTP.

**Documentation:** https://opencode.ai/docs/server/
**OpenAPI spec (at runtime):** `http://<host>:<port>/doc`

**How it works:**
1. The Docker entrypoint generates OpenCode config via `openpact opencode-config` (produces `OPENCODE_CONFIG_CONTENT` JSON)
2. The entrypoint launches `opencode serve --port 4098 --hostname 127.0.0.1` as `openpact-ai` in a restart loop
3. On startup, the engine's `Start()` sets `baseURL` and polls `GET /global/health` until the server is ready
4. All session and message operations go through the REST API:
   - `POST /session` — Create a new session
   - `GET /session` — List all sessions
   - `GET /session/:id` — Get session details
   - `DELETE /session/:id` — Delete a session
   - `POST /session/:id/message` — Send a message (with `parts` array, optional `system` prompt and `model` override)
   - `GET /session/:id/message` — Get message history
   - `POST /session/:id/abort` — Abort a running session
   - `GET /event` — SSE event stream for real-time updates
5. On shutdown, `Stop()` is a no-op — the entrypoint manages the OpenCode process lifecycle

**Auth:** If `engine.password` is set in config, requests use HTTP basic auth (`username: "opencode"`, password from config). The entrypoint also passes the password to OpenCode via `OPENCODE_SERVER_PASSWORD` env var.

**Session management:** OpenCode manages all session storage internally (SQLite). Chat providers use per-channel session tracking (persisted to `<DataDir>/channel_sessions.json`), where each `(provider, channelID)` pair maps to its own session. If a channel has no session when a message arrives, one is created automatically. The Admin UI can interact with any session directly.

**Config (`secure/config.yaml`):**
```yaml
engine:
  type: opencode
  port: 4098          # Port for opencode serve (must match entrypoint)
  password: ""        # Optional OPENCODE_SERVER_PASSWORD
```

## Workspace Directory Structure

The workspace uses a security-first split between system and AI data:

```
/workspace/
├── secure/                     # SYSTEM-ONLY — AI has ZERO access
│   ├── config.yaml             # Main config (may contain passwords)
│   └── data/                   # All admin/system data
│       ├── jwt_secret
│       ├── users.json
│       ├── approvals.json
│       ├── secrets.json
│       ├── chat_providers.json
│       ├── channel_sessions.json
│       ├── setup_state.json
│       └── opencode/           # OpenCode engine state (SQLite, logs, etc.)
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

## Key Design Decisions

- **No database** — All persistence is file-based JSON (users, script approvals)
- **Security boundary at MCP** — AI never gets direct filesystem/network access; everything goes through registered MCP tools. MCP workspace tools are scoped to `ai-data/` only.
- **Physical security split** — `secure/` for system data (config, secrets, JWT), `ai-data/` for AI-accessible files. No env var needed — derived from workspace path.
- **Secret redaction** — Starlark scripts can use secrets, but all output is scanned and secret values are replaced with `[REDACTED:NAME]` before the AI sees results
- **Two-user Docker model** — `openpact-system` (privileged) and `openpact-ai` (restricted) for principle of least privilege
- **Go standard library for HTTP** — Uses `net/http` directly, no web framework

## Configuration

The app reads `secure/config.yaml` from the workspace. All paths are derived from `WORKSPACE_PATH` — no separate data dir env var. Key env vars: `DISCORD_TOKEN`, `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`.

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

Module path: `github.com/open-pact/openpact`, Go 1.22.
