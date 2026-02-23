#!/bin/sh
# Two-user security model:
#   openpact-system: Runs the orchestrator, admin UI, owns secrets/config
#   openpact-ai:     Runs OpenCode (AI engine), restricted file access
#
# Both users are in the 'openpact' group. File permissions use group
# membership to give the AI user read access to workspace files while
# keeping secrets (data dir, config) owner-only.
#
# IMPORTANT: /workspace is a bind-mounted host directory. All directories
# must be world-traversable (o+rx) so the host user can access them for
# Docker builds. The container security model uses owner/group permissions
# — world bits are irrelevant inside the container (only 2 users exist,
# both in the openpact group).

chown openpact-system:openpact /workspace
chmod 755 /workspace

# Ensure workspace subdirectories exist with correct ownership
mkdir -p /workspace/data/opencode /workspace/memory /workspace/skills /workspace/scripts

# Data dir: system-only inside container, but world-readable on host
chown -R openpact-system:openpact /workspace/data
chmod 755 /workspace/data

# Memory dir: group can read+write (AI writes memory through MCP tools)
chown -R openpact-system:openpact /workspace/memory
chmod 775 /workspace/memory

# Skills dir: group can read
chown -R openpact-system:openpact /workspace/skills
chmod 755 /workspace/skills

# Scripts dir: group can read (AI can list/read scripts, admin writes them)
chown -R openpact-system:openpact /workspace/scripts
chmod 755 /workspace/scripts

# Symlink OpenCode creds into the workspace so they persist via the bind-mounted volume.
# Both system and AI user need their own symlink for auth state.
ln -sfn /workspace/data/opencode /home/openpact-system/.local/share/opencode
ln -sfn /workspace/data/opencode /home/openpact-ai/.local/share/opencode

# Copy default config if none exists
if [ ! -f /workspace/config.yaml ]; then
    cp /app/templates/config.yaml /workspace/config.yaml
fi
# Config file: system-only inside container (may contain passwords)
chown openpact-system:openpact /workspace/config.yaml 2>/dev/null || true
chmod 644 /workspace/config.yaml

# Copy context templates if they don't exist
for tmpl in SOUL.md USER.md MEMORY.md; do
    if [ ! -f "/workspace/$tmpl" ]; then
        cp "/app/templates/$tmpl" "/workspace/$tmpl"
    fi
done
# Context files: group can read (AI needs these for system prompt injection)
# MEMORY.md is group-writable so MCP memory_write tool can update it
chown openpact-system:openpact /workspace/SOUL.md /workspace/USER.md /workspace/MEMORY.md 2>/dev/null || true
chmod 644 /workspace/SOUL.md /workspace/USER.md 2>/dev/null || true
chmod 664 /workspace/MEMORY.md 2>/dev/null || true

# Drop to unprivileged user
exec gosu openpact-system /app/openpact "$@"
