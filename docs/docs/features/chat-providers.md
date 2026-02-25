---
title: Chat Providers
sidebar_position: 2
---

# Chat Providers

OpenPact supports multiple chat platforms simultaneously through a unified provider interface. Each provider connects your AI assistant to a different platform while sharing the same engine, MCP tools, and context files.

## Supported Providers

| Provider | Library | Connection | Commands |
|----------|---------|------------|----------|
| **Discord** | discordgo | WebSocket | Slash commands (`/new`, `/sessions`, `/switch`, `/context`, `/mode-*`) |
| **Telegram** | go-telegram-bot-api | Long polling | Bot commands (`/new`, `/sessions`, `/switch`, `/context`) |
| **Slack** | slack-go | Socket Mode | Slash commands (`/openpact-new`, `/openpact-context`, etc.) |

## Architecture

All providers implement the same `chat.Provider` interface, which the orchestrator uses to manage message routing and session tracking.

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│   Discord    │   │  Telegram   │   │    Slack    │
│   Client     │   │   Client    │   │   Client    │
└──────┬───────┘   └──────┬──────┘   └──────┬──────┘
       │                  │                  │
       ▼                  ▼                  ▼
┌──────────────────────────────────────────────────┐
│              Chat Provider Interface              │
│  SetMessageHandler() / SetCommandHandler()        │
│  Start() / Stop() / SendMessage()                 │
└──────────────────────┬───────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────┐
│                 Orchestrator                      │
│  Per-channel session management                   │
│  Source context injection                         │
│  Unified command handling                         │
└──────────────────────┬───────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────┐
│              AI Engine (OpenCode)                 │
└──────────────────────────────────────────────────┘
```

## Per-Channel Sessions

Each `(provider, channelID)` pair gets its own independent session. This means:

- A Discord channel `#general` has a separate conversation from Telegram group `MyChat`
- Two Discord channels each maintain their own session and history
- The `/switch` command only affects the channel where it was issued

Session mappings are persisted to `<DataDir>/channel_sessions.json`, and detail mode settings to `<DataDir>/channel_modes.json`:

```json
{
  "sessions": {
    "discord:123456789": "ses_abc123",
    "telegram:98765432": "ses_def456",
    "slack:C12345678": "ses_ghi789"
  }
}
```

### Automatic Session Creation

If a channel has no active session when a message arrives, one is created automatically. You don't need to run `/new` before chatting.

### Session Commands

All providers support the same commands:

| Command | Discord | Telegram | Slack | Description |
|---------|---------|----------|-------|-------------|
| New session | `/new` | `/new` | `/openpact-new` | Start a fresh conversation |
| List sessions | `/sessions` | `/sessions` | `/openpact-sessions` | Show all sessions |
| Switch session | `/switch <id>` | `/switch <id>` | `/openpact-switch <id>` | Switch to existing session |
| Context usage | `/context` | `/context` | `/openpact-context` | Show context window usage |
| Detail mode | `/mode-simple`, `/mode-thinking`, `/mode-tools`, `/mode-full` | `/mode-simple`, etc. | — | Control response detail level ([Discord docs](./discord-integration#detail-mode)) |

## Source Context

When a message arrives from any provider, the orchestrator prepends source information before sending it to the AI engine:

```
[via telegram, channel:98765432, user:12345]
What's the weather like today?
```

This lets the AI know which platform and channel a message came from, enabling provider-aware responses.

## Unified `chat_send` MCP Tool

The AI can proactively send messages to any connected provider using the `chat_send` MCP tool:

```json
{
  "name": "chat_send",
  "arguments": {
    "provider": "telegram",
    "target": "98765432",
    "message": "Reminder: Your meeting starts in 15 minutes!"
  }
}
```

The `provider` parameter determines which platform to send through. The `target` format depends on the provider:

| Provider | Target Format | Examples |
|----------|--------------|---------|
| Discord | Channel ID or `user:<id>` for DMs | `123456789`, `user:987654321` |
| Telegram | Chat ID (numeric) | `98765432`, `-100123456789` |
| Slack | Channel ID or user ID | `C12345678`, `U12345678` |

## Enabling Multiple Providers

Configure each provider in `openpact.yaml`:

```yaml
discord:
  enabled: true
  allowed_users:
    - "123456789012345678"

telegram:
  enabled: true
  allowed_users:
    - "987654321"

slack:
  enabled: true
  allowed_users:
    - "U12345678"
  allowed_chans:
    - "C12345678"
```

Set the corresponding environment variables:

```bash
DISCORD_TOKEN=your_discord_bot_token
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
SLACK_BOT_TOKEN=xoxb-your-slack-bot-token
SLACK_APP_TOKEN=xapp-your-slack-app-token
```

If a provider is enabled but its token is missing, OpenPact logs a warning and skips that provider. The remaining providers still start normally.

## Admin UI Sessions

The Admin UI can create, view, and chat with any session — including those created by chat providers. Each chat provider channel tracks its own session independently, so actions in the Admin UI have no effect on which session a Discord channel or Telegram group is using, and vice versa.

## Related Documentation

- **[Discord Integration](./discord-integration)** - Discord setup and configuration
- **[Telegram Integration](./telegram-integration)** - Telegram setup and configuration
- **[Slack Integration](./slack-integration)** - Slack setup and configuration
- **[MCP Tools Reference](./mcp-tools)** - `chat_send` tool documentation
- **[Configuration Overview](../configuration/overview)** - General configuration
