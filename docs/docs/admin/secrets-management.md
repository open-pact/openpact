---
sidebar_position: 4
title: Secrets Management
description: Adding and removing secrets, security best practices
---

# Secrets Management

The Admin UI provides a secure interface for managing secrets (API keys, tokens, and other credentials) that Starlark scripts can access. This page covers how to add, update, and remove secrets safely.

## Overview

Secrets in OpenPact are:

- **Stored securely** - Encrypted at rest in the data directory
- **Never exposed** - Values are never returned via API or shown in the UI
- **Automatically redacted** - If a secret appears in script output, it's replaced with `[REDACTED:SECRET_NAME]`
- **Scoped to scripts** - Only available to Starlark scripts, not directly to the AI

## Secrets UI

Navigate to `/secrets` to manage secrets.

```
┌─────────────────────────────────────────────────────────────────┐
│  Secrets Management                              [+ Add Secret] │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Name                │ Status   │ Last Updated  │ Actions   │ │
│  ├────────────────────────────────────────────────────────────┤ │
│  │ WEATHER_API_KEY     │ ● Set    │ Jan 15, 2024  │ [Delete]  │ │
│  │ GITHUB_TOKEN        │ ● Set    │ Jan 14, 2024  │ [Delete]  │ │
│  │ SLACK_WEBHOOK       │ ○ Not Set│ -             │ [Set]     │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

The interface shows:

- **Secret name** - The identifier used in scripts
- **Status** - Whether a value has been set
- **Last updated** - When the secret was last modified
- **Actions** - Set or delete the secret

## Adding Secrets

### Via Admin UI

1. Click **+ Add Secret**
2. Enter the secret name (e.g., `WEATHER_API_KEY`)
3. Enter the secret value
4. Click **Save**

### Via API

```
POST /api/secrets/:name
```

Request:
```json
{
  "value": "sk-abc123..."
}
```

Response:
```json
{
  "name": "WEATHER_API_KEY",
  "set": true
}
```

### Via Configuration

Secrets can also be configured in `openpact.yaml`:

```yaml
starlark:
  secrets:
    WEATHER_API_KEY: "${WEATHER_API_KEY}"
    GITHUB_TOKEN: "${GITHUB_TOKEN}"
    SLACK_WEBHOOK: "${SLACK_WEBHOOK_URL}"
```

When using environment variable references (`${VAR}`), the actual values come from the environment at startup.

## Updating Secrets

To update an existing secret:

1. Navigate to the Secrets page
2. Find the secret you want to update
3. Click on the secret name or the **Update** action
4. Enter the new value
5. Click **Save**

:::note
Updating a secret does not require re-approval of scripts that use it. The script continues working with the new value.
:::

## Deleting Secrets

To delete a secret:

1. Navigate to the Secrets page
2. Click **Delete** next to the secret
3. Confirm the deletion

### Via API

```
DELETE /api/secrets/:name
```

Response: `204 No Content`

:::warning
Deleting a secret will cause scripts that depend on it to fail. Review which scripts use a secret before deleting it.
:::

## Listing Secrets

### Via API

```
GET /api/secrets
```

Response:
```json
{
  "secrets": [
    {
      "name": "WEATHER_API_KEY",
      "set": true,
      "last_updated": "2024-01-15T10:30:00Z"
    },
    {
      "name": "GITHUB_TOKEN",
      "set": true,
      "last_updated": "2024-01-14T09:00:00Z"
    }
  ]
}
```

:::note
The API never returns actual secret values - only metadata about which secrets exist.
:::

## Using Secrets in Scripts

Scripts access secrets using the `secrets` module:

```python
def get_weather(city):
    api_key = secrets.get("WEATHER_API_KEY")
    url = format("https://api.weather.com/v1/current?key=%s&city=%s", api_key, city)
    response = http.get(url)
    return json.decode(response["body"])
