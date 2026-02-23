---
title: First Steps
sidebar_position: 3
---

# First Steps

Now that OpenPact is installed, let's configure it properly for your use case.

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
4. Copy the **Token** - this is your `DISCORD_TOKEN`

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

Use this ID in your configuration's `allowed_users` list.

## AI Provider Authentication

The easiest way to authenticate with your AI provider is through the **Admin UI**:

1. Open the Admin UI at `http://localhost:8080`
2. Navigate to **Engine Auth**
3. Click **Sign In** to authenticate via OAuth (no API key needed)

This uses browser-based OAuth, so you do not need to manage API keys manually.

:::tip Alternative: API Keys
If you prefer pay-per-token billing or need to run headless without OAuth, you can set a provider API key as an environment variable instead:

| Provider | Environment Variable | Get Key At |
|----------|---------------------|------------|
| Anthropic | `ANTHROPIC_API_KEY` | [console.anthropic.com](https://console.anthropic.com/) |
| OpenAI | `OPENAI_API_KEY` | [platform.openai.com](https://platform.openai.com/) |
| Google | `GOOGLE_API_KEY` | [aistudio.google.com](https://aistudio.google.com/) |
:::

### GitHub Token (Optional)

For GitHub integration:

1. Go to [github.com/settings/tokens](https://github.com/settings/tokens)
2. Click **"Generate new token (classic)"**
3. Select scopes:
   - `repo` (for private repositories)
   - `public_repo` (for public repositories only)
4. Copy the token - this is your `GITHUB_TOKEN`

## Basic Configuration

### Create the Configuration File

Create `openpact.yaml` with your settings:

```yaml
# Workspace for file storage
workspace:
  path: /workspace

# Discord bot settings
discord:
  enabled: true
  allowed_users:
    - "YOUR_DISCORD_USER_ID"  # Replace with your ID

# AI engine configuration
engine:
  type: opencode
  provider: anthropic
  model: claude-sonnet-4-20250514

# Logging settings
logging:
  level: info
  json: false

# Health check server
server:
  health_addr: ":8080"
```

### Environment Variables

Set your secrets as environment variables:

```bash
export DISCORD_TOKEN=your_discord_bot_token
```

Or use a `.env` file with Docker Compose:

```bash
# .env
DISCORD_TOKEN=your_discord_bot_token
```

## Testing the Connection

### Start OpenPact

```bash
# Docker
docker run -d \
  --name openpact \
  -v openpact-workspace:/workspace \
  -v $(pwd)/openpact.yaml:/config/openpact.yaml:ro \
  -e DISCORD_TOKEN=$DISCORD_TOKEN \
  -p 8080:8080 \
  ghcr.io/open-pact/openpact:latest \
  --config /config/openpact.yaml

# Or with Docker Compose
docker compose up -d
```

### Verify Health

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{
  "status": "healthy",
  "checks": {
    "discord": "connected",
    "mcp": "ready"
  }
}
```

### Test Discord

1. Open Discord and find your bot
2. Send a direct message: "Hello!"
3. The bot should respond using the configured AI model

If you added yourself to `allowed_users`, only you can message the bot. Remove the list to allow anyone.

## Initial Customization

### Set Up Identity (SOUL.md)

Create a `SOUL.md` file in your workspace to give your AI a personality:

```markdown
# Identity

You are a helpful personal assistant. You are friendly, concise, and focused on being useful.

## Guidelines

- Be direct and helpful
- Ask clarifying questions when needed
- Respect privacy - don't share user information
- Admit when you don't know something
```

### Add Personal Context (USER.md)

Create a `USER.md` file with information about yourself:

```markdown
# User Profile

Name: Your Name
Timezone: America/New_York
Preferences: Prefers concise responses

## Projects

- Currently working on: Project X
- Technologies: Python, React, PostgreSQL
```

### Enable Memory (MEMORY.md)

Create a `MEMORY.md` file for persistent notes:

```markdown
# Memory

## Important Notes

(The AI can update this file to remember things)
```

## Next Steps

- **[Configuration Overview](../configuration/overview)** - Full configuration options
- **[YAML Reference](../configuration/yaml-reference)** - Complete settings reference
- **[Context Files](../configuration/context-files)** - Customize AI behavior
- **[MCP Tools](../features/mcp-tools)** - Available capabilities

## Troubleshooting

### Bot Shows as Offline

- Check that `DISCORD_TOKEN` is set correctly
- Verify the bot was added to your server
- Check logs: `docker logs openpact`

### Bot Doesn't Respond to Messages

- Verify Message Content Intent is enabled in Discord Developer Portal
- Check if your user ID is in `allowed_users` (or remove the restriction)
- Check logs for errors

### "Unauthorized" Errors

- Your API key may be invalid or expired
- Check that the correct environment variable is set
- Verify the key has not been revoked

### Rate Limiting

If you hit rate limits:

- Reduce the number of messages
- Configure rate limiting in `openpact.yaml`:

```yaml
server:
  rate_limit:
    rate: 5   # requests per second
    burst: 10 # max burst
```
