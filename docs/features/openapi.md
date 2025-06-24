# OpenAPI Integration

The gateway provides comprehensive OpenAPI (Swagger) support for automatic API documentation, validation, and client SDK generation.

## Overview

OpenAPI integration enables:
- Automatic API documentation generation
- Request/response validation
- Mock response generation
- Client SDK generation
- API versioning support
- Schema-based routing
- Contract testing

## Configuration

### Basic Setup

```yaml
gateway:
  openapi:
    enabled: true
    spec:
      # Load from file
      file: /specs/api-v1.yaml
      
      # Or from URL
      url: https://api.example.com/openapi.yaml
      
      # Or inline
      inline: |
        openapi: 3.0.0
        info:
          title: My API
          version: 1.0.0
        paths:
          /users:
            get:
              summary: List users
```

### Multiple Specifications

```yaml
gateway:
  openapi:
    enabled: true
    specs:
      - name: users-api
        file: /specs/users-v1.yaml
        pathPrefix: /api/users
      
      - name: orders-api
        file: /specs/orders-v1.yaml
        pathPrefix: /api/orders
      
      - name: admin-api
        file: /specs/admin-v1.yaml
        pathPrefix: /admin
        auth: required
```

## Request Validation

### Enable Validation

```yaml
gateway:
  openapi:
    validation:
      request:
        enabled: true
        body: true
        query: true
        headers: true
        path: true
      
      response:
        enabled: true
        body: true
        headers: true
        status: true
```

### Validation Errors

When validation fails:

```json
{
  "error": "validation_failed",
  "message": "Request validation failed",
  "details": {
    "body": [
      {
        "field": "email",
        "error": "format",
        "message": "Invalid email format"
      }
    ],
    "query": [
      {
        "parameter": "limit",
        "error": "maximum",
        "message": "Value must be <= 100"
      }
    ]
  }
}
```

### Strict Mode

```yaml
gateway:
  openapi:
    validation:
      strictMode: true
      # Reject requests with:
      # - Extra body fields
      # - Unknown query parameters
      # - Missing required headers
```

## Response Validation

### Configuration

```yaml
gateway:
  openapi:
    validation:
      response:
        enabled: true
        # What to do on validation failure
        onError: "log"  # "log", "reject", "fix"
        
        # Fix invalid responses
        fix:
          removeExtraFields: true
          coerceTypes: true
          setDefaults: true
```

### Monitoring Invalid Responses

```yaml
gateway:
  openapi:
    validation:
      response:
        metrics: true
        # Exposes metrics:
        # gateway_openapi_response_validation_errors_total
        # gateway_openapi_response_validation_fixed_total
```

## Mock Responses

### Enable Mocking

```yaml
gateway:
  openapi:
    mocking:
      enabled: true
      # When to return mocks
      when:
        - serviceUnavailable  # Backend is down
        - headerPresent: "X-Mock-Response"
        - pathPrefix: "/mock"
```

### Mock Examples

OpenAPI spec with examples:

```yaml
paths:
  /users/{id}:
    get:
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
              examples:
                user1:
                  value:
                    id: 123
                    name: "John Doe"
                    email: "john@example.com"
```

### Dynamic Mocks

```yaml
gateway:
  openapi:
    mocking:
      dynamic:
        enabled: true
        # Generate realistic data
        faker: true
        # Respect schema constraints
        respectConstraints: true
```

## Schema-Based Routing

### Route by Operation ID

```yaml
gateway:
  openapi:
    routing:
      byOperationId: true
      
      # Maps operationId to backend service
      operations:
        getUsers: user-service
        createOrder: order-service
        processPayment: payment-service
```

### Route by Tags

```yaml
gateway:
  openapi:
    routing:
      byTags: true
      
      # Maps tags to services
      tags:
        users: user-service
        orders: order-service
        billing: billing-service
```

## API Documentation

### Built-in UI

```yaml
gateway:
  openapi:
    ui:
      enabled: true
      path: /api-docs
      
      # Swagger UI configuration
      swaggerUI:
        theme: "flattop"
        tryItOut: true
        displayRequestDuration: true
      
      # ReDoc alternative
      redoc:
        enabled: true
        path: /api-redoc
```

### Custom Branding

```yaml
gateway:
  openapi:
    ui:
      branding:
        title: "My API Gateway"
        logo: "/assets/logo.png"
        favicon: "/assets/favicon.ico"
        css: |
          .swagger-ui .topbar {
            background-color: #333;
          }
```

## Security Integration

### Authentication Schemes

