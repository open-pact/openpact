---
title: Development Setup
sidebar_position: 1
---

# Development Setup

This guide walks you through setting up OpenPact for local development.

## Prerequisites

- **Go 1.25+** — [Download Go](https://golang.org/dl/) (the project's `go.mod` pins 1.25; the toolchain auto-upgrades to 1.25.x).
- **Node.js** — managed via **nvm**. The repo has a `.nvmrc`; run `nvm use` from the project root.
- **Git**.
- **Make** (optional, for the Makefile shortcuts).
- **Docker** (optional, only if you want to test the production image).

## Clone the Repository

```bash
git clone https://github.com/open-pact/openpact.git
cd openpact-app
```

## Project Structure

```
openpact-app/
├── cmd/
│   ├── openpact/       # Main binary — orchestrator + admin UI + engine stack
│   ├── admin/          # Standalone admin server (dev convenience)
│   └── mcp-server/     # Standalone stdio MCP server (optional, for external clients)
├── internal/
│   ├── admin/          # Admin web server (JWT auth, setup wizard, dual-handler routes)
│   ├── chat/           # chat.Provider interface
│   ├── config/         # Bootstrap-only YAML loader
│   ├── context/        # SOUL/USER/MEMORY loader
│   ├── engine/         # stackllm Stack assembly + tool-registry adapter
│   ├── health/         # Health checks + Prometheus metrics
│   ├── logging/        # slog wrappers
│   ├── mcp/            # MCP tool catalogue
│   ├── orchestrator/   # Central coordinator
│   ├── providers/      # Discord, Slack, Telegram adapters
│   ├── ratelimit/      # Token-bucket rate limiter
│   ├── scheduler/      # Cron-based Starlark + agent jobs
│   ├── starlark/       # Sandboxed scripting
│   └── storage/        # Package-per-domain SQLite persistence
├── admin-ui/           # Vue 3 + Naive UI admin SPA (embedded via //go:embed)
├── docs/               # Docusaurus docs site
├── examples/           # Example configs / scripts
├── templates/          # Default config + context templates
└── ai/                 # Internal specs and theme reference
```

## Build & Test

### Makefile

```bash
make build      # Build the Go binary into ./openpact (CGO_ENABLED=0)
make test       # Run all Go tests
make coverage   # Generate HTML coverage report
make fmt        # gofmt
make lint       # golangci-lint (install separately if needed)
make run        # Build and run locally
make docker     # Build the production Docker image
```

### Manual Build

The admin UI is embedded into the binary via `//go:embed all:dist`. Build it first:

```bash
cd admin-ui && npm ci && npm run build && cd ..
```

Then build the binary:

```bash
CGO_ENABLED=0 go build -o openpact ./cmd/openpact
```

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test -v ./internal/engine/...

# Race detector
go test -race ./...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

Some tests need the admin UI build to be present (`admin-ui/dist/`); if you skipped the UI build, the embed will fail and tests won't compile. Always build the UI at least once before running tests.

## Running Locally

### With everything in one binary

```bash
./openpact start --workspace ~/tmp/opact-dev
```

The first run creates the workspace tree under `~/tmp/opact-dev/{secure,ai-data}/`. Open `http://localhost:8888` in a browser to run through the setup wizard. After signing in to a provider you can chat from the **Sessions** view.

### Bootstrap config

There is no need for a config file in development — defaults work. If you do want to override anything, drop a `config.yaml` at `<workspace>/secure/config.yaml`:

```yaml
workspace:
  path: /home/you/tmp/opact-dev

admin:
  bind: localhost:8888

starlark:
  max_execution_ms: 30000
  max_memory_mb: 128
```

### Env vars (optional fallbacks)

```bash
export DISCORD_TOKEN=your_dev_bot_token   # Optional — can also be set in the UI
export GITHUB_TOKEN=your_github_token     # Optional — for github_* tools
```

LLM provider tokens are **not** set via env vars — sign in via the admin UI's `/engine` page.

## Admin UI Development

The admin UI is Vue 3 + Naive UI + UnoCSS, built with Vite, embedded into the Go binary via `//go:embed all:dist`.

```bash
cd admin-ui

# Install deps (uses nvm; run `nvm use` first)
npm install

# Dev server with hot reload (proxies API calls to localhost:8888)
npm run dev

# Production build (output: admin-ui/dist/)
npm run build
```

The Vite dev server runs on `http://localhost:5173`. It proxies `/api/*` to `http://localhost:8888`. To use it productively:

1. Start `./openpact start` (or `cmd/admin`) in one terminal — listens on `:8888`.
2. Start `npm run dev` in another — opens `:5173` with the live UI.
3. Edit Vue SFCs; Vite hot-reloads.

:::warning YummyAdmin theme rule
Never invent CSS values. Every height, padding, calc, and class structure must come from the theme source at `ai/theme/YummyAdmin/src/`. Read the relevant theme files before writing any new admin UI component. See `CLAUDE.md` for the full rule and rationale.
:::

## Standalone admin server (dev only)

If you want the admin UI without chat providers running:

```bash
go run ./cmd/admin --workspace ~/tmp/opact-dev
```

It builds a minimal `engine.Stack` (workspace + memory tools only) so `/api/engine/*` doesn't 503. Useful for UI work, not for production.

## Docker Development

```bash
# Build the production image
make docker
# or:
docker build -t openpact:dev .

# Run with the project's compose file
docker compose up --build
```

The compose file mounts `${HOST_WORKSPACE_PATH}` from `.env` into `/workspace`. Set it before first run:

```bash
cp .env.sample .env
# Edit HOST_WORKSPACE_PATH to point somewhere writable on your host.
mkdir -p ~/.config/openpact/workspace
```

## Hot Reloading the Go Binary

Use [air](https://github.com/cosmtrek/air):

```bash
go install github.com/cosmtrek/air@latest
air
```

`.air.toml`:

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/openpact ./cmd/openpact"
bin = "./tmp/openpact"
args = ["start"]
include_ext = ["go", "yaml"]
exclude_dir = ["tmp", "docs", "admin-ui/node_modules", "admin-ui/dist"]
```

Note: re-running `npm run build` is required to pick up admin-UI changes — air won't do that for you. For UI work, use `npm run dev` + a long-lived backend instead.

## Documentation Site

The Docusaurus site lives in `docs/`:

```bash
cd docs

# Install deps (uses yarn)
yarn install

# Dev server
yarn start

# Production build
yarn build
```

## IDE Setup

### VS Code

Recommended extensions:

- Go (by Google)
- Volar (Vue 3)
- YAML
- Docker
- EditorConfig

`.vscode/settings.json`:

```json
{
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  }
}
```

### GoLand / IntelliJ IDEA

- Enable "Format on Save".
- Configure golangci-lint as an external tool.
- Set Go SDK to 1.25+.

## Troubleshooting

### `embed: no matching files found` building the binary

You skipped the admin UI build. Run:

```bash
cd admin-ui && npm ci && npm run build && cd ..
```

### Tests fail on a fresh checkout

Same root cause as above — the `admin-ui/embed.go` `//go:embed` requires `admin-ui/dist/` to exist. Build the UI once, then tests compile cleanly.

### Setup wizard reappears on every boot

The wizard is gated on `setup_state.*` rows in `op_kv`. If you nuke the workspace DB, the wizard reappears. To preserve setup state across a re-init, copy `<workspace>/secure/data/stackllm.db` to your new workspace.

### Provider sign-in says "no credentials"

Tokens live in `<workspace>/secure/data/stackllm_auth.json` (mode `0600`). Re-sign-in via `/engine` — the file is rewritten by stackllm.

## Next Steps

- Read the [Architecture](./architecture) overview.
- Review [Code Style](./code-style) guidelines.
- Check existing [issues](https://github.com/open-pact/openpact/issues).
