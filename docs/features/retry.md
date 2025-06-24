# Retry Middleware

The gateway provides intelligent retry capabilities to handle transient failures and improve reliability of backend service calls.

## Overview

The retry middleware:
- Automatically retries failed requests
- Supports multiple backoff strategies
- Implements retry budgets to prevent overload
- Configurable per-route or globally
- Works with circuit breakers
- Respects idempotency

## Configuration

### Basic Setup

```yaml
gateway:
  middleware:
    retry:
      enabled: true
      default:
        maxAttempts: 3
        initialDelay: 100ms
        maxDelay: 5s
        backoffMultiplier: 2
```

### Backoff Strategies

#### Exponential Backoff

```yaml
gateway:
  middleware:
    retry:
      default:
        strategy: exponential
        maxAttempts: 5
        initialDelay: 100ms
        maxDelay: 10s
        backoffMultiplier: 2
        # Delays: 100ms, 200ms, 400ms, 800ms, 1.6s
```

#### Linear Backoff

```yaml
gateway:
  middleware:
    retry:
      default:
        strategy: linear
        maxAttempts: 4
        initialDelay: 500ms
        delayIncrement: 500ms
        # Delays: 500ms, 1s, 1.5s, 2s
```

#### Fixed Delay

```yaml
gateway:
  middleware:
    retry:
      default:
        strategy: fixed
        maxAttempts: 3
        delay: 1s
        # Delays: 1s, 1s, 1s
```

#### Jittered Backoff

```yaml
gateway:
  middleware:
    retry:
      default:
        strategy: exponential-jitter
        maxAttempts: 5
        initialDelay: 100ms
        maxDelay: 5s
        jitterFactor: 0.3  # ±30% randomization
```

## Retry Conditions

### Retryable Errors

By default, retries occur for:
- Network errors
- HTTP 502, 503, 504 (gateway errors)
- Connection timeouts
- DNS resolution failures

### Non-Retryable Errors

Never retried:
- HTTP 4xx client errors
- HTTP 501 Not Implemented
- Request body read errors

### Custom Retry Conditions

```yaml
gateway:
  middleware:
    retry:
      default:
        retryableStatusCodes:
          - 429  # Rate limited
          - 500  # Internal server error
          - 502  # Bad gateway
          - 503  # Service unavailable
          - 504  # Gateway timeout
        
        retryableHeaders:
          - name: "X-Retry-After"
            value: "*"  # Any value
```

## Per-Route Configuration

```yaml
gateway:
  router:
    rules:
      - id: critical-api
        path: /api/critical/*
        serviceName: critical-service
        middleware:
          - retry:
              maxAttempts: 5
              strategy: exponential-jitter
              initialDelay: 50ms
      
      - id: batch-api
        path: /api/batch/*
        serviceName: batch-service
        middleware:
          - retry:
              maxAttempts: 10
              strategy: linear
              initialDelay: 2s
              maxDelay: 30s
```

## Retry Budgets

Prevent retry storms with budgets:

```yaml
gateway:
  middleware:
    retry:
      budget:
        enabled: true
        ratio: 0.2          # 20% retry budget
        minRetries: 10      # Always allow 10 retries/second
        window: 10s         # Rolling window
```

### How Budgets Work

1. Track total requests and retries
2. Calculate retry ratio
3. Allow retries if under budget
4. Always permit `minRetries`
5. Prevent cascade failures

### Per-Service Budgets

```yaml
gateway:
  middleware:
    retry:
      serviceBudgets:
        payment-service:
          ratio: 0.1       # Strict 10% for payments
          minRetries: 5
        
        analytics-service:
          ratio: 0.5       # Relaxed 50% for analytics
          minRetries: 20
```

## Idempotency

### Safe Methods

Always retried (RFC 7231):
- GET
- HEAD
- OPTIONS
- TRACE

### Unsafe Methods

Retried with caution:
- POST (only with idempotency key)
- PUT
- DELETE
- PATCH

