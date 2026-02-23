---
title: Introduction
sidebar_position: 1
---

# Introduction to OpenPact

OpenPact is a secure, minimal framework for running your own AI assistant. Built in Go and designed with security as the top priority, it gives you complete control over what your AI can access while keeping your data private.


<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <img
    src="/img/logo-full.svg"
    alt="OpenPact Logo"
    style={{width: '100%', maxWidth: '350px', height: 'auto'}}
  />
</div>




## What is OpenPact?

OpenPact is an **AI orchestration framework** that connects your preferred AI model to various services and capabilities through a secure, sandboxed environment. It acts as a secure bridge between:

- **AI Models** (Claude, GPT, Gemini, and 75+ providers via OpenCode)
- **Communication Channels** (Discord, Telegram, Slack)
- **Your Data** (files, notes, calendars, GitHub issues)
- **Custom Scripts** (safely sandboxed Starlark)

Think of OpenPact as your personal AI infrastructure - you control the AI, the tools it can use, and where your data goes.

## Why OpenPact?

### Security First

OpenPact implements the **principle of least privilege**. Your AI assistant can only use the tools you explicitly enable, and secrets never leak to the AI model.

```
AI Model <---> OpenPact <---> Your Services
              (sandbox)
```

- **Tool Allowlisting**: Only explicitly configured MCP tools are available
- **Secret Redaction**: API keys and tokens are automatically hidden from the AI
- **Two-User Docker Model**: Container runs with separated privileges
- **No Arbitrary Code Execution**: AI cannot run arbitrary commands

### 75+ AI Providers

OpenPact is powered by [OpenCode](https://opencode.ai), giving you access to a wide range of LLM providers:

- **Anthropic** (Claude), **OpenAI** (GPT), **Google** (Gemini), **Ollama**, and many more

Switch between models and providers without changing your configuration or losing your setup.

### Sandboxed Scripting

Extend OpenPact's capabilities with **Starlark scripts** - a Python-like language that runs in a secure sandbox:

- **No Filesystem Access**: Scripts cannot read or write files
- **HTTP Only**: Network access limited to HTTP/HTTPS
- **Execution Limits**: Configurable timeouts prevent runaway scripts
- **Automatic Secret Redaction**: Even script output is scanned for leaked secrets

### Production Ready

OpenPact is built for real-world deployment:

- **Docker Native**: Easy deployment with official container images
- **Health Checks**: Built-in endpoints for monitoring (`/health`, `/ready`, `/metrics`)
- **Prometheus Metrics**: Export metrics for your monitoring stack
- **Structured Logging**: JSON logging for log aggregation
- **Rate Limiting**: Built-in request rate limiting

## Key Concepts

### MCP Tools

OpenPact uses the **Model Context Protocol (MCP)** to expose capabilities to AI models. Each tool has a specific purpose and clear boundaries:

| Category | Tools |
|----------|-------|
| Workspace | `workspace_read`, `workspace_write`, `workspace_list` |
| Memory | `memory_read`, `memory_write` |
| Communication | `chat_send` |
| Integrations | `calendar_read`, `vault_*`, `github_*`, `web_fetch` |
| Scripting | `script_run`, `script_exec`, `script_list`, `script_reload` |

### Context Files

OpenPact uses special markdown files to shape AI behavior:

- **SOUL.md**: Defines the AI's identity and personality
- **USER.md**: Contains user preferences and context
- **MEMORY.md**: Persistent memory that the AI can read and update

### Starlark Scripts

Custom scripts let you extend OpenPact's capabilities safely:

```python
# Example: Fetch weather data
api_key = secrets.get("WEATHER_API_KEY")
resp = http.get(format("https://api.example.com/weather?key=%s", api_key))
data = json.decode(resp["body"])
```

The AI sees results but never the actual API key values.

## Quick Links

- **[Quick Start](./getting-started/quick-start)** - Get running in 5 minutes
- **[Installation](./getting-started/installation)** - Detailed setup options
- **[Configuration](./configuration/overview)** - Configure OpenPact for your needs
- **[MCP Tools Reference](./features/mcp-tools)** - All available tools

## Community

- **GitHub**: [github.com/open-pact/openpact](https://github.com/open-pact/openpact)
- **Discord**: Join our community for support and discussion
- **Website**: [openpact.ai](https://openpact.ai)
