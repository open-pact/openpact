---
sidebar_position: 5
title: Script Sandboxing
description: Starlark restrictions, execution limits, and output sanitization
---

# Script Sandboxing

OpenPact uses Starlark, a Python-like language designed for safe execution, to run scripts in a secure sandbox. This page explains the restrictions, limits, and security mechanisms that protect your system.

## Why Starlark?

Starlark (formerly Skylark) was developed by Google for the Bazel build system. It provides:

- **Python-like syntax** - Familiar to most developers
- **No side effects** - Cannot access filesystem, network, or system resources directly
- **Deterministic execution** - Same inputs always produce same outputs
- **Hermetic** - Completely isolated from the host environment
- **Fast** - Efficient interpretation with minimal overhead

## What Scripts CAN Do

### Make HTTP Requests

```python
# HTTP GET
response = http.get("https://api.example.com/data")

# HTTP POST with body
response = http.post(
    "https://api.example.com/webhook",
    body=json.encode({"message": "hello"}),
    headers={"Content-Type": "application/json"}
)
```

**Constraints:**
- Only `http://` and `https://` protocols
- Response body limited to 10MB
- Subject to script execution timeout

### Access Configured Secrets

```python
api_key = secrets.get("API_KEY")
token = secrets.get("AUTH_TOKEN")
```

**Constraints:**
- Only secrets explicitly configured
- Values are redacted from all output
- Cannot access environment variables

### Parse and Generate JSON

```python
# Decode JSON
data = json.decode('{"name": "test", "value": 42}')
name = data["name"]

# Encode JSON
output = json.encode({"result": "success"})
```

### Get Current Time

```python
# Get current UTC time
timestamp = time.now()  # Returns RFC3339 formatted string
```

### Sleep (Limited)

```python
# Wait for a short period
time.sleep(2)  # Maximum 5 seconds
```

### String Formatting

```python
# Format strings
url = format("https://api.example.com?key=%s&city=%s", api_key, city)
```

## What Scripts CANNOT Do

### No Filesystem Access

```python
# None of these work
open("file.txt")           # Not available
read_file("/etc/passwd")   # Not available
write_file("output.txt")   # Not available
os.path.exists("file")     # Not available
```

**Why:** Filesystem access would allow scripts to read sensitive data, modify system files, or exfiltrate information.

### No System Command Execution

```python
# None of these work
os.system("ls")            # Not available
subprocess.run(["cmd"])    # Not available
exec("code")               # Not available
eval("expression")         # Not available
```

**Why:** Command execution would bypass all sandboxing and give full system access.

### No Environment Variable Access

```python
# None of these work
os.environ["SECRET"]       # Not available
os.getenv("API_KEY")       # Not available
```

**Why:** Environment variables may contain secrets not intended for script access.

### No Module Imports

```python
# None of these work
import requests             # Not available
import os                   # Not available
from urllib import request  # Not available
```

**Why:** External modules could provide capabilities that bypass the sandbox.

### No Raw Network Access

```python
# None of these work
socket.connect()           # Not available
urllib.urlopen()           # Not available
ftp.download()             # Not available
```

**Why:** Raw network access could be used to attack internal services or exfiltrate data.

### No Reflection or Dynamic Execution

```python
# None of these work
globals()                  # Not available
locals()                   # Not available
getattr(obj, "method")     # Not available (dynamic attribute access)
__import__("module")       # Not available
```

**Why:** Dynamic capabilities could be used to escape the sandbox.

## Execution Limits

### Time Limit

Scripts are terminated if they exceed the maximum execution time:

| Setting | Default | Range |
|---------|---------|-------|
| `max_execution_ms` | 30000 | 1000-300000 |

```yaml
# openpact.yaml
starlark:
  max_execution_ms: 60000  # 60 seconds
```

### Memory Limit

Scripts have limited memory allocation:

| Setting | Default | Notes |
|---------|---------|-------|
| Memory | ~128MB | Terminates on exhaustion |

### Sleep Limit

`time.sleep()` is capped:

| Function | Limit |
|----------|-------|
| `time.sleep(n)` | Maximum 5 seconds per call |

Longer values are automatically reduced to 5 seconds.

### Response Size Limit

HTTP responses are limited:

| Type | Limit |
|------|-------|
| Response body | 10MB |

Larger responses will cause an error.

### Recursion Limit

Deep recursion is prevented:

```python
# This will be terminated
def infinite():
    return infinite()

infinite()  # Recursion limit exceeded
```

### Loop Iteration Limit

Long-running loops are monitored:

```python
# May be terminated if taking too long
for i in range(1000000):
    process(i)  # Will hit time limit
```

## Output Sanitization

### Automatic Secret Redaction

All script output is scanned for secret values:

