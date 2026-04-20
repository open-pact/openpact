# OpenPact Dockerfile
#
# Single static binary, single user. Everything the app needs — the LLM
# engine (stackllm, in-process), the MCP tool registry (native Go), the
# admin UI (embedded via //go:embed), and the SQLite session store
# (pure-Go modernc.org/sqlite) — ships inside one Go binary. No Node,
# no opencode, no separate processes.

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

# ca-certificates is required for outbound TLS (provider APIs).
# git is kept for the Obsidian vault integration's auto-sync path.
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --system --create-home --home-dir /home/openpact openpact

RUN mkdir -p /app /workspace && \
    chown -R openpact:openpact /app /workspace && \
    chmod 755 /app /workspace

COPY --from=builder /build/openpact /app/openpact
COPY --from=builder /build/mcp-server /app/mcp-server
RUN chmod 755 /app/openpact /app/mcp-server

COPY templates/ /app/templates/
RUN chown -R openpact:openpact /app/templates

ENV HOME=/home/openpact
ENV WORKSPACE_PATH=/workspace
ENV ADMIN_BIND=0.0.0.0:8888

USER openpact
WORKDIR /workspace
VOLUME /workspace

EXPOSE 8888

ENTRYPOINT ["/app/openpact"]
CMD ["start"]
