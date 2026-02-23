# OpenPact Dockerfile
# Two-user security model: openpact-system (privileged) and openpact-ai (restricted)

FROM node:22-alpine AS ui-builder

WORKDIR /ui
COPY admin-ui/package.json admin-ui/package-lock.json* ./
RUN npm install
COPY admin-ui/ .
RUN npm run build

# ---

FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
COPY --from=ui-builder /ui/dist/ ./admin-ui/dist/
RUN CGO_ENABLED=0 GOOS=linux go build -o openpact ./cmd/openpact
RUN CGO_ENABLED=0 GOOS=linux go build -o mcp-server ./cmd/mcp-server

# ---

FROM debian:bookworm-slim

# Install dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    gosu \
    openssh-client \
    nodejs \
    npm \
    && rm -rf /var/lib/apt/lists/*

# Install OpenCode (pinned version)
ARG OPENCODE_VERSION=1.1.53
RUN npm install -g opencode-ai@${OPENCODE_VERSION}

# Create users and groups
RUN addgroup --system openpact && \
    adduser --system --home /home/openpact-system --ingroup openpact openpact-system && \
    adduser --system --ingroup openpact openpact-ai

# Create directories with correct permissions
RUN mkdir -p /app /workspace /workspace/secure/data /workspace/engine /workspace/ai-data/memory /workspace/ai-data/skills /workspace/ai-data/scripts /run/mcp && \
    chown -R openpact-system:openpact /app /workspace /run/mcp && \
    chown -R openpact-ai:openpact /workspace/engine && \
    chmod 750 /app /workspace && \
    chmod 700 /workspace/secure && \
    chmod 700 /workspace/secure/data && \
    chmod 770 /workspace/engine && \
    chmod 775 /workspace/ai-data && \
    chmod 770 /run/mcp

# Copy binaries
COPY --from=builder /build/openpact /app/openpact
COPY --from=builder /build/mcp-server /app/mcp-server
RUN chmod 755 /app/openpact /app/mcp-server

# Copy default templates
COPY templates/ /app/templates/
RUN chown -R openpact-system:openpact /app/templates

# Workspace files: system owns, group can read
# (openpact-ai can read but not write directly)

# Create home directory structure (entrypoint symlinks opencode creds into workspace)
RUN mkdir -p /home/openpact-system/.local/share && \
    chown -R openpact-system:openpact /home/openpact-system

# Create home for AI user (OpenCode runs as this user)
RUN mkdir -p /home/openpact-ai/.local/share && \
    chown -R openpact-ai:openpact /home/openpact-ai

ENV HOME=/home/openpact-system

WORKDIR /workspace
VOLUME /workspace

# Copy entrypoint script
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod 755 /app/docker-entrypoint.sh

# Expose admin UI port and OpenCode OAuth callback port
EXPOSE 8080 1455

# Entrypoint runs as root to fix bind-mount permissions, then drops to openpact-system
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["start"]