```

### Available Functions

| Function | Description |
|----------|-------------|
| `secrets.get("NAME")` | Get a secret value, returns empty string if not set |
| `secrets.list()` | List available secret names (not values) |

## Automatic Redaction

When scripts return data, OpenPact automatically scans for secret values and redacts them:

### Example

Script:
```python
api_key = secrets.get("API_KEY")
url = format("https://api.example.com?key=%s", api_key)
return {"url": url, "key": api_key}
```

What the AI sees:
```json
{
  "url": "https://api.example.com?key=[REDACTED:API_KEY]",
  "key": "[REDACTED:API_KEY]"
}
```

### Redaction Coverage

Redaction scans:
- Return values from scripts
- Error messages
- Nested data structures (dicts, lists, strings)
- Partial matches within strings

## Security Best Practices

### 1. Use Specific Secrets

Create separate secrets for different purposes:

```yaml
starlark:
  secrets:
    WEATHER_API_KEY: "${WEATHER_API_KEY}"      # Only for weather scripts
    STOCK_API_KEY: "${STOCK_API_KEY}"          # Only for stock scripts
    NOTIFICATION_WEBHOOK: "${SLACK_WEBHOOK}"   # Only for notifications
```

This limits exposure if one secret is compromised.

### 2. Rotate Secrets Regularly

Set a schedule to rotate secrets:

- API keys: Every 90 days
- Tokens: When they expire or every 30 days
- Webhooks: When personnel changes

### 3. Use Least Privilege

Request only the permissions you need when creating API keys:

| Instead of | Use |
|------------|-----|
| Full admin access | Read-only access |
| All repositories | Specific repository access |
| Unlimited rate | Limited rate tier |

### 4. Monitor Secret Usage

Review which scripts use which secrets:

```python
# Script metadata
# @name: weather.star
# @secrets: WEATHER_API_KEY
```

### 5. Never Log Secret Values

Avoid patterns that might leak secrets:

```python
# BAD - might leak in logs
print("Using key: " + api_key)

# GOOD - reference by name only
print("Using secret: WEATHER_API_KEY")
```

### 6. Secure the Data Directory

The secrets file is stored in the data directory:

```
data/
└── secrets.json    # Encrypted at rest
```

Ensure proper file permissions:
```bash
chmod 600 data/secrets.json
chmod 700 data/
```

### 7. Use Environment Variables in Production

For containerized deployments, inject secrets via environment:

```bash
docker run -e WEATHER_API_KEY="sk-..." openpact
```

Or use a secrets manager:

```yaml
# docker-compose.yml with secrets
services:
  openpact:
    secrets:
      - weather_api_key
    environment:
      WEATHER_API_KEY_FILE: /run/secrets/weather_api_key

secrets:
  weather_api_key:
    external: true
```

## Storage Details

### File-Based Storage

Secrets are stored in `data/secrets.json`:

```json
{
  "WEATHER_API_KEY": {
    "value": "encrypted:...",
    "last_updated": "2024-01-15T10:30:00Z"
  }
}
```

- File permissions: 0600 (owner read/write only)
- Values are encrypted using a key derived from the JWT secret

### Environment Variable Override

Environment variables can override file-based secrets:

```bash
export OPENPACT_SECRET_WEATHER_API_KEY="sk-..."
```

Priority order:
1. Environment variable (`OPENPACT_SECRET_*`)
2. Admin UI / API set value
3. Configuration file reference

## Troubleshooting

### Script reports "secret not found"

- Verify the secret name matches exactly (case-sensitive)
- Check if the secret has been set in the Admin UI
- Ensure the secret name is in the script's `@secrets` metadata

### Secret appears in output (not redacted)

- The value might be transformed (base64, etc.) - redaction only catches exact matches
- Very short secrets (< 4 chars) may not be redacted to avoid false positives
- Report this as a security issue if it persists

### Cannot delete secret

- Ensure no scripts currently depend on this secret
- Check for any pending script approvals that reference the secret
