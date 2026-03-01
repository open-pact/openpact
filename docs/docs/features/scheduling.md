---
title: Scheduling
sidebar_position: 9
---

# Scheduling

OpenPact includes a cron-based job scheduling system that lets you create, manage, and run recurring jobs. Schedules can execute Starlark scripts or start AI agent sessions on a timer, with optional output delivery to a chat channel.

The scheduler uses [`robfig/cron/v3`](https://github.com/robfig/cron) internally — not system cron — so it works identically across all platforms including Docker.

## Run-Once Schedules

Schedules can be configured as **run-once** (one-off) jobs by setting `run_once: true`. When a run-once schedule completes execution (whether success or error), the scheduler automatically disables it. This is useful for deferred tasks like "run this script at 3 AM tonight" or "send a summary on Friday at 5 PM".

The schedule is not deleted — only disabled — so you can still see its last run result and re-enable it if needed.

## Job Types

### Script Jobs

Script jobs run a [Starlark script](/docs/admin/managing-scripts) by name. The scheduler:

1. Checks the script's approval status (must be approved)
2. Loads the script via the Starlark Loader
3. Executes it in the sandbox with a **5-minute timeout**
4. Sanitizes the output to redact any secrets

```json
{
  "type": "script",
  "script_name": "daily_report.star"
}
```

### Agent Jobs

Agent jobs create a new AI session and send it a prompt. The scheduler:

1. Creates a new session via the engine API
2. Sends the prompt as a user message
3. Collects the streamed response with a **10-minute timeout**

```json
{
  "type": "agent",
  "prompt": "Summarize today's open GitHub issues and post a status update."
}
```

## Output Targets

Jobs can optionally send their output to a chat channel (Discord, Telegram, Slack) via the existing chat provider plumbing. Configure this per-schedule with an `output_target`:

```json
{
  "output_target": {
    "provider": "discord",
    "channel_id": "channel:123456789"
  }
}
```

When an output target is set, the scheduler formats the result as a message including the job name, status, and output (truncated to 1800 characters for chat delivery), then sends it via the specified provider.

If no output target is set, the job still runs and its result is stored — you can view it in the Admin UI or via the API.

## Cron Expression Format

Schedules use standard 5-field cron expressions:

```
┌───────────── minute (0–59)
│ ┌───────────── hour (0–23)
│ │ ┌───────────── day of month (1–31)
│ │ │ ┌───────────── month (1–12)
│ │ │ │ ┌───────────── day of week (0–6, Sun=0)
│ │ │ │ │
* * * * *
```

### Common Examples

| Expression | Description |
|-----------|-------------|
| `*/5 * * * *` | Every 5 minutes |
| `0 9 * * 1-5` | Weekdays at 9:00 AM |
| `0 0 1 * *` | First of every month at midnight |
| `30 14 * * *` | Every day at 2:30 PM |
| `0 */2 * * *` | Every 2 hours |
| `0 8 * * 1` | Every Monday at 8:00 AM |

Cron expressions are validated at creation and update time. Invalid expressions are rejected with an error message.

## Creating Schedules

### Via MCP Tools (AI Agent)

The AI agent can manage schedules through 6 [MCP tools](/docs/features/mcp-tools#schedule-tools):

```json
{
  "name": "schedule_create",
  "arguments": {
    "name": "Daily report",
    "cron_expr": "0 9 * * 1-5",
    "type": "script",
    "script_name": "daily_report.star",
    "output_provider": "discord",
    "output_channel": "channel:123456789"
  }
}
```

**Run-once example** — run a script at 3 AM tonight and auto-disable:

```json
{
  "name": "schedule_create",
  "arguments": {
    "name": "One-time migration",
    "cron_expr": "0 3 * * *",
    "type": "script",
    "script_name": "migrate.star",
    "run_once": true
  }
}
```

### Via Admin UI

Navigate to the **Schedules** page in the Admin UI to create, edit, enable/disable, and delete schedules through a visual interface. See [Schedule Management](/docs/admin/schedule-management) for a full walkthrough.

### Via Admin API

Use the [Schedule REST endpoints](/docs/api/admin-api#schedule-endpoints) for programmatic access:

```bash
curl -X POST http://localhost:8080/api/schedules \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Daily report",
    "cron_expr": "0 9 * * 1-5",
    "type": "script",
    "script_name": "daily_report.star",
    "enabled": true
  }'
```

## Persistence

Schedules persist to `secure/data/schedules.json` (file-based JSON, same pattern as secrets and providers). They survive restarts — the scheduler reloads all enabled schedules on startup.

## Last Run Tracking

Each schedule records its most recent execution:

- **last_run_at** — Timestamp of the last run
- **last_run_status** — `"success"` or `"error"`
- **last_run_error** — Error message (if status is `"error"`)
- **last_run_output** — Truncated output (max 2000 characters)

This information is visible in the Admin UI, API responses, and MCP tool results.

## Architecture

```
AI Agent ──── MCP Tools (schedule_*) ──→ Orchestrator ──→ ScheduleStore (JSON file)
                                              │                    ↑
Admin UI ──── REST API (/api/schedules) ──→ Handlers ─────────────┘
                                              │
                                              ↓
                                         Scheduler (robfig/cron)
                                           ├── script: Sandbox → Loader → SanitizeResult
                                           └── agent:  Engine.CreateSession → Engine.Send
                                                          │
                                                          ↓
                                                    OutputTarget → ChatAPI.SendViaProvider
```

- The **Orchestrator** implements both the MCP interface (for AI tools) and the Admin API interface (for HTTP handlers)
- The **Scheduler** receives engine and chat APIs via setters (lazy wiring)
- The **Store** is shared between the scheduler, MCP tools, and admin handlers
- All mutations go through the store, then trigger `Reload()` to sync cron entries

## Related Documentation

- [Schedule Management](/docs/admin/schedule-management) — Admin UI guide
- [MCP Tools Reference](/docs/features/mcp-tools#schedule-tools) — Schedule MCP tools
- [Admin API](/docs/api/admin-api#schedule-endpoints) — Schedule REST endpoints
- [Managing Scripts](/docs/admin/managing-scripts) — Script approval workflow
