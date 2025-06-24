# Environment Variables Guide

This guide documents all environment variables supported by the Gateway, organized by feature area.

## Overview

The Gateway supports environment variable substitution in configuration files using the syntax:
- `${VAR_NAME}` - Required variable (will error if not set)
- `${VAR_NAME:-default}` - Optional variable with default value

## Core Gateway Settings

### Server Configuration
- `GATEWAY_HOST` - Gateway listening host (default: `0.0.0.0`)
- `GATEWAY_PORT` - Gateway listening port (default: `8080`)
- `GATEWAY_READ_TIMEOUT` - HTTP read timeout in seconds (default: `30`)
- `GATEWAY_WRITE_TIMEOUT` - HTTP write timeout in seconds (default: `30`)
- `GATEWAY_MAX_REQUEST_SIZE` - Maximum request body size in bytes (default: `10485760`)

### TLS Configuration
- `GATEWAY_TLS_ENABLED` - Enable HTTPS (default: `false`)
- `GATEWAY_TLS_CERT_FILE` - Path to TLS certificate file
- `GATEWAY_TLS_KEY_FILE` - Path to TLS key file
- `GATEWAY_TLS_MIN_VERSION` - Minimum TLS version (default: `1.2`)
- `GATEWAY_TLS_MAX_VERSION` - Maximum TLS version (default: `1.3`)

### Backend Connection Settings
- `BACKEND_MAX_IDLE_CONNS` - Maximum idle connections (default: `100`)
- `BACKEND_MAX_IDLE_CONNS_PER_HOST` - Maximum idle connections per host (default: `10`)
- `BACKEND_MAX_CONNS_PER_HOST` - Maximum connections per host (default: `50`)
- `BACKEND_IDLE_CONN_TIMEOUT` - Idle connection timeout in seconds (default: `90`)
- `BACKEND_DIAL_TIMEOUT` - Connection dial timeout in seconds (default: `10`)
- `BACKEND_RESPONSE_HEADER_TIMEOUT` - Response header timeout in seconds (default: `10`)

### Backend TLS Settings
- `BACKEND_TLS_ENABLED` - Enable TLS for backend connections (default: `false`)
- `BACKEND_TLS_INSECURE_SKIP_VERIFY` - Skip TLS verification (default: `false`)
- `BACKEND_TLS_SERVER_NAME` - Expected server name for TLS verification
- `BACKEND_TLS_CLIENT_CERT_FILE` - Client certificate for mTLS
- `BACKEND_TLS_CLIENT_KEY_FILE` - Client key for mTLS

## Authentication Settings

### JWT Authentication
- `JWT_ENABLED` - Enable JWT authentication (default: `false`)
- `JWT_ISSUER` - Expected token issuer
- `JWT_AUDIENCE` - Expected token audience
- `JWT_JWKS_ENDPOINT` - JWKS endpoint URL
- `JWT_ALGORITHM` - JWT signing algorithm (default: `RS256`)
- `JWT_SECRET` - Shared secret for HMAC algorithms
- `JWT_PUBLIC_KEY_FILE` - Public key file for RSA algorithms
- `JWT_VALIDATE_EXP` - Validate token expiration (default: `true`)
- `JWT_VALIDATE_NBF` - Validate not-before claim (default: `true`)
- `JWT_CLOCK_SKEW` - Clock skew tolerance in seconds (default: `60`)

### API Key Authentication
- `APIKEY_ENABLED` - Enable API key authentication (default: `false`)
- `APIKEY_HEADER` - Header name for API key (default: `X-API-Key`)
- `APIKEY_QUERY` - Query parameter for API key (default: `api_key`)
- `APIKEY_HASH_ALGORITHM` - Hashing algorithm (default: `sha256`)

### OAuth2/OIDC Settings
- `OAUTH2_ENABLED` - Enable OAuth2 authentication (default: `false`)
- `OAUTH2_REQUIRE_SCOPES` - Required scopes (comma-separated)

