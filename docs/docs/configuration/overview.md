---
title: Configuration Overview
sidebar_position: 1
---

# Configuration Overview

OpenPact splits configuration cleanly between two surfaces:

| What | Where | Why |
|------|-------|-----|
| **Bootstrap** — workspace path, admin bind, optional engine DB-path override | `secure/config.yaml` (and a small set of env vars) | Needed before the SQLite database is opened. |
| **Everything else** — providers, default model, logging level, rate limiter, health bind, Starlark limits, calendars, vault, GitHub, chat-provider tokens, allowed users/channels, schedules, secrets | Admin UI → SQLite (`<workspace>/secure/data/stackllm.db`) | Editable from the browser; persisted in `op_*` tables. |

The only YAML you ever need to write is the bootstrap file, and most users never need to touch even that — the defaults are sane and `WORKSPACE_PATH` / `ADMIN_BIND` env vars cover almost every deployment.

## Bootstrap Configuration File

### Location

OpenPact looks for the bootstrap file at:

1. `$CONFIG_PATH` (if set).
2. `<WORKSPACE_PATH>/secure/config.yaml`.

If the file is missing the binary still starts — bootstrap defaults are sufficient for a development run.

### Schema

The complete schema:

```yaml
# All fields are optional. Defaults shown.

workspace:
  path: /workspace          # Workspace root.

engine:
  db_path: ""               # Optional override for stackllm's SQLite.
                            # Default: <workspace>/secure/data/stackllm.db.

admin:
  enabled: true             # Serve the admin UI.
  bind: "localhost:8888"    # Bind address.
  allowlist: []             # Always-approved Starlark scripts (filenames).

starlark:
  max_execution_ms: 30000   # Per-script wall-clock cap.
  max_memory_mb: 128        # Per-script memory cap.

discord:                    # Discord defaults — every field is overrideable
  enabled: true             # from the admin UI's Providers page. The DB
  allowed_users: []         # row wins when both YAML and DB are set.
  allowed_chans: []
```

That's it. There are no `engine.type`, `engine.provider`, `engine.model`, `engine.port`, `logging.level`, `vault.*`, `calendars[]`, `github.*`, `server.*`, or `starlark.secrets` fields any more — they all live in the database.

See the [YAML Reference](./yaml-reference) for field-by-field detail.

## Where the Old YAML Fields Went

If you're migrating from a pre-stackllm install, here's where each old field is set now:

| Old YAML | New location |
|----------|-------------|
| `engine.type` / `engine.port` / `engine.password` / `engine.hostname` | Removed — stackllm runs in-process. |
| `engine.provider` / `engine.model` | Admin UI → `/engine` (default model picker). |
| `logging.level` / `logging.json` | Admin UI → `/settings/advanced` (logging section). |
| `server.health_addr` | Admin UI → `/settings/advanced` (health section). |
| `server.rate_limit.rate` / `.burst` | Admin UI → `/settings/advanced` (rate-limit section). |
| `vault.path` / `git_repo` / `auto_sync` | Admin UI → `/integrations` (Obsidian vault section). |
| `calendars[]` | Admin UI → `/integrations` (Calendar feeds section). |
| `github.enabled` | Admin UI → `/integrations` (GitHub toggle). |
| `starlark.secrets` | Admin UI → `/secrets` (AES-GCM encrypted at rest). |
| `discord.allowed_users` / `discord.allowed_channels` | Admin UI → `/providers` (per-provider allow lists). |
| `slack.*` / `telegram.*` enablement + tokens | Admin UI → `/providers`. |

**Hot-reload note:** the admin UI persists changes immediately, but a few processes (logger, rate limiter, health server) read their values once at boot. The relevant pages flag these "Restart required" inline.

## Environment Variables (Bootstrap Only)

Only a small set of env vars affects boot. Every other setting comes from the DB.

| Variable | Default | Purpose |
|----------|---------|---------|
| `WORKSPACE_PATH` | `/workspace` | Workspace root. |
| `CONFIG_PATH` | `<workspace>/secure/config.yaml` | Path to the bootstrap YAML. |
| `ADMIN_BIND` | from YAML (default `localhost:8888`) | Admin UI bind address. |
| `ADMIN_JWT_SECRET` | randomly generated and persisted to `secure/data/jwt_secret` | Override the JWT signing key. |
| `DISCORD_TOKEN` | _(unset)_ | Fallback Discord token. The DB row wins when both are set. |
| `SLACK_BOT_TOKEN` | _(unset)_ | Fallback Slack bot token. |
| `SLACK_APP_TOKEN` | _(unset)_ | Fallback Slack app-level token (Socket Mode). |
| `TELEGRAM_BOT_TOKEN` | _(unset)_ | Fallback Telegram token. |
| `GITHUB_TOKEN` | _(unset)_ | Fallback GitHub token. The `GITHUB_TOKEN` row in `op_secrets` wins when both are set. |

LLM provider credentials are **not** read from env vars. Sign in via the admin UI at `/engine`; tokens are written to `<workspace>/secure/data/stackllm_auth.json` (mode `0600`).

See [Environment Variables](./environment-variables) for fuller detail.

## Validation

The binary validates the bootstrap config on startup. The wizard refuses to finish until a default LLM model is set, so the system is never in a "running but cannot reply" state in production.

## Hot Reloading

- **Database-backed settings** that are read on each request (provider tokens, allowed users/channels, calendar list, vault config, GitHub toggle, secrets) take effect immediately.
- **Boot-time settings** (logging level, rate limit, health bind, Starlark limits) require a restart. The admin UI flags these in place.

To restart:

```bash
# Docker
docker restart openpact

# Docker Compose
docker compose restart

# systemd
sudo systemctl restart openpact
```

## Next Steps

- **[YAML Reference](./yaml-reference)** — complete bootstrap schema.
- **[Environment Variables](./environment-variables)** — full env-var list.
- **[Context Files](./context-files)** — `SOUL.md`, `USER.md`, `MEMORY.md`.
