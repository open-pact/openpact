---
title: Development Setup
sidebar_position: 1
---

# Development Setup

This guide walks you through setting up OpenPact for local development.

## Prerequisites

Before you begin, ensure you have the following installed:

- **Go 1.22+** - [Download Go](https://golang.org/dl/)
- **Git** - For version control
- **Docker** (optional) - For container-based development
- **Make** - For running build commands
- **Node.js 18+** (optional) - For Admin UI development

## Clone the Repository

```bash
git clone https://github.com/open-pact/openpact.git
cd openpact-app
```

## Project Structure

```
openpact-app/
├── cmd/
│   ├── openpact/       # Main application entry point
│   └── admin/          # Admin server entry point
├── internal/
│   ├── admin/          # Admin UI backend
│   ├── config/         # Configuration management
│   ├── context/        # Context file handling
│   ├── discord/        # Discord integration
│   ├── engine/         # AI engine adapters
│   ├── health/         # Health check endpoints
│   ├── logging/        # Structured logging
│   ├── mcp/            # MCP server and tools
│   ├── orchestrator/   # Main orchestration logic
│   ├── ratelimit/      # Rate limiting
│   └── starlark/       # Starlark script engine
├── admin-ui/           # React Admin UI frontend
├── docs/               # Documentation (Docusaurus)
├── examples/           # Example configurations
├── pkg/                # Public packages
└── templates/          # Template files
```

## Building OpenPact

### Using Make

The project includes a Makefile with common commands:

```bash
# Build the binary
make build

# Run tests
make test

# Run tests with coverage
make coverage

# Build Docker image
make docker

# Run locally
make run

# Format code
make fmt

# Run linter (requires golangci-lint)
make lint
```

### Manual Build

```bash
# Build the main binary
go build -o openpact ./cmd/openpact

# Run the application
./openpact start
```

## Running Tests

Run the full test suite:

```bash
# Run all tests with verbose output
go test -v ./...

# Run tests for a specific package
go test -v ./internal/mcp/...

# Run tests with race detection
go test -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Configuration for Development

Create a local configuration file:

```bash
cp .env.sample .env
```

Edit `.env` with your development credentials:

```bash
# Required for Discord integration
DISCORD_BOT_TOKEN=your_dev_bot_token
DISCORD_CHANNEL_ID=your_test_channel

# Optional integrations
GITHUB_TOKEN=your_github_token
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
```

Create a development `config.yaml`:

```yaml
workspace_path: "./dev-workspace"
memory_file: "./dev-memory.md"
soul_file: "./SOUL.md"
user_file: "./USER.md"

engine:
  type: "opencode"

mcp:
  enabled_tools:
    - workspace_read
    - workspace_write
    - memory_read
    - memory_write

server:
  port: 8080
  admin_port: 8081

logging:
  level: debug
  format: text
```

## Admin UI Development

The Admin UI is a React application built with Vite:

```bash
cd admin-ui

# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build
```

The Admin UI development server runs on `http://localhost:5173` by default.

## Docker Development

Build and run with Docker:

```bash
# Build the Docker image
docker build -t openpact:dev .

# Run with docker-compose
docker-compose up -d
```

## Hot Reloading

For development with hot reloading, you can use tools like [air](https://github.com/cosmtrek/air):

```bash
# Install air
go install github.com/cosmtrek/air@latest

# Run with hot reload
air
```

Create an `.air.toml` configuration:

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/openpact ./cmd/openpact"
bin = "./tmp/openpact"
args = ["start"]
include_ext = ["go", "yaml"]
exclude_dir = ["tmp", "docs", "admin-ui"]
```

## IDE Setup

### VS Code

Recommended extensions:
- Go (by Google)
- YAML
- Docker
- EditorConfig

Settings (`.vscode/settings.json`):
```json
{
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  }
}
```

### GoLand / IntelliJ IDEA

- Enable "Format on Save"
- Configure golangci-lint as external tool
- Set Go SDK to 1.22+

## Troubleshooting

### Common Issues

**Build fails with missing dependencies:**
```bash
go mod tidy
go mod download
```

**Tests fail with race conditions:**
```bash
# Run with race detector
go test -race ./...
```

**Docker build fails:**
```bash
# Clean Docker cache
docker builder prune
docker build --no-cache -t openpact:dev .
```

## Next Steps

- Read the [Architecture](./architecture) overview
- Review [Code Style](./code-style) guidelines
- Check existing [issues](https://github.com/open-pact/openpact/issues)
