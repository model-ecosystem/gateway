# Management API

The gateway provides a comprehensive REST API for runtime management, monitoring, and configuration of the gateway instance.

## Overview

The Management API allows you to:
- Monitor gateway health and metrics
- View and update routing configuration
- Manage circuit breakers and rate limits
- Access service discovery information
- Control gateway lifecycle
- Debug and troubleshoot issues

## Configuration

### Enabling the Management API

```yaml
gateway:
  management:
    enabled: true
    host: "127.0.0.1"  # Bind to localhost only for security
    port: 9001
    
    # Authentication
    auth:
      type: "bearer"
      token: "${MANAGEMENT_API_TOKEN}"
    
    # CORS settings
    cors:
      enabled: true
      origins:
        - "http://localhost:3000"
        - "https://admin.example.com"
```

### Security Configuration

```yaml
gateway:
  management:
    tls:
      enabled: true
      certFile: "/certs/management.crt"
      keyFile: "/certs/management.key"
      clientAuth: true
      caFile: "/certs/ca.crt"
    
    # IP whitelist
    allowedIPs:
      - "10.0.0.0/8"
      - "172.16.0.0/12"
      - "192.168.0.0/16"
```

## API Endpoints

### Health and Status

#### Get Gateway Health

```http
GET /health
```

Response:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": 3600,
  "timestamp": "2024-01-15T10:30:00Z",
  "components": {
    "router": "healthy",
    "registry": "healthy",
    "middleware": "healthy"
  }
}
```

#### Get Gateway Info

```http
GET /info
```

Response:
```json
{
  "version": "1.0.0",
  "buildTime": "2024-01-15T08:00:00Z",
  "gitCommit": "abc123",
  "goVersion": "1.21",
  "platform": "linux/amd64",
  "config": {
    "frontendPort": 8080,
    "backendCount": 5,
    "middlewareEnabled": ["auth", "ratelimit", "cors"]
  }
}
```

### Service Management

#### List Services

```http
GET /services
```

Response:
```json
{
  "services": [
    {
      "name": "user-service",
      "instances": 3,
      "healthy": 3,
      "unhealthy": 0,
      "loadBalancer": "round_robin"
    },
    {
      "name": "order-service",
      "instances": 2,
      "healthy": 1,
      "unhealthy": 1,
      "loadBalancer": "least_connections"
    }
  ]
}
```

#### Get Service Details

```http
GET /services/{service-name}
```

Response:
```json
{
  "name": "user-service",
  "instances": [
    {
      "id": "user-1",
      "address": "10.0.0.1",
      "port": 8080,
      "healthy": true,
      "weight": 100,
      "activeConnections": 45,
      "totalRequests": 12345,
      "errorRate": 0.001,
      "avgResponseTime": 23.5
    }
  ],
  "configuration": {
    "timeout": 30,
    "retries": 3,
    "circuitBreaker": {
      "enabled": true,
      "threshold": 5,
      "timeout": 60
    }
  }
}
```

#### Update Service Instance

```http
PUT /services/{service-name}/instances/{instance-id}
```

Request:
```json
{
  "weight": 50,
  "enabled": false,
  "drain": true
}
```

### Route Management

#### List Routes

```http
GET /routes
```

Response:
```json
{
  "routes": [
    {
      "id": "api-v1",
      "path": "/api/v1/*",
      "service": "api-service",
      "methods": ["GET", "POST"],
      "middleware": ["auth", "ratelimit"],
      "priority": 100,
      "enabled": true
    }
  ]
}
```

#### Update Route

```http
PUT /routes/{route-id}
```

Request:
```json
{
  "enabled": false,
  "middleware": ["auth", "ratelimit", "transform"],
  "metadata": {
    "version": "v1",
    "deprecated": true
  }
}
```

#### Reload Routes

```http
POST /routes/reload
```

Response:
```json
{
  "status": "success",
  "routesLoaded": 15,
  "routesUpdated": 3,
  "routesRemoved": 1
}
```

### Circuit Breaker Management

#### Get Circuit Breakers

```http
GET /circuit-breakers
```

Response:
```json
{
  "breakers": [
    {
      "name": "user-service",
      "state": "closed",
      "failures": 2,
      "successRate": 0.98,
      "lastFailure": "2024-01-15T10:25:00Z",
      "lastStateChange": "2024-01-15T09:00:00Z"
    }
  ]
}
```

#### Reset Circuit Breaker

```http
POST /circuit-breakers/{name}/reset
```

Response:
```json
{
  "status": "success",
  "previousState": "open",
  "currentState": "closed"
}
```

### Rate Limit Management

#### Get Rate Limits

```http
GET /rate-limits
```

Response:
```json
{
  "limits": [
    {
      "id": "api-limit",
      "path": "/api/*",
      "limit": 1000,
      "window": "1m",
      "type": "sliding",
      "currentUsage": 234
    }
  ]
}
```

#### Update Rate Limit

```http
PUT /rate-limits/{limit-id}
```

Request:
```json
{
  "limit": 2000,
  "burst": 100
}
```

### Configuration Management

#### Get Current Configuration

```http
GET /config
```

Response:
```json
{
  "gateway": {
    "frontend": {
      "http": {
        "port": 8080,
        "readTimeout": 30,
        "writeTimeout": 30
      }
    },
    "backend": {
      "http": {
        "maxIdleConns": 100,
        "timeout": 30
      }
    }
  }
}
```

#### Update Configuration

```http
PATCH /config
```

Request:
```json
{
  "path": "/gateway/backend/http/timeout",
  "value": 60
}
```

### Metrics and Monitoring

#### Get Metrics

```http
GET /metrics
```

Response (Prometheus format):
```text
# HELP gateway_requests_total Total requests processed
# TYPE gateway_requests_total counter
gateway_requests_total{method="GET",path="/api/users",status="200"} 12345