#### Provider-specific settings (replace `{PROVIDER}` with provider name)
- `{PROVIDER}_CLIENT_ID` - OAuth2 client ID
- `{PROVIDER}_CLIENT_SECRET` - OAuth2 client secret
- `{PROVIDER}_ISSUER_URL` - OIDC issuer URL
- `{PROVIDER}_AUTH_URL` - Authorization endpoint
- `{PROVIDER}_TOKEN_URL` - Token endpoint
- `{PROVIDER}_USERINFO_URL` - UserInfo endpoint
- `{PROVIDER}_SCOPES` - Requested scopes (comma-separated)

Common providers:
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`
- `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`
- `KEYCLOAK_CLIENT_ID`, `KEYCLOAK_CLIENT_SECRET`, `KEYCLOAK_ISSUER_URL`
- `AUTH0_CLIENT_ID`, `AUTH0_CLIENT_SECRET`, `AUTH0_DOMAIN`, `AUTH0_AUDIENCE`

## Authorization Settings

### RBAC Configuration
- `RBAC_ENABLED` - Enable RBAC authorization (default: `false`)
- `RBAC_ENFORCEMENT_MODE` - Enforcement mode: `enforce` or `permissive` (default: `enforce`)
- `RBAC_DEFAULT_ALLOW` - Default allow/deny policy (default: `false`)
- `RBAC_CACHE_SIZE` - Policy cache size (default: `1000`)
- `RBAC_CACHE_TTL` - Policy cache TTL in seconds (default: `300`)

## Rate Limiting Settings

### General Rate Limiting
- `RATELIMIT_ENABLED` - Enable rate limiting (default: `false`)
- `RATELIMIT_ALGORITHM` - Algorithm: `token_bucket` or `sliding_window` (default: `token_bucket`)
- `RATELIMIT_STORAGE` - Storage backend: `memory` or `redis` (default: `memory`)

### Global Rate Limits
- `RATELIMIT_GLOBAL_ENABLED` - Enable global rate limiting (default: `true`)
- `RATELIMIT_GLOBAL_RATE` - Requests per period (default: `10000`)
- `RATELIMIT_GLOBAL_BURST` - Burst capacity (default: `1000`)
- `RATELIMIT_GLOBAL_PERIOD` - Time period (default: `1m`)

### Per-IP Rate Limits
- `RATELIMIT_PER_IP_ENABLED` - Enable per-IP rate limiting (default: `true`)
- `RATELIMIT_PER_IP_RATE` - Requests per period (default: `100`)
- `RATELIMIT_PER_IP_BURST` - Burst capacity (default: `50`)
- `RATELIMIT_PER_IP_PERIOD` - Time period (default: `1m`)

## Service Discovery Settings

### Docker Discovery
- `DOCKER_ENABLED` - Enable Docker discovery (default: `false`)
- `DOCKER_ENDPOINT` - Docker daemon endpoint (default: `unix:///var/run/docker.sock`)
- `DOCKER_LABEL_PREFIX` - Label prefix for discovery (default: `gateway`)
- `DOCKER_REFRESH_INTERVAL` - Refresh interval in seconds (default: `30`)

### Kubernetes Discovery
- `K8S_ENABLED` - Enable Kubernetes discovery (default: `false`)
- `K8S_NAMESPACE` - Kubernetes namespace (default: `default`)
- `K8S_LABEL_SELECTOR` - Label selector for services
- `K8S_REFRESH_INTERVAL` - Refresh interval in seconds (default: `30`)
- `K8S_KUBECONFIG` - Path to kubeconfig file (optional)

### Docker Compose Discovery
- `COMPOSE_ENABLED` - Enable Docker Compose discovery (default: `false`)
- `COMPOSE_PROJECT_NAME` - Docker Compose project name
- `COMPOSE_NETWORK_NAME` - Docker network name
- `COMPOSE_REFRESH_INTERVAL` - Refresh interval in seconds (default: `30`)

