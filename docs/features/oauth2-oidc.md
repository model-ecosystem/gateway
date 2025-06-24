# OAuth2/OIDC Authentication

The gateway supports OAuth2 and OpenID Connect (OIDC) authentication, allowing you to secure your APIs using popular identity providers like Google, Auth0, Okta, or any OIDC-compliant provider.

## Overview

OAuth2/OIDC authentication in the gateway:
- Validates JWT tokens issued by configured OAuth2 providers
- Supports multiple providers simultaneously  
- Automatically fetches and caches JWKS (JSON Web Key Sets)
- Validates token claims including issuer, audience, and expiration
- Supports scope-based authorization
- Integrates seamlessly with RBAC for fine-grained access control

## Configuration

### Basic OAuth2/OIDC Setup

```yaml
gateway:
  middleware:
    auth:
      oauth2:
        enabled: true
        providers:
          - name: google
            issuer: "https://accounts.google.com"
            clientId: "your-client-id"
            clientSecret: "your-client-secret"
            redirectURL: "https://your-domain.com/callback"
            scopes:
              - openid
              - email
              - profile
          
          - name: auth0
            issuer: "https://your-tenant.auth0.com/"
            clientId: "your-client-id"
            clientSecret: "your-client-secret"
            redirectURL: "https://your-domain.com/callback"
            scopes:
              - openid
              - profile
```

### Provider Discovery

The gateway automatically discovers provider configuration using OpenID Connect Discovery:

```yaml
providers:
  - name: custom-provider
    issuer: "https://identity.example.com"
    # Discovery endpoint: https://identity.example.com/.well-known/openid-configuration
    # JWKS endpoint: Automatically discovered
    clientId: "gateway-client"
    clientSecret: "secret"
```

### Advanced Configuration

```yaml
gateway:
  middleware:
    auth:
      oauth2:
        enabled: true
        providers:
          - name: enterprise
            issuer: "https://identity.company.com"
            clientId: "api-gateway"
            clientSecret: "secret"
            
            # Custom endpoints (if discovery is not available)
            authorizationEndpoint: "https://identity.company.com/oauth2/authorize"
            tokenEndpoint: "https://identity.company.com/oauth2/token"
            userInfoEndpoint: "https://identity.company.com/oauth2/userinfo"
            jwksURI: "https://identity.company.com/oauth2/jwks"
            
            # Token validation
            audience: "https://api.company.com"
            clockSkew: 30s  # Allow 30 seconds clock skew
            
            # Required scopes
            scopes:
              - openid
              - api:read
              - api:write
            
            # JWKS caching
            jwksCacheDuration: 1h
            jwksRefreshInterval: 15m
```

## Route Configuration

Apply OAuth2 authentication to specific routes:

```yaml
gateway:
  router:
    rules:
      - id: protected-api
        path: /api/v1/*
        serviceName: backend-service
        middleware:
          - oauth2:
              provider: google
              requiredScopes:
                - api:read
      
      - id: admin-api
        path: /admin/*
        serviceName: admin-service
        middleware:
          - oauth2:
              provider: enterprise
              requiredScopes:
                - admin:full
```

## Token Validation

The gateway validates tokens according to OAuth2/OIDC standards:

1. **Signature Verification**: Using JWKS from the provider
2. **Issuer (iss)**: Must match configured issuer
3. **Audience (aud)**: Must include configured audience
4. **Expiration (exp)**: Token must not be expired
5. **Not Before (nbf)**: Token must be valid
6. **Issued At (iat)**: Token issue time validation
7. **Scopes**: Required scopes must be present

## Integration with RBAC

OAuth2 authentication works seamlessly with RBAC:

```yaml
gateway:
  middleware:
    authz:
      rbac:
        enabled: true
        policies:
          - id: user-policy
            subjects:
              - oauth2:google:user@example.com
            permissions:
              - documents:read
              - documents:write
          
          - id: admin-policy
            subjects:
              - oauth2:enterprise:admin@company.com
            permissions:
              - "*:*"
```

## Request Headers

Validated tokens add headers to upstream requests:

```
X-Auth-Subject: user@example.com
X-Auth-Provider: google
X-Auth-Scopes: openid,email,profile,api:read
X-Auth-Claims: {"sub":"12345","email":"user@example.com","name":"John Doe"}
```

## Error Handling

OAuth2 authentication errors return appropriate HTTP status codes:

- `401 Unauthorized`: Missing or invalid token
- `403 Forbidden`: Insufficient scopes
- `500 Internal Server Error`: Provider configuration issues

Example error response:
```json
{
  "error": "unauthorized",
  "message": "Invalid token",
  "details": {
    "reason": "token_expired",
    "expired_at": "2024-01-15T10:30:00Z"
  }
}
```

## Best Practices

1. **Use HTTPS**: Always use HTTPS for OAuth2 flows
2. **Secure Storage**: Store client secrets securely (environment variables or secrets management)
3. **Token Caching**: The gateway caches JWKS to reduce provider calls
4. **Scope Management**: Use minimal required scopes
5. **Regular Rotation**: Rotate client secrets regularly
6. **Monitor Failures**: Track authentication failures for security

## Provider Examples

### Google OAuth2

```yaml
providers:
  - name: google
    issuer: "https://accounts.google.com"
    clientId: "${GOOGLE_CLIENT_ID}"
    clientSecret: "${GOOGLE_CLIENT_SECRET}"
    redirectURL: "https://api.example.com/auth/callback"
    scopes:
      - openid
      - email
      - profile
```

### Auth0

```yaml
providers:
  - name: auth0
    issuer: "https://${AUTH0_DOMAIN}/"
    clientId: "${AUTH0_CLIENT_ID}"
    clientSecret: "${AUTH0_CLIENT_SECRET}"
    audience: "https://api.example.com"
    scopes:
      - openid
      - profile
      - email
```

### Okta

```yaml
providers:
  - name: okta
    issuer: "https://${OKTA_DOMAIN}/oauth2/default"
    clientId: "${OKTA_CLIENT_ID}"
    clientSecret: "${OKTA_CLIENT_SECRET}"
    audience: "api://default"
    scopes:
      - openid
      - profile
      - email
```

## Troubleshooting

### Common Issues

1. **Token Validation Failures**
   - Check token expiration
   - Verify issuer and audience match
   - Ensure JWKS endpoint is accessible

2. **JWKS Fetch Errors**
   - Verify network connectivity to provider
   - Check provider's JWKS endpoint URL
   - Review gateway logs for detailed errors

3. **Scope Validation**
   - Ensure token contains required scopes
   - Check scope configuration in provider
   - Verify scope names match exactly

### Debug Logging

Enable debug logging for OAuth2:

```yaml
gateway:
  logging:
    level: debug
    modules:
      auth.oauth2: debug
```

This provides detailed information about token validation, JWKS fetching, and claim extraction.