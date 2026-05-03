---
title: First Steps
sidebar_position: 3
---

# First Steps

Now that OpenPact is running, let's set up an admin account, sign in to a provider, and verify the bot can talk to a chat platform.

## Run the Setup Wizard

Open `http://localhost:8888` in a browser. On a fresh workspace OpenPact will redirect you to a 3-step wizard:

1. **Account** — create the first admin user (username + password). This issues a refresh-token cookie immediately so subsequent steps can call authenticated endpoints.
2. **Profile** — fill in `SOUL.md`, `USER.md`, and (optionally) seed `MEMORY.md`. These get stored under `<workspace>/ai-data/` and are injected into the system prompt on the first turn of each new session.
3. **LLM Provider** — sign in to at least one provider, then pick a default model.

The wizard cannot finish until a default model is set.

## Sign in to an LLM Provider

OpenPact ships with the in-process [stackllm](https://github.com/stack-bound/stackllm) engine and exposes four providers:

| Provider | How you sign in |
|----------|-----------------|
| **OpenAI** | Paste an API key, **or** click "Sign in with ChatGPT" for the Codex device-flow (no API key required). |
| **GitHub Copilot** | Click "Sign in with GitHub". OpenPact shows a `user_code` and a verification URL — open the URL, paste the code, approve. |
| **Google Gemini** | Paste an API key from [Google AI Studio](https://aistudio.google.com/apikey). |
| **Ollama** | Enter the base URL (e.g. `http://localhost:11434`). |

Anthropic is intentionally not supported — third-party harness use is no longer permitted by Anthropic's terms.

After at least one provider is authenticated, pick a default model from the dropdown and click **Set as default**. The wizard's "Finish" button enables once a default is set.

:::tip Switch model later
You can change the default model at any time from `/engine` in the admin UI. New sessions pick up the new default immediately; existing sessions keep using their first-turn model unless you start a fresh session.
:::

## Discord Bot Setup

### Create a Discord Application

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications)
2. Click **"New Application"**
3. Give it a name (e.g., "My AI Assistant")
4. Click **"Create"**

### Configure the Bot

1. Navigate to **"Bot"** in the left sidebar
2. Click **"Add Bot"** if prompted
3. Under **"Privileged Gateway Intents"**, enable:
   - **Message Content Intent** (required for reading messages)
4. Copy the **Token**

:::caution Keep Your Token Secret
Never share your bot token or commit it to version control. Anyone with this token can control your bot.
:::

### Generate an Invite Link

1. Navigate to **"OAuth2"** > **"URL Generator"**
2. Select scopes:
   - `bot`
   - `applications.commands` (required for slash commands)
3. Select bot permissions:
   - Send Messages
   - Read Message History
   - Add Reactions (optional)
   - Attach Files (optional)
4. Copy the generated URL
5. Open it in your browser and select a server to add the bot

### Get Your Discord User ID

To restrict who can talk to your bot:

1. Enable Developer Mode in Discord:
   - User Settings > App Settings > Advanced > Developer Mode
2. Right-click your username anywhere in Discord
3. Click **"Copy User ID"**

You'll add this ID to the **Allowed Users** list in the admin UI.

### Connect the Bot

In the OpenPact admin UI:

1. Navigate to **Providers**.
2. Pick **Discord**, paste the bot token, add your user ID under **Allowed Users**.
3. Toggle **Enabled**.

OpenPact connects to Discord immediately. DM the bot or mention it in a channel — the orchestrator routes the message through the stackllm agent and replies with streaming text.

:::tip Env-var fallback
If you'd rather not paste the token in the UI, you can set `DISCORD_TOKEN` (and `SLACK_BOT_TOKEN` / `SLACK_APP_TOKEN` / `TELEGRAM_BOT_TOKEN` for those) before starting the binary. The DB-stored value wins when both are present.
:::

## Optional: GitHub Token

The `github_*` MCP tools need a personal access token:

1. Go to [github.com/settings/tokens](https://github.com/settings/tokens).
2. Click **"Generate new token (classic)"**.
3. Select scopes:
   - `repo` (private repositories)
   - or `public_repo` (public only)
4. Copy the token.

Either set it via the admin UI (**Secrets** → `GITHUB_TOKEN`) or via the `GITHUB_TOKEN` environment variable. The DB row wins if both are set.

## Test the Engine from the Admin UI

You can chat with your AI directly from the browser without involving Discord:

1. Navigate to **Sessions**.
2. Click **New Session**, type a prompt, hit send.
3. The reply streams in via SSE. Detail-mode toggles let you show/hide thinking blocks and tool calls.

If anything goes wrong, check the logs (`docker compose logs -f`, or `journalctl -u openpact -f`).

## Verify Health

```bash
curl http://localhost:8081/healthz   # Default health-server bind; configurable in /settings/advanced
```

The detailed `/health` endpoint reports per-component status (chat providers, scheduler, etc.).

## Initial Customization

The setup wizard already creates `SOUL.md`, `USER.md`, and `MEMORY.md` in `<workspace>/ai-data/`. You can edit them directly on disk or through the admin UI's **Profile** view. They're loaded into the system prompt on the first turn of every new session.

```markdown
# Identity (SOUL.md)

You are a helpful personal assistant. You are friendly, concise, and focused on being useful.

## Guidelines

- Be direct and helpful
- Ask clarifying questions when needed
- Respect privacy — don't share user information
- Admit when you don't know something
```

```markdown
# User Profile (USER.md)

Name: Your Name
Timezone: America/New_York
Preferences: Prefers concise responses

## Projects

- Currently working on: Project X
- Technologies: Python, React, PostgreSQL
```

```markdown
# Memory (MEMORY.md)

(The AI updates this file via the memory_write tool to remember things across sessions.)
```

## Next Steps

- **[Configuration Overview](../configuration/overview)** — what lives in YAML versus the admin UI.
- **[YAML Reference](../configuration/yaml-reference)** — the complete (and short) bootstrap schema.
- **[Context Files](../configuration/context-files)** — customize AI behavior.
- **[MCP Tools](../features/mcp-tools)** — available capabilities.

## Troubleshooting

### Bot Shows as Offline

- Verify the token in the **Providers** view (or `DISCORD_TOKEN` env var).
- Confirm the bot was actually added to your server (the OAuth invite step).
- Check logs: `docker logs openpact` or `journalctl -u openpact`.

### Bot Doesn't Respond

- Confirm Message Content Intent is enabled in the Discord Developer Portal.
- Confirm your user ID is in the Allowed Users list (or remove the restriction).
- Check the **Engine** page shows an authenticated provider with a default model selected.

### Provider Sign-in Fails

- For API-key providers: re-paste the key, double-check there are no leading/trailing whitespace characters.
- For device-flow providers (Copilot / OpenAI Codex): make sure you click the verification link **and** approve before the code expires (typically ~15 minutes).
- The stackllm `ManagedHandler` serializes one device flow per provider — if a flow is stuck, the page will reuse the in-flight code rather than minting a new one. Cancel and retry from a different tab if needed.

### Rate Limiting

If you hit rate limits, lower the rate/burst values in the admin UI's **Advanced Settings** view (`/settings/advanced`). Changes take effect after a restart — the rate limiter reads the value once at boot.
