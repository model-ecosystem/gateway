# Multi-Version API Support

The gateway provides comprehensive API versioning support, allowing you to run multiple API versions simultaneously and manage version transitions smoothly.

## Features

- **Multiple versioning strategies**: Path, header, query parameter, or Accept header
- **Version-based routing**: Route requests to version-specific backend services
- **Deprecation notices**: Communicate version deprecation to clients
- **Default version**: Fallback when no version is specified
- **Per-version configuration**: Different settings for each API version

## Versioning Strategies

### 1. Path-based Versioning

Extract version from URL path (e.g., `/v2/users`):

```yaml
versioning:
  enabled: true
  strategy: path
  defaultVersion: "1.0"
```

### 2. Header-based Versioning

Use a custom header to specify version:

```yaml
versioning:
  enabled: true
  strategy: header
  versionHeader: "X-API-Version"
  defaultVersion: "1.0"
```

### 3. Query Parameter Versioning

Extract version from query parameter:

```yaml
versioning:
  enabled: true
  strategy: query
  versionQuery: "version"
  defaultVersion: "1.0"
```

### 4. Accept Header Versioning

Extract version from Accept header:

```yaml
versioning:
  enabled: true
  strategy: accept
  acceptPattern: 'version=(\d+(?:\.\d+)?)'
  defaultVersion: "1.0"
```

## Version Mappings

Route different versions to different backend services:

```yaml
versionMappings:
  "1.0":
    service: "api-v1"     # Route to v1 service
  "2.0":
    service: "api-v2"     # Route to v2 service
    pathPrefix: "/api"    # Optional: add path prefix
  "3.0":
    service: "api-v3"     # Route to v3 service
```

## Deprecation Handling

Communicate version deprecation to clients:

```yaml
deprecatedVersions:
  "1.0":
    message: "Version 1.0 is deprecated and will be removed on 2025-06-01"
    sunsetDate: "2025-06-01T00:00:00Z"
    removalDate: "2025-08-01T00:00:00Z"
```

Deprecated versions receive these response headers:
- `X-API-Deprecated: true`
- `X-API-Deprecation-Message: <message>`
- `Sunset: <RFC1123 date>`

## Examples

### Basic Multi-Version Setup

See `configs/examples/multi-version-simple.yaml`:

```yaml
gateway:
  versioning:
    enabled: true
    strategy: path
    defaultVersion: "1.0"
    versionMappings:
      "1.0":
        service: "api-v1"
      "2.0":
        service: "api-v2"
```

### gRPC Multi-Version Support

See `configs/examples/grpc-multi-version.yaml`:

```yaml
gateway:
  versioning:
    enabled: true
    strategy: header
    versionHeader: "x-api-version"
    versionMappings:
      "1.0":
        service: "grpc-service-v1"
      "2.0":
        service: "grpc-service-v2"
```

### OpenAPI Multi-Version Support

See `configs/examples/openapi-multi-version.yaml`:

```yaml
gateway:
  versioning:
    enabled: true
    strategy: accept
    acceptPattern: 'version=(\d+(?:\.\d+)?)'
  openapi:
    specs:
      - name: "api-v1"
        path: "/specs/openapi-v1.yaml"
        version: "1.0"
      - name: "api-v2"
        path: "/specs/openapi-v2.yaml"
        version: "2.0"
```

## Client Usage Examples

### Path-based
```bash
curl http://localhost:8080/v2/users
```

### Header-based
```bash
curl -H "X-API-Version: 2.0" http://localhost:8080/users
```

### Query-based
```bash
curl http://localhost:8080/users?version=2.0
```

### Accept header
```bash
curl -H "Accept: application/json;version=2.0" http://localhost:8080/users
```

## Best Practices

1. **Use semantic versioning**: Follow major.minor format (e.g., 2.0, 2.1)
2. **Provide deprecation notices**: Give clients time to migrate
3. **Document version differences**: Clearly document changes between versions
4. **Test version transitions**: Ensure smooth migration paths
5. **Monitor version usage**: Track which versions clients are using

## Benefits

- **Backward compatibility**: Support old clients while adding new features
- **Gradual migration**: Clients can migrate at their own pace
- **A/B testing**: Test new versions with subset of traffic
- **Breaking changes**: Introduce breaking changes safely in new versions
- **Clear communication**: Deprecation headers inform clients about version lifecycle