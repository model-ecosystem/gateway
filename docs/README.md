# Gateway Documentation

Welcome to the Gateway documentation. This guide will help you understand, configure, and operate the gateway effectively.

## 📚 Documentation Structure

### Getting Started
- **[Getting Started Guide](guides/getting-started.md)** - Quick setup and basic usage
- **[Configuration Guide](guides/configuration.md)** - Complete configuration reference
- **[Environment Variables](guides/environment-variables.md)** - Environment variable reference

### Feature Guides
- **[Authentication](guides/authentication.md)** - JWT and API key authentication
- **[Rate Limiting](guides/rate-limiting.md)** - Request rate limiting with memory and Redis backends
- **[TLS Setup](guides/tls-setup.md)** - Configuring TLS/mTLS for frontend and backend
- **[gRPC Support](guides/grpc.md)** - gRPC backends and HTTP transcoding
- **[Health Checks](guides/health-checks.md)** - Backend health monitoring
- **[Metrics & Monitoring](guides/metrics.md)** - OpenTelemetry integration and observability
- **[Resilience Patterns](guides/resilience.md)** - Circuit breakers, retries, and load balancing

### Advanced Features
- **[OAuth2/OIDC](features/oauth2-oidc.md)** - OAuth2 and OpenID Connect support
- **[OpenAPI](features/openapi.md)** - OpenAPI specification and dynamic routing
- **[RBAC](features/rbac.md)** - Role-based access control
- **[Circuit Breaker](features/circuit-breaker.md)** - Advanced circuit breaker patterns
- **[Transformations](features/transform.md)** - Request/response transformations
- **[Hot Reload](features/hot-reload.md)** - Configuration hot reloading
- **[Management API](features/management-api.md)** - Runtime management endpoints
- **[Multi-Version Support](features/multi-version-support.md)** - API versioning
- **[Kubernetes Discovery](features/kubernetes-discovery.md)** - K8s service discovery
- **[Docker Compose Discovery](features/docker-compose-discovery.md)** - Docker Compose integration

### Architecture
- **[Architecture Overview](architecture/overview.md)** - System design and components
- **[Session Affinity](architecture/session-affinity.md)** - Sticky sessions design

### Deployment
- **[Deployment Guide](guides/deployment.md)** - Production deployment best practices

## 🎯 Quick Links

### Common Tasks
1. **Basic HTTP Gateway**: Start with [Getting Started](guides/getting-started.md)
2. **Add Authentication**: See [Authentication Guide](guides/authentication.md)
3. **Enable Rate Limiting**: Check [Rate Limiting Guide](guides/rate-limiting.md)
4. **Setup TLS**: Follow [TLS Setup Guide](guides/tls-setup.md)
5. **Monitor Performance**: Read [Metrics Guide](guides/metrics.md)

### Example Configurations
Find working examples in:
- `/configs/base/gateway.yaml` - Basic HTTP routing configuration
- `/configs/examples/complete.yaml` - Complete configuration with all options
- `/configs/examples/`:
  - `auth.yaml` - JWT and API Key authentication
  - `tls.yaml` - TLS/mTLS configuration
  - `websocket.yaml` - WebSocket support
  - `sse.yaml` - Server-Sent Events
  - `grpc.yaml` - gRPC backend and transcoding
  - `docker.yaml` - Docker service discovery
  - `session-affinity.yaml` - Sticky sessions
  - `ratelimit.yaml` - Rate limiting configuration
  - `circuit-breaker.yaml` - Circuit breaker patterns
  - `retry.yaml` - Retry configuration

## 🔍 Finding Information

### By Protocol
- **HTTP/REST**: [Getting Started](guides/getting-started.md), [Configuration](guides/configuration.md)
- **WebSocket**: [Configuration Guide](guides/configuration.md#websocket)
- **SSE**: [Configuration Guide](guides/configuration.md#sse)
- **gRPC**: [gRPC Support Guide](guides/grpc.md)

### By Feature
- **Security**: [Authentication](guides/authentication.md), [TLS Setup](guides/tls-setup.md)
- **Performance**: [Rate Limiting](guides/rate-limiting.md), [Resilience](guides/resilience.md)
- **Monitoring**: [Metrics](guides/metrics.md), [Health Checks](guides/health-checks.md)

### By Deployment Stage
1. **Development**: [Getting Started](guides/getting-started.md)
2. **Testing**: [Health Checks](guides/health-checks.md), [Metrics](guides/metrics.md)
3. **Production**: [Deployment](guides/deployment.md), [TLS Setup](guides/tls-setup.md)

## 📖 Reading Order

For new users, we recommend this reading order:
1. [Getting Started Guide](guides/getting-started.md)
2. [Architecture Overview](architecture/overview.md)
3. [Configuration Guide](guides/configuration.md)
4. Feature guides based on your needs

## 🤝 Contributing

Documentation improvements are welcome! When contributing:
- Keep language clear and concise
- Include practical examples
- Test all configuration samples
- Update relevant guides when adding features

## 📞 Support

For questions or issues:
- Check the relevant guide first
- Review example configurations
- Search existing GitHub issues
- Open a new issue with details