---
sidebar_position: 3
title: Admin API
description: REST API for authentication, scripts, secrets, sessions, and schedule management
---

# Admin API

The Admin API provides RESTful endpoints for managing OpenPact through the Admin UI or programmatically. All endpoints (except setup and login) require authentication.

## Base URL

```
http://localhost:8080/api
```

## Authentication

The Admin API uses a two-token authentication system:

| Token | Storage | Lifetime | Purpose |
|-------|---------|----------|---------|
| **Refresh Token** | HTTP-only cookie | 3 days | Obtain new access tokens |
| **Access Token** | In-memory (JS) | 15 minutes | API authorization |

### Authentication Flow

```
1. POST /api/auth/login     → Receive refresh token cookie
2. GET /api/session         → Exchange cookie for access token
3. GET /api/scripts         → Use access token in Authorization header
4. (token expires)          → Automatically refresh via /api/session
```

---

## Setup Endpoints

These endpoints are only available before first-run setup is complete.

### GET /api/setup/status

Check if first-run setup is required.

**Response (Setup Required):**

```json
{
  "setup_required": true
}
```

**Response (Setup Complete):**

```json
{
  "setup_required": false
}
```

### POST /api/setup

Complete first-run setup by creating the admin user.

**Request:**

```json
{
  "username": "admin",
  "password": "your-secure-password",
  "confirm_password": "your-secure-password"
}
```

**Password Requirements:**

- **Option 1:** 16+ characters (passphrase style)
- **Option 2:** 12+ characters with 3 of 4: uppercase, lowercase, number, symbol

**Response (Success):**

```json
{
  "success": true,
  "message": "Setup complete. Please log in."
}
```

**Errors:**

| Status | Code | Description |
|--------|------|-------------|
| 400 | `password_invalid` | Password doesn't meet requirements |
| 400 | `password_mismatch` | Passwords don't match |
| 403 | `setup_completed` | Setup already completed |

---

## Authentication Endpoints

### POST /api/auth/login

Authenticate and receive a refresh token cookie.

**Request:**

```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response (Success):**

```json
{
  "message": "Login successful"
}
```

**Response Headers:**

```
Set-Cookie: refresh=xxx; HttpOnly; Secure; Path=/api/session; SameSite=Strict; Max-Age=259200
```

**Errors:**

| Status | Code | Description |
|--------|------|-------------|
| 401 | `invalid_credentials` | Username or password incorrect |
| 429 | `rate_limited` | Too many login attempts (5/minute) |

### GET /api/session

Exchange refresh token cookie for an access token.

**Request:**

Requires `refresh` cookie (sent automatically by browser).

**Response (Success):**

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2024-01-15T10:45:00Z",
  "username": "admin"
}
```

**Errors:**

| Status | Code | Description |
|--------|------|-------------|
| 401 | `no_refresh_token` | No refresh token cookie present |
| 401 | `invalid_refresh_token` | Token expired or invalid |

### POST /api/auth/logout

Clear the refresh token cookie.

**Response:**

```
204 No Content
```

**Response Headers:**

```
Set-Cookie: refresh=; Max-Age=-1; HttpOnly; Secure; Path=/api/session; SameSite=Strict
```

### GET /api/auth/me

Get current user information.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "username": "admin",
  "role": "admin"
}
```

---

## Script Endpoints

All script endpoints require authentication via Bearer token.

### GET /api/scripts

List all scripts with their status.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "scripts": [
    {
      "name": "weather.star",
      "path": "scripts/weather.star",
      "hash": "sha256:abc123def456...",
      "status": "approved",
      "description": "Get current weather for a city",
      "required_secrets": ["WEATHER_API_KEY"],
      "approved_at": "2024-01-15T10:30:00Z",
      "approved_by": "admin",
      "created_at": "2024-01-15T09:00:00Z",
      "modified_at": "2024-01-15T10:00:00Z"
    },
    {
      "name": "new_feature.star",
      "path": "scripts/new_feature.star",
      "hash": "sha256:789ghi012jkl...",
      "status": "pending",
      "description": "Experimental feature",
      "required_secrets": [],
      "created_at": "2024-01-15T11:00:00Z",
      "modified_at": "2024-01-15T11:00:00Z"
    }
  ]
}
```

