# Gateway

A simple HTTP gateway written in idiomatic Go. The gateway provides HTTP reverse proxy functionality with static service discovery and round-robin load balancing.

## Features

- HTTP reverse proxy with streaming responses (no buffering)
- Static service discovery
- Round-robin load balancing
- Path-based routing with per-route timeout control
- Structured error handling with proper HTTP status codes
- Structured logging with slog
- Graceful shutdown
- Clean layered architecture

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

## Configuration

Configuration is stored in `configs/gateway.yaml`:

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

## Architecture

The gateway follows a clean layered architecture with idiomatic Go patterns.

### Layers

- **Frontend Layer**: HTTP adapter that receives requests
- **Routing Layer**: Routes requests to backend services
- **Backend Layer**: Forwards requests to backend instances
- **Middleware Layer**: Cross-cutting concerns like logging and recovery

### Key Patterns

- Function types for handlers and middleware
- Small, focused interfaces
- Streaming responses (no buffering)
- Structured error handling with proper HTTP status codes
- Per-route timeout control
- Standard library first approach (net/http)

## Project Structure

```
gateway/
├── cmd/gateway/        # Main program
├── configs/            # Configuration files
├── internal/           # Private packages
│   ├── backend/        # Backend connectors
│   ├── config/         # Config loading
│   ├── core/           # Core types and interfaces
│   ├── frontend/http/  # HTTP server with error handling
│   ├── middleware/     # Logging and recovery middleware
│   ├── registry/       # Service discovery
│   └── router/         # Routing with load balancing
└── pkg/                # Public packages
    └── errors/         # Structured error types
```

## Error Handling

Structured errors automatically map to HTTP status codes:

- `ErrorTypeNotFound` → 404 (route or service not found)
- `ErrorTypeUnavailable` → 503 (no healthy instances)
- `ErrorTypeTimeout` → 408 (request timeout)
- `ErrorTypeBadRequest` → 400 (invalid request)

All errors include contextual details for debugging in logs.

## Testing

Use the provided scripts for testing:

```bash
# Start demo environment with test servers
./scripts/start-demo.sh

# Run gateway tests
./scripts/test-gateway.sh
```

### Manual Testing

```bash
# Start test servers
go run test/test-server.go -port 3000 &
go run test/test-server.go -port 3001 &

# Test the gateway
curl http://localhost:8080/api/example/test
```

## Requirements

- Go 1.24 or later
- Make (for build commands)

## Dependencies

- `gopkg.in/yaml.v3` - YAML configuration parsing
- Standard library only for all other functionality

## License

This project is released under the [MIT License](LICENSE).
