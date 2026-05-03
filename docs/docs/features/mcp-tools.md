---
title: MCP Tools Reference
sidebar_position: 1
---

# MCP Tools Reference

OpenPact exposes capabilities to its in-process LLM agent through tools registered using the **Model Context Protocol (MCP)** as a registration model. The agent runs inside the OpenPact binary (via [stackllm](https://github.com/stack-bound/stackllm)), so tool calls are Go function calls — there is no inter-process JSON-RPC at runtime.

This page documents all built-in tools.

## How tools are registered

Tools are registered **once at boot**. `mcp.RegisterAllTools` walks the configured tool catalogue, and `engine.RegisterMCPTools` copies each one into stackllm's native `tools.Registry` via `mcpToolAdapter.Call`. The adapter translates stackllm's JSON-args calling convention to MCP's `(ctx, argsMap)` shape; schemas pass through verbatim.

Every tool is conditional on the configuration that makes it usable:

| Tool group | Always registered? | Condition |
|------------|-------------------|-----------|
| Workspace, memory | Yes | — |
| Web (`web_fetch`) | Yes | — |
| Calendar | Conditional | At least one calendar feed configured (`/integrations`). |
| Vault | Conditional | Vault path configured (`/integrations`). |
| GitHub | Conditional | `GITHUB_TOKEN` set in `op_secrets` or env. |
| Script tools | Yes | Script registry initialized (default). |
| Chat (`chat_send`) | Conditional | At least one chat provider enabled. |
| Model (`model_list`, `model_set_default`) | Conditional | Provider lookup wired (production main binary; not the standalone MCP server). |
| Schedule | Conditional | Scheduler wired (production main binary). |

## Security model

All MCP tools follow the principle of least privilege:

1. **Static surface** — the registry is closed after boot; new capabilities require a binary rebuild.
2. **Scoped access** — workspace/vault paths are validated; `web_fetch` is HTTPS-capped; chat-send respects allow lists.
3. **Secret protection** — provider tokens, JWT keys, and Starlark secrets never appear in tool inputs/outputs.
4. **Audit trail** — invocations are logged via the standard structured logger.

```
LLM agent (in-process; cannot see secrets)
        │
        ▼
┌─────────────────────────────────────────┐
│   stackllm tools.Registry               │  ← static set, populated at boot
│                                         │
│   mcpToolAdapter → mcp.ToolHandler      │
└─────────────────────────────────────────┘
        │
        ▼
External services (provider-specific auth handled inside the tool)
```

## Built-in tools

### Workspace tools

Tools for managing files in `<workspace>/ai-data/`. All workspace tools are scoped to that root — `secure/` is unreachable.

#### workspace_read

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Relative path within `ai-data/` |

```json
{
  "name": "workspace_read",
  "arguments": { "path": "notes/todo.md" }
}
```

Returns: file contents as a string, or an error if the file doesn't exist.

#### workspace_write

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Relative path within `ai-data/` |
| `content` | string | Yes | Content to write |

```json
{
  "name": "workspace_write",
  "arguments": {
    "path": "notes/new-note.md",
    "content": "# My Note\n\nContent here..."
  }
}
```

Returns: success confirmation, or an error.

#### workspace_list

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | No | Relative path within `ai-data/` (defaults to root) |

```json
{
  "name": "workspace_list",
  "arguments": { "path": "notes" }
}
```

Returns: list of files and directories.

### Memory tools

#### memory_read

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file` | string | No | Memory file (default `MEMORY.md`) |

```json
{ "name": "memory_read", "arguments": {} }
```

#### memory_write

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `content` | string | Yes | Content to write |
| `file` | string | No | Memory file (default `MEMORY.md`) |
| `append` | boolean | No | Append rather than replace (default false) |

```json
{
  "name": "memory_write",
  "arguments": {
    "content": "## New Section\n\n- Important note",
    "append": true
  }
}
```

### Communication tools

#### chat_send

Send a message via any connected [chat provider](./chat-providers).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `provider` | string | Yes | `"discord"`, `"telegram"`, or `"slack"` |
| `target` | string | Yes | `user:<id>` for DMs, `channel:<id>` for channels, or just `<id>` |
| `message` | string | Yes | Message content |

The available providers are computed at runtime from the configured + connected providers in `op_chat_providers`.

```json
{
  "name": "chat_send",
  "arguments": {
    "provider": "discord",
    "target": "channel:123456789",
    "message": "Reminder: meeting in 15 minutes."
  }
}
```

:::note
This tool is for **proactive** messaging. Normal conversation responses don't need it — the orchestrator already replies on the originating provider.
:::

### Model tools

#### model_list

List all available AI models grouped by provider, with the current default marked.

```json
{ "name": "model_list", "arguments": {} }
```

Returns: models grouped by provider with context/output limits.

#### model_set_default

Set the default model for new sessions. Supports fuzzy matching — partial names work.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model ID or partial name (e.g. `"gpt-4o"` or just `"4o"`) |
| `provider` | string | No | Provider ID (e.g. `"openai"`). Inferred from the match if omitted. |

```json
{
  "name": "model_set_default",
  "arguments": { "model": "gpt-4o", "provider": "openai" }
}
```

Or fuzzy:

```json
{
  "name": "model_set_default",
  "arguments": { "model": "gemini-2.0-flash" }
}
```

:::note
Changing the default only affects **new** sessions. Existing sessions keep their original model.
:::

### Calendar tools

#### calendar_read

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `calendar` | string | No | Calendar name (reads all if omitted) |
| `days` | number | No | Days to look ahead (default 7) |

```json
{
  "name": "calendar_read",
  "arguments": { "calendar": "Personal", "days": 14 }
}
```

Calendar feeds are configured at `/integrations` in the admin UI.

### Vault tools

Tools for managing an Obsidian vault. Path configured at `/integrations`.

#### vault_read

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to note within vault |

#### vault_write

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path within vault |
| `content` | string | Yes | Note content |

If `auto_sync` is enabled, the change is committed to git automatically.

#### vault_list

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | No | Path within vault (defaults to root) |

#### vault_search

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query |

### Web tools

#### web_fetch

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | URL to fetch (HTTP or HTTPS) |

```json
{
  "name": "web_fetch",
  "arguments": { "url": "https://example.com/api/docs" }
}
```

Returns: HTML converted to readable text. Response size is capped; only `http://` and `https://` schemes are accepted.

### GitHub tools

Token configured via `GITHUB_TOKEN` env var or the `GITHUB_TOKEN` row in `op_secrets`. Toggle enablement at `/integrations`.

#### github_list_issues

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `state` | string | No | `open` / `closed` / `all` (default `open`) |

#### github_create_issue

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `title` | string | Yes | Issue title |
| `body` | string | No | Markdown body |
| `labels` | string[] | No | Labels |

### Script tools

#### script_list

```json
{ "name": "script_list", "arguments": {} }
```

Returns: list of scripts with name, description, declared secrets, and approval status.

#### script_run

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Script name (without `.star`) |
| `function` | string | No | Specific function to call |
| `args` | array | No | Function arguments |

```json
{
  "name": "script_run",
  "arguments": { "name": "weather", "function": "get_weather", "args": ["London"] }
}
```

Approval is required before first run unless the script is in `admin.allowlist`.

#### script_exec

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `code` | string | Yes | Starlark source to execute |

```json
{
  "name": "script_exec",
  "arguments": { "code": "result = 2 + 2\nprint(result)" }
}
```

#### script_reload

```json
{ "name": "script_reload", "arguments": {} }
```

Returns: list of reloaded scripts.

### Schedule tools

Tools for managing [scheduled jobs](./scheduling). Schedules run Starlark scripts or AI agent sessions on a cron timer.

#### schedule_list

```json
{ "name": "schedule_list", "arguments": {} }
```

Returns: list of schedules (id, name, type, cron expression, enabled, run_once, output target, last-run info).

#### schedule_create

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Display name |
| `cron_expr` | string | Yes | 5-field cron expression |
| `type` | string | Yes | `"script"` or `"agent"` |
| `script_name` | string | No | Required for `"script"` type (e.g. `"daily_report.star"`) |
| `prompt` | string | No | Required for `"agent"` type |
| `enabled` | boolean | No | Default `true` |
| `output_provider` | string | No | Chat provider for output delivery |
| `output_channel` | string | No | Channel target |
| `run_once` | boolean | No | Auto-disable after first run |

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

#### schedule_update / schedule_delete / schedule_enable / schedule_disable

All take a single `id` parameter (`schedule_update` also takes any of the create-fields to overwrite).

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

## Tool summary

| Tool | Category | Description |
|------|----------|-------------|
| `workspace_read` | Workspace | Read files from `ai-data/` |
| `workspace_write` | Workspace | Write files to `ai-data/` |
| `workspace_list` | Workspace | List `ai-data/` files |
| `memory_read` | Memory | Read memory files |
| `memory_write` | Memory | Write to memory files |
| `chat_send` | Communication | Proactive message via any chat provider |
| `model_list` | Models | List available AI models |
| `model_set_default` | Models | Change the default for new sessions |
| `calendar_read` | Calendar | Read calendar events |
| `vault_read/write/list/search` | Vault | Obsidian vault access |
| `web_fetch` | Web | Fetch HTTP/HTTPS pages |
| `github_list_issues` / `github_create_issue` | GitHub | Issue management |
| `script_list/run/exec/reload` | Scripts | Starlark execution |
| `schedule_list/create/update/delete/enable/disable` | Schedules | Cron schedules |

## Related

- **[Configuration Overview](../configuration/overview)** — what the tools depend on.
- **[Security Overview](../security/overview)** — how the registry boundary works.
- **[Workspace Layout](./workspace)** — what the workspace tools can and can't reach.
