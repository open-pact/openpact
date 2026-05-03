---
sidebar_position: 4
title: Docker Security
description: Single-user container, filesystem layout, hardening recipes
---

# Docker Security

OpenPact ships as a single static binary inside a slim Debian-based image. There is no Node, no separate AI service, and no two-user split — everything runs as one unprivileged user (`openpact`) in one process.

## Single-User Model

```
Container UID/GID: openpact (system user, no shell)
Single Go process: /app/openpact start
```

The image is built from `debian:bookworm-slim` with `ca-certificates` and `git` installed (`git` is needed by the Obsidian-vault auto-sync path; `ca-certificates` for outbound TLS to provider APIs). It does **not** ship a shell for the runtime user. The binary is `chmod 755`, owned by `root:root`, and run as `openpact`.

There is no entrypoint script. The Dockerfile's `ENTRYPOINT` is `["/app/openpact"]` and the default `CMD` is `["start"]`.

## Filesystem Layout

```
Container Filesystem
├── /app/                    # Application binaries (root-owned, world-readable)
│   ├── openpact             # Main binary
│   ├── mcp-server           # Stdio MCP server (optional, for external clients)
│   └── templates/           # Default config/context templates
├── /home/openpact/          # System user home (created by useradd --system)
└── /workspace/              # Volume mount, writable by `openpact`
    ├── secure/                       # 0700 — system-only
    │   ├── config.yaml               # Bootstrap-only YAML
    │   └── data/                     # 0700
    │       ├── jwt_secret            # JWT signing key
    │       ├── data_encryption_key   # AES-256 key for op_secrets
    │       ├── stackllm_auth.json    # 0600 — provider tokens
    │       ├── stackllm_config.json  # Default model + recent models
    │       └── stackllm.db           # 0600 — shared SQLite (op_* + stackllm_*)
    └── ai-data/                      # 0755 — AI-accessible
        ├── SOUL.md
        ├── USER.md
        ├── MEMORY.md
        ├── memory/                   # Daily memory files
        ├── scripts/                  # Starlark scripts
        └── skills/                   # Skill definitions
```

The `secure/` and `secure/data/` directories are created with mode `0700` by `EnsureDirs` on first boot; `ai-data/` and its children are `0755`. The DB file is chmodded `0600` after open. There is no group-permission acrobatics — the AI agent is a function call inside the same process, so the OS-level boundary that used to live between two users is now an in-process boundary at the MCP tool registry.

## Why a Single User Is Still Safe

The pre-stackllm architecture used two users to prevent a compromised LLM process from reading `secure/`. With stackllm running in-process there is no separate LLM process to compromise — the agent loop is a Go function call inside the same binary. The relevant security boundary moved up:

| Old layer | New layer |
|-----------|-----------|
| Linux UID separation between orchestrator and `opencode serve` | Function-level separation: the agent can only call tools registered in `tools.Registry`. |
| `OPENCODE_CONFIG_CONTENT` disabling built-in tools | The registry only contains the tools `mcp.RegisterAllTools` populated. Anything not registered is unreachable. |
| Filtered env passed to the AI subprocess | No subprocess. The agent doesn't see env vars; provider tokens live in `stackllm_auth.json` (0600). |
| Container UID `openpact-ai` blocked from `secure/` | Workspace MCP tools are scoped to `ai-data/` in code (`internal/mcp/workspace_tools.go`). |

The agent literally cannot call a tool that wasn't registered, and the registered tools all enforce the `ai-data/` boundary in their own code.

## Hardening Recipes

### Read-only root filesystem

```yaml
services:
  openpact:
    image: ghcr.io/open-pact/openpact:latest
    read_only: true
    tmpfs:
      - /tmp:size=64M,mode=1777
    volumes:
      - openpact-workspace:/workspace
```

The binary doesn't write to `/` — only `/workspace`. With `read_only: true` and a writable volume on `/workspace`, the rest of the rootfs is immutable.

### Drop all capabilities

```yaml
services:
  openpact:
    cap_drop:
      - ALL
```

OpenPact doesn't need any Linux capabilities — it binds an unprivileged port (`8888`), reads/writes inside `/workspace`, and makes outbound HTTPS calls.

### Disallow privilege escalation

```yaml
services:
  openpact:
    security_opt:
      - no-new-privileges:true
```

