---
title: YAML Reference
sidebar_position: 2
---

# YAML Configuration Reference

Complete reference for all `openpact.yaml` configuration options.

## workspace

Workspace configuration for file storage.

```yaml
workspace:
  path: ./workspace
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | `./workspace` | Path to the workspace directory |

The workspace is the top-level directory containing two subdirectories:
- `secure/` — System-only: configuration (`secure/config.yaml`) and admin data (`secure/data/` — users, approvals, secrets)
- `ai-data/` — AI-accessible: context files (SOUL.md, USER.md, MEMORY.md), memory files, Starlark scripts (`scripts/`), skills (`skills/`), and any files the AI creates or modifies

All paths are derived from `WORKSPACE_PATH`. The config file itself lives at `secure/config.yaml` within the workspace.

## discord

Discord bot configuration.

```yaml
discord:
  enabled: true
  allowed_users:
    - "123456789012345678"
    - "234567890123456789"
  allowed_channels:
    - "987654321098765432"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable/disable Discord integration |
| `allowed_users` | string[] | `[]` | Discord user IDs allowed to interact (empty = allow all) |
| `allowed_channels` | string[] | `[]` | Channel IDs where bot responds (empty = all channels) |

:::tip User and Channel IDs
Discord IDs are numeric strings. Enable Developer Mode in Discord settings to copy IDs by right-clicking on users or channels.
:::

## telegram

Telegram bot configuration.

```yaml
telegram:
  enabled: true
  allowed_users:
    - "123456789"
    - "johndoe"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable/disable Telegram integration |
| `allowed_users` | string[] | `[]` | Telegram user IDs or usernames allowed to interact (empty = allow all) |

:::tip User IDs
Telegram user IDs are numeric. You can find yours by messaging [@userinfobot](https://t.me/userinfobot). Usernames (without `@`) are also accepted.
:::

## slack

Slack bot configuration (Socket Mode).

```yaml
slack:
  enabled: true
  allowed_users:
    - "U12345678"
  allowed_chans:
    - "C12345678"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable/disable Slack integration |
| `allowed_users` | string[] | `[]` | Slack user IDs allowed to interact (empty = allow all) |
| `allowed_chans` | string[] | `[]` | Slack channel IDs where bot responds (empty = all channels) |

Requires both `SLACK_BOT_TOKEN` and `SLACK_APP_TOKEN` environment variables. See [Slack Integration](../features/slack-integration) for setup instructions.

## vault

Obsidian vault integration for note storage.

```yaml
vault:
  path: /vault
  git_repo: git@github.com:username/vault.git
  auto_sync: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | - | Local path to the vault directory |
| `git_repo` | string | - | Git repository URL for syncing |
| `auto_sync` | boolean | `false` | Automatically sync changes to git |

When `auto_sync` is enabled, changes made through `vault_write` will be automatically committed and pushed to the configured git repository.

## calendars

iCal calendar feeds for event reading.

```yaml
calendars:
  - name: Personal
    url: https://calendar.google.com/calendar/ical/example/basic.ics
  - name: Work
    url: https://outlook.office365.com/owa/calendar/abc123/calendar.ics
```

Each calendar entry:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Display name for the calendar |
| `url` | string | Yes | iCal feed URL |

Supported calendar formats:
- Google Calendar (iCal export)
- Microsoft Outlook (ICS link)
- Apple iCloud Calendar
- Any standard iCal/ICS feed

## github

GitHub integration for issue management.

```yaml
github:
  enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable GitHub integration |

Requires `GITHUB_TOKEN` environment variable with appropriate scopes:
- `public_repo` for public repositories
- `repo` for private repositories

## starlark

Sandboxed scripting configuration.

