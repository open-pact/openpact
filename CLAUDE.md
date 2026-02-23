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

# Admin UI (Vue 3 + Vite)
cd admin-ui && npm install && npm run build   # Build (output: admin-ui/dist/)
cd admin-ui && npm run dev                     # Dev server with API proxy to :8080

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
- **context/** — Loads SOUL.md, USER.md, MEMORY.md from workspace for AI context injection
- **health/** — Health checks and Prometheus metrics
- **ratelimit/** — Token bucket rate limiter
- **logging/** — Structured logging with configurable levels

**Two entry points in `cmd/`:**
- `cmd/openpact/` — Main orchestrator binary
- `cmd/admin/` — Standalone admin server

**Admin UI (`admin-ui/`):** Vue 3 + Naive UI component library. Built with Vite. The compiled output is embedded into the Go binary. During development, the Vite dev server proxies `/api` requests to `localhost:8080`.

## OpenCode Engine Integration

The engine (`internal/engine/opencode.go`) communicates with [OpenCode](https://opencode.ai) by running `opencode serve` as a persistent child process and calling its REST API over HTTP.

**Documentation:** https://opencode.ai/docs/server/
**OpenAPI spec (at runtime):** `http://<host>:<port>/doc`

**How it works:**
1. On startup, the engine spawns `opencode serve --port <port> --hostname 127.0.0.1` as a child process
2. It polls `GET /global/health` until the server is ready
3. All session and message operations go through the REST API:
   - `POST /session` — Create a new session
   - `GET /session` — List all sessions
   - `GET /session/:id` — Get session details
   - `DELETE /session/:id` — Delete a session
   - `POST /session/:id/message` — Send a message (with `parts` array, optional `system` prompt and `model` override)
   - `GET /session/:id/message` — Get message history
   - `POST /session/:id/abort` — Abort a running session
   - `GET /event` — SSE event stream for real-time updates
4. On shutdown, the engine sends `SIGINT` to the child process for graceful exit

**Auth:** If `engine.password` is set in config, requests use HTTP basic auth (`username: "opencode"`, password from config). The password is also passed to the child process via `OPENCODE_SERVER_PASSWORD` env var.

**Session management:** OpenCode manages all session storage internally (SQLite). Chat providers use per-channel session tracking (persisted to `<DataDir>/channel_sessions.json`), where each `(provider, channelID)` pair maps to its own session. If a channel has no session when a message arrives, one is created automatically. The Admin UI can interact with any session directly.

**Config (`openpact.yaml`):**
```yaml
engine:
  type: opencode
  port: 4098          # Port for opencode serve (default: pick random free port)
  password: ""        # Optional OPENCODE_SERVER_PASSWORD
```

## Key Design Decisions

- **No database** — All persistence is file-based JSON (users, script approvals)
- **Security boundary at MCP** — AI never gets direct filesystem/network access; everything goes through registered MCP tools
- **Secret redaction** — Starlark scripts can use secrets, but all output is scanned and secret values are replaced with `[REDACTED:NAME]` before the AI sees results
- **Two-user Docker model** — `openpact-system` (privileged) and `openpact-ai` (restricted) for principle of least privilege
- **Go standard library for HTTP** — Uses `net/http` directly, no web framework

## Configuration

The app reads `openpact.yaml` (or `config.yaml` in workspace). Key env vars: `DISCORD_TOKEN`, `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`. Starlark secrets are configured under `starlark.secrets` and can reference env vars with `${VAR}` syntax.

## Admin UI Theme Reference

The admin UI is based on the [YummyAdmin](https://github.com/nicevoice/yummy-admin) theme (Naive UI + Vue 3). The full AI reference document for the theme is at `ai/theme/theme-instructions.md`, and the original theme source is at `ai/theme/YummyAdmin/src/`. Consult these when implementing or modifying admin UI components, layouts, or styling.

Key conventions:
- Page width is controlled at the layout level (`AppLayout.vue`), not per-page — individual pages should NOT set their own `max-width`
- Uses UnoCSS (with `presetUno`, `presetAttributify`, `presetWind`) for utility classes
- Dark mode via `dark:` UnoCSS variants and CSS variables in `admin-ui/src/styles/main.scss`
- Component library: Naive UI (`n-` prefixed components)

## Go Module

Module path: `github.com/open-pact/openpact`, Go 1.22.
