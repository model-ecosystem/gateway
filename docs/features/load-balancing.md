# Load Balancing

The gateway provides multiple load balancing algorithms to distribute traffic across backend service instances for optimal performance and reliability.

## Overview

Load balancing features include:
- Multiple algorithm implementations
- Health-aware load balancing
- Session affinity (sticky sessions)
- Weight-based distribution
- Dynamic instance management
- Performance-based routing

## Load Balancing Algorithms

### Round Robin

Distributes requests evenly across all healthy instances:

```yaml
gateway:
  router:
    rules:
      - id: api-route
        path: /api/*
        serviceName: api-service
        loadBalance: round_robin
```

### Least Connections

Routes to the instance with the fewest active connections:

```yaml
gateway:
  router:
    rules:
      - id: ws-route
        path: /ws/*
        serviceName: websocket-service
        loadBalance: least_connections
        leastConnections:
          # Consider request weight
          weightedConnections: true
```

### Weighted Round Robin

Distributes based on instance weights:

```yaml
gateway:
  router:
    rules:
      - id: weighted-route
        path: /api/*
        serviceName: api-service
        loadBalance: weighted_round_robin

  registry:
    static:
      services:
        - name: api-service
          instances:
            - id: api-1
              address: 10.0.0.1
              port: 8080
              weight: 100  # Gets more traffic
            - id: api-2
              address: 10.0.0.2
              port: 8080
              weight: 50   # Gets less traffic
```

### Consistent Hashing

Uses consistent hashing for stable routing:

```yaml
gateway:
  router:
    rules:
      - id: cache-route
        path: /cache/*
        serviceName: cache-service
        loadBalance: consistent_hash
        consistentHash:
          # Hash key source
          key: header:X-User-ID
          # Virtual nodes for better distribution
          replicaCount: 150
```

### Response Time Based

Routes based on response time percentiles:

```yaml
gateway:
  router:
    rules:
      - id: latency-route
        path: /api/*
        serviceName: api-service
        loadBalance: response_time
        responseTime:
          # Use p95 response time
          percentile: 95
          # Measurement window
          window: 1m
          # Update frequency
          updateInterval: 10s
```

### Resource Based

Routes based on resource utilization:

```yaml
gateway:
  router:
    rules:
      - id: resource-route
        path: /compute/*
        serviceName: compute-service
        loadBalance: resource_based
        resourceBased:
          # Metrics to consider
          metrics:
            - type: cpu
              weight: 0.4
            - type: memory
              weight: 0.3
            - type: connections
              weight: 0.3
```

### Random

Simple random selection:

```yaml
gateway:
  router:
    rules:
      - id: random-route
        path: /test/*
        serviceName: test-service
        loadBalance: random
```

### IP Hash

Routes based on client IP:

```yaml
gateway:
  router:
    rules:
      - id: ip-route
        path: /api/*
        serviceName: api-service
        loadBalance: ip_hash
        ipHash:
          # Use X-Forwarded-For if present
          useForwardedFor: true
          # Subnet mask for grouping
          subnetMask: 24
```

## Session Affinity

### Cookie-Based Affinity

```yaml
gateway:
  router:
    rules:
      - id: session-route
        path: /app/*
        serviceName: app-service
        sessionAffinity:
          enabled: true
          type: cookie
          cookie:
            name: GATEWAY_SESSION
            path: /
            maxAge: 3600
            httpOnly: true
            secure: true
            sameSite: strict
```

### Header-Based Affinity

```yaml
gateway:
  router:
    rules:
      - id: header-route
        path: /api/*
        serviceName: api-service
        sessionAffinity:
          enabled: true
          type: header
          header:
            name: X-Session-ID
            # Create session if missing
            createIfMissing: true
```

### IP-Based Affinity

```yaml
gateway:
  router:
    rules:
      - id: ip-affinity-route
        path: /stream/*
        serviceName: stream-service
        sessionAffinity:
          enabled: true
          type: ip
          ip:
            # Include port in hash
            includePort: false
            # Use X-Real-IP if available
            useRealIP: true
```

## Advanced Load Balancing

### Multi-Level Load Balancing

Combine multiple algorithms:

```yaml
gateway:
  router:
    rules:
      - id: multi-level-route
        path: /api/*
        serviceName: api-service
        loadBalance: multi_level
        multiLevel:
          # First level: data center
          - level: datacenter
            algorithm: weighted_round_robin
            key: metadata.datacenter
          # Second level: instance within DC
          - level: instance
            algorithm: least_connections
```

### Adaptive Load Balancing

Automatically adjust based on performance:

```yaml
gateway:
  router:
    rules:
      - id: adaptive-route
        path: /api/*
        serviceName: api-service
        loadBalance: adaptive
        adaptive:
          # Start with round robin
          initialAlgorithm: round_robin
          # Switch based on metrics
          triggers:
            - metric: error_rate
              threshold: 0.05
              algorithm: least_connections
            - metric: response_time_p99
              threshold: 500ms
              algorithm: response_time
```

### Canary Deployments

Gradual traffic shifting:

