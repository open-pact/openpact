---
sidebar_position: 2
title: MCP Protocol
description: How OpenPact registers tools and exposes them to the in-process LLM agent
---

# MCP Protocol

OpenPact uses the **Model Context Protocol (MCP)** as a registration model — tool definitions, schemas, and handlers — and surfaces those tools to the in-process LLM agent via [stackllm](https://github.com/stack-bound/stackllm). Because the agent runs inside the OpenPact binary, there is no network or stdio transport between OpenPact and the agent at runtime: tool calls are Go function calls.

For external clients that want to talk MCP-over-stdio (e.g. Claude Desktop, another LLM, an inspector tool), OpenPact ships a separate `mcp-server` binary alongside the main one.

## Runtime Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     OpenPact process                         │
│  ┌───────────────────────────┐                              │
│  │   stackllm agent.Agent    │                              │
│  │   (in-process)            │                              │
│  └─────────────┬─────────────┘                              │
│                │ Go function call                            │
│                ▼                                             │
│  ┌───────────────────────────┐                              │
│  │  stackllm tools.Registry   │  ← registered at boot       │
│  │  (with mcpToolAdapter)     │     by engine.Register-     │
│  └─────────────┬─────────────┘     MCPTools                 │
│                │                                             │
│                ▼                                             │
│  ┌───────────────────────────┐                              │
│  │   internal/mcp tool funcs  │                              │
│  │   workspace_*, script_*,   │                              │
│  │   memory_*, web_fetch, ... │                              │
│  └───────────────────────────┘                              │
└─────────────────────────────────────────────────────────────┘
```

`engine.RegisterMCPTools` walks `mcp.Server.ListTools()` at boot and registers each one in `tools.Registry` via an adapter (`mcpToolAdapter.Call`) that forwards stackllm's JSON-args calling convention to MCP's `(ctx, argsMap)` shape. The MCP `InputSchema` (a JSON-Schema map) passes through verbatim — stackllm consumes the same shape.

## Tool definition

Tools are defined in Go code in `internal/mcp/`. Each tool provides:

```go
type Tool struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    InputSchema InputSchema    `json:"inputSchema"` // JSON Schema
    Handler     ToolHandler    // func(ctx, argsMap) (any, error)
}

type InputSchema struct {
    Type       string              `json:"type"` // "object"
    Properties map[string]Property `json:"properties"`
    Required   []string            `json:"required,omitempty"`
}

type Property struct {
    Type        string `json:"type"`
    Description string `json:"description"`
}
```

Tools are registered in `internal/mcp/register.go` via `RegisterAllTools(srv, cfg)`. The registry is closed once `engine.RegisterMCPTools` has copied the catalogue into stackllm; runtime additions are not supported.

## Built-in Tools

| Category | Tool | Description |
|----------|------|-------------|
| **Workspace** | `workspace_read`, `workspace_write`, `workspace_list` | Files under `ai-data/` |
| **Memory** | `memory_read`, `memory_write` | SOUL/USER/MEMORY + daily files |
| **Vault** | `vault_read`, `vault_write`, `vault_list`, `vault_search` | Obsidian vault (configured at `/integrations`) |
| **Chat** | `chat_send` | Proactive message via Discord/Telegram/Slack |
| **Models** | `model_list`, `model_set_default` | View/change the default model |
| **Calendar** | `calendar_read` | iCal feeds (configured at `/integrations`) |
| **GitHub** | `github_list_issues`, `github_create_issue` | Issue management |
| **Web** | `web_fetch` | HTTP/HTTPS GET, response capped |
| **Scripts** | `script_list`, `script_run`, `script_exec`, `script_reload` | Starlark execution |
| **Schedules** | `schedule_list`, `schedule_create`, `schedule_update`, `schedule_delete`, `schedule_enable`, `schedule_disable` | Cron job management |

See [MCP Tools Reference](/docs/features/mcp-tools) for full per-tool schemas.

## Script approval integration

When `script_run` or `script_exec` is invoked, the script tool checks approval status before executing:

```
Tool call: script_run / script_exec
        ↓
   Is script in admin.allowlist?
        │
       YES → Execute directly
        │
       NO  → Check op_approvals
                  │
            APPROVED → Execute (sandboxed)
                  │
            NOT APPROVED → Return error to agent