**Script Status Values:**

| Status | Description |
|--------|-------------|
| `pending` | Awaiting admin review |
| `approved` | Can be executed |
| `rejected` | Blocked from execution |

### GET /api/scripts/:name

Get detailed information about a specific script.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "name": "weather.star",
  "source": "# @description: Get current weather for a city\n# @secrets: WEATHER_API_KEY\n\ndef get_weather(city):\n    ...",
  "hash": "sha256:abc123def456...",
  "status": "approved",
  "metadata": {
    "description": "Get current weather for a city",
    "author": "admin",
    "version": "1.0.0",
    "secrets": ["WEATHER_API_KEY"]
  },
  "execution_history": [
    {
      "timestamp": "2024-01-15T14:30:00Z",
      "success": true,
      "duration_ms": 150
    },
    {
      "timestamp": "2024-01-15T14:00:00Z",
      "success": true,
      "duration_ms": 145
    }
  ]
}
```

### POST /api/scripts

Create a new script.

**Request Headers:**

```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request:**

```json
{
  "name": "new_script.star",
  "source": "# @description: My new script\n\ndef main():\n    return {\"message\": \"Hello\"}"
}
```

**Response:**

```json
{
  "name": "new_script.star",
  "status": "pending",
  "hash": "sha256:newscripthash..."
}
```

**Note:** New scripts always start with `pending` status.

### PUT /api/scripts/:name

Update an existing script.

**Request Headers:**

```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request:**

```json
{
  "source": "# @description: Updated script\n\ndef main():\n    return {\"message\": \"Updated\"}"
}
```

**Response:**

```json
{
  "name": "existing_script.star",
  "status": "pending",
  "hash": "sha256:newupdatedhash..."
}
```

**Note:** Editing a script resets its status to `pending` (requires re-approval).

### DELETE /api/scripts/:name

Delete a script.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```
204 No Content
```

### POST /api/scripts/:name/approve

Approve a script for execution.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "name": "weather.star",
  "status": "approved",
  "approved_at": "2024-01-15T15:00:00Z",
  "approved_by": "admin"
}
```

### POST /api/scripts/:name/reject

Reject a script (blocks execution).

**Request Headers:**

```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request:**

```json
{
  "reason": "Script accesses unauthorized external API"
}
```

**Response:**

```json
{
  "name": "unsafe_script.star",
  "status": "rejected",
  "rejected_at": "2024-01-15T15:00:00Z",
  "rejected_by": "admin",
  "reason": "Script accesses unauthorized external API"
}
```

### POST /api/scripts/:name/test

Test-run an approved script.

**Request Headers:**

```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request:**

```json
{
  "args": {
    "city": "London"
  }
}
```

**Response (Success):**

```json
{
  "success": true,
  "result": {
    "city": "London",
    "temp_c": 15.5,
    "condition": "Partly cloudy"
  },
  "duration_ms": 150,
  "logs": [
    "Fetching weather for London",
    "API request successful"
  ]
}
```

**Response (Failure):**

```json
{
  "success": false,
  "error": "HTTP request failed: connection timeout",
  "duration_ms": 30000,
  "logs": [
    "Fetching weather for London",
    "API request timed out"
  ]
}
```

**Note:** Only approved scripts can be tested via this endpoint.

---

## Version History Endpoints

### GET /api/scripts/:name/history

Get version history for a script.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "versions": [
    {
      "commit": "abc123def456...",
      "message": "Update weather.star via admin UI",
      "author": "admin",
      "timestamp": "2024-01-15T14:30:00Z"
    },
    {
      "commit": "789ghi012jkl...",
      "message": "Create weather.star via admin UI",
      "author": "admin",
      "timestamp": "2024-01-15T10:00:00Z"
    }
  ]
}
```

### GET /api/scripts/:name/history/:commit

Get script source at a specific version.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "commit": "789ghi012jkl...",
  "source": "# @description: Original weather script\n..."
}
```

### GET /api/scripts/:name/diff

Get diff between two versions.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `from` | string | Starting commit hash |
| `to` | string | Ending commit hash |

**Request:**

```
GET /api/scripts/weather.star/diff?from=789ghi012jkl&to=abc123def456
```

**Response:**

