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
chmod -R 775 /workspace/engine

# Create AI-accessible area (group-readable/writable for AI user)
mkdir -p /workspace/ai-data/memory /workspace/ai-data/skills /workspace/ai-data/scripts

chown -R openpact-system:openpact /workspace/ai-data
chmod 775 /workspace/ai-data
chmod 775 /workspace/ai-data/memory
chmod 755 /workspace/ai-data/skills
chmod 755 /workspace/ai-data/scripts
# Ensure AI user (openpact-ai, same group) can write files created by either user
find /workspace/ai-data/memory -type f -exec chmod g+w {} + 2>/dev/null || true

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

# --- Launch OpenCode as openpact-ai with a restart loop ---

# Generate OpenCode config JSON using Go (single source of truth)
OC_CONFIG=$(/app/openpact opencode-config)
if [ $? -ne 0 ]; then
    echo "FATAL: failed to generate OpenCode config" >&2
    exit 1
fi

# Build the allowlisted environment for the AI process.
# Only system basics and LLM provider keys are passed through.
OC_ENV=""
for key in ANTHROPIC_API_KEY OPENAI_API_KEY GOOGLE_API_KEY AZURE_OPENAI_API_KEY OLLAMA_HOST; do
    val=$(eval echo "\$$key")
    if [ -n "$val" ]; then
        OC_ENV="$OC_ENV $key=$val"
    fi
done

# Read optional password from config (the Go binary already loaded it, but
# we need it for the env var). Use a simple grep since it's YAML.
OC_PASSWORD=$(grep -oP '^\s*password:\s*\K\S+' /workspace/secure/config.yaml 2>/dev/null || true)

start_opencode() {
    while true; do
        echo "Starting opencode serve on port 4098 as openpact-ai..."
        gosu openpact-ai env \
            OPENCODE_CONFIG_CONTENT="$OC_CONFIG" \
            HOME=/home/openpact-ai \
            USER=openpact-ai \
            PATH="$PATH" \
            LANG="${LANG:-C.UTF-8}" \
            TERM="${TERM:-xterm}" \
            ${OC_PASSWORD:+OPENCODE_SERVER_PASSWORD=$OC_PASSWORD} \
            $OC_ENV \
            opencode serve --port 4098 --hostname 127.0.0.1
        echo "opencode exited ($?), restarting in 2s..."
        sleep 2
    done
}

start_opencode &
OPENCODE_PID=$!

# Clean up the background loop on exit
cleanup() {
    echo "Stopping opencode restart loop (pid $OPENCODE_PID)..."
    kill $OPENCODE_PID 2>/dev/null
    wait $OPENCODE_PID 2>/dev/null
}
trap cleanup EXIT

# Drop to unprivileged user and start the orchestrator
exec gosu openpact-system /app/openpact "$@"
