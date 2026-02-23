---
title: GitHub Integration
sidebar_position: 7
---

# GitHub Integration

OpenPact integrates with GitHub, allowing your AI assistant to list and create issues in your repositories. This enables project management, bug tracking, and task organization through natural conversation.

## Overview

The GitHub integration provides:

- **List Issues**: View open and closed issues from repositories
- **Create Issues**: Create new issues with titles, descriptions, and labels
- **Repository Access**: Work with multiple repositories
- **Personal Token Auth**: Secure authentication via personal access tokens

## GitHub Token Setup

### Create a Personal Access Token

1. Go to [GitHub Settings > Developer settings > Personal access tokens](https://github.com/settings/tokens)
2. Click **Generate new token** (classic) or **Fine-grained tokens**
3. Give your token a descriptive name (e.g., "OpenPact Assistant")
4. Set an expiration date
5. Select the required scopes

### Required Scopes

For classic tokens, select:

- **repo** - Full control of private repositories (or `public_repo` for public only)

For fine-grained tokens, grant:

- **Issues** - Read and write access
- **Repository access** - Select specific repositories or all repositories

### Configure the Token

Add your token to the environment:

```bash
# .env file
GITHUB_TOKEN=ghp_your_personal_access_token
```

Or pass it directly to Docker:

```bash
docker run -d \
  -v openpact-workspace:/workspace \
  -e DISCORD_TOKEN=your_token \
  -e GITHUB_TOKEN=ghp_your_token \
  ghcr.io/open-pact/openpact:latest
```

### Enable in Configuration

Enable GitHub integration in `openpact.yaml`:

```yaml
github:
  enabled: true
  # Token is read from GITHUB_TOKEN environment variable
```

## Listing Issues

Use the `github_list_issues` tool to view issues from a repository.

### Tool Usage

```json
{
  "name": "github_list_issues",
  "arguments": {
    "repo": "owner/repository",
    "state": "open"
  }
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `repo` | string | Yes | Repository in `owner/repo` format |
| `state` | string | No | Filter by state: `open`, `closed`, or `all` (default: `open`) |
| `labels` | string | No | Comma-separated list of labels to filter by |
| `limit` | number | No | Maximum issues to return (default: 30) |

### Examples

**List open issues:**
```json
{
  "name": "github_list_issues",
  "arguments": {
    "repo": "open-pact/openpact"
  }
}
```

**List bugs only:**
```json
{
  "name": "github_list_issues",
  "arguments": {
    "repo": "open-pact/openpact",
    "labels": "bug"
  }
}
```

**List all issues (open and closed):**
```json
{
  "name": "github_list_issues",
  "arguments": {
    "repo": "open-pact/openpact",
    "state": "all",
    "limit": 50
  }
}
```

### Response Format

Issues are returned with:

- **Number**: Issue number
- **Title**: Issue title
- **State**: Open or closed
- **Labels**: Assigned labels
- **Created**: Creation date
- **Author**: Issue author
- **URL**: Link to the issue

Example response:
```json
{
  "issues": [
    {
      "number": 42,
      "title": "Add support for multiple calendars",
      "state": "open",
      "labels": ["enhancement", "feature"],
      "created": "2024-01-10T15:30:00Z",
      "author": "johndoe",
      "url": "https://github.com/open-pact/openpact/issues/42"
    },
    {
      "number": 38,
      "title": "Memory file not loading on startup",
      "state": "open",
      "labels": ["bug"],
      "created": "2024-01-08T09:15:00Z",
      "author": "janedoe",
      "url": "https://github.com/open-pact/openpact/issues/38"
    }
  ]
}
```

## Creating Issues

Use the `github_create_issue` tool to create new issues.

### Tool Usage

```json
{
  "name": "github_create_issue",
  "arguments": {
    "repo": "owner/repository",
    "title": "Issue title",
    "body": "Issue description"
  }
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `repo` | string | Yes | Repository in `owner/repo` format |
| `title` | string | Yes | Issue title |
| `body` | string | No | Issue description (markdown supported) |
| `labels` | array | No | Labels to apply |

### Examples

**Create a simple issue:**
```json
{
  "name": "github_create_issue",
  "arguments": {
    "repo": "open-pact/openpact",
    "title": "Add dark mode support"
  }
}
```

**Create a detailed bug report:**
```json
{
  "name": "github_create_issue",
  "arguments": {
    "repo": "open-pact/openpact",
    "title": "Discord messages not received after reconnect",
    "body": "## Description\n\nAfter a Discord reconnect event, the bot stops receiving messages.\n\n## Steps to Reproduce\n\n1. Start OpenPact\n2. Force a disconnect (network interruption)\n3. Wait for reconnection\n4. Try sending a message\n\n## Expected Behavior\n\nMessages should be received after reconnection.\n\n## Actual Behavior\n\nNo messages are received until restart.",
    "labels": ["bug", "discord"]
  }
}
```

**Create a feature request:**
```json
{
  "name": "github_create_issue",
  "arguments": {
    "repo": "open-pact/openpact",
    "title": "Support for Slack integration",
    "body": "It would be great to have Slack as an alternative to Discord for team environments.",
    "labels": ["enhancement", "feature-request"]
  }
}
```

### Response Format

Returns the created issue details:

```json
{
  "number": 45,
  "title": "Add dark mode support",
  "url": "https://github.com/open-pact/openpact/issues/45",
  "state": "open"
}
```

## Repository Permissions

### Access Levels

Your token determines what repositories you can access:

- **Public repositories**: Readable by anyone, writable with `public_repo` scope
- **Private repositories**: Require `repo` scope for full access
- **Organization repositories**: May require additional permissions

### Organization Access

For organization repositories:

1. Ensure your token has access to the organization
2. The organization may require SSO authorization
3. Approve the token in organization settings if required

### Fine-grained Token Permissions

With fine-grained tokens, you can limit access to specific repositories:

1. Choose "Only select repositories"
2. Select the repositories you want OpenPact to access
3. This provides better security than full account access

## Use Cases

### Project Management

```
User: "What open issues do we have on the OpenPact repo?"

AI: Let me check the issues.
[Uses github_list_issues]

You have 12 open issues:
- 3 bugs (2 high priority)
- 7 feature requests
- 2 documentation tasks

Would you like me to summarize any specific category?
```

### Bug Tracking

```
User: "Create an issue for the login bug we discussed"

AI: I'll create that issue for you.
[Uses github_create_issue with details from conversation]

Done! Created issue #48: "Login fails when session expires"
https://github.com/open-pact/openpact/issues/48
```

### Task Organization

```
User: "List all issues labeled 'good first issue' in the repo"

AI: [Uses github_list_issues with labels filter]

Here are the beginner-friendly issues:
1. #23 - Update README with new configuration options
2. #31 - Add unit tests for memory module
3. #35 - Improve error messages in workspace tools
```

### Development Workflow

```
User: "I just fixed issue #42. What should I work on next?"

AI: Let me check the open issues.
[Uses github_list_issues]

Based on priority and your recent work, I'd suggest:
1. Issue #38 (bug) - Related to the area you just worked on
2. Issue #44 (enhancement) - Requested by multiple users
```

## Security Best Practices

### Token Security

1. **Minimal Scope**: Only grant the permissions you need
2. **Expiration**: Set token expiration dates
3. **Rotation**: Regularly rotate your tokens
4. **Secrets**: Never commit tokens to version control

### Repository Selection

1. **Limit Access**: Use fine-grained tokens to limit repository access
2. **Review Permissions**: Periodically review what OpenPact can access
3. **Organization Policies**: Follow your organization's security policies

### Audit Trail

GitHub provides an audit log of API actions:
- View in repository Settings > Security > Audit log
- Monitor for unexpected activity
- Review issues created via API

## Troubleshooting

### Authentication Errors

If you see authentication errors:

1. Verify your token is correct and not expired
2. Check the token has required scopes
3. For organizations, ensure SSO is authorized
4. Try regenerating the token

### Permission Denied

If you can't access a repository:

1. Verify the repository name is correct (`owner/repo`)
2. Check your token has access to that repository
3. For private repos, ensure `repo` scope is granted
4. For organizations, check membership and permissions

### Rate Limiting

GitHub has API rate limits:

- 5,000 requests per hour for authenticated requests
- Monitor usage in response headers
- OpenPact handles rate limiting gracefully

### Issues Not Found

If listing returns no issues:

1. Verify the repository has issues enabled
2. Check the state filter (open/closed/all)
3. Ensure label filters match existing labels
4. Try without filters first

## Related Documentation

- **[MCP Tools Reference](./mcp-tools)** - Complete tool documentation
- **[Configuration Overview](../configuration/overview)** - General configuration
- **[Environment Variables](../configuration/environment-variables)** - Setting `GITHUB_TOKEN`
