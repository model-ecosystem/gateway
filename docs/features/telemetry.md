# Telemetry

The gateway provides comprehensive observability through OpenTelemetry integration, supporting distributed tracing, metrics collection, and log correlation across your entire system.

## Overview

Telemetry features include:
- Distributed tracing with OpenTelemetry
- Automatic trace propagation
- Custom metrics collection
- Log correlation
- Multiple exporter support
- Performance monitoring
- Error tracking
- Service dependency mapping

## Configuration

### Basic Setup

```yaml
gateway:
  telemetry:
    enabled: true
    serviceName: "api-gateway"
    serviceVersion: "1.0.0"
    environment: "production"
    
    # Sampling
    sampling:
      type: "probabilistic"
      rate: 0.1  # 10% of requests
```

### Exporters

#### Jaeger Exporter

```yaml
gateway:
  telemetry:
    tracing:
      exporters:
        jaeger:
          enabled: true
          endpoint: "http://jaeger:14268/api/traces"
          # Or agent endpoint
          agentEndpoint: "jaeger:6831"
```

#### OTLP Exporter

```yaml
gateway:
  telemetry:
    tracing:
      exporters:
        otlp:
          enabled: true
          endpoint: "otel-collector:4317"
          headers:
            api-key: "${OTLP_API_KEY}"
          tls:
            enabled: true
            insecure: false
```

#### Zipkin Exporter

```yaml
gateway:
  telemetry:
    tracing:
      exporters:
        zipkin:
          enabled: true
          endpoint: "http://zipkin:9411/api/v2/spans"
```

## Distributed Tracing

### Automatic Instrumentation

The gateway automatically:
- Creates spans for each request
- Propagates trace context
- Records key attributes
- Tracks errors and status

### Trace Propagation

```yaml
gateway:
  telemetry:
    tracing:
      propagation:
        # Formats to extract/inject
        formats:
          - w3c       # W3C Trace Context
          - b3        # Zipkin B3
          - jaeger    # Uber Jaeger
          - baggage   # W3C Baggage
```

### Custom Spans

```yaml
gateway:
  telemetry:
    tracing:
      customSpans:
        - name: "authentication"
          attributes:
            - "auth.method"
            - "auth.user_id"
        
        - name: "rate_limiting"
          attributes:
            - "ratelimit.limit"
            - "ratelimit.remaining"
```

## Metrics Collection

### Built-in Metrics

```yaml
gateway:
  telemetry:
    metrics:
      enabled: true
      exporters:
        prometheus:
          enabled: true
          port: 9090
          path: /metrics
```

Available metrics:
- Request rate
- Request duration
- Response sizes
- Error rates
- Active connections
- Backend latency

### Custom Metrics

```yaml
gateway:
  telemetry:
    metrics:
      custom:
        - name: "cache_hit_rate"
          type: "gauge"
          description: "Cache hit rate percentage"
        
        - name: "auth_failures"
          type: "counter"
          description: "Authentication failures"
          labels:
            - "method"
            - "reason"
```

### Histogram Configuration

```yaml
gateway:
  telemetry:
    metrics:
      histograms:
        requestDuration:
          buckets: [0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10]
        
        responseSize:
          buckets: [100, 1000, 10000, 100000, 1000000]
```

## Trace Attributes

### Standard Attributes

Automatically collected:
```
http.method = "GET"
http.target = "/api/users"
http.host = "api.example.com"
http.scheme = "https"
http.status_code = 200
http.user_agent = "Mozilla/5.0..."
net.peer.ip = "192.168.1.100"
net.peer.port = 54321
```

### Custom Attributes

```yaml
gateway:
  telemetry:
    tracing:
      attributes:
        # Static attributes
        static:
          deployment.region: "us-east-1"
          deployment.zone: "us-east-1a"
        
        # Dynamic attributes from headers
        fromHeaders:
          - header: "X-Request-ID"
            attribute: "request.id"
          
          - header: "X-User-ID"
            attribute: "user.id"
        
        # From request context
        fromContext:
          - "route.id"
          - "service.name"
          - "service.version"
```

## Sampling Strategies

### Probabilistic Sampling

```yaml
gateway:
  telemetry:
    sampling:
      type: "probabilistic"
      rate: 0.1  # 10% of traces
```

### Adaptive Sampling

```yaml
gateway:
  telemetry:
    sampling:
      type: "adaptive"
      targetRate: 100  # 100 traces per second
      maxRate: 0.2     # Max 20% sampling
```

### Priority Sampling

```yaml
gateway:
  telemetry:
    sampling:
      type: "priority"
      rules:
        # Always sample errors
        - condition: "error"
          rate: 1.0
        
        # High sampling for critical paths
        - condition: "path:/api/payments/*"
          rate: 0.5
        
        # Low sampling for health checks
        - condition: "path:/health"
          rate: 0.001
        
        # Default
        - condition: "*"
          rate: 0.1
```

