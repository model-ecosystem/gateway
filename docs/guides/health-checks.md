# Health Check Guide

The gateway provides comprehensive health check endpoints for monitoring and orchestration systems like Kubernetes.

## Overview

The gateway supports three standard health check endpoints:

- `/health` - Comprehensive health status with all checks
- `/ready` - Readiness probe for load balancer integration
- `/live` - Liveness probe for container orchestration

## Configuration

Enable health checks in your gateway configuration:

```yaml
gateway:
  health:
    enabled: true
    healthPath: /health    # Default: /health
    readyPath: /ready      # Default: /ready
    livePath: /live        # Default: /live
    checks:
      # HTTP health check
      backend-api:
        type: http
        interval: 30       # Check interval in seconds
        timeout: 5         # Timeout in seconds
        config:
          url: "http://backend-service:8080/health"
      
      # TCP connectivity check
      database:
        type: tcp
        interval: 60
        timeout: 3
        config:
          address: "postgres:5432"
```

## Check Types

### HTTP Check

Verifies HTTP endpoint availability:

```yaml
checks:
  api-service:
    type: http
    timeout: 5
    config:
      url: "http://api.example.com/health"
```

### TCP Check

Tests TCP connectivity:

```yaml
checks:
  redis:
    type: tcp
    timeout: 3
    config:
      address: "redis:6379"
```

## Response Format

### Health Endpoint

```json
{
  "status": "healthy",
  "timestamp": "2024-01-18T10:30:00Z",
  "version": "1.0.0",
  "service_id": "gateway-1234567890",
  "checks": {
    "gateway": {
      "status": "healthy",
      "duration": 142
    },
    "registry": {
      "status": "healthy",
      "duration": 1523
    },
    "backend-api": {
      "status": "unhealthy",
      "error": "request failed: connection refused",
      "duration": 5012
    }
  }
}
```

### Ready Endpoint

```json
{
  "ready": true,
  "timestamp": "2024-01-18T10:30:00Z"
}
```

### Live Endpoint

```json
{
  "status": "ok",
  "timestamp": "2024-01-18T10:30:00Z"
}
```

## HTTP Status Codes

- **200 OK**: Service is healthy/ready/live
- **503 Service Unavailable**: Service is unhealthy or not ready

## Kubernetes Integration

Example Kubernetes deployment with health checks:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
spec:
  template:
    spec:
      containers:
      - name: gateway
        image: gateway:latest
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
```

## Custom Health Checks

You can implement custom health checks by creating a Check function:

```go
func DatabaseCheck(db *sql.DB) health.Check {
    return func(ctx context.Context) error {
        return db.PingContext(ctx)
    }
}
```

Register it during gateway initialization:

```go
healthChecker.RegisterCheck("database", DatabaseCheck(db))
```

## Best Practices

1. **Check Timeouts**: Set appropriate timeouts to prevent hanging checks
2. **Check Frequency**: Balance between freshness and system load
3. **Critical vs Non-Critical**: Use readiness probe for critical dependencies
4. **Error Details**: Include helpful error messages for debugging
5. **Performance**: Keep liveness checks lightweight

## Monitoring Integration

Health check endpoints can be integrated with monitoring systems:

- **Prometheus**: Scrape `/health` endpoint and parse JSON
- **Grafana**: Create dashboards based on health metrics
- **AlertManager**: Set up alerts for unhealthy services