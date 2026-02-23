---
title: Configuration Overview
sidebar_position: 1
---

# Configuration Overview

OpenPact uses a combination of YAML configuration files and environment variables to control its behavior.

## Configuration Sources

OpenPact reads configuration from multiple sources, in order of precedence (highest first):

1. **Command-line flags** - Override everything
2. **Environment variables** - Override config file values
3. **Configuration file** - Base configuration (`openpact.yaml`)
4. **Default values** - Built-in defaults

## Configuration File

The primary configuration is stored in a YAML file, typically named `openpact.yaml`.

### Location

By default, OpenPact looks for configuration in:

1. Path specified by `--config` flag
2. `./openpact.yaml` (current directory)
3. `/config/openpact.yaml` (Docker default)

### Minimal Example

```yaml
workspace:
  path: /workspace

discord:
  enabled: true
  allowed_users:
    - "123456789012345678"

engine:
  type: opencode
  provider: anthropic
  model: claude-sonnet-4-20250514
```

### Full Example

```yaml
# Workspace configuration
workspace:
  path: /workspace

# Chat provider settings
discord:
  enabled: true
  allowed_users:
    - "123456789012345678"
  allowed_channels:
    - "987654321098765432"

telegram:
  enabled: false

slack:
  enabled: false

# Obsidian vault integration
vault:
  path: /vault
  git_repo: git@github.com:user/vault.git
  auto_sync: true

# Calendar feeds
calendars:
  - name: Personal
    url: https://calendar.google.com/calendar/ical/...
  - name: Work
    url: https://outlook.office365.com/owa/calendar/...

# GitHub integration
github:
  enabled: true

# Starlark scripting
starlark:
  enabled: true
  max_execution_ms: 30000
  secrets:
    WEATHER_API_KEY: "${WEATHER_API_KEY}"
    CUSTOM_API_KEY: "${CUSTOM_API_KEY}"

# AI engine configuration
engine:
  type: opencode
  provider: anthropic
  model: claude-sonnet-4-20250514

# Logging configuration
logging:
  level: info
  json: false

# Server configuration
server:
  health_addr: ":8080"
  rate_limit:
    rate: 10
    burst: 20
```

## Environment Variables

Sensitive values like API keys should be passed as environment variables rather than stored in configuration files.

### Required Variables

| Variable | Description |
|----------|-------------|
| `DISCORD_TOKEN` | Discord bot token (if Discord enabled) |

### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ANTHROPIC_API_KEY` | Anthropic API key (alternative to OAuth sign-in) | - |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token | - |
| `SLACK_BOT_TOKEN` | Slack bot token | - |
| `SLACK_APP_TOKEN` | Slack app-level token (Socket Mode) | - |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `GOOGLE_API_KEY` | Google AI API key | - |
| `GITHUB_TOKEN` | GitHub personal access token | - |
| `OPENPACT_LOG_LEVEL` | Log level | `info` |
| `OPENPACT_LOG_JSON` | JSON logging | `false` |

See [Environment Variables](./environment-variables) for the complete reference.

## Variable Substitution

Environment variables can be referenced in the configuration file using `${VAR_NAME}` syntax:

```yaml
starlark:
  secrets:
    API_KEY: "${MY_API_KEY}"  # Substituted from environment
```

This is particularly useful for:
- Keeping secrets out of config files
- Different values per environment (dev, staging, production)
- Dynamic configuration

## Configuration Precedence

When the same setting is specified in multiple places, the following precedence applies:

```
CLI flags > Environment variables > Config file > Defaults
```

For example:
- Config file sets `logging.level: info`
- Environment variable `OPENPACT_LOG_LEVEL=debug` overrides it
- CLI flag `--log-level=warn` overrides everything

## Validating Configuration

OpenPact validates configuration on startup. Invalid configuration will prevent the application from starting.

Common validation errors:

- **Missing required fields**: Discord token, workspace path
- **Invalid values**: Unknown log level, invalid rate limit values
- **Path errors**: Non-existent directories (for some configurations)

Check logs for specific error messages if startup fails.

## Hot Reloading

Currently, OpenPact requires a restart to apply configuration changes. Future versions may support hot reloading for some settings.

To apply changes:

```bash
# Docker
docker restart openpact

# Docker Compose
docker compose restart

# From source
# Stop and restart the process
```

## Next Steps

- **[YAML Reference](./yaml-reference)** - Complete YAML configuration reference
- **[Environment Variables](./environment-variables)** - All environment variables
- **[Context Files](./context-files)** - SOUL.md, USER.md, MEMORY.md
