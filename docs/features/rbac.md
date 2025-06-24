# Role-Based Access Control (RBAC)

The gateway provides a flexible RBAC system for fine-grained authorization control over your APIs. RBAC works seamlessly with all authentication methods (JWT, API Key, OAuth2) to enforce access policies.

## Overview

RBAC in the gateway:
- Define roles with specific permissions
- Assign roles to users, API keys, or OAuth2 identities
- Support wildcard permissions for flexible policies
- Cache policy decisions for performance
- Integrate with authentication middleware
- Per-route authorization rules

## Configuration

### Basic RBAC Setup

```yaml
gateway:
  middleware:
    authz:
      rbac:
        enabled: true
        policies:
          - id: reader-policy
            description: Read-only access to documents
            subjects:
              - user:john@example.com
              - apikey:read-key-123
            roles:
              - document-reader
            permissions:
              - documents:read
              - documents:list
          
          - id: writer-policy
            description: Full access to documents
            subjects:
              - user:jane@example.com
              - oauth2:google:jane@gmail.com
            roles:
              - document-writer
            permissions:
              - documents:read
              - documents:write
              - documents:delete
```

### Role Definitions

Define reusable roles:

```yaml
gateway:
  middleware:
    authz:
      rbac:
        enabled: true
        roles:
          - name: viewer
            permissions:
              - "*:read"
              - "*:list"
          
          - name: editor
            permissions:
              - "*:read"
              - "*:write"
              - "*:update"
          
          - name: admin
            permissions:
              - "*:*"  # Full access
        
        policies:
          - id: user-roles
            subjects:
              - user:viewer@example.com
            roles:
              - viewer
          
          - id: admin-users
            subjects:
              - user:admin@example.com
            roles:
              - admin
```

## Permission Syntax

Permissions follow the pattern: `resource:action`

### Wildcard Permissions

- `*:*` - Full access to all resources and actions
- `documents:*` - All actions on documents resource
- `*:read` - Read access to all resources
- `documents:read` - Read access to documents only

### Examples

```yaml
permissions:
  # Specific permissions
  - users:create
  - users:read
  - users:update
  - users:delete
  
  # Wildcard permissions
  - users:*          # All actions on users
  - "*:read"         # Read any resource
  - "*:*"            # Full admin access
  
  # API permissions
  - api:v1:read
  - api:v2:*
  - metrics:export
```

## Route Authorization

Apply RBAC to specific routes:

```yaml
gateway:
  router:
    rules:
      - id: public-api
        path: /api/public/*
        serviceName: backend
        # No authorization required
      
      - id: user-api
        path: /api/users/*
        serviceName: user-service
        middleware:
          - jwt: {}
          - rbac:
              requiredPermissions:
                - users:read
      
      - id: admin-api
        path: /api/admin/*
        serviceName: admin-service
        middleware:
          - jwt: {}
          - rbac:
              requiredPermissions:
                - admin:*
              requireAll: true
```

## Subject Types

RBAC supports different subject types based on authentication method:

### JWT Subjects

```yaml
subjects:
  - user:john@example.com      # From JWT 'sub' claim
  - jwt:custom-claim:value      # From custom JWT claims
```

### API Key Subjects

```yaml
subjects:
  - apikey:prod-key-123         # API key ID
  - apikey:name:mobile-app      # API key name
```

### OAuth2 Subjects

```yaml
subjects:
  - oauth2:google:user@gmail.com     # OAuth2 provider:subject
  - oauth2:auth0:auth0|123456        # Auth0 user ID
```

## Dynamic Permissions

Extract permissions from authentication tokens:

```yaml
gateway:
  middleware:
    authz:
      rbac:
        enabled: true
        extractPermissions:
          jwt:
            claimPath: permissions     # Array of permissions in JWT
          oauth2:
            scopeMapping:             # Map OAuth2 scopes to permissions
              "api:read": 
                - documents:read
                - users:read
              "api:write":
                - documents:write
                - users:write
```

## Caching

RBAC decisions are cached for performance:

```yaml
gateway:
  middleware:
    authz:
      rbac:
        cache:
          enabled: true
          ttl: 5m              # Cache decisions for 5 minutes
          maxEntries: 10000    # Maximum cache entries
```

## Policy Evaluation

Policies are evaluated in order:

1. Check if subject matches any policy
2. Collect all permissions from matched policies
3. Verify required permissions are present
4. Cache the decision

### Evaluation Logic

```yaml
policies:
  - id: default-user
    subjects:
      - user:*              # Matches any user
    permissions:
      - profile:read
  
  - id: premium-user
    subjects:
      - user:premium@example.com
    permissions:
      - profile:write
      - api:unlimited

# User 'premium@example.com' gets both default and premium permissions
```

## Integration Examples

### With JWT Authentication

```yaml
gateway:
  middleware:
    auth:
      jwt:
        enabled: true
        publicKeyPath: /keys/public.pem
    
    authz:
      rbac:
        enabled: true
        policies:
          - id: jwt-users
            subjects:
              - user:${jwt.sub}    # Dynamic subject from JWT
            permissions:
              - api:access
```

### With API Keys

```yaml
gateway:
  middleware:
    auth:
      apikey:
        enabled: true
        keys:
          - id: mobile-key
            key: "${MOBILE_API_KEY}"
            name: mobile-app
    
    authz:
      rbac:
        enabled: true
        policies:
          - id: mobile-access
            subjects:
              - apikey:name:mobile-app
            permissions:
              - mobile:*
```

## Error Responses

RBAC authorization failures return `403 Forbidden`:

```json
{
  "error": "forbidden",
  "message": "Insufficient permissions",
  "details": {
    "required": ["documents:write"],
    "actual": ["documents:read"],
    "subject": "user:john@example.com"
  }
}
```

## Best Practices

1. **Principle of Least Privilege**: Grant minimal required permissions
2. **Use Roles**: Group permissions into logical roles
3. **Wildcard Carefully**: Avoid overly broad wildcard permissions
4. **Regular Audits**: Review and audit permission assignments
5. **Cache Wisely**: Balance performance with permission changes
6. **Clear Naming**: Use descriptive permission names

## Advanced Patterns

### Hierarchical Permissions

```yaml
permissions:
  - org:read
  - org:123:read         # Specific org
  - org:123:users:read   # Users in org 123
  - org:123:users:456:*  # All actions on user 456
```

### Time-Based Permissions

```yaml
policies:
  - id: temporary-access
    subjects:
      - user:contractor@example.com
    validFrom: "2024-01-01T00:00:00Z"
    validUntil: "2024-12-31T23:59:59Z"
    permissions:
      - projects:read
```

### Conditional Permissions

```yaml
policies:
  - id: ip-restricted
    subjects:
      - user:remote@example.com
    conditions:
      - type: ip_range
        value: "10.0.0.0/8"
    permissions:
      - internal:access
```

## Troubleshooting

### Common Issues

1. **Permission Denied**
   - Verify subject identifier format
   - Check permission spelling
   - Review policy evaluation order

2. **Policy Not Applied**
   - Ensure authentication completes first
   - Verify subject extraction from token
   - Check RBAC middleware ordering

3. **Performance Issues**
   - Enable caching
   - Reduce policy complexity
   - Optimize permission checks

### Debug Logging

```yaml
gateway:
  logging:
    level: debug
    modules:
      authz.rbac: debug
```

This logs policy evaluation, subject matching, and permission checks.