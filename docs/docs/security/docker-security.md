---
sidebar_position: 4
title: Docker Security
description: Two-user model, container isolation, and volume mounts
---

# Docker Security

OpenPact is designed to run securely in Docker containers. This page covers the security architecture of containerized deployments, including the two-user model, filesystem isolation, and best practices.

## Two-User Model

OpenPact uses a two-user model in Docker to minimize the attack surface:

### User Roles

| User | UID | Purpose | Permissions |
|------|-----|---------|-------------|
| `root` | 0 | Container initialization | Creates directories, sets permissions |
| `openpact` | 1000 | Runtime execution | Runs application, owns data |

### Why Two Users?

1. **Root for setup only** - Some operations require root (creating users, setting permissions)
2. **Non-root for runtime** - The application never needs root privileges
3. **Principle of least privilege** - Runtime has only necessary permissions
4. **Container escape mitigation** - If compromised, attacker has limited privileges

### Dockerfile Implementation

```dockerfile
FROM alpine:3.19 AS base

# Create non-root user
RUN addgroup -g 1000 -S openpact && \
    adduser -u 1000 -S openpact -G openpact

# Create directories with correct ownership
RUN mkdir -p /data /workspace /app && \
    chown -R openpact:openpact /data /workspace

# Copy application
COPY --chown=openpact:openpact openpact /app/openpact

# Switch to non-root user for runtime
USER openpact

WORKDIR /app
ENTRYPOINT ["/app/openpact"]
CMD ["serve"]
```

## Container Isolation

### Filesystem Isolation

```
Container Filesystem
├── /app/                 # Application (read-only in production)
│   └── openpact          # Binary
├── /data/                # Persistent data (mounted volume)
│   ├── scripts/          # Starlark scripts
│   ├── secrets.json      # Encrypted secrets
│   ├── approvals.json    # Script approvals
│   └── jwt_secret        # JWT signing key
├── /workspace/           # AI workspace (mounted volume)
│   └── ...               # User files
└── /config/              # Configuration (mounted or built-in)
    └── openpact.yaml     # Config file
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
      - openpact-data:/data
      - ./workspace:/workspace
```

This prevents any writes to the container filesystem, limiting what a compromised process can do.

### Volume Mounts

| Volume | Purpose | Permissions |
|--------|---------|-------------|
| `/data` | Persistent data (secrets, approvals) | `openpact:openpact`, 700 |
| `/workspace` | AI workspace files | `openpact:openpact`, 755 |
| `/config` | Configuration (optional) | Read-only |

```yaml
# docker-compose.yml
services:
  openpact:
    volumes:
      # Named volume for data (Docker manages)
      - openpact-data:/data

      # Bind mount for workspace (user manages)
      - ./workspace:/workspace

      # Read-only config
      - ./openpact.yaml:/config/openpact.yaml:ro

volumes:
  openpact-data:
```

## Security Hardening

### Dropped Capabilities

Remove unnecessary Linux capabilities:

```yaml
# docker-compose.yml
services:
  openpact:
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE  # Only if binding to port < 1024
```

### No Privilege Escalation

Prevent privilege escalation attacks:

```yaml
# docker-compose.yml
services:
  openpact:
    security_opt:
      - no-new-privileges:true
```

### Resource Limits

Prevent resource exhaustion:

```yaml
# docker-compose.yml
services:
  openpact:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 128M
```

### Seccomp Profile

Use a restrictive seccomp profile:

```yaml
# docker-compose.yml
services:
  openpact:
    security_opt:
      - seccomp:seccomp-profile.json
```

## Network Security

### Network Modes

| Mode | Use Case | Security |
|------|----------|----------|
| Bridge (default) | General use | Isolated network namespace |
| Host | Not recommended | Shares host network (less secure) |
| None | Maximum isolation | No network access |
| Custom | Production | Define allowed connections |

### Network Configuration

```yaml
# docker-compose.yml
services:
  openpact:
    networks:
      - frontend  # Admin UI access
      - backend   # Internal services only

networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge
    internal: true  # No external access
```

