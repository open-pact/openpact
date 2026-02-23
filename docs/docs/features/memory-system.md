---
title: Memory System
sidebar_position: 4
---

# Memory System

OpenPact provides a persistent memory system that allows your AI assistant to remember information across conversations. This enables continuity, personalization, and the ability to track long-term tasks and preferences.

## How Memory Works

The memory system consists of two components:

1. **Long-term Memory (MEMORY.md)**: Persistent facts, preferences, and important information
2. **Daily Memory Files (memory/YYYY-MM-DD.md)**: Day-specific notes and context

Both are stored in the workspace and automatically loaded into the AI's context.

```
workspace/
├── MEMORY.md              # Long-term memory
└── memory/
    ├── 2024-01-15.md      # Daily memory
    ├── 2024-01-16.md
    └── 2024-01-17.md
```

### Context Loading

When a conversation starts, OpenPact loads:

1. The full contents of `MEMORY.md`
2. Today's daily memory file (if it exists)
3. Recent daily memory files (configurable)

This gives the AI context about you and recent events without you having to repeat information.

## Long-term Memory (MEMORY.md)

`MEMORY.md` stores persistent information that remains relevant over time.

### What to Store

- Personal preferences
- Important facts about you
- Ongoing projects and goals
- Key relationships and contacts
- System preferences and conventions

### Example MEMORY.md

```markdown
# Memory

## About the User
- Name: Alex
- Location: San Francisco, CA
- Timezone: America/Los_Angeles
- Preferred communication style: Concise and direct

## Preferences
- Coffee: Oat milk latte, no sugar
- Music: Jazz and lo-fi while working
- News sources: Hacker News, The Verge

## Ongoing Projects
- **Home Automation**: Setting up Home Assistant
- **Learning Rust**: Working through "The Rust Book"
- **Garden**: Growing tomatoes and herbs

## Important Contacts
- **Dr. Smith**: Primary care physician, appointments via MyChart
- **Sarah**: Project manager at work, prefers Slack

## Reminders
- Take vitamins with breakfast
- Water plants every Sunday
- Monthly budget review on the 1st
```

## Daily Memory Files

Daily memory files (stored in `memory/YYYY-MM-DD.md`) capture day-specific information.

### Automatic vs Manual

Your AI can create and update daily memory files:

- **Automatically**: AI notes important things from conversations
- **Manually**: You explicitly ask the AI to remember something

### What to Store

- Tasks completed today
- Important conversations
- Decisions made
- Things to follow up on
- Daily observations

### Example Daily Memory

```markdown
# 2024-01-15

## Conversations
- Discussed Q1 project timeline with Sarah
- Reviewed code for authentication module

## Tasks Completed
- [x] Submitted expense report
- [x] Updated project documentation
- [x] Fixed login bug (issue #234)

## Notes
- Sarah mentioned new deadline is February 15
- Need to order more coffee beans
- Weather turning cold this week

## Follow-ups
- Send meeting notes to team by tomorrow
- Review PR from Mike
```

## Memory Tools

OpenPact provides two MCP tools for memory operations.

### memory_read

Read from a memory file.

```json
{
  "name": "memory_read",
  "arguments": {
    "file": "MEMORY.md"
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file` | string | No | Memory file name (defaults to MEMORY.md) |

**Examples:**

```json
// Read long-term memory
{ "name": "memory_read", "arguments": {} }

// Read today's memory
{ "name": "memory_read", "arguments": { "file": "memory/2024-01-15.md" } }
```

### memory_write

Write to a memory file.

```json
{
  "name": "memory_write",
  "arguments": {
    "content": "## New Section\n\nImportant information to remember.",
    "append": true
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `content` | string | Yes | Content to write |
| `file` | string | No | Memory file name (defaults to MEMORY.md) |
| `append` | boolean | No | Append to file instead of replacing (default: false) |

**Examples:**

```json
// Append to long-term memory
{
  "name": "memory_write",
  "arguments": {
    "content": "\n## New Project\n- Started learning Go",
    "append": true
  }
}

// Create/update today's memory
{
  "name": "memory_write",
  "arguments": {
    "file": "memory/2024-01-15.md",
    "content": "# 2024-01-15\n\n## Notes\n- Had productive morning"
  }
}
```

## Memory Patterns

### Asking the AI to Remember

You can explicitly ask your AI to remember things:

```
User: "Remember that my favorite restaurant is Chez Panisse"

AI: I'll add that to your memory.
[Uses memory_write to append to MEMORY.md]

Done! I've noted that your favorite restaurant is Chez Panisse.
```

### AI Proactively Remembering

During conversations, the AI may note important information:

```
User: "I just got promoted to Senior Engineer!"

AI: Congratulations! That's wonderful news!
[Uses memory_write to update MEMORY.md with career milestone]
```

### Recalling Information

The AI automatically has access to memory during conversations:

```
User: "What's my favorite coffee order?"

AI: Your favorite is an oat milk latte with no sugar.
[Retrieved from MEMORY.md without needing to use any tools]
```

## Best Practices

### Structure Long-term Memory

Use clear sections and formatting:

```markdown
# Memory

## Personal
[Personal information]

## Work
[Work-related information]

## Preferences
[Likes, dislikes, preferences]

## Projects
[Ongoing projects and goals]
```

### Keep Memory Relevant

Periodically review and clean up memory:

- Remove outdated information
- Update changed preferences
- Archive completed projects

### Use Daily Memory for Context

Let daily memory capture the ephemeral:

```markdown
# 2024-01-15

## Today's Context
- Working from home
- Feeling under the weather
- Need to leave early for appointment

## Action Items
- [ ] Review pull request
- [ ] Send status update
```

### Privacy Considerations

Remember that memory contents are sent to the AI provider. Avoid storing:

- Passwords or secrets
- Highly sensitive personal data
- Financial account numbers
- Medical details (unless comfortable)

## Configuration

Memory behavior can be configured in `openpact.yaml`:

```yaml
workspace:
  path: /workspace

# Memory files are stored within the workspace
# MEMORY.md at workspace root
# Daily files in workspace/memory/
```

## Troubleshooting

### AI Doesn't Remember

If the AI seems to forget information:

1. Check that `MEMORY.md` exists and contains the information
2. Verify the memory file is in the workspace
3. Look at logs to confirm memory was loaded at startup

### Memory Not Updating

If `memory_write` doesn't seem to work:

1. Check workspace write permissions
2. Verify the path is correct
3. Review logs for any errors

### Daily Memory Not Loading

If today's context seems missing:

1. Check that the daily memory file exists
2. Verify the date format: `YYYY-MM-DD.md`
3. Ensure the file is in the `memory/` directory

## Related Documentation

- **[MCP Tools Reference](./mcp-tools)** - Complete tool documentation
- **[Workspace](./workspace)** - Workspace file management
- **[Context Files](../configuration/context-files)** - SOUL.md and USER.md
