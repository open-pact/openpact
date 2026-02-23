---
sidebar_position: 5
title: Script Approval
description: Managing and approving Starlark scripts through the Admin UI
---

# Script Approval Workflow

OpenPact provides an approval workflow for Starlark scripts through the Admin UI. This ensures that only reviewed and authorized scripts can be executed by the AI.

## Why Script Approval?

Even though Starlark scripts run in a sandbox, they can still:

- Make HTTP requests to external services
- Access configured secrets
- Return data to the AI

The approval workflow provides:

- **Oversight:** Review scripts before they're available to the AI
- **Audit trail:** Track who approved what and when
- **Control:** Disable scripts without deleting them
- **Security:** Prevent unauthorized scripts from being executed

## Approval States

Scripts can be in one of three states:

| State | Description | AI Can Execute? |
|-------|-------------|-----------------|
| **Pending** | New or modified script awaiting review | No |
| **Approved** | Reviewed and authorized for use | Yes |
| **Rejected** | Reviewed and not authorized | No |

## Admin UI Overview

The Admin UI provides a dedicated section for script management:

### Script List View

```
┌─────────────────────────────────────────────────────────────────┐
│  Scripts                                            [+ Add New] │
├─────────────────────────────────────────────────────────────────┤
│  Name          │ Status    │ Version │ Last Modified │ Actions │
├─────────────────────────────────────────────────────────────────┤
│  weather       │ Approved  │ 1.0.0   │ 2026-02-05   │ [View]  │
│  stocks        │ Pending   │ 1.2.0   │ 2026-02-05   │ [View]  │
│  notification  │ Rejected  │ 1.0.0   │ 2026-02-04   │ [View]  │
└─────────────────────────────────────────────────────────────────┘
```

### Script Detail View

```
┌─────────────────────────────────────────────────────────────────┐
│  weather.star                                          [Edit]   │
├─────────────────────────────────────────────────────────────────┤
│  Status: Approved                                               │
│  Version: 1.0.0                                                 │
│  Author: Admin                                                  │
│  Description: Get current weather for a city                    │
│  Secrets: WEATHER_API_KEY                                       │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ # @description: Get current weather for a city          │   │
│  │ # @secrets: WEATHER_API_KEY                              │   │
│  │                                                          │   │
│  │ def get_weather(city):                                   │   │
│  │     api_key = secrets.get("WEATHER_API_KEY")             │   │
│  │     ...                                                  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  [Approve] [Reject] [Test] [Delete]                            │
└─────────────────────────────────────────────────────────────────┘
```

## Approval Workflow

### 1. Script Creation

When a new script is added to the scripts directory:

1. OpenPact detects the new file
2. Script is parsed and validated for syntax errors
3. Script is added to the database with "Pending" status
4. Admin is notified (if notifications are configured)

### 2. Script Review

The admin reviews the script in the Admin UI:

1. Navigate to Scripts section
2. Click on the pending script
3. Review the code for:
   - Security concerns
   - Appropriate secret usage
   - Correct functionality
4. Optionally test the script

### 3. Testing Scripts

Before approving, you can test scripts:

