---
sidebar_position: 3
title: Managing Scripts
description: Viewing, approving, rejecting scripts, version history, and testing
---

# Managing Scripts

The Admin UI provides a complete workflow for managing Starlark scripts, from creation through approval to execution. This page covers the script lifecycle and how to use the Admin UI to manage it.

## Script Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│                      Script Lifecycle                            │
└─────────────────────────────────────────────────────────────────┘

  AI Creates Script          Admin Reviews              AI Executes
        │                         │                          │
        ▼                         ▼                          ▼
   ┌─────────┐              ┌──────────┐              ┌──────────┐
   │ PENDING │─────────────>│ APPROVED │─────────────>│ EXECUTED │
   └─────────┘   approve    └──────────┘   run        └──────────┘
        │                         │
        │ reject                  │ edit (content changes)
        ▼                         │
   ┌──────────┐                   │
   │ REJECTED │<──────────────────┘
   └──────────┘        (resets to PENDING)
```

### Status Rules

| Action | Resulting Status |
|--------|------------------|
| New script created | `pending` |
| Script edited (content changed) | `pending` (requires re-review) |
| Script approved | `approved` |
| Script rejected | `rejected` |
| Approved script executed | Remains `approved` |

## Scripts List View

Navigate to `/scripts` to see all scripts in your system.

The list displays:

- **Script name** - The filename (e.g., `weather.star`)
- **Status badge** - Color-coded status indicator
- **Description** - From script metadata
- **Required secrets** - Which secrets the script needs
- **Last modified** - When the script was last changed
- **Actions** - Quick access to approve/reject/edit

### Status Badges

| Status | Color | Meaning |
|--------|-------|---------|
| Pending | Yellow | Awaiting admin review |
| Approved | Green | Ready for execution |
| Rejected | Red | Blocked from execution |

## Script Editor

Click on any script to open the editor view.

```
┌─────────────────────────────────────────────────────────────────┐
│  weather.star                              [Test] [Save] [Close]│
├─────────────────────────────────┬───────────────────────────────┤
│                                 │ History                       │
│  # weather.star                 │ ─────────────────────────────│
│  # @secrets: WEATHER_API_KEY    │ ● Current (pending)          │
│                                 │   Modified 2 min ago          │
│  def get_weather(city):         │                               │
│      api_key = secrets.get(...) │ ○ abc123 (approved)          │
│      url = format(              │   Jan 15, 2:30 PM             │
│          "https://api.v2...",   │   "Update API to v2"          │
│          api_key,               │   [View] [Diff] [Restore]     │
│          city                   │                               │
│      )                          │ ○ def456                      │
│      ...                        │   Jan 15, 10:00 AM            │
│                                 │   "Create weather.star"       │
│                                 │   [View] [Diff] [Restore]     │
└─────────────────────────────────┴───────────────────────────────┘
```

### Editor Features

- **Syntax highlighting** - Python/Starlark syntax coloring
- **Line numbers** - For easy reference during review
- **Required secrets indicator** - Shows which secrets the script needs
- **Read-only mode** - Pending scripts show in read-only until approved
- **Version history sidebar** - See all previous versions

## Approving Scripts

To approve a script:

1. Navigate to the script in the Scripts list
2. Review the code carefully
3. Click the **Approve** button
4. The script status changes to `approved`

### Approval Checks

Before approving, verify:

- [ ] The script does what it claims to do
- [ ] No suspicious network requests to unknown domains
- [ ] Secrets are used appropriately and not leaked
- [ ] No attempts to circumvent sandboxing
- [ ] Error handling is adequate
- [ ] Resource usage is reasonable

### API Endpoint

```
POST /api/scripts/:name/approve
```

Response:
```json
{
  "name": "weather.star",
  "status": "approved",
  "approved_at": "2024-01-15T10:30:00Z",
  "approved_by": "admin"
}
```

## Rejecting Scripts

To reject a script:

1. Navigate to the script in the Scripts list
2. Click the **Reject** button
3. Optionally provide a rejection reason
4. The script status changes to `rejected`

### API Endpoint

```
POST /api/scripts/:name/reject
```

Request:
```json
{
  "reason": "Uses unauthorized external endpoint"
}
```

Response:
```json
{
  "name": "suspicious.star",
  "status": "rejected",
  "reason": "Uses unauthorized external endpoint",
  "rejected_at": "2024-01-15T11:00:00Z"
}
```

## Version History

Scripts are automatically version-controlled using Git (managed internally by OpenPact).

### Viewing History

The history sidebar shows all previous versions with:

- Commit hash (short form)
- Timestamp
- Commit message
- Approval status at that version

### Comparing Versions

Click **Diff** to compare any two versions:

```
┌─────────────────────────────────────────────────────────────────┐
│  Comparing: def456 → abc123                            [Close]  │
├────────────────────────────────┬────────────────────────────────┤
│  def456 (Jan 15, 10:00 AM)     │  abc123 (Jan 15, 2:30 PM)     │
├────────────────────────────────┼────────────────────────────────┤
│      url = format(             │      url = format(             │
│-         "https://api.v1...",  │+         "https://api.v2...",  │
│          api_key,              │          api_key,              │
│          city                  │          city                  │
│      )                         │      )                         │
└────────────────────────────────┴────────────────────────────────┘
```

### Restoring Previous Versions

Click **Restore** to revert to a previous version:

1. Select the version you want to restore
2. Click **Restore**
3. A new version is created with the old content
4. The script status resets to `pending` (requires re-approval)

### History API Endpoints

```
GET /api/scripts/:name/history
```

Response:
```json
{
  "versions": [
    {
      "commit": "abc123def456...",
      "message": "Update weather.star via admin UI",
      "author": "admin",
      "timestamp": "2024-01-15T14:30:00Z"
    },
    {
      "commit": "def456abc123...",
      "message": "Create weather.star via admin UI",
      "author": "admin",
      "timestamp": "2024-01-15T10:00:00Z"
    }
  ]
}
```

```
GET /api/scripts/:name/history/:commit
```

Response:
```json
{
  "commit": "abc123def456...",
  "source": "# weather.star\ndef get_weather(city):..."
}
```

```
GET /api/scripts/:name/diff?from=def456&to=abc123
```

Response:
```json
{
  "diff": "@@ -15,7 +15,7 @@ def get_weather(city):\n     url = format(\n-        \"https://api.v1...\",\n+        \"https://api.v2...\","
}
```

```
POST /api/scripts/:name/restore/:commit
```

Response:
```json
{
  "name": "weather.star",
  "status": "pending",
  "restored_from": "def456abc123..."
}
```

## Testing Scripts

Approved scripts can be tested from the Admin UI before production use.

### Running a Test

1. Open an approved script
2. Click the **Test** button
3. Provide test arguments (optional)
4. View the execution result

### Test Execution

```
POST /api/scripts/:name/test
```

Request:
```json
{
  "args": {
    "city": "London"
  }
}
```

Response:
```json
{
  "success": true,
  "result": {
    "temperature": 15.5,
    "conditions": "partly cloudy"
  },
  "duration_ms": 150,
  "logs": [
    "Fetching weather for London...",
    "API responded with 200 OK"
  ]
}
```

:::note
Testing is only available for approved scripts. Pending and rejected scripts cannot be tested.
:::

## Script Allowlisting

For trusted scripts (e.g., those in version control), you can configure an allowlist:

```yaml
scripts:
  allowlist:
    - "weather.star"
    - "jokes.star"