### Resource limits

```yaml
services:
  openpact:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1024M
```

Memory should accommodate the SQLite working set + admin UI assets + any large MCP tool responses. 512 MB is enough for a quiet single-user instance; 1 GB gives some headroom.

## Network Exposure

Only the admin UI port is exposed. There is no inter-process port any more — stackllm runs in-process, and the optional stdio MCP server (`/app/mcp-server`) speaks JSON-RPC over stdin/stdout if you ever launch it for an external client.

```yaml
services:
  openpact:
    ports:
      - "127.0.0.1:8888:8888"   # Admin UI on localhost only
```

For internet exposure, put a TLS-terminating reverse proxy (nginx, Caddy, Traefik) in front of `:8888`.

## Environment Variables

The container only reads bootstrap env vars: `WORKSPACE_PATH`, `CONFIG_PATH`, `ADMIN_BIND`, `ADMIN_JWT_SECRET`, and the chat-provider / GitHub fallback tokens. LLM provider tokens are **never** passed in via env vars — they're written to `stackllm_auth.json` (0600) by the admin UI sign-in flow.

```yaml
services:
  openpact:
    environment:
      ADMIN_BIND: "0.0.0.0:8888"
      DISCORD_TOKEN: "${DISCORD_TOKEN}"   # Optional fallback
      GITHUB_TOKEN: "${GITHUB_TOKEN}"     # Optional fallback
```

Add to a `.env` file (not committed):

```bash
DISCORD_TOKEN=your-discord-token
GITHUB_TOKEN=your-github-pat
```

## Complete Production Compose

```yaml
services:
  openpact:
    image: ghcr.io/open-pact/openpact:latest
    container_name: openpact

    read_only: true
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL

    tmpfs:
      - /tmp:size=64M,mode=1777

    volumes:
      - openpact-workspace:/workspace

    environment:
      ADMIN_BIND: "0.0.0.0:8888"
      DISCORD_TOKEN: "${DISCORD_TOKEN}"
      GITHUB_TOKEN: "${GITHUB_TOKEN}"

    ports:
      - "127.0.0.1:8888:8888"

    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1024M

    healthcheck:
      test: ["CMD-SHELL", "wget -q --spider http://localhost:8081/healthz || exit 1"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

    restart: unless-stopped

volumes:
  openpact-workspace:
```

Pair it with a reverse proxy (nginx, Caddy, Traefik) terminating TLS in front of `:8888`.

## Verification

### Process user

```bash
docker exec openpact ps -o user,pid,cmd -A
# Expected: only one openpact process, owned by user `openpact`.
```

### File permissions

```bash
docker exec openpact ls -la /workspace
# secure/ should be drwx------ (700), owned by openpact:openpact

docker exec openpact ls -la /workspace/secure
# config.yaml should be -rw------- (600)
# data/ should be drwx------ (700)

docker exec openpact ls -la /workspace/secure/data
# stackllm_auth.json should be -rw------- (600)
# stackllm.db should be -rw------- (600)
```

### Health check

```bash
curl http://localhost:8081/healthz
# 200 OK + {"status":"ok"}
```

(The health server bind defaults to `:8081` and is configurable in `/settings/advanced`.)

## Security Checklist

### Build time

- [ ] Use the official image (`ghcr.io/open-pact/openpact`) or pin to a specific tag.
- [ ] Verify the image runs as the `openpact` user (`docker inspect` → `Config.User`).

### Runtime

- [ ] Read-only rootfs (`read_only: true`).
- [ ] All capabilities dropped (`cap_drop: [ALL]`).
- [ ] `no-new-privileges:true` set.
- [ ] CPU/memory limits set.
- [ ] Workspace volume is mounted with appropriate UID:GID for `openpact` (UID assigned by `useradd --system`).
- [ ] TLS in front (reverse proxy).
- [ ] Admin UI bound to localhost or behind authenticated reverse proxy if exposed.

### Monitoring

- [ ] Health checks configured.
- [ ] Logs ingested somewhere durable.
- [ ] File permissions verified post-deploy (`secure/` 0700, `stackllm_auth.json` 0600).
- [ ] JWT secret rotated periodically (delete `secure/data/jwt_secret`; OpenPact regenerates on next boot — all sessions are forced to re-login).
