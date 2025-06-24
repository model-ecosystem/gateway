# Rate Limiting Guide

This guide explains how to configure and use rate limiting in the gateway to protect your backend services from excessive requests.

## Overview

The gateway provides flexible rate limiting using a token bucket algorithm:

- Per-route rate limits with different settings for each endpoint
- Configurable burst capacity for handling traffic spikes
- Multiple storage backends (memory or Redis)
- Distributed rate limiting across gateway instances
- Standard HTTP 429 (Too Many Requests) responses

## Basic Configuration

Rate limiting is configured per route. Each route can have independent rate limit settings.

### Simple Rate Limiting

```yaml
gateway:
  router:
    rules:
      - id: api-route
        path: /api/*
        serviceName: api-service
        loadBalance: round_robin
        rateLimit: 100        # 100 requests per second
        rateLimitBurst: 200   # Allow burst up to 200 requests
```

### Configuration Options

- **rateLimit**: The sustained request rate (requests per second)
- **rateLimitBurst**: The maximum burst size (defaults to rateLimit if not specified)
- **rateLimitStorage**: Storage backend to use (defaults to "memory")

## Storage Backends

### Memory Storage (Default)

Memory storage is the default option with no external dependencies:
- Fast and efficient for single-instance deployments
- Rate limits are per-instance (not shared)
- Automatically cleans up expired entries
- No configuration required

### Redis Storage

Redis storage enables distributed rate limiting across multiple gateway instances:
- Shared rate limits across all gateways
- Supports Redis Cluster and Sentinel
- Atomic operations using Lua scripts
- Automatic fallback to memory if Redis unavailable

## Advanced Configuration

### Global Storage Configuration

Define available storage backends:

```yaml
gateway:
  # Define available storage backends
  rateLimitStorage:
    default: "memory"  # Default storage for routes
    stores:
      # Memory storage
      memory:
        type: "memory"
      
      # Redis storage
      redis-primary:
        type: "redis"
        redis:
          host: "localhost"
          port: 6379
          db: 0
          password: ""
          # Connection pool settings
          poolSize: 10
          minIdleConns: 5
          idleTimeout: 300
          # Retry settings
          maxRetries: 3
          minRetryBackoff: 8
          maxRetryBackoff: 512
```

### Per-Route Storage Selection

```yaml
gateway:
  router:
    rules:
      # Route using memory storage
      - id: public-api
        path: /api/public/*
        serviceName: public-service
        rateLimit: 1000
        rateLimitBurst: 2000
        # Uses default storage (memory)
      
      # Route using Redis storage
      - id: user-api
        path: /api/users/*
        serviceName: api-service
        rateLimit: 50
        rateLimitBurst: 100
        rateLimitStorage: "redis-primary"
```

## Redis Configuration Options

### Basic Redis

```yaml
redis:
  host: "localhost"
  port: 6379
  db: 0
  password: ""
```

### Redis Cluster

```yaml
redis:
  cluster: true
  addresses:
    - "redis-node1:6379"
    - "redis-node2:6379"
    - "redis-node3:6379"
  password: ""
```

### Redis Sentinel

```yaml
redis:
  sentinel: true
  masterName: "mymaster"
  sentinelAddresses:
    - "sentinel1:26379"
    - "sentinel2:26379"
    - "sentinel3:26379"
  password: ""
```

### Redis with TLS

```yaml
redis:
  host: "redis.example.com"
  port: 6379
  tls:
    enabled: true
    certFile: "/path/to/cert.pem"
    keyFile: "/path/to/key.pem"
    caFile: "/path/to/ca.pem"
    insecureSkipVerify: false
```

## Example Scenarios

### Different Limits for Different Routes

```yaml
gateway:
  router:
    rules:
      # Public API with generous limits
      - id: public-api
        path: /api/public/*
        serviceName: public-service
        rateLimit: 1000
        rateLimitBurst: 2000
      
      # Authenticated API with moderate limits
      - id: user-api
        path: /api/users/*
        serviceName: api-service
        rateLimit: 100
        rateLimitBurst: 200
      
      # Admin API with strict limits
      - id: admin-api
        path: /api/admin/*
        serviceName: admin-service
        rateLimit: 10
        rateLimitBurst: 20
```

### Multi-Instance Gateway with Shared Limits

For multiple gateway instances sharing rate limits:

