---
title: Installation
sidebar_position: 2
---

# Installation

OpenPact ships as a single static Go binary with the admin UI embedded. You can run it directly on Linux/macOS/Windows, drop it into a slim Docker container, or build from source. There is no Node, no separate AI service, and no two-user split — everything runs in one process.

## Docker (recommended)

The official image is a single-user `debian:bookworm-slim` image with one binary inside.

### Quick Start

```bash
docker run -d \
  --name openpact \
  -v openpact-workspace:/workspace \
  -p 8888:8888 \
  ghcr.io/open-pact/openpact:latest
```

The admin UI is published on port `8888`. Open `http://localhost:8888` to run through the setup wizard (account → profile → LLM provider login + default model).

:::tip Discord/Slack/Telegram tokens
Chat-provider tokens can be set in the admin UI (Providers page) **or** via the `DISCORD_TOKEN`, `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN`, `TELEGRAM_BOT_TOKEN` env vars as a fallback. The DB wins when both are present.
:::

### With a Bootstrap Configuration File

If you want non-default workspace paths or admin binds you can mount a small `config.yaml` into `<workspace>/secure/`:

```bash
docker run -d \
  --name openpact \
  -v openpact-workspace:/workspace \
  -v /path/to/config.yaml:/workspace/secure/config.yaml:ro \
  -p 8888:8888 \
  ghcr.io/open-pact/openpact:latest
```

This file only carries bootstrap values (workspace path, optional engine DB-path override, admin bind, Starlark allowlist). Everything else — providers, models, logging level, calendars, vault, GitHub — is set from the admin UI and lives in SQLite. See [YAML Reference](../configuration/yaml-reference) for the full schema.

### Available Tags

| Tag | Description |
|-----|-------------|
| `latest` | Latest stable release |
| `vX.Y.Z` | Specific version (e.g., `v1.0.0`) |
| `main` | Latest development build (may be unstable) |

## Docker Compose

Create a `docker-compose.yml`:

```yaml
services:
  openpact:
    image: ghcr.io/open-pact/openpact:latest
    container_name: openpact
    restart: unless-stopped
    ports:
      - "8888:8888"
    volumes:
      - openpact-workspace:/workspace
    env_file:
      - .env

volumes:
  openpact-workspace:
```

And a `.env` (never commit this):

```bash
# Optional fallbacks — these can also be set in the admin UI.
DISCORD_TOKEN=
GITHUB_TOKEN=
```

Then:

```bash
docker compose up -d
```

### Common Commands

```bash
docker compose up -d        # Start in background
docker compose logs -f      # Follow logs
docker compose down         # Stop
docker compose pull && docker compose up -d   # Update
```

## Native Binary (no Docker)

OpenPact is a single static binary. Releases for Linux, macOS, and Windows can be downloaded from GitHub (or built from source — see below).

```bash
# Run with the default workspace (./workspace)
./openpact start

# Or point at a specific workspace
./openpact start --workspace ~/.local/share/openpact
```

The first run creates the workspace tree (`secure/`, `ai-data/`) with appropriate permissions and starts the admin UI on `localhost:8888` by default. Open the URL and complete the setup wizard to sign in to a provider.

### systemd

A hardened systemd unit ships at `docs/systemd/openpact.service`. To install:

```bash
sudo cp docs/systemd/openpact.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now openpact
```

The unit pins `NoNewPrivileges`, `ProtectSystem=strict`, `MemoryDenyWriteExecute`, and scopes `ReadWritePaths` to the workspace directory.

## Building from Source

### Prerequisites

- **Go 1.25** or later
- **Node.js** (any version that works with Vite — the project uses **nvm**; run `nvm use` first)
- **Make** (optional, for convenience)
- **Git**

### Clone and Build

```bash
git clone https://github.com/open-pact/openpact.git
cd openpact

# Build the embedded admin UI first — required because admin-ui/embed.go
# uses //go:embed all:dist.
cd admin-ui && npm ci && npm run build && cd ..

# Build the binary (CGO-free; pure-Go SQLite via modernc.org/sqlite).
make build
# or:
CGO_ENABLED=0 go build -o openpact ./cmd/openpact
```

The resulting binary is roughly 22 MB.

### Run Locally

```bash
./openpact start --workspace ~/tmp/opact-dev
```

Open `http://localhost:8888` and complete the setup wizard.

### Run Tests

```bash
make test       # All Go tests
make coverage   # HTML coverage report
make lint       # Requires golangci-lint
```

## System Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 1 core | 2+ cores |
| RAM | 256 MB | 1+ GB |
| Disk | 50 MB binary + workspace | 1+ GB workspace |
| Network | Outbound HTTPS | Stable connection |

### Supported Platforms

- **Linux**: x86_64, ARM64
- **macOS**: Intel, Apple Silicon
- **Windows**: x86_64 (native or via WSL2)

### Network Requirements

OpenPact needs outbound access to:

- The LLM provider you sign in to (OpenAI, Google, GitHub Copilot device-flow endpoints, or your local Ollama).
- Any chat platforms you enable (Discord, Slack, Telegram).
- Any third-party services you integrate (GitHub API, calendar feeds, vault git remote).

## Updating

### Docker

```bash
docker compose pull
docker compose up -d
```

### Native binary

Replace the binary and restart your service. The workspace is forward-compatible — schema migrations run automatically on boot.

## Next Steps

- **[First Steps](./first-steps)** — sign in to your first provider and try the admin chat.
- **[Configuration Overview](../configuration/overview)** — what lives in `config.yaml` versus the admin UI.
- **[Environment Variables](../configuration/environment-variables)** — bootstrap-only env vars.
