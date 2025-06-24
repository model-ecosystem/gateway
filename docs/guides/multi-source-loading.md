# Multi-Source Descriptor Loading Guide

This guide covers how to load gRPC descriptors and OpenAPI specifications from multiple sources.

## Overview

The gateway supports loading descriptors and specifications from various sources:
- Local file system
- HTTP/HTTPS endpoints
- Kubernetes ConfigMaps

## Source Interface

The gateway uses a flexible source interface that can be extended:

```go
type Source interface {
    Load(ctx context.Context, path string) (io.ReadCloser, error)
    Type() string
}
```

## Supported Sources

### 1. File Source
Loads from local file system.

```yaml
descriptorSources:
  - type: file
    paths:
      - "/app/descriptors/service.desc"
      - "/etc/gateway/specs/api.yaml"
```

### 2. HTTP/HTTPS Source
Loads from remote HTTP endpoints.

```yaml
descriptorSources:
  - type: http
    urls:
      - "https://api.example.com/descriptors/service.desc"
      - "https://raw.githubusercontent.com/example/specs/main/api.yaml"
    headers:
      Authorization: "Bearer ${API_TOKEN}"
    timeout: 10
```

### 3. Kubernetes ConfigMap Source
Loads from Kubernetes ConfigMaps.

```yaml
descriptorSources:
  - type: k8s-configmap
    namespace: default
    configMaps:
      - name: grpc-descriptors
        keys:
          - "service.desc"
          - "common.desc"
```

## Configuration Examples

### gRPC Multi-Source Configuration

```yaml
gateway:
  protocols:
    grpc:
      enabled: true
      descriptorSources:
        # Load from local files
        - type: file
          paths:
            - "/app/descriptors/echo.desc"
            - "/app/descriptors/user.desc"
        
        # Load from HTTP endpoints
        - type: http
          urls:
            - "https://api.example.com/descriptors/payment.desc"
          headers:
            Authorization: "Bearer ${TOKEN}"
        
        # Load from Kubernetes ConfigMap
        - type: k8s-configmap
          namespace: default
          configMaps:
            - name: grpc-descriptors
              keys:
                - "notification.desc"
```

### OpenAPI Multi-Source Configuration

```yaml
gateway:
  protocols:
    openapi:
      enabled: true
      specSources:
        # Load from local files
        - type: file
          paths:
            - "/app/specs/user-api.yaml"
            - "/app/specs/product-api.json"
        
        # Load from HTTP endpoints
        - type: http
          urls:
            - "https://api.example.com/specs/payment-api.yaml"
        
        # Load from Kubernetes ConfigMap
        - type: k8s-configmap
          namespace: default
          configMaps:
            - name: api-specs
              keys:
                - "analytics-api.yaml"
```

## URI Format

You can also load directly using URI format:

```go
// Load from file
loader.LoadDescriptorFromURI("file:///path/to/descriptor.desc")

// Load from HTTP
loader.LoadDescriptorFromURI("https://example.com/descriptor.desc")

// Load from Kubernetes ConfigMap
loader.LoadDescriptorFromURI("k8s://configmap-name/descriptor-key")
```

## Auto-Reload Support

The gateway supports automatic reloading when descriptors change:

```yaml
autoReload: true
reloadInterval: 30s
```

Note: File watching is only supported for local files. Remote sources use interval-based polling.

## Extending with Custom Sources

To add a new source type:

1. Implement the `Source` interface:

```go
type MyCustomSource struct {
    // your fields
}

func (s *MyCustomSource) Load(ctx context.Context, path string) (io.ReadCloser, error) {
    // Load implementation
}

func (s *MyCustomSource) Type() string {
    return "my-custom"
}
```

2. Register with the source registry:

```go
registry := loader.NewSourceRegistry()
registry.Register("my-custom", NewMyCustomSource())
```

## Environment Variables

The configuration supports environment variable substitution:

```yaml
headers:
  Authorization: "Bearer ${DESCRIPTOR_API_TOKEN}"
  X-API-Key: "${API_KEY}"
```

## Error Handling

By default, the gateway continues loading if some sources fail. To fail fast:

```yaml
failOnError: true
```

## Caching

The gateway includes a caching layer for remote sources:

```go
cachedSource := loader.NewCachedSource(httpSource, 5*time.Minute)
```

## Security Considerations

1. **HTTPS**: Always use HTTPS for remote sources
2. **Authentication**: Use appropriate authentication headers
3. **ConfigMap Access**: Ensure proper RBAC permissions for ConfigMap access
4. **File Permissions**: Restrict file access appropriately

## Troubleshooting

1. **ConfigMap Not Found**: Check namespace and RBAC permissions
2. **HTTP Timeout**: Increase timeout value in configuration
3. **File Not Found**: Use absolute paths and verify file exists
4. **Authentication Failed**: Check token/credentials in environment variables