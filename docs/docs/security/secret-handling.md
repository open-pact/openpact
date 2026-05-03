---
sidebar_position: 3
title: Secret Handling
description: Secret injection, redaction, and environment variables
---

# Secret Handling

OpenPact provides a comprehensive system for managing secrets (API keys, tokens, credentials) that keeps them secure while allowing scripts to use them. This page explains how secrets flow through the system and the security measures that protect them.

## Secret Flow Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       Secret Sources                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   Admin UI   │  │ Environment  │  │   Config File        │  │
│  │   (API)      │  │  Variables   │  │   (references)       │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Secret Store                                 │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                 Encrypted at Rest                           ││
│  │  WEATHER_API_KEY: encrypted:aes256:...                      ││
│  │  GITHUB_TOKEN: encrypted:aes256:...                         ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Script Runtime                                 │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ api_key = secrets.get("WEATHER_API_KEY")                    ││
│  │ # Returns actual value for use in script                    ││
│  │                                                              ││
│  │ http.get(format("https://api.weather.com?key=%s", api_key)) ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Output Sanitizer                               │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Scans all output for secret values                          ││
│  │ Replaces with [REDACTED:SECRET_NAME]                        ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        AI Model                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Receives: {"url": "...key=[REDACTED:WEATHER_API_KEY]"}      ││
│  │ Never sees actual secret values                             ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Secret Injection

### From the Admin UI

This is the only supported path for Starlark secrets. The values are encrypted at rest in `op_secrets` (AES-256-GCM under `data_encryption_key`).

1. Navigate to **Secrets** (`/secrets`).
2. Click **Add Secret**.
3. Enter name and value.
4. Click **Save**.

The value is encrypted before insert; the API never returns it.

### Special: GitHub token

`GITHUB_TOKEN` is a slight exception — it's used by the GitHub MCP tools (not just Starlark scripts) and can be supplied either via the admin UI's **Secrets** page (as a `GITHUB_TOKEN` row in `op_secrets`) **or** via the `GITHUB_TOKEN` environment variable. The DB row wins when both are set.

### Special: chat-provider tokens

Discord, Slack, and Telegram tokens live in `op_chat_providers`, not `op_secrets`. Set them via **Providers** in the admin UI, or use the `DISCORD_TOKEN` / `SLACK_BOT_TOKEN` / `SLACK_APP_TOKEN` / `TELEGRAM_BOT_TOKEN` env-var fallbacks.

### Special: LLM provider tokens

OpenAI, Gemini, Copilot, and Ollama credentials are owned by stackllm and stored in `<workspace>/secure/data/stackllm_auth.json` (mode `0600`). Sign in via the admin UI's **Engine** page (`/engine`); there is no env-var fallback for these.

:::warning
Never commit plain-text secrets to version control. Use the admin UI for Starlark secrets, env vars for provider-fallback tokens, and a real secrets manager (Vault, AWS Secrets Manager, etc.) injecting env vars in production.
:::

## Secret Storage

### Encryption at Rest

Starlark secrets live in the `op_secrets` table inside the shared SQLite database (`<workspace>/secure/data/stackllm.db`). Each value is encrypted with **AES-256-GCM** under a per-installation `data_encryption_key` (separate from the JWT signing key, with its own rotation schedule).

A row looks roughly like:

```
op_secrets
| name              | nonce | ciphertext       | last_updated         |
|-------------------|-------|------------------|----------------------|
| WEATHER_API_KEY   | …     | <encrypted blob> | 2026-04-15T10:30:00Z |
| GITHUB_TOKEN      | …     | <encrypted blob> | 2026-04-14T09:00:00Z |
```

The plaintext value is decrypted in-memory only when a Starlark script calls `secrets.get()`.

### File Permissions

```bash
# Encryption key (separate file)
-rw-------  1 openpact openpact   32 Apr 15 10:00 secure/data/data_encryption_key

# JWT signing key (also separate)
-rw-------  1 openpact openpact   32 Apr 15 10:00 secure/data/jwt_secret

# Shared SQLite DB (op_secrets lives here, encrypted at the column level)
-rw-------  1 openpact openpact  ... Apr 15 10:30 secure/data/stackllm.db

# Secure directory (AI has ZERO access)
drwx------  2 openpact openpact  ... Apr 15 10:00 secure/
drwx------  2 openpact openpact  ... Apr 15 10:00 secure/data/
```

Only the OpenPact process can read these files. The agent has no tool that reaches `secure/` and Linux permissions back the boundary up.

## Automatic Redaction

### How Redaction Works

1. **Track accessed secrets** - When a script calls `secrets.get()`, the value is recorded
2. **Execute script** - Script runs with access to real secret values
3. **Scan output** - All return values are scanned for secret values
4. **Replace matches** - Any occurrence of a secret value is replaced with `[REDACTED:NAME]`

### Redaction Example

Script code:
```python
api_key = secrets.get("API_KEY")
url = format("https://api.example.com?key=%s&city=%s", api_key, city)
response = http.get(url)
return {
    "url": url,
    "data": json.decode(response["body"]),
    "key_used": api_key
}
```

What the AI sees:
```json
{
  "url": "https://api.example.com?key=[REDACTED:API_KEY]&city=London",
  "data": {"temperature": 15.5},
  "key_used": "[REDACTED:API_KEY]"
}
```

### Redaction Coverage

The sanitizer scans:

