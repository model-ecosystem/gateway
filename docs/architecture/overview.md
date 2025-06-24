# Gateway Architecture Overview

## 🏗️ System Architecture

The gateway implements a clean three-layer architecture designed for extensibility, maintainability, and performance.

### High-Level Architecture

![Gateway Architecture](../../assets/images/architecture/gateway-overview.svg)

The gateway follows a container-based architecture with clear separation between:
- **Protocol Adapters**: Handle incoming client connections (HTTP/WebSocket/SSE)
- **Middleware Pipeline**: Process requests through authentication, rate limiting, and routing
- **Backend Connectors**: Proxy requests to upstream services
- **Control Plane**: Configuration and service registry management

## 🔧 Component Architecture

### Frontend Layer (Protocol Adapters)

Protocol adapters handle incoming client connections and convert protocol-specific requests to the gateway's internal format.

```
internal/adapter/
├── http/
│   ├── adapter.go      # HTTP server with TLS support
│   ├── config.go       # HTTP-specific configuration
│   ├── request.go      # HTTP request → core.Request
│   └── response.go     # core.Response → HTTP response
├── websocket/
│   ├── adapter.go      # WebSocket server
│   ├── conn.go         # Connection lifecycle management
│   ├── handler.go      # WebSocket message handling
│   └── response.go     # WebSocket-specific responses
└── sse/
    ├── adapter.go      # SSE handler (integrated with HTTP)
    ├── handler.go      # SSE event handling
    ├── reader.go       # Event stream parsing
    └── writer.go       # Event stream writing
```

**Key responsibilities:**
- Protocol handling (HTTP/1.1, HTTP/2, WebSocket, SSE)
- TLS termination
- Request/response transformation
- Connection management

### Middle Layer (Core Gateway)

The middle layer contains the business logic for routing, service discovery, and request processing.

```
internal/
├── router/
│   ├── router.go           # Route matching and dispatching
│   ├── loadbalancer.go     # Round-robin load balancing
│   └── sticky_balancer.go  # Session affinity
├── registry/
│   ├── static/            # Configuration-based discovery
│   └── docker/            # Docker container discovery
├── middleware/
│   ├── middleware.go      # Middleware chain implementation
│   ├── auth/             # Authentication middleware
│   ├── ratelimit/        # Rate limiting middleware
│   └── recovery.go       # Panic recovery
└── core/
    ├── interfaces.go     # Core abstractions
    └── types.go          # Shared types
```

**Key responsibilities:**
- Request routing based on path patterns
- Service discovery and health checking
- Load balancing across instances
- Middleware pipeline execution
- Cross-cutting concerns (auth, rate limiting)

### Backend Layer (Service Connectors)

Connectors handle communication with backend services, managing connections and protocol-specific details.

```
internal/connector/
├── http/
│   └── connector.go    # HTTP/HTTPS client with pooling
├── websocket/
│   └── connector.go    # WebSocket client with proxy
├── sse/
│   ├── connector.go    # SSE client
│   └── reader.go       # Event stream reader
└── grpc/
    ├── connector.go    # gRPC client
    └── transcoder.go   # HTTP→gRPC transcoding
```

**Key responsibilities:**
- Connection pooling and reuse
- Protocol-specific communication
- Error handling and retries
- Response streaming
- Protocol conversion (HTTP→gRPC)

## 🔄 Request Flow Architecture

![Request Flow](../../assets/images/architecture/request-flow.svg)

### HTTP Request Flow

The request flows through the gateway in a well-defined pipeline:

1. **Client Request**: HTTP request arrives at the Protocol Adapter
2. **Protocol Adaptation**: Convert to internal request format with TLS termination
3. **Middleware Pipeline**:
   - Recovery: Panic protection
   - Logging: Request/response logging
   - Authentication: JWT/API Key validation
   - Rate Limiting: Token bucket algorithm