## Resilience Settings

### Circuit Breaker
- `CIRCUITBREAKER_ENABLED` - Enable circuit breaker (default: `false`)
- `CIRCUITBREAKER_FAILURE_THRESHOLD` - Failures before opening (default: `5`)
- `CIRCUITBREAKER_SUCCESS_THRESHOLD` - Successes before closing (default: `2`)
- `CIRCUITBREAKER_TIMEOUT` - Timeout in seconds (default: `30`)
- `CIRCUITBREAKER_HALF_OPEN_REQUESTS` - Requests in half-open state (default: `3`)
- `CIRCUITBREAKER_OBSERVABILITY_WINDOW` - Window size in seconds (default: `60`)

### Retry Configuration
- `RETRY_ENABLED` - Enable retry logic (default: `false`)
- `RETRY_MAX_ATTEMPTS` - Maximum retry attempts (default: `3`)
- `RETRY_INITIAL_DELAY` - Initial delay in milliseconds (default: `100`)
- `RETRY_MAX_DELAY` - Maximum delay in milliseconds (default: `10000`)
- `RETRY_MULTIPLIER` - Delay multiplier (default: `2`)
- `RETRY_JITTER` - Jitter factor (default: `0.1`)
- `RETRY_BUDGET_ENABLED` - Enable retry budget (default: `true`)
- `RETRY_BUDGET_RATIO` - Retry budget ratio (default: `0.1`)

## Observability Settings

### Health Checks
- `HEALTH_ENABLED` - Enable health checks (default: `true`)
- `HEALTH_INTERVAL` - Check interval in seconds (default: `10`)
- `HEALTH_TIMEOUT` - Check timeout in seconds (default: `5`)
- `HEALTH_UNHEALTHY_THRESHOLD` - Failures before unhealthy (default: `3`)
- `HEALTH_HEALTHY_THRESHOLD` - Successes before healthy (default: `2`)

### Metrics
- `METRICS_ENABLED` - Enable metrics collection (default: `true`)
- `METRICS_PATH` - Metrics endpoint path (default: `/metrics`)
- `METRICS_INCLUDE_PATH_LABEL` - Include path in labels (default: `true`)
- `METRICS_INCLUDE_METHOD_LABEL` - Include method in labels (default: `true`)
- `METRICS_INCLUDE_STATUS_LABEL` - Include status in labels (default: `true`)

### OpenTelemetry
- `OTEL_ENABLED` - Enable OpenTelemetry (default: `false`)
- `OTEL_SERVICE_NAME` - Service name for telemetry (default: `api-gateway`)
- `OTEL_API_KEY` - API key for telemetry backend

