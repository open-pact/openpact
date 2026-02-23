---
sidebar_position: 2
title: Authentication
description: Setting up admin password, JWT tokens, and session management
---

# Authentication

The Admin UI uses a secure two-token JWT authentication system designed to protect against common web attacks while providing a smooth user experience.

## Token Architecture

OpenPact uses two types of tokens for authentication:

| Token | Storage | Lifetime | Purpose |
|-------|---------|----------|---------|
| **Refresh Token** | HTTP-only cookie | 3 days | Obtain new access tokens |
| **Access Token** | In-memory (JavaScript) | 15 minutes | API authorization |

### Security Benefits

This architecture provides several security advantages:

- **XSS Protection** - Refresh token cannot be accessed by JavaScript (HTTP-only cookie)
- **Limited Exposure** - Access token is short-lived, limiting damage if leaked
- **Narrow Scope** - Refresh cookie only sent to `/api/session` endpoint
- **CSRF Protection** - SameSite=Strict prevents cross-site request forgery

## Authentication Flow

```
┌────────┐                  ┌────────────┐                ┌─────────┐
│ Browser│                  │  Admin API │                │ Storage │
└───┬────┘                  └─────┬──────┘                └────┬────┘
    │                             │                             │
    │  1. POST /api/auth/login    │                             │
    │     {username, password}    │                             │
    │────────────────────────────>│                             │
    │                             │  Validate credentials       │
    │                             │────────────────────────────>│
    │                             │<────────────────────────────│
    │  200 OK                     │                             │
    │  Set-Cookie: refresh=xxx;   │                             │
    │    HttpOnly; Secure;        │                             │
    │    Path=/api/session;       │                             │
    │    SameSite=Strict;         │                             │
    │    Max-Age=259200           │  (3 days)                   │
    │<────────────────────────────│                             │
    │                             │                             │
    │  2. GET /api/session        │                             │
    │     Cookie: refresh=xxx     │                             │
    │────────────────────────────>│                             │
    │                             │  Validate refresh token     │
    │  200 OK                     │                             │
    │  {access_token, expires_at} │  (15 min token)             │
    │<────────────────────────────│                             │
    │                             │                             │
    │  3. GET /api/scripts        │                             │
    │     Authorization: Bearer   │                             │
    │────────────────────────────>│                             │
    │                             │  Validate access token      │
    │  200 OK {scripts: [...]}    │                             │
    │<────────────────────────────│                             │
```

## Automatic Token Refresh

When the access token expires, the frontend automatically refreshes it:

```
┌────────┐                  ┌────────────┐
│ Browser│                  │  Admin API │
└───┬────┘                  └─────┬──────┘
    │                             │
    │  GET /api/scripts           │
    │  Authorization: Bearer xxx  │  (expired token)
    │────────────────────────────>│
    │  401 Unauthorized           │
    │<────────────────────────────│
    │                             │
    │  GET /api/session           │  (interceptor auto-retries)
    │  Cookie: refresh=xxx        │
    │────────────────────────────>│
    │  200 OK                     │
    │  {access_token: "new..."}   │
    │<────────────────────────────│
    │                             │
    │  GET /api/scripts           │  (retry original request)
    │  Authorization: Bearer new  │
    │────────────────────────────>│
    │  200 OK {scripts: [...]}    │
    │<────────────────────────────│
```

This happens transparently - users do not see any interruption.

## Password Policy

Passwords must meet one of these requirements:

### Option 1: Long Password (Recommended)

16 or more characters. This allows passphrase-style passwords that are easy to remember:

```
correct horse battery staple
my favorite coffee shop is downtown
```

### Option 2: Complex Password

12 or more characters with at least 3 of these 4 categories:

- Uppercase letters (A-Z)
- Lowercase letters (a-z)
- Numbers (0-9)
- Symbols (!@#$%^&*...)

Examples:

```
MySecure123!     (uppercase, lowercase, numbers, symbol)
Password2024!    (uppercase, lowercase, numbers, symbol)
secure_pass_42   (lowercase, numbers, symbol)
```

## JWT Secret Management

The JWT signing secret is automatically generated on first run and saved to disk.

### Automatic Generation

1. On first launch, OpenPact generates a cryptographically secure 256-bit secret
2. The secret is saved to `data/jwt_secret` with restrictive permissions (0600)
3. Subsequent launches load the secret from this file

### Environment Variable Override

For containerized deployments, you can set the secret via environment variable:

```bash
export ADMIN_JWT_SECRET="your-256-bit-secret-here"
```

The environment variable takes priority over the file-based secret.

:::warning Secret Security
Never commit the JWT secret to version control. Use environment variables or secure secret management in production.
:::

## Configuration

Configure authentication settings in `openpact.yaml`:

```yaml
admin:
  jwt:
    # secret: auto-generated, or override with ADMIN_JWT_SECRET env var
    access_expiry: "15m"      # Short-lived access token
    refresh_expiry: "72h"     # 3-day refresh token
    issuer: "openpact"
```

## Rate Limiting

To prevent brute-force attacks, the login endpoint is rate-limited:

| Endpoint | Limit |
|----------|-------|
| `POST /api/auth/login` | 5 attempts per minute |

After exceeding the limit, additional attempts return `429 Too Many Requests` until the window resets.

## Session Endpoints

### Login

```
POST /api/auth/login
```

Request:
```json
{
  "username": "admin",
  "password": "your-password"
}
```

Response (200 OK):
```json
{
  "message": "Login successful"
}
```

The response also sets an HTTP-only cookie containing the refresh token.

### Get Session

```
GET /api/session
```

Request: Cookie is sent automatically by browser.

Response (200 OK):
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2024-01-15T10:45:00Z",
  "username": "admin"
}
```

### Current User

```
GET /api/auth/me
```

Headers:
```
Authorization: Bearer <access_token>
```

Response (200 OK):
```json
{
  "username": "admin",
  "role": "admin"
}
```

### Logout

```
POST /api/auth/logout
```

Response: `204 No Content`

This clears the refresh token cookie.

## Security Checklist

When deploying the Admin UI in production:

- [ ] Use HTTPS (required for secure cookies)
- [ ] Set a strong password meeting policy requirements
- [ ] Consider IP allowlisting for additional protection
- [ ] Monitor login attempts for suspicious activity
- [ ] Rotate JWT secret periodically in high-security environments
- [ ] Use a reverse proxy (nginx, Caddy) for TLS termination

## Troubleshooting

### "Invalid credentials" after correct password

- Check that the password was set correctly during setup
- Verify you're using the correct username
- Check for copy/paste issues with hidden characters

### Token refresh failing

- Clear browser cookies and log in again
- Check that your system time is accurate (JWT validation uses timestamps)
- Verify the server hasn't restarted with a new JWT secret

### Secure cookie warnings in development

When running on localhost, secure cookies are automatically disabled. This is normal for development but should never happen in production.
