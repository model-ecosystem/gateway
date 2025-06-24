# Transform Middleware

The gateway's transform middleware allows you to modify requests and responses on the fly, enabling API adaptation, versioning, and data transformation without changing backend services.

## Overview

Transform middleware provides:
- Request header manipulation
- Response header modification
- URL path rewriting
- Query parameter transformation
- Request/response body transformation
- Content-type conversion
- API versioning support

## Configuration

### Basic Header Transformation

```yaml
gateway:
  middleware:
    transform:
      enabled: true
      rules:
        - id: add-headers
          request:
            headers:
              add:
                X-Gateway-Version: "1.0"
                X-Request-ID: "${requestId}"
              remove:
                - X-Internal-Header
              rename:
                X-Old-Header: X-New-Header
          
          response:
            headers:
              add:
                X-Response-Time: "${responseTime}"
              remove:
                - X-Internal-Data
```

### Path Rewriting

```yaml
gateway:
  middleware:
    transform:
      rules:
        - id: version-rewrite
          match:
            path: "/api/v1/*"
          request:
            path:
              # Remove version from path
              pattern: "^/api/v[0-9]+/(.*)"
              replacement: "/api/$1"
        
        - id: service-prefix
          match:
            path: "/users/*"
          request:
            path:
              # Add service prefix
              pattern: "^/users/(.*)"
              replacement: "/user-service/users/$1"
```

### Query Parameter Transformation

```yaml
gateway:
  middleware:
    transform:
      rules:
        - id: query-transform
          request:
            query:
              add:
                api_version: "2.0"
                source: "gateway"
              remove:
                - internal_param
              rename:
                old_param: new_param
              
              # Transform values
              transform:
                - param: limit
                  type: integer
                  default: 10
                  max: 100
                
                - param: sort
                  values:
                    "created": "created_at"
                    "updated": "updated_at"
```

## Body Transformation

### JSON Transformation

```yaml
gateway:
  middleware:
    transform:
      rules:
        - id: json-transform
          match:
            contentType: "application/json"
          
          request:
            body:
              json:
                # Add fields
                add:
                  - path: "$.metadata.gateway"
                    value: "api-gateway"
                  
                  - path: "$.timestamp"
                    value: "${now}"
                
                # Remove fields
                remove:
                  - "$.internal"
                  - "$.debug"
                
                # Rename fields
                rename:
                  - from: "$.user_id"
                    to: "$.userId"
                
                # Transform values
                transform:
                  - path: "$.status"
                    type: "uppercase"
```

### XML Transformation

```yaml
gateway:
  middleware:
    transform:
      rules:
        - id: xml-transform
          match:
            contentType: "application/xml"
          
          request:
            body:
              xml:
                namespaces:
                  soap: "http://schemas.xmlsoap.org/soap/envelope/"
                
                add:
                  - xpath: "/request"
                    element: "<source>gateway</source>"
                
                remove:
                  - xpath: "//debug"
                
                transform:
                  - xpath: "//username"
                    value: "lowercase"
```

## Content-Type Conversion

### JSON to XML

```yaml
gateway:
  middleware:
    transform:
      rules:
        - id: json-to-xml
          match:
            contentType: "application/json"
            path: "/legacy-api/*"
          
          request:
            body:
              convert:
                from: json
                to: xml
                rootElement: "request"
```

### Form to JSON

```yaml
gateway:
  middleware:
    transform:
      rules:
        - id: form-to-json
          match:
            contentType: "application/x-www-form-urlencoded"
          
          request:
            body:
              convert:
                from: form
                to: json
```

## Advanced Transformations

### Conditional Transformations

```yaml
gateway:
  middleware:
    transform:
      rules:
        - id: conditional-transform
          match:
            headers:
              X-API-Version: "v1"
          
          request:
            headers:
              add:
                X-Legacy-Client: "true"
            
            body:
              json:
                # Transform v1 to v2 format
                rename:
                  - from: "$.user_name"
                    to: "$.username"
                  - from: "$.email_address"
                    to: "$.email"
```

### Template-Based Transformation

```yaml
gateway:
  middleware:
    transform:
      rules:
        - id: template-transform
          request:
            body:
              template:
                engine: "handlebars"
                source: |
                  {
                    "request": {
                      "id": "{{requestId}}",
                      "method": "{{method}}",
                      "path": "{{path}}",
                      "data": {{body}}
                    }
                  }
```

### JavaScript Transformation