```python
api_key = secrets.get("API_KEY")
return {"key": api_key, "url": format("...?key=%s", api_key)}
```

AI receives:
```json
{
  "key": "[REDACTED:API_KEY]",
  "url": "...?key=[REDACTED:API_KEY]"
}
```

### Redaction Process

```
┌────────────────────────────────┐
│     Script Execution           │
│  (secrets accessed internally) │
└────────────────────────────────┘
              │
              ▼
┌────────────────────────────────┐
│     Output Generated           │
│  {"key": "sk-abc123..."}       │
└────────────────────────────────┘
              │
              ▼
┌────────────────────────────────┐
│     Sanitizer Scans            │
│  Finds: "sk-abc123..."         │
│  Matches: API_KEY secret       │
└────────────────────────────────┘
              │
              ▼
┌────────────────────────────────┐
│     Output Redacted            │
│  {"key": "[REDACTED:API_KEY]"} │
└────────────────────────────────┘
              │
              ▼
┌────────────────────────────────┐
│     Returned to AI             │
│  Secret value never exposed    │
└────────────────────────────────┘
```

### Error Message Sanitization

Error messages are also sanitized:

```python
# If this fails with the API key in the error
http.get(format("https://api.example.com?key=%s", api_key))

# Error message is redacted
# "connection failed: https://api.example.com?key=[REDACTED:API_KEY]"
```

## Security Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     MCP Request Layer                           │
│  Validates request, checks script approval                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Script Loader                                  │
│  Loads script, verifies hash matches approval                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Starlark Runtime                               │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Built-in modules only:                                       ││
│  │ - http (GET, POST)                                          ││
│  │ - json (encode, decode)                                     ││
│  │ - time (now, sleep)                                         ││
│  │ - secrets (get, list, has)                                  ││
│  │ - format (string formatting)                                ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Resource monitors:                                          ││
│  │ - Execution time                                            ││
│  │ - Memory usage                                              ││
│  │ - Recursion depth                                           ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Output Sanitizer                               │
│  Scans for secrets, redacts matches, returns clean output       │
└─────────────────────────────────────────────────────────────────┘
```

## Comparison with Alternatives

| Feature | Starlark | Python | JavaScript | Lua |
|---------|----------|--------|------------|-----|
| Sandboxed by default | Yes | No | No | Partial |
| Filesystem access | None | Full | Full | Full |
| Network access | HTTP only | Full | Full | Full |
| Module imports | None | Full | Full | Full |
| Secret redaction | Automatic | Manual | Manual | Manual |
| Deterministic | Yes | No | No | No |
| Time limits | Enforced | Optional | Optional | Optional |

## Security Best Practices

### For Script Authors

1. **Return minimal data**
   ```python
   # Good
   return {"temperature": data["temp"]}

   # Bad - may contain sensitive data
   return data
   ```

2. **Don't construct URLs with secrets in return values**
   ```python
   # Good - use secret internally only
   resp = http.get(format("...?key=%s", api_key))
   return json.decode(resp["body"])

   # Bad - URL with secret in return value
   url = format("...?key=%s", api_key)
   return {"url": url}  # Will be redacted but still risky
   ```

3. **Handle errors without exposing details**
   ```python
   # Good
   if resp["status"] != 200:
       return {"error": "API request failed"}

   # Bad - may expose sensitive info
   if resp["status"] != 200:
       return {"error": resp["body"]}
   ```

4. **Validate inputs**
   ```python
   def process(input):
       if not input or type(input) != "string":
           return {"error": "Invalid input"}
       if len(input) > 1000:
           return {"error": "Input too long"}
       # ...
   ```

### For Administrators

1. **Review scripts carefully before approval**
   - Check all URLs and endpoints
   - Verify secret usage is appropriate
   - Look for potential data leaks

2. **Set appropriate limits**
   ```yaml
   starlark:
     max_execution_ms: 30000
     max_pending: 50
   ```

3. **Monitor execution logs**
   - Watch for failed executions
   - Look for unusual patterns
   - Review redaction activity

4. **Use allowlists for trusted scripts**
   ```yaml
   scripts:
     allowlist:
       - "weather.star"  # Trusted, version-controlled
   ```

## Troubleshooting

### Script times out

- Reduce complexity or data volume
- Increase `max_execution_ms` if necessary
- Check for infinite loops

### "Function not available" error

- The function is not provided in the sandbox
- Use only available built-in modules
- Rewrite to use provided alternatives

### Memory exhaustion

- Process data in smaller chunks
- Reduce data structures in memory
- Avoid keeping large responses

### Secret appears in output

- Check if secret was transformed (encoded)
- Ensure using `secrets.get()` properly
- Report as security issue if exact value appears
