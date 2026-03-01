---
title: MCP Tools Reference
sidebar_position: 1
---

# MCP Tools Reference

OpenPact exposes capabilities to AI models through MCP (Model Context Protocol) tools. This page documents all built-in tools.

## What is MCP?

The **Model Context Protocol (MCP)** is a standard for connecting AI models to external tools and data sources. It provides:

- **Standardized Interface**: Common format for tool definitions and invocations
- **Security Boundaries**: Clear separation between AI and system capabilities
- **Extensibility**: Easy addition of new tools without changing the core system

OpenPact implements MCP to give AI models controlled access to:
- File systems (workspace, vault)
- Communication channels (Discord, Telegram, Slack)
- External services (GitHub, calendars, web)
- Custom scripts (Starlark)

## Security Model

All MCP tools in OpenPact follow the principle of least privilege:

1. **Explicit Enablement**: Tools must be configured to be available
2. **Scoped Access**: Tools can only access designated resources
3. **Secret Protection**: API keys and tokens are never exposed to the AI
4. **Audit Trail**: All tool invocations are logged

```
AI Model (cannot see secrets)
     │
     ▼
┌─────────────────────┐
│   MCP Tool Layer    │ ← Tools defined here
│   (OpenPact)        │
└─────────────────────┘
     │
     ▼
External Services (secrets injected here)
```

## Built-in Tools

### Workspace Tools

Tools for managing files in the `ai-data/` subdirectory within the workspace. All workspace tools are scoped to `ai-data/` -- the AI cannot access files in `secure/`.

#### workspace_read

Read a file from the workspace.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Relative path within `ai-data/` |

**Example:**
```json
{
  "name": "workspace_read",
  "arguments": {
    "path": "notes/todo.md"
  }
}
```

**Returns:** File contents as string, or error if file doesn't exist.

---

#### workspace_write

Write content to a file in the `ai-data/` directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Relative path within `ai-data/` |
| `content` | string | Yes | Content to write |

**Example:**
```json
{
  "name": "workspace_write",
  "arguments": {
    "path": "notes/new-note.md",
    "content": "# My Note\n\nContent here..."
  }
}
```

**Returns:** Success confirmation or error.

---

#### workspace_list

List files in the `ai-data/` directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | No | Relative path within `ai-data/` (defaults to `ai-data/` root) |

**Example:**
```json
{
  "name": "workspace_list",
  "arguments": {
    "path": "notes"
  }
}
```

**Returns:** List of files and directories.

---

### Memory Tools

Tools for reading and writing persistent memory.

#### memory_read

