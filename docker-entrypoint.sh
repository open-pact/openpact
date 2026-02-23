#!/bin/sh
# Two-user security model:
#   openpact-system: Runs the orchestrator, admin UI, owns secrets/config
#   openpact-ai:     Runs OpenCode (AI engine), restricted file access
#
# Both users are in the 'openpact' group. File permissions use group
# membership to give the AI user read access to workspace files while
# keeping secrets (data dir, config) owner-only.

# Workspace root: system owns, group can traverse
chown openpact-system:openpact /workspace
chmod 750 /workspace

# Ensure workspace subdirectories exist with correct ownership
mkdir -p /workspace/data/opencode /workspace/memory /workspace/skills /workspace/scripts

# Data dir: owner only (contains secrets, provider tokens, jwt key, config)
chown -R openpact-system:openpact /workspace/data
chmod 700 /workspace/data

# Memory dir: group can read+write (AI writes memory through MCP tools)
chown -R openpact-system:openpact /workspace/memory
chmod 770 /workspace/memory

# Skills dir: group can read
chown -R openpact-system:openpact /workspace/skills
chmod 750 /workspace/skills

# Scripts dir: group can read (AI can list/read scripts, admin writes them)
chown -R openpact-system:openpact /workspace/scripts
chmod 750 /workspace/scripts

# Symlink OpenCode creds into the workspace so they persist via the bind-mounted volume.
# Both system and AI user need their own symlink for auth state.
ln -sfn /workspace/data/opencode /home/openpact-system/.local/share/opencode
ln -sfn /workspace/data/opencode /home/openpact-ai/.local/share/opencode

# Copy default config if none exists
if [ ! -f /workspace/config.yaml ]; then
    cp /app/templates/config.yaml /workspace/config.yaml
fi
# Config file: owner only (may contain passwords)
chown openpact-system:openpact /workspace/config.yaml 2>/dev/null || true
chmod 600 /workspace/config.yaml

# Copy context templates if they don't exist
for tmpl in SOUL.md USER.md MEMORY.md; do
    if [ ! -f "/workspace/$tmpl" ]; then
        cp "/app/templates/$tmpl" "/workspace/$tmpl"
    fi
done
# Context files: group can read (AI needs these for system prompt injection)
# MEMORY.md is group-writable so MCP memory_write tool can update it
chown openpact-system:openpact /workspace/SOUL.md /workspace/USER.md /workspace/MEMORY.md 2>/dev/null || true
chmod 640 /workspace/SOUL.md /workspace/USER.md 2>/dev/null || true
chmod 660 /workspace/MEMORY.md 2>/dev/null || true

# Drop to unprivileged user
exec gosu openpact-system /app/openpact "$@"
