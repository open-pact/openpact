---
sidebar_position: 2
title: Getting Started
description: Create and run your first Starlark script
---

# Getting Started with Starlark Scripts

This guide walks you through creating and running your first Starlark script in OpenPact.

## Prerequisites

Before creating scripts, ensure:

1. OpenPact is running with Starlark enabled
2. You have a scripts directory configured
3. Any required secrets are configured

### Configuration

In your `openpact.yaml`, enable Starlark scripting:

```yaml
starlark:
  enabled: true
  max_execution_ms: 30000  # 30 seconds
  secrets:
    MY_API_KEY: "${MY_API_KEY}"  # From environment variable
```

## Script Location

Scripts are stored in the `scripts/` subdirectory of the workspace. Each script is a `.star` file:

```
<workspace>/
└── scripts/
    ├── weather.star
    ├── stocks.star
    └── notifications.star
```

## Creating Your First Script

### Step 1: Create the Script File

Create a file named `hello.star` in your scripts directory:

```python
# @description: A simple hello world script
# @author: Your Name
# @version: 1.0.0

def greet(name):
    """Return a greeting message"""
    return {"message": format("Hello, %s!", name)}

# Default execution (when no function is specified)
result = greet("World")
```

### Step 2: Reload Scripts

After creating or modifying scripts, the AI can reload them:

```
Tool: script_reload
Args: {}
```

Or they are automatically loaded when OpenPact starts.

### Step 3: Run the Script

The AI can run your script in several ways:

**Run with default execution:**

```
Tool: script_run
Args: {"name": "hello"}
```

Result: `{"message": "Hello, World!"}`

**Call a specific function:**

```
Tool: script_run
Args: {"name": "hello", "function": "greet", "args": ["OpenPact"]}
```

Result: `{"message": "Hello, OpenPact!"}`

## Script Metadata

Scripts can declare metadata in special comments at the top of the file:

```python
# @description: What this script does
# @author: Your Name
# @version: 1.0.0
# @secrets: API_KEY, OTHER_SECRET
```

### Available Metadata Tags

| Tag | Description |
|-----|-------------|
| `@description` | Brief description of what the script does |
| `@author` | Script author name |
| `@version` | Version string (e.g., 1.0.0) |
| `@secrets` | Comma-separated list of required secrets |

The `script_list` tool returns this metadata, helping the AI understand what scripts are available and what they need.

## Script Structure

A well-structured script typically includes:

```python
# @description: Fetch weather data for a city
# @secrets: WEATHER_API_KEY

def get_weather(city):
    """
    Fetch current weather for the specified city.

    Args:
        city: City name or coordinates

    Returns:
        Dict with weather information
    """
    api_key = secrets.get("WEATHER_API_KEY")
    url = format("https://api.example.com/weather?q=%s&key=%s", city, api_key)

    resp = http.get(url)
    if resp["status"] != 200:
        return {"error": "Failed to fetch weather", "status": resp["status"]}

    data = json.decode(resp["body"])
    return {
        "city": city,
        "temperature": data["temp"],
        "conditions": data["conditions"]
    }

# Default execution
result = get_weather("London")
```

### Key Elements

1. **Metadata comments** - Document the script's purpose and requirements
2. **Function definitions** - Reusable functions that can be called with arguments
3. **Docstrings** - Document what functions do and what they return
4. **Error handling** - Check response status codes and handle failures
5. **Default execution** - A top-level statement that runs when no function is specified

## Listing Available Scripts

The AI can discover available scripts:

```
Tool: script_list
Args: {}
```

Returns:

```json
{
  "scripts": [
    {
      "name": "hello",
      "description": "A simple hello world script",
      "author": "Your Name",
      "version": "1.0.0",
      "secrets": []
    },
    {
      "name": "weather",
      "description": "Fetch weather data for a city",
      "author": "Your Name",
      "version": "1.0.0",
      "secrets": ["WEATHER_API_KEY"]
    }
  ]
}
```

## Executing Arbitrary Code

For one-off operations, the AI can execute Starlark code directly:

```
Tool: script_exec
Args: {
  "code": "result = http.get('https://api.example.com/status')\nresult['body']"
}
```

:::caution
`script_exec` executes arbitrary code and should be used carefully. Consider creating a proper script file for operations that will be repeated.
:::

## Debugging Scripts

### Check for Syntax Errors

When a script fails to load, the error message will indicate the problem:

```
Error: syntax error at line 5: unexpected token
```

### Test Incrementally

Build scripts step by step, testing each function:

```
Tool: script_exec
Args: {
  "code": "resp = http.get('https://httpbin.org/get')\nresp['status']"
}
```

### Use Print-Style Debugging

Return intermediate values to see what's happening:

```python
def debug_example():
    step1 = http.get(url)
    # Return early to check step1
    return {"debug": "step1", "status": step1["status"], "body": step1["body"]}
```

## Next Steps

- [Built-in Functions](./built-in-functions) - Learn all available functions
- [Security Model](./security-model) - Understand security guarantees
- [Examples](./examples) - See complete working scripts
