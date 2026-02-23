---
sidebar_position: 1
title: Overview
description: Introduction to Starlark scripting in OpenPact
---

# Starlark Scripting Overview

OpenPact includes a powerful, sandboxed scripting system using [Starlark](https://github.com/bazelbuild/starlark), a Python-like language designed for safe execution in untrusted environments.

## What is Starlark?

Starlark (formerly known as Skylark) is a dialect of Python designed by Google for configuration and extension. It was originally developed for the Bazel build system but has since become a popular choice for embedding scripting capabilities in applications.

Key characteristics of Starlark:

- **Python-like syntax** - Familiar to most developers
- **Deterministic execution** - Same inputs always produce same outputs
- **No side effects** - Cannot access filesystem, network, or system resources directly
- **Hermetic** - Completely isolated from the host environment
- **Fast** - Efficient interpretation with minimal overhead

## Why Sandboxed Scripting?

OpenPact is designed with security as a core principle. Traditional scripting languages like Python or JavaScript have full access to system resources, which creates significant security risks when executing code on behalf of an AI.

Starlark solves this by providing:

### Controlled Capabilities

Scripts can only access functionality explicitly provided by OpenPact:

| Capability | Starlark Module | Security Consideration |
|------------|-----------------|------------------------|
| HTTP requests | `http.get()`, `http.post()` | Limited to http/https protocols only |
| JSON parsing | `json.encode()`, `json.decode()` | No code execution from parsed data |
| Time access | `time.now()`, `time.sleep()` | Sleep limited to 5 seconds |
| Secrets | `secrets.get()`, `secrets.list()` | Values never exposed to AI |

### No Filesystem Access

Scripts cannot:
- Read files from the host system
- Write files to disk
- Execute system commands
- Access environment variables directly

### Automatic Secret Redaction

When scripts access secrets (like API keys), the actual values are used for operations but automatically redacted from any output returned to the AI.

## When to Use Scripts

Starlark scripts are ideal for:

### 1. API Integrations

Fetch data from external APIs that require authentication:

```python
# Weather, stock prices, notifications, etc.
api_key = secrets.get("API_KEY")
resp = http.get(format("https://api.example.com/data?key=%s", api_key))
```

### 2. Data Transformation

Process and transform data in ways the AI cannot:

```python
# Parse complex responses, aggregate data, format output
data = json.decode(response["body"])
result = transform_data(data)
return json.encode(result)
```

### 3. Authenticated Operations

Perform actions that require credentials the AI should never see:

```python
# Post to services, trigger webhooks, send notifications
token = secrets.get("WEBHOOK_TOKEN")
http.post(webhook_url, body=json.encode(payload), headers={"Authorization": token})
```

### 4. Combining Multiple Sources

Aggregate data from multiple APIs in a single operation:

```python
# Combine weather, calendar, and other data
weather = fetch_weather(city)
events = fetch_calendar()
return {"weather": weather, "events": events}
```

## When NOT to Use Scripts

Scripts are not appropriate for:

- **File operations** - Use workspace or vault tools instead
- **Memory operations** - Use the memory system tools
- **Chat messaging** - Use the chat_send tool
- **Long-running processes** - Scripts have execution time limits
- **Complex computation** - Scripts are interpreted, not compiled

## Script Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         AI Model                                 │
│  (Requests script execution via MCP tools)                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     MCP Script Tools                             │
│  script_run, script_exec, script_list, script_reload            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Starlark Sandbox                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ HTTP Client │  │    JSON     │  │   Secret Provider       │  │
│  │ (http/https │  │ encode/     │  │   secrets.get("KEY")    │  │
│  │  only)      │  │ decode      │  │   → returns real value  │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Result Sanitizer                               │
│  Scans all output for secret values and replaces them with      │
│  [REDACTED:SECRET_NAME] before returning to AI                  │
└─────────────────────────────────────────────────────────────────┘
```

## Available MCP Tools

OpenPact provides four MCP tools for script interaction:

| Tool | Description |
|------|-------------|
| `script_list` | List available scripts with metadata |
| `script_run` | Execute a script by name (optionally call specific function) |
| `script_exec` | Execute arbitrary Starlark code directly |
| `script_reload` | Reload scripts from disk after changes |

## Next Steps

- [Getting Started](./getting-started) - Create and run your first script
- [Built-in Functions](./built-in-functions) - Complete function reference
- [Security Model](./security-model) - Understand the security guarantees
- [Examples](./examples) - Real-world script examples
