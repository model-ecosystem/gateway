# Health Checks

The gateway provides a comprehensive health check system to monitor the health of backend services, the gateway itself, and all its components.

## Overview

The health check system includes:
- Active health checks for backend services
- Passive health checks based on request failures
- Gateway component health monitoring
- Custom health check endpoints
- Health-based load balancing
- Configurable check intervals and thresholds

## Configuration

### Basic Health Check Setup

```yaml
gateway:
  health:
    enabled: true
    endpoint: "/_gateway/health"
    
    # Gateway self-checks
    checks:
      - name: "memory"
        type: "memory"
        threshold: 90  # Alert if memory usage > 90%
      
      - name: "goroutines"
        type: "goroutines"
        threshold: 10000  # Alert if goroutines > 10k
```

### Backend Service Health Checks

```yaml
gateway:
  registry:
    static:
      services:
        - name: api-service
          instances:
            - id: api-1
              address: 10.0.0.1
              port: 8080
              health:
                enabled: true
                path: "/health"
                interval: 10s
                timeout: 2s
                healthyThreshold: 2
                unhealthyThreshold: 3
```

## Health Check Types

### 1. HTTP Health Checks

```yaml
health:
  type: http
  path: "/health"
  method: GET
  headers:
    X-Health-Check: "gateway"
  expectedStatus: 200
  expectedBody: "OK"
  interval: 10s
  timeout: 5s
```

### 2. TCP Health Checks

```yaml
health:
  type: tcp
  port: 8080
  interval: 5s
  timeout: 2s
```

### 3. gRPC Health Checks

```yaml
health:
  type: grpc
  service: "grpc.health.v1.Health"
  method: "Check"
  interval: 10s
  timeout: 3s
```

### 4. Custom Script Checks

```yaml
health:
  type: script
  command: "/usr/local/bin/check-service.sh"
  args: ["--service", "api"]
  interval: 30s
  timeout: 5s
  expectedExitCode: 0
```

## Health Status

### Service Instance States

- **Healthy**: Passing health checks
- **Unhealthy**: Failing health checks
- **Unknown**: No health check data yet
- **Draining**: Being removed from rotation

### Transitions

```yaml
gateway:
  health:
    transitions:
      # Require 2 consecutive successes to mark healthy
      healthyThreshold: 2
      
      # Require 3 consecutive failures to mark unhealthy
      unhealthyThreshold: 3
      
      # Grace period for new instances
      initialDelay: 30s
      
      # Drain time before removing
      drainTimeout: 60s
```

## Advanced Health Checks

### Composite Health Checks

```yaml
health:
  checks:
    - name: "database"
      type: composite
      require: "all"  # all, any, majority
      checks:
        - type: tcp
          port: 5432
        - type: http
          path: "/db/ping"
          expectedStatus: 200
```

### Weighted Health Scores

```yaml
health:
  scoring:
    enabled: true
    checks:
      - name: "response_time"
        weight: 0.3
        type: "latency"
        threshold: 100ms
      
      - name: "error_rate"
        weight: 0.5
        type: "error_rate"
        threshold: 0.01
      
      - name: "active_connections"
        weight: 0.2
        type: "connections"
        threshold: 1000
```

### Dependency Checks

```yaml
health:
  dependencies:
    - name: "database"
      critical: true
      check:
        type: tcp
        host: "db.internal"
        port: 5432
    
    - name: "cache"
      critical: false
      check:
        type: http
        url: "http://redis:6379/ping"
```

## Health Check Endpoints

### Gateway Health Endpoint

```bash
GET /_gateway/health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "memory": {
      "status": "healthy",
      "details": {
        "used": 512,
        "total": 1024,
        "percentage": 50
      }
    },
    "services": {
      "status": "healthy",
      "details": {
        "healthy": 10,
        "unhealthy": 0,
        "total": 10
      }
    }
  }
}
```

### Service Health Endpoint

```bash
GET /_gateway/health/services/{service-name}
```

Response:
```json
{
  "service": "api-service",
  "status": "healthy",
  "instances": [
    {
      "id": "api-1",
      "address": "10.0.0.1:8080",
      "status": "healthy",
      "lastCheck": "2024-01-15T10:29:55Z",
      "consecutiveSuccesses": 15,
      "consecutiveFailures": 0
    }
  ]
}
```

## Integration with Load Balancing

### Health-Aware Load Balancing

```yaml
gateway:
  router:
    rules:
      - id: api-route
        path: /api/*
        serviceName: api-service
        loadBalance: health_weighted
        healthWeight:
          # Weight based on health score
          healthy: 100
          degraded: 50
          unhealthy: 0
```

### Circuit Breaker Integration

```yaml
gateway:
  middleware:
    circuitbreaker:
      healthCheck:
        enabled: true
        # Open circuit if health check fails
        openOnUnhealthy: true
        # Close circuit when healthy
        closeOnHealthy: true
```

## Monitoring and Alerting

### Metrics

Health check metrics exposed:
- `gateway_health_check_duration_seconds`
- `gateway_health_check_failures_total`
- `gateway_health_check_successes_total`
- `gateway_service_health_score`
- `gateway_service_instances_healthy`
- `gateway_service_instances_unhealthy`

### Events

Health events emitted:
- `health.check.started`
- `health.check.completed`
- `health.status.changed`
- `health.instance.added`
- `health.instance.removed`

### Webhooks

```yaml
gateway:
  health:
    notifications:
      webhooks:
        - url: "https://alerts.example.com/health"
          events:
            - "instance.unhealthy"
            - "service.degraded"
            - "service.down"
          headers:
            Authorization: "Bearer ${WEBHOOK_TOKEN}"
```

## Performance Optimization

### Batching Health Checks

```yaml
gateway:
  health:
    batching:
      enabled: true
      maxBatchSize: 50
      batchInterval: 1s
```

### Caching Results

```yaml
gateway:
  health:
    cache:
      enabled: true
      ttl: 5s
      # Share results across goroutines
      shared: true
```

### Jittered Intervals

```yaml
gateway:
  health:
    jitter:
      enabled: true
      factor: 0.2  # ±20% randomization
```

## Custom Health Checks

### Implementing Custom Checks

Create custom health checks by implementing the interface:

```go
type HealthChecker interface {
    Check(ctx context.Context) HealthResult
}

type HealthResult struct {
    Status  HealthStatus
    Message string
    Details map[string]interface{}
}
```

### Registering Custom Checks

```yaml
gateway:
  health:
    custom:
      - name: "business-logic"
        class: "com.example.BusinessHealthCheck"
        config:
          threshold: 100
          query: "SELECT COUNT(*) FROM active_users"
```

## Best Practices

1. **Set Appropriate Intervals**: Balance between freshness and load
2. **Use Timeouts**: Prevent hanging health checks
3. **Monitor Check Duration**: Track how long checks take
4. **Implement Graceful Degradation**: Handle partial failures
5. **Use Passive Checks**: Supplement active checks with request data
6. **Set Proper Thresholds**: Avoid flapping between states

## Troubleshooting

### Common Issues

1. **Flapping Health Status**
   - Increase thresholds
   - Add jitter to checks
   - Review timeout settings

2. **High Check Latency**
   - Reduce check complexity
   - Increase timeouts
   - Use caching

3. **Missing Instances**
   - Verify health check endpoints
   - Check network connectivity
   - Review security settings

### Debug Mode

```yaml
gateway:
  health:
    debug: true
    logLevel: debug
    # Log all check results
    logResults: true
```

This provides detailed logging of all health check operations for troubleshooting.