---
sidebar_position: 5
title: Schedule Management
description: Creating, editing, and managing scheduled jobs in the Admin UI
---

# Schedule Management

The Admin UI provides an interface for managing scheduled jobs — recurring tasks that run Starlark scripts or start AI agent sessions on a cron schedule. This page covers how to create, edit, enable, run, and delete schedules.

## Overview

Scheduled jobs in OpenPact can:

- **Run Starlark scripts** on a timer (e.g., fetch data every hour)
- **Start AI agent sessions** with a prompt (e.g., daily summary at 9 AM)
- **Deliver output** to a chat channel (Discord, Telegram, Slack)

Schedules persist to disk and survive restarts.

## Schedules UI

Navigate to `/schedules` in the Admin UI to manage schedules.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Schedules                                          [New Schedule]  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ Name            │ Type   │ Schedule    │ Enabled │ Last Run   │  │
│  ├───────────────────────────────────────────────────────────────┤  │
│  │ Daily report    │ script │ 0 9 * * 1-5 │ Active  │ ✓ Mar 1   │  │
│  │ Status check    │ agent  │ */30 * * * * │ Active  │ ✓ Mar 1   │  │
│  │ Weekly cleanup  │ script │ 0 0 * * 0   │ Disabled│ —         │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

The table shows:

- **Name** — Human-readable schedule name
- **Type** — `script` or `agent`
- **Schedule** — Cron expression displayed as code
- **Enabled** — Active (green) or Disabled (grey)
- **Last Run** — Status tag (success/error) with timestamp, or `—` if never run
- **Actions** — Edit, Enable/Disable, Run Now, Delete

## Creating a Schedule

1. Click **New Schedule**
2. Fill in the form:

| Field | Description |
|-------|-------------|
| **Name** | A descriptive name (e.g., "Daily report") |
| **Type** | `Script` (runs a Starlark script) or `Agent` (starts an AI session) |
| **Cron Expression** | Standard 5-field cron (e.g., `0 9 * * 1-5` for weekdays at 9 AM) |
| **Script Name** | *(Script type only)* Filename of the script (e.g., `daily_report.star`) |
| **Prompt** | *(Agent type only)* The prompt to send to the AI session |
| **Enabled** | Whether the schedule starts active |
| **Run Once** | If enabled, the schedule auto-disables after one execution |
| **Output Provider** | *(Optional)* Chat provider for output delivery (e.g., `discord`) |
| **Output Channel** | *(Optional)* Channel ID for output delivery (e.g., `channel:123456`) |

3. Click **Create**

The schedule is immediately registered with the cron runner if enabled.

### Cron Expression Quick Reference

```
┌───────────── minute (0–59)
│ ┌───────────── hour (0–23)
│ │ ┌───────────── day of month (1–31)
│ │ │ ┌───────────── month (1–12)
│ │ │ │ ┌───────────── day of week (0–6, Sun=0)
│ │ │ │ │
* * * * *
```

| Expression | Meaning |
|-----------|---------|
| `*/5 * * * *` | Every 5 minutes |
| `0 9 * * 1-5` | Weekdays at 9:00 AM |
| `0 0 1 * *` | First of every month at midnight |
| `30 14 * * *` | Every day at 2:30 PM |

Invalid cron expressions are rejected with an error message when you save.

## Editing a Schedule

1. Click **Edit** on the schedule row
2. Modify the fields you want to change
3. Click **Save**

The scheduler automatically reloads to pick up changes.

:::note
Editing a schedule does not reset its last run information.
:::

## Run-Once Schedules

Toggle **Run Once** in the create/edit form to create a one-off job. After the job runs (success or error), the scheduler automatically disables it. This is useful for deferred tasks like "run this script at 3 AM tonight".

The schedule is preserved (not deleted) after running, so you can view its results and re-enable it if needed. Run-once schedules are marked with an "Once" tag in the table.

## Enabling and Disabling

Click **Enable** or **Disable** on a schedule to toggle it:

- **Disabling** removes the schedule from the cron runner but preserves all configuration and history
- **Enabling** re-registers the schedule with the cron runner

This is useful for temporarily pausing a job without losing its setup.

## Running Immediately

Click **Run Now** to trigger a schedule immediately, regardless of its cron timing. The job runs in a background goroutine:

- The schedule does not need to be enabled to run manually
- Last run information is updated with the result
- Output is delivered to the configured target (if set)

## Deleting a Schedule

1. Click **Delete** on the schedule row
2. Confirm in the dialog

:::warning
Deleting a schedule is permanent and cannot be undone. The schedule is removed from both the store and the cron runner.
:::

## Viewing Results

The **Last Run** column shows the result of the most recent execution:

- **Success** (green tag) — Job completed without errors
- **Error** (red tag) — Job failed; hover or check the API for the error message

For detailed output (up to 2000 characters), use the [Admin API](/docs/api/admin-api#get-apischedules-1) to fetch the full schedule object, which includes `last_run_output` and `last_run_error`.

## Output Delivery

When **Output Provider** and **Output Channel** are both set, the scheduler sends job results to the specified chat channel after each run. The message includes:

- Job name
- Status (success/error)
- Output (truncated to 1800 characters for chat)

To stop output delivery, edit the schedule and clear both output fields.

## Related Documentation

- [Scheduling Overview](/docs/features/scheduling) — Feature overview, job types, architecture
- [Managing Scripts](/docs/admin/managing-scripts) — Script approval (required for script-type jobs)
- [Secrets Management](/docs/admin/secrets-management) — Secrets used by scripts
