#!/bin/sh
# Two-user security model:
#   openpact-system: Runs the orchestrator, admin UI, owns secrets/config
#   openpact-ai:     Runs OpenCode (AI engine), restricted file access
#
# Directory structure:
#   /workspace/secure/       — SYSTEM-ONLY (config, secrets, system data)
#   /workspace/engine/       — ENGINE data (OpenCode auth, sessions — AI user needs access)
#   /workspace/ai-data/      — AI-ACCESSIBLE (context files, memory, scripts, skills)
#
# Both users are in the 'openpact' group. File permissions use group
# membership to give the AI user access to ai-data/ while keeping
# secure/ owner-only.

chown openpact-system:openpact /workspace
chmod 755 /workspace

# Create secure area (system-only)
mkdir -p /workspace/secure/data

chown -R openpact-system:openpact /workspace/secure
chmod 700 /workspace/secure
chmod 700 /workspace/secure/data

# Create engine data area (AI user needs access for OpenCode auth/sessions)
mkdir -p /workspace/engine

chown -R openpact-ai:openpact /workspace/engine
chmod 770 /workspace/engine

# Create AI-accessible area (group-readable/writable for AI user)
mkdir -p /workspace/ai-data/memory /workspace/ai-data/skills /workspace/ai-data/scripts

chown -R openpact-system:openpact /workspace/ai-data
chmod 775 /workspace/ai-data
chmod 775 /workspace/ai-data/memory
chmod 755 /workspace/ai-data/skills
chmod 755 /workspace/ai-data/scripts

# Symlink OpenCode data dir so auth state persists via the bind-mounted volume.
ln -sfn /workspace/engine /home/openpact-system/.local/share/opencode
ln -sfn /workspace/engine /home/openpact-ai/.local/share/opencode

# Copy default config if none exists
if [ ! -f /workspace/secure/config.yaml ]; then
    cp /app/templates/config.yaml /workspace/secure/config.yaml
fi
# Config file: system-only inside container (may contain passwords)
chown openpact-system:openpact /workspace/secure/config.yaml 2>/dev/null || true
chmod 644 /workspace/secure/config.yaml

# Copy context templates if they don't exist
for tmpl in SOUL.md USER.md MEMORY.md; do
    if [ ! -f "/workspace/ai-data/$tmpl" ]; then
        cp "/app/templates/$tmpl" "/workspace/ai-data/$tmpl"
    fi
done
# Context files: group can read (AI needs these for system prompt injection)
# MEMORY.md is group-writable so MCP memory_write tool can update it
chown openpact-system:openpact /workspace/ai-data/SOUL.md /workspace/ai-data/USER.md /workspace/ai-data/MEMORY.md 2>/dev/null || true
chmod 644 /workspace/ai-data/SOUL.md /workspace/ai-data/USER.md 2>/dev/null || true
chmod 664 /workspace/ai-data/MEMORY.md 2>/dev/null || true

# Drop to unprivileged user
exec gosu openpact-system /app/openpact "$@"
