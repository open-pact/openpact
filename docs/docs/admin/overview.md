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
│                     Go Application                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   MCP Server │  │  Admin API   │  │  Static File Server  │  │
│  │   (JSON-RPC) │  │  (REST/JSON) │  │  (Embedded SPA)      │  │
│  │   :3000      │  │  :8080/api   │  │  :8080/              │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
│                           │                    │                 │
│                           ▼                    ▼                 │
│                    ┌─────────────────────────────┐              │
│                    │      JWT Middleware         │              │
│                    └─────────────────────────────┘              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

The Admin UI consists of:

- **Vue 3 SPA** - Single-page application embedded in the Go binary
- **REST API** - JSON-based API endpoints under `/api/`
- **JWT Authentication** - Secure token-based authentication

## First-Run Setup

On first launch, if no admin user exists, OpenPact enters **setup mode**. During setup:

- The MCP server does not start
- All API endpoints return `503 Service Unavailable`
- Users are redirected to the `/setup` page

### Setup Process

1. Navigate to `http://localhost:8080/setup`
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

- The setup endpoint is permanently disabled
- The MCP server starts accepting connections
- You can log in with your new credentials

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

Enable the Admin UI in your `openpact.yaml`:

```yaml
admin:
  enabled: true
  bind: "0.0.0.0:8080"

  jwt:
    access_expiry: "15m"    # Short-lived access tokens
    refresh_expiry: "72h"   # 3-day refresh tokens
    issuer: "openpact"

  # Optional: Restrict to specific IP ranges
  allowed_ips:
    - "10.0.0.0/8"
    - "192.168.0.0/16"
```

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
| Sessions | `/sessions` | Manage AI conversation sessions (create, switch, delete, chat) |
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