```

Allowlisted scripts:

- Are always considered approved
- Do not require manual approval
- Can be executed immediately
- Still show in the Admin UI for monitoring

## Execution Authorization

When the MCP server receives a script execution request, it performs these checks:

```
1. Is the script on the allowlist?
   YES → Execute immediately
   NO → Continue to step 2

2. Is there an approval record for this script?
   NO → Return "script not approved" error
   YES → Continue to step 3

3. Is the approval status "approved"?
   NO → Return "script not approved" error
   YES → Continue to step 4

4. Does the current script hash match the approved hash?
   NO → Return "script modified" error (requires re-approval)
   YES → Execute the script
```

### Error Responses

When execution is blocked:

**Script not approved:**
```json
{
  "error": "script_not_approved",
  "message": "Script 'new_feature.star' is pending approval. An administrator must review and approve this script before it can be executed.",
  "script": "new_feature.star",
  "status": "pending"
}
```

**Script modified after approval:**
```json
{
  "error": "script_modified",
  "message": "Script 'weather.star' has been modified since approval. Re-approval required.",
  "script": "weather.star",
  "approved_hash": "sha256:abc123...",
  "current_hash": "sha256:def456..."
}
```

## Best Practices

### For Reviewers

1. **Understand the script's purpose** - Read any documentation or comments
2. **Verify external endpoints** - Ensure all URLs are legitimate and expected
3. **Check secret usage** - Secrets should only be used for authentication
4. **Look for data leaks** - Ensure secrets don't appear in return values
5. **Test with sample data** - Use the Test feature before approving

### For Script Authors

1. **Add clear comments** - Explain what the script does and why
2. **Use descriptive function names** - Make the code self-documenting
3. **Return only necessary data** - Don't return URLs or data containing secrets
4. **Handle errors gracefully** - Return meaningful error messages
5. **Keep scripts focused** - One script should do one thing well
