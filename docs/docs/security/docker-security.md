---
sidebar_position: 4
title: Docker Security
description: Two-user model, container isolation, and volume mounts
---

# Docker Security

OpenPact is designed to run securely in Docker containers. This page covers the security architecture of containerized deployments, including the two-user model, filesystem isolation, and best practices.

## Two-User Model

OpenPact uses a two-user model in Docker to separate the privileged orchestrator from the restricted AI engine:

### User Roles

| User | Purpose | Permissions |
|------|---------|-------------|
| `root` | Container initialization only | Entrypoint sets file permissions, then drops privileges |
| `openpact-system` | Orchestrator, admin UI, secrets management | Owns all files, runs main process |
| `openpact-ai` | AI engine (OpenCode), MCP tools | Group member, restricted file access |

Both `openpact-system` and `openpact-ai` are members of the `openpact` group. File permissions use group membership to give the AI user controlled access.

### Why Two Users?

1. **Privilege separation** -- The AI process cannot access secrets, config, or anything under `secure/`
2. **Defense in depth** -- Even if the AI bypasses MCP tool restrictions, Linux permissions prevent access to `secure/`
3. **Container escape mitigation** -- A compromised AI process has minimal privileges
4. **Auditable** -- `ps aux` shows which user each process runs as

### How It Works

1. Container starts as `root` (entrypoint only)
2. `docker-entrypoint.sh` creates directories and sets file permissions
3. Entrypoint drops to `openpact-system` via `gosu`
4. Orchestrator spawns OpenCode with `SysProcAttr.Credential` set to `openpact-ai`
5. MCP server binary inherits `openpact-ai` UID from its parent (OpenCode)

## File Permission Model

The entrypoint sets these permissions at container startup:

```
/workspace/                       750  openpact-system:openpact  # AI can traverse
/workspace/secure/                700  openpact-system:openpact  # AI CANNOT access
/workspace/secure/config.yaml     600  openpact-system:openpact  # AI CANNOT access
/workspace/secure/data/           700  openpact-system:openpact  # AI CANNOT access
/workspace/ai-data/               750  openpact-system:openpact  # AI can traverse
/workspace/ai-data/memory/        770  openpact-system:openpact  # AI can read+write
/workspace/ai-data/scripts/       750  openpact-system:openpact  # AI can read
/workspace/ai-data/skills/        750  openpact-system:openpact  # AI can read
/workspace/ai-data/SOUL.md        640  openpact-system:openpact  # AI can read
/workspace/ai-data/USER.md        640  openpact-system:openpact  # AI can read
/workspace/ai-data/MEMORY.md      660  openpact-system:openpact  # AI can read+write
```

### What This Prevents

| Attack | Prevention |
|--------|-----------|
| AI reads secrets from `secure/data/` | 700 permission on `secure/` blocks group access |
| AI reads `secure/config.yaml` (may contain passwords) | 700 permission on `secure/` blocks group access |
| AI modifies SOUL.md directly | 640 permission blocks group write |
| AI writes to scripts dir | 750 permission blocks group write |
| AI reads/writes memory | 770/660 permits via MCP tools |

## Container Isolation

### Filesystem Layout

```
Container Filesystem
├── /app/                    # Application binaries
│   ├── openpact             # Orchestrator binary
│   ├── mcp-server           # Standalone MCP server binary
│   └── templates/           # Default config/context templates
├── /home/
│   ├── openpact-system/     # System user home
│   │   └── .local/share/opencode -> /workspace/secure/data/opencode
│   └── openpact-ai/         # AI user home
│       └── .local/share/opencode -> /workspace/secure/data/opencode
└── /workspace/              # Bind-mounted workspace volume
    ├── secure/              # SYSTEM-ONLY — AI has ZERO access
    │   ├── config.yaml      # Configuration (owner-only)
    │   └── data/            # Secrets, JWT key, approvals (owner-only)
    └── ai-data/             # AI-ACCESSIBLE — MCP tools scope here
        ├── SOUL.md          # AI persona (group-readable)
        ├── USER.md          # User profile (group-readable)
        ├── MEMORY.md        # Long-term memory (group-read/write)
        ├── memory/          # Daily memory files (group-writable)
        ├── scripts/         # Starlark scripts (group-readable)
        └── skills/          # Skill definitions (group-readable)
```

### Read-Only Root Filesystem

For maximum security, run with a read-only root filesystem:

