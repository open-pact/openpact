---
title: Obsidian Vault
sidebar_position: 5
---

# Obsidian Vault Integration

OpenPact can connect to an [Obsidian](https://obsidian.md/) vault, allowing your AI assistant to read, write, and search your notes. With optional Git sync, changes are automatically committed and pushed.

## Overview

The Obsidian integration provides:

- **Read Notes**: Access any note in your vault
- **Write Notes**: Create or update notes
- **List Files**: Browse vault structure
- **Search Content**: Find notes by content
- **Git Sync**: Automatic commit and push on writes

## Vault Configuration

Configure your vault in `openpact.yaml`:

```yaml
vault:
  path: /vault
  git_repo: git@github.com:username/obsidian-vault.git
  auto_sync: true
```

### Configuration Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `path` | string | Yes | Local path to vault directory |
| `git_repo` | string | No | Git repository URL for sync |
| `auto_sync` | boolean | No | Auto-commit and push on writes (default: false) |

### Docker Setup

Mount your vault as a volume:

```bash
docker run -d \
  -v /path/to/your/vault:/vault \
  -v openpact-workspace:/workspace \
  -e DISCORD_TOKEN=your_token \
  ghcr.io/open-pact/openpact:latest
```

With Docker Compose:

```yaml
services:
  openpact:
    image: ghcr.io/open-pact/openpact:latest
    volumes:
      - /path/to/your/vault:/vault
      - openpact-workspace:/workspace
    environment:
      - DISCORD_TOKEN=${DISCORD_TOKEN}
```

## Git Sync Setup

For automatic synchronization with a remote Git repository:

### 1. Initialize Your Vault as a Git Repository

If not already a Git repo:

```bash
cd /path/to/your/vault
git init
git remote add origin git@github.com:username/obsidian-vault.git
git add .
git commit -m "Initial commit"
git push -u origin main
```

### 2. Configure SSH Keys

For the Docker container to push to Git, it needs SSH access:

```bash
# Mount your SSH directory (read-only for security)
docker run -d \
  -v /path/to/your/vault:/vault \
  -v ~/.ssh:/root/.ssh:ro \
  -v openpact-workspace:/workspace \
  ghcr.io/open-pact/openpact:latest
```

Or use a deploy key:

1. Generate a key specifically for the vault:
   ```bash
   ssh-keygen -t ed25519 -f vault-deploy-key -N ""
   ```

2. Add the public key to your repository's deploy keys (with write access)

3. Mount the private key:
   ```bash
   -v ./vault-deploy-key:/root/.ssh/id_ed25519:ro
   ```

### 3. Enable Auto-sync

```yaml
vault:
  path: /vault
  git_repo: git@github.com:username/obsidian-vault.git
  auto_sync: true
```

When `auto_sync` is enabled:
- Every `vault_write` automatically commits the change
- Commits are pushed to the remote repository
- Commit messages include the file path and timestamp

## Vault Tools

### vault_read

Read a note from your vault.

```json
{
  "name": "vault_read",
  "arguments": {
    "path": "Projects/OpenPact.md"
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to note within vault |

**Returns:** Note contents as a string.

### vault_write

Write or update a note in your vault.

```json
{
  "name": "vault_write",
  "arguments": {
    "path": "Daily/2024-01-15.md",
    "content": "# 2024-01-15\n\n## Tasks\n- [ ] Review code\n- [ ] Write docs"
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to note within vault |
| `content` | string | Yes | Note content (markdown) |

**Returns:** Success confirmation. If `auto_sync` is enabled, also confirms Git commit.

### vault_list

List notes and folders in a directory.

```json
{
  "name": "vault_list",
  "arguments": {
    "path": "Projects"
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | No | Path within vault (defaults to root) |

**Returns:** List of files and directories.

### vault_search

Search for content across your vault.

```json
{
  "name": "vault_search",
  "arguments": {
    "query": "meeting notes"
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query |

**Returns:** List of matching notes with relevant excerpts.

## Reading Notes

Your AI can read any note from your vault:

```
User: "What's in my project notes for OpenPact?"

AI: Let me check your vault.
[Uses vault_read with path "Projects/OpenPact.md"]

Here's what I found in your OpenPact project notes:
- Status: In development
- Next milestone: Beta release
- Key tasks: Complete documentation, add tests
...
```

## Writing Notes

### Creating New Notes

```
User: "Create a new note for my meeting with the design team"

AI: I'll create that note for you.
[Uses vault_write with path "Meetings/2024-01-15-design-team.md"]

Done! I've created the meeting note at Meetings/2024-01-15-design-team.md
```

### Updating Existing Notes

```
User: "Add a new task to my project notes"

AI: Let me update your project notes.
[Uses vault_read to get current content]
[Uses vault_write with updated content]

I've added the new task to your project notes.
```

## Searching Vault Content

Find information across all your notes:

```
User: "Find all my notes about API design"

AI: Let me search your vault.
[Uses vault_search with query "API design"]

I found 5 notes mentioning API design:
1. Projects/API-Redesign.md - Main design document
2. Meetings/2024-01-10-backend.md - Discussion about REST vs GraphQL
3. Daily/2024-01-08.md - Notes on authentication endpoints
...
```

## Obsidian Compatibility

### Markdown Format

OpenPact respects Obsidian's markdown format:

- Standard markdown syntax
- YAML frontmatter
- Wiki-style links (`[[Note Name]]`)
- Tags (`#tag`)
- Callouts and admonitions

### Folder Structure

Maintain your existing folder organization:

```
vault/
├── Daily/              # Daily notes
├── Projects/           # Project documentation
├── Areas/              # Areas of responsibility
├── Resources/          # Reference material
├── Archive/            # Archived notes
└── Templates/          # Note templates
```

### Sync with Obsidian App

If you use Obsidian on your computer:

1. Point both Obsidian and OpenPact to the same vault
2. Or use Git sync to keep them synchronized
3. Changes made by the AI appear in your Obsidian app

## Security Considerations

### Path Restrictions

Like the workspace, vault operations are restricted:

- Cannot access files outside the vault directory
- Path traversal attempts (`..`) are blocked
- Symbolic links outside vault are not followed

### Sensitive Information

Consider what's in your vault:

- Avoid storing passwords or secrets in notes
- Be mindful of personal information
- Use Obsidian's built-in encryption for sensitive vaults

### Git Repository Security

If using Git sync:

- Use a private repository
- Consider SSH keys over HTTPS
- Review what's being committed
- Don't store secrets in the vault

## Troubleshooting

### Cannot Read Notes

If `vault_read` fails:

1. Verify the vault path in configuration
2. Check the note path is correct
3. Ensure the volume is mounted correctly
4. Check file permissions

### Git Sync Not Working

If auto-sync fails:

1. Verify SSH key access to the repository
2. Check Git remote is configured correctly
3. Ensure the container has network access
4. Review logs for Git errors

### Search Returns No Results

If `vault_search` finds nothing:

1. Verify the vault contains notes
2. Check the query isn't too specific
3. Ensure notes are plain text markdown
4. Try a broader search term

## Related Documentation

- **[MCP Tools Reference](./mcp-tools)** - Complete tool documentation
- **[Workspace](./workspace)** - Workspace file management
- **[Configuration Overview](../configuration/overview)** - General configuration
