---
sidebar_position: 1
title: Overview
description: Security philosophy, threat model, and defense in depth
---

# Security Overview

OpenPact is designed with security as a foundational principle. This page outlines the security philosophy, threat model, and the defense-in-depth approach that protects your systems.

## Security Philosophy

### 1. Assume the AI is untrusted

While AI assistants are powerful tools, they should not be given unrestricted access to sensitive resources. OpenPact treats AI-generated code and tool calls as untrusted input that must be validated and sandboxed.

### 2. Defense in depth

No single security control is sufficient. OpenPact layers multiple security mechanisms so that if one fails, others continue to protect the system.

### 3. Principle of least privilege

Every component receives only the minimum permissions necessary. Tools cannot reach resources they don't need; the AI cannot see secrets it shouldn't know.

### 4. Secure by default

OpenPact ships with secure defaults. Features that could compromise security are opt-in, and dangerous operations require explicit approval.

### 5. Transparent security

Security mechanisms should be understandable. Rather than relying on obscurity, OpenPact's security model is documented and auditable.

## Threat Model

### Malicious / hijacked agent behavior

**Threat:** prompt injection, jailbreaks, or a mis-aligned model attempt to call destructive tools, exfiltrate secrets, or pivot to unauthorized systems.

**Mitigations:**

- **Tool registry as the security perimeter** — the agent can only call tools that have been registered. There is no shell, no `eval`, no arbitrary file IO unless explicitly registered.
- **In-tool scoping** — workspace tools enforce the `ai-data/` boundary; vault tools are confined to the configured vault path; chat-send tools respect allow lists; web fetch is HTTPS-only with a size cap.
- **Starlark sandbox** — script execution has no filesystem access, only HTTP, with timeouts and memory caps.
- **Secret redaction** — values returned by `secrets.get()` are scrubbed from script output before the agent ever sees them.

### Credential theft

**Threat:** an attacker (or a successful prompt injection) tries to extract API keys, tokens, or other credentials.

**Mitigations:**

- LLM provider tokens are stored in `secure/data/stackllm_auth.json` (mode `0600`) and are not exposed to any registered tool.
- Starlark secrets are AES-256-GCM encrypted at rest in `op_secrets`; decrypted in-memory only.
- Output sanitisation: Starlark output is scanned for any literal secret value and replaced with `[REDACTED:NAME]` before reaching the agent.
- Chat-provider tokens (`DISCORD_TOKEN`, `SLACK_BOT_TOKEN`, etc.) live in `op_chat_providers` and are never surfaced to tool calls.
- The JWT signing key (`secure/data/jwt_secret`) is not exposed to any tool path.

### Privilege escalation

**Threat:** a compromised tool implementation tries to read outside its scope, or the agent tries to obtain capabilities it wasn't granted.

**Mitigations:**

- **No new capability path at runtime.** Tools are registered once at boot (`mcp.RegisterAllTools` → `engine.RegisterMCPTools`) and the registry is closed. Adding a tool requires editing Go code and rebuilding.
- **Filesystem permissions** — `secure/` is mode `0700`, `secure/data/*` files are `0600`. Even if a tool implementation has a path-traversal bug, the OS refuses the read.
- **JWT-protected admin API** — the `/api/*` surface (including the mounted `/api/engine/*` routes) sits behind `withAuth`; the only exception is the narrow whitelist in `RequireSetupMiddleware` that lets the setup wizard sign in to a first provider.
- **Container hardening** — the Docker image runs as a single unprivileged user; capabilities can be dropped via `cap_drop: [ALL]`; rootfs can be made read-only.

### Denial of service

**Threat:** resource exhaustion through infinite loops, excessive memory, or network flooding.

**Mitigations:**

- Starlark execution and memory caps (`/settings/advanced`).
- Token-bucket rate limiter on the public surfaces (`/settings/advanced`).
- Per-session mutex serializes incoming chat events; no unbounded fan-out.
- Web-fetch response size cap.

### Unauthorized access

**Threat:** attackers attempt to access the admin UI or API without credentials.

**Mitigations:**

- JWT auth (HS256), short-lived access tokens, refresh-cookie rotation.
- Mandatory first-run setup wizard — there is no default admin account.
- Rate limiting in front of login attempts.
- Optional reverse-proxy IP allowlisting / mTLS for production.

## Defense in Depth

