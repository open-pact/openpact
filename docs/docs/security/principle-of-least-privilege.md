---
sidebar_position: 2
title: Principle of Least Privilege
description: Tool registry, MCP boundaries, and workspace scoping
---

# Principle of Least Privilege

OpenPact implements the principle of least privilege throughout its architecture. Every component receives only the minimum permissions necessary to perform its function.

## What Is Least Privilege?

The principle of least privilege (PoLP) states that every component, user, or process should have only the access rights necessary to perform its legitimate purpose. This limits the damage that can result from accidents, errors, or unauthorized use.

## Three-Layer Security Model

OpenPact enforces least privilege through three independent layers. Each one assumes the layer above it could fail.

### Layer 1: Tool Registry Boundary

The LLM agent is a Go function call, not a separate process. It can only call tools that have been explicitly registered in stackllm's `tools.Registry`. The registry is populated **once**, at boot, by `engine.RegisterMCPTools` walking OpenPact's MCP server catalogue.

This means:

- A tool that wasn't registered cannot be called. There is no shell, no `bash`, no `eval`, no arbitrary file IO — those tools were never registered.
- Adding a new capability to the agent requires writing Go code, registering it via `mcp.RegisterAllTools`, and rebuilding the binary. There is no runtime path to grant new tools.

### Layer 2: In-Tool Authorization

Each registered tool enforces its own scoping in code. The agent can call the tool, but the tool decides what the call does.

| Tool | Scoping rule |
|------|--------------|
| `workspace_*` | Path validated to be inside `<workspace>/ai-data/`. Symlink escape prevented. |
| `memory_*` | Hard-coded paths: `MEMORY.md`, `SOUL.md`, `USER.md`, daily files under `ai-data/memory/`. |
| `script_run` / `script_exec` | Starlark sandbox: no filesystem, no shell, HTTP-only, configurable wall-clock + memory caps. |
| `vault_*` | Constrained to the configured vault path. Git operations only against the configured remote. |
| `web_fetch` | Outbound HTTPS only, response size capped, no `file://`. |
| `github_*` | Scoped to the GitHub PAT's permissions. |
| `calendar_read` | Reads only the iCal feeds configured in `/integrations`. |
| `discord_send` | Posts only to channels enabled for the bot. |

### Layer 3: Filesystem Permissions

The workspace is split into `secure/` (mode `0700`, system-only) and `ai-data/` (mode `0755`, AI-accessible):

| Path | Mode | Description |
|------|------|-------------|
| `<workspace>/` | 0755 | Top-level (browseable). |
| `<workspace>/secure/` | 0700 | System-only. |
| `<workspace>/secure/config.yaml` | 0600 | Bootstrap config. |
| `<workspace>/secure/data/` | 0700 | System data dir. |
| `<workspace>/secure/data/jwt_secret` | 0600 | JWT signing key. |
| `<workspace>/secure/data/data_encryption_key` | 0600 | AES-256 key for `op_secrets`. |
| `<workspace>/secure/data/stackllm_auth.json` | 0600 | Provider tokens. |
| `<workspace>/secure/data/stackllm.db` | 0600 | Shared SQLite (`op_*` + `stackllm_*` tables). |
| `<workspace>/ai-data/` | 0755 | AI-accessible root. |
| `<workspace>/ai-data/memory/` | 0755 | Daily memory files. |
| `<workspace>/ai-data/scripts/` | 0755 | Starlark scripts. |
| `<workspace>/ai-data/skills/` | 0755 | Skills. |
| `<workspace>/ai-data/SOUL.md` etc. | 0644 | Context files. |

This is mostly belt-and-braces protection now — Layer 2 already keeps the agent out of `secure/` because no registered tool exposes those paths. Layer 3 catches the case where a tool implementation has a path-traversal bug: the OS refuses the read because the running user has no permission to enter `secure/`.

## What the Agent Can Access

