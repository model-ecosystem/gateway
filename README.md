# Gateway

A high-performance, multi-protocol API gateway written in Go with clean architecture, supporting HTTP/WebSocket/SSE/gRPC protocols, authentication, rate limiting, and service discovery.

## 🚀 Features

### Core Features
- **HTTP Gateway**: High-performance reverse proxy with streaming responses
- **Load Balancing**: Round-robin with health checking
- **Static Service Discovery**: Configuration-based service registry
- **Structured Error Handling**: Type-safe errors with proper HTTP status mapping
- **Per-Route Configuration**: Timeouts, load balancing strategies
- **Clean Architecture**: Layered design with clear separation of concerns

### Protocol Support
- **Multi-Protocol Frontend**: HTTP/HTTPS, WebSocket, Server-Sent Events (SSE)
- **Multi-Protocol Backend**: HTTP, WebSocket, SSE, gRPC
- **Protocol Conversion**: HTTP to gRPC transcoding (basic)
- **Stateful Connections**: WebSocket and SSE with connection management
- **TLS/mTLS**: Full encryption support for frontend and backend

### Security & Authentication
- **JWT Authentication**: RS256/HS256, JWKS endpoint support
- **API Key Authentication**: SHA256 hashing, flexible extraction
- **Rate Limiting**: Token bucket algorithm, per-route configuration
- **CORS Support**: Configurable cross-origin resource sharing

### Service Discovery
- **Static Configuration**: YAML-based service definitions
- **Docker Discovery**: Automatic discovery via container labels
- **Health Checking**: Active and passive health monitoring

## Quick Start

```bash
# Build the gateway
make build

# Build and run
make run

# Run without building
make dev

# Run tests
make test

# Clean build artifacts
make clean
```

## Examples

See the `/examples` directory for complete working examples:
- `basic/` - Simple HTTP routing example
- `websocket-chat/` - Real-time chat using WebSocket
- `sse-dashboard/` - Live dashboard using Server-Sent Events
- `microservices/` - Complete microservices setup with authentication

## Configuration

Configuration is stored in YAML files. Basic example (`configs/base/gateway.yaml`):

```yaml
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
      readTimeout: 30
      writeTimeout: 30

  registry:
    type: static
    static:
      services:
        - name: example-service
          instances:
            - id: example-1
              address: "127.0.0.1"
              port: 3000
              health: healthy

  router:
    rules:
      - id: example-route
        path: /api/example/*
        serviceName: example-service
        loadBalance: round_robin
        timeout: 10  # Per-route timeout in seconds
```

## 🏗️ Architecture

The gateway follows a clean three-layer architecture with idiomatic Go patterns.

### Core Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Frontend Layer                         │
│  ┌─────────┐  ┌───────────┐  ┌─────┐  ┌──────────────────┐ │
│  │  HTTP   │  │ WebSocket │  │ SSE │  │ Protocol Adapter │ │
│  └────┬────┘  └─────┬─────┘  └──┬──┘  └────────┬─────────┘ │
└───────┼─────────────┼───────────┼──────────────┼───────────┘
        │             │           │              │
┌───────▼─────────────▼───────────▼──────────────▼───────────┐
│                        Middle Layer                           │
│  ┌────────────┐  ┌──────────┐  ┌─────────┐  ┌───────────┐  │
│  │ Middleware │  │  Router  │  │ Registry│  │ Load      │  │
│  │ Chain      │  │          │  │         │  │ Balancer  │  │
│  └────────────┘  └──────────┘  └─────────┘  └───────────┘  │
└──────────────────────────────────────────────────────────────┘
        │             │           │              │
