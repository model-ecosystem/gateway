# Advanced Routing Guide

This guide covers advanced routing patterns and techniques for the gateway.

## Table of Contents

- [Path Matching Patterns](#path-matching-patterns)
- [Route Priorities](#route-priorities)
- [Method-Based Routing](#method-based-routing)
- [Header-Based Routing](#header-based-routing)
- [Query Parameter Routing](#query-parameter-routing)
- [Content-Type Routing](#content-type-routing)
- [Composite Routes](#composite-routes)
- [Route Groups](#route-groups)
- [Dynamic Route Loading](#dynamic-route-loading)
- [Route Transformations](#route-transformations)

## Path Matching Patterns

### Exact Match

```yaml
gateway:
  router:
    rules:
      - id: exact-match
        path: /api/v1/users
        serviceName: user-service
        # Only matches exactly /api/v1/users
```

### Prefix Match

```yaml
gateway:
  router:
    rules:
      - id: prefix-match
        path: /api/v1/*
        serviceName: api-service
        # Matches /api/v1/users, /api/v1/products, etc.
```

### Wildcard Patterns

```yaml
gateway:
  router:
    rules:
      - id: wildcard-middle
        path: /api/*/users
        serviceName: user-service
        # Matches /api/v1/users, /api/v2/users, etc.
        
      - id: multi-wildcard
        path: /api/*/users/*
        serviceName: user-service
        # Matches /api/v1/users/123, /api/v2/users/profile, etc.
```

### Regular Expression Matching

```yaml
gateway:
  router:
    rules:
      - id: regex-match
        path: /api/v[0-9]+/users
        pathType: regex
        serviceName: user-service
        # Matches /api/v1/users, /api/v2/users, etc.
        
      - id: complex-regex
        path: ^/api/(v[0-9]+|latest)/users/([0-9]+|profile)$
        pathType: regex
        serviceName: user-service
        # Capture groups available in transformations
```

## Route Priorities

### Explicit Priority

```yaml
gateway:
  router:
    rules:
      - id: specific-route
        path: /api/v1/users/admin
        serviceName: admin-service
        priority: 100  # Higher priority
        
      - id: general-route
        path: /api/v1/users/*
        serviceName: user-service
        priority: 50   # Lower priority
```

### Automatic Priority

The gateway automatically assigns priorities based on specificity:
1. Exact paths (highest)
2. Paths with fewer wildcards
3. Longer paths
4. Regex paths (lowest)

## Method-Based Routing

### Single Method

```yaml
gateway:
  router:
    rules:
      - id: read-only
        path: /api/users/*
        methods: [GET, HEAD, OPTIONS]
        serviceName: read-service
        
      - id: write-operations
        path: /api/users/*
        methods: [POST, PUT, PATCH, DELETE]
        serviceName: write-service
```

### Method-Specific Configuration

```yaml
gateway:
  router:
    rules:
      - id: user-operations
        path: /api/users/*
        serviceName: user-service
        methodConfig:
          GET:
            timeout: 5s
            cache: true
          POST:
            timeout: 30s
            rateLimit: 100
          DELETE:
            auth: required
            audit: true
```

## Header-Based Routing

### Simple Header Matching

```yaml
gateway:
  router:
    rules:
      - id: mobile-route
        path: /api/*
        headers:
          User-Agent: Mobile/*
        serviceName: mobile-api
        
      - id: internal-route
        path: /api/*
        headers:
          X-Internal-Request: "true"
        serviceName: internal-api
```

### Complex Header Conditions

```yaml
gateway:
  router:
    rules:
      - id: versioned-api
        path: /api/*
        headers:
          X-API-Version: 
            match: regex
            value: "^[2-3]\\..*"
        serviceName: api-v2-service
        
      - id: beta-features
        path: /api/*
        headers:
          - name: X-Beta-User
            present: true
          - name: X-Feature-Flag
            values: ["feature-x", "feature-y"]
        serviceName: beta-service
```

## Query Parameter Routing

### Parameter Presence

```yaml
gateway:
  router:
    rules:
      - id: debug-route
        path: /api/*
        queryParams:
          debug: 
            present: true
        serviceName: debug-service
        
      - id: test-route
        path: /api/*
        queryParams:
          env:
            value: test
        serviceName: test-service
```

### Parameter Patterns

```yaml
gateway:
  router:
    rules:
      - id: pagination-route
        path: /api/list
        queryParams:
          page:
            match: regex
            value: "^[0-9]+$"
          size:
            match: regex
            value: "^(10|20|50|100)$"
        serviceName: paginated-service
```

## Content-Type Routing

### Request Content-Type

```yaml
gateway:
  router:
    rules:
      - id: json-api
        path: /api/*
        contentType:
          request: application/json
        serviceName: json-service
        
      - id: xml-api
        path: /api/*
        contentType:
          request: application/xml
        serviceName: xml-service
```

### Accept Header Routing

```yaml
gateway:
  router:
    rules:
      - id: graphql-route
        path: /graphql
        headers:
          Accept: application/graphql+json
        serviceName: graphql-service
        
      - id: rest-route
        path: /api/*
        headers:
          Accept: application/json
        serviceName: rest-service
```

## Composite Routes

### AND Conditions

```yaml
gateway:
  router:
    rules:
      - id: authenticated-mobile
        path: /api/secure/*
        conditions:
          all:  # All conditions must match
            - headers:
                Authorization: Bearer *
            - headers:
                User-Agent: Mobile/*
            - methods: [GET, POST]
        serviceName: secure-mobile-service
```

### OR Conditions

```yaml
gateway:
  router:
    rules:
      - id: public-or-internal
        path: /api/data/*
        conditions:
          any:  # Any condition matches
            - headers:
                X-Public-Access: "true"
            - headers:
                X-Internal-Token: *
            - sourceIP: 10.0.0.0/8
        serviceName: data-service
```

## Route Groups

### Service Groups

```yaml
gateway:
  router:
    groups:
      - name: user-services
        pathPrefix: /api/users
        serviceName: user-service
        middleware: [auth, ratelimit]
        routes:
          - id: user-profile
            path: /profile
            methods: [GET]
            
          - id: user-settings
            path: /settings
            methods: [GET, PUT]
            
          - id: user-delete
            path: /delete
            methods: [DELETE]
            requireRole: admin
```

### Version Groups

```yaml
gateway:
  router:
    groups:
      - name: api-v1
        pathPrefix: /api/v1
        deprecated: true
        middleware: [deprecation-header]
        routes:
          - id: v1-users
            path: /users/*
            serviceName: user-service-v1
            
      - name: api-v2
        pathPrefix: /api/v2
        routes:
          - id: v2-users
            path: /users/*
            serviceName: user-service-v2
```

## Dynamic Route Loading

### File-Based Routes

```yaml
gateway:
  router:
    dynamic:
      enabled: true
      sources:
        - type: file
          path: /etc/gateway/routes/*.yaml
          watch: true
          
        - type: directory
          path: /etc/gateway/routes.d/
          recursive: true
```

### API-Based Routes

```yaml
gateway:
  router:
    dynamic:
      sources:
        - type: http
          url: https://api-catalog/routes
          interval: 30s
          transform: |
            routes.map(r => ({
              id: r.id,
              path: r.basePath + "/*",
              serviceName: r.service,
              metadata: r.metadata
            }))
```

## Route Transformations

### Path Rewriting

```yaml
gateway:
  router:
    rules:
      - id: rewrite-version
        path: /api/latest/*
        serviceName: api-service
        transform:
          path:
            # Remove /api/latest prefix
            stripPrefix: /api/latest
            # Add /v2 prefix
            addPrefix: /v2
```

### Path Templates

```yaml
gateway:
  router:
    rules:
      - id: template-route
        path: /api/{version}/users/{id}
        pathType: template
        serviceName: user-service
        transform:
          path:
            template: /internal/users/{id}?version={version}
```

### Header Transformations

```yaml
gateway:
  router:
    rules:
      - id: header-transform
        path: /api/*
        serviceName: api-service
        transform:
          headers:
            add:
              X-Gateway-Version: "1.0"
              X-Request-ID: "${requestId}"
            remove:
              - X-Internal-Secret
              - Authorization
            rename:
              X-Original-Host: Host
```

## Advanced Examples

### Multi-Tenant Routing

```yaml
gateway:
  router:
    rules:
      - id: tenant-route
        path: /{tenant}/api/*
        pathType: template
        serviceName: tenant-service
        transform:
          headers:
            add:
              X-Tenant-ID: "{tenant}"
          path:
            # Remove tenant from path
            template: /api/*
        validation:
          tenant:
            pattern: "^[a-z0-9-]+$"
            minLength: 3
            maxLength: 63
```

### A/B Testing Routes

```yaml
gateway:
  router:
    rules:
      - id: ab-test
        path: /api/features/*
        serviceName: feature-service
        abTesting:
          enabled: true
          experiments:
            - name: new-algorithm
              percentage: 20
              serviceName: feature-service-v2
              criteria:
                headers:
                  X-User-Segment: ["beta", "power-user"]
```

### Geographic Routing

```yaml
gateway:
  router:
    rules:
      - id: geo-route
        path: /api/content/*
        geoRouting:
          enabled: true
          defaultService: global-content
          regions:
            - codes: ["US", "CA"]
              serviceName: na-content
            - codes: ["GB", "DE", "FR"]
              serviceName: eu-content
            - codes: ["JP", "KR", "CN"]
              serviceName: asia-content
```

## Best Practices

1. **Order Routes Carefully**: Place more specific routes before general ones
2. **Use Meaningful IDs**: Route IDs should describe their purpose
3. **Group Related Routes**: Use route groups for better organization
4. **Validate Inputs**: Add validation rules for path parameters
5. **Monitor Route Performance**: Track metrics per route
6. **Document Routes**: Include descriptions and examples
7. **Test Route Conflicts**: Ensure routes don't overlap unintentionally

## Troubleshooting

### Route Not Matching

1. Check route priority
2. Verify path pattern syntax
3. Test with route debugging enabled
4. Check for conflicting routes

### Performance Issues

1. Optimize regex patterns
2. Use exact matches where possible
3. Reduce transformation complexity
4. Enable route caching

### Debug Mode

```yaml
gateway:
  router:
    debug:
      enabled: true
      logMatching: true
      logTransforms: true
      traceHeaders:
        - X-Debug-Route
```