## Context Propagation

### Baggage Items

```yaml
gateway:
  telemetry:
    baggage:
      # Items to propagate
      items:
        - key: "user.id"
          from: "header:X-User-ID"
        
        - key: "session.id"
          from: "cookie:session"
        
        - key: "tenant.id"
          from: "jwt:tenant_id"
      
      # Max baggage size
      maxSize: 8192
```

### Correlation IDs

```yaml
gateway:
  telemetry:
    correlation:
      enabled: true
      header: "X-Correlation-ID"
      generateIfMissing: true
      
      # Add to logs
      logField: "correlation_id"
      
      # Add to traces
      traceAttribute: "correlation.id"
```

## Log Integration

### Structured Logging

```yaml
gateway:
  telemetry:
    logging:
      # Inject trace context into logs
      injectTraceContext: true
      
      fields:
        - "trace_id"
        - "span_id"
        - "trace_flags"
      
      # Log format
      format: "json"
```

### Log Correlation Example

```json
{
  "level": "info",
  "msg": "Request processed",
  "time": "2024-01-15T10:30:45Z",
  "trace_id": "7c3b1e0a8f65d4e2",
  "span_id": "a1b2c3d4",
  "correlation_id": "req-123456",
  "http.method": "GET",
  "http.path": "/api/users",
  "http.status": 200,
  "duration_ms": 45
}
```

## Performance Monitoring

### Latency Tracking

```yaml
gateway:
  telemetry:
    performance:
      # Track percentiles
      latencyPercentiles: [0.5, 0.9, 0.95, 0.99]
      
      # Slow request threshold
      slowRequestThreshold: 1s
      
      # Track slow requests
      slowRequests:
        enabled: true
        sampleRate: 1.0  # Sample all slow requests
```

### Resource Monitoring

```yaml
gateway:
  telemetry:
    resources:
      # System metrics
      system:
        enabled: true
        interval: 10s
      
      # Runtime metrics
      runtime:
        enabled: true
        interval: 10s
        metrics:
          - memory
          - goroutines
          - gc
```

## Error Tracking

### Error Attributes

```yaml
gateway:
  telemetry:
    errors:
      # Capture stack traces
      captureStackTrace: true
      
      # Error grouping
      grouping:
        - "error.type"
        - "http.route"
        
      # Sensitive data
      sanitize:
        headers:
          - "Authorization"
          - "Cookie"
        
        body:
          - "password"
          - "credit_card"
```

### Error Sampling

```yaml
gateway:
  telemetry:
    errors:
      sampling:
        # Always sample first occurrence
        firstOccurrence: true
        
        # Then sample rate
        rate: 0.1
        
        # But always sample critical errors
        alwaysSample:
          - "panic"
          - "fatal"
          - "5xx"
```

## Service Map

### Dependency Tracking

```yaml
gateway:
  telemetry:
    serviceMap:
      enabled: true
      
      # Track service dependencies
      trackDependencies: true
      
      # Include in traces
      attributes:
        - "service.name"
        - "service.version"
        - "service.instance"
```

## Best Practices

1. **Sampling Strategy**: Balance visibility with performance
2. **Attribute Selection**: Include relevant context without overhead
3. **Error Tracking**: Always sample errors and anomalies
4. **Correlation**: Use correlation IDs across services
5. **Performance**: Monitor telemetry overhead
6. **Security**: Sanitize sensitive data

## Integration Examples

### Complete Setup

```yaml
gateway:
  telemetry:
    enabled: true
    serviceName: "api-gateway"
    
    tracing:
      exporters:
        otlp:
          enabled: true
          endpoint: "otel-collector:4317"
      
      sampling:
        type: "adaptive"
        targetRate: 100
    
    metrics:
      exporters:
        prometheus:
          enabled: true
          port: 9090
    
    logging:
      injectTraceContext: true
      format: "json"
    
    attributes:
      static:
        environment: "production"
        region: "us-east-1"
```

### Development Setup

```yaml
gateway:
  telemetry:
    enabled: true
    serviceName: "api-gateway-dev"
    
    tracing:
      exporters:
        jaeger:
          enabled: true
          agentEndpoint: "localhost:6831"
      
      sampling:
        type: "probabilistic"
        rate: 1.0  # Sample everything in dev
    
    metrics:
      exporters:
        console:
          enabled: true
          interval: 10s
```

## Troubleshooting

### Debug Telemetry

```yaml
gateway:
  telemetry:
    debug:
      enabled: true
      logSpans: true
      logMetrics: true
      
  logging:
    modules:
      telemetry: debug
```

### Common Issues

1. **Missing Traces**: Check sampling rate and export configuration
2. **High Overhead**: Reduce sampling rate or attributes
3. **Context Loss**: Verify propagation formats match
4. **Export Failures**: Check network and authentication

This comprehensive telemetry system provides full observability into your gateway and backend services.