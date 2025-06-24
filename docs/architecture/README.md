# Architecture Documentation

This directory contains the architectural documentation for the API Gateway project.

## Overview

The gateway follows the [C4 Model](https://c4model.com/) for architecture documentation, providing views at different levels of abstraction.

## Architecture Diagrams

### Gateway Overview
Shows the high-level architecture of the API Gateway.

![Gateway Overview](../../assets/images/architecture/gateway-overview.svg)

### Request Flow
Illustrates the request processing pipeline through the gateway.

![Request Flow](../../assets/images/architecture/request-flow.svg)

## Diagram Sources

The architecture diagrams are maintained as PlantUML files in this directory:

- `gateway-architecture.puml` - Container-level architecture
- `request-flow.puml` - Request processing sequence

These are automatically rendered to SVG format by GitHub Actions when changes are pushed.

## Design Principles

### 1. Separation of Concerns
- **Data Plane**: High-performance request processing path
- **Control Plane**: Configuration and service discovery management

### 2. Pipeline Architecture
Each request flows through a well-defined pipeline:
1. Protocol adaptation
2. Authentication
3. Rate limiting
4. Routing
5. Load balancing
6. Backend connection

### 3. Streaming First
- No request/response buffering
- Direct streaming from backend to client
- Minimal memory footprint

### 4. Extensibility
- Interface-driven design
- Pluggable middleware
- Protocol-agnostic core

## Component Responsibilities

### Protocol Adapters
- Parse incoming requests
- Handle protocol-specific features
- Convert to internal request format

### Authentication Middleware
- JWT token validation
- API key verification
- Request context enrichment

### Rate Limiter
- Token bucket implementation
- Per-route configuration
- Graceful degradation

### Router
- Path pattern matching
- Route configuration lookup
- Service selection

### Load Balancer
- Round-robin distribution
- Health-aware routing
- Sticky session support

### Backend Connectors
- Protocol-specific proxying
- Connection pooling
- Response streaming

## Security Considerations

### Authentication Flow
- Fail-closed by default
- Short-circuit on auth service unavailability
- No bypass mechanisms

### Service Discovery
- Strict allowlist of backend services
- No dynamic URL resolution from requests
- Regular endpoint validation

### Rate Limiting
- Per-client tracking
- Configurable burst allowance
- Redis-backed for distributed deployments

## Performance Characteristics

### Latency Budget
- Protocol parsing: ~0.5ms
- Authentication: ~1-2ms
- Rate limiting: ~0.5ms
- Routing: ~0.1ms
- Total overhead: <5ms

### Throughput
- 10K+ requests/second per instance
- Linear scaling with instances
- No shared state bottlenecks

## Deployment Architecture

### High Availability
- Multiple gateway instances
- External load balancer (L4)
- No single point of failure

### Scalability
- Horizontal scaling
- Stateless instances
- Shared-nothing architecture

## Monitoring and Observability

### Metrics
- Request rate by route
- Response time percentiles
- Error rates by type
- Backend health status

### Logging
- Structured JSON logs
- Request correlation IDs
- Configurable verbosity

### Tracing
- OpenTelemetry support
- Distributed trace context
- End-to-end visibility