4. **Routing**: Match route pattern and select service
5. **Load Balancing**: Choose healthy backend instance
6. **Backend Connection**: Proxy to backend service with connection pooling
7. **Response Streaming**: Stream response back to client

### WebSocket Connection Flow

```
WebSocket Upgrade Request
    │
    ▼
HTTP Adapter
    │ - Validate upgrade
    │
    ▼
WebSocket Adapter (internal/adapter/websocket/)
    │ - Complete handshake
    │ - Create connection
    │
    ▼
Router (sticky session)
    │ - Find target service
    │ - Maintain affinity
    │
    ▼
WebSocket Connector (internal/connector/websocket/)
    │ - Establish backend connection
    │ - Bidirectional proxy
    │
    ▼
Backend WebSocket Service
```

## 🏛️ Architectural Patterns

### 1. Interface-Driven Design

All major components are defined by interfaces, allowing for easy extension and testing:

```go
type Handler interface {
    Handle(ctx context.Context, req Request) (Response, error)
}

type Middleware func(Handler) Handler

type Registry interface {
    GetService(name string) (*Service, error)
    ListInstances(service string) ([]*Instance, error)
}
```

### 2. Middleware Chain Pattern

Middleware is implemented as a chain of handlers:

```go
// Middleware wraps handlers
func AuthMiddleware(next Handler) Handler {
    return HandlerFunc(func(ctx context.Context, req Request) (Response, error) {
        // Authentication logic
        if !authenticated {
            return errorResponse, nil
        }
        return next.Handle(ctx, req)
    })
}

// Chain combines multiple middleware
handler = Chain(
    RecoveryMiddleware,
    LoggingMiddleware,
    AuthMiddleware,
    RateLimitMiddleware,
)(router)
```

### 3. Factory Pattern

Complex objects are created through factories for better testability:

```go
// internal/app/factory/
├── auth.go      // Creates auth providers
├── backend.go   // Creates connectors
├── frontend.go  // Creates adapters
└── registry.go  // Creates service registries
```

### 4. Streaming Architecture

The gateway streams responses without buffering:

```go
// Direct streaming from backend to client
func (c *Connector) Forward(backend io.Reader, client io.Writer) error {
    _, err := io.Copy(client, backend)
    return err
}
```

### 5. Error as Values

Structured errors with context:

```go
type Error struct {
    Type    ErrorType
    Message string
    Cause   error
    Context map[string]interface{}
}

// Automatic HTTP status mapping
func (e *Error) HTTPStatus() int {
    switch e.Type {
    case ErrorTypeNotFound:
        return 404
    case ErrorTypeUnauthorized:
        return 401
    // ...
    }
}
```

## 🔐 Security Architecture

### Authentication Flow

```
Request with Credentials
    │
    ▼
Auth Middleware
    │
    ├─→ JWT Provider
    │   └─→ Verify signature
    │       └─→ Extract claims
    │
    └─→ API Key Provider
        └─→ Lookup key
            └─→ Validate
    │
    ▼
Authenticated Request Context
```

### TLS Architecture

```
Client ←──── TLS ────→ Gateway ←──── TLS ────→ Backend
         Frontend TLS           Backend TLS
         (Termination)         (Origination)
```

## 📊 Performance Architecture

### Connection Pooling

```go
// HTTP connection pool per backend
Transport: &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
}
```

### Zero-Copy Streaming

```go
// Direct streaming without buffering
io.Copy(clientWriter, backendReader)
```

### Efficient Routing

```go
// O(n) route matching with early termination
for _, route := range routes {
    if route.Matches(path) {
        return route
    }
}
```

## 🎯 Design Principles

1. **Simplicity First**: Use standard library where possible
2. **Interface Segregation**: Small, focused interfaces
3. **Explicit over Implicit**: Clear data flow, no magic
4. **Error Handling**: Errors are values with context
5. **Performance**: Stream, don't buffer
6. **Testability**: Dependency injection, interface-based
7. **Observability**: Structured logging, metrics, tracing