### Port Exposure

Expose only necessary ports:

```yaml
# docker-compose.yml
services:
  openpact:
    ports:
      - "127.0.0.1:8080:8080"  # Admin UI - localhost only
      - "1455:1455"             # OpenCode OAuth callback
    # MCP uses stdio, no port needed
```

## Environment Variable Security

### Sensitive Variables

```yaml
# docker-compose.yml
services:
  openpact:
    environment:
      # Reference from .env file
      ADMIN_JWT_SECRET: "${ADMIN_JWT_SECRET}"
      WEATHER_API_KEY: "${WEATHER_API_KEY}"
```

```bash
# .env file (not committed to git)
ADMIN_JWT_SECRET=your-256-bit-secret
WEATHER_API_KEY=sk-abc123...
```

### Docker Secrets (Swarm/Compose)

For production, use Docker secrets:

```yaml
# docker-compose.yml
services:
  openpact:
    secrets:
      - jwt_secret
      - weather_api_key
    environment:
      ADMIN_JWT_SECRET_FILE: /run/secrets/jwt_secret
      WEATHER_API_KEY_FILE: /run/secrets/weather_api_key

secrets:
  jwt_secret:
    external: true
  weather_api_key:
    external: true
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
    user: "1000:1000"
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
      - openpact-data:/data
      - ./workspace:/workspace
      - ./openpact.yaml:/config/openpact.yaml:ro

    # Environment
    environment:
      OPENPACT_CONFIG: /config/openpact.yaml
      ADMIN_JWT_SECRET: "${ADMIN_JWT_SECRET}"
      WEATHER_API_KEY: "${WEATHER_API_KEY}"

    # Network
    ports:
      - "127.0.0.1:8080:8080"
      - "1455:1455"   # OpenCode OAuth callback
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

    # Restart policy
    restart: unless-stopped

networks:
  openpact-net:
    driver: bridge

volumes:
  openpact-data:
```

## Health Checks

### Container Health Check

```yaml
healthcheck:
  test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 10s
```

### Orchestrator Integration

Kubernetes example:

```yaml
# kubernetes deployment
spec:
  containers:
    - name: openpact
      livenessProbe:
        httpGet:
          path: /health
          port: 8080
        initialDelaySeconds: 10
        periodSeconds: 30
      readinessProbe:
        httpGet:
          path: /ready
          port: 8080
        initialDelaySeconds: 5
        periodSeconds: 10
```

## Logging and Monitoring

### Log Output

Configure JSON logging for container environments:

```yaml
# openpact.yaml
logging:
  format: json
  level: info
  output: stdout
```

### Log Collection

```yaml
# docker-compose.yml
services:
  openpact:
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
```

## Backup and Recovery

### Data Backup

```bash
# Backup data volume
docker run --rm -v openpact-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/openpact-data-backup.tar.gz /data

# Restore data volume
docker run --rm -v openpact-data:/data -v $(pwd):/backup alpine \
  tar xzf /backup/openpact-data-backup.tar.gz -C /
```

### Configuration Backup

Keep configuration in version control (without secrets):

```bash
# .gitignore
.env
*.secret
openpact-data-backup.tar.gz
```

## Security Checklist

### Build Time

- [ ] Use official base image (alpine, distroless)
- [ ] Create non-root user
- [ ] Remove unnecessary packages
- [ ] Scan image for vulnerabilities

### Runtime

- [ ] Run as non-root user
- [ ] Use read-only root filesystem
- [ ] Drop all capabilities
- [ ] Disable privilege escalation
- [ ] Set resource limits
- [ ] Use Docker secrets for sensitive data

### Network

- [ ] Expose only necessary ports
- [ ] Bind to localhost when possible
- [ ] Use custom networks
- [ ] Consider network policies

### Monitoring

- [ ] Enable health checks
- [ ] Configure logging
- [ ] Set up alerts
- [ ] Regular security scans
