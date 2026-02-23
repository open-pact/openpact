---
title: Slack Integration
sidebar_position: 4
---

# Slack Integration

OpenPact connects to Slack using Socket Mode, which means no public URL or webhook infrastructure is needed. Your AI assistant can communicate through Slack channels and direct messages.

## Setting Up a Slack App

### Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Click **Create New App** > **From scratch**
3. Name your app (e.g., "OpenPact AI") and select your workspace
4. Click **Create App**

### Enable Socket Mode

1. In the left sidebar, go to **Socket Mode**
2. Toggle **Enable Socket Mode** on
3. Create an app-level token with the `connections:write` scope
4. Name it (e.g., "openpact-socket") and click **Generate**
5. Copy the token (starts with `xapp-`) - this is your `SLACK_APP_TOKEN`

### Configure Bot Permissions

1. Go to **OAuth & Permissions** in the left sidebar
2. Under **Bot Token Scopes**, add:
   - `chat:write` - Send messages
   - `channels:read` - View channel info
   - `channels:history` - Read channel messages
   - `im:read` - View DM info
   - `im:history` - Read DM messages
   - `app_mentions:read` - Detect @mentions

### Subscribe to Events

1. Go to **Event Subscriptions** in the left sidebar
2. Toggle **Enable Events** on
3. Under **Subscribe to bot events**, add:
   - `message.channels` - Messages in public channels
   - `message.im` - Direct messages

### Create Slash Commands

1. Go to **Slash Commands** in the left sidebar
2. Create these commands:

| Command | Description | Usage Hint |
|---------|-------------|------------|
| `/openpact-new` | Start a new conversation session | |
| `/openpact-sessions` | List all conversation sessions | |
| `/openpact-switch` | Switch to an existing session | `[session_id]` |

:::note Slack Command Naming
Slack requires globally unique slash command names within a workspace. The `/openpact-` prefix avoids conflicts. OpenPact strips this prefix internally, so `/openpact-new` maps to the `new` command.
:::

### Install to Workspace

1. Go to **Install App** in the left sidebar
2. Click **Install to Workspace**
3. Review and authorize the permissions
4. Copy the **Bot User OAuth Token** (starts with `xoxb-`) - this is your `SLACK_BOT_TOKEN`

## Configuration

Set the tokens in your environment:

```bash
# .env file
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_APP_TOKEN=xapp-your-app-token
```

Configure Slack in `openpact.yaml`:

```yaml
slack:
  enabled: true
  allowed_users:
    - "U12345678"       # Slack user ID
  allowed_chans:
    - "C12345678"       # Allowed channel ID
```

### Finding User and Channel IDs

- **User ID**: Click on a user's name > **View profile** > **More** (three dots) > **Copy member ID**
- **Channel ID**: Right-click a channel name > **View channel details** > the ID is at the bottom

## Allowlisting

### User Allowlisting

When `allowed_users` is configured, only those users' messages are processed:

```yaml
slack:
  enabled: true
  allowed_users:
    - "U12345678"     # User 1
    - "U23456789"     # User 2
```

If `allowed_users` is empty, all users can interact with the bot.

### Channel Allowlisting

Restrict which channels the bot responds in:

```yaml
slack:
  enabled: true
  allowed_users:
    - "U12345678"
  allowed_chans:
    - "C12345678"     # #ai-assistant
    - "C23456789"     # #engineering
```

If `allowed_chans` is empty, the bot responds in all channels it has been added to.

## Slash Commands

| Slack Command | Maps To | Description |
|---------------|---------|-------------|
| `/openpact-new` | `new` | Start a new conversation session for this channel |
| `/openpact-sessions` | `sessions` | List all sessions (marks active for this channel) |
| `/openpact-switch <id>` | `switch` | Switch this channel to a different session |

Command responses are ephemeral (only visible to the user who ran the command).

### Examples

**Start a new session:**
```
/openpact-new
→ New session started: ses_abc123... - New session
```

**List sessions:**
```
/openpact-sessions
→ Sessions:
  - ses_abc123... — Debugging the API (active in this channel)
  - ses_def456... — Code review discussion
```

**Switch to another session:**
```
/openpact-switch ses_def456
→ Switched to session: ses_def456... - Code review discussion
```

## Message Handling

### How Messages Are Processed

1. User sends a message in an allowed channel or DM
2. OpenPact checks user and channel allowlists
3. Messages from the bot itself and message subtypes (edits, joins, etc.) are ignored
4. The message is forwarded to the AI engine with source context
5. The AI response is posted back to the same channel

### Source Context

Messages to the AI include provider context:

```
[via slack, channel:C12345678, user:U12345678]
Can you check the latest deployment?
```

### Per-Channel Sessions

Each Slack channel and DM conversation gets its own session. The `#engineering` channel has a separate conversation from `#design`, and both are independent from any DM conversations.

## Proactive Messaging

The AI can send messages to any Slack channel or user using the `chat_send` MCP tool:

```json
{
  "name": "chat_send",
  "arguments": {
    "provider": "slack",
    "target": "C12345678",
    "message": "Build completed successfully! All 171 tests passed."
  }
}
```

For DMs, use the user ID as the target:

```json
{
  "name": "chat_send",
  "arguments": {
    "provider": "slack",
    "target": "U12345678",
    "message": "Your report is ready."
  }
}
```

## Troubleshooting

### Bot Not Responding

1. **Check both tokens**: Both `SLACK_BOT_TOKEN` and `SLACK_APP_TOKEN` must be set
2. **Socket Mode**: Verify Socket Mode is enabled in the Slack app settings
3. **Event subscriptions**: Ensure `message.channels` and `message.im` events are subscribed
4. **Channel membership**: The bot must be invited to channels with `/invite @BotName`
5. **Review logs**: Check OpenPact logs for Slack auth or socket errors

### Permission Errors

If the bot can read but not send messages:

1. Verify `chat:write` scope is in the bot token scopes
2. Reinstall the app if you added scopes after initial installation
3. Check channel-specific permission overrides in Slack

### Slash Commands Not Working

1. Verify commands are created in the Slack app settings
2. Check that the command names match exactly (`/openpact-new`, `/openpact-sessions`, `/openpact-switch`)
3. Reinstall the app after adding new slash commands

### Bot Goes Offline

Socket Mode connections can drop. OpenPact logs reconnection attempts. If the bot stays offline:

1. Check your network connectivity
2. Verify the `SLACK_APP_TOKEN` hasn't been revoked
3. Restart OpenPact

## Related Documentation

- **[Chat Providers Overview](./chat-providers)** - Multi-provider architecture
- **[MCP Tools Reference](./mcp-tools)** - `chat_send` tool documentation
- **[Configuration Overview](../configuration/overview)** - General configuration
- **[Environment Variables](../configuration/environment-variables)** - Setting Slack tokens
