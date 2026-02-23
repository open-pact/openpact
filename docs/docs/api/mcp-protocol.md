---
sidebar_position: 2
title: MCP Protocol
description: Model Context Protocol specification and tool communication
---

# MCP Protocol

OpenPact uses the **Model Context Protocol (MCP)** to communicate between AI engines and the tool server. This page provides an overview of the protocol and how tools are registered and invoked.

## What is MCP?

MCP (Model Context Protocol) is a JSON-RPC 2.0 based protocol for communication between AI models and tool servers. It defines a standard way for:

- **Tool Discovery**: AI models can query available tools
- **Tool Invocation**: AI models can call tools with structured arguments
- **Result Handling**: Tools return structured results to the AI

## Protocol Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        AI Engine                                 │
│                       (OpenCode)                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ JSON-RPC 2.0 over stdio
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      OpenPact MCP Server                         │
│                        (Port 3000)                               │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────────┐ │
│  │  Tool Router │ │ Tool Registry│ │   Request Handler        │ │
│  └──────────────┘ └──────────────┘ └──────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Tool Implementations                        │
│  workspace_read, memory_write, script_run, chat_send, etc.      │
└─────────────────────────────────────────────────────────────────┘
```

## JSON-RPC 2.0 Format

All MCP communication follows the JSON-RPC 2.0 specification.

### Request Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "workspace_read",
    "arguments": {
      "path": "/workspace/notes.md"
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `jsonrpc` | string | Always `"2.0"` |
| `id` | number/string | Request identifier for response matching |
| `method` | string | MCP method being called |
| `params` | object | Method-specific parameters |

### Response Format (Success)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "File contents here..."
      }
    ]
  }
}
```

### Response Format (Error)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params: file not found"
  }
}
```

| Error Code | Meaning |
|------------|---------|
| `-32700` | Parse error |
| `-32600` | Invalid request |
| `-32601` | Method not found |
| `-32602` | Invalid params |
| `-32603` | Internal error |

## MCP Methods

### tools/list

Lists all available tools with their schemas.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list"
}
```

**Response:**

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
            "path": {
              "type": "string",
              "description": "Path to the file relative to workspace root"
            }
          },
          "required": ["path"]
        }
      },
      {
        "name": "script_run",
        "description": "Execute a Starlark script",
        "inputSchema": {
          "type": "object",
          "properties": {
            "name": {
              "type": "string",
              "description": "Name of the script (without .star extension)"
            },
            "function": {
              "type": "string",
              "description": "Optional function name to call"
            },
            "args": {
              "type": "array",
              "description": "Optional arguments to pass to the function"
            }
          },
          "required": ["name"]
        }
      }
    ]
  }
}
```

### tools/call

Invokes a specific tool with arguments.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "script_run",
    "arguments": {
      "name": "weather",
      "function": "get_weather",
      "args": ["London"]
    }
  }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"city\": \"London\", \"temp_c\": 15.5, \"condition\": \"Cloudy\"}"
      }
    ],
    "isError": false
  }
}
```

### Error Response Example

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Error: script 'unknown' not found"
      }
    ],
    "isError": true
  }
}
```

## Tool Registration

Tools are registered with the MCP server during initialization. Each tool provides:

1. **Name**: Unique identifier for the tool
2. **Description**: Human-readable description
3. **Input Schema**: JSON Schema defining valid arguments
4. **Handler**: Function that executes the tool

### Tool Definition Structure

```go
type Tool struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
    Type       string              `json:"type"`
    Properties map[string]Property `json:"properties"`
    Required   []string            `json:"required,omitempty"`
}