┌───────▼─────────────▼───────────▼──────────────▼───────────┐
│                        Backend Layer                          │
│  ┌─────────┐  ┌───────────┐  ┌─────┐  ┌──────┐            │
│  │  HTTP   │  │ WebSocket │  │ SSE │  │ gRPC │            │
│  └─────────┘  └───────────┘  └─────┘  └──────┘            │
└──────────────────────────────────────────────────────────────┘
```

### Key Design Principles

- **Interface-Driven**: Small, focused interfaces for extensibility
- **Function Types**: Handlers and middleware as function types
- **Streaming First**: No buffering, direct streaming from backend to client
- **Error as Values**: Structured errors with context and proper HTTP mapping
- **Standard Library**: Minimal dependencies, built on net/http
- **Graceful Degradation**: Fallback mechanisms for service failures

## 📁 Project Structure

```
gateway/
├── cmd/gateway/              # Main application entry point
├── internal/                 # Private application code
│   ├── adapter/              # Protocol adapters (Frontend)
│   │   ├── http/            # HTTP/HTTPS adapter
│   │   ├── websocket/       # WebSocket adapter
│   │   └── sse/             # Server-Sent Events adapter
│   ├── connector/            # Backend connectors
│   │   ├── http/            # HTTP backend connector
│   │   ├── websocket/       # WebSocket backend connector
│   │   ├── sse/             # SSE backend connector
│   │   └── grpc/            # gRPC backend connector
│   ├── middleware/           # Middleware implementations
│   │   ├── auth/            # Authentication (JWT, API Key)
│   │   ├── ratelimit/       # Rate limiting
│   │   └── recovery/        # Panic recovery
│   ├── router/              # Request routing and load balancing
│   ├── registry/            # Service discovery
│   │   ├── static/          # Static configuration
│   │   └── docker/          # Docker-based discovery
│   ├── app/                 # Application setup and factories
│   ├── config/              # Configuration management
│   └── core/                # Core interfaces and types
├── pkg/                     # Public packages
│   ├── errors/              # Error types and handling
│   └── tls/                 # TLS utilities
├── configs/                 # Configuration files
│   ├── base/                # Base configurations
│   ├── dev/                 # Development configs
│   └── examples/            # Example configurations
├── test/                    # Test suites
│   ├── integration/         # Integration tests
│   ├── e2e/                 # End-to-end tests
│   └── mock/                # Mock servers and data
├── examples/                # Example applications
├── deployments/             # Deployment configurations
├── scripts/                 # Build and development scripts
└── scripts/                 # Build and development scripts
```

## 🛡️ Error Handling

The gateway uses structured errors that automatically map to HTTP status codes:

| Error Type | HTTP Status | Description |
|------------|-------------|-------------|
| `ErrorTypeNotFound` | 404 | Route or service not found |
| `ErrorTypeUnavailable` | 503 | No healthy backend instances |
| `ErrorTypeTimeout` | 408 | Request timeout exceeded |
| `ErrorTypeBadRequest` | 400 | Invalid request format |
| `ErrorTypeUnauthorized` | 401 | Authentication required |
| `ErrorTypeForbidden` | 403 | Insufficient permissions |
| `ErrorTypeRateLimit` | 429 | Rate limit exceeded |
| `ErrorTypeInternal` | 500 | Internal server error |

All errors include contextual information for debugging and are properly logged with trace IDs.

## Testing

```bash
# Run all tests
./scripts/test/run-tests.sh

# Run specific test suites
go test ./internal/...
go test ./test/integration/...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./internal/router/...
```

### Test Environment

```bash
# Start test environment with mock servers
cd test/mock
docker-compose up

# Run integration tests against test environment
go test ./test/integration/...
```

## Requirements

- Go 1.24 or later
- Make (for build commands)

## Dependencies

- `gopkg.in/yaml.v3` - YAML configuration parsing
- `github.com/gorilla/websocket` - WebSocket support
- `github.com/docker/docker` - Docker service discovery
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- Standard library for core functionality

## Configuration Examples

See `/configs/examples/` for working configuration examples:
- Basic HTTP routing
- Authentication (JWT, API Key)
- TLS/mTLS setup
- WebSocket configuration
- SSE configuration
- Docker discovery
- gRPC backend and transcoding
- Session affinity

## License

This project is released under the [MIT License](LICENSE).