```json
{
  "diff": "@@ -15,7 +15,7 @@ def get_weather(city):\n     url = format(\n-        \"https://api.v1...\",\n+        \"https://api.v2...\","
}
```

### POST /api/scripts/:name/restore/:commit

Restore a script to a previous version.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "name": "weather.star",
  "status": "pending",
  "restored_from": "789ghi012jkl..."
}
```

**Note:** Restoring creates a new commit and resets status to `pending`.

---

## Secrets Endpoints

Secret values are never returned via the API. Only metadata is provided.

### GET /api/secrets

List all configured secrets.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "secrets": [
    {
      "name": "WEATHER_API_KEY",
      "set": true,
      "last_updated": "2024-01-15T10:00:00Z"
    },
    {
      "name": "GITHUB_TOKEN",
      "set": true,
      "last_updated": "2024-01-14T09:00:00Z"
    },
    {
      "name": "SLACK_WEBHOOK",
      "set": false,
      "last_updated": null
    }
  ]
}
```

### POST /api/secrets/:name

Set a secret value.

**Request Headers:**

```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request:**

```json
{
  "value": "sk-your-api-key-here"
}
```

**Response:**

```json
{
  "name": "WEATHER_API_KEY",
  "set": true
}
```

**Note:** The actual secret value is never returned.

### DELETE /api/secrets/:name

Remove a secret.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```
204 No Content
```

---

## Session Endpoints

Session endpoints manage AI conversation sessions. OpenPact proxies these to the OpenCode server, which stores all session data internally. See the [OpenCode server documentation](https://opencode.ai/docs/server/) for details on the underlying API.

### GET /api/sessions

List all sessions with active status.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
[
  {
    "id": "ses_379f7b1c2ffekEa2VDmHmviVwS",
    "slug": "misty-eagle",
    "title": "Debugging the API endpoint",
    "directory": "/workspace",
    "version": "1.2.6",
    "time": {
      "created": 1771775217213,
      "updated": 1771775221546
    },
    "active": true
  }
]
```

The `active` field indicates which session currently receives Discord messages.

### POST /api/sessions

Create a new session and set it as active.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "id": "ses_new123abc",
  "slug": "bright-fox",
  "title": "",
  "time": {
    "created": 1771775300000,
    "updated": 1771775300000
  }
}
```

### GET /api/sessions/:id

Get details for a specific session.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "id": "ses_379f7b1c2ffekEa2VDmHmviVwS",
  "slug": "misty-eagle",
  "title": "Debugging the API endpoint",
  "directory": "/workspace",
  "version": "1.2.6",
  "time": {
    "created": 1771775217213,
    "updated": 1771775221546
  }
}
```

### DELETE /api/sessions/:id

Delete a session and all its messages.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "ok": true
}
```

### GET /api/sessions/:id/messages

Get message history for a session.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | `50` | Maximum number of messages to return |

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
[
  {
    "id": "msg_abc123",
    "sessionID": "ses_379f7b1c2ffekEa2VDmHmviVwS",
    "role": "user",
    "parts": [
      { "type": "text", "text": "Hello, can you help me?" }
    ],
    "time": {
      "created": 1771775220000,
      "updated": 1771775220000
    }
  }
]
```

### WS /api/sessions/:id/chat

WebSocket endpoint for real-time chat within a session. Authenticates via query parameter since browsers cannot set headers on WebSocket upgrades.

**Connection:**

```
ws://localhost:8080/api/sessions/:id/chat?token=<access_token>
```

**Client → Server:**

```json
{
  "type": "message",
  "content": "Hello, what can you help me with?"
}
```

**Server → Client:**

```json
{ "type": "connected", "session_id": "ses_abc123" }
{ "type": "text", "content": "I can help you with..." }
{ "type": "done" }
{ "type": "error", "content": "Engine error: ..." }
```

Messages are streamed incrementally as `text` events, followed by a `done` event when the response is complete.

---

## Model Endpoints

Model endpoints allow viewing available AI models and changing the default model used for new sessions. The preference is persisted to disk and survives restarts.

### GET /api/models

