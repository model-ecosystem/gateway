# Authentication Guide

This guide explains how to configure and use authentication in the gateway.

## Overview

The gateway supports multiple authentication methods:
- **JWT (JSON Web Tokens)**: For OAuth2/OIDC integration
- **API Keys**: For service-to-service authentication

## Configuration

### Basic Authentication Setup

```yaml
gateway:
  auth:
    required: true              # Make authentication mandatory
    providers:                  # List of enabled providers
      - jwt
      - apikey
    skipPaths:                  # Paths that don't require auth
      - /public/
      - /health
    requiredScopes:             # Scopes required for all requests
      - api:read
```

### JWT Configuration

```yaml
jwt:
  enabled: true
  issuer: "https://auth.example.com"           # Expected token issuer
  audience:                                    # Expected audiences
    - "https://api.example.com"
  signingMethod: "RS256"                       # Algorithm (RS256, HS256, etc.)
  publicKey: |                                 # Public key for RS256
    -----BEGIN PUBLIC KEY-----
    MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
    -----END PUBLIC KEY-----
  # OR for JWKS endpoint
  jwksEndpoint: "https://auth.example.com/.well-known/jwks.json"
  jwksCacheDuration: 3600                      # Cache duration in seconds (default: 3600)
  headerName: "Authorization"                  # Header to extract token from (default: "Authorization")
  cookieName: "jwt_token"                      # Cookie name for fallback extraction
  scopeClaim: "scope"                          # JWT claim containing scopes (default: "scope")
  subjectClaim: "sub"                          # JWT claim containing subject (default: "sub")
```

### API Key Configuration

```yaml
apikey:
  enabled: true
  hashKeys: true                               # Store hashed keys (recommended)
  defaultScopes:                               # Default scopes for all keys
    - api:read
  headerName: "X-API-Key"                      # Header to extract key from (default: "X-API-Key", case-insensitive)
  scheme: ""                                   # Optional: Extract from Authorization header with custom scheme
  keys:
    service-key-1:
      key: "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3"  # SHA256 hash of actual key
      subject: "payment-service"               # Optional: defaults to key name if not specified
      type: "service"                          # Optional: defaults to "service"
      scopes:
        - api:read
        - api:write
        - payments:process
```

## Usage Examples

### Using JWT Authentication

```bash
# Get JWT token from your auth server
TOKEN=$(curl -s https://auth.example.com/token | jq -r .access_token)

# Make authenticated request (Bearer scheme is required)
curl -H "Authorization: Bearer $TOKEN" \
     http://gateway:8080/api/users
     
# Or using cookie fallback
curl --cookie "jwt_token=$TOKEN" \
     http://gateway:8080/api/users
```

### Using API Key Authentication

```bash
# Using API key in header (default)
curl -H "X-API-Key: your-api-key-here" \
     http://gateway:8080/api/users

# Using API key with custom scheme in Authorization header
# (requires scheme: "ApiKey" in config)
curl -H "Authorization: ApiKey your-api-key-here" \
     http://gateway:8080/api/users

# Note: Query parameter extraction is not currently implemented
# The gateway will hash the provided key and compare with stored hashes
```

### Generating API Key Hashes

To generate SHA256 hashes for your API keys:

```bash
# Using echo and sha256sum
echo -n "your-secret-api-key" | sha256sum

# Using OpenSSL
echo -n "your-secret-api-key" | openssl dgst -sha256
```

## Authentication Flow

1. **Request arrives** at the gateway
2. **Path check**: If path is in `skipPaths`, authentication is skipped
3. **Credential extraction**: Extractors try to find credentials in headers
4. **Provider authentication**: Each configured provider attempts authentication
5. **Scope validation**: Required scopes are checked
6. **Context enrichment**: Auth info is added to request context
7. **Headers added**: Auth headers are added to upstream request

## Response Headers

When authentication succeeds, the gateway adds these headers to responses:
- `X-Auth-Subject`: The authenticated subject (user/service ID)
- `X-Auth-Type`: The subject type (user/service/device)

## Additional Notes

### JWT Authentication
- The `Bearer` scheme in the Authorization header is required and not configurable
- JWT tokens can contain a `typ` claim to specify subject type (user/service/device)
- Scopes can be a space-separated string or an array in the JWT claim
- Default signing method is RS256 if not specified

### API Key Authentication  
- API keys are matched case-insensitively in headers
- If no subject is specified for a key, the key name itself becomes the subject
- Query parameter extraction (`queryParam`) is defined in config but not implemented

## Security Best Practices

1. **Always use HTTPS** in production to protect credentials in transit
2. **Rotate API keys** regularly
3. **Use short JWT expiration** times
4. **Store only hashed API keys** in configuration
5. **Implement proper RBAC** with scopes
6. **Monitor authentication failures** for security threats

## Testing Authentication

The gateway includes test scripts for authentication:

```bash
# Run authentication tests
./scripts/test-auth.sh

# Generate test JWT token
go run ./scripts/generate-jwt.go
```

## Troubleshooting

### Common Issues

1. **401 Unauthorized**: Check credentials are correctly formatted
2. **403 Forbidden**: User authenticated but lacks required scopes
3. **Invalid token**: Verify JWT signature and expiration
4. **Key not found**: Ensure API key is properly hashed if `hashKeys: true`

### Debug Logging

Enable debug logging to troubleshoot authentication:

```bash
./gateway -log-level=debug
```

This will show:
- Credential extraction attempts
- Provider authentication attempts
- Scope validation results
- Auth context details