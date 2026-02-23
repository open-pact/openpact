---
title: Telegram Integration
sidebar_position: 3
---

# Telegram Integration

OpenPact connects to Telegram via the Bot API, allowing your AI assistant to communicate through Telegram chats and groups. The integration uses long polling (no webhook infrastructure needed).

## Setting Up a Telegram Bot

### Create a Bot with BotFather

1. Open Telegram and search for [@BotFather](https://t.me/BotFather)
2. Send `/newbot`
3. Choose a display name (e.g., "My AI Assistant")
4. Choose a username (must end in `bot`, e.g., `my_ai_assistant_bot`)
5. BotFather will give you a token - save it securely

:::danger Never Share Your Token
Your bot token grants full control of your bot. Never commit it to version control or share it publicly. If compromised, use `/revoke` with BotFather to generate a new one.
:::

### Configure Bot Settings

With BotFather, you can optionally:

1. `/setcommands` - Register bot commands for autocomplete:
   ```
   new - Start a new conversation session
   sessions - List all conversation sessions
   switch - Switch to an existing session
   ```
2. `/setdescription` - Set a description shown when users first open the bot
3. `/setabouttext` - Set the "About" text in the bot's profile

### Find Your Telegram User ID

To restrict who can use the bot, you need your numeric user ID:

1. Message [@userinfobot](https://t.me/userinfobot) on Telegram
2. It will reply with your user ID (a number like `123456789`)
3. You can also use usernames in the allowlist

## Configuration

Set the bot token in your environment:

```bash
# .env file
TELEGRAM_BOT_TOKEN=123456789:ABCdefGhIJKlmNoPQRsTUVwxyz
```

Configure Telegram in `openpact.yaml`:

```yaml
telegram:
  enabled: true
  allowed_users:
    - "123456789"       # Numeric user ID
    - "johndoe"         # Or Telegram username (without @)
```

## User Allowlisting

When `allowed_users` is configured, only listed users can interact with the bot. Users can be identified by:

- **Numeric user ID** (recommended - doesn't change): `"123456789"`
- **Username** (may change): `"johndoe"`

```yaml
telegram:
  enabled: true
  allowed_users:
    - "123456789"     # User by ID
    - "janedoe"       # User by username
```

If `allowed_users` is empty, all users can interact with the bot.

## Bot Commands

Telegram natively supports `/command` syntax, which maps directly to OpenPact's session management:

| Command | Description |
|---------|-------------|
| `/new` | Start a new conversation session for this chat |
| `/sessions` | List all sessions (marks the active one for this chat) |
| `/switch <session_id>` | Switch this chat to a different session |

### Examples

**Start a new session:**
```
/new
→ New session started: ses_abc123... - New session
```

**List sessions:**
```
/sessions
→ Sessions:
  - ses_abc123... — Debugging the API (active in this channel)
  - ses_def456... — Code review discussion
```

**Switch to another session:**
```
/switch ses_def456
→ Switched to session: ses_def456... - Code review discussion
```

## Message Handling

### How Messages Are Processed

1. User sends a message to the bot (DM or group chat)
2. OpenPact checks the user against `allowed_users`
3. If it's a `/command`, the command handler processes it
4. Otherwise, the message is forwarded to the AI engine with source context
5. The AI response is sent back through Telegram

### Source Context

Messages to the AI include provider context:

```
[via telegram, channel:98765432, user:123456789]
What files are in the workspace?
```

### Per-Channel Sessions

Each Telegram chat (individual or group) gets its own session. If you DM the bot and also use it in a group, they maintain separate conversations.

### Message Length

Telegram has a 4096-character message limit. OpenPact automatically splits longer responses into multiple messages.

## Group Chat Usage

To use the bot in a group:

1. Add the bot to the group
2. The bot responds to all messages (not just mentions) if the sender is in `allowed_users`
3. Each group chat gets its own independent session

:::tip
If you want the bot to only respond to commands in groups (not every message), use BotFather's `/setjoingroups` to control group behavior, or manage access through `allowed_users`.
:::

## Proactive Messaging

The AI can send messages to any Telegram chat using the `chat_send` MCP tool:

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

The target is a numeric chat ID (user ID for DMs, group chat ID for groups).

## Troubleshooting

### Bot Not Responding

1. **Check the token**: Ensure `TELEGRAM_BOT_TOKEN` is set correctly
2. **Verify user ID**: Confirm your Telegram user ID is in `allowed_users`
3. **Review logs**: Check OpenPact logs for connection errors
4. **Test manually**: Message the bot directly - group permissions may differ

### Finding Chat IDs

For group chats, the chat ID is a negative number. You can find it by:

1. Adding [@RawDataBot](https://t.me/RawDataBot) to the group temporarily
2. It will display the chat ID when someone sends a message
3. Remove the bot after getting the ID

### Long Polling Issues

If the bot seems slow or misses messages:

1. Check your network connectivity
2. Ensure no other bot instances are running with the same token
3. Review the OpenPact logs for polling errors

## Related Documentation

- **[Chat Providers Overview](./chat-providers)** - Multi-provider architecture
- **[MCP Tools Reference](./mcp-tools)** - `chat_send` tool documentation
- **[Configuration Overview](../configuration/overview)** - General configuration
- **[Environment Variables](../configuration/environment-variables)** - Setting `TELEGRAM_BOT_TOKEN`