```yaml
gateway:
  middleware:
    transform:
      rules:
        - id: js-transform
          request:
            body:
              javascript: |
                // Access request data
                const data = JSON.parse(request.body);
                
                // Transform data
                data.timestamp = new Date().toISOString();
                data.source = 'gateway';
                
                // Return transformed data
                return JSON.stringify(data);
```

## Route-Specific Transforms

```yaml
gateway:
  router:
    rules:
      - id: user-api
        path: /api/users/*
        middleware:
          - transform:
              request:
                headers:
                  add:
                    X-Service: "user-service"
                path:
                  pattern: "^/api/users/(.*)"
                  replacement: "/v2/users/$1"
      
      - id: legacy-api
        path: /legacy/*
        middleware:
          - transform:
              request:
                body:
                  convert:
                    from: xml
                    to: json
              response:
                body:
                  convert:
                    from: json
                    to: xml
```

## Variables and Functions

### Built-in Variables

```yaml
transform:
  request:
    headers:
      add:
        X-Request-ID: "${requestId}"
        X-Timestamp: "${timestamp}"
        X-Client-IP: "${clientIp}"
        X-Method: "${method}"
        X-Path: "${path}"
        X-Host: "${host}"
```

### Functions

```yaml
transform:
  request:
    headers:
      add:
        X-Date: "${date('YYYY-MM-DD')}"
        X-UUID: "${uuid()}"
        X-Hash: "${hash('sha256', body)}"
        X-Env: "${env('ENVIRONMENT')}"
```

## API Versioning

### Version in Path

```yaml
gateway:
  middleware:
    transform:
      versioning:
        strategy: path
        rules:
          - version: "v1"
            path: "/api/v1/*"
            transform:
              # v1 to internal format
              request:
                path:
                  pattern: "^/api/v1/(.*)"
                  replacement: "/internal/$1"
          
          - version: "v2"
            path: "/api/v2/*"
            transform:
              # v2 uses internal format directly
              request:
                path:
                  pattern: "^/api/v2/(.*)"
                  replacement: "/internal/$1"
```

### Version in Header

```yaml
gateway:
  middleware:
    transform:
      versioning:
        strategy: header
        headerName: "X-API-Version"
        rules:
          - version: "1.0"
            transform:
              request:
                body:
                  json:
                    rename:
                      - from: "$.user_name"
                        to: "$.username"
```

## Performance Considerations

### Caching Transformations

```yaml
gateway:
  middleware:
    transform:
      cache:
        enabled: true
        ttl: 5m
        maxEntries: 1000
```

### Streaming Transformations

```yaml
gateway:
  middleware:
    transform:
      streaming:
        enabled: true
        bufferSize: 8192
        # Stream large responses
        minSize: 1MB
```

## Error Handling

### Transformation Errors

```yaml
gateway:
  middleware:
    transform:
      errorHandling:
        # What to do on transformation error
        onError: "pass-through"  # or "fail", "default"
        
        # Default response on error
        defaultResponse:
          status: 500
          body: |
            {
              "error": "transformation_failed",
              "message": "Request transformation error"
            }
```

## Best Practices

1. **Keep Transformations Simple**
   - Complex logic belongs in services
   - Use for adaptation, not business logic

2. **Version Carefully**
   - Document all transformations
   - Test version migrations thoroughly

3. **Monitor Performance**
   - Track transformation time
   - Cache when possible

4. **Handle Errors Gracefully**
   - Define fallback behavior
   - Log transformation failures

## Examples

### API Migration

```yaml
# Migrate from old to new API format
old-to-new-api:
  transform:
    request:
      body:
        json:
          # Old format: { "user_data": { "name": "John" } }
          # New format: { "user": { "fullName": "John" } }
          rename:
            - from: "$.user_data"
              to: "$.user"
            - from: "$.user.name"
              to: "$.user.fullName"
```

### Security Headers

```yaml
security-headers:
  transform:
    response:
      headers:
        add:
          X-Content-Type-Options: "nosniff"
          X-Frame-Options: "DENY"
          X-XSS-Protection: "1; mode=block"
        remove:
          - Server
          - X-Powered-By
```

### Request Enrichment

```yaml
enrich-request:
  transform:
    request:
      headers:
        add:
          X-Request-ID: "${uuid()}"
          X-Correlation-ID: "${header('X-Correlation-ID') || uuid()}"
          X-Real-IP: "${clientIp}"
      body:
        json:
          add:
            - path: "$.metadata"
              value:
                gateway: "api-gateway"
                timestamp: "${timestamp}"
                version: "1.0"
```