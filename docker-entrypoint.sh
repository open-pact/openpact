#!/bin/sh
# Give openpact-system ownership of the workspace bind mount so the app
# can create subdirectories. Keep 755 so the host user can still read it
# (and Docker can include it in the build context).
chown openpact-system:openpact /workspace
chmod 755 /workspace

# Ensure workspace subdirectories exist with correct ownership.
mkdir -p /workspace/data/opencode /workspace/memory /workspace/skills /workspace/scripts
chown -R openpact-system:openpact /workspace/data /workspace/memory /workspace/skills /workspace/scripts

# Symlink OpenCode creds into the workspace so they persist via the bind-mounted volume.
ln -sfn /workspace/data/opencode /home/openpact-system/.local/share/opencode

# Copy default config if none exists
if [ ! -f /workspace/config.yaml ]; then
    cp /app/templates/config.yaml /workspace/config.yaml
fi
chown openpact-system:openpact /workspace/config.yaml 2>/dev/null || true

# Copy context templates if they don't exist
for tmpl in SOUL.md USER.md MEMORY.md; do
    if [ ! -f "/workspace/$tmpl" ]; then
        cp "/app/templates/$tmpl" "/workspace/$tmpl"
    fi
done
chown openpact-system:openpact /workspace/SOUL.md /workspace/USER.md /workspace/MEMORY.md 2>/dev/null || true

# Drop to unprivileged user
exec gosu openpact-system /app/openpact "$@"
