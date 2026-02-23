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
- Secrets are never exposed to the AI
- Automatic redaction of secret values in all output
- Secrets stored encrypted at rest
- Environment variable isolation in Docker

### Privilege Escalation

**Threat:** A compromised component attempts to gain additional access or capabilities.

**Mitigations:**
- Container runs as non-root user by default
- Two-user model separates admin and runtime processes
- MCP tools have explicit capability boundaries
- Network isolation limits lateral movement

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
┌─────────────────────────────────────────────────────────────────┐
│                        External Boundary                         │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Reverse Proxy (nginx/Caddy)                                 ││
│  │ - TLS termination                                           ││
│  │ - Rate limiting                                              ││
│  │ - IP filtering                                               ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Application Boundary                        │
│  ┌──────────────────────┐  ┌──────────────────────────────────┐│
│  │    Admin UI Auth     │  │         MCP Protocol             ││
│  │ - JWT validation     │  │ - Stdio transport (local only)   ││
│  │ - Session management │  │ - Tool capability checks         ││
│  │ - CSRF protection    │  │ - Request validation             ││
│  └──────────────────────┘  └──────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Script Boundary                            │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                   Approval Workflow                         ││
│  │ - Human review required                                     ││
│  │ - Hash verification                                         ││
│  │ - Allowlist for trusted scripts                             ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                   Starlark Sandbox                          ││
│  │ - No filesystem access                                      ││
│  │ - No system command execution                               ││
│  │ - HTTP/HTTPS only networking                                ││
│  │ - Execution time limits                                     ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Secret Boundary                            │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                   Secret Management                         ││
│  │ - Encrypted at rest                                         ││
│  │ - Never returned via API                                    ││
│  │ - Automatic output redaction                                ││
│  │ - Script-only access                                        ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Container Boundary                          │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                   Docker Isolation                          ││
│  │ - Non-root execution                                        ││
│  │ - Read-only root filesystem                                 ││
│  │ - Dropped capabilities                                      ││
│  │ - Resource limits                                           ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Security Layers Explained

### Layer 1: Network Boundary

**Purpose:** Protect against unauthorized network access.

| Control | Description |
|---------|-------------|
| TLS encryption | All traffic encrypted in transit |
| Reverse proxy | Hide internal architecture |
| Rate limiting | Prevent brute force attacks |
| IP filtering | Restrict access to known networks |

### Layer 2: Authentication

**Purpose:** Verify identity of administrators.

| Control | Description |
|---------|-------------|
| JWT tokens | Cryptographically signed sessions |
| Password policy | Strong password requirements |
| Session management | Short-lived access tokens |
| Secure cookies | HTTP-only, SameSite strict |

### Layer 3: Authorization

**Purpose:** Ensure components only access permitted resources.

| Control | Description |
|---------|-------------|
| Script approval | Human review before execution |
| Hash verification | Detect unauthorized modifications |
| MCP tool boundaries | Explicit capability limits |
| Secret scoping | Per-script secret access |

### Layer 4: Sandboxing

**Purpose:** Contain execution of untrusted code.

| Control | Description |
|---------|-------------|
| Starlark sandbox | No system access from scripts |
| Execution limits | Timeout and memory caps |
| Network restrictions | HTTP/HTTPS only |
| Output sanitization | Redact sensitive data |

### Layer 5: Infrastructure

**Purpose:** Secure the runtime environment.

| Control | Description |
|---------|-------------|
| Container isolation | Docker/OCI boundaries |
| Non-root execution | Minimum privileges |
| Read-only filesystem | Prevent tampering |
| Resource limits | CPU/memory quotas |

## Security Checklist

### Before Deployment

- [ ] Configure TLS (use reverse proxy or native HTTPS)
- [ ] Set strong admin password
- [ ] Review default configuration
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

## Comparison with Alternatives

| Feature | OpenPact | Direct API Access | Custom Solutions |
|---------|----------|-------------------|------------------|
| AI sandboxing | Built-in | None | Manual |
| Secret redaction | Automatic | None | Manual |
| Script approval | Required | N/A | Optional |
| Execution limits | Enforced | None | Optional |
| Container isolation | Default | N/A | Optional |
| Authentication | Built-in | N/A | Manual |

OpenPact provides a complete security framework out of the box, whereas alternative approaches require significant additional development.

## Next Steps

- [Principle of Least Privilege](./principle-of-least-privilege) - Understand access restrictions
- [Secret Handling](./secret-handling) - Learn how secrets are protected
- [Docker Security](./docker-security) - Container isolation details
- [Script Sandboxing](./script-sandboxing) - Starlark security model
