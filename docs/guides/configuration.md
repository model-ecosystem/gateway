# Configuration Guide

The gateway is designed to work with minimal configuration while providing extensive customization options when needed.

## Minimal Configuration

Start with just 10 lines to proxy requests:

```yaml
gateway:
  router:
    rules:
      - path: /*
        serviceName: my-backend
  registry:
    static:
      services:
        - name: my-backend
          instances:
            - address: localhost
              port: 3000
```

That's it! The gateway will:
- Listen on `localhost:8080` (default)
- Route all requests to your backend at `localhost:3000`
- Use sensible defaults for timeouts, connection pooling, etc.

## Common Additions

### Custom Port

```yaml
gateway:
  frontend:
    http:
      port: 9000  # Default: 8080
```

### Multiple Routes

```yaml
gateway:
  router:
    rules:
      - path: /api/*
        serviceName: api-service
      - path: /static/*
        serviceName: static-service
```

### Authentication

Add JWT authentication to specific routes:

```yaml
gateway:
  auth:
    jwt:
      publicKey: |
        -----BEGIN PUBLIC KEY-----
        YOUR_PUBLIC_KEY_HERE
        -----END PUBLIC KEY-----
  
  router:
    rules:
      - path: /api/*
        serviceName: api-service
        authRequired: true  # Protect this route
```

### Rate Limiting

Protect your services from overload:

```yaml
gateway:
  router:
    rules:
      - path: /api/*
        serviceName: api-service
        rateLimit: 100      # 100 requests per second
        rateLimitBurst: 200 # Allow bursts up to 200
```

## Load Balancing

When you have multiple backend instances:

```yaml
gateway:
  registry:
    static:
      services:
        - name: api-service
          instances:
            - address: 10.0.0.1
              port: 3000
            - address: 10.0.0.2
              port: 3000
            - address: 10.0.0.3
              port: 3000
  
  router:
    rules:
      - path: /api/*
        serviceName: api-service
        loadBalance: round_robin  # Default behavior
```

## TLS/HTTPS

Enable HTTPS for the gateway:

```yaml
gateway:
  frontend:
    http:
      tls:
        enabled: true
        certFile: /path/to/cert.pem
        keyFile: /path/to/key.pem
```

## Complete Example

Here's a production-ready configuration combining common features:

```yaml
gateway:
  # Frontend (where clients connect)
  frontend:
    http:
      port: 443
      tls:
        enabled: true
        certFile: /etc/certs/fullchain.pem
        keyFile: /etc/certs/privkey.pem
  
  # Authentication
  auth:
    jwt:
      jwksEndpoint: https://auth.example.com/.well-known/jwks.json
  
  # Service Registry
  registry:
    static:
      services:
        - name: api-service
          instances:
            - address: api-1.internal
              port: 3000
            - address: api-2.internal
              port: 3000
  
  # Routing Rules
  router:
    rules:
      # Public endpoints
      - path: /health
        serviceName: api-service
        
      # Protected API
      - path: /api/*
        serviceName: api-service
        authRequired: true
        rateLimit: 1000
        rateLimitBurst: 2000
```

## Advanced Configuration

### Protocol Support

The gateway supports multiple protocols:

```yaml
gateway:
  # WebSocket support
  frontend:
    websocket:
      enabled: true
  
  # gRPC backend
  backend:
    grpc:
      enabled: true
  
  router:
    rules:
      - path: /ws/*
        serviceName: websocket-service
        protocol: websocket
      
      - path: /grpc/*
        serviceName: grpc-service
        protocol: grpc
```

### Health Checks

Monitor backend health automatically:

```yaml
gateway:
  health:
    enabled: true
    interval: 30s
    checks:
      - type: http
        services: ["api-service"]
        path: /health
        expectedStatus: 200
```

### Circuit Breakers

Protect against cascading failures:

```yaml
gateway:
  circuitBreaker:
    enabled: true
    failureThreshold: 5     # Open after 5 failures
    successThreshold: 2     # Close after 2 successes
    timeout: 60s           # Try again after 60s
```

### Observability

Enable metrics and tracing:

```yaml
gateway:
  telemetry:
    enabled: true
    otlp:
      endpoint: localhost:4317
    service:
      name: api-gateway
      version: 1.0.0
```

## Configuration Reference

### Default Values

When values are omitted, these defaults apply:

**Frontend:**
- Host: `127.0.0.1`
- Port: `8080`
- Read timeout: `30s`
- Write timeout: `30s`

**Backend:**
- Connection timeout: `10s`
- Keep-alive: `enabled`
- Max idle connections: `100`

**Router:**
- Load balance: `round_robin`
- Request timeout: `30s`

**Health Checks:**
- Interval: `30s`
- Timeout: `5s`

### Environment Variables

Override configuration with environment variables:

```bash
# Override port
GATEWAY_FRONTEND_HTTP_PORT=9000

# Override backend timeout
GATEWAY_BACKEND_HTTP_DIALTIMEOUT=5
```

### Dynamic Reloading

The gateway can reload configuration without restart:

```yaml
gateway:
  config:
    watch: true           # Watch for config changes
    reloadDebounce: 5s    # Wait 5s before reloading
```

## Best Practices

1. **Start Simple**: Begin with minimal configuration and add features as needed
2. **Use Defaults**: The defaults are optimized for most use cases
3. **Monitor Health**: Always configure health checks for production
4. **Secure by Default**: Enable TLS and authentication for public endpoints
5. **Plan for Scale**: Configure rate limiting before you need it

## Next Steps

- [Authentication Guide](authentication.md) - Secure your APIs
- [TLS Setup Guide](tls-setup.md) - Enable HTTPS
- [Rate Limiting Guide](rate-limiting.md) - Protect from overload
- [Deployment Guide](deployment.md) - Production deployment