```yaml
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
    
    apiKey:
      type: apiKey
      in: header
      name: X-API-Key
    
    oauth2:
      type: oauth2
      flows:
        authorizationCode:
          authorizationUrl: /oauth/authorize
          tokenUrl: /oauth/token
          scopes:
            read: Read access
            write: Write access
```

### Apply Security

```yaml
gateway:
  openapi:
    security:
      # Default security for all operations
      default:
        - bearerAuth: []
      
      # Override per path
      overrides:
        "/public/*":
          security: []  # No auth
        
        "/admin/*":
          security:
            - bearerAuth: []
            - apiKey: []
```

## Versioning Support

### Version in Path

```yaml
gateway:
  openapi:
    versioning:
      strategy: path
      versions:
        v1:
          spec: /specs/api-v1.yaml
          path: /v1/*
          deprecated: false
        
        v2:
          spec: /specs/api-v2.yaml
          path: /v2/*
          default: true
```

### Version Negotiation

```yaml
gateway:
  openapi:
    versioning:
      negotiation:
        header: Accept-Version
        query: version
        # Precedence: header > query > path
```

## Code Generation

### Client SDKs

```yaml
gateway:
  openapi:
    codegen:
      enabled: true
      clients:
        - language: typescript
          output: /generated/ts-client
          package: "@mycompany/api-client"
        
        - language: python
          output: /generated/py-client
          package: "mycompany-api"
        
        - language: go
          output: /generated/go-client
          package: "github.com/mycompany/api"
```

### Server Stubs

```yaml
gateway:
  openapi:
    codegen:
      servers:
        - language: go
          output: /generated/server
          # Generate handler interfaces
          interfaces: true
```

## Contract Testing

### Request Contract Tests

```yaml
gateway:
  openapi:
    testing:
      contracts:
        enabled: true
        # Validate actual traffic
        sampleRate: 0.1  # 10% of requests
        
        # Report violations
        reporting:
          webhook: https://monitoring.example.com/contract-violations
```

### Response Contract Tests

```yaml
gateway:
  openapi:
    testing:
      responses:
        # Validate all backend responses
        validateAll: true
        
        # Fail requests on contract violation
        failOnViolation: false
        
        # Log violations
        logViolations: true
```

## Advanced Features

### Schema Transformations

```yaml
gateway:
  openapi:
    transformations:
      # Remove internal fields from responses
      response:
        remove:
          - /properties/internal*
          - /properties/debug*
      
      # Add fields to requests
      request:
        add:
          - path: /properties/timestamp
            schema:
              type: string
              format: date-time
```

### Deprecation Handling

```yaml
gateway:
  openapi:
    deprecation:
      # Warn about deprecated operations
      warnDeprecated: true
      
      # Add deprecation headers
      headers:
        sunset: "Sunset"
        deprecation: "Deprecation"
      
      # Block deprecated after date
      blockAfter: "2024-12-31"
```

## Performance

### Caching

```yaml
gateway:
  openapi:
    cache:
      # Cache parsed specs
      specs:
        enabled: true
        ttl: 1h
      
      # Cache validation results
      validation:
        enabled: true
        ttl: 5m
        maxEntries: 10000
```

### Lazy Loading

```yaml
gateway:
  openapi:
    loading:
      lazy: true
      # Load specs on first use
      preload:
        - users-api  # Critical APIs
```

## Monitoring

### Metrics

Available metrics:
- `gateway_openapi_validation_errors_total`
- `gateway_openapi_mock_responses_total`
- `gateway_openapi_schema_cache_hits_total`
- `gateway_openapi_contract_violations_total`

### Logging

```yaml
gateway:
  logging:
    modules:
      openapi: debug
      openapi.validation: info
      openapi.mocking: debug
```

## Best Practices

1. **Keep Specs Updated**: Use CI/CD to validate spec changes
2. **Version Properly**: Use semantic versioning
3. **Document Examples**: Provide realistic examples
4. **Test Contracts**: Enable contract testing in staging
5. **Monitor Violations**: Track and fix contract violations
6. **Cache Wisely**: Cache specs and validation results

## Integration Example

Complete integration:

```yaml
gateway:
  openapi:
    enabled: true
    specs:
      - name: api-v2
        file: /specs/api-v2.yaml
        default: true
    
    validation:
      request:
        enabled: true
        strictMode: false
      response:
        enabled: true
        onError: log
    
    mocking:
      enabled: true
      when:
        - headerPresent: "X-Mock"
    
    ui:
      enabled: true
      path: /docs
    
    routing:
      byOperationId: true
```

This configuration provides full OpenAPI integration with validation, mocking, documentation, and routing based on the OpenAPI specification.