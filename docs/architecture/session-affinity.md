# Session Affinity Architecture

## Overview

Session affinity (sticky sessions) ensures that requests from the same client are consistently routed to the same backend instance. This is crucial for stateful services that maintain session state in memory.

## Design Principles

1. **Protocol Agnostic**: Works consistently across HTTP, WebSocket, SSE, and gRPC
2. **Configuration Driven**: No hardcoded session extraction logic
3. **Pluggable Extractors**: Easy to add new session sources
4. **Clean Interfaces**: Follows the gateway's interface-driven design

## Architecture

### Core Components

#### 1. SessionAffinityConfig (core/interfaces.go)
```go
type SessionAffinityConfig struct {
    Enabled    bool
    TTL        time.Duration
    Source     SessionSource  // cookie, header, query
    CookieName string
    HeaderName string
    QueryParam string
}
```

#### 2. Session Extractors (session/extractor.go)
- `CookieExtractor`: Extracts session ID from HTTP cookies
- `HeaderExtractor`: Extracts session ID from request headers
- `QueryExtractor`: Extracts session ID from URL query parameters

#### 3. RequestAwareLoadBalancer Interface
```go
type RequestAwareLoadBalancer interface {
    LoadBalancer
    SelectForRequest(Request, []ServiceInstance) (*ServiceInstance, error)
}
```

#### 4. StickySessionBalancer
- Implements `RequestAwareLoadBalancer`
- Uses configured extractor to get session ID
- Maintains session-to-instance mappings
- Falls back to round-robin for new sessions

### Data Flow

```
Request → Router → RequestAwareLoadBalancer → Session Extractor
                            ↓
                   Session Store (get/set)
                            ↓
                   Select Instance → Response
```

### Session Store

The in-memory session store maintains mappings between session IDs and instance IDs:

```go
type SessionStore interface {
    GetInstance(sessionID string) (string, bool)
    SetInstance(sessionID string, instanceID string, ttl time.Duration)
    RemoveInstance(sessionID string)
    Cleanup()
}
```

Features:
- TTL-based expiration
- Automatic cleanup of expired sessions
- Thread-safe operations
- Health-aware (removes unhealthy instances)

## Configuration

Session affinity is configured at the route level:

```yaml
router:
  rules:
    - id: stateful-route
      path: /api/*
      serviceName: my-service
      loadBalance: sticky_session
      sessionAffinity:
        enabled: true
        ttl: 3600
        source: cookie
        cookieName: SESSION_ID
```

## Protocol-Specific Considerations

### HTTP
- Cookies are the most common choice
- Headers work well for API clients
- Query parameters for simple cases

### WebSocket
- Initial HTTP upgrade request carries session info
- Cookie support depends on client implementation
- Headers are reliable for custom clients

### SSE
- Initial HTTP request establishes session
- Headers recommended (cookies may not work after connection)
- Query parameters as fallback

### gRPC
- Metadata (headers) are the standard approach
- No cookie support in gRPC protocol

## Implementation Details

### Session ID Extraction Process

1. Router determines if route uses sticky sessions
2. Creates appropriate extractor based on configuration
3. Extractor parses request to find session ID
4. Returns empty string if no session found

### Instance Selection Algorithm

1. Extract session ID from request
2. If no session ID, use fallback balancer
3. Check session store for existing mapping
4. If mapping exists and instance is healthy, use it
5. If not, select new instance and store mapping

### Health Monitoring

- Unhealthy instances are automatically removed from sticky sessions
- Sessions are redistributed to healthy instances
- No manual intervention required

## Best Practices

1. **Choose Appropriate TTL**: Balance between session duration and memory usage
2. **Use Consistent Session Sources**: Don't mix sources for the same service
3. **Monitor Session Distribution**: Ensure even load distribution
4. **Plan for Failures**: Sessions will be redistributed on instance failure

## Future Enhancements

1. **Distributed Session Store**: Redis/Etcd for multi-gateway deployments
2. **Session Migration**: Graceful session transfer during scaling
3. **Custom Extractors**: Plugin system for application-specific session sources
4. **Session Persistence**: Survive gateway restarts