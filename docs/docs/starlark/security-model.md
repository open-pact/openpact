---
sidebar_position: 4
title: Security Model
description: Understanding Starlark's security guarantees and limitations
---

# Starlark Security Model

OpenPact's Starlark integration is designed with security as the primary concern. This page explains what scripts can and cannot do, and how secrets are protected.

## Security Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         AI Model                                 │
│  (Cannot see secret values - only sees [REDACTED:NAME])         │
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
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      AI Model                                    │
│  Receives: {"temp_c": 15.5, "url": "...key=[REDACTED:API_KEY]"} │
└─────────────────────────────────────────────────────────────────┘
```

## What Scripts CAN Do

### Make HTTP Requests

Scripts can make HTTP GET and POST requests to external services:

```python
resp = http.get("https://api.example.com/data")
resp = http.post("https://api.example.com/webhook", body=json.encode(data))
```

**Constraints:**
- Only `http://` and `https://` protocols
- No `file://`, `ftp://`, or other protocols
- Response body limited to 10MB
- Subject to script execution timeout

### Access Configured Secrets

Scripts can retrieve secret values for use in API calls:

```python
api_key = secrets.get("API_KEY")
token = secrets.get("AUTH_TOKEN")
```

**Constraints:**
- Only secrets explicitly configured in `openpact.yaml`
- Secret values are redacted from all output
- Cannot access environment variables directly

### Parse and Generate JSON

Scripts can work with structured data:

```python
data = json.decode(response_body)
output = json.encode({"result": data["value"]})
```

### Get Current Time

Scripts can access the current time:

```python
timestamp = time.now()  # Returns RFC3339 formatted UTC time
```

### Sleep (Limited)

Scripts can pause execution briefly:

```python
time.sleep(2)  # Wait 2 seconds (max 5 seconds)
```

---

## What Scripts CANNOT Do

### No Filesystem Access

Scripts cannot read or write files:

```python
# These operations are NOT available
open("file.txt")           # Not available
read_file("/etc/passwd")   # Not available
write_file("output.txt")   # Not available
```

**Why:** Filesystem access would allow scripts to read sensitive data, modify system files, or exfiltrate information.

### No System Command Execution

Scripts cannot run shell commands:

```python
# These operations are NOT available
os.system("ls")            # Not available
subprocess.run(["cmd"])    # Not available
exec("code")               # Not available (Python exec)
```

**Why:** Command execution would bypass all sandboxing and give full system access.

### No Direct Environment Variable Access

Scripts cannot read environment variables:

```python
# These operations are NOT available
os.environ["API_KEY"]      # Not available
os.getenv("SECRET")        # Not available
```

**Why:** Environment variables may contain secrets not intended for script access. Only secrets explicitly configured in `openpact.yaml` are available.

### No Network Protocols Besides HTTP/HTTPS

Scripts cannot use other network protocols:

```python
# These operations are NOT available
socket.connect()           # Not available
ftp.get("file")            # Not available
ssh.exec("cmd")            # Not available
```

**Why:** Other protocols could be used to bypass security controls or access internal services.

### No Import of External Modules

Scripts cannot import Python modules:

```python
# These operations are NOT available
import requests            # Not available
import os                  # Not available
from urllib import request # Not available
```

**Why:** External modules could provide capabilities that bypass the sandbox.

### No Infinite Loops or Excessive Resource Usage

Scripts have execution limits:

```python
# This will be terminated
while True:
    pass  # Infinite loop - script will timeout
```

**Why:** Resource limits prevent denial-of-service attacks.

---

## Secret Redaction

One of the most important security features is automatic secret redaction. Here's how it works:

### How Redaction Works

1. Script requests a secret: `api_key = secrets.get("API_KEY")`
2. OpenPact provides the actual value for script operations
3. Script makes an API call using the secret
4. Script returns a result (which may accidentally contain the secret)
5. **Before returning to AI:** OpenPact scans all output
6. Any occurrence of secret values is replaced with `[REDACTED:SECRET_NAME]`

### Example

