---
title: Architecture
sidebar_position: 2
---

# Architecture Overview

This document describes the internal architecture of OpenPact, providing a high-level overview of the components and their interactions.

## Component Diagram

```
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│   Discord    │   │   Telegram   │   │    Slack     │
│   Client     │   │   Client     │   │   Client     │
└──────┬───────┘   └──────┬───────┘   └──────┬───────┘
       │                  │                  │
       ▼                  ▼                  ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      Chat Provider Interface                          │
│                        (internal/chat)                                │
│  - Unified message/command handling                                   │
│  - Provider: Discord, Telegram, Slack                                 │
└────────────────────────────────────┬─────────────────────────────────┘
                                     │
                                     ▼
┌──────────────────────────────────────────────────────────────────────┐
│                           Orchestrator                                │
│                      (internal/orchestrator)                          │
│  - Request routing                                                    │
│  - Per-channel session management                                     │
│  - Source context injection                                           │
└───────────────┬────────────────────┴────────────────────┬────────────┘
                │                                         │
                ▼                                         ▼
┌───────────────────────────┐           ┌────────────────────────────────┐
│       AI Engine           │           │         MCP Server             │
│   (internal/engine)       │           │       (internal/mcp)           │
│  - OpenCode adapter       │◄─────────►│  - Tool registration           │
│                           │   Tools   │  - Request handling            │
│  - Provider abstraction   │           │  - Response formatting         │
└───────────────────────────┘           └─────────────────┬──────────────┘
                                                          │
                              ┌───────────────────────────┼───────────────────────────┐
                              │                           │                           │
                              ▼                           ▼                           ▼
                ┌─────────────────────┐   ┌─────────────────────┐   ┌─────────────────────┐
                │   Workspace Tools   │   │   Integration Tools │   │   Script Engine     │
                │                     │   │                     │   │  (internal/starlark)│
                │  - workspace_read   │   │  - calendar_read    │   │  - Sandboxed exec   │
                │  - workspace_write  │   │  - vault_*          │   │  - Secret injection │
                │  - workspace_list   │   │  - github_*         │   │  - HTTP functions   │
                │  - memory_read/write│   │  - web_fetch        │   │  - script_run       │
                └─────────────────────┘   └─────────────────────┘   └─────────────────────┘
```

## Core Packages

### cmd/openpact

The main application entry point. Handles:
- CLI command parsing
- Configuration loading
- Service initialization
- Graceful shutdown

### internal/orchestrator

The central coordination layer that:
- Receives messages from all chat providers (Discord, Telegram, Slack)
- Routes requests to the AI engine via session-based messaging
- Manages per-channel sessions (`<DataDir>/channel_sessions.json`) — each provider:channel pair gets its own session
- Injects source context (`[via telegram, channel:X, user:Y]`) into messages
- Manages conversation context (SOUL/USER/MEMORY injection)
- Implements the `admin.SessionAPI` interface for the Admin UI
- Coordinates MCP tool calls

### internal/engine