```
Layer 1: Tool Registry Boundary
┌─────────────────────────────────────────────────────────────────┐
│ The agent runs inside the OpenPact process. It can only call    │
│ tools that were registered in stackllm's tools.Registry at boot.│
│ No registration → not callable. There is no shell tool.         │
└─────────────────────────────────────────────────────────────────┘

Layer 2: In-Tool Authorization
┌─────────────────────────────────────────────────────────────────┐
│ Each tool enforces its own scoping in code:                     │
│  • workspace_*  → ai-data/ only, path validation                │
│  • web_fetch    → HTTP/HTTPS only, size capped                  │
│  • script_run   → admin approval workflow + Starlark sandbox    │
│  • discord_send → bot's configured allow list                   │
└─────────────────────────────────────────────────────────────────┘

Layer 3: Filesystem Permissions
┌─────────────────────────────────────────────────────────────────┐
│ secure/        0700  — system data, JWT key, encryption key,    │
│                        provider tokens, SQLite DB.              │
│ secure/data/*  0600  — sensitive files.                         │
│ ai-data/       0755  — AI-accessible.                           │
└─────────────────────────────────────────────────────────────────┘

Layer 4: Container / Runtime Isolation
┌─────────────────────────────────────────────────────────────────┐
│ Single unprivileged user inside Docker / systemd unit.          │
│ Optional: read-only rootfs, cap_drop: ALL, no-new-privileges.   │
│ Reverse-proxy TLS termination in front of the admin UI.         │
└─────────────────────────────────────────────────────────────────┘
```

## Security Layers Explained

### Layer 1: Tool registry

**Purpose:** make capability addition impossible at runtime.

| Control | Description |
|---------|-------------|
| Static registration | `mcp.RegisterAllTools` populates `tools.Registry` at boot; no runtime additions. |
| In-process invocation | The agent's tool calls are Go function calls inside the same binary; no external surface to attack. |
| No shell / eval | No tool exposes shell execution, dynamic Go code, or arbitrary file IO. |

### Layer 2: In-tool authorization

**Purpose:** ensure each tool can only do what its purpose requires.

| Control | Description |
|---------|-------------|
| Workspace boundary | `workspace_*` validates paths against the `ai-data/` root, rejecting traversal attempts. |
| Vault scope | `vault_*` constrains operations to the configured vault path. |
| HTTP-only fetch | `web_fetch` rejects `file://`, restricts to `http://` and `https://`, caps response size. |
| Allow lists | Chat-provider sends respect the configured user/channel allow lists. |
| Sandbox | Starlark execution has no filesystem, no shell; configurable wall-clock and memory caps. |

### Layer 3: Filesystem permissions

**Purpose:** belt-and-braces backup if a tool implementation has a path-traversal bug.

| Control | Description |
|---------|-------------|
| `secure/` 0700 | Owner-only; nothing in `secure/` is readable by other users on the host. |
| `secure/data/*` 0600 | Token files, encryption keys, the SQLite DB. |
| `ai-data/` 0755 | AI-accessible portion of the workspace. |

### Layer 4: Container / runtime

**Purpose:** secure the runtime environment.

| Control | Description |
|---------|-------------|
| Single unprivileged user | Image runs as `openpact`, never root. |
| Optional read-only rootfs | Binary doesn't write outside `/workspace`. |
| Optional capability drop | `cap_drop: [ALL]` works fine. |
| systemd hardening | The shipped unit pins `NoNewPrivileges`, `ProtectSystem=strict`, `MemoryDenyWriteExecute`. |
| Reverse-proxy TLS | Production: terminate TLS in nginx/Caddy/Traefik in front of `:8888`. |

## Security Checklist

### Before deployment

- [ ] Configure TLS (use a reverse proxy or terminate TLS at the load balancer).
- [ ] Set a strong admin password during the setup wizard.
- [ ] Restrict the admin UI to localhost or behind authenticated reverse proxy if internet-exposed.
- [ ] Set up monitoring and alerting for the health endpoints.
- [ ] Configure IP allowlisting at the proxy level if applicable.
- [ ] Decide on a secrets-management strategy (admin UI / `op_secrets` vs external secrets manager injecting env vars).

### During operation

- [ ] Review pending Starlark scripts promptly.
- [ ] Rotate provider tokens on schedule (re-sign-in via `/engine`).
- [ ] Rotate the JWT secret periodically (delete `secure/data/jwt_secret`; OpenPact regenerates on next boot — all sessions forced to re-login).
- [ ] Keep OpenPact and the host updated.
- [ ] Audit the script execution and tool-use logs.

### Incident response

- [ ] Know how to revoke admin credentials (delete user from the admin UI or directly in `op_users`).
- [ ] Have a process to disable compromised scripts (`script_reload` after removing the file from `ai-data/scripts/`).
- [ ] Maintain backups of `secure/data/` (encrypted).
- [ ] Document escalation procedures.

## Next Steps

- [Principle of Least Privilege](./principle-of-least-privilege) — tool registry, in-tool authorization, filesystem perms.
- [Secret Handling](./secret-handling) — how secrets are stored, redacted, and rotated.
- [Docker Security](./docker-security) — container hardening recipes.
- [Script Sandboxing](./script-sandboxing) — Starlark security model.
