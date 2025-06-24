# gRPC Support Guide

This guide covers how to use the gateway's gRPC backend support and HTTP to gRPC transcoding capabilities.

## Overview

The gateway provides comprehensive gRPC features:

1. **Native gRPC Backend Support**: Connect to gRPC services as backend targets
2. **HTTP to gRPC Transcoding**: Accept HTTP/JSON requests and convert them to gRPC calls
3. **Dynamic Descriptor Loading**: Load and reload Protocol Buffer definitions at runtime

## Features

- gRPC client connection pooling with keepalive
- TLS/mTLS support for secure gRPC connections
- Full HTTP to gRPC transcoding with protobuf support
- Proto descriptor loading (file, base64, or dynamic)
- Automatic JSON to Protobuf conversion
- gRPC error mapping to HTTP status codes
- Configurable connection parameters
- Hot-reloading of proto descriptors

### Current Limitations

- Streaming RPCs are not yet supported
- Google API HTTP annotations are not yet implemented
- Custom field mappings are not available

## Basic Configuration

### gRPC Backend

```yaml
gateway:
  backend:
    grpc:
      maxConcurrentStreams: 100
      keepAliveTime: 30s
      keepAliveTimeout: 10s
      tls: false
  
  registry:
    type: static
    static:
      services:
        - name: grpc-service
          type: grpc  # Specify gRPC service type
          instances:
            - id: grpc-1
              address: "localhost"
              port: 50051
              health: healthy
  
  router:
    rules:
      - id: grpc-route
        path: /api/grpc/*
        serviceName: grpc-service
        backend: grpc  # Use gRPC connector
```

### gRPC with TLS

```yaml
gateway:
  backend:
    grpc:
      tls: true
      # TLS config is loaded from gateway.tls settings
```

### Advanced Connection Settings

```yaml
gateway:
  backend:
    grpc:
      maxConcurrentStreams: 1000
      initialConnWindowSize: 65536     # 64KB
      initialWindowSize: 65536         # 64KB
      keepAliveTime: 30s
      keepAliveTimeout: 10s
      maxRetryAttempts: 3
      retryTimeout: 5s
```

## HTTP to gRPC Transcoding

### Basic Transcoding

Enable transcoding to convert HTTP/JSON requests to gRPC:

```yaml
gateway:
  router:
    rules:
      - id: grpc-route
        path: /api/grpc/*
        serviceName: grpc-service
        grpc:
          enableTranscoding: true
          service: "example.v1.ExampleService"
          # Option 1: Load from file
          protoDescriptor: "/path/to/service.pb"
          # Option 2: Embed as base64
          protoDescriptorBase64: "CgtzZXJ2aWNlLnByb3RvEg..."
```

### Dynamic Descriptor Loading

For runtime reloading of proto definitions:

```yaml
router:
  rules:
    - id: grpc-route
      path: /api/grpc/*
      serviceName: grpc-backend
      metadata:
        grpc:
          service: "example.v1.ExampleService"
          enableTranscoding: true
          dynamicDescriptors:
            files:
              - "/path/to/service.desc"
            directories:
              - "/path/to/descriptors/"
            autoReload: true
            reloadInterval: 60
```

#### Dynamic Loading Options

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `files` | `[]string` | List of `.desc` files to load | `[]` |
| `directories` | `[]string` | Directories to scan for `.desc` files | `[]` |
| `autoReload` | `bool` | Enable automatic reloading | `false` |
| `reloadInterval` | `int` | Reload check interval in seconds | `30` |

## Generating Proto Descriptors

### Basic Compilation

```bash
# Generate descriptor file
protoc --descriptor_set_out=service.pb \
       --include_imports \
       --include_source_info \
       service.proto

# Or for dynamic loading (.desc extension)
protoc --descriptor_set_out=service.desc \
       --include_imports \
       service.proto
```

### Multiple Proto Files

```bash
protoc --descriptor_set_out=all.desc --include_imports *.proto
```

### With Import Paths

```bash
protoc -I/path/to/imports \
       -I/another/import/path \
       --descriptor_set_out=service.desc \
       --include_imports \
       service.proto
```

### Convert to Base64

```bash
base64 service.pb
```

## Usage Examples

### Simple gRPC Path Routing

The transcoder expects gRPC paths in the format: `/package.Service/Method`

```bash
curl -X POST http://localhost:8080/api/grpc/example.v1.UserService/GetUser \
  -H "Content-Type: application/json" \
  -d '{"id": "123"}'
```

### RESTful API with gRPC Backend

Configure RESTful paths that map to gRPC methods:

```yaml
router:
  rules:
    - id: rest-api
      path: /api/v1/*
      serviceName: api-service
      backend: grpc
      transcoding:
        pathMappings:
          /api/v1/users: user.v1.UserService
          /api/v1/orders: order.v1.OrderService
        methodMappings:
          "GET /api/v1/users": ListUsers
          "GET /api/v1/users/{id}": GetUser
          "POST /api/v1/users": CreateUser
```

### gRPC with Load Balancing

```yaml
registry:
  static:
    services:
      - name: grpc-cluster
        type: grpc
        instances:
          - id: grpc-1
            address: "10.0.1.10"
            port: 50051
          - id: grpc-2
            address: "10.0.1.11"
            port: 50051
          - id: grpc-3
            address: "10.0.1.12"
            port: 50051

router:
  rules:
    - id: grpc-lb
      path: /grpc/*
      serviceName: grpc-cluster
      backend: grpc
      loadBalance: round_robin
```