type Property struct {
    Type        string `json:"type"`
    Description string `json:"description"`
}
```

### Built-in Tools

OpenPact registers the following tools:

| Category | Tool | Description |
|----------|------|-------------|
| **Workspace** | `workspace_read` | Read files from workspace |
| | `workspace_write` | Write files to workspace |
| | `workspace_list` | List files in workspace |
| **Memory** | `memory_read` | Read memory files |
| | `memory_write` | Write to memory |
| **Vault** | `vault_read` | Read Obsidian notes |
| | `vault_write` | Write Obsidian notes |
| | `vault_list` | List vault files |
| | `vault_search` | Search vault content |
| **Chat** | `chat_send` | Send messages via any chat provider |
| **Calendar** | `calendar_read` | Read calendar events |
| **GitHub** | `github_list_issues` | List repository issues |
| | `github_create_issue` | Create new issues |
| **Web** | `web_fetch` | Fetch web pages |
| **Scripts** | `script_list` | List available scripts |
| | `script_run` | Run a named script |
| | `script_exec` | Execute arbitrary Starlark |
| | `script_reload` | Reload scripts from disk |

## Script Approval Integration

When `script_run` or `script_exec` is called, the MCP server checks approval status:

```
┌─────────────────────────────────────────────────────────────────┐
│                     MCP Server receives                          │
│                     tools/call: script_run                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │ Is script in    │
                    │ config allowlist?│
                    └─────────────────┘
                        │         │
                       YES       NO
                        │         │
                        ▼         ▼
                   ┌────────┐  ┌──────────────────┐
                   │Execute │  │Check approval    │
                   │directly│  │status from Admin │
                   └────────┘  │API               │
                               └──────────────────┘
                                   │          │
                              APPROVED    NOT APPROVED
                                   │          │
                                   ▼          ▼
                              ┌────────┐  ┌────────────┐
                              │Execute │  │Return error│
                              │script  │  │message     │
                              └────────┘  └────────────┘
```

### Unapproved Script Response

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Error: Script 'new_feature.star' is pending approval. An administrator must review and approve this script before it can be executed."
      }
    ],
    "isError": true
  }
}
```

## Connection Modes

### stdio Mode (Default)

The MCP server communicates over standard input/output. This is the default mode used by OpenCode.

```yaml
# OpenCode MCP config
mcp:
  servers:
    openpact:
      command: "./openpact"
      args: ["--mcp"]
```

### HTTP Mode (Future)

HTTP-based transport for remote MCP servers (planned for future releases).

## Rate Limiting

MCP requests are subject to rate limiting to prevent abuse:

| Limit Type | Default | Description |
|------------|---------|-------------|
| Requests/second | 10 | Maximum sustained request rate |
| Burst | 20 | Maximum burst size |

Rate limit exceeded response:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "error": {
    "code": -32000,
    "message": "Rate limit exceeded. Please try again later."
  }
}
```

## Security Considerations

### Tool Access Control

- Tools only expose explicitly configured capabilities
- Workspace tools are restricted to configured paths
- Scripts must be approved before execution
- Secrets are never exposed in tool responses

### Secret Redaction

All tool responses are scanned for secret values. If a secret value appears in the output, it is replaced:

```json
{
  "result": {
    "content": [
      {
        "type": "text",
        "text": "API response: {\"key\": \"[REDACTED:API_KEY]\"}"
      }
    ]
  }
}
```

### Path Traversal Prevention

Workspace and vault tools validate paths to prevent directory traversal:

```
✓ Allowed: /workspace/notes.md
✓ Allowed: /workspace/subdir/file.txt
✗ Blocked: /workspace/../etc/passwd
✗ Blocked: /etc/passwd
```

## Debugging MCP Communication

### Enable Debug Logging

```yaml
logging:
  level: debug
```

### Example Debug Output

```
DEBUG mcp: received request method=tools/list id=1
DEBUG mcp: sending response id=1 tools=16
DEBUG mcp: received request method=tools/call id=2 tool=workspace_read
DEBUG mcp: tool execution completed id=2 duration=5ms success=true
```

### Testing with curl (HTTP mode)

When HTTP mode is enabled (future):

```bash
# List tools
curl -X POST http://localhost:3000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Call a tool
curl -X POST http://localhost:3000/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":2,
    "method":"tools/call",
    "params":{
      "name":"workspace_read",
      "arguments":{"path":"/workspace/test.md"}
    }
  }'
```

## Protocol Extensions

OpenPact extends the base MCP specification with:

1. **Script approval status** in error responses
2. **Secret redaction** in all responses
3. **Rate limiting** with informative error codes
4. **Execution metrics** available via `/metrics` endpoint

## Related Documentation

- [MCP Tools Reference](/docs/features/mcp-tools) - Detailed documentation for each tool
- [Script Sandboxing](/docs/security/script-sandboxing) - Script execution security
- [Secret Handling](/docs/security/secret-handling) - How secrets are protected