```python
# Script code
api_key = secrets.get("API_KEY")
url = format("https://api.example.com?key=%s", api_key)
return {"url": url, "key_used": api_key}
```

**What the AI sees:**

```json
{
  "url": "https://api.example.com?key=[REDACTED:API_KEY]",
  "key_used": "[REDACTED:API_KEY]"
}
```

### Redaction Coverage

Redaction scans:
- All return values from scripts
- Error messages
- Nested data structures (dicts, lists)
- Partial matches within strings

### Limitations

While redaction is thorough, be aware:

- **Encoded secrets:** If a secret is base64 encoded or otherwise transformed, the original value might not be detected
- **Partial secrets:** Very short secrets (< 4 characters) may produce false positives
- **Split secrets:** If a secret is split across multiple values, individual parts may not be redacted

:::tip Best Practice
Design scripts to avoid returning secrets entirely. Only return the data the AI needs.
:::

---

## Execution Limits

Scripts are subject to several limits to prevent abuse:

### Time Limit

| Setting | Default | Description |
|---------|---------|-------------|
| `max_execution_ms` | 30000 | Maximum execution time in milliseconds |

Scripts that exceed this limit are terminated. Configure in `openpact.yaml`:

```yaml
starlark:
  max_execution_ms: 60000  # 60 seconds
```

### Memory Limit

Scripts are limited in memory usage. Attempting to allocate excessive memory will fail.

### Response Size Limit

HTTP responses are limited to 10MB. Larger responses will be truncated or cause an error.

### Sleep Limit

`time.sleep()` is capped at 5 seconds per call. Longer sleep values are reduced to 5 seconds.

### Recursion Limit

Deep recursion is limited to prevent stack overflow attacks.

---

## Security Best Practices

### 1. Minimize Secret Exposure

```python
# Good: Only return needed data
def get_weather(city):
    api_key = secrets.get("API_KEY")
    resp = http.get(format("https://api.example.com?key=%s&city=%s", api_key, city))
    data = json.decode(resp["body"])
    return {"temperature": data["temp"]}  # Only return what's needed

# Avoid: Don't return the URL or other data that might contain secrets
def get_weather_bad(city):
    api_key = secrets.get("API_KEY")
    url = format("https://api.example.com?key=%s&city=%s", api_key, city)
    return {"url": url, "response": http.get(url)}  # URL contains secret!
```

### 2. Validate Input

```python
def process_data(input_value):
    # Validate input before using it
    if not input_value or type(input_value) != "string":
        return {"error": "Invalid input"}

    if len(input_value) > 1000:
        return {"error": "Input too long"}

    # Process validated input
    ...
```

### 3. Handle Errors Gracefully

```python
def safe_api_call(endpoint):
    resp = http.get(endpoint)

    if resp["status"] != 200:
        return {
            "error": "API call failed",
            "status": resp["status"]
            # Don't include the full response which might contain sensitive data
        }

    return json.decode(resp["body"])
```

### 4. Use Specific Secrets

Configure specific secrets for specific purposes:

```yaml
starlark:
  secrets:
    WEATHER_API_KEY: "${WEATHER_API_KEY}"      # Only for weather scripts
    STOCK_API_KEY: "${STOCK_API_KEY}"          # Only for stock scripts
    NOTIFICATION_WEBHOOK: "${SLACK_WEBHOOK}"    # Only for notifications
```

### 5. Review Script Output

Periodically review what scripts are returning to ensure no sensitive data is leaking:

- Check logs for redaction markers
- Audit script return values
- Monitor for unexpected patterns

---

## Comparison with Alternatives

| Feature | Starlark (OpenPact) | Python | JavaScript (Node) |
|---------|---------------------|--------|-------------------|
| Filesystem access | None | Full | Full |
| Network access | HTTP/HTTPS only | Full | Full |
| System commands | None | Full | Full |
| Module imports | None | Full | Full |
| Secret redaction | Automatic | Manual | Manual |
| Execution limits | Enforced | Optional | Optional |
| Sandboxed by default | Yes | No | No |

Starlark provides strong security guarantees that would require significant additional work to achieve with general-purpose scripting languages.