```yaml
gateway:
  rateLimitStorage:
    default: "redis-shared"
    stores:
      redis-shared:
        type: "redis"
        redis:
          host: "redis.internal"
          port: 6379
  
  router:
    rules:
      - id: api
        path: /api/*
        serviceName: backend
        rateLimit: 1000  # 1000 req/s shared across all instances
        rateLimitBurst: 2000
```

### Mixed Storage Strategy

Different storage for different routes based on requirements:

```yaml
gateway:
  rateLimitStorage:
    default: "memory"
    stores:
      memory:
        type: "memory"
      
      redis-critical:
        type: "redis"
        redis:
          host: "redis-ha.internal"
          port: 6379
  
  router:
    rules:
      # Public endpoints - local rate limiting
      - id: public
        path: /public/*
        serviceName: public-backend
        rateLimit: 1000
        # Uses default (memory)
      
      # Payment API - needs global rate limiting
      - id: payments
        path: /api/payments/*
        serviceName: payment-backend
        rateLimit: 10
        rateLimitStorage: "redis-critical"
```

### No Rate Limiting for Specific Routes

To disable rate limiting for a specific route, simply don't specify the rateLimit field:

```yaml
gateway:
  router:
    rules:
      # Health check endpoint - no rate limiting
      - id: health
        path: /health
        serviceName: health-service
        # No rateLimit field - unlimited requests
```

## Rate Limiting Algorithm

The gateway uses a token bucket algorithm with sliding window:

1. **Token Bucket**: Each key (e.g., IP address) has a bucket with tokens
2. **Rate**: Tokens are replenished at the configured rate
3. **Burst**: Maximum tokens that can accumulate (burst capacity)
4. **Sliding Window**: Ensures smooth rate limiting without sudden resets

## Response Headers

When rate limiting is active, the gateway adds standard headers to responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1640995200
```

When rate limit is exceeded:
- HTTP Status: 429 Too Many Requests
- Body: `{"error": "rate limit exceeded"}`

## Monitoring and Debugging

### Logs

The gateway logs rate limiting decisions:

```
INFO Creating memory limiter store name=memory
INFO Creating Redis limiter store name=redis-primary host=localhost port=6379
INFO Rate limiting configured for route route=api-route rate=100 burst=200 storage=memory
WARN Rate limit exceeded key=192.168.1.100 path=/api/users method=GET
```

### Metrics

When metrics are enabled, rate limiting metrics are exposed:
- `gateway_ratelimit_allowed_total`: Total allowed requests
- `gateway_ratelimit_denied_total`: Total denied requests
- `gateway_ratelimit_remaining`: Remaining tokens per key

## Best Practices

1. **Start with Memory Storage**: Use memory storage for development and single-instance deployments
2. **Use Redis for Production**: Enable Redis storage for multi-instance production deployments
3. **Configure Appropriate Limits**: Set rate limits based on your service capacity
4. **Monitor and Adjust**: Use metrics to fine-tune rate limits
5. **Consider Burst Capacity**: Allow reasonable burst to handle traffic spikes
6. **Separate Storage by Criticality**: Use different storage backends for different API criticality levels
7. **Protect Critical Endpoints**: Apply stricter limits to expensive operations

## Troubleshooting

### Redis Connection Issues

If Redis is unavailable, the gateway falls back to memory storage:

```
WARN Failed to create Redis client, falling back to memory store name=redis-primary error=dial tcp localhost:6379: connection refused
```

### Rate Limit Key Selection

By default, rate limiting is per IP address. The gateway uses:
- `X-Forwarded-For` header if present (for proxied requests)
- `X-Real-IP` header as fallback
- Direct connection IP as last resort

### Performance Considerations

- **Memory storage**: Minimal overhead, scales with number of unique clients
- **Redis storage**: Network latency added (typically 1-2ms), but enables horizontal scaling

### Common Issues

1. **Rate limits not working**: Ensure the route has `rateLimit` configured
2. **429 errors in development**: Check if limits are too low for testing
3. **Inconsistent limiting**: Verify all gateway instances use the same Redis backend
4. **Memory growth**: Normal for memory storage; entries expire automatically

## See Also

- [Configuration Guide](configuration.md) - Complete configuration reference
- [Resilience Guide](resilience.md) - Circuit breakers and retry logic
- [Metrics Guide](metrics.md) - Monitoring and observability