# Observability Guide

This guide covers the gateway's observability features including OpenTelemetry integration, distributed tracing, metrics collection, and active health monitoring.

## OpenTelemetry Integration

The gateway includes comprehensive OpenTelemetry support for distributed tracing and metrics collection.

### Configuration

```yaml
gateway:
  telemetry:
    enabled: true
    otlp:
      endpoint: "localhost:4317"
      insecure: true
      timeout: 10
      headers:
        api-key: "your-api-key"
    service:
      name: "api-gateway"
      version: "1.0.0"
      environment: "production"
    sampling:
      ratio: 1.0  # Sample 100% of requests
```

### Distributed Tracing

Every request through the gateway is automatically traced with:

- Request metadata (method, path, status)
- Backend service calls
- Middleware execution times
- Error details and stack traces

Example trace attributes:
- `http.method`: HTTP method
- `http.target`: Request path
- `http.status_code`: Response status
- `net.peer.addr`: Client address
- `service.name`: Backend service name
- `service.instance`: Selected instance ID

### Metrics Collection

The gateway collects the following metrics:

#### Request Metrics
- `gateway.requests.total`: Total request count (counter)
- `gateway.requests.duration`: Request duration in ms (histogram)
- `gateway.requests.size`: Request body size (histogram)
- `gateway.responses.size`: Response body size (histogram)

#### Error Metrics
- `gateway.errors.total`: Total error count by type (counter)

#### Backend Metrics
- `gateway.backend.requests.total`: Backend requests by service (counter)
- `gateway.backend.requests.duration`: Backend request duration (histogram)
- `gateway.backend.errors.total`: Backend errors by service (counter)

#### Circuit Breaker Metrics
- `gateway.circuit_breaker.state`: Current state (0=closed, 1=open, 2=half-open)
- `gateway.circuit_breaker.failures`: Consecutive failures
- `gateway.circuit_breaker.successes`: Consecutive successes

#### Health Check Metrics
- `gateway.health.checks.total`: Total health checks performed
- `gateway.health.service.instances`: Number of instances per service
- `gateway.health.service.healthy`: Number of healthy instances per service

### Context Propagation

The gateway automatically propagates trace context using W3C Trace Context standard:
- Incoming `traceparent` and `tracestate` headers are parsed
- Trace context is propagated to backend services
- New spans are created for each gateway operation

## Active Health Monitoring

The gateway includes active health monitoring that periodically checks backend service health.

### Configuration

```yaml
gateway:
  health:
    enabled: true
    interval: 30s
    timeout: 5s
    path: "/health"
    checks:
      - type: http
        services: ["api-service", "auth-service"]
        path: "/health"
        expectedStatus: 200
      - type: tcp
        services: ["cache-service"]
      - type: grpc
        services: ["grpc-service"]
        service: "grpc.health.v1.Health"
```

### Health Check Types

1. **HTTP Health Checks**
   - Sends GET requests to specified path
   - Validates response status code
   - Supports custom paths per service

2. **TCP Health Checks**
   - Attempts TCP connection to service
   - Validates connection establishment
   - Lightweight connectivity check

3. **gRPC Health Checks**
   - Uses standard gRPC health checking protocol
   - Supports service-specific health status
   - Compatible with grpc-health-probe

### Health Status Updates

- Services are marked unhealthy after failed checks
- Load balancers automatically exclude unhealthy instances
- Health status changes are logged and traced
- Metrics track instance health over time

## Monitoring Best Practices

### 1. Set Up Proper Sampling

For production environments, adjust sampling ratio to balance observability with performance:

```yaml
telemetry:
  sampling:
    ratio: 0.1  # Sample 10% of requests
```

### 2. Use Structured Logging

The gateway uses structured logging with trace correlation:

```
2025/06/22 INFO request completed 
  component=http 
  trace_id=7b5e3d4a9c2f1e8d 
  span_id=3a2b1c9d8e7f6a5b
  duration=45.2ms 
  status=200
```

### 3. Monitor Key Metrics

Set up alerts for:
- Error rate > 1%
- P99 latency > 1s
- Circuit breaker open state
- Unhealthy instance ratio > 50%

### 4. Trace Sampling Strategies

Consider using head-based sampling for:
- Error responses (always sample)
- Slow requests (latency > threshold)
- Specific routes or services

### 5. Export Telemetry Data

The gateway supports OTLP export to:
- Jaeger
- Zipkin (via OTLP collector)
- Prometheus (metrics)
- Grafana Tempo
- Cloud providers (AWS X-Ray, GCP Trace, etc.)

## Example: Local Development Setup

1. Start Jaeger for tracing:
```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest
```

2. Configure gateway:
```yaml
gateway:
  telemetry:
    enabled: true
    otlp:
      endpoint: "localhost:4317"
      insecure: true
    service:
      name: "gateway-dev"
      version: "dev"
```

3. View traces at http://localhost:16686

## Performance Impact

The telemetry implementation is designed for minimal overhead:
- Tracing adds ~50μs per request
- Metrics collection adds ~10μs per request
- Context propagation is zero-allocation
- Sampling reduces data volume
- Async export prevents blocking

## Troubleshooting

### No Traces Appearing
1. Check OTLP endpoint connectivity
2. Verify sampling ratio > 0
3. Check for export errors in logs
4. Ensure trace headers are propagated

### High Memory Usage
1. Reduce sampling ratio
2. Decrease metric cardinality
3. Configure batch export settings
4. Monitor span creation rate

### Missing Metrics
1. Verify metrics are enabled
2. Check OTLP metric export support
3. Review metric names and labels
4. Ensure Prometheus scraping works