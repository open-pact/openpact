---
sidebar_position: 2
title: Principle of Least Privilege
description: Tool access, MCP restrictions, and network isolation
---

# Principle of Least Privilege

OpenPact implements the principle of least privilege throughout its architecture. Every component receives only the minimum permissions necessary to perform its function.

## What Is Least Privilege?

The principle of least privilege (PoLP) states that every component, user, or process should have only the access rights necessary to perform its legitimate purpose. This limits the damage that can result from accidents, errors, or unauthorized use.

## AI Access Restrictions

### What the AI Can Access

| Resource | Access Level | Notes |
|----------|--------------|-------|
| MCP tools | Defined set | Only registered tools |
| Script execution | Approved only | Requires admin approval |
| Workspace files | Scoped directory | Cannot escape workspace |
| Memory system | Dedicated storage | Isolated from system files |
| Secrets | Never | Values are redacted |

### What the AI Cannot Access

- System files outside the workspace
- Environment variables directly
- Raw network sockets
- Shell command execution
- Secret values (sees `[REDACTED]`)
- Unapproved scripts
- Admin UI operations

## MCP Tool Boundaries

Each MCP tool has explicit capability boundaries:

### Script Tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `script_run` | Execute approved scripts | No pending/rejected scripts |
| `script_exec` | Execute inline Starlark | Sandboxed, no filesystem |
| `script_list` | View script metadata | No source code access |
| `script_reload` | Reload from disk | Admin must place files |

### File Tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `workspace_read` | Read workspace files | Workspace directory only |
| `workspace_write` | Write workspace files | Workspace directory only |
| `workspace_list` | List workspace contents | No parent directory access |

### Memory Tools

| Tool | Capabilities | Restrictions |
|------|--------------|--------------|
| `memory_store` | Save key-value data | Dedicated memory file |
| `memory_retrieve` | Read stored data | Own data only |
| `memory_search` | Search memory | Own data only |

## Network Isolation

### Starlark Scripts

Scripts can only make HTTP/HTTPS requests:

```python
# Allowed
http.get("https://api.example.com/data")
http.post("https://webhook.example.com", body=data)

# Not allowed
socket.connect("tcp://internal:9000")  # No raw sockets
ftp.get("ftp://server/file")           # No FTP
ssh.exec("command")                    # No SSH
```

### Container Networking

When running in Docker, network access can be further restricted:

```yaml
# docker-compose.yml
services:
  openpact:
    networks:
      - openpact-net
    # No access to host network or other containers

networks:
  openpact-net:
    internal: true  # No external access if desired
```

### Egress Filtering

For production environments, consider egress filtering:

```
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│   OpenPact   │ ───> │ Egress Proxy │ ───> │   Internet   │
│              │      │ (allowlist)  │      │              │
└──────────────┘      └──────────────┘      └──────────────┘
```

Only allow connections to known, trusted API endpoints.

## Script Capability Model

Scripts declare their requirements, which are verified before execution:

### Secret Declaration

```python
# @name: weather.star
# @secrets: WEATHER_API_KEY

def get_weather(city):
    key = secrets.get("WEATHER_API_KEY")
    # ...
```

The script can only access secrets it declares.

### Capability Enforcement

```
Script requests WEATHER_API_KEY
       │
       ▼
┌─────────────────────────────┐
│ Is WEATHER_API_KEY in       │
│ script's @secrets metadata? │
└─────────────────────────────┘
       │
   ┌───┴───┐
   │       │
  YES      NO
   │       │
   ▼       ▼
Return   Return
value    error
```

## Data Access Boundaries

### Workspace Isolation

Each OpenPact instance has a dedicated workspace:

```
workspace/
├── documents/     # AI can read/write
├── projects/      # AI can read/write
└── temp/          # AI can read/write

/etc/              # AI cannot access
/home/             # AI cannot access
/var/              # AI cannot access
```

### Memory Isolation

The memory system is separate from the filesystem:

```
data/
├── memory.json    # Memory system storage
├── secrets.json   # Secrets (not accessible to AI)
└── approvals.json # Approval records (not accessible to AI)
```

