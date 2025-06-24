# Circuit Breaker

The gateway includes a sophisticated circuit breaker implementation to protect your services from cascading failures and improve overall system resilience.

## Overview

The circuit breaker pattern:
- Monitors backend service health
- Opens circuit on repeated failures
- Prevents requests to failing services
- Allows periodic health checks
- Automatically recovers when service is healthy
- Configurable per-route or per-service

## How It Works

The circuit breaker has three states:

1. **Closed** (normal operation)
   - All requests pass through
   - Failures are counted
   - Opens if failure threshold exceeded

2. **Open** (failure detected)
   - All requests fail immediately
   - No load on failing service
   - Waits for timeout period

3. **Half-Open** (testing recovery)
   - Limited requests allowed
   - Success closes the circuit
   - Failure reopens the circuit

## Configuration

### Basic Setup

```yaml
gateway:
  middleware:
    circuitbreaker:
      enabled: true
      default:
        maxFailures: 5          # Open after 5 failures
        failureThreshold: 0.5   # Open if 50% of requests fail
        timeout: 60s            # Stay open for 60 seconds
        maxRequests: 1          # Allow 1 request in half-open
        interval: 60s           # Reset counters every 60s
```

### Per-Route Configuration

```yaml
gateway:
  middleware:
    circuitbreaker:
      enabled: true
      default:
        maxFailures: 10
        timeout: 30s
      
      routes:
        critical-api:
          maxFailures: 3       # More sensitive for critical routes
          timeout: 10s         # Faster recovery attempts
        
        batch-api:
          maxFailures: 20      # More tolerant for batch operations
          timeout: 120s        # Longer timeout
```

### Per-Service Configuration

```yaml
gateway:
  middleware:
    circuitbreaker:
      enabled: true
      default:
        maxFailures: 5
      
      services:
        payment-service:
          maxFailures: 2      # Very sensitive for payments
          failureThreshold: 0.3
          timeout: 30s
        
        analytics-service:
          maxFailures: 15     # More tolerant
          failureThreshold: 0.7
          timeout: 90s
```

## Route Integration

Apply circuit breakers to specific routes:

```yaml
gateway:
  router:
    rules:
      - id: user-api
        path: /api/users/*
        serviceName: user-service
        middleware:
          - circuitbreaker:
              maxFailures: 5
              timeout: 30s
      
      - id: payment-api
        path: /api/payments/*
        serviceName: payment-service
        middleware:
          - circuitbreaker:
              maxFailures: 2
              failureThreshold: 0.2
              timeout: 60s
```

## Failure Detection

### What Counts as a Failure

By default, these are considered failures:
- HTTP 5xx responses
- Network errors
- Timeouts
- Connection refused

### What Doesn't Count

These are NOT counted as failures:
- HTTP 4xx client errors
- Successful responses (2xx, 3xx)

### Custom Failure Detection

```yaml
gateway:
  middleware:
    circuitbreaker:
      default:
        # Consider slow responses as failures
        slowCallDuration: 5s
        slowCallThreshold: 0.8  # 80% slow calls opens circuit
        
        # Custom HTTP status codes as failures
        failureStatusCodes:
          - 502
          - 503
          - 504
```

## Advanced Features

### State Change Callbacks

Monitor circuit breaker state changes:

```yaml
gateway:
  middleware:
    circuitbreaker:
      notifications:
        webhook:
          url: https://monitoring.example.com/circuit-breaker
          events:
            - open
            - close
            - half-open
```

### Metrics Integration

Circuit breaker metrics are exposed:

```
# HELP gateway_circuit_breaker_state Current state (0=closed, 1=open, 2=half-open)
gateway_circuit_breaker_state{name="payment-service"} 0

# HELP gateway_circuit_breaker_failures_total Total number of failures
gateway_circuit_breaker_failures_total{name="payment-service"} 2

# HELP gateway_circuit_breaker_successes_total Total number of successes
gateway_circuit_breaker_successes_total{name="payment-service"} 198

# HELP gateway_circuit_breaker_rejections_total Requests rejected by open circuit
gateway_circuit_breaker_rejections_total{name="payment-service"} 0
```

## Response When Open

When circuit is open, gateway returns:

```http
HTTP/1.1 503 Service Unavailable
Content-Type: application/json

{
  "error": "service_unavailable",
  "message": "Service temporarily unavailable",
  "details": {
    "circuit_breaker": "open",
    "service": "payment-service",
    "retry_after": "2024-01-15T10:35:00Z"
  }
}
```

## Best Practices

### 1. Tune Thresholds Carefully

```yaml
# Critical services: fail fast
payment-service:
  maxFailures: 2
  failureThreshold: 0.2
  timeout: 30s

# Non-critical services: more tolerant
logging-service:
  maxFailures: 20
  failureThreshold: 0.8
  timeout: 120s
```

### 2. Monitor Circuit State

Set up alerts for circuit breaker state changes:
- Alert when circuit opens frequently
- Track recovery time
- Monitor rejection rates

### 3. Coordinate with Retry

Circuit breaker works with retry logic:

```yaml
middleware:
  - retry:
      maxAttempts: 3
      backoff: exponential
  - circuitbreaker:
      maxFailures: 5
```

### 4. Test Circuit Breaker

Regularly test circuit breaker behavior:
- Simulate service failures
- Verify circuit opens correctly
- Test recovery behavior

## Use Cases

### 1. Microservice Protection

```yaml
services:
  user-service:
    maxFailures: 5
    timeout: 30s
  
  order-service:
    maxFailures: 3
    timeout: 45s
  
  inventory-service:
    maxFailures: 10
    timeout: 60s
```

### 2. External API Protection

```yaml
routes:
  external-payment-api:
    maxFailures: 2
    failureThreshold: 0.1  # Very sensitive
    timeout: 120s          # Long timeout for external
```

### 3. Database Protection

```yaml
services:
  database-service:
    maxFailures: 3
    slowCallDuration: 1s   # DB queries should be fast
    slowCallThreshold: 0.5
```

## Troubleshooting

### Circuit Opens Too Frequently

1. Increase `maxFailures` threshold
2. Adjust `failureThreshold` percentage
3. Check backend service health
4. Review timeout settings

### Circuit Never Opens

1. Decrease failure thresholds
2. Check failure detection logic
3. Verify errors are counted correctly
4. Enable debug logging

### Recovery Issues

1. Adjust `maxRequests` in half-open
2. Increase recovery timeout
3. Check if service is truly healthy
4. Monitor half-open success rate

### Debug Logging

```yaml
gateway:
  logging:
    level: debug
    modules:
      middleware.circuitbreaker: debug
```

Logs show:
- State transitions
- Failure counts
- Request decisions
- Recovery attempts

## Integration with Health Checks

Circuit breaker can integrate with health checks:

```yaml
gateway:
  middleware:
    circuitbreaker:
      healthCheck:
        enabled: true
        interval: 10s
        timeout: 2s
        # Close circuit if health check passes
        closeOnHealthy: true
```

This provides proactive circuit management based on service health rather than just request failures.