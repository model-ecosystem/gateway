# Resilience Guide

This guide covers the gateway's resilience features including circuit breakers, retry logic, and advanced load balancing algorithms.

## Circuit Breaker Pattern

The gateway implements the circuit breaker pattern to prevent cascading failures and protect backend services.

### Configuration

```yaml
gateway:
  circuitBreaker:
    enabled: true
    failureThreshold: 5      # Failures before opening
    successThreshold: 2      # Successes before closing
    timeout: 60s            # Time before half-open
    halfOpenRequests: 3     # Requests in half-open state
```

### How It Works

1. **Closed State**: Normal operation, requests pass through
2. **Open State**: Requests fail fast without calling backend
3. **Half-Open State**: Limited requests test if service recovered

### Per-Route Circuit Breakers

Configure circuit breakers per route:

```yaml
router:
  rules:
    - id: critical-api
      path: /api/critical/*
      serviceName: critical-service
      metadata:
        circuitBreaker:
          enabled: true
          failureThreshold: 3  # More sensitive
          timeout: 30s         # Faster recovery attempts
```

### Circuit Breaker Metrics

Monitor circuit breaker state:
- State changes logged with reason
- Metrics track state transitions
- Consecutive failures/successes tracked
- Last state change timestamp available

## Retry Logic with Budget

The gateway implements intelligent retry logic with budget tracking to prevent retry storms.

### Configuration

```yaml
gateway:
  retry:
    enabled: true
    maxAttempts: 3
    initialInterval: 100ms
    maxInterval: 2s
    multiplier: 2.0
    retryableStatusCodes: [502, 503, 504]
    budget:
      ratio: 0.1        # 10% retry budget
      minRequests: 100  # Minimum before enforcement
```

### Retry Budget

The retry budget prevents retry storms by limiting the percentage of requests that can be retried:

- Tracks retry attempts over a time window
- Enforces maximum retry ratio
- Prevents cascading failures
- Allows bursts within limits

### Per-Route Retry Configuration

```yaml
router:
  rules:
    - id: resilient-api
      path: /api/resilient/*
      serviceName: backend-service
      metadata:
        retry:
          maxAttempts: 5
          initialInterval: 50ms
          retryableStatusCodes: [500, 502, 503]
```

### Retry Strategies

1. **Exponential Backoff**: Default strategy with configurable multiplier
2. **Jittered Backoff**: Adds randomization to prevent thundering herd
3. **Status-Based**: Only retry specific HTTP status codes
4. **Budget-Aware**: Respects global retry budget

## Advanced Load Balancing

The gateway supports multiple advanced load balancing algorithms beyond basic round-robin.

### Weighted Round-Robin

Distribute traffic based on instance weights:

```yaml
registry:
  static:
    services:
      - name: api-service
        instances:
          - id: powerful-instance
            address: "10.0.0.1"
            port: 8080
            metadata:
              weight: 3  # Gets 3x traffic
          - id: standard-instance
            address: "10.0.0.2"
            port: 8080
            metadata:
              weight: 1  # Gets 1x traffic
              
router:
  rules:
    - id: weighted-route
      path: /api/*
      serviceName: api-service
      loadBalance: weighted_round_robin
```

### Least Connections

Route to instance with fewest active connections:

```yaml
router:
  rules:
    - id: connection-balanced
      path: /api/compute/*
      serviceName: compute-service
      loadBalance: least_connections
```

Features:
- Tracks active connections per instance
- Atomic connection counting
- Automatic decrement on completion
- Ideal for long-lived connections

### Response Time Based

Route to instances with best response times:

```yaml
router:
  rules:
    - id: latency-sensitive
      path: /api/realtime/*
      serviceName: realtime-service
      loadBalance: response_time
```

Features:
- Tracks response time history
- Uses EWMA for smooth averaging
- Prefers consistently fast instances
- Adapts to performance changes

### Adaptive Load Balancing

Automatically selects best algorithm based on performance:

```yaml
router:
  rules:
    - id: adaptive-route
      path: /api/adaptive/*
      serviceName: adaptive-service
      loadBalance: adaptive
```

Features:
- Combines multiple strategies
- Learns from request outcomes
- Adjusts weights dynamically
- Optimizes for success rate and latency

### Weighted Random

Random selection with weight bias:

```yaml
router:
  rules:
    - id: random-weighted
      path: /api/random/*
      serviceName: weighted-service
      loadBalance: weighted_random
```

## Resilience Best Practices

### 1. Layer Your Defenses

Combine multiple resilience patterns:
- Circuit breakers prevent cascading failures
- Retries handle transient errors
- Load balancing distributes load
- Health checks remove bad instances

### 2. Configure Timeouts Properly

```yaml
router:
  rules:
    - id: api-route
      timeout: 30s          # Request timeout
      metadata:
        retry:
          maxAttempts: 3
        circuitBreaker:
          timeout: 60s      # Circuit breaker timeout
```

### 3. Monitor Resilience Metrics

Key metrics to track:
- Circuit breaker state changes
- Retry attempt rates
- Retry budget consumption
- Load balancer selection distribution
- Instance health transitions

### 4. Test Failure Scenarios

Regular testing should include:
- Backend service failures
- Slow response times
- Connection timeouts
- Partial failures
- Recovery scenarios

### 5. Use Gradual Rollouts

When deploying changes:
- Start with low traffic routes
- Monitor error rates closely
- Adjust thresholds based on observed behavior
- Document failure patterns

## Example: Highly Resilient Configuration

```yaml
gateway:
  # Global circuit breaker
  circuitBreaker:
    enabled: true
    failureThreshold: 5
    successThreshold: 2
    timeout: 60s
    
  # Smart retry with budget
  retry:
    enabled: true
    maxAttempts: 3
    initialInterval: 100ms
    budget:
      ratio: 0.1
      
  # Active health monitoring  
  health:
    enabled: true
    interval: 10s
    timeout: 2s
    
  router:
    rules:
      - id: critical-api
        path: /api/critical/*
        serviceName: critical-service
        loadBalance: adaptive
        timeout: 10s
        metadata:
          authRequired: true
          rateLimit: 1000
          retry:
            maxAttempts: 2  # Less aggressive
          circuitBreaker:
            failureThreshold: 3  # More sensitive
```

## Troubleshooting

### Circuit Breaker Always Open
- Check failure threshold vs actual error rate
- Verify backend service health
- Review timeout configuration
- Check success threshold isn't too high

### Retry Storms
- Monitor retry budget consumption
- Reduce retry attempts
- Increase backoff intervals
- Check for systematic failures

### Uneven Load Distribution
- Verify instance weights
- Check health status of instances
- Monitor response times
- Review load balancer algorithm choice