The AI can only access memory.json through MCP tools, never directly.

## User Separation (Docker)

OpenPact uses a two-user model in Docker:

| User | Purpose | Permissions |
|------|---------|-------------|
| `root` | Initial setup | Creates directories, sets permissions |
| `openpact` | Runtime | Runs application with limited access |

```dockerfile
# Dockerfile example
FROM alpine:3.19

# Create non-root user
RUN addgroup -S openpact && adduser -S openpact -G openpact

# Set up directories with proper ownership
RUN mkdir -p /data /workspace && \
    chown -R openpact:openpact /data /workspace

# Switch to non-root user
USER openpact

# Run the application
ENTRYPOINT ["/app/openpact"]
```

## Configuration Restrictions

### Environment Variables

Scripts cannot access environment variables directly:

```python
# Not available in Starlark
os.environ["SECRET_KEY"]  # No os module
getenv("SECRET_KEY")      # No getenv function
```

Secrets must be explicitly configured and accessed via `secrets.get()`.

### File Paths

Configuration cannot reference paths outside allowed directories:

```yaml
# Valid
workspace:
  path: "/workspace"

# Invalid - would be rejected
workspace:
  path: "/etc"  # System directory
  path: "../../sensitive"  # Path traversal
```

## API Permission Model

### Admin API

| Endpoint | Authentication | Purpose |
|----------|----------------|---------|
| `/api/auth/*` | Public | Login/logout |
| `/api/setup` | Setup mode only | First-run config |
| `/api/scripts/*` | JWT required | Script management |
| `/api/secrets/*` | JWT required | Secret management |
| `/api/health` | Public | Health checks |

### MCP Protocol

MCP communication is restricted to local stdio transport:

```
AI Client ←── stdio ──→ OpenPact MCP Server
```

- No network MCP transport
- Process must be launched by Claude Desktop
- Inherits permissions of parent process

## Implementing Least Privilege

### For Operators

1. **Use dedicated service accounts**
   ```bash
   # Create dedicated user for OpenPact
   useradd -r -s /bin/false openpact
   ```

2. **Restrict file permissions**
   ```bash
   # Data directory
   chmod 700 /opt/openpact/data
   chown openpact:openpact /opt/openpact/data
   ```

3. **Use IP allowlisting**
   ```yaml
   admin:
     allowed_ips:
       - "10.0.0.0/8"
   ```

4. **Configure resource limits**
   ```yaml
   starlark:
     max_execution_ms: 30000
     max_memory_mb: 128
   ```

### For Script Authors

1. **Request only needed secrets**
   ```python
   # @secrets: WEATHER_API_KEY
   # Don't request secrets you don't use
   ```

2. **Return minimal data**
   ```python
   # Good - only needed data
   return {"temperature": data["temp"]}

   # Bad - excessive data
   return data  # May contain unnecessary info
   ```

3. **Scope API access**
   ```python
   # Use read-only API keys when possible
   # Use scoped tokens instead of admin tokens
   ```

## Monitoring Least Privilege

### Audit Questions

Periodically review:

- Which scripts have access to which secrets?
- Are all approved scripts still needed?
- Are there unused secrets that should be removed?
- Is the workspace directory appropriately scoped?

### Logging

Enable audit logging to track access:

```yaml
logging:
  level: info
  audit: true  # Log all tool invocations
```

Review logs for unexpected access patterns:

```
[AUDIT] script_run: weather.star by AI (approved)
[AUDIT] secret_access: WEATHER_API_KEY by weather.star
[AUDIT] http_request: GET https://api.weather.com/...
```

## Summary

| Component | Least Privilege Implementation |
|-----------|-------------------------------|
| AI | No direct system access, redacted secrets |
| Scripts | Sandboxed, declared capabilities only |
| Workspace | Scoped directory, no escape |
| Network | HTTP/HTTPS only, no raw sockets |
| Container | Non-root, dropped capabilities |
| Admin | JWT auth, IP allowlist option |