| Resource | Access | How |
|----------|--------|-----|
| Workspace files | Read/write under `ai-data/` | `workspace_*` tools |
| Memory | Read all, write to MEMORY.md and daily files | `memory_*` tools |
| Web content | HTTP/HTTPS GET, response capped | `web_fetch` |
| Calendar | Read-only, configured feeds only | `calendar_read` |
| Vault | Read/write under vault path; optional git auto-sync | `vault_*` |
| GitHub | Limited by PAT scopes | `github_*` |
| Scripts | Approved scripts in `ai-data/scripts/` | `script_run` |
| Inline Starlark | Sandboxed; no filesystem | `script_exec` |
| Discord | Send to configured channels | `discord_send` |

## What the Agent Cannot Access

- **Shell / `os.exec`** — no such tool is registered.
- **Direct filesystem writes outside `ai-data/`** — workspace tools enforce the boundary.
- **`secure/`** — no tool exposes it; OS permissions back the boundary up.
- **Secret values** — Starlark `secrets.get()` returns the value to the script, but output is scanned and replaced with `[REDACTED:NAME]` before reaching the agent.
- **Provider tokens** — stored in `stackllm_auth.json` (0600), never surfaced to tools.
- **JWT signing key** — stored in `secure/data/jwt_secret` (0600).
- **Process environment** — there is no env-passing path from agent code to tool implementations; tools that need env values read them in Go, never via the agent's prompts.
- **Other operators' admin sessions** — JWT-protected admin API; rate-limited login.

## The MCP Tool Boundaries Table

### File tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `workspace_read` | Read workspace files | `ai-data/` only, path validation. |
| `workspace_write` | Write workspace files | `ai-data/` only, path validation. |
| `workspace_list` | List workspace contents | `ai-data/` only. |

### Memory tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `memory_read` | Read MEMORY.md, SOUL.md, USER.md, daily files | Validated paths only. |
| `memory_write` | Write memory/context files | Validated paths only; reload on next session. |

### Script tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `script_run` | Execute approved scripts | Admin approval required (or in `admin.allowlist`); sandboxed. |
| `script_exec` | Execute inline Starlark | Sandboxed; no filesystem. |
| `script_list` | View script metadata | Includes approval status. |
| `script_reload` | Reload scripts from disk | Admin must place files. |

### Communication tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `discord_send` | Send messages on Discord | Channel must be in the bot's allow list. |
| `web_fetch` | Fetch HTTP/HTTPS URLs | Read-only; size cap. |
| `calendar_read` | Read calendar events | Configured feeds only. |
| `vault_read/write/list/search` | Operate on the configured vault | Optional git auto-sync to configured remote. |
| `github_*` | GitHub issue management | Scoped by PAT permissions. |

## Configuration

There is no toggle for the registry. The set of tools is whatever `mcp.RegisterAllTools` registers at startup; reducing the surface area means editing that function and rebuilding. The registry is the security model.

For runtime restrictions:

- **Calendar feeds, vault path, GitHub toggle** → `/integrations` in the admin UI.
- **Chat-provider allow lists** → `/providers` (Discord/Slack/Telegram allow lists).
- **Starlark limits** → `/settings/advanced` (`max_execution_ms`, `max_memory_mb`).
- **Starlark allowlist** (scripts pre-approved without per-run prompt) → `admin.allowlist` in `config.yaml`.

## Auditing

### Periodic review

- Which scripts are approved? Are all of them still needed?
- Which scripts have access to which secrets?
- Who has admin access to the UI?
- Are the allow lists for Discord/Slack/Telegram still correct?

### Verification

```bash
# Check workspace permissions
docker exec openpact ls -la /workspace
# secure/ should be drwx------ (700)

docker exec openpact ls -la /workspace/secure/data
# stackllm_auth.json should be -rw------- (600)
# stackllm.db should be -rw------- (600)
```

```bash
# Confirm the registered tool set
docker exec openpact /app/mcp-server --list-tools 2>/dev/null
# (or inspect internal/mcp/register.go in the source)
```

## Summary

| Layer | Implementation |
|-------|---------------|
| Tool registry | Static set populated at boot by `mcp.RegisterAllTools`. Unregistered tools cannot be called. |
| In-tool authorization | Each tool enforces its own scoping (workspace boundary, allow lists, sandbox). |
| Filesystem permissions | `secure/` 0700, `secure/data/*` 0600, `ai-data/` 0755. |
| Admin API | JWT auth, rate-limited; setup wizard guards onboarding. |
| Starlark | Sandbox + secret redaction + admin approval workflow. |