# HELP gateway_request_duration_seconds Request duration
# TYPE gateway_request_duration_seconds histogram
gateway_request_duration_seconds_bucket{le="0.1"} 10000
gateway_request_duration_seconds_bucket{le="0.5"} 11000
```

#### Get Stats

```http
GET /stats
```

Response:
```json
{
  "uptime": 3600,
  "requests": {
    "total": 1000000,
    "rate": 150.5,
    "errors": 523,
    "errorRate": 0.0005
  },
  "connections": {
    "active": 234,
    "idle": 766,
    "total": 1000
  },
  "memory": {
    "alloc": 256,
    "totalAlloc": 1024,
    "sys": 512,
    "numGC": 42
  }
}
```

### Debug Endpoints

#### Get Debug Info

```http
GET /debug/pprof/
```

Available profiles:
- `/debug/pprof/heap`
- `/debug/pprof/goroutine`
- `/debug/pprof/cpu`
- `/debug/pprof/trace`

#### Get Goroutine Stack

```http
GET /debug/stack
```

#### Enable Debug Logging

```http
POST /debug/log-level
```

Request:
```json
{
  "level": "debug",
  "modules": ["router", "registry"]
}
```

## WebSocket Support

### Real-time Events

```javascript
const ws = new WebSocket('wss://gateway:9001/events');

ws.on('message', (data) => {
  const event = JSON.parse(data);
  console.log('Event:', event.type, event.data);
});
```

Event types:
- `route.added`
- `route.updated`
- `route.removed`
- `service.healthy`
- `service.unhealthy`
- `circuitbreaker.open`
- `circuitbreaker.closed`
- `ratelimit.exceeded`

## Authentication

### Bearer Token

```http
GET /health
Authorization: Bearer <token>
```

### mTLS

Configure client certificate:
```bash
curl --cert client.crt \
     --key client.key \
     --cacert ca.crt \
     https://gateway:9001/health
```

## SDKs and Tools

### CLI Tool

```bash
# Install CLI
go install github.com/gateway/gwctl@latest

# Configure
gwctl config set endpoint https://gateway:9001
gwctl config set token $MANAGEMENT_TOKEN

# Use
gwctl services list
gwctl routes update api-v1 --disable
gwctl circuit-breaker reset user-service
```

### Go SDK

```go
import "github.com/gateway/management-sdk-go"

client := management.New("https://gateway:9001", token)

// List services
services, err := client.Services().List()

// Update route
err = client.Routes().Update("api-v1", &RouteUpdate{
    Enabled: false,
})
```

### Python SDK

```python
from gateway_management import Client

client = Client("https://gateway:9001", token=token)

# Get health
health = client.health()

# Reset circuit breaker
client.circuit_breakers.reset("user-service")
```

## Best Practices

1. **Secure Access**: Always use authentication and TLS
2. **Limit Exposure**: Bind to localhost or internal network
3. **Rate Limit**: Apply rate limits to management endpoints
4. **Audit Logging**: Log all management operations
5. **Monitoring**: Monitor management API usage
6. **Graceful Updates**: Use maintenance mode for updates

## Error Responses

Standard error format:
```json
{
  "error": {
    "code": "ROUTE_NOT_FOUND",
    "message": "Route 'api-v3' not found",
    "details": {
      "routeId": "api-v3",
      "suggestion": "Use GET /routes to list available routes"
    }
  }
}
```

Common error codes:
- `UNAUTHORIZED` - Invalid or missing authentication
- `FORBIDDEN` - Insufficient permissions
- `NOT_FOUND` - Resource not found
- `VALIDATION_ERROR` - Invalid request data
- `CONFLICT` - Resource conflict
- `INTERNAL_ERROR` - Server error