```yaml
starlark:
  enabled: true
  max_execution_ms: 30000
  secrets:
    WEATHER_API_KEY: "${WEATHER_API_KEY}"
    DATABASE_TOKEN: "${DATABASE_TOKEN}"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable Starlark scripting |
| `max_execution_ms` | integer | `30000` | Maximum script execution time (ms) |
| `secrets` | map | `{}` | Secrets available to scripts |

Scripts are always stored in the `ai-data/scripts/` subdirectory of the workspace.

### Secrets Configuration

Secrets are key-value pairs available to scripts via `secrets.get("KEY")`. Use environment variable substitution for actual values:

```yaml
starlark:
  secrets:
    # Direct value (not recommended - use env vars)
    STATIC_KEY: "hardcoded-value"

    # From environment (recommended)
    API_KEY: "${MY_API_KEY}"
```

:::caution Secret Safety
Values from `secrets.get()` are automatically redacted from any output returned to the AI. The AI never sees the actual secret values.
:::

## engine

AI engine configuration.

```yaml
engine:
  type: opencode
  provider: anthropic
  model: claude-sonnet-4-20250514
  port: 4098
  password: ""
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `type` | string | `opencode` | Engine type: `opencode` |
| `provider` | string | `anthropic` | LLM provider for OpenCode |
| `model` | string | `claude-sonnet-4-20250514` | Model identifier |
| `port` | integer | `0` | Port for `opencode serve` (0 = auto-pick a free port) |
| `password` | string | `""` | Optional password for the OpenCode server API (sets `OPENCODE_SERVER_PASSWORD`) |

OpenPact runs `opencode serve` as a persistent child process and communicates with it via REST API. See the [OpenCode server documentation](https://opencode.ai/docs/server/) for details on the underlying API.

### Supported Providers

| Provider | Provider Value | API Key Variable |
|----------|---------------|------------------|
| Anthropic | `anthropic` | `ANTHROPIC_API_KEY` |
| OpenAI | `openai` | `OPENAI_API_KEY` |
| Google | `google` | `GOOGLE_API_KEY` |
| Ollama | `ollama` | - (local) |
| Azure OpenAI | `azure` | `AZURE_OPENAI_API_KEY` |

## logging

Logging configuration.

```yaml
logging:
  level: info
  json: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `json` | boolean | `false` | Output logs in JSON format |

### Log Levels

| Level | Description |
|-------|-------------|
| `debug` | Verbose debugging information |
| `info` | Normal operational messages |
| `warn` | Warning conditions |
| `error` | Error conditions only |

### JSON Logging

Enable JSON logging for production environments and log aggregation:

```yaml
logging:
  json: true
```

Output example:
```json
{"level":"info","timestamp":"2024-01-15T10:30:00Z","message":"Discord connected","component":"discord"}
```

## server

HTTP server configuration for health checks and metrics.

```yaml
server:
  health_addr: ":8080"
  rate_limit:
    rate: 10
    burst: 20
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `health_addr` | string | `:8080` | Address for health check server |
| `rate_limit.rate` | integer | `10` | Requests per second limit |
| `rate_limit.burst` | integer | `20` | Maximum burst size |

### Health Endpoints

When the server is running, these endpoints are available:

| Endpoint | Description |
|----------|-------------|
| `/health` | Detailed health status with component checks |
| `/healthz` | Kubernetes-style liveness probe |
| `/ready` | Readiness check |
| `/metrics` | Prometheus-format metrics |

### Rate Limiting

Rate limiting applies to incoming requests. The token bucket algorithm allows:
- `rate` sustained requests per second
- Up to `burst` requests in a short burst

## Complete Example

```yaml
# Complete openpact.yaml example

workspace:
  path: /workspace

discord:
  enabled: true
  allowed_users:
    - "123456789012345678"

telegram:
  enabled: false

slack:
  enabled: false

vault:
  path: /vault
  git_repo: git@github.com:user/my-vault.git
  auto_sync: true

calendars:
  - name: Personal
    url: https://calendar.google.com/calendar/ical/example/basic.ics

github:
  enabled: true

starlark:
  enabled: true
  max_execution_ms: 30000
  secrets:
    WEATHER_API_KEY: "${WEATHER_API_KEY}"

engine:
  type: opencode
  provider: anthropic
  model: claude-sonnet-4-20250514
  port: 0
  password: ""

logging:
  level: info
  json: false

server:
  health_addr: ":8080"
  rate_limit:
    rate: 10
    burst: 20
```
