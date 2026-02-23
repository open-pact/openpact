---
title: Workspace
sidebar_position: 3
---

# Workspace

The workspace is OpenPact's top-level directory that contains both secure system data and AI-accessible files. The AI assistant can read, write, and manage files within the `ai-data/` subdirectory, while sensitive configuration and data are isolated in the `secure/` subdirectory where the AI has zero access.

## Overview

The workspace provides:

- **Persistent Storage**: Files survive container restarts
- **AI File Access**: Your assistant can manage files in `ai-data/` on your behalf
- **Security Boundary**: Sensitive config and data in `secure/` are inaccessible to the AI; MCP tools are scoped to `ai-data/` only
- **Organized Structure**: Keep scripts, notes, and data organized

## Configuration

Configure the workspace path in `openpact.yaml`:

```yaml
workspace:
  path: ./workspace  # default; use /workspace in Docker
```

When using Docker, mount a volume to persist files:

```bash
docker run -d \
  -v openpact-workspace:/workspace \
  -e DISCORD_TOKEN=your_token \
  ghcr.io/open-pact/openpact:latest
```

Or with Docker Compose:

```yaml
services:
  openpact:
    image: ghcr.io/open-pact/openpact:latest
    volumes:
      - openpact-workspace:/workspace

volumes:
  openpact-workspace:
```

## Workspace Tools

OpenPact provides three MCP tools for workspace operations. All workspace tools are scoped to the `ai-data/` subdirectory -- the AI cannot access files in `secure/`.

### workspace_read

Read the contents of a file.

```json
{
  "name": "workspace_read",
  "arguments": {
    "path": "notes/todo.md"
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Relative path within `ai-data/` |

**Returns:** File contents as a string, or an error if the file doesn't exist.

### workspace_write

Write content to a file, creating directories as needed.

```json
{
  "name": "workspace_write",
  "arguments": {
    "path": "notes/meeting-notes.md",
    "content": "# Meeting Notes\n\n## Attendees\n- Alice\n- Bob\n\n## Discussion\n..."
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Relative path within `ai-data/` |
| `content` | string | Yes | Content to write to the file |

**Returns:** Success confirmation or error message.

### workspace_list

List files and directories at a path.

```json
{
  "name": "workspace_list",
  "arguments": {
    "path": "notes"
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | No | Relative path within `ai-data/` (defaults to `ai-data/` root) |

**Returns:** List of files and directories with metadata.

## Path Security

OpenPact enforces strict path security to protect your system.

### Restricted to ai-data/

All file operations are confined to the `ai-data/` subdirectory within the workspace:

```
Allowed:
  ai-data/notes/todo.md            ✓
  ai-data/scripts/helper.star      ✓
  ai-data/memory/2024-01-15.md     ✓

Blocked:
  secure/config.yaml               ✗  (system-only)
  secure/data/secrets.json         ✗  (system-only)
  /etc/passwd                      ✗
  ../../../etc/shadow              ✗
  /home/user/.ssh/id_rsa           ✗
```

### Path Traversal Prevention

Attempts to escape `ai-data/` using `..` are blocked:

```json
// This will fail
{
  "name": "workspace_read",
  "arguments": {
    "path": "../../../etc/passwd"
  }
}
// Error: path traversal not allowed
```

### Normalized Paths

Paths are normalized before validation:

- `./notes/../notes/todo.md` becomes `notes/todo.md`
- Double slashes are cleaned: `notes//todo.md` becomes `notes/todo.md`
- Trailing slashes are removed

### Symbolic Links

By default, symbolic links that point outside `ai-data/` are not followed, preventing link-based escape attempts.

## Directory Structure

A typical workspace organization:

```
workspace/
├── secure/                     # SYSTEM-ONLY — AI has ZERO access
│   ├── config.yaml             # OpenPact configuration
│   └── data/                   # Secrets, JWT key, approvals, etc.
│       ├── jwt_secret
│       ├── users.json
│       ├── approvals.json
│       ├── secrets.json
│       └── opencode/
├── ai-data/                    # AI-ACCESSIBLE — MCP tools scope here
│   ├── SOUL.md                 # AI identity and personality
│   ├── USER.md                 # User preferences
│   ├── MEMORY.md               # Persistent memory
│   ├── memory/                 # Daily memory files
│   │   ├── 2024-01-15.md
│   │   └── 2024-01-16.md
│   ├── scripts/                # Starlark scripts
│   │   ├── weather.star
│   │   └── stocks.star
│   ├── skills/                 # Skill definitions
│   ├── notes/                  # General notes
│   │   ├── todo.md
│   │   └── projects/
│   └── downloads/              # Downloaded content
```

## Use Cases

### Note Taking

Your AI can create and maintain notes:

```
User: "Create a note about today's meeting with the marketing team"

AI uses workspace_write to create notes/meetings/2024-01-15-marketing.md
```

### File Organization

Ask your AI to organize files:

```
User: "List all the files in my notes folder"

AI uses workspace_list with path "notes" to show the structure
```

### Data Storage

Store data from integrations:

```
User: "Save the weather forecast for the next week"

AI fetches weather data and uses workspace_write to save it
```

### Script Development

Your AI can help create and modify Starlark scripts:

```
User: "Create a script to check stock prices"

AI uses workspace_write to create ai-data/scripts/stocks.star
```

## Best Practices

### Organize with Directories

Keep your workspace tidy with clear directory structure:

```yaml
# Good organization
notes/
  personal/
  work/
  projects/

# Avoid flat structure with many files
note1.md
note2.md
note3.md
...
```

### Use Meaningful Names

Name files descriptively:

```yaml
# Good
meeting-notes-2024-01-15-product-review.md

# Less helpful
notes.md
```

### Regular Backups

Since the workspace is a Docker volume, back it up regularly:

```bash
# Backup workspace volume
docker run --rm \
  -v openpact-workspace:/workspace \
  -v $(pwd):/backup \
  alpine tar czf /backup/workspace-backup.tar.gz /workspace
```

### Version Control for Scripts

Consider keeping scripts in version control:

```bash
# In workspace/ai-data/scripts/
git init
git add *.star
git commit -m "Initial scripts"
```

## Troubleshooting

### File Not Found

If `workspace_read` returns "file not found":

1. Check the exact path with `workspace_list`
2. Verify the file was created successfully
3. Check for typos in the filename

### Permission Denied

In rare cases with Docker volume permissions:

1. Check volume mount permissions
2. Ensure the container user can write to the volume
3. Review Docker volume configuration

### Path Errors

If you see "path traversal not allowed":

1. Use only relative paths within `ai-data/`
2. Don't use `..` to navigate up
3. Don't use absolute paths

## Related Documentation

- **[MCP Tools Reference](./mcp-tools)** - Complete tool documentation
- **[Memory System](./memory-system)** - Memory files in workspace
- **[Configuration Overview](../configuration/overview)** - Workspace configuration