List all available models and the current default.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "models": [
    {
      "provider_id": "anthropic",
      "model_id": "claude-sonnet-4-20250514",
      "context_limit": 200000,
      "output_limit": 16000
    },
    {
      "provider_id": "anthropic",
      "model_id": "claude-opus-4-20250514",
      "context_limit": 200000,
      "output_limit": 32000
    },
    {
      "provider_id": "openai",
      "model_id": "gpt-4o",
      "context_limit": 128000,
      "output_limit": 16384
    }
  ],
  "default": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514"
  }
}
```

### PUT /api/models/default

Set the default model for new sessions.

**Request Headers:**

```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request:**

```json
{
  "provider": "anthropic",
  "model": "claude-opus-4-20250514"
}
```

**Response:**

```json
{
  "ok": true,
  "default": {
    "provider": "anthropic",
    "model": "claude-opus-4-20250514"
  }
}
```

**Errors:**

| Status | Description |
|--------|-------------|
| 400 | `model` field is missing |

:::note
Changing the default model only affects new sessions. Existing sessions continue using the model they were started with.
:::

---

## Schedule Endpoints

Schedule endpoints manage [cron-based scheduled jobs](/docs/features/scheduling). All endpoints require authentication via Bearer token. Mutations automatically trigger a scheduler reload.

### GET /api/schedules

List all schedules.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "schedules": [
    {
      "id": "a1b2c3d4e5f6g7h8",
      "name": "Daily report",
      "cron_expr": "0 9 * * 1-5",
      "type": "script",
      "enabled": true,
      "script_name": "daily_report.star",
      "output_target": {
        "provider": "discord",
        "channel_id": "channel:123456789"
      },
      "created_at": "2026-03-01T12:00:00Z",
      "updated_at": "2026-03-01T12:00:00Z",
      "last_run_at": "2026-03-01T09:00:00Z",
      "last_run_status": "success",
      "last_run_output": "Report generated successfully"
    }
  ]
}
```

### POST /api/schedules

Create a new schedule.

**Request Headers:**

```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request:**

```json
{
  "name": "Daily report",
  "cron_expr": "0 9 * * 1-5",
  "type": "script",
  "enabled": true,
  "script_name": "daily_report.star",
  "output_target": {
    "provider": "discord",
    "channel_id": "channel:123456789"
  }
}
```

**Required fields:** `name`, `cron_expr`, `type`

**Type-specific fields:**
- `type: "script"` requires `script_name`
- `type: "agent"` requires `prompt`

**Optional fields:**
- `run_once` (boolean) — If `true`, the schedule auto-disables after one execution

**Response (201 Created):**

```json
{
  "id": "a1b2c3d4e5f6g7h8",
  "name": "Daily report",
  "cron_expr": "0 9 * * 1-5",
  "type": "script",
  "enabled": true,
  "script_name": "daily_report.star",
  "output_target": {
    "provider": "discord",
    "channel_id": "channel:123456789"
  },
  "created_at": "2026-03-01T12:00:00Z",
  "updated_at": "2026-03-01T12:00:00Z"
}
```

**Errors:**

| Status | Description |
|--------|-------------|
| 400 | Invalid JSON, missing required fields, or invalid cron expression |

### GET /api/schedules/:id

Get a single schedule by ID.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "id": "a1b2c3d4e5f6g7h8",
  "name": "Daily report",
  "cron_expr": "0 9 * * 1-5",
  "type": "script",
  "enabled": true,
  "script_name": "daily_report.star",
  "output_target": {
    "provider": "discord",
    "channel_id": "channel:123456789"
  },
  "created_at": "2026-03-01T12:00:00Z",
  "updated_at": "2026-03-01T12:00:00Z",
  "last_run_at": "2026-03-01T09:00:00Z",
  "last_run_status": "success",
  "last_run_error": "",
  "last_run_output": "Report generated successfully"
}
```

**Errors:**

| Status | Description |
|--------|-------------|
| 404 | Schedule not found |

### PUT /api/schedules/:id

Update an existing schedule. Only provided fields are updated.

**Request Headers:**

```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request:**

```json
{
  "name": "Morning report",
  "cron_expr": "0 10 * * 1-5"
}
```

**Response:**

```json
{
  "id": "a1b2c3d4e5f6g7h8",
  "name": "Morning report",
  "cron_expr": "0 10 * * 1-5",
  "type": "script",
  "enabled": true,
  "script_name": "daily_report.star",
  "created_at": "2026-03-01T12:00:00Z",
  "updated_at": "2026-03-01T14:00:00Z"
}
```