Read from a memory file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file` | string | No | Memory file name (defaults to MEMORY.md) |

**Example:**
```json
{
  "name": "memory_read",
  "arguments": {}
}
```

**Returns:** Memory file contents.

---

#### memory_write

Write to a memory file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `content` | string | Yes | Content to write |
| `file` | string | No | Memory file name (defaults to MEMORY.md) |
| `append` | boolean | No | Append instead of replace (default: false) |

**Example:**
```json
{
  "name": "memory_write",
  "arguments": {
    "content": "## New Section\n\n- Important note",
    "append": true
  }
}
```

**Returns:** Success confirmation.

---

### Communication Tools

#### chat_send

Send a message via any connected [chat provider](./chat-providers) (Discord, Telegram, Slack).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `provider` | string | Yes | Chat provider name (e.g., `"discord"`, `"telegram"`, `"slack"`) |
| `target` | string | Yes | Target: `user:<id>` for DMs, `channel:<id>` for channels, or just `<id>` |
| `message` | string | Yes | Message content to send |

The available providers are listed dynamically based on which providers are configured and connected.

**Example (Discord):**
```json
{
  "name": "chat_send",
  "arguments": {
    "provider": "discord",
    "target": "123456789012345678",
    "message": "Reminder: Team meeting in 15 minutes!"
  }
}
```

**Example (Telegram):**
```json
{
  "name": "chat_send",
  "arguments": {
    "provider": "telegram",
    "target": "98765432",
    "message": "Build completed successfully!"
  }
}
```

**Example (Slack):**
```json
{
  "name": "chat_send",
  "arguments": {
    "provider": "slack",
    "target": "C12345678",
    "message": "Deployment finished. All tests passed."
  }
}
```

**Returns:** Success confirmation (e.g., "Message sent via discord to 123456789012345678") or error.

:::note
This tool is for proactive messaging. Normal conversation responses don't require this tool - they're handled automatically by each provider.
:::

---

### Model Tools

Tools for viewing available AI models and changing the default model used for new sessions.

#### model_list

List all available AI models grouped by provider, showing the current default.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| - | - | - | No parameters |

**Example:**
```json
{
  "name": "model_list",
  "arguments": {}
}
```

**Returns:** Models grouped by provider with context/output limits, current default marked.

---

#### model_set_default

Set the default AI model for new sessions. Supports fuzzy matching — you can use a partial model name (e.g. "opus" or "sonnet") and the provider will be inferred automatically.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model ID or partial name to match (e.g. `"claude-sonnet-4-20250514"` or `"opus"`) |
| `provider` | string | No | Provider ID (e.g. `"anthropic"`). Inferred from model match if omitted. |

**Example (exact):**
```json
{
  "name": "model_set_default",
  "arguments": {
    "model": "claude-opus-4-20250514",
    "provider": "anthropic"
  }
}
```

**Example (fuzzy):**
```json
{
  "name": "model_set_default",
  "arguments": {
    "model": "opus"
  }
}
```

**Returns:** Confirmation with the matched model's full ID and limits, or an error with suggestions if the match is ambiguous or not found.

:::note
Changing the default model only affects **new sessions**. Existing sessions continue using the model they were started with.
:::

---

### Calendar Tools

#### calendar_read

Read events from configured calendar feeds.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `calendar` | string | No | Calendar name (reads all if not specified) |
| `days` | number | No | Number of days to look ahead (default: 7) |

**Example:**
```json
{
  "name": "calendar_read",
  "arguments": {
    "calendar": "Personal",
    "days": 14
  }
}
```

**Returns:** List of events with title, time, and location.

---

### Vault Tools

Tools for managing an Obsidian vault.

#### vault_read

Read a note from the vault.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to note within vault |

**Example:**
```json
{
  "name": "vault_read",
  "arguments": {
    "path": "Projects/ProjectX.md"
  }
}
```

**Returns:** Note contents.

---

#### vault_write

Write a note to the vault.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to note within vault |
| `content` | string | Yes | Note content |

**Example:**
```json
{
  "name": "vault_write",
  "arguments": {
    "path": "Daily/2024-01-15.md",
    "content": "# Daily Note\n\n## Tasks\n- [ ] Review code"
  }
}
```

**Returns:** Success confirmation. If auto_sync is enabled, also commits to git.

---

#### vault_list

List notes in a vault directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | No | Path within vault (defaults to root) |

**Example:**
```json
{
  "name": "vault_list",
  "arguments": {
    "path": "Projects"
  }
}
```

**Returns:** List of notes and folders.

---

#### vault_search

Search vault content.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query |

**Example:**
```json
{
  "name": "vault_search",
  "arguments": {
    "query": "meeting notes"
  }
}
```

**Returns:** List of matching notes with excerpts.

---

### Web Tools

#### web_fetch

Fetch and parse a web page.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | URL to fetch |

**Example:**
```json
{
  "name": "web_fetch",
  "arguments": {
    "url": "https://example.com/api/docs"
  }
}
```

**Returns:** Parsed content from the web page (HTML converted to readable text).

:::caution Rate Limiting
Web fetching is subject to rate limiting. Avoid excessive requests to the same domain.
:::

---

### GitHub Tools

#### github_list_issues

List issues from a GitHub repository.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `state` | string | No | Issue state: open, closed, all (default: open) |

**Example:**
```json
{
  "name": "github_list_issues",
  "arguments": {
    "owner": "open-pact",
    "repo": "openpact",
    "state": "open"
  }
}
```

**Returns:** List of issues with title, number, labels, and assignees.

---

#### github_create_issue

Create a new GitHub issue.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `title` | string | Yes | Issue title |
| `body` | string | No | Issue body (markdown) |
| `labels` | string[] | No | Labels to apply |

**Example:**
```json
{
  "name": "github_create_issue",
  "arguments": {
    "owner": "open-pact",
    "repo": "openpact",
    "title": "Add support for webhooks",
    "body": "## Description\n\nWe should add webhook support for...",
    "labels": ["enhancement"]
  }
}
```

**Returns:** Created issue URL and number.

---

### Script Tools

Tools for Starlark script execution.

#### script_list

List available Starlark scripts.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| - | - | - | No parameters |

**Example:**
```json
{
  "name": "script_list",
  "arguments": {}
}
```

**Returns:** List of scripts with name, description, and required secrets.

---

#### script_run

Execute a named Starlark script.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Script name (without .star extension) |
| `function` | string | No | Specific function to call |
| `args` | array | No | Arguments for the function |

**Example (run entire script):**
```json
{
  "name": "script_run",
  "arguments": {
    "name": "weather"
  }
}
```

**Example (call specific function):**
```json
{
  "name": "script_run",
  "arguments": {
    "name": "weather",
    "function": "get_weather",
    "args": ["London"]
  }
}
```

**Returns:** Script output (with secrets automatically redacted).

---

#### script_exec

Execute arbitrary Starlark code.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `code` | string | Yes | Starlark code to execute |

**Example:**
```json
{
  "name": "script_exec",
  "arguments": {
    "code": "result = 2 + 2\nprint(result)"
  }
}
```

**Returns:** Execution output.

:::caution Approval Required
Arbitrary code execution may require approval depending on configuration.
:::

---

#### script_reload

Reload scripts from disk.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| - | - | - | No parameters |

**Example:**
```json
{
  "name": "script_reload",
  "arguments": {}
}
```

**Returns:** List of reloaded scripts.

---

### Schedule Tools

Tools for managing [scheduled jobs](/docs/features/scheduling). Schedules run Starlark scripts or AI agent sessions on a cron timer.

#### schedule_list

List all scheduled jobs with their status and last run info.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| - | - | - | No parameters |

**Example:**
```json
{
  "name": "schedule_list",
  "arguments": {}
}
```

**Returns:** List of schedules with ID, name, type, cron expression, enabled status, output target, and last run info.

---

#### schedule_create

Create a new scheduled job. Validates the cron expression at creation time.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Human-readable name for the schedule |
| `cron_expr` | string | Yes | Cron expression (5 fields: min hour dom month dow) |
| `type` | string | Yes | Job type: `"script"` or `"agent"` |
| `script_name` | string | No | Script filename (required for type `"script"`, e.g. `"my_script.star"`) |
| `prompt` | string | No | Prompt for AI session (required for type `"agent"`) |
| `enabled` | boolean | No | Whether the schedule is active (default: `true`) |
| `output_provider` | string | No | Chat provider to send output to (e.g. `"discord"`) |
| `output_channel` | string | No | Channel ID to send output to (e.g. `"channel:123456"`) |

**Example (script job):**
```json
{
  "name": "schedule_create",
  "arguments": {
    "name": "Daily report",
    "cron_expr": "0 9 * * 1-5",
    "type": "script",
    "script_name": "daily_report.star"
  }
}
```

**Example (agent job with output):**
```json
{
  "name": "schedule_create",
  "arguments": {
    "name": "Status update",
    "cron_expr": "0 */2 * * *",
    "type": "agent",
    "prompt": "Summarize today's open issues and post a status update.",
    "output_provider": "discord",
    "output_channel": "channel:123456789"
  }
}
```

**Returns:** Confirmation with the new schedule's ID and name.

---

#### schedule_update

Update an existing scheduled job by ID. Only provided fields are updated.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | Yes | Schedule ID to update |
| `name` | string | No | New name |
| `cron_expr` | string | No | New cron expression |
| `type` | string | No | New job type: `"script"` or `"agent"` |
| `script_name` | string | No | New script name |
| `prompt` | string | No | New prompt |
| `output_provider` | string | No | Chat provider for output delivery |
| `output_channel` | string | No | Channel ID for output delivery |

**Example:**
```json
{
  "name": "schedule_update",
  "arguments": {
    "id": "a1b2c3d4e5f6g7h8",
    "cron_expr": "0 10 * * 1-5",
    "name": "Morning report"
  }
}
```

**Returns:** Confirmation with the updated schedule's ID and name.

---

#### schedule_delete

Delete a scheduled job by ID.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | Yes | Schedule ID to delete |

**Example:**
```json
{
  "name": "schedule_delete",
  "arguments": {
    "id": "a1b2c3d4e5f6g7h8"
  }
}
```

**Returns:** Confirmation that the schedule was deleted.

---

#### schedule_enable

Enable a scheduled job by ID.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | Yes | Schedule ID to enable |

**Example:**
```json
{
  "name": "schedule_enable",
  "arguments": {
    "id": "a1b2c3d4e5f6g7h8"
  }
}
```

**Returns:** Confirmation that the schedule was enabled.

---

#### schedule_disable

Disable a scheduled job by ID. The job stops running but its configuration is preserved.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | Yes | Schedule ID to disable |

**Example:**
```json
{
  "name": "schedule_disable",
  "arguments": {
    "id": "a1b2c3d4e5f6g7h8"
  }
}
```

**Returns:** Confirmation that the schedule was disabled.

---

## Tool Summary Table

| Tool | Category | Description |
|------|----------|-------------|
| `workspace_read` | Workspace | Read files from `ai-data/` |
| `workspace_write` | Workspace | Write files to `ai-data/` |
| `workspace_list` | Workspace | List `ai-data/` files |
| `memory_read` | Memory | Read memory files |
| `memory_write` | Memory | Write to memory files |
| `chat_send` | Communication | Send messages via any chat provider |
| `model_list` | Models | List available AI models |
| `model_set_default` | Models | Change the default model for new sessions |
| `calendar_read` | Calendar | Read calendar events |
| `vault_read` | Vault | Read Obsidian notes |
| `vault_write` | Vault | Write Obsidian notes |
| `vault_list` | Vault | List vault notes |
| `vault_search` | Vault | Search vault content |
| `web_fetch` | Web | Fetch web pages |
| `github_list_issues` | GitHub | List repository issues |
| `github_create_issue` | GitHub | Create new issues |
| `script_list` | Scripts | List Starlark scripts |
| `script_run` | Scripts | Run named scripts |
| `script_exec` | Scripts | Execute Starlark code |
| `script_reload` | Scripts | Reload scripts from disk |
| `schedule_list` | Schedules | List all scheduled jobs |
| `schedule_create` | Schedules | Create a new scheduled job |
| `schedule_update` | Schedules | Update an existing scheduled job |
| `schedule_delete` | Schedules | Delete a scheduled job |
| `schedule_enable` | Schedules | Enable a scheduled job |
| `schedule_disable` | Schedules | Disable a scheduled job |

## Related Documentation

- **[Configuration Overview](../configuration/overview)** - Enable and configure tools
- **[YAML Reference](../configuration/yaml-reference)** - Tool-specific settings
- **[Security Overview](../security/overview)** - Tool security model