```

The "not approved" response is a normal tool result with an error message, so the agent can react gracefully:

```text
Error: Script 'new_feature.star' is pending approval. An administrator must
review and approve this script before it can be executed.
```

Approval is granted by an admin via the admin UI (`/scripts/<name>/approve`).

## External MCP clients

OpenPact ships a standalone stdio MCP server at `/app/mcp-server` (in the Docker image) or `./mcp-server` (in source builds). This one **does** speak JSON-RPC 2.0 over stdio, so external clients can call OpenPact tools directly.

### Wire format (stdio JSON-RPC 2.0)

#### Request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "workspace_read",
    "arguments": { "path": "notes/todo.md" }
  }
}
```

#### Response (success)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      { "type": "text", "text": "File contents here..." }
    ]
  }
}
```

#### Response (error)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": { "code": -32602, "message": "Invalid params: file not found" }
}
```

#### Standard JSON-RPC error codes

| Code | Meaning |
|------|---------|
| `-32700` | Parse error |
| `-32600` | Invalid request |
| `-32601` | Method not found |
| `-32602` | Invalid params |
| `-32603` | Internal error |

### MCP methods

#### `tools/list`

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list"
}
```

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "workspace_read",
        "description": "Read a file from the workspace",
        "inputSchema": {
          "type": "object",
          "properties": {
            "path": { "type": "string", "description": "Relative path within ai-data/" }
          },
          "required": ["path"]
        }
      }
    ]
  }
}
```

#### `tools/call`

See the Request/Response examples above.

### Tool errors

A tool that fails returns a normal `result` with `isError: true`:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      { "type": "text", "text": "Error: script 'unknown' not found" }
    ],
    "isError": true
  }
}
```

### Wiring an external client

The external client launches the stdio binary directly:

```bash
# OPENPACT_WORKSPACE_PATH points at the workspace whose tools you want to use.
# OPENPACT_FEATURES is an optional comma-separated allowlist (default: all).
OPENPACT_WORKSPACE_PATH=/workspace \
OPENPACT_FEATURES=scripts,workspace,memory \
/app/mcp-server
```

The stdio server uses the same `internal/mcp/` package as the in-process registry, so the tool surface is identical (modulo features that need orchestrator state — chat, models, schedules).

## Security considerations

### Tool surface is closed

The agent (in-process) and any external stdio client can only call tools that were registered at boot. There is no runtime path to add a tool — that requires a binary rebuild.

### Path validation

Workspace and vault tools validate every path:

```
✓ Allowed: ai-data/notes.md
✓ Allowed: ai-data/sub/file.txt
✗ Blocked: ../../../etc/passwd
✗ Blocked: /etc/passwd
✗ Blocked: secure/config.yaml   (outside ai-data/)
```

### Secret redaction

Starlark scripts have access to secrets via `secrets.get("KEY")`, but output is scanned and replaced before reaching the agent:

```json
{
  "content": [
    { "type": "text", "text": "API response: {\"key\": \"[REDACTED:API_KEY]\"}" }
  ]
}
```

The agent (and any tool consuming the result) sees `[REDACTED:API_KEY]`, never the raw value.

## Rate limiting

The token-bucket rate limiter applies to inbound HTTP requests on the public surfaces (`/api/*`); it doesn't apply to in-process tool calls. The standalone stdio MCP server has no rate limiter — it's intended for trusted local use.

## Debugging

### Logging

```sh
# /settings/advanced → logging level → debug
```

Example debug output:

```
DEBUG engine: registered 21 tools from MCP catalogue
DEBUG agent: tool_call name=workspace_read args={"path":"notes/todo.md"}
DEBUG agent: tool_result name=workspace_read duration=5ms success=true
```

### Inspecting registered tools

The mcp-server `--list-tools` flag prints the registered set in JSON:

```bash
/app/mcp-server --list-tools
```

Or grep `internal/mcp/register.go` in source.

## Protocol Extensions

Beyond base MCP/JSON-RPC, OpenPact contributes:

1. **Script approval status** — `script_run` returns a structured "pending approval" error.
2. **Secret redaction** — applied to every tool result before returning.
3. **In-process invocation** — at runtime there is no JSON-RPC overhead; the agent calls the tool function directly via the stackllm adapter.

## Related Documentation

- [MCP Tools Reference](/docs/features/mcp-tools) — detailed per-tool schemas.
- [Script Sandboxing](/docs/security/script-sandboxing) — Starlark execution security.
- [Secret Handling](/docs/security/secret-handling) — how secrets are protected.