#### Tracing
- `OTEL_TRACING_ENABLED` - Enable tracing (default: `true`)
- `OTEL_TRACING_SAMPLER` - Sampler type: `always`, `never`, `probabilistic` (default: `always`)
- `OTEL_TRACING_SAMPLER_ARG` - Sampler argument (default: `1.0`)
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` - OTLP traces endpoint
- `OTEL_EXPORTER_OTLP_TRACES_INSECURE` - Use insecure connection (default: `true`)

#### Metrics
- `OTEL_METRICS_ENABLED` - Enable OTEL metrics (default: `true`)
- `OTEL_METRICS_INTERVAL` - Export interval in seconds (default: `60`)
- `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` - OTLP metrics endpoint

#### Logging
- `OTEL_LOGGING_ENABLED` - Enable structured logging (default: `true`)
- `OTEL_LOG_LEVEL` - Log level: `debug`, `info`, `warn`, `error` (default: `info`)
- `OTEL_LOG_FORMAT` - Log format: `json` or `text` (default: `json`)

## Storage Settings

### Redis Configuration
- `REDIS_ENABLED` - Enable Redis (default: `false`)
- `REDIS_ADDRESSES` - Redis addresses (comma-separated)
- `REDIS_PASSWORD` - Redis password
- `REDIS_DB` - Redis database number (default: `0`)
- `REDIS_MAX_RETRIES` - Maximum retry attempts (default: `3`)
- `REDIS_POOL_SIZE` - Connection pool size (default: `10`)
- `REDIS_MIN_IDLE_CONNS` - Minimum idle connections (default: `5`)

### Redis Cluster
- `REDIS_CLUSTER_ENABLED` - Enable Redis Cluster mode (default: `false`)
- `REDIS_CLUSTER_ADDRESSES` - Cluster node addresses (comma-separated)

## Management API Settings

- `MANAGEMENT_ENABLED` - Enable management API (default: `false`)
- `MANAGEMENT_HOST` - Management API host (default: `127.0.0.1`)
- `MANAGEMENT_PORT` - Management API port (default: `9090`)
- `MANAGEMENT_AUTH_ENABLED` - Enable management auth (default: `true`)
- `MANAGEMENT_AUTH_TYPE` - Auth type: `basic` or `token` (default: `basic`)
- `MANAGEMENT_USERNAME` - Basic auth username (default: `admin`)
- `MANAGEMENT_PASSWORD` - Basic auth password
- `MANAGEMENT_TOKEN` - Bearer token for token auth

## CORS Settings

- `CORS_ENABLED` - Enable CORS (default: `false`)
- `CORS_ALLOW_ORIGINS` - Allowed origins (comma-separated)
- `CORS_ALLOW_METHODS` - Allowed methods (comma-separated)
- `CORS_ALLOW_HEADERS` - Allowed headers (comma-separated)
- `CORS_EXPOSE_HEADERS` - Exposed headers (comma-separated)
- `CORS_ALLOW_CREDENTIALS` - Allow credentials (default: `true`)
- `CORS_MAX_AGE` - Preflight cache duration in seconds (default: `86400`)

## Advanced Features

### OpenAPI Dynamic Loading
- `OPENAPI_ENABLED` - Enable OpenAPI support (default: `false`)
- `OPENAPI_SPECS_DIRECTORY` - Directory for OpenAPI specs (default: `./specs`)
- `OPENAPI_WATCH_FILES` - Watch for file changes (default: `true`)
- `OPENAPI_RELOAD_INTERVAL` - Reload interval in seconds (default: `300`)
- `OPENAPI_DEFAULT_SERVICE` - Default backend service

### Versioning
- `VERSIONING_ENABLED` - Enable API versioning (default: `false`)
- `VERSIONING_STRATEGY` - Strategy: `header`, `path`, `query` (default: `header`)
- `VERSIONING_HEADER_NAME` - Version header name (default: `X-API-Version`)
- `VERSIONING_QUERY_PARAM` - Version query parameter (default: `version`)
- `VERSIONING_DEFAULT_VERSION` - Default version (default: `v1`)

### Hot Reload
- `CONFIG_WATCH_ENABLED` - Enable config hot reload (default: `false`)
- `CONFIG_WATCH_INTERVAL` - Watch interval in seconds (default: `5`)
- `CONFIG_RELOAD_DEBOUNCE` - Reload debounce in seconds (default: `2`)

## Security Settings

### Request Limits
- `REQUEST_TIMEOUT` - Default request timeout in seconds (default: `30`)
- `REQUEST_MAX_BODY_SIZE` - Maximum request body size in bytes
- `REQUEST_MAX_HEADER_SIZE` - Maximum header size in bytes

### Security Headers
- `SECURITY_HEADERS_ENABLED` - Add security headers (default: `true`)
- `SECURITY_HSTS_ENABLED` - Enable HSTS (default: `false`)
- `SECURITY_HSTS_MAX_AGE` - HSTS max age in seconds (default: `31536000`)
- `SECURITY_CSP_ENABLED` - Enable Content Security Policy (default: `false`)
- `SECURITY_CSP_POLICY` - CSP policy string

## Development Settings

- `DEV_MODE` - Enable development mode (default: `false`)
- `DEV_VERBOSE_LOGGING` - Enable verbose logging (default: `false`)
- `DEV_DISABLE_AUTH` - Disable authentication (default: `false`)
- `DEV_DISABLE_RATE_LIMIT` - Disable rate limiting (default: `false`)

## Example Usage

### Basic Configuration

```bash
# Core settings
export GATEWAY_PORT=8080
export GATEWAY_HOST=0.0.0.0