### Development Setup with Hot-Reload

```yaml
metadata:
  grpc:
    enableTranscoding: true
    dynamicDescriptors:
      files:
        - "./proto/compiled/service.desc"
      directories:
        - "./proto/compiled/deps/"
      autoReload: true
      reloadInterval: 10  # Check every 10 seconds
```

### Multiple API Versions

Support multiple versions simultaneously:

```yaml
rules:
  - id: grpc-v1
    path: /api/v1/grpc/*
    metadata:
      grpc:
        dynamicDescriptors:
          files:
            - "/descriptors/service_v1.desc"
  
  - id: grpc-v2
    path: /api/v2/grpc/*
    metadata:
      grpc:
        dynamicDescriptors:
          files:
            - "/descriptors/service_v2.desc"
```

## Error Handling

gRPC status codes are automatically mapped to HTTP status codes:

| gRPC Status | HTTP Status | Description |
|-------------|-------------|-------------|
| OK | 200 | Success |
| NOT_FOUND | 404 | Resource not found |
| INVALID_ARGUMENT | 400 | Bad request |
| DEADLINE_EXCEEDED | 408 | Request timeout |
| UNAVAILABLE | 503 | Service unavailable |
| Others | 500 | Internal server error |

## Best Practices

### Development Environment

- Enable `autoReload` with a short interval (10-60 seconds)
- Use directory scanning for easy addition of new services
- Keep descriptor files in your project repository

```yaml
dynamicDescriptors:
  directories:
    - "./proto/descriptors/"
  autoReload: true
  reloadInterval: 30
```

### Production Environment

- Disable `autoReload` or use a longer interval
- Use explicit file lists for better control
- Consider using CI/CD to update descriptor files

```yaml
dynamicDescriptors:
  files:
    - "/etc/gateway/descriptors/user_service_v1.2.desc"
    - "/etc/gateway/descriptors/order_service_v2.0.desc"
  autoReload: false
```

### Descriptor Management

1. **Version Control**: Keep `.desc` files in version control alongside `.proto` files
2. **Build Process**: Generate `.desc` files as part of your build process
3. **Naming Convention**: Use versioned names like `service_v1.0.desc`
4. **Directory Structure**:
   ```
   descriptors/
   ├── services/
   │   ├── user_service.desc
   │   ├── order_service.desc
   │   └── payment_service.desc
   └── common/
       ├── shared_types.desc
       └── google/
           └── api/
               └── annotations.desc
   ```

## Performance Tuning

### Connection Pooling

The gRPC connector maintains persistent connections to backend services. Each unique target (address:port) gets its own connection that is reused across requests.

### Window Sizes

Adjust window sizes for high-throughput scenarios:

```yaml
backend:
  grpc:
    initialConnWindowSize: 1048576  # 1MB for connection
    initialWindowSize: 1048576      # 1MB per stream
```

### Keepalive Settings

Configure keepalive to detect dead connections:

```yaml
backend:
  grpc:
    keepAliveTime: 30s      # Send keepalive ping every 30s
    keepAliveTimeout: 10s   # Wait 10s for ping response
```

## Monitoring

Monitor gRPC connections through logs:

```
INFO created gRPC connection target=localhost:50051
DEBUG transcoded HTTP to gRPC service=users.UserService method=GetUser bodySize=25
ERROR failed to close gRPC connection target=localhost:50051 error="..."
```

Enable debug logging for detailed information:

```yaml
logging:
  level: debug
```

## Troubleshooting

### Connection Refused

If you see "connection refused" errors:
1. Verify the gRPC service is running
2. Check the address and port configuration
3. Ensure no firewall is blocking the connection

### TLS Handshake Failures

For TLS issues:
1. Verify server certificate is valid
2. Check certificate common name matches the server address
3. Ensure CA certificates are properly configured

### Transcoding Errors

If transcoding fails:
1. Verify the request path matches the expected format
2. Check JSON payload is valid
3. Ensure the gRPC method exists on the service

### Descriptor Loading Issues

1. **File Not Found**
   - Ensure the path is absolute or relative to the gateway's working directory
   - Check file permissions

2. **Invalid Descriptor Format**
   - Ensure the file was generated with `protoc`
   - Check that all imports are included with `--include_imports`

3. **Method Not Found**
   - Verify the service name matches the one in the `.proto` file
   - Check that the descriptor contains the expected service

4. **Auto-Reload Not Working**
   - Verify `autoReload` is set to `true`
   - Check file system permissions for watching files
   - Look for reload errors in logs

## Future Enhancements

The following features are planned for future releases:

1. **Streaming Support**
   - Server streaming RPCs
   - Client streaming RPCs
   - Bidirectional streaming

2. **Advanced Transcoding**
   - Google API HTTP annotations
   - Custom field mappings
   - Query parameter binding
   - RESTful resource mapping

3. **Service Discovery**
   - gRPC health checking protocol
   - DNS-based discovery
   - Consul/Etcd integration

4. **Enhanced Features**
   - Request/response transformations
   - Field-level validation
   - Custom error mapping
   - Metadata propagation

## See Also

- [Configuration Guide](configuration.md) - General gateway configuration
- [TLS Setup Guide](tls-setup.md) - Configuring TLS/mTLS
- [Authentication Guide](authentication.md) - Adding authentication to gRPC routes