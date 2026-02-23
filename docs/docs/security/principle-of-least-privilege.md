---
sidebar_position: 2
title: Principle of Least Privilege
description: Tool access, MCP restrictions, and environment isolation
---

# Principle of Least Privilege

OpenPact implements the principle of least privilege throughout its architecture. Every component receives only the minimum permissions necessary to perform its function.

## What Is Least Privilege?

The principle of least privilege (PoLP) states that every component, user, or process should have only the access rights necessary to perform its legitimate purpose. This limits the damage that can result from accidents, errors, or unauthorized use.

## Two-Layer Security Model

OpenPact enforces least privilege through two independent layers:

### Layer 1: Linux User Separation

The AI process (`opencode serve`) runs as `openpact-ai`, a restricted Linux user, while the orchestrator runs as `openpact-system`. File permissions enforce boundaries:

| Path | Owner | Mode | AI Access |
|------|-------|------|-----------|
| `/workspace/` | openpact-system:openpact | 750 | Group-read/execute (can list/read) |
| `/workspace/data/` | openpact-system:openpact | 700 | **Denied** (owner-only) |
| `/workspace/memory/` | openpact-system:openpact | 770 | Group-read/write |
| `/workspace/scripts/` | openpact-system:openpact | 750 | Group-read |
| `/workspace/config.yaml` | openpact-system:openpact | 600 | **Denied** (owner-only) |
| `/workspace/SOUL.md` | openpact-system:openpact | 640 | Group-read |
| `/workspace/USER.md` | openpact-system:openpact | 640 | Group-read |
| `/workspace/MEMORY.md` | openpact-system:openpact | 660 | Group-read/write |

### Layer 2: Application Tool Restriction

OpenCode's built-in tools (bash, write, edit, read, grep, glob, list, patch, webfetch, websearch) are all disabled via the `OPENCODE_CONFIG_CONTENT` environment variable. The AI can only use explicitly registered MCP tools provided by OpenPact's standalone MCP server.

## AI Access Restrictions

### What the AI Can Access

| Resource | Access Level | How |
|----------|--------------|-----|
| MCP tools | Defined set only | Registered tools via MCP server |
| Script execution | Approved only | `script_run` MCP tool (requires admin approval) |
| Workspace files | Read via MCP | `workspace_read`, `workspace_list` tools |
| Memory system | Read/write via MCP | `memory_read`, `memory_write` tools |
| Web content | Fetch via MCP | `web_fetch` tool |
| Calendar | Read via MCP | `calendar_read` tool |
| Chat providers | Send via MCP | `chat_send` tool |

### What the AI Cannot Access

- **Shell commands** -- OpenCode's `bash` tool is disabled
- **Direct file writes** -- OpenCode's `write`/`edit`/`patch` tools are disabled
- **Environment variables** -- only LLM provider keys and system basics are passed through
- **Sensitive tokens** -- DISCORD_TOKEN, GITHUB_TOKEN, SLACK_BOT_TOKEN, ADMIN_JWT_SECRET excluded
- **Data directory** -- owner-only permissions (700) block group access
- **Config file** -- owner-only permissions (600)
- **Secret values** -- scripts see secrets but output is redacted with `[REDACTED]`
- **Unapproved scripts** -- `script_run` checks approval status
- **Admin UI operations** -- admin API requires JWT auth

## Environment Variable Isolation

The AI process receives a filtered environment. Only these variables pass through:

**System basics:** `PATH`, `HOME`, `USER`, `LANG`, `TERM`, `TZ`, `TMPDIR`, `XDG_*`

**LLM provider keys:** `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`, `AZURE_OPENAI_API_KEY`, `OLLAMA_HOST`

**Explicitly excluded:** `DISCORD_TOKEN`, `GITHUB_TOKEN`, `SLACK_BOT_TOKEN`, `TELEGRAM_BOT_TOKEN`, `ADMIN_JWT_SECRET`, all Starlark secrets, and any other environment variable not in the allowlist.

When `run_as_user` is configured, `HOME` and `USER` are overridden to point to the AI user's home directory.

## MCP Tool Boundaries

Each MCP tool has explicit capability boundaries:

### File Tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `workspace_read` | Read workspace files | Workspace directory only, path validation |
| `workspace_write` | Write workspace files | Workspace directory only, path validation |
| `workspace_list` | List workspace contents | Workspace directory only |

### Memory Tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `memory_read` | Read MEMORY.md, SOUL.md, USER.md, daily files | Validated paths only |
| `memory_write` | Write memory/context files | Validated paths only, triggers context reload |

### Script Tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `script_run` | Execute approved scripts | Admin approval required, sandboxed |
| `script_exec` | Execute inline Starlark | Sandboxed, no filesystem access |
| `script_list` | View script metadata | Includes approval status |
| `script_reload` | Reload scripts from disk | Admin must place files |

### Communication Tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `chat_send` | Send messages via providers | Provider must be active |
| `web_fetch` | Fetch HTTP/HTTPS URLs | Read-only, size limits |
| `calendar_read` | Read calendar events | Configured feeds only |

## Configuration

### Enabling User Separation

```yaml
# openpact.yaml
engine:
  type: opencode
  run_as_user: "openpact-ai"     # Run AI as restricted user
  mcp_binary: "/app/mcp-server"  # Standalone MCP server binary
```

Set `run_as_user` to empty string for development mode (runs AI as current user).

### Dev Mode vs Production

| Setting | Dev Mode | Production (Docker) |
|---------|----------|---------------------|
| `run_as_user` | `""` (empty) | `"openpact-ai"` |
| `mcp_binary` | `""` (empty) | `"/app/mcp-server"` |
| Built-in tools | Disabled (config still applied) | Disabled |
| File permissions | Host OS permissions | Entrypoint sets strict permissions |

## Monitoring Least Privilege

### Audit Questions

Periodically review:

- Which scripts have access to which secrets?
- Are all approved scripts still needed?
- Are there unused secrets that should be removed?
- Is the workspace directory appropriately scoped?

### Verification

```bash
# Verify AI process runs as correct user
docker exec <container> ps aux | grep opencode
# Should show openpact-ai, not openpact-system

# Verify file permissions
docker exec <container> ls -la /workspace/
docker exec <container> ls -la /workspace/data/
```

## Summary

| Component | Least Privilege Implementation |
|-----------|-------------------------------|
| AI process | Runs as `openpact-ai`, filtered env, disabled built-in tools |
| MCP tools | Explicit registration, path validation, approval workflow |
| Scripts | Starlark sandbox, admin approval, secret redaction |
| Workspace | Scoped directory, group-based permissions |
| Secrets | Owner-only data dir, env filtering, output redaction |
| Container | Non-root users, entrypoint permissions, Docker isolation |