# Backend settings
export BACKEND_MAX_IDLE_CONNS=100
export BACKEND_DIAL_TIMEOUT=10

# Start gateway
./gateway -config gateway.yaml
```

### Production Configuration

```bash
# Core settings
export GATEWAY_PORT=443
export GATEWAY_TLS_ENABLED=true
export GATEWAY_TLS_CERT_FILE=/etc/certs/server.crt
export GATEWAY_TLS_KEY_FILE=/etc/certs/server.key

# Authentication
export JWT_ENABLED=true
export JWT_ISSUER=https://auth.example.com
export JWT_AUDIENCE=api.example.com
export JWT_JWKS_ENDPOINT=https://auth.example.com/.well-known/jwks.json

# Rate limiting with Redis
export RATELIMIT_ENABLED=true
export RATELIMIT_STORAGE=redis
export REDIS_ENABLED=true
export REDIS_ADDRESSES=redis.example.com:6379
export REDIS_PASSWORD=secret123

# Observability
export OTEL_ENABLED=true
export OTEL_SERVICE_NAME=api-gateway-prod
export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://otel.example.com:4318/v1/traces
export OTEL_API_KEY=your-api-key

# Management API
export MANAGEMENT_ENABLED=true
export MANAGEMENT_PASSWORD=admin-secret

# Start gateway
./gateway -config production.yaml
```

### Docker Compose Example

```yaml
# docker-compose.yml
version: '3.8'
services:
  gateway:
    image: gateway:latest
    environment:
      # Core
      - GATEWAY_PORT=8080
      
      # Service discovery
      - DOCKER_ENABLED=true
      - DOCKER_ENDPOINT=unix:///var/run/docker.sock
      
      # Authentication
      - JWT_ENABLED=true
      - JWT_SECRET=${JWT_SECRET}
      
      # Redis
      - REDIS_ENABLED=true
      - REDIS_ADDRESSES=redis:6379
      
      # Telemetry
      - OTEL_ENABLED=true
      - OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://jaeger:4318/v1/traces
    
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./configs:/etc/gateway
    
    ports:
      - "8080:8080"
    
    command: ["-config", "/etc/gateway/gateway.yaml"]
```

### Kubernetes Example

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-env
data:
  GATEWAY_PORT: "8080"
  K8S_ENABLED: "true"
  K8S_NAMESPACE: "default"
  OTEL_ENABLED: "true"
  OTEL_SERVICE_NAME: "api-gateway"
  
---
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: gateway-secrets
stringData:
  JWT_SECRET: "your-jwt-secret"
  REDIS_PASSWORD: "redis-password"
  MANAGEMENT_PASSWORD: "admin-password"
  OTEL_API_KEY: "telemetry-api-key"

---
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
spec:
  template:
    spec:
      containers:
      - name: gateway
        image: gateway:latest
        envFrom:
        - configMapRef:
            name: gateway-env
        - secretRef:
            name: gateway-secrets
```

## Best Practices

1. **Security**: Never commit sensitive environment variables. Use secrets management tools.

2. **Configuration Precedence**:
   - Environment variables override configuration file values
   - Command-line flags override environment variables

3. **Naming Convention**: Use consistent prefixes for related settings

4. **Documentation**: Document all custom environment variables in your deployment

5. **Validation**: The gateway validates required environment variables on startup

6. **Defaults**: Provide sensible defaults in configuration files

7. **12-Factor App**: Follow 12-factor principles for configuration management