```
┌─────────────────────────────────────────────────────────────────┐
│  Test Script: weather.star                                       │
├─────────────────────────────────────────────────────────────────┤
│  Function: get_weather                                          │
│  Arguments: ["London"]                                          │
│                                                                 │
│  [Run Test]                                                     │
│                                                                 │
│  Result:                                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ {                                                        │   │
│  │   "city": "London",                                      │   │
│  │   "temp_c": 15.5,                                        │   │
│  │   "condition": "Partly cloudy"                           │   │
│  │ }                                                        │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 4. Approval Decision

After review:

- **Approve:** Script becomes available to the AI
- **Reject:** Script remains unavailable, with optional rejection reason

### 5. Script Modification

When an approved script is modified:

1. The script status changes back to "Pending"
2. A new version is created
3. The previous approved version remains active until the new version is approved
4. Admin reviews the changes

## Version History

The Admin UI maintains a version history for each script:

```
┌─────────────────────────────────────────────────────────────────┐
│  Version History: weather.star                                   │
├─────────────────────────────────────────────────────────────────┤
│  Version │ Status    │ Modified     │ Approved By │ Actions    │
├─────────────────────────────────────────────────────────────────┤
│  1.2.0   │ Pending   │ 2026-02-05   │ -           │ [View]     │
│  1.1.0   │ Approved  │ 2026-02-03   │ admin       │ [View]     │
│  1.0.0   │ Approved  │ 2026-02-01   │ admin       │ [Rollback] │
└─────────────────────────────────────────────────────────────────┘
```

You can:
- View previous versions
- Compare versions (diff view)
- Rollback to a previous approved version

## Allowlisting

For trusted scripts or development environments, you can configure automatic approval:

### Configuration

In `openpact.yaml`:

```yaml
starlark:
  enabled: true
  approval:
    required: true                    # Require approval for new scripts
    auto_approve_patterns:            # Auto-approve matching scripts
      - "trusted/*"                   # All scripts in trusted/ directory
      - "weather.star"                # Specific script
    notify_on_pending: true           # Send notification when scripts need approval
```

### Auto-Approval Rules

| Pattern | Matches |
|---------|---------|
| `*.star` | All scripts |
| `trusted/*` | Scripts in trusted/ subdirectory |
| `weather.star` | Specific script by name |
| `api-*.star` | Scripts starting with "api-" |

:::caution
Use auto-approval carefully. It bypasses the review process and should only be used for:
- Development environments
- Scripts from trusted sources
- Directories with restricted write access
:::

## Script Reload

After approving scripts or making changes, reload the script cache:

**From Admin UI:** Click the "Reload Scripts" button

**From AI:** Use the `script_reload` tool

```
Tool: script_reload
Args: {}
```

**Automatic:** OpenPact can watch for file changes (if configured)

```yaml
starlark:
  watch_for_changes: true  # Automatically reload on file changes
```

## Audit Logging

All script-related actions are logged:

```
2026-02-05T10:30:00Z INFO script approved name=weather.star version=1.0.0 approved_by=admin
2026-02-05T10:31:00Z INFO script executed name=weather.star function=get_weather user=ai
2026-02-05T10:32:00Z INFO script rejected name=unsafe.star version=1.0.0 rejected_by=admin reason="Accesses unauthorized endpoints"
```

## Best Practices

### 1. Review All Scripts Carefully

Before approving, verify:
- [ ] Script only accesses intended APIs
- [ ] Secrets used are appropriate for the script's purpose
- [ ] No sensitive data is returned unnecessarily
- [ ] Error handling doesn't expose sensitive information
- [ ] Script follows your organization's coding standards

### 2. Use Descriptive Metadata

Encourage script authors to include metadata:

```python
# @description: Fetch current stock price from Alpha Vantage
# @author: John Smith
# @version: 1.0.0
# @secrets: ALPHA_VANTAGE_API_KEY
# @approved_for: stock-related queries only
```

### 3. Limit Secret Access

Configure only the secrets each script needs:

```yaml
starlark:
  secrets:
    # Don't give all scripts access to all secrets
    WEATHER_API_KEY: "${WEATHER_API_KEY}"  # Only for weather.star
    STOCK_API_KEY: "${STOCK_API_KEY}"      # Only for stocks.star
```

### 4. Implement Change Control

For production environments:
- Require approval from multiple reviewers
- Document why scripts were approved or rejected
- Maintain an audit trail
- Periodically review approved scripts

### 5. Test Before Approving

Always test scripts with realistic inputs:
- Valid inputs (happy path)
- Invalid inputs (error handling)
- Edge cases (empty values, large inputs)
- Missing secrets (graceful degradation)