### Idempotency Configuration

```yaml
gateway:
  middleware:
    retry:
      idempotency:
        enabled: true
        headerName: "X-Idempotency-Key"
        # Only retry POST/PUT/DELETE with key
        requireKeyForUnsafe: true
```

## Integration with Circuit Breaker

Retry and circuit breaker work together:

```yaml
gateway:
  router:
    rules:
      - id: protected-api
        middleware:
          # Order matters: retry wraps circuit breaker
          - retry:
              maxAttempts: 3
          - circuitbreaker:
              maxFailures: 10
```

### Coordination Logic

1. Retry attempts request
2. Circuit breaker checks state
3. If open, retry gets immediate failure
4. Retry applies backoff
5. Next attempt when circuit half-open

## Headers and Metadata

### Request Headers

Retry adds headers to upstream:

```
X-Retry-Attempt: 2
X-Retry-Max: 3
X-Retry-Reason: "http_502"
```

### Response Headers

Gateway adds retry info to client:

```
X-Gateway-Retries: 2
X-Gateway-Retry-Delay-Ms: 400
```

## Advanced Features

### Retry After Header

Respect server retry hints:

```yaml
gateway:
  middleware:
    retry:
      respectRetryAfter: true
      maxRetryAfterDelay: 60s
```

### Retry on Different Instance

```yaml
gateway:
  middleware:
    retry:
      tryDifferentInstance: true
      # Works with load balancer
```

### Conditional Retries

```yaml
gateway:
  middleware:
    retry:
      conditions:
        - header: "X-Retry-Allowed"
          value: "true"
        - method: ["GET", "POST"]
        - path: ["/api/*", "/v2/*"]
```

## Best Practices

### 1. Set Reasonable Limits

```yaml
# API calls: fast retry
api-service:
  maxAttempts: 3
  initialDelay: 100ms
  maxDelay: 2s

# Batch jobs: slow retry
batch-service:
  maxAttempts: 5
  initialDelay: 5s
  maxDelay: 60s
```

### 2. Use Jitter

Prevents thundering herd:

```yaml
strategy: exponential-jitter
jitterFactor: 0.5  # ±50% randomization
```

### 3. Monitor Retry Metrics

```
# Retry attempts
gateway_retry_attempts_total{service="payment"} 1234

# Retry success rate
gateway_retry_success_rate{service="payment"} 0.85

# Budget exhaustion
gateway_retry_budget_exhausted_total{service="payment"} 5
```

### 4. Test Retry Behavior

- Simulate failures
- Verify backoff timing
- Check budget limits
- Test idempotency

## Troubleshooting

### Too Many Retries

1. Reduce `maxAttempts`
2. Increase backoff delays
3. Enable retry budgets
4. Check service health

### Retries Not Working

1. Verify error is retryable
2. Check retry conditions
3. Review middleware order
4. Enable debug logging

### Budget Exhaustion

1. Increase budget ratio
2. Raise `minRetries`
3. Investigate root cause
4. Add circuit breaker

### Debug Logging

```yaml
gateway:
  logging:
    level: debug
    modules:
      middleware.retry: debug
```

Shows:
- Retry decisions
- Backoff calculations
- Budget state
- Error details

## Examples

### E-commerce Checkout

```yaml
checkout-api:
  retry:
    maxAttempts: 3
    strategy: exponential-jitter
    initialDelay: 200ms
    # Only retry with idempotency key
    idempotency:
      requireKeyForUnsafe: true
```

### External API Integration

```yaml
external-weather-api:
  retry:
    maxAttempts: 5
    strategy: exponential
    initialDelay: 1s
    maxDelay: 30s
    respectRetryAfter: true
```

### Internal Microservices

```yaml
internal-services:
  retry:
    maxAttempts: 4
    strategy: linear
    initialDelay: 100ms
    tryDifferentInstance: true
    budget:
      ratio: 0.3
```