```yaml
# docker-compose.yml
services:
  openpact:
    image: openpact:latest
    read_only: true
    tmpfs:
      - /tmp:size=64M,mode=1777
    volumes:
      - ./workspace:/workspace
```

## Environment Variable Security

### AI Process Environment

The AI process (OpenCode) receives a filtered environment. Only allowlisted variables pass through:

**Included:** `PATH`, `HOME`, `USER`, `LANG`, `TERM`, `TZ`, `TMPDIR`, `XDG_*`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`, `AZURE_OPENAI_API_KEY`, `OLLAMA_HOST`

**Excluded:** `DISCORD_TOKEN`, `GITHUB_TOKEN`, `SLACK_BOT_TOKEN`, `TELEGRAM_BOT_TOKEN`, `ADMIN_JWT_SECRET`, and all other environment variables.

### Sensitive Variables

```yaml
# docker-compose.yml
services:
  openpact:
    environment:
      DISCORD_TOKEN: "${DISCORD_TOKEN}"
      ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
```

```bash
# .env file (not committed to git)
DISCORD_TOKEN=your-discord-token
ANTHROPIC_API_KEY=sk-ant-...
```

Note: `DISCORD_TOKEN` is used by the orchestrator (openpact-system) to connect to Discord. It is **not** passed to the AI process.

## Security Hardening

### Dropped Capabilities

Remove unnecessary Linux capabilities:

```yaml
services:
  openpact:
    cap_drop:
      - ALL
```

### No Privilege Escalation

```yaml
services:
  openpact:
    security_opt:
      - no-new-privileges:true
```

### Resource Limits

```yaml
services:
  openpact:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M
```

## Network Security

### Port Exposure

```yaml
services:
  openpact:
    ports:
      - "127.0.0.1:8080:8080"  # Admin UI - localhost only
      - "1455:1455"             # OpenCode OAuth callback
    # MCP uses stdio between processes, no network port needed
```

### Network Configuration

```yaml
services:
  openpact:
    networks:
      - frontend   # Admin UI access
      - backend    # Internal services only

networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge
    internal: true  # No external access
```

## Complete Production Example

```yaml
# docker-compose.yml - Production Configuration
version: '3.8'

services:
  openpact:
    image: openpact:latest
    container_name: openpact

    # Security settings
    read_only: true
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL

    # Temporary filesystem
    tmpfs:
      - /tmp:size=64M,mode=1777

    # Volumes
    volumes:
      - ./workspace:/workspace

    # Environment (orchestrator sees all, AI process gets filtered subset)
    environment:
      DISCORD_TOKEN: "${DISCORD_TOKEN}"
      ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"

    # Network
    ports:
      - "127.0.0.1:8080:8080"
      - "1455:1455"
    networks:
      - openpact-net

    # Resources
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M

    # Health check
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

    restart: unless-stopped

networks:
  openpact-net:
    driver: bridge
```

## Verification

### Check Process Users

```bash
# Verify AI runs as openpact-ai
docker exec <container> ps aux | grep opencode
# Expected: openpact-ai ... opencode serve --port ...

# Verify orchestrator runs as openpact-system
docker exec <container> ps aux | grep openpact
# Expected: openpact-+ ... /app/openpact start
```

### Check File Permissions

```bash
docker exec <container> ls -la /workspace/
# secure/ should be drwx------ (700)

docker exec <container> ls -la /workspace/ai-data/
# memory/ should be drwxrwx--- (770)
# MEMORY.md should be -rw-rw---- (660)
```

### Check Environment Isolation

```bash
# From the AI, if bash were available (it's disabled):
# echo $DISCORD_TOKEN -> empty
# echo $ANTHROPIC_API_KEY -> would show key (needed for LLM calls)
```

## Security Checklist

### Build Time

- [ ] Use official base image
- [ ] Create both non-root users
- [ ] Build mcp-server binary alongside main binary
- [ ] Remove unnecessary packages

### Runtime

- [ ] Entrypoint sets correct file permissions
- [ ] `run_as_user` set to `openpact-ai` in config
- [ ] `mcp-server` binary exists at `/app/mcp-server` (auto-discovered)
- [ ] Read-only root filesystem (recommended)
- [ ] Capabilities dropped
- [ ] Resource limits set

### Monitoring

- [ ] Health checks enabled
- [ ] Logging configured
- [ ] Process user verified (ps aux)
- [ ] File permissions verified
