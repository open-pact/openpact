---
sidebar_position: 3
title: Built-in Functions
description: Complete reference for Starlark built-in functions
---

# Built-in Functions Reference

OpenPact provides several built-in modules and functions for Starlark scripts. This page documents all available functionality.

## Quick Reference

| Module | Function | Description |
|--------|----------|-------------|
| `http` | `http.get(url, headers={})` | HTTP GET request |
| `http` | `http.post(url, body="", headers={}, content_type="application/json")` | HTTP POST request |
| `json` | `json.encode(value)` | Convert value to JSON string |
| `json` | `json.decode(string)` | Parse JSON string to value |
| `time` | `time.now()` | Current UTC time (RFC3339 format) |
| `time` | `time.sleep(seconds)` | Sleep (max 5 seconds) |
| `secrets` | `secrets.get(name)` | Get a secret value |
| `secrets` | `secrets.list()` | List available secret names |
| - | `format(fmt, args...)` | Printf-style string formatting |

---

## HTTP Module

The `http` module provides functions for making HTTP requests to external services.

### http.get()

Make an HTTP GET request.

**Signature:**
```python
http.get(url, headers={})
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | The URL to request (must be http:// or https://) |
| `headers` | dict | No | Optional HTTP headers to include |

**Returns:**

A dictionary with the response:

```python
{
    "status": 200,           # HTTP status code
    "body": "...",           # Response body as string
    "headers": {             # Response headers
        "Content-Type": "application/json",
        ...
    }
}
```

**Example:**

```python
# Simple GET request
resp = http.get("https://api.example.com/data")
if resp["status"] == 200:
    data = json.decode(resp["body"])

