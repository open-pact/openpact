---
title: Environment Variables
sidebar_position: 3
---

# Environment Variables

Complete reference for all environment variables used by OpenPact.

## Required Variables

These variables must be set for OpenPact to function.

### DISCORD_TOKEN

**Required** - Discord bot authentication token.

```bash
DISCORD_TOKEN=your_discord_bot_token_here
```

Get this from the [Discord Developer Portal](https://discord.com/developers/applications) under your application's Bot settings.

:::caution Security
Never share your bot token or commit it to version control. Anyone with this token can control your bot.
:::

## AI Provider Keys

**All provider keys are optional.** The recommended way to authenticate is through the Admin UI (Engine Auth page), which uses browser-based OAuth. If you prefer pay-per-token billing or need to run headless without OAuth, set the appropriate key for your chosen provider:

| Variable | Provider | Get Key At |
|----------|----------|------------|
| `ANTHROPIC_API_KEY` | Anthropic (Claude) | [console.anthropic.com](https://console.anthropic.com/) |
| `OPENAI_API_KEY` | OpenAI (GPT) | [platform.openai.com](https://platform.openai.com/) |
| `GOOGLE_API_KEY` | Google (Gemini) | [aistudio.google.com](https://aistudio.google.com/) |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI | Azure Portal |

## Chat Provider Tokens

### TELEGRAM_BOT_TOKEN

**Optional** - Telegram bot token (required if Telegram is enabled).

```bash
TELEGRAM_BOT_TOKEN=123456789:ABCdefGhIJKlmNoPQRsTUVwxyz
```

Get this from [@BotFather](https://t.me/BotFather) on Telegram. See [Telegram Integration](../features/telegram-integration) for setup.

### SLACK_BOT_TOKEN

**Optional** - Slack Bot User OAuth Token (required if Slack is enabled).

```bash
SLACK_BOT_TOKEN=xoxb-your-bot-token
```

Get this from [api.slack.com/apps](https://api.slack.com/apps) under **OAuth & Permissions** after installing your app.

### SLACK_APP_TOKEN

**Optional** - Slack app-level token for Socket Mode (required if Slack is enabled).

```bash
SLACK_APP_TOKEN=xapp-your-app-token
```

Get this from [api.slack.com/apps](https://api.slack.com/apps) under **Basic Information** > **App-Level Tokens**. See [Slack Integration](../features/slack-integration) for setup.

## Optional Integration Keys

### GITHUB_TOKEN

GitHub personal access token for issue management.

```bash
GITHUB_TOKEN=ghp_...
```

Required scopes:
- `public_repo` - For public repositories only
- `repo` - For private repositories

Get this from [github.com/settings/tokens](https://github.com/settings/tokens).

## Script Secrets

Custom secrets for Starlark scripts. These are accessed via `secrets.get("KEY")` in scripts.

```bash
# Example script secrets
WEATHER_API_KEY=your_weather_api_key
CUSTOM_API_KEY=your_custom_key
DATABASE_TOKEN=your_database_token
```

To make secrets available to scripts, reference them in `openpact.yaml`:

```yaml
starlark:
  secrets:
    WEATHER_API_KEY: "${WEATHER_API_KEY}"
    CUSTOM_API_KEY: "${CUSTOM_API_KEY}"
```

:::tip Secret Safety
Values from `secrets.get()` are automatically redacted from all output. The AI never sees the actual secret values - only `[REDACTED:KEY_NAME]`.
:::

## Workspace Path

### WORKSPACE_PATH

The root workspace directory. All internal paths are derived from this:

```bash
WORKSPACE_PATH=/workspace  # default in Docker
```

| Derived Path | Description |
|-------------|-------------|
| `$WORKSPACE_PATH/secure/config.yaml` | Configuration file |
| `$WORKSPACE_PATH/secure/data/` | Admin data (secrets, users, approvals) |
| `$WORKSPACE_PATH/ai-data/` | AI-accessible files (MCP tools scope here) |
| `$WORKSPACE_PATH/ai-data/memory/` | Daily memory files |
| `$WORKSPACE_PATH/ai-data/scripts/` | Starlark scripts |
| `$WORKSPACE_PATH/ai-data/skills/` | Skill definitions |

There is no separate `OPENPACT_DATA_DIR` variable -- all paths are derived from `WORKSPACE_PATH`.

## Runtime Configuration

These variables override corresponding YAML configuration values.

### OPENPACT_ENGINE_TYPE

AI engine type override.

```bash
OPENPACT_ENGINE_TYPE=opencode
```

### OPENPACT_PROVIDER

LLM provider override.

```bash
OPENPACT_PROVIDER=anthropic  # anthropic, openai, google, ollama, etc.
```

### OPENPACT_MODEL

AI model override.

```bash
OPENPACT_MODEL=claude-sonnet-4-20250514
```

## Logging Configuration

### OPENPACT_LOG_LEVEL

Log verbosity level.

```bash
OPENPACT_LOG_LEVEL=info  # debug, info, warn, error
```

| Value | Description |
|-------|-------------|
| `debug` | Verbose output for troubleshooting |
| `info` | Normal operational messages (default) |
| `warn` | Warning conditions only |
| `error` | Error conditions only |

### OPENPACT_LOG_JSON

Enable JSON-formatted logging for production.

```bash
OPENPACT_LOG_JSON=true  # true or false
```

## Server Configuration

### OPENPACT_HEALTH_ADDR

Address for the health check HTTP server.

```bash
OPENPACT_HEALTH_ADDR=:8080  # default
OPENPACT_HEALTH_ADDR=:9090  # alternative port
OPENPACT_HEALTH_ADDR=0.0.0.0:8080  # bind to all interfaces
```

### OPENPACT_RATE_LIMIT

Requests per second limit.

```bash
OPENPACT_RATE_LIMIT=10  # default
```

### OPENPACT_RATE_BURST

Maximum request burst size.

```bash
OPENPACT_RATE_BURST=20  # default
```

## Setting Environment Variables

### Linux/macOS (Shell)

```bash
# Temporary (current session)
export DISCORD_TOKEN=your_token

# Permanent (add to ~/.bashrc or ~/.zshrc)
echo 'export DISCORD_TOKEN=your_token' >> ~/.bashrc
```

### Docker Run

```bash
docker run -d \
  -e DISCORD_TOKEN=your_token \
  ghcr.io/open-pact/openpact:latest
```

### Docker Compose (.env file)

Create a `.env` file in the same directory as `docker-compose.yml`:

```bash
# .env
DISCORD_TOKEN=your_discord_bot_token
GITHUB_TOKEN=your_github_token

# Script secrets
WEATHER_API_KEY=your_weather_api_key
```

Then reference in `docker-compose.yml`:

```yaml
services:
  openpact:
    env_file:
      - .env
```

Or use explicit environment references:

```yaml
services:
  openpact:
    environment:
      - DISCORD_TOKEN=${DISCORD_TOKEN}
```

## Security Best Practices

### Do

- Use environment variables for all secrets
- Use `.env` files for local development (add to `.gitignore`)
- Use secret management tools in production (Vault, AWS Secrets Manager, etc.)
- Rotate keys periodically
- Use minimal required scopes for API keys

### Don't

- Hard-code secrets in configuration files
- Commit `.env` files to version control
- Share API keys or tokens
- Use production keys in development
- Log secret values

### .gitignore Example

```gitignore
# Never commit these
.env
.env.local
.env.production
*.key
*.pem
```

## Troubleshooting

### Variable Not Being Read

1. Check spelling (case-sensitive)
2. Verify the variable is exported: `echo $VARIABLE_NAME`
3. Check for extra spaces or quotes
4. Restart the application after changes

### Docker Not Seeing Variables

1. Verify `-e` flag syntax: `-e VAR=value`
2. Check `.env` file is in the correct location
3. Verify `env_file` path in `docker-compose.yml`

### Precedence Issues

Remember the precedence order:
```
CLI flags > Environment variables > Config file > Defaults
```

An environment variable will override the config file value.
