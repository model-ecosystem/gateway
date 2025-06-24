# Request Flow and Sequence Diagrams

This document illustrates how different types of requests flow through the gateway with detailed sequence diagrams.

## 📋 Table of Contents

1. [HTTP Request Flow](#http-request-flow)
2. [Authenticated Request Flow](#authenticated-request-flow)
3. [WebSocket Connection Flow](#websocket-connection-flow)
4. [SSE Stream Flow](#sse-stream-flow)
5. [Rate Limited Request Flow](#rate-limited-request-flow)
6. [Service Discovery Flow](#service-discovery-flow)
7. [Error Handling Flow](#error-handling-flow)

## HTTP Request Flow

Basic HTTP request processing through the gateway:

```mermaid
sequenceDiagram
    participant Client
    participant HTTPAdapter as HTTP Adapter
    participant Middleware as Middleware Chain
    participant Router
    participant Registry as Service Registry
    participant LB as Load Balancer
    participant HTTPConnector as HTTP Connector
    participant Backend as Backend Service

    Client->>HTTPAdapter: HTTP Request
    Note over HTTPAdapter: TLS termination<br/>Parse HTTP request
    
    HTTPAdapter->>HTTPAdapter: Convert to core.Request
    HTTPAdapter->>Middleware: Handle(ctx, request)
    
    Note over Middleware: Recovery → Logging → Auth → RateLimit
    
    Middleware->>Router: Route(request)
    Router->>Router: Match path pattern
    Router->>Registry: GetService(serviceName)
    Registry-->>Router: Service instances
    
    Router->>LB: SelectInstance(instances)
    LB-->>Router: Selected instance
    
    Router->>HTTPConnector: Forward(request, instance)
    HTTPConnector->>HTTPConnector: Get connection from pool
    HTTPConnector->>Backend: HTTP Request
    Backend-->>HTTPConnector: HTTP Response
    
    HTTPConnector-->>Router: core.Response
    Router-->>Middleware: core.Response
    Middleware-->>HTTPAdapter: core.Response
    
    HTTPAdapter->>HTTPAdapter: Convert to HTTP response
    HTTPAdapter-->>Client: HTTP Response
```

### Key Steps Explained

1. **TLS Termination**: The HTTP adapter handles TLS, converting HTTPS to internal HTTP
2. **Request Conversion**: HTTP-specific details are abstracted into `core.Request`
3. **Middleware Pipeline**: Each middleware can modify the request or short-circuit
4. **Route Matching**: The router uses pattern matching (e.g., `/api/*`) to find routes
5. **Service Discovery**: The registry provides healthy instances for the service
6. **Load Balancing**: An instance is selected based on the configured algorithm
7. **Connection Pooling**: The connector reuses connections for efficiency
8. **Response Streaming**: Responses are streamed back without buffering

## Authenticated Request Flow

Request flow with JWT authentication:

```mermaid
sequenceDiagram
    participant Client
    participant HTTPAdapter
    participant AuthMiddleware as Auth Middleware
    participant JWTProvider as JWT Provider
    participant Router
    participant Backend

    Client->>HTTPAdapter: Request + JWT Token
    HTTPAdapter->>AuthMiddleware: Handle(ctx, request)
    
    AuthMiddleware->>AuthMiddleware: Extract token from header
    AuthMiddleware->>JWTProvider: Validate(token)
    
    alt Token is valid
        JWTProvider->>JWTProvider: Verify signature
        JWTProvider->>JWTProvider: Check expiration
        JWTProvider->>JWTProvider: Extract claims
        JWTProvider-->>AuthMiddleware: AuthResult{user, permissions}
        
        AuthMiddleware->>AuthMiddleware: Add auth info to context
        AuthMiddleware->>Router: Handle(authCtx, request)
        Router->>Backend: Authenticated request
        Backend-->>Router: Response
        Router-->>AuthMiddleware: Response
        AuthMiddleware-->>HTTPAdapter: Response
        HTTPAdapter-->>Client: 200 OK + Response
    else Token is invalid
        JWTProvider-->>AuthMiddleware: Error: Invalid token
        AuthMiddleware-->>HTTPAdapter: 401 Unauthorized
        HTTPAdapter-->>Client: 401 Unauthorized
    end
```

### Authentication Details

1. **Token Extraction**: Supports Bearer tokens, custom headers, or cookies
2. **JWT Validation**: Signature verification using RS256/HS256
3. **JWKS Support**: Automatic key rotation via JWKS endpoint
4. **Claims Mapping**: JWT claims are mapped to gateway permissions
5. **Context Enrichment**: Auth info is added to request context

## WebSocket Connection Flow

WebSocket upgrade and bidirectional communication:

```mermaid
sequenceDiagram
    participant Client
    participant HTTPAdapter
    participant WSAdapter as WebSocket Adapter
    participant Router
    participant WSConnector as WebSocket Connector
    participant Backend

    Client->>HTTPAdapter: GET /ws<br/>Upgrade: websocket
    HTTPAdapter->>HTTPAdapter: Validate upgrade headers
    HTTPAdapter->>WSAdapter: HandleUpgrade(w, r)
    
    WSAdapter->>Client: 101 Switching Protocols
    Note over Client,WSAdapter: WebSocket connection established
    
    WSAdapter->>Router: Route WebSocket request
    Router->>WSConnector: Connect(backend)
    WSConnector->>Backend: WebSocket handshake
    Backend-->>WSConnector: 101 Switching Protocols
    
    Note over WSConnector,Backend: Backend connection established
    
    par Client to Backend
        Client->>WSAdapter: WebSocket message
        WSAdapter->>WSConnector: Forward message
        WSConnector->>Backend: WebSocket message
    and Backend to Client
        Backend->>WSConnector: WebSocket message
        WSConnector->>WSAdapter: Forward message
        WSAdapter->>Client: WebSocket message
    end
    
    Client->>WSAdapter: Close connection
    WSAdapter->>WSConnector: Close backend
    WSConnector->>Backend: Close connection
```

### WebSocket Features

1. **Protocol Upgrade**: Standard HTTP to WebSocket upgrade
2. **Sticky Sessions**: Maintains connection to same backend instance
3. **Bidirectional Proxy**: Messages flow in both directions simultaneously
4. **Connection Management**: Proper cleanup on disconnect
5. **Message Types**: Supports text, binary, ping/pong frames

## SSE Stream Flow

Server-Sent Events streaming:

```mermaid
sequenceDiagram
    participant Client
    participant HTTPAdapter
    participant SSEAdapter as SSE Adapter
    participant Router
    participant SSEConnector as SSE Connector
    participant Backend

    Client->>HTTPAdapter: GET /events<br/>Accept: text/event-stream
    HTTPAdapter->>SSEAdapter: HandleSSE(w, r)
    
    SSEAdapter->>SSEAdapter: Set SSE headers
    SSEAdapter->>Client: 200 OK<br/>Content-Type: text/event-stream
    
    SSEAdapter->>Router: Route SSE request
    Router->>SSEConnector: Connect(backend)
    SSEConnector->>Backend: GET /events<br/>Accept: text/event-stream
    Backend-->>SSEConnector: 200 OK + Event stream
    
    loop Event Streaming
        Backend->>SSEConnector: Event data
        SSEConnector->>SSEConnector: Parse event
        SSEConnector->>SSEAdapter: Forward event
        SSEAdapter->>Client: data: {event}\n\n
    end
    
    Note over SSEAdapter: Periodic keepalive
    loop Keepalive
        SSEAdapter->>Client: :keepalive\n\n
    end
    
    Client->>HTTPAdapter: Close connection
    HTTPAdapter->>SSEConnector: Close stream
```

### SSE Features

1. **Auto-Detection**: Detects SSE requests by Accept header
2. **Event Parsing**: Handles multi-line events and event types
3. **Keepalive**: Prevents proxy timeouts with periodic comments
4. **Buffering Control**: Disables buffering for real-time delivery
5. **Reconnection**: Client can reconnect with Last-Event-ID

## Rate Limited Request Flow

Request handling with rate limiting:

```mermaid
sequenceDiagram
    participant Client
    participant HTTPAdapter
    participant RateLimitMiddleware as RateLimit Middleware
    participant TokenBucket as Token Bucket
    participant Router
    participant Backend

    Client->>HTTPAdapter: Request
    HTTPAdapter->>RateLimitMiddleware: Handle(ctx, request)
    
    RateLimitMiddleware->>RateLimitMiddleware: Extract key (IP/User/Custom)
    RateLimitMiddleware->>TokenBucket: GetBucket(key)
    TokenBucket-->>RateLimitMiddleware: Bucket
    
    RateLimitMiddleware->>TokenBucket: TryConsume(1)
    
    alt Tokens available
        TokenBucket-->>RateLimitMiddleware: true
        RateLimitMiddleware->>Router: Handle(ctx, request)
        Router->>Backend: Forward request
        Backend-->>Router: Response
        Router-->>RateLimitMiddleware: Response
        RateLimitMiddleware-->>HTTPAdapter: Response
        HTTPAdapter-->>Client: 200 OK
    else Rate limit exceeded
        TokenBucket-->>RateLimitMiddleware: false
        RateLimitMiddleware->>RateLimitMiddleware: Calculate retry after
        RateLimitMiddleware-->>HTTPAdapter: 429 Too Many Requests
        HTTPAdapter-->>Client: 429 + Retry-After header
    end
```

### Rate Limiting Details

1. **Token Bucket Algorithm**: Allows burst traffic while maintaining average rate
2. **Multiple Keys**: Can limit by IP, authenticated user, or custom key
3. **Per-Route Config**: Different limits for different endpoints
4. **Retry-After**: Tells clients when to retry
5. **Automatic Cleanup**: Removes inactive buckets to save memory

## Service Discovery Flow

How services are discovered and registered:

```mermaid
sequenceDiagram
    participant Router
    participant Registry
    participant DockerAPI as Docker API
    participant Cache as Instance Cache

    Note over Registry: Startup/Refresh cycle
    
    Registry->>DockerAPI: List containers
    DockerAPI-->>Registry: Container list
    
    loop For each container
        Registry->>Registry: Check labels<br/>(gateway.enable=true)
        
        alt Service enabled
            Registry->>Registry: Extract service info
            Registry->>DockerAPI: Inspect container
            DockerAPI-->>Registry: Container details
            Registry->>Registry: Build instance info
            Registry->>Cache: Store instance
        end
    end
    
    Router->>Registry: GetService("api-service")
    Registry->>Cache: Lookup instances
    Cache-->>Registry: Healthy instances
    Registry-->>Router: [instance1, instance2, ...]
    
    Note over Registry: Background refresh
    loop Every 30 seconds
        Registry->>DockerAPI: List containers
        Registry->>Registry: Update cache
    end
```

### Discovery Features

1. **Label-Based**: Services opt-in via Docker labels
2. **Health Checking**: Only returns healthy instances
3. **Automatic Refresh**: Discovers new instances automatically
4. **Multiple Networks**: Handles containers in different networks
5. **Metadata Support**: Additional config via labels

## Error Handling Flow

How errors are handled and propagated:

```mermaid
sequenceDiagram
    participant Client
    participant HTTPAdapter
    participant Middleware
    participant Router
    participant Connector
    participant Backend

    Client->>HTTPAdapter: Request
    HTTPAdapter->>Middleware: Handle(ctx, request)
    Middleware->>Router: Route(request)
    Router->>Connector: Forward(request)
    
    alt Backend Error
        Connector->>Backend: Request
        Backend-->>Connector: Connection refused
        Connector->>Connector: Wrap error with context
        Connector-->>Router: ErrorTypeUnavailable
        Router-->>Middleware: Error
        Middleware->>Middleware: Log error with context
        Middleware-->>HTTPAdapter: Error
        HTTPAdapter->>HTTPAdapter: Map to HTTP status
        HTTPAdapter-->>Client: 503 Service Unavailable
    else Timeout
        Connector->>Backend: Request
        Note over Connector,Backend: Timeout exceeded
        Connector-->>Router: ErrorTypeTimeout
        Router-->>HTTPAdapter: Error
        HTTPAdapter-->>Client: 408 Request Timeout
    else Route Not Found
        Router->>Router: No matching route
        Router-->>HTTPAdapter: ErrorTypeNotFound
        HTTPAdapter-->>Client: 404 Not Found
    end
```

### Error Handling Principles

1. **Structured Errors**: Type-safe errors with context
2. **Error Wrapping**: Original errors preserved with additional context
3. **HTTP Mapping**: Error types automatically map to HTTP status codes
4. **Logging**: All errors logged with trace IDs
5. **Client-Friendly**: Safe error messages sent to clients

## Performance Optimizations

### Connection Pooling

```
┌─────────────┐     Connection Pool      ┌─────────────┐
│   Gateway   │ ←───────────────────────→ │   Backend   │
└─────────────┘   - Max connections: 100  └─────────────┘
                  - Idle timeout: 90s
                  - Keep-alive: enabled
```

### Request Pipelining

```
Client ──→ Gateway ──→ Backend
  ↑          ↓  ↑         ↓
  └──────────┘  └─────────┘
   Streaming    Streaming
   Response     Response
```

### Zero-Copy Transfer

```go
// Direct streaming without buffering
io.Copy(clientWriter, backendReader)
// Data flows: Backend → Kernel → Gateway → Kernel → Client
// No user-space buffering
```

## Monitoring Points

Key points where metrics and logs are generated:

1. **Request Start**: Request ID generated, timer started
2. **Auth Decision**: Success/failure logged with user info
3. **Route Match**: Route and service logged
4. **Backend Selection**: Instance and pool stats
5. **Backend Response**: Status code and latency
6. **Request Complete**: Total latency and bytes transferred

## Summary

The gateway's request flow is designed for:

- **Performance**: Streaming, connection pooling, zero-copy
- **Reliability**: Error handling, timeouts, circuit breaking (future)
- **Security**: Authentication, rate limiting, TLS
- **Observability**: Structured logging, metrics, tracing (future)
- **Flexibility**: Multiple protocols, pluggable components