# GET with custom headers
resp = http.get(
    "https://api.example.com/data",
    headers={
        "Authorization": "Bearer " + secrets.get("API_TOKEN"),
        "Accept": "application/json"
    }
)
```

### http.post()

Make an HTTP POST request.

**Signature:**
```python
http.post(url, body="", headers={}, content_type="application/json")
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | The URL to request (must be http:// or https://) |
| `body` | string | No | Request body content |
| `headers` | dict | No | Optional HTTP headers to include |
| `content_type` | string | No | Content-Type header (default: "application/json") |

**Returns:**

Same response dictionary as `http.get()`.

**Example:**

```python
# POST JSON data
payload = json.encode({
    "message": "Hello from OpenPact",
    "timestamp": time.now()
})

resp = http.post(
    "https://api.example.com/webhook",
    body=payload,
    headers={
        "Authorization": "Bearer " + secrets.get("WEBHOOK_TOKEN")
    }
)

if resp["status"] != 200:
    return {"error": "POST failed", "status": resp["status"]}

# POST with custom content type
resp = http.post(
    "https://api.example.com/form",
    body="field1=value1&field2=value2",
    content_type="application/x-www-form-urlencoded"
)
```

### HTTP Limitations

- **Protocol:** Only `http://` and `https://` URLs are allowed (no `file://`, etc.)
- **Response size:** Maximum 10MB response body
- **Timeout:** Requests timeout based on script execution limit
- **Redirects:** Followed automatically (up to a reasonable limit)

---

## JSON Module

The `json` module provides functions for encoding and decoding JSON data.

### json.encode()

Convert a Starlark value to a JSON string.

**Signature:**
```python
json.encode(value)
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `value` | any | Yes | The value to encode (dict, list, string, number, bool, None) |

**Returns:**

A JSON-formatted string.

**Example:**

```python
data = {
    "name": "OpenPact",
    "version": 1,
    "features": ["security", "scripting", "integrations"],
    "enabled": True
}

json_string = json.encode(data)
# Result: '{"name":"OpenPact","version":1,"features":["security","scripting","integrations"],"enabled":true}'
```

### json.decode()

Parse a JSON string into a Starlark value.

**Signature:**
```python
json.decode(string)
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `string` | string | Yes | A valid JSON string |

**Returns:**

The parsed value (dict, list, string, number, bool, or None).

**Example:**

```python
resp = http.get("https://api.example.com/data")
data = json.decode(resp["body"])

# Access nested values
name = data["user"]["name"]
items = data["items"]

for item in items:
    print(item["id"])
```

### JSON Type Mapping

| JSON Type | Starlark Type |
|-----------|---------------|
| object | dict |
| array | list |
| string | string |
| number (int) | int |
| number (float) | float |
| true/false | True/False |
| null | None |

---

## Time Module

The `time` module provides functions for working with time.

### time.now()

Get the current UTC time.

**Signature:**
```python
time.now()
```

**Parameters:** None

**Returns:**

A string containing the current time in RFC3339 format.

**Example:**

```python
current_time = time.now()
# Result: "2026-02-05T14:30:00Z"

# Use in a payload
payload = json.encode({
    "event": "script_executed",
    "timestamp": time.now()
})
```

### time.sleep()

Pause execution for the specified duration.

**Signature:**
```python
time.sleep(seconds)
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `seconds` | number | Yes | Duration to sleep (max 5 seconds) |

**Returns:** None

**Example:**

```python
# Wait between API calls (rate limiting)
result1 = http.get(url1)
time.sleep(1)  # Wait 1 second
result2 = http.get(url2)
```

:::caution
The maximum sleep duration is 5 seconds. Attempting to sleep longer will be capped at 5 seconds. This prevents scripts from consuming resources indefinitely.
:::

---

## Secrets Module

The `secrets` module provides secure access to configured secrets.

### secrets.get()

Retrieve a secret value.

**Signature:**
```python
secrets.get(name)
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | The name of the secret to retrieve |

**Returns:**

The secret value as a string. Returns an empty string if the secret doesn't exist.

**Example:**

```python
api_key = secrets.get("WEATHER_API_KEY")
if not api_key:
    return {"error": "WEATHER_API_KEY not configured"}

url = format("https://api.weather.com/current?key=%s", api_key)
resp = http.get(url)
```

:::tip Security Note
Secret values are available to your script for operations (like API calls), but they are **automatically redacted** from any output returned to the AI. The AI will see `[REDACTED:SECRET_NAME]` instead of the actual value.
:::

### secrets.list()

List all available secret names.

**Signature:**
```python
secrets.list()
```

**Parameters:** None

**Returns:**

A list of secret names (strings).

**Example:**

```python
available = secrets.list()
# Result: ["WEATHER_API_KEY", "GITHUB_TOKEN", "WEBHOOK_SECRET"]

# Check if a secret exists
if "WEATHER_API_KEY" in secrets.list():
    # Proceed with weather API call
    pass
```

### Configuring Secrets

Secrets are configured in `openpact.yaml`:

```yaml
starlark:
  secrets:
    WEATHER_API_KEY: "${WEATHER_API_KEY}"  # From environment
    GITHUB_TOKEN: "${GITHUB_TOKEN}"
    STATIC_SECRET: "literal-value"          # Direct value (not recommended)
```

:::warning
Avoid putting literal secret values in configuration files. Always use environment variable references (`${VAR_NAME}`) for sensitive values.
:::

---

## format() Function

A built-in function for printf-style string formatting.

**Signature:**
```python
format(fmt, args...)
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `fmt` | string | Yes | Format string with `%s`, `%d`, etc. placeholders |
| `args...` | any | No | Values to substitute into the format string |

**Returns:**

A formatted string.

**Format Specifiers:**

| Specifier | Description |
|-----------|-------------|
| `%s` | String |
| `%d` | Integer |
| `%f` | Float |
| `%%` | Literal percent sign |

**Example:**

```python
# String formatting
url = format("https://api.example.com/%s/%s", "users", "123")
# Result: "https://api.example.com/users/123"

# Mixed types
message = format("User %s has %d items (%.2f%% complete)", "alice", 5, 75.5)
# Result: "User alice has 5 items (75.50% complete)"

# Building API URLs
api_key = secrets.get("API_KEY")
city = "London"
url = format("https://api.weather.com/v1/current?key=%s&q=%s&format=json", api_key, city)
```

---

## Standard Starlark Functions

In addition to OpenPact's modules, standard Starlark functions are available:

### Type Functions

```python
len(x)          # Length of string, list, or dict
str(x)          # Convert to string
int(x)          # Convert to integer
float(x)        # Convert to float
bool(x)         # Convert to boolean
type(x)         # Get type name as string
```

### List Functions

```python
list(x)         # Convert to list
sorted(x)       # Return sorted list
reversed(x)     # Return reversed list
range(n)        # Generate range [0, n)
range(a, b)     # Generate range [a, b)
range(a, b, c)  # Generate range [a, b) with step c
```

### Dict Functions

```python
dict(x)         # Convert to dict
keys = d.keys()         # Get dict keys
values = d.values()     # Get dict values
items = d.items()       # Get (key, value) pairs
d.get(key, default)     # Get with default
```

### String Methods

```python
s.upper()       # Uppercase
s.lower()       # Lowercase
s.strip()       # Remove whitespace
s.split(sep)    # Split by separator
s.join(list)    # Join list with separator
s.startswith(x) # Check prefix
s.endswith(x)   # Check suffix
s.replace(a, b) # Replace occurrences
```

### Control Flow

```python
# Conditionals
if condition:
    ...
elif other:
    ...
else:
    ...

# Loops
for item in items:
    ...

for i in range(10):
    ...

# List comprehensions
squares = [x * x for x in range(10)]
```

---

## Complete Example

Here's a complete script demonstrating multiple modules:

```python
# @description: Fetch and format weather data
# @secrets: WEATHER_API_KEY

def get_weather(city):
    """Fetch weather for a city and return formatted data."""

    # Check for required secret
    if "WEATHER_API_KEY" not in secrets.list():
        return {"error": "WEATHER_API_KEY not configured"}

    # Build request URL
    api_key = secrets.get("WEATHER_API_KEY")
    url = format(
        "https://api.weatherapi.com/v1/current.json?key=%s&q=%s",
        api_key,
        city
    )

    # Make request
    resp = http.get(url)
    if resp["status"] != 200:
        return {
            "error": "API request failed",
            "status": resp["status"],
            "timestamp": time.now()
        }

    # Parse response
    data = json.decode(resp["body"])

    # Return formatted result
    return {
        "city": data["location"]["name"],
        "country": data["location"]["country"],
        "temp_c": data["current"]["temp_c"],
        "temp_f": data["current"]["temp_f"],
        "condition": data["current"]["condition"]["text"],
        "humidity": data["current"]["humidity"],
        "wind_kph": data["current"]["wind_kph"],
        "fetched_at": time.now()
    }

# Default execution
result = get_weather("London")
```
