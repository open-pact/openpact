---
title: YAML Reference
sidebar_position: 2
---

# YAML Configuration Reference

The bootstrap file lives at `<workspace>/secure/config.yaml` (override with `CONFIG_PATH`). It only needs to carry the values OpenPact must know **before** it can open the SQLite database. Everything else is configured through the admin UI and persisted in `op_*` tables.

The file is optional — every field has a default, and the binary boots from sensible defaults if no file is present.

## workspace

Workspace path configuration.

```yaml
workspace:
  path: /workspace
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | `/workspace` | Workspace root. Override with `WORKSPACE_PATH`. |

The workspace contains:

- `secure/` — system-only (mode `0700`): `config.yaml`, `data/jwt_secret`, `data/data_encryption_key`, `data/stackllm_auth.json`, `data/stackllm_config.json`, `data/stackllm.db`.
- `ai-data/` — AI-accessible (mode `0755`): `SOUL.md`, `USER.md`, `MEMORY.md`, `memory/`, `scripts/`, `skills/`, plus any files MCP tools write.

See [Workspace](../features/workspace) for the full layout.

## engine

The only knob the YAML exposes is the SQLite path; everything else (provider credentials, default model, recent models) is managed by stackllm under `<workspace>/secure/data/`.

```yaml
engine:
  db_path: ""
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `db_path` | string | `<workspace>/secure/data/stackllm.db` | Override the SQLite path. The DB carries both stackllm's `stackllm_*` tables and OpenPact's `op_*` tables. |

The supported providers (OpenAI / GitHub Copilot / Google Gemini / Ollama) are baked into the binary. Sign in to them via the admin UI at `/engine`. Anthropic is intentionally not surfaced.

## admin

Admin UI bind + Starlark allowlist.

```yaml
admin:
  enabled: true
  bind: "localhost:8888"
  allowlist:
    - "trusted_script.star"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `true` | Serve the admin UI. Disabling it doesn't disable `/api/*` — those still run; the SPA simply isn't mounted. |
| `bind` | string | `localhost:8888` | Bind address. Override with `ADMIN_BIND`. |
| `allowlist` | string[] | `[]` | Starlark script filenames that are pre-approved (no manual approval prompt before first run). |

## starlark

Starlark sandbox limits.

```yaml
starlark:
  max_execution_ms: 30000
  max_memory_mb: 128
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_execution_ms` | integer | `30000` | Wall-clock execution cap per script. |
| `max_memory_mb` | integer | `128` | Memory cap per script. |

:::tip Mutable at runtime via the admin UI
These same fields are surfaced in `/settings/advanced` and can be tuned without a YAML edit. The DB value wins after the first wizard completion. The values here are the bootstrap defaults used on first boot.
:::

Starlark **secrets** are not configured in YAML any more — manage them at `/secrets` in the admin UI. They are AES-256-GCM encrypted at rest in `op_secrets`.

## discord (bootstrap defaults)

Discord defaults to enabled with no allow-lists. These values are picked up on first boot only and become editable in `/providers` thereafter.

```yaml
discord:
  enabled: true
  allowed_users: []
  allowed_chans: []
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `true` | Whether the Discord adapter starts. |
| `allowed_users` | string[] | `[]` | User IDs allowed to DM (empty = anyone). |
| `allowed_chans` | string[] | `[]` | Channel IDs the bot responds in (empty = all). |

The Discord, Slack, and Telegram **tokens** never go in YAML. Set them via `/providers` or via the corresponding env-var fallback (`DISCORD_TOKEN`, `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN`, `TELEGRAM_BOT_TOKEN`). The DB row wins when both are set.

## Complete Example

```yaml
# Everything below is optional. Defaults are reasonable for a single-user
# deployment listening on localhost:8888.

workspace:
  path: /workspace

engine:
  db_path: ""                  # Use the default location.

admin:
  enabled: true
  bind: "localhost:8888"
  allowlist: []

starlark:
  max_execution_ms: 30000
  max_memory_mb: 128

discord:
  enabled: true
  allowed_users: []
  allowed_chans: []
```

## What's NOT in the YAML

Settings that used to live in YAML and now live in the database (edited from the admin UI):

| Old field | New home |
|-----------|----------|
| `engine.type` / `engine.port` / `engine.password` / `engine.hostname` | _Removed — stackllm is in-process._ |
| `engine.provider` / `engine.model` | `/engine` (default model picker) |
| `logging.level` / `logging.json` | `/settings/advanced` |
| `server.health_addr` | `/settings/advanced` |
| `server.rate_limit.rate` / `.burst` | `/settings/advanced` |
| `vault.path` / `git_repo` / `auto_sync` | `/integrations` |
| `calendars[]` | `/integrations` |
| `github.enabled` | `/integrations` |
| `starlark.secrets` | `/secrets` |
| `slack.*` / `telegram.*` (full block, including tokens) | `/providers` |
| `discord.token` | `/providers` (or `DISCORD_TOKEN` env-var fallback) |

This was a deliberate redesign: every runtime-mutable knob is now a single grep `op_kv` lookup away, and the binary cannot be in a state where the YAML and the running process disagree.