**Errors:**

| Status | Description |
|--------|-------------|
| 400 | Invalid cron expression or invalid field values |
| 404 | Schedule not found |

### DELETE /api/schedules/:id

Delete a schedule.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```
204 No Content
```

**Errors:**

| Status | Description |
|--------|-------------|
| 404 | Schedule not found |

### POST /api/schedules/:id/enable

Enable a schedule.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "status": "enabled"
}
```

**Errors:**

| Status | Description |
|--------|-------------|
| 404 | Schedule not found |

### POST /api/schedules/:id/disable

Disable a schedule. The job stops running but its configuration is preserved.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "status": "disabled"
}
```

**Errors:**

| Status | Description |
|--------|-------------|
| 404 | Schedule not found |

### POST /api/schedules/:id/run

Trigger an immediate run of a schedule. The job executes in a background goroutine.

**Request Headers:**

```
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "status": "triggered"
}
```

**Errors:**

| Status | Description |
|--------|-------------|
| 400 | Invalid schedule or execution error |
| 404 | Schedule not found |
| 503 | Scheduler not available |

:::note
The run endpoint triggers the job asynchronously. The response confirms the job was triggered, not that it completed. Check the schedule's `last_run_*` fields for the result.
:::

---

## Error Responses

All error responses follow a consistent format:

```json
{
  "error": "error_code",
  "message": "Human-readable error description"
}
```

### Common Error Codes

| HTTP Status | Error Code | Description |
|-------------|------------|-------------|
| 400 | `invalid_request` | Malformed request body |
| 401 | `unauthorized` | Missing or invalid authentication |
| 403 | `forbidden` | Authenticated but not permitted |
| 404 | `not_found` | Resource doesn't exist |
| 409 | `conflict` | Resource already exists |
| 429 | `rate_limited` | Too many requests |
| 500 | `internal_error` | Server error |

### Script-Specific Errors

| Error Code | Description |
|------------|-------------|
| `script_not_found` | Script doesn't exist |
| `script_not_approved` | Cannot test unapproved script |
| `script_modified` | Script changed since approval |
| `invalid_script` | Script has syntax errors |

---

## Rate Limiting

The Admin API enforces rate limits to prevent abuse:

| Endpoint | Limit |
|----------|-------|
| `/api/auth/login` | 5 requests/minute |
| All other endpoints | 60 requests/minute |

**Rate Limit Headers:**

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1705323600
```

---

## CORS Configuration

CORS is restricted to same-origin by default. For development, you may need to configure allowed origins:

```yaml
admin:
  cors:
    allowed_origins:
      - "http://localhost:3000"
      - "http://localhost:5173"
```

---

## Security Headers

All Admin API responses include security headers:

```
Content-Security-Policy: default-src 'self'
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

---

## Example: Complete Workflow

### 1. Check Setup Status

```bash
curl http://localhost:8080/api/setup/status
# {"setup_required": true}
```

### 2. Complete Setup

```bash
curl -X POST http://localhost:8080/api/setup \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "my-secure-passphrase-here",
    "confirm_password": "my-secure-passphrase-here"
  }'
```

### 3. Login

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -c cookies.txt \
  -d '{"username": "admin", "password": "my-secure-passphrase-here"}'
```

### 4. Get Access Token

```bash
curl http://localhost:8080/api/session \
  -b cookies.txt
# {"access_token": "eyJ...", "expires_at": "...", "username": "admin"}
```

### 5. List Scripts

```bash
curl http://localhost:8080/api/scripts \
  -H "Authorization: Bearer eyJ..."
```

### 6. Approve a Script

```bash
curl -X POST http://localhost:8080/api/scripts/weather.star/approve \
  -H "Authorization: Bearer eyJ..."
```

---

## Related Documentation

- [Admin UI Overview](/docs/admin/overview) - Using the web interface
- [Authentication](/docs/admin/authentication) - Detailed auth configuration
- [Managing Scripts](/docs/admin/managing-scripts) - Script workflow guide
- [Secrets Management](/docs/admin/secrets-management) - Secrets best practices
- [Schedule Management](/docs/admin/schedule-management) - Schedule workflow guide
