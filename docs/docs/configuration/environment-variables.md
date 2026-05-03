---
title: Environment Variables
sidebar_position: 3
---

# Environment Variables

OpenPact reads only a small set of bootstrap env vars. LLM provider credentials, logging level, rate-limit settings, calendar feeds, vault config, and so on are **not** read from the environment — they live in the SQLite database and are managed through the admin UI.

The complete list, in alphabetical order:

| Variable | Default | Purpose |
|----------|---------|---------|
| [`ADMIN_BIND`](#admin_bind) | from YAML | Override the admin UI bind address. |
| [`ADMIN_JWT_SECRET`](#admin_jwt_secret) | randomly generated | Override the JWT signing key. |
| [`CONFIG_PATH`](#config_path) | `<workspace>/secure/config.yaml` | Path to the bootstrap YAML. |
| [`DISCORD_TOKEN`](#discord_token) | _(unset)_ | Discord bot token (DB wins when both set). |
| [`GITHUB_TOKEN`](#github_token) | _(unset)_ | GitHub PAT for the GitHub MCP tools. |
| [`SLACK_APP_TOKEN`](#slack_app_token) | _(unset)_ | Slack app-level token (Socket Mode). |
| [`SLACK_BOT_TOKEN`](#slack_bot_token) | _(unset)_ | Slack bot user OAuth token. |
| [`TELEGRAM_BOT_TOKEN`](#telegram_bot_token) | _(unset)_ | Telegram bot token. |
| [`WORKSPACE_PATH`](#workspace_path) | `/workspace` | Workspace root. |

## Bootstrap

### WORKSPACE_PATH

The root workspace directory. Almost everything else is derived from this.

```bash
WORKSPACE_PATH=/workspace   # Docker default
```

Derived paths:

| Derived path | Description |
|-------------|-------------|
| `$WORKSPACE_PATH/secure/config.yaml` | Bootstrap YAML. |
| `$WORKSPACE_PATH/secure/data/jwt_secret` | JWT signing key. |
| `$WORKSPACE_PATH/secure/data/data_encryption_key` | AES key for `op_secrets`. |
| `$WORKSPACE_PATH/secure/data/stackllm_auth.json` | LLM provider tokens (mode `0600`). |
| `$WORKSPACE_PATH/secure/data/stackllm_config.json` | Default model + recent models. |
| `$WORKSPACE_PATH/secure/data/stackllm.db` | Shared SQLite (`stackllm_*` and `op_*` tables). |
| `$WORKSPACE_PATH/ai-data/` | AI-accessible data. |
| `$WORKSPACE_PATH/ai-data/memory/` | Daily memory rolls. |
| `$WORKSPACE_PATH/ai-data/scripts/` | Starlark scripts. |
| `$WORKSPACE_PATH/ai-data/skills/` | Skills directory. |

### CONFIG_PATH

Override the bootstrap YAML location. Useful when running the binary outside the standard workspace layout (e.g. local development).

```bash
CONFIG_PATH=/etc/openpact/config.yaml
```

### ADMIN_BIND

Override the admin UI bind address.

```bash
ADMIN_BIND=0.0.0.0:8888    # Listen on all interfaces (Docker default)
ADMIN_BIND=localhost:9090  # Listen on localhost only, alternative port
```

### ADMIN_JWT_SECRET

Override the JWT signing key. By default, OpenPact generates a key on first boot and persists it to `secure/data/jwt_secret`. Set this env var if you need a deterministic key (e.g. behind a load balancer with multiple replicas — though that's an unusual deployment for a self-hosted assistant).

```bash
ADMIN_JWT_SECRET=$(openssl rand -hex 32)
```

## Chat Provider Tokens (env-var fallbacks)

These tokens can also be set via `/providers` in the admin UI. **The DB row wins when both are present**, so use one or the other consistently.

### DISCORD_TOKEN

```bash
DISCORD_TOKEN=your_discord_bot_token
```

Get one from the [Discord Developer Portal](https://discord.com/developers/applications). See [Discord Integration](../features/discord-integration).

### TELEGRAM_BOT_TOKEN

```bash
TELEGRAM_BOT_TOKEN=123456789:ABCdefGhIJKlmNoPQRsTUVwxyz
```

Get one from [@BotFather](https://t.me/BotFather). See [Telegram Integration](../features/telegram-integration).

### SLACK_BOT_TOKEN

```bash
SLACK_BOT_TOKEN=xoxb-your-bot-token
```

From your Slack app's **OAuth & Permissions** page after install.

### SLACK_APP_TOKEN

```bash
SLACK_APP_TOKEN=xapp-your-app-token
```

From your Slack app's **Basic Information** > **App-Level Tokens** section. Required for Socket Mode. See [Slack Integration](../features/slack-integration).

## Integration Tokens

### GITHUB_TOKEN

GitHub personal access token for the `github_*` MCP tools.

```bash
GITHUB_TOKEN=ghp_...
```

Required scopes:

- `public_repo` — public repositories only
- `repo` — private repositories

Generate one at [github.com/settings/tokens](https://github.com/settings/tokens). Can also be set as a `GITHUB_TOKEN` row in `op_secrets` via the admin UI's **Secrets** page; the DB row wins when both are present.

## What's NOT an Env Var Anymore

The following env vars used to be read by the binary and **are no longer**:

| Removed | Replacement |
|---------|-------------|
| `ANTHROPIC_API_KEY` | _Anthropic is not supported._ |
| `OPENAI_API_KEY` | Sign in via `/engine` (API key or Codex device flow). |
| `GOOGLE_API_KEY` | Sign in via `/engine` (Gemini API key). |
| `AZURE_OPENAI_API_KEY` | _Azure is not surfaced in the admin UI._ |
| `OPENCODE_*` | _OpenCode is no longer used._ |
| `OPENPACT_LOG_LEVEL` / `OPENPACT_LOG_JSON` | `/settings/advanced` (logging section). |
| `OPENPACT_HEALTH_ADDR` | `/settings/advanced` (health section). |
| `OPENPACT_RATE_LIMIT` / `OPENPACT_RATE_BURST` | `/settings/advanced` (rate-limit section). |
| `OPENPACT_ENGINE_TYPE` / `OPENPACT_PROVIDER` / `OPENPACT_MODEL` | `/engine`. |

LLM provider tokens are written to `<workspace>/secure/data/stackllm_auth.json` (mode `0600`) by stackllm; they never appear in process environment.

## Setting Environment Variables

### Linux/macOS shell

```bash
# Temporary
export DISCORD_TOKEN=your_token

# Persistent (bash)
echo 'export DISCORD_TOKEN=your_token' >> ~/.bashrc
```

### Docker

```bash
docker run -d \
  -e DISCORD_TOKEN=your_token \
  -e GITHUB_TOKEN=your_pat \
  -p 8888:8888 \
  -v openpact-workspace:/workspace \
  ghcr.io/open-pact/openpact:latest
```

### Docker Compose

Create a `.env` next to `docker-compose.yml` (and add it to `.gitignore`):

```bash
DISCORD_TOKEN=your_discord_bot_token
GITHUB_TOKEN=your_github_token
```

Reference it from `docker-compose.yml`:

```yaml
services:
  openpact:
    env_file:
      - .env
```

### systemd

Use the unit file at `docs/systemd/openpact.service` and add an `EnvironmentFile=`:

```ini
[Service]
EnvironmentFile=/etc/openpact/openpact.env
```

…then put your env vars in `/etc/openpact/openpact.env` (mode `0600`, owned by the service user).

## Security Best Practices

- Use environment variables (or the admin UI's encrypted `op_secrets` store) for secrets. Never hard-code into YAML.
- Add `.env` to `.gitignore` and never commit.
- Rotate tokens periodically.
- Use minimum-scope tokens (e.g. `public_repo` rather than `repo` if you don't need private repos).
- For production, prefer a secret manager (Vault, AWS Secrets Manager, Doppler, etc.) injecting env vars at runtime, **or** the admin UI's encrypted secrets store for Starlark-accessible values.

## Troubleshooting

### Variable not being read

1. Check spelling — env vars are case-sensitive.
2. Confirm it's exported: `echo $VARIABLE_NAME`.
3. Strip stray whitespace and quotes.
4. Restart the process after changes — env vars are read once at boot.

### Provider sign-in says "no credentials available"

LLM provider sign-ins live in `<workspace>/secure/data/stackllm_auth.json`, **not** in env vars. Open `/engine` and sign in via the UI.

### Token set in both YAML and admin UI — which wins?

The DB row in `op_chat_providers` always wins for chat-platform tokens. For `GITHUB_TOKEN`, the `op_secrets` row wins. Pick one source of truth and stick with it.
