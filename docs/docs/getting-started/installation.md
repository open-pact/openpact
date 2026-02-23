---
title: Installation
sidebar_position: 2
---

# Installation

This guide covers all the ways to install and run OpenPact.

## Docker (Recommended)

Docker is the recommended way to run OpenPact. It provides isolation, easy updates, and consistent behavior across platforms.

### Quick Start

```bash
docker run -d \
  --name openpact \
  -v openpact-workspace:/workspace \
  -e DISCORD_TOKEN=your_token \
  -p 8080:8080 \
  -p 1455:1455 \
  ghcr.io/open-pact/openpact:latest
```

### With Configuration File

For more complex setups, mount a configuration file into the `secure/` directory:

```bash
docker run -d \
  --name openpact \
  -v openpact-workspace:/workspace \
  -v /path/to/openpact.yaml:/workspace/secure/config.yaml:ro \
  -e DISCORD_TOKEN=your_token \
  -p 8080:8080 \
  -p 1455:1455 \
  ghcr.io/open-pact/openpact:latest
```

### Available Tags

| Tag | Description |
|-----|-------------|
| `latest` | Latest stable release |
| `vX.Y.Z` | Specific version (e.g., `v1.0.0`) |
| `main` | Latest development build (may be unstable) |

## Docker Compose

Docker Compose is ideal for development and for managing OpenPact alongside other services.

### Basic Setup

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  openpact:
    image: ghcr.io/open-pact/openpact:latest
    container_name: openpact
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "1455:1455"   # OpenCode OAuth callback
    volumes:
      - openpact-workspace:/workspace
      - ./openpact.yaml:/workspace/secure/config.yaml:ro
    environment:
      - DISCORD_TOKEN=${DISCORD_TOKEN}

volumes:
  openpact-workspace:
```

### With Environment File

Create a `.env` file (never commit this to version control):

```bash
# .env
DISCORD_TOKEN=your_discord_bot_token
GITHUB_TOKEN=your_github_token
```

Then run:

```bash
docker compose up -d
```

### Development Setup with Live Reload

For development, you can mount the source code and rebuild:

```yaml
version: '3.8'

services:
  openpact:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: openpact-dev
    ports:
      - "8080:8080"
      - "1455:1455"   # OpenCode OAuth callback
    volumes:
      - ./workspace:/workspace
      - ./openpact.yaml:/workspace/secure/config.yaml:ro
    env_file:
      - .env
```

Rebuild after changes:

```bash
docker compose up --build
```

### Commands

```bash
# Start in background
docker compose up -d

# View logs
docker compose logs -f

# Stop
docker compose down

# Rebuild and start
docker compose up --build -d
```

## Building from Source

Build OpenPact yourself if you want to modify the code or run without Docker.

### Prerequisites

- **Go 1.22** or later
- **Make** (optional, for convenience)
- **Git**

### Clone and Build

```bash
# Clone the repository
git clone https://github.com/open-pact/openpact.git
cd openpact

# Build with Make
make build

# Or build directly with Go
go build -o openpact ./cmd/openpact
```

### Run

```bash
# Set environment variables
export DISCORD_TOKEN=your_token

# Run with config file
./openpact --config openpact.yaml
```

### Run Tests

```bash
# Run all tests
make test

# Run with coverage
make coverage

# Run linter
make lint
```

## System Requirements

### Minimum Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 1 core | 2+ cores |
| RAM | 512 MB | 1+ GB |
| Disk | 100 MB | 1+ GB (for workspace) |
| Network | Internet access | Stable connection |

### Supported Platforms

OpenPact runs on any platform that supports Docker or Go:

- **Linux**: x86_64, ARM64
- **macOS**: Intel, Apple Silicon
- **Windows**: x86_64 (via WSL2 or Docker Desktop)

### Network Requirements

OpenPact needs outbound access to:

- Discord API (`discord.com`)
- Your LLM provider (e.g., `api.anthropic.com`)
- Any services you integrate (GitHub, calendar feeds, etc.)

## Updating

### Docker

```bash
# Pull latest image
docker pull ghcr.io/open-pact/openpact:latest

# Restart container
docker stop openpact
docker rm openpact
docker run -d ... # (your usual run command)
```

### Docker Compose

```bash
docker compose pull
docker compose up -d
```

### From Source

```bash
git pull
make build
# Restart your service
```

## Next Steps

- **[First Steps](./first-steps)** - Configure Discord and API keys
- **[Configuration Overview](../configuration/overview)** - Full configuration guide
- **[Environment Variables](../configuration/environment-variables)** - All available options