```yaml
gateway:
  router:
    rules:
      - id: canary-route
        path: /api/*
        loadBalance: canary
        canary:
          # Main service
          stable:
            serviceName: api-stable
            weight: 90
          # Canary service
          canary:
            serviceName: api-canary
            weight: 10
          # Gradual rollout
          rollout:
            enabled: true
            increment: 5
            interval: 5m
            maxWeight: 50
```

### A/B Testing

Route based on experiment groups:

```yaml
gateway:
  router:
    rules:
      - id: ab-route
        path: /api/*
        loadBalance: ab_test
        abTest:
          experiments:
            - name: new-feature
              service: api-v2
              percentage: 20
              criteria:
                - header: X-Beta-User
                  value: "true"
            - name: control
              service: api-v1
              percentage: 80
```

## Health-Aware Load Balancing

### Health Scoring

```yaml
gateway:
  router:
    healthAware:
      enabled: true
      scoring:
        # Factors for health score
        factors:
          - type: success_rate
            weight: 0.4
            window: 1m
          - type: response_time
            weight: 0.3
            threshold: 200ms
          - type: active_connections
            weight: 0.2
            max: 1000
          - type: circuit_breaker
            weight: 0.1
      # Minimum score to receive traffic
      minScore: 0.5
      # Gradual traffic reduction
      degradation:
        enabled: true
        curve: linear  # linear, exponential
```

### Outlier Detection

```yaml
gateway:
  router:
    outlierDetection:
      enabled: true
      # Consecutive errors before ejection
      consecutiveErrors: 5
      # Error rate threshold
      errorRate: 0.5
      # Ejection duration
      baseEjectionTime: 30s
      # Max ejection percentage
      maxEjectionPercent: 50
      # Analysis interval
      interval: 10s
```

## Performance Optimization

### Connection Pooling

```yaml
gateway:
  router:
    connectionPool:
      # Max connections per instance
      maxConnectionsPerInstance: 100
      # Max pending requests
      maxPendingRequests: 1000
      # Connection timeout
      connectTimeout: 5s
      # Idle timeout
      idleTimeout: 90s
```

### Request Hedging

Send redundant requests to reduce latency:

```yaml
gateway:
  router:
    rules:
      - id: hedged-route
        path: /api/critical/*
        serviceName: api-service
        hedging:
          enabled: true
          # Hedge after delay
          delay: 50ms
          # Max hedged requests
          maxAttempts: 2
          # Only for safe methods
          methods: [GET, HEAD]
```

### Retry with Different Instance

```yaml
gateway:
  router:
    retry:
      # Try different instances
      tryDifferentInstance: true
      # Exclude failed instances
      excludeFailed: true
      excludeDuration: 10s
```

## Monitoring

### Metrics

Load balancing metrics:
- `gateway_lb_selections_total` - Total selections per algorithm
- `gateway_lb_instance_connections` - Active connections per instance
- `gateway_lb_instance_requests_total` - Requests per instance
- `gateway_lb_instance_errors_total` - Errors per instance
- `gateway_lb_instance_latency_seconds` - Response time per instance
- `gateway_lb_rebalance_total` - Instance pool changes

### Load Distribution Analysis

```yaml
gateway:
  router:
    analytics:
      enabled: true
      # Track distribution fairness
      distribution:
        enabled: true
        window: 5m
        reportInterval: 1m
      # Detect imbalances
      imbalanceDetection:
        enabled: true
        threshold: 0.2  # 20% deviation
        action: alert
```

## Configuration Examples

### High-Performance API

```yaml
gateway:
  router:
    rules:
      - id: high-perf-api
        path: /api/v2/*
        serviceName: api-service
        loadBalance: least_connections
        sessionAffinity:
          enabled: false
        connectionPool:
          maxConnectionsPerInstance: 200
        healthCheck:
          interval: 1s
          fastFail: true
```

### Stateful WebSocket

```yaml
gateway:
  router:
    rules:
      - id: websocket
        path: /ws/*
        serviceName: ws-service
        loadBalance: consistent_hash
        consistentHash:
          key: header:X-User-ID
        sessionAffinity:
          enabled: true
          type: cookie
          ttl: 24h
        connectionPool:
          maxConnectionsPerInstance: 10000
```

### Geographic Distribution

```yaml
gateway:
  router:
    rules:
      - id: geo-api
        path: /api/*
        serviceName: api-service
        loadBalance: geographic
        geographic:
          # Prefer local region
          preferLocal: true
          # Fallback to nearest
          fallback: nearest
          # Region detection
          detection:
            - source: header:X-Region
            - source: geoip
```

## Best Practices

1. **Choose Appropriate Algorithm**: Match algorithm to workload characteristics
2. **Monitor Distribution**: Track request distribution across instances
3. **Set Connection Limits**: Prevent overwhelming individual instances
4. **Use Health Checks**: Ensure only healthy instances receive traffic
5. **Configure Timeouts**: Set appropriate connection and request timeouts
6. **Test Failover**: Verify behavior when instances fail
7. **Consider Affinity**: Use session affinity for stateful applications

## Troubleshooting

### Uneven Distribution

- Check instance weights
- Verify health check status
- Review session affinity settings
- Analyze request patterns

### Connection Errors

- Verify connection pool settings
- Check instance capacity
- Review timeout configurations
- Monitor network health

### Performance Issues

- Enable connection pooling
- Consider response-time routing
- Implement request hedging
- Review outlier detection