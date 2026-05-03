---
title: Introduction
sidebar_position: 1
---

# Introduction to OpenPact

:::caution Early Release
OpenPact is in early beta and is open for testing. APIs, configuration, and features are subject to change as the project evolves. We welcome feedback at [hello@openpact.ai](mailto:hello@openpact.ai).
:::

OpenPact is a secure, minimal framework for running your own AI assistant. It ships as a single static Go binary with an embedded Vue admin UI and an in-process LLM engine. You configure providers, sign in, and pick a model from the web UI — there is no separate AI service to install or supervise.


<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <img
    src="/img/logo-full.svg"
    alt="OpenPact Logo"
    style={{width: '100%', maxWidth: '350px', height: 'auto'}}
  />
</div>




## What is OpenPact?

OpenPact is an **AI orchestration framework** that connects your preferred AI model to various services and capabilities through a secure, sandboxed environment. It acts as a bridge between:

- **AI Providers** — OpenAI, GitHub Copilot, Google Gemini, and Ollama via the in-process [stackllm](https://github.com/stack-bound/stackllm) engine
- **Communication Channels** — Discord, Telegram, Slack
- **Your Data** — files, notes, calendars, GitHub issues
- **Custom Scripts** — sandboxed Starlark

You control the AI, the tools it can use, and where your data goes.

## Why OpenPact?

### Security First

OpenPact implements the **principle of least privilege**. The agent can only call tools that have been explicitly registered, and Starlark secrets are redacted before any output reaches the model.

```
AI Provider <---> stackllm (in-process) <---> OpenPact tool registry <---> Your services
                                                  (sandboxed)
```

- **Tool allowlisting** — only explicitly registered MCP tools are exposed to the agent.
- **Secret redaction** — API keys and tokens injected into Starlark scripts are scrubbed from output before the model sees them.
- **Workspace boundary** — workspace tools are scoped to `ai-data/`; `secure/` (config, JWT key, encryption key, SQLite DB) is unreachable.
- **JWT-protected admin API** — all `/api/*` endpoints require a valid token after the setup wizard finishes.

### Multi-provider, web-driven

Sign in to providers from the admin UI without ever editing config or environment variables:

| Provider | Sign-in flow |
|----------|--------------|
| OpenAI | API key, or "Sign in with ChatGPT" (Codex device flow) |
| GitHub Copilot | GitHub device flow |
| Google Gemini | API key |
| Ollama | local base URL |

Switch the default model at any time from `/engine`. Tokens are stored under `<workspace>/secure/data/` (file mode `0600`) and never appear in `config.yaml`.

### Sandboxed Scripting

Extend OpenPact with **Starlark scripts** — a Python-like language that runs in a secure sandbox:

- **No filesystem access** — scripts cannot read or write files.
- **HTTP only** — `http://` and `https://` URLs only.
- **Execution limits** — configurable timeout and response-size cap.
- **Automatic secret redaction** — output is scanned for leaked secret values.

### Production Ready

- **Single static binary** — pure-Go (CGO-free) build via `modernc.org/sqlite`. ~22 MB.
- **Health checks** — `/health`, `/healthz`, `/ready`.
- **Prometheus metrics** — `/metrics`.
- **Structured logging** — text or JSON.
- **Rate limiting** — token-bucket on the public surfaces.

## Key Concepts

### MCP Tools

OpenPact uses the **Model Context Protocol** as a tool-registration model. Each registered tool has a specific purpose and clear boundaries; at runtime they are invoked in-process by the stackllm agent.

| Category | Tools |
|----------|-------|
| Workspace | `workspace_read`, `workspace_write`, `workspace_list` |
| Memory | `memory_read`, `memory_write` |
| Communication | `discord_send` |
| Integrations | `calendar_read`, `vault_*`, `github_*`, `web_fetch` |
| Scripting | `script_run`, `script_exec`, `script_list`, `script_reload` |

### Context Files

OpenPact loads three markdown files from `ai-data/` into the system prompt on the first turn of each session:

- **SOUL.md** — defines the AI's identity and personality.
- **USER.md** — user preferences and context.
- **MEMORY.md** — persistent memory the AI can update via the `memory_*` tools.

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

- **[Quick Start](./getting-started/quick-start)** — get running in 5 minutes
- **[Installation](./getting-started/installation)** — detailed setup options
- **[Configuration](./configuration/overview)** — configure OpenPact for your needs
- **[MCP Tools Reference](./features/mcp-tools)** — all available tools

## Community

- **GitHub**: [github.com/open-pact/openpact](https://github.com/open-pact/openpact)
- **Discord**: Join our community for support and discussion
- **Website**: [openpact.ai](https://openpact.ai)
