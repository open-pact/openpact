---
sidebar_position: 1
title: Overview
description: Introduction to the OpenPact Admin UI
---

# Admin UI Overview

OpenPact includes a web-based administration interface for managing scripts, secrets, and approval workflows. The Admin UI is served directly from the same Go application, requiring no separate deployment.

## What Admin UI Provides

The Admin UI enables administrators to:

- **Review and approve scripts** - Before AI-generated scripts can execute, they must be reviewed and approved
- **Manage secrets** - Add, update, and remove API keys and other sensitive credentials
- **Monitor system health** - View uptime, execution statistics, and script status
- **Track script versions** - Git-backed version history with diff viewing and rollback capability
- **Test scripts safely** - Execute approved scripts with test parameters before production use

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                  OpenPact (single binary)                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Admin Web Server (port 8888 by default)                  │   │
│  │                                                            │   │
│  │  /api/...                  Embedded SPA (Vue 3)           │   │
│  │  /api/engine/...     →  stackllm web.ManagedHandler        │   │
│  │  /api/setup,                                               │   │
│  │   /auth, /scripts,                                         │   │
│  │   /secrets, /config,                                       │   │
│  │   /providers,                                              │   │
│  │   /schedules           ← internal/admin handlers           │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              │                                   │
│                              ▼                                   │
│              ┌─────────────────────────────────┐                │
│              │   JWT middleware (HS256)         │                │
│              └─────────────────────────────────┘                │
│                                                                  │
│  Health server: separate port (default :8081, configurable      │
│                  in /settings/advanced)                          │
└─────────────────────────────────────────────────────────────────┘
```

The Admin UI consists of:

- **Vue 3 SPA** — embedded into the Go binary via `//go:embed`.
- **REST API** — JSON endpoints under `/api/*`.
- **`/api/engine/*` mount** — stackllm's `web.ManagedHandler` provides provider login, model selection, SSE chat, individual session GET/DELETE.
- **JWT auth** — short-lived access tokens (15 min) + refresh-cookie rotation (3-day TTL).

## First-Run Setup

On first launch, if no admin user exists, OpenPact enters **setup mode**. During setup:

- All `/api/*` endpoints return `503 Service Unavailable` except a narrow whitelist (`/api/setup/*`, the wizard's provider-login subset of `/api/engine/*`).
- Users are redirected to the `/setup` page.

### Setup Process (3 steps)

1. **Account** — open `http://localhost:8888/setup` and create the first admin user. The refresh cookie is set immediately so the rest of the wizard can call authenticated endpoints.
2. **Profile** — fill in `SOUL.md`, `USER.md`, `MEMORY.md`.
3. **LLM provider** — sign in to OpenAI / Copilot / Gemini / Ollama and pick a default model. The wizard refuses to finish until a default is set.

You can rerun any step from the admin UI later (`/engine` for providers, `/profile` for context files), but the initial wizard runs once.

The flow:

1. Navigate to `http://localhost:8888/setup`
2. Choose a username (default: `admin`)
3. Create a password meeting the policy requirements
4. Confirm the password
5. Click "Complete Setup"

```
┌─────────────────────────────────────────────────────────────────┐
│                    OpenPact First-Time Setup                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Welcome! Let's secure your installation.                        │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │  Username                                                   │ │
│  │  ┌──────────────────────────────────────────────────────┐  │ │
│  │  │ admin                                                 │  │ │
│  │  └──────────────────────────────────────────────────────┘  │ │
│  │                                                             │ │
│  │  Password                                                   │ │
│  │  ┌──────────────────────────────────────────────────────┐  │ │
│  │  │ ••••••••••••••••                                     │  │ │
│  │  └──────────────────────────────────────────────────────┘  │ │
│  │  ✓ 16+ characters OR 12+ with mixed case/numbers/symbols   │ │
│  │                                                             │ │
│  │  Confirm Password                                           │ │
│  │  ┌──────────────────────────────────────────────────────┐  │ │
│  │  │ ••••••••••••••••                                     │  │ │
│  │  └──────────────────────────────────────────────────────┘  │ │
│  │                                                             │ │
│  │  ┌──────────────────────────────────────────────────────┐  │ │
│  │  │              Complete Setup                          │  │ │
│  │  └──────────────────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

After setup completes:

- The setup endpoint is permanently disabled.
- The agent endpoint (`/api/engine/chat`) becomes generally callable behind the standard JWT middleware.
- You can log in with your new credentials.

## Password Policy

Passwords must meet one of these requirements:

| Option | Length | Requirements |
|--------|--------|--------------|
| Long password | 16+ characters | None - passphrase style encouraged |
| Complex password | 12+ characters | Must include 3 of 4: uppercase, lowercase, numbers, symbols |

Examples of valid passwords:

- `correct horse battery staple` (16+ characters, passphrase)
- `MySecure123!` (12 characters, mixed types)
- `P@ssword2024!!` (14 characters, all 4 types)

## Configuration

The admin UI is enabled by default. Override the bind address from `secure/config.yaml`:

```yaml
admin:
  enabled: true
  bind: "0.0.0.0:8888"
  allowlist: []           # Always-approved Starlark scripts
```

Or via env var:

```bash
ADMIN_BIND=0.0.0.0:8888
```

JWT lifetimes (access: 15 min, refresh: 3 days), the JWT signing algorithm (HS256), and IP allowlisting are not surfaced as YAML/env-var knobs — the values are baked in. If you need IP-level restriction, terminate at a reverse proxy.

:::warning Production Security
In production, always run the Admin UI behind a reverse proxy with HTTPS. The Admin UI uses secure cookies that require HTTPS in non-localhost environments.
:::

## UI Pages

The Admin UI includes the following pages:

| Page | Path | Description |
|------|------|-------------|
| Setup | `/setup` | First-run configuration (only available once) |
| Login | `/login` | Username and password authentication |
| Dashboard | `/` | Overview statistics, pending scripts alert |
| Sessions | `/sessions` | Manage AI conversation sessions (create, switch, delete, chat) and change the default AI model |
| Scripts | `/scripts` | List all scripts with status badges |
| Script Editor | `/scripts/:name` | View, edit, approve, reject, and test scripts |
| Secrets | `/secrets` | Manage API keys and credentials |

## Single Binary Deployment

The Admin UI is embedded in the Go binary at compile time using `go:embed`. This means:

- No separate static file deployment needed
- Single binary contains everything
- UI assets are served from memory
- Works offline after initial load

## Next Steps

- [Authentication](./authentication) - Set up admin password and understand session management
- [Managing Scripts](./managing-scripts) - Learn the script approval workflow
- [Secrets Management](./secrets-management) - Securely manage API keys
