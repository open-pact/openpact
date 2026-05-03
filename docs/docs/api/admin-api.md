---
sidebar_position: 3
title: Admin API
description: REST API for setup, authentication, scripts, secrets, sessions, providers, and schedules
---

# Admin API

The Admin API is everything served under `/api/*` by the OpenPact binary. Most endpoints require authentication via JWT bearer token. The admin UI calls these directly; you can call them from scripts and clients too.

## Base URL

```
http://localhost:8888/api
```

Configurable via `ADMIN_BIND` env var or `admin.bind` in `config.yaml` (defaults to `localhost:8888`; the Docker image exposes `0.0.0.0:8888`).

## Endpoint families

| Prefix | Owner | Purpose |
|--------|-------|---------|
| `/api/setup/*` | `internal/admin/setup.go` | First-run wizard. |
| `/api/auth/*`, `/api/session` | `internal/admin/session_auth.go` | Login, refresh, logout. |
| `/api/scripts*` | `internal/admin/scripts.go` | Starlark script CRUD + approval workflow. |
| `/api/secrets*` | `internal/admin/secrets.go` | Encrypted Starlark secret CRUD. |
| `/api/config/*` | `internal/admin/config_api.go` | Advanced settings + integrations (logging, rate limit, calendars, vault, GitHub). |
| `/api/engine/*` | Mounted [`web.ManagedHandler`](https://github.com/stack-bound/stackllm) | Provider login, model selection, SSE chat, individual session GET/DELETE. |
| `/api/engine/sessions` | `internal/admin/engine_sessions.go` | Paginated session list (we add this; stackllm only exposes individual sessions). |
| `/api/providers*` | `internal/admin/providers.go` | Discord/Slack/Telegram provider config. |
| `/api/schedules*` | `internal/admin/schedules.go` | Cron schedules (script + agent jobs). |

## Authentication

Two-token system:

| Token | Storage | Lifetime | Purpose |
|-------|---------|----------|---------|
| **Refresh token** | HTTP-only cookie (`Path=/api/session`) | 3 days | Obtain new access tokens |
| **Access token** | In-memory (JS) | 15 minutes | Bearer-token auth on all `/api/*` |

### Authentication Flow

```
1. POST /api/auth/login    → Set refresh cookie
2. GET  /api/session       → Exchange cookie for { access_token, ... }
3. Any /api/...            → Send `Authorization: Bearer <token>`
4. (token expires) → /api/session again to refresh
```

The `/api/engine/*` mount (which includes the chat SSE endpoint) uses the same Bearer-token middleware once setup is complete. During the setup wizard, a narrow whitelist (`isSetupEngineEndpoint`) lets the wizard call provider-login endpoints before the user has an admin account.

## Setup Endpoints

### GET /api/setup/status

Return whether setup is complete and what stage it's at.

**Response (incomplete):**

```json
{
  "setup_required": true,
  "account_complete": true,
  "profile_complete": false,
  "provider_complete": false
}
```

**Response (complete):**

```json
{
  "setup_required": false
}
```

### POST /api/setup

Create the first admin user. Issues a refresh-token cookie immediately so the wizard can proceed.

**Request:**

```json
{
  "username": "admin",
  "password": "your-secure-password",
  "confirm_password": "your-secure-password"
}
```

**Password requirements:**

- 16+ characters (passphrase style), **or**
- 12+ characters with 3 of 4: uppercase, lowercase, number, symbol.

**Response:**

```json
{ "success": true, "message": "Setup complete. Please log in." }
```

**Errors:**

| Status | Code | Description |
|--------|------|-------------|
| 400 | `password_invalid` | Password doesn't meet requirements |
| 400 | `password_mismatch` | Passwords don't match |
| 403 | `setup_completed` | Setup already completed |

### POST /api/setup/profile

Mark the profile step (SOUL/USER/MEMORY) of the wizard complete. Requires `Authorization: Bearer <token>`.

```json
{ "soul": "...", "user": "...", "memory": "..." }
```

### POST /api/setup/provider

Mark the provider-login step complete. The handler verifies a default model has been set in stackllm before flipping `setup_state.provider_complete`. Requires `Authorization: Bearer <token>`.

```json
{}   // no body — the handler queries stackllm.Manager.Default()
```

## Authentication Endpoints

### POST /api/auth/login

```json
{ "username": "admin", "password": "your-password" }
```

Sets a refresh cookie (HttpOnly, Secure, `Path=/api/session`, `Max-Age=259200`). Body returns `{ "message": "Login successful" }`.

| Status | Code | Description |
|--------|------|-------------|
| 401 | `invalid_credentials` | Wrong username/password |
| 429 | `rate_limited` | Login rate limit (5/min) |

### GET /api/session

Exchange the refresh cookie for an access token.

**Response:**

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2026-04-25T10:45:00Z",
  "username": "admin"
}
```

### POST /api/auth/logout

Clears the refresh cookie. `204 No Content`.

### GET /api/auth/me

```json
{ "username": "admin", "role": "admin" }
```

## Engine Endpoints (`/api/engine/*`)

The `/api/engine/*` subtree is mounted as a `http.StripPrefix`-wrapped `web.ManagedHandler` from stackllm. It is the single source of truth for provider login, model selection, and chat. We add one wrapper endpoint (`GET /api/engine/sessions`) that the upstream handler doesn't provide.

All endpoints require `Authorization: Bearer <token>` once the setup wizard is complete; during step 3 of the wizard, a narrow whitelist lets the user sign in to a first provider before having a JWT.

### GET /api/engine/providers

List providers and authentication status.

```json
{
  "providers": [
    { "name": "openai",   "type": "api_key",     "authenticated": true  },
    { "name": "gemini",   "type": "api_key",     "authenticated": false },
    { "name": "copilot",  "type": "device_flow", "authenticated": false },
    { "name": "ollama",   "type": "base_url",    "authenticated": false }
  ]
}
```

### POST /api/engine/providers/openai/login, POST /api/engine/providers/gemini/login

API-key login.

```json
{ "key": "sk-..." }
```

Returns `{ "ok": true }` or an error.

### POST /api/engine/providers/ollama/login

```json
{ "base_url": "http://localhost:11434" }
```

### POST /api/engine/providers/openai/oauth/login

Start the Codex "Sign in with ChatGPT" device flow. Returns:

```json
{
  "user_code": "ABCD-1234",
  "verify_url": "https://chat.openai.com/auth/...",
  "status": "pending"
}
```

### GET /api/engine/providers/openai/oauth/status

```json
{ "user_code": "ABCD-1234", "verify_url": "...", "status": "pending|authenticated|error" }
```

Poll at ~2-second intervals from the UI until `status` flips. The handler serializes one device flow per provider; calling the start endpoint while one is in flight returns the existing code rather than minting a new one.

### POST /api/engine/providers/copilot/login

Start the GitHub Copilot device flow. Same response shape as OpenAI OAuth.

### GET /api/engine/providers/copilot/status

Poll until `status` is `authenticated`.

### POST /api/engine/providers/:name/logout

Sign out and remove the credentials from `stackllm_auth.json`.

### GET /api/engine/models

List models across all authenticated providers.

```json
{
  "models": [
    { "provider": "openai",  "model": "gpt-4o",          "context_limit": 128000, "output_limit": 16384 },
    { "provider": "openai",  "model": "gpt-4o-mini",     "context_limit": 128000, "output_limit": 16384 },
    { "provider": "gemini",  "model": "gemini-2.0-flash", "context_limit": 1000000, "output_limit": 8192 }
  ]
}
```

### GET /api/engine/models/:provider

Filter to one provider.

### GET /api/engine/default

```json
{ "set": true, "provider": "openai", "model": "gpt-4o", "endpoint": "" }
```

`set: false` means no default has been chosen yet — the setup wizard refuses to finish in that case.

### POST /api/engine/default

```json
{ "provider": "openai", "model": "gpt-4o", "endpoint": "" }
```

`endpoint` is only relevant for Ollama (override the base URL per-default).

### POST /api/engine/chat

Server-Sent Events stream. Body:

```json
{
  "session_id": "",
  "message": {
    "role": "user",
    "blocks": [{ "type": "text", "text": "Hello!" }]
  }
}
```

Pass an empty `session_id` to start a fresh session. The response stream:

```
event: block_start
data: {"block_type":"text"}

event: block_delta
data: {"block_type":"text","delta":"Hello"}

event: block_delta
data: {"block_type":"text","delta":"! How can I help?"}

event: block_end
data: {"block_type":"text"}

event: done
data: {"session_id":"7c2a5e1d-..."}
```

`block_type` may be `text`, `thinking`, `tool_use`, or `tool_result`. The admin UI's **Sessions** view filters which block types it renders based on the user's detail-mode toggle.

`event: error` is emitted with `{ "error": "..." }` and the stream terminates.

### GET /api/engine/sessions

Paginated session list.

**Query parameters:**

| Param | Default | Description |
|-------|---------|-------------|
| `limit` | `50` | Page size (max `200`) |
| `offset` | `0` | Pagination offset |

**Response:**

```json
{
  "sessions": [
    {
      "id": "7c2a5e1d-...",
      "name": "Discord: general",
      "created_at": 1700000000000,
      "updated_at": 1700000003000
    }
  ],
  "total": 17,
  "limit": 50,
  "offset": 0
}
```

Sessions created by chat providers carry `name` like `"Discord: <channel>"`, `"Slack: <channel>"`, or `"Telegram: <chat>"`. Scheduler-created sessions are named `"Scheduled: <first 40 chars of prompt>"`.

This endpoint is OpenPact's wrapper around stackllm's `session.SessionPaginator.ListPage` (added in stackllm v0.4.1). The upstream `web.ManagedHandler` does not expose a list endpoint.

### GET /api/engine/sessions/:id

Fetch a single session including its message history.

### DELETE /api/engine/sessions/:id

Permanently delete a session.

## Script Endpoints

All script endpoints require Bearer-token auth.

### GET /api/scripts

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
      "approved_at": "2026-04-15T10:30:00Z",
      "approved_by": "admin"
    }
  ]
}
```

Statuses: `pending`, `approved`, `rejected`.

### GET /api/scripts/:name

Returns full source plus metadata and execution history.

### POST /api/scripts

```json
{
  "name": "new_script.star",
  "source": "# @description: My new script\n\ndef main():\n    return {\"message\": \"Hello\"}"
}
```

New scripts always start `pending`.

### PUT /api/scripts/:name

Update an existing script. Editing resets the status to `pending` (re-approval required).

### DELETE /api/scripts/:name

`204 No Content`.

### POST /api/scripts/:name/approve

```json
{ "name": "weather.star", "status": "approved", "approved_at": "...", "approved_by": "admin" }
```

### POST /api/scripts/:name/reject

```json
{ "reason": "Script accesses unauthorized external API" }
```

### POST /api/scripts/:name/test

Run an approved script.

```json
{ "args": { "city": "London" } }
```

Response on success:

```json
{ "success": true, "result": { ... }, "duration_ms": 150, "logs": [...] }
```

### Version history

- `GET /api/scripts/:name/history` — list versions.
- `GET /api/scripts/:name/history/:commit` — fetch source at a version.
- `GET /api/scripts/:name/diff?from=...&to=...` — diff.
- `POST /api/scripts/:name/restore/:commit` — restore (creates a new commit, status reverts to `pending`).

## Secrets Endpoints

Secret values are never returned via the API.

### GET /api/secrets

```json
{
  "secrets": [
    { "name": "WEATHER_API_KEY", "set": true,  "last_updated": "2026-04-15T10:00:00Z" },
    { "name": "GITHUB_TOKEN",    "set": true,  "last_updated": "2026-04-14T09:00:00Z" },
    { "name": "SLACK_WEBHOOK",   "set": false, "last_updated": null }
  ]
}
```

### POST /api/secrets/:name

```json
{ "value": "sk-your-api-key-here" }
```

Stored AES-256-GCM encrypted in `op_secrets` under the `data_encryption_key`.

### DELETE /api/secrets/:name

`204 No Content`.

## Configuration Endpoints (`/api/config/*`)

### GET / PUT /api/config/advanced

Logging level + JSON toggle, rate limiter rate/burst, health server bind, Starlark limits.

```json
{
  "logging":   { "level": "info", "json": false },
  "rate_limit":{ "rate": 10, "burst": 20 },
  "health":    { "bind": ":8081" },
  "starlark":  { "max_execution_ms": 30000, "max_memory_mb": 128 }
}
```

Logger / rate limiter / health server read these once at boot — restart required for changes to take effect.

### GET / PUT /api/config/integrations

Calendar feeds, Obsidian vault, GitHub toggle.

```json
{
  "calendars": [
    { "name": "Personal", "url": "https://..." }
  ],
  "vault":  { "path": "/vault", "git_repo": "git@github.com:user/vault.git", "auto_sync": true },
  "github": { "enabled": true }
}
```

## Provider Endpoints (`/api/providers*`)

Configure Discord, Slack, Telegram. Tokens are persisted to `op_chat_providers`.

### GET /api/providers

List of providers with status, allow lists, but **without** revealing tokens.

### POST /api/providers/:name

Create or update a provider.

```json
{
  "enabled": true,
  "token": "your-token",            // discord/telegram
  "bot_token": "xoxb-...",          // slack
  "app_token": "xapp-...",          // slack
  "allowed_users": ["123456..."],
  "allowed_chans": ["chan-id"]
}
```

### POST /api/providers/:name/start, POST /api/providers/:name/stop

Toggle a provider without changing its config.

## Schedule Endpoints

Mutations automatically reload the in-memory cron scheduler.

### GET /api/schedules

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
      "output_target": { "provider": "discord", "channel_id": "channel:123456789" },
      "created_at": "2026-04-01T12:00:00Z",
      "updated_at": "2026-04-01T12:00:00Z",
      "last_run_at": "2026-04-15T09:00:00Z",
      "last_run_status": "success",
      "last_run_output": "Report generated successfully"
    }
  ]
}
```

### POST /api/schedules

```json
{
  "name": "Daily report",
  "cron_expr": "0 9 * * 1-5",
  "type": "script",
  "enabled": true,
  "script_name": "daily_report.star",
  "output_target": { "provider": "discord", "channel_id": "channel:123456789" }
}
```

`type: "script"` requires `script_name`. `type: "agent"` requires `prompt`. Optional `run_once: true` auto-disables after one execution.

`201 Created` on success.

### GET / PUT / DELETE /api/schedules/:id

Standard CRUD. PUT does partial updates.

### POST /api/schedules/:id/enable, POST /api/schedules/:id/disable

```json
{ "status": "enabled" }
```

### POST /api/schedules/:id/run

Trigger immediate execution in the background.

```json
{ "status": "triggered" }
```

Check `last_run_*` fields in `GET /api/schedules/:id` for the result.

## Error Responses

```json
{ "error": "error_code", "message": "Human-readable description" }
```

| HTTP | Error | Description |
|------|-------|-------------|
| 400 | `invalid_request` | Malformed body |
| 401 | `unauthorized` | Missing or invalid auth |
| 403 | `forbidden` | Authenticated but not permitted |
| 404 | `not_found` | Resource missing |
| 409 | `conflict` | Resource already exists |
| 429 | `rate_limited` | Too many requests |
| 500 | `internal_error` | Server error |

## Rate Limiting

| Endpoint | Limit |
|----------|-------|
| `/api/auth/login` | 5 / minute |
| All other endpoints | configurable in `/settings/advanced` (default 60 / min) |

Headers:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1745323600
```

## Security Headers

```
Content-Security-Policy: default-src 'self'
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

## Example: Complete Workflow

### 1. Check setup status

```bash
curl http://localhost:8888/api/setup/status
# {"setup_required": true, "account_complete": false, ...}
```

### 2. Create the admin user

```bash
curl -X POST http://localhost:8888/api/setup \
  -H "Content-Type: application/json" \
  -c cookies.txt \
  -d '{
    "username": "admin",
    "password": "my-secure-passphrase-here",
    "confirm_password": "my-secure-passphrase-here"
  }'
```

The refresh cookie is set automatically.

### 3. Get an access token

```bash
TOKEN=$(curl -s http://localhost:8888/api/session -b cookies.txt | jq -r .access_token)
```

### 4. Sign in to OpenAI

```bash
curl -X POST http://localhost:8888/api/engine/providers/openai/login \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"sk-..."}'
```

### 5. Pick a default model

```bash
curl http://localhost:8888/api/engine/models -H "Authorization: Bearer $TOKEN"

curl -X POST http://localhost:8888/api/engine/default \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"provider":"openai","model":"gpt-4o","endpoint":""}'
```

### 6. Mark the wizard complete

```bash
curl -X POST http://localhost:8888/api/setup/provider \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### 7. Send a chat message

```bash
curl -N -X POST http://localhost:8888/api/engine/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "",
    "message": {"role":"user","blocks":[{"type":"text","text":"Hello!"}]}
  }'
# Server-Sent Events stream of block_delta + done events.
```

## Related Documentation

- [Admin UI Overview](/docs/admin/overview) — using the web interface.
- [Authentication](/docs/admin/authentication) — detailed auth configuration.
- [Managing Scripts](/docs/admin/managing-scripts) — script workflow guide.
- [Secrets Management](/docs/admin/secrets-management) — secrets best practices.
- [Schedule Management](/docs/admin/schedule-management) — schedule workflow guide.
