---
sidebar_position: 1
title: Health Endpoints
description: Health check, readiness, and metrics endpoints
---

# Health Endpoints

OpenPact provides several health check endpoints for monitoring, orchestration, and observability. These endpoints are unauthenticated and designed for use by load balancers, container orchestrators, and monitoring systems.

## Endpoints Overview

| Endpoint | Purpose | Authentication |
|----------|---------|----------------|
| `/health` | General health status | None |
| `/healthz` | Kubernetes-style health check | None |
| `/ready` | Readiness probe | None |
| `/metrics` | Application metrics | None |

## GET /health

Returns the general health status of the application.

### Request

```
GET /health HTTP/1.1
Host: localhost:8080
```

### Response

**200 OK** - Application is healthy

```json
{
  "status": "healthy",
  "uptime": "24h3m15s",
  "version": "1.0.0"
}
```

**503 Service Unavailable** - Application is unhealthy or in setup mode

```json
{
  "status": "unhealthy",
  "reason": "setup_required",
  "message": "First-time setup has not been completed"
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | `healthy` or `unhealthy` |
| `uptime` | string | Time since application started |
| `version` | string | Application version |
| `reason` | string | (unhealthy only) Reason code |
| `message` | string | (unhealthy only) Human-readable message |

### Use Cases

- General monitoring dashboards
- Quick health verification
- Alerting systems

## GET /healthz

Kubernetes-style liveness probe. Returns a minimal response for fast checking.

### Request

```
GET /healthz HTTP/1.1
Host: localhost:8080
```

### Response

**200 OK** - Application is alive

```
OK
```

**503 Service Unavailable** - Application is not functional

```
Service Unavailable
```

### Response Details

- Content-Type: `text/plain`
- Body: `OK` or error message
- No JSON parsing required

### Use Cases

- Kubernetes liveness probes
- Load balancer health checks
- Simple uptime monitoring

### Kubernetes Configuration

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
    - name: openpact
      livenessProbe:
        httpGet:
          path: /healthz
          port: 8080
        initialDelaySeconds: 10
        periodSeconds: 30
        timeoutSeconds: 5
        failureThreshold: 3
```

## GET /ready

Readiness probe indicating the application is ready to accept traffic.

### Request

```
GET /ready HTTP/1.1
Host: localhost:8080
```

### Response

**200 OK** - Application is ready

```json
{
  "ready": true,
  "checks": {
    "admin_setup": true,
    "script_store": true,
    "mcp_server": true
  }
}
```

**503 Service Unavailable** - Application is not ready

```json
{
  "ready": false,
  "checks": {
    "admin_setup": false,
    "script_store": true,
    "mcp_server": false
  },
  "reason": "admin_setup pending"
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `ready` | boolean | Overall readiness status |
| `checks` | object | Individual component checks |
| `checks.admin_setup` | boolean | First-run setup completed |
| `checks.script_store` | boolean | Script storage accessible |
| `checks.mcp_server` | boolean | MCP server running |
| `reason` | string | (not ready) First failing check |

### Difference from /healthz

| Aspect | /healthz | /ready |
|--------|----------|--------|
| Purpose | Is process alive? | Can it serve requests? |
| During startup | Returns 200 | Returns 503 |
| During setup | Returns 200 | Returns 503 |
| After shutdown signal | Returns 503 | Returns 503 |

### Kubernetes Configuration

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
    - name: openpact
      readinessProbe:
        httpGet:
          path: /ready
          port: 8080
        initialDelaySeconds: 5
        periodSeconds: 10
        timeoutSeconds: 3
        failureThreshold: 3
```

## GET /metrics

Returns application metrics in JSON format.

### Request

```
GET /metrics HTTP/1.1
Host: localhost:8080
```

### Response

**200 OK**

```json
{
  "scripts": {
    "total": 5,
    "approved": 4,
    "pending": 1,
    "rejected": 0
  },
  "executions": {
    "total": 1234,
    "success": 1200,
    "failed": 34,
    "avg_duration_ms": 150
  },
  "secrets": {
    "total": 3,
    "configured": 3
  },
  "system": {
    "uptime_seconds": 86400,
    "memory_mb": 45,
    "goroutines": 12
  }
}
```

### Metrics Fields

#### Scripts

| Field | Description |
|-------|-------------|
| `total` | Total number of scripts |
| `approved` | Scripts with approved status |
| `pending` | Scripts awaiting review |
| `rejected` | Scripts that were rejected |

#### Executions

| Field | Description |
|-------|-------------|
| `total` | Total script executions |
| `success` | Successful executions |
| `failed` | Failed executions |
| `avg_duration_ms` | Average execution time |

#### Secrets

| Field | Description |
|-------|-------------|
| `total` | Total configured secret names |
| `configured` | Secrets with values set |

#### System

| Field | Description |
|-------|-------------|
| `uptime_seconds` | Seconds since startup |
| `memory_mb` | Current memory usage |
| `goroutines` | Number of active goroutines |

### Use Cases

- Monitoring dashboards
- Alerting on pending scripts
- Performance tracking
- Capacity planning

## Load Balancer Configuration

### nginx

```nginx
upstream openpact {
    server openpact-1:8080;
    server openpact-2:8080;
}

server {
    location /api/ {
        proxy_pass http://openpact;
    }

    location /health {
        proxy_pass http://openpact;
        access_log off;
    }
}
```

### HAProxy

```haproxy
backend openpact
    option httpchk GET /healthz
    http-check expect string OK
    server openpact-1 10.0.0.1:8080 check
    server openpact-2 10.0.0.2:8080 check
```

### AWS ALB

```json
{
  "HealthCheckPath": "/healthz",
  "HealthCheckIntervalSeconds": 30,
  "HealthyThresholdCount": 2,
  "UnhealthyThresholdCount": 3,
  "HealthCheckTimeoutSeconds": 5
}
```

## Docker Health Check

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
  CMD wget -q --spider http://localhost:8080/healthz || exit 1
```

Or in docker-compose:

```yaml
services:
  openpact:
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
```

## Monitoring Integration

### Prometheus (via JSON exporter)

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'openpact'
    metrics_path: /metrics
    static_configs:
      - targets: ['openpact:8080']
```

### Datadog

```yaml
# datadog agent config
init_config:

instances:
  - name: openpact
    url: http://openpact:8080/metrics
    method: GET
```

### Example Alert Rules

```yaml
# Alert on unhealthy status
- alert: OpenPactUnhealthy
  expr: openpact_health_status != 1
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "OpenPact is unhealthy"

# Alert on pending scripts
- alert: OpenPactPendingScripts
  expr: openpact_scripts_pending > 5
  for: 1h
  labels:
    severity: warning
  annotations:
    summary: "{{ $value }} scripts pending review"

# Alert on high failure rate
- alert: OpenPactHighFailureRate
  expr: rate(openpact_executions_failed[5m]) > 0.1
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "High script failure rate"
```

## Troubleshooting

### /healthz returns 503 but application seems running

1. Check if first-run setup is complete
2. Verify data directory is accessible
3. Check application logs for errors

### /ready returns 503 during startup

This is expected behavior. The readiness probe fails until:
- First-run setup is complete
- All components are initialized
- MCP server is accepting connections

### /metrics shows unexpected values

1. Metrics reset on application restart
2. Some metrics may be cached (up to 10 seconds)
3. Check for multiple instances affecting aggregated metrics