| Location | Scanned |
|----------|---------|
| Return values | Yes |
| Nested dictionaries | Yes |
| Nested lists | Yes |
| String contents | Yes |
| Error messages | Yes |
| Stack traces | Yes |

### Redaction Limitations

Be aware of these edge cases:

1. **Encoded secrets** - If you base64 encode a secret, the encoded form won't be redacted
   ```python
   # Bad - encoded secret won't be redacted
   encoded = base64_encode(secrets.get("KEY"))
   ```

2. **Partial secrets** - If a secret is split, individual parts may not be redacted
   ```python
   # Bad - split secret parts won't be redacted
   key = secrets.get("KEY")
   return {"part1": key[:10], "part2": key[10:]}
   ```

3. **Short secrets** - Secrets shorter than 4 characters may produce false positives and are not recommended

### Best Practices

```python
# Good - only return needed data
def get_weather(city):
    api_key = secrets.get("API_KEY")
    resp = http.get(format("https://api.weather.com?key=%s&city=%s", api_key, city))
    data = json.decode(resp["body"])
    return {"temperature": data["temp"]}  # Only the data needed

# Bad - returns data that might contain secrets
def get_weather_bad(city):
    api_key = secrets.get("API_KEY")
    url = format("https://api.weather.com?key=%s&city=%s", api_key, city)
    return {"url": url, "response": http.get(url)}  # URL contains secret!
```

## Environment Variable Security

### Container Isolation

In Docker, environment variables are isolated:

```yaml
# docker-compose.yml
services:
  openpact:
    environment:
      WEATHER_API_KEY: "${WEATHER_API_KEY}"
      GITHUB_TOKEN: "${GITHUB_TOKEN}"
    # These are only available to the OpenPact process
```

### Variable Naming

Use clear, specific names:

```bash
# Good - specific purpose
export WEATHER_API_KEY="..."
export GITHUB_READONLY_TOKEN="..."
export SLACK_WEBHOOK_URL="..."

# Bad - generic names
export API_KEY="..."
export TOKEN="..."
export SECRET="..."
```

### Variable Scoping

Scope variables to specific scripts when possible:

```yaml
starlark:
  secrets:
    # Weather scripts only
    WEATHER_API_KEY: "${WEATHER_API_KEY}"

    # Notification scripts only
    SLACK_WEBHOOK: "${SLACK_WEBHOOK_URL}"

    # GitHub scripts only
    GITHUB_TOKEN: "${GITHUB_READONLY_TOKEN}"
```

## Secret Access in Scripts

### Available Functions

| Function | Description |
|----------|-------------|
| `secrets.get("NAME")` | Get secret value, empty string if not set |
| `secrets.list()` | List available secret names (not values) |
| `secrets.has("NAME")` | Check if secret exists |

### Usage Patterns

```python
# Get a secret (returns empty string if not found)
api_key = secrets.get("API_KEY")

# Check if secret exists before using
if secrets.has("API_KEY"):
    api_key = secrets.get("API_KEY")
else:
    return {"error": "API_KEY not configured"}

# List available secrets (for debugging)
available = secrets.list()  # Returns ["API_KEY", "TOKEN", ...]
```

### Script Metadata

Declare required secrets in script metadata:

```python
# @name: weather.star
# @description: Fetch weather data
# @secrets: WEATHER_API_KEY

def get_weather(city):
    key = secrets.get("WEATHER_API_KEY")
    # ...
```

This helps administrators know which secrets a script needs.

## Secret Rotation

### Updating Secrets

1. **Via Admin UI:**
   - Navigate to `/secrets`
   - Click on the secret name
   - Enter new value
   - Save

2. **Via Environment Variable:**
   - Update the environment variable
   - Restart OpenPact

### Zero-Downtime Rotation

For critical secrets, use this pattern:

1. Add new secret with different name
   ```yaml
   WEATHER_API_KEY_V2: "${WEATHER_API_KEY_V2}"
   ```

2. Update script to use new secret
   ```python
   api_key = secrets.get("WEATHER_API_KEY_V2")
   ```

3. Re-approve the updated script

4. Remove old secret after confirming new one works

## Security Checklist

### Configuration

- [ ] Use environment variables for all production secrets
- [ ] Never commit secrets to version control
- [ ] Use specific, descriptive secret names
- [ ] Document which scripts need which secrets

### Storage

- [ ] Verify `secure/` directory permissions (700)
- [ ] Verify `secure/data/stackllm.db` and `secure/data/data_encryption_key` permissions (600)
- [ ] Ensure backups don't contain unencrypted secrets

### Access

- [ ] Review which scripts access which secrets
- [ ] Remove unused secrets
- [ ] Rotate secrets on schedule
- [ ] Use read-only/scoped credentials when possible

### Monitoring

- [ ] Log secret access (name only, not values)
- [ ] Alert on unusual secret access patterns
- [ ] Review redaction logs for missed patterns

## Troubleshooting

### "Secret not found" in script

1. Verify the secret is configured in Admin UI or config file
2. Check for typos in the secret name (case-sensitive)
3. Ensure environment variable is set if using `${VAR}` reference

### Secret appears in AI output (not redacted)

1. Check if the secret was transformed (base64, etc.)
2. Verify the script is accessing the secret via `secrets.get()`
3. Report as a bug if exact value appears unredacted

### Cannot set secret via Admin UI

1. Ensure you're authenticated
2. Check for special characters that may need escaping
3. Verify the `secure/data/` directory is writable