Provides adapters for different AI backends. Communicates with the [OpenCode server](https://opencode.ai/docs/server/) via REST API.

```go
// Engine interface
type Engine interface {
    Start(ctx context.Context) error   // Start opencode serve as child process
    Stop() error                        // Gracefully stop the server
    Send(ctx context.Context, sessionID string, messages []Message) (<-chan Response, error)
    SetSystemPrompt(prompt string)

    // Session management (proxied to OpenCode server)
    CreateSession() (*Session, error)
    ListSessions() ([]Session, error)
    GetSession(id string) (*Session, error)
    DeleteSession(id string) error
    AbortSession(id string) error
    GetMessages(sessionID string, limit int) ([]MessageInfo, error)
}
```

The engine spawns `opencode serve --port <port>` as a persistent child process and communicates via HTTP. OpenCode manages all session storage (SQLite) internally.

Implementations:
- **OpenCode**: Supports 75+ LLM providers

### internal/mcp

Model Context Protocol server implementation:

```go
// Tool interface
type Tool interface {
    Name() string
    Description() string
    Schema() Schema
    Execute(ctx context.Context, params map[string]any) (any, error)
}
```

Built-in tool categories:
- **Workspace**: File read/write operations
- **Memory**: Persistent memory management
- **Chat**: Unified messaging across all providers (`chat_send`)
- **Calendar**: Google Calendar integration
- **Vault**: Obsidian vault access
- **GitHub**: Issue and PR management
- **Web**: HTTP fetching
- **Script**: Starlark script execution

### internal/starlark

Sandboxed script execution engine:

- **Loader**: Script discovery and parsing
- **Sandbox**: Secure execution environment
- **Secrets**: Secret injection and redaction

Security features:
- No filesystem access
- HTTP-only networking
- Execution timeouts
- Automatic secret redaction

### internal/config

Configuration management:

```go
type Config struct {
    WorkspacePath string
    MemoryFile    string
    SoulFile      string
    UserFile      string
    Engine        EngineConfig
    MCP           MCPConfig
    Server        ServerConfig
    Logging       LogConfig
}
```

Supports:
- YAML configuration files
- Environment variable overrides
- Secure secret handling

### internal/chat

Defines the generic `chat.Provider` interface that all messaging platforms implement. Includes `MessageHandler` and `CommandHandler` callback types.

### internal/discord

Discord bot integration (implements `chat.Provider`):

- WebSocket connection management
- Message event handling
- Slash commands (`/new`, `/sessions`, `/switch`) for session management
- Channel and user allowlisting

### internal/telegram

Telegram bot integration (implements `chat.Provider`):

- Long-polling update handling (no webhook needed)
- Native `/command` support
- 4096-character message splitting
- User allowlisting by ID or username

### internal/slack

Slack bot integration (implements `chat.Provider`):

- Socket Mode connection (no public URL needed)
- Events API message handling
- Slash commands (`/openpact-new`, `/openpact-sessions`, `/openpact-switch`)
- User and channel allowlisting

### internal/admin

Admin UI backend:

- JWT authentication
- Auth session management (login/logout/refresh)
- AI session management (create/list/switch/delete/chat via WebSocket)
- Script approval workflow
- Secrets management API

### internal/health

Health check endpoints:

- `/health` - Liveness check
- `/ready` - Readiness check
- `/metrics` - Prometheus metrics

### internal/logging

Structured logging:

- JSON and text formats
- Configurable log levels
- Request ID tracking

### internal/ratelimit

Request rate limiting:

- Token bucket algorithm
- Per-user limits
- Configurable thresholds

## Data Flow

### Message Processing

1. **Receive**: Chat provider (Discord, Telegram, or Slack) receives user message or command
2. **Session**: Orchestrator gets (or creates) the per-channel session for this provider:channel pair
3. **Context**: Load SOUL, USER, and MEMORY files as system prompt; prepend source context (`[via telegram, channel:X, user:Y]`)
4. **Send**: `POST /session/:id/message` to the OpenCode server with the enriched message
5. **Process**: OpenCode routes to the configured AI provider, which generates a response
6. **Stream**: Response is streamed back through the response channel
7. **Respond**: Send response back through the originating chat provider (or Admin UI WebSocket)

### Tool Execution

1. **Request**: AI requests tool via MCP protocol
2. **Validate**: Server validates tool name and parameters
3. **Authorize**: Check tool is in allowed list
4. **Execute**: Run tool implementation
5. **Redact**: Scan output for secrets
6. **Return**: Send result to AI

### Script Execution

1. **Load**: Script loaded from scripts directory
2. **Parse**: Starlark parser validates syntax
3. **Inject**: Secrets injected into environment
4. **Execute**: Run in sandboxed interpreter
5. **Redact**: Remove secrets from output
6. **Return**: Return sanitized result

## Security Architecture

### Principle of Least Privilege

```
┌────────────────────────────────────────────────────┐
│                   AI Model                          │
│  - No direct network access                         │
│  - No filesystem access                             │
│  - Only sees tool results                           │
└─────────────────────────┬──────────────────────────┘
                          │ MCP Protocol
                          ▼
┌────────────────────────────────────────────────────┐
│                  OpenPact MCP                       │
│  - Tool allowlisting                                │
│  - Secret redaction                                 │
│  - Rate limiting                                    │
└─────────────────────────┬──────────────────────────┘
                          │ Internal Calls
                          ▼
┌────────────────────────────────────────────────────┐
│                 Tool Implementations                │
│  - Sandboxed execution                              │
│  - Scoped permissions                               │
│  - Audit logging                                    │
└────────────────────────────────────────────────────┘
```

### Secret Handling

1. Secrets stored outside workspace
2. Never passed to AI directly
3. Injected into scripts at runtime
4. Automatically redacted from all outputs

### Docker Security Model

Uses two-user privilege separation:
- **Root user**: Initial container setup only
- **App user**: Runtime execution with minimal privileges

## Extension Points

### Adding New Tools

1. Implement the `Tool` interface
2. Register in `internal/mcp/tools.go`
3. Add to configuration allowlist

### Adding New Engines

1. Implement the `Engine` interface
2. Add adapter in `internal/engine/`
3. Register in engine factory

### Adding Starlark Built-ins

1. Add function in `internal/starlark/sandbox.go`
2. Register in built-in module
3. Document in Starlark reference
