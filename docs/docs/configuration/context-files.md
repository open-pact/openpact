---
title: Context Files
sidebar_position: 4
---

# Context Files

OpenPact uses special markdown files to shape the AI's behavior, personality, and memory. These files are injected into the AI's context at the start of each conversation.

## Overview

| File | Purpose | AI Access |
|------|---------|-----------|
| `SOUL.md` | Core identity and personality | Read only |
| `USER.md` | User preferences and context | Read only |
| `MEMORY.md` | Persistent notes and memory | Read and write |

All context files are stored in the workspace directory (default: `/workspace`).

## SOUL.md

The `SOUL.md` file defines the AI's core identity, personality, and behavioral guidelines. Think of it as the AI's "character sheet."

### Location

```
/workspace/SOUL.md
```

### Example

```markdown
# Identity

You are Alex, a helpful personal AI assistant. You work for [User Name] and help them with daily tasks, research, and project management.

## Personality

- Friendly and professional
- Concise but thorough
- Proactive about suggesting improvements
- Honest about limitations

## Communication Style

- Use clear, direct language
- Avoid jargon unless the user uses it first
- Ask clarifying questions when requirements are ambiguous
- Summarize long responses with key points

## Guidelines

### Always
- Respect user privacy
- Admit when you don't know something
- Ask before taking significant actions
- Provide sources when making factual claims

### Never
- Share user information with others
- Make assumptions about sensitive topics
- Provide medical, legal, or financial advice as if you were a professional
- Execute destructive actions without confirmation

## Expertise Areas

- Software development (Python, Go, JavaScript)
- Project management
- Research and analysis
- Writing and editing
```

### Best Practices

1. **Be specific** - Vague instructions lead to inconsistent behavior
2. **Set boundaries** - Clearly state what the AI should and shouldn't do
3. **Define expertise** - Help the AI understand its strengths
4. **Include examples** - Show the communication style you want

## USER.md

The `USER.md` file contains information about you - the user. This helps the AI personalize its responses and understand your context.

### Location

```
/workspace/USER.md
```

### Example

```markdown
# User Profile

## Basic Info

- Name: Jamie Smith
- Location: San Francisco, CA
- Timezone: America/Los_Angeles
- Preferred Language: English

## Work

- Role: Senior Software Engineer at TechCorp
- Team: Platform Infrastructure
- Current Focus: Microservices migration project

## Preferences

### Communication
- Prefers concise, bullet-point responses
- Likes code examples over lengthy explanations
- Appreciates proactive suggestions

### Technical
- Primary languages: Go, Python, TypeScript
- Editor: VS Code
- OS: macOS

### Schedule
- Working hours: 9 AM - 6 PM Pacific
- Focus time: Mornings (don't interrupt with non-urgent items)
- Meetings: Usually afternoons

## Current Projects

### Microservices Migration
- Status: In progress
- Goal: Break monolith into 12 services
- Timeline: Q2 2024
- Tech stack: Go, Kubernetes, PostgreSQL

### Personal
- Learning Rust on weekends
- Building a home automation system

## Important Dates

- Team standup: Daily 10 AM
- Sprint planning: Mondays 2 PM
- 1:1 with manager: Fridays 3 PM
```

### What to Include

- **Personal context**: Name, timezone, preferences
- **Professional context**: Role, projects, technologies
- **Communication preferences**: How you like to receive information
- **Current focus**: What you're working on right now

## MEMORY.md

The `MEMORY.md` file is the AI's persistent memory. Unlike SOUL.md and USER.md, the AI can read and update this file using the `memory_write` tool.

### Location

```
/workspace/MEMORY.md
```

### Example

```markdown
# Memory

## Important Notes

- User prefers dark mode in all applications
- Project deadline moved to March 15
- API rate limit issue resolved with caching

## Ongoing Tasks

- [ ] Review PR #234 for authentication service
- [x] Set up monitoring dashboards
- [ ] Write documentation for new API endpoints

## Recent Decisions

### 2024-01-15
- Decided to use PostgreSQL instead of MongoDB for the user service
- Reason: Better transaction support, team familiarity

### 2024-01-10
- Chose Kubernetes over ECS for container orchestration
- Reason: Multi-cloud flexibility, existing expertise

## Learned Preferences

- User likes responses under 200 words when possible
- Always include code examples for technical questions
- Check calendar before scheduling suggestions

## Context from Previous Conversations

- Discussed microservices patterns on 2024-01-12
- User mentioned interest in event-driven architecture
- Team is considering GraphQL for the new API
```

### How Memory Works

1. **AI reads**: Memory is included in context at conversation start
2. **AI updates**: AI can use `memory_write` to add or modify notes
3. **Persistent**: Changes survive between conversations
4. **User editable**: You can edit the file directly too

### Memory Best Practices

1. **Structure it**: Use headings and lists for organization
2. **Date entries**: Include dates for time-sensitive information
3. **Review periodically**: Remove outdated information
4. **Keep it focused**: Don't let it grow too large (impacts context limits)

## Daily Memory Files

In addition to `MEMORY.md`, OpenPact supports daily memory files for ephemeral notes.

### Location

```
/workspace/memory/YYYY-MM-DD.md
```

### Purpose

- Short-term notes that don't need to persist forever
- Daily task tracking
- Conversation summaries
- Temporary context

### Example

```markdown
# 2024-01-15

## Today's Focus
- Finish authentication service review
- Prepare for sprint planning

## Notes
- User mentioned they'll be in meetings 2-5 PM
- Deployment scheduled for 6 PM

## Conversation Summary
- Discussed API rate limiting approaches
- Recommended token bucket algorithm
- User will implement tomorrow
```

## Context File Loading

### Load Order

1. `SOUL.md` - Always loaded first (identity)
2. `USER.md` - Loaded second (user context)
3. `MEMORY.md` - Loaded third (persistent memory)
4. Daily memory - Loaded if exists for current date

### Context Limits

Be aware that context files consume tokens from the AI's context window. Very large files may:
- Slow down responses
- Increase costs
- Leave less room for conversation

**Recommendations:**
- Keep SOUL.md under 500 lines
- Keep USER.md under 300 lines
- Keep MEMORY.md under 500 lines
- Archive old memory periodically

## Creating Context Files

### Manual Creation

Create files directly in your workspace:

```bash
# Docker - copy files into container
docker cp SOUL.md openpact:/workspace/SOUL.md

# Docker Compose - mount from host
volumes:
  - ./context/SOUL.md:/workspace/SOUL.md:ro
  - ./context/USER.md:/workspace/USER.md:ro
  - ./workspace:/workspace
```

### Using Templates

OpenPact includes default templates. You can:

1. Start with defaults and customize
2. Have the AI help you write them
3. Use the examples above as starting points

### AI Assistance

Ask the AI to help populate your context files:

> "Help me create a USER.md file. Ask me questions about my preferences and work context, then generate the file."

## Troubleshooting

### Changes Not Taking Effect

Context files are loaded at conversation start. For changes to take effect:
- Start a new conversation, or
- Restart OpenPact

### File Not Found

Check:
1. File is in the correct location (`/workspace/`)
2. File name is correct (case-sensitive)
3. File has `.md` extension
4. Container has read access

### Memory Not Persisting

If `MEMORY.md` changes aren't saving:
1. Check workspace volume is mounted correctly
2. Verify container has write permissions
3. Check logs for file write errors
