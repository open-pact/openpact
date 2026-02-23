---
title: Code Style
sidebar_position: 3
---

# Code Style Guide

This document outlines the coding conventions and guidelines for contributing to OpenPact.

## Go Conventions

### Formatting

All Go code must be formatted with `gofmt`:

```bash
# Format all files
go fmt ./...

# Or use make
make fmt
```

### Imports

Organize imports into three groups, separated by blank lines:

```go
import (
    // Standard library
    "context"
    "fmt"
    "net/http"

    // Third-party packages
    "github.com/bwmarrin/discordgo"
    "gopkg.in/yaml.v3"

    // Internal packages
    "github.com/open-pact/openpact/internal/config"
    "github.com/open-pact/openpact/internal/mcp"
)
```

### Naming

Follow Go naming conventions:

```go
// Package names: lowercase, single word
package mcp

// Exported functions/types: PascalCase
type ToolServer struct {}
func (s *ToolServer) RegisterTool() {}

// Unexported functions/types: camelCase
type toolRegistry struct {}
func (r *toolRegistry) addTool() {}

// Constants: PascalCase for exported, camelCase for unexported
const DefaultTimeout = 30 * time.Second
const maxRetries = 3

// Acronyms: consistent casing
type HTTPClient struct {}  // not HttpClient
type JSONParser struct {}  // not JsonParser
var userID string          // not userId
```

### Error Handling

Always handle errors explicitly:

```go
// Good
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Bad - don't ignore errors
result, _ := doSomething()
```

Wrap errors with context:

```go
if err := config.Load(); err != nil {
    return fmt.Errorf("loading config: %w", err)
}
```

### Context

Pass `context.Context` as the first parameter:

```go
func (s *Server) HandleRequest(ctx context.Context, req *Request) error {
    // Use ctx for cancellation and deadlines
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // Continue processing
    }
}
```

### Interface Design

Keep interfaces small and focused:

```go
// Good - focused interface
type Reader interface {
    Read(ctx context.Context, path string) ([]byte, error)
}

// Good - compose interfaces
type ReadWriter interface {
    Reader
    Writer
}

// Avoid - too many methods
type FileSystem interface {
    Read(path string) ([]byte, error)
    Write(path string, data []byte) error
    Delete(path string) error
    List(path string) ([]string, error)
    Stat(path string) (FileInfo, error)
    // ... many more methods
}
```

### Struct Tags

Use consistent tag ordering:

```go
type Config struct {
    Name     string `yaml:"name" json:"name"`
    Enabled  bool   `yaml:"enabled" json:"enabled"`
    Timeout  int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}
```

## Testing

### Test File Organization

Place tests in the same package with `_test.go` suffix:

```
internal/mcp/
├── server.go
├── server_test.go
├── tools.go
└── tools_test.go
```

### Test Naming

Use descriptive test names:

```go
func TestToolServer_RegisterTool_Success(t *testing.T) {}
func TestToolServer_RegisterTool_DuplicateName(t *testing.T) {}
func TestToolServer_Execute_InvalidParams(t *testing.T) {}
```

### Table-Driven Tests

Use table-driven tests for comprehensive coverage:

```go
func TestParseDuration(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    time.Duration
        wantErr bool
    }{
        {
            name:  "seconds",
            input: "30s",
            want:  30 * time.Second,
        },
        {
            name:  "minutes",
            input: "5m",
            want:  5 * time.Minute,
        },
        {
            name:    "invalid",
            input:   "invalid",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseDuration(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseDuration() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("ParseDuration() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Test Helpers

Use `t.Helper()` for test helper functions:

```go
func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

func assertEqual[T comparable](t *testing.T, got, want T) {
    t.Helper()
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

### Mocking

Use interfaces for testability:

```go
// Production code
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

type WebFetcher struct {
    client HTTPClient
}

// Test code
type mockHTTPClient struct {
    response *http.Response
    err      error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
    return m.response, m.err
}
```

## Documentation

### Package Comments

Every package should have a package comment:

```go
// Package mcp implements the Model Context Protocol server
// for exposing tools to AI models.
//
// The server handles tool registration, request validation,
// and response formatting according to the MCP specification.
package mcp
```

### Function Comments

Document exported functions:

```go
// RegisterTool adds a new tool to the server's registry.
// It returns an error if a tool with the same name already exists.
//
// The tool's schema is validated during registration to ensure
// it conforms to the MCP specification.
func (s *Server) RegisterTool(tool Tool) error {
    // ...
}
```

### Inline Comments

Use inline comments sparingly, for non-obvious logic:

```go
// Calculate exponential backoff with jitter
// to prevent thundering herd
delay := baseDelay * time.Duration(1<<attempt)
jitter := time.Duration(rand.Int63n(int64(delay / 4)))
time.Sleep(delay + jitter)
```

## Pull Request Guidelines

### Branch Naming

Use descriptive branch names:

```
feature/add-slack-integration
fix/discord-reconnect-loop
docs/update-mcp-reference
refactor/extract-tool-registry
```

### Commit Messages

Write clear, concise commit messages:

```
Add Slack integration support

- Implement SlackClient in internal/slack
- Add slack_send MCP tool
- Update configuration for Slack tokens
- Add integration tests

Closes #123
```

### PR Checklist

Before submitting a PR:

- [ ] Code follows style guidelines
- [ ] All tests pass (`make test`)
- [ ] New code has tests
- [ ] Code is formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated if needed
- [ ] Commit messages are clear
- [ ] PR description explains changes

### Code Review

When reviewing PRs, check for:

1. **Correctness**: Does the code work as intended?
2. **Security**: Are there any security concerns?
3. **Performance**: Are there any obvious performance issues?
4. **Maintainability**: Is the code easy to understand and modify?
5. **Testing**: Are tests comprehensive and meaningful?
6. **Documentation**: Is new functionality documented?

## Linting

Use `golangci-lint` for static analysis:

```bash
# Install
go install github.com/golangci-lint/golangci-lint/cmd/golangci-lint@latest

# Run
make lint

# Or directly
golangci-lint run
```

The project uses the following linters:
- `gofmt` - Formatting
- `govet` - Suspicious constructs
- `errcheck` - Unchecked errors
- `staticcheck` - Static analysis
- `gosec` - Security issues
- `ineffassign` - Unused assignments
- `misspell` - Spelling errors

## Security Considerations

When contributing security-sensitive code:

1. **Never log secrets**: Use structured logging with redaction
2. **Validate inputs**: Check all user-provided data
3. **Use prepared statements**: Avoid SQL injection
4. **Limit permissions**: Follow principle of least privilege
5. **Handle errors safely**: Don't expose internal details
6. **Review dependencies**: Check for known vulnerabilities

```go
// Good - sanitized logging
log.Info("processing request",
    "user_id", req.UserID,
    "action", req.Action)

// Bad - potential secret exposure
log.Info("request: %+v", req)
```

## Getting Help

If you have questions about code style or contribution guidelines:

1. Check existing code for examples
2. Ask in GitHub discussions
3. Join the Discord community
4. Open a draft PR for feedback
