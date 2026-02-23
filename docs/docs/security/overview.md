---
sidebar_position: 1
title: Overview
description: Security philosophy, threat model, and defense in depth
---

# Security Overview

OpenPact is designed with security as a foundational principle. This page outlines our security philosophy, threat model, and the defense-in-depth approach that protects your systems.

## Security Philosophy

OpenPact operates on several core security principles:

### 1. Assume AI Is Untrusted

While AI assistants are powerful tools, they should not be given unrestricted access to sensitive resources. OpenPact treats AI-generated code and requests as untrusted input that must be validated and sandboxed.

### 2. Defense in Depth

No single security control is sufficient. OpenPact layers multiple security mechanisms so that if one fails, others continue to protect the system.

### 3. Principle of Least Privilege

Every component receives only the minimum permissions necessary to perform its function. Scripts cannot access resources they don't need, and the AI cannot see secrets it shouldn't know.

### 4. Secure by Default

OpenPact ships with secure defaults. Features that could compromise security are opt-in, and dangerous operations require explicit approval.

### 5. Transparent Security

Security mechanisms should be understandable. Rather than relying on obscurity, OpenPact's security model is documented and auditable.

## Threat Model

OpenPact protects against several threat categories:

### Malicious Scripts

**Threat:** AI generates a script that attempts to steal secrets, access unauthorized resources, or damage the system.

**Mitigations:**
- Starlark sandboxing prevents filesystem and system access
- Script approval workflow requires human review
- Hash-based verification detects modifications
- Automatic secret redaction prevents exfiltration

### Credential Theft

**Threat:** An attacker or malicious AI attempts to extract API keys, tokens, or other credentials.

**Mitigations:**
- Environment variable filtering: only LLM provider keys are passed to the AI process
- Sensitive tokens (DISCORD_TOKEN, GITHUB_TOKEN, etc.) are excluded from the AI's environment
- Automatic redaction of secret values in all script output
- Secrets stored in the data directory, which is owner-only (mode 700) and inaccessible to the AI user

### Privilege Escalation

**Threat:** A compromised component attempts to gain additional access or capabilities.

**Mitigations:**
- Two-user model: `openpact-system` (orchestrator) and `openpact-ai` (AI engine)
- AI process runs as `openpact-ai` with restricted file permissions
- OpenCode's built-in tools (bash, write, edit, read, etc.) are disabled via configuration
- AI can only interact with the system through registered MCP tools
- Linux file permissions enforce access boundaries independent of application logic

### Denial of Service

**Threat:** Resource exhaustion through infinite loops, excessive memory usage, or network flooding.

**Mitigations:**
- Script execution timeouts
- Memory limits on script execution
- Rate limiting on API endpoints
- Resource quotas per script

### Unauthorized Access

**Threat:** Attackers attempt to access the Admin UI or API without credentials.

**Mitigations:**
- JWT-based authentication with short-lived tokens
- Mandatory first-run setup (no default credentials)
- Rate limiting on login attempts
- Optional IP allowlisting

## Defense in Depth Architecture

```
Layer 1: Linux User Separation
┌─────────────────────────────────────────────────────────────────┐
│  openpact-system (orchestrator, admin UI, secrets)              │
│  openpact-ai (AI engine, MCP tools only)                       │
│  File permissions enforce boundary: data dir 700, workspace 750│
└─────────────────────────────────────────────────────────────────┘

Layer 2: Application-Level Tool Restriction
┌─────────────────────────────────────────────────────────────────┐
│  OpenCode built-in tools disabled (bash, write, edit, etc.)    │
│  MCP server provides controlled tool access                     │
│  Environment variable allowlist (no secrets leaked)             │
└─────────────────────────────────────────────────────────────────┘

Layer 3: Script Sandboxing
┌─────────────────────────────────────────────────────────────────┐
│  Starlark sandbox: no filesystem, no system commands            │
│  Script approval workflow: human review required                │
│  Secret redaction: values replaced with [REDACTED] in output   │
└─────────────────────────────────────────────────────────────────┘

Layer 4: Container Isolation
┌─────────────────────────────────────────────────────────────────┐
│  Docker isolation, non-root execution                           │
│  Entrypoint sets file permissions before dropping privileges    │
│  Optional: read-only root filesystem, dropped capabilities     │
└─────────────────────────────────────────────────────────────────┘
```

## Security Layers Explained

### Layer 1: Linux User Separation

**Purpose:** Enforce access boundaries at the OS level.

| Control | Description |
|---------|-------------|
| Two-user model | `openpact-system` owns secrets/config, `openpact-ai` runs the AI |
| File permissions | Data dir (700), workspace (750), memory (770), config (600) |
| SysProcAttr | AI process spawned with `openpact-ai` UID/GID via syscall |
| Group membership | Both users in `openpact` group for controlled shared access |

### Layer 2: Application Tool Restriction

**Purpose:** Ensure the AI can only use explicitly registered MCP tools.

| Control | Description |
|---------|-------------|
| Disabled built-in tools | bash, write, edit, read, grep, glob, list, patch all disabled |
| MCP-only access | AI interacts with system exclusively through MCP tool calls |
| Environment filtering | Only PATH, HOME, LANG, TZ, TMPDIR, XDG_*, and LLM keys passed |
| OpenCode config | Tool restrictions enforced via OPENCODE_CONFIG_CONTENT |

### Layer 3: Script Sandboxing

**Purpose:** Contain execution of untrusted code.

| Control | Description |
|---------|-------------|
| Starlark sandbox | No system access from scripts |
| Execution limits | Timeout and memory caps |
| Script approval | Human review before execution |
| Output sanitization | Redact sensitive data in results |

### Layer 4: Container Isolation

**Purpose:** Secure the runtime environment.

| Control | Description |
|---------|-------------|
| Docker isolation | Container boundaries |
| Non-root execution | Both users are non-root |
| Entrypoint permissions | File permissions set at container start |
| Resource limits | CPU/memory quotas via Docker |

## Security Checklist

### Before Deployment

- [ ] Configure TLS (use reverse proxy or native HTTPS)
- [ ] Set strong admin password
- [ ] Review default configuration (especially `run_as_user` and `mcp_binary`)
- [ ] Set up monitoring and alerting
- [ ] Configure IP allowlisting if possible

### During Operation

- [ ] Monitor login attempts for anomalies
- [ ] Review pending scripts promptly
- [ ] Rotate secrets on schedule
- [ ] Keep OpenPact updated
- [ ] Audit script execution logs

### Incident Response

- [ ] Know how to revoke JWT secrets
- [ ] Have a process to disable compromised scripts
- [ ] Maintain backups of configuration
- [ ] Document escalation procedures

## Next Steps

- [Principle of Least Privilege](./principle-of-least-privilege) - Understand access restrictions
- [Secret Handling](./secret-handling) - Learn how secrets are protected
- [Docker Security](./docker-security) - Container isolation details
- [Script Sandboxing](./script-sandboxing) - Starlark security model
