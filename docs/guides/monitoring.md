# Monitoring Guide

This guide covers comprehensive monitoring setup for the gateway, including metrics collection, distributed tracing, logging, and alerting.

## Table of Contents

- [Overview](#overview)
- [Metrics](#metrics)
- [Distributed Tracing](#distributed-tracing)
- [Logging](#logging)
- [Health Monitoring](#health-monitoring)
- [Dashboards](#dashboards)
- [Alerting](#alerting)
- [SLI/SLO Configuration](#slislo-configuration)
- [Troubleshooting](#troubleshooting)

## Overview

The gateway provides comprehensive observability through:
- OpenTelemetry-based metrics and tracing
- Structured JSON logging
- Real-time health monitoring
- Prometheus metrics exposition
- Distributed trace correlation

## Metrics

### Enabling Metrics

```yaml
gateway:
  telemetry:
    metrics:
      enabled: true
      
      # Prometheus endpoint
      prometheus:
        enabled: true
        path: /metrics
        port: 9090
        
      # OTLP export
      otlp:
        enabled: true
        endpoint: otel-collector:4317
        interval: 10s
        timeout: 5s
```

### Core Metrics

#### Request Metrics

```yaml
# Total requests
gateway_requests_total{method="GET",path="/api/*",status="200"}

# Request duration histogram
gateway_request_duration_seconds{method="GET",path="/api/*",quantile="0.99"}

# Active requests
gateway_requests_active{method="GET",path="/api/*"}

# Request body size
gateway_request_body_bytes{method="POST",path="/api/*"}

# Response body size
gateway_response_body_bytes{method="GET",path="/api/*",status="200"}
```

#### Backend Metrics

```yaml
# Backend request duration
gateway_backend_duration_seconds{service="api-service",instance="10.0.0.1:8080"}

# Backend connection pool
gateway_backend_connections_active{service="api-service"}
gateway_backend_connections_idle{service="api-service"}

# Backend errors
gateway_backend_errors_total{service="api-service",error="timeout"}
```

#### System Metrics

```yaml
# Go runtime metrics
go_goroutines
go_memstats_alloc_bytes
go_gc_duration_seconds

# Process metrics
process_cpu_seconds_total
process_resident_memory_bytes
process_open_fds
```

### Custom Metrics

```yaml
gateway:
  telemetry:
    metrics:
      custom:
        - name: business_transactions_total
          type: counter
          help: "Total business transactions"
          labels: ["type", "status"]
          
        - name: cache_hit_ratio
          type: gauge
          help: "Cache hit ratio"
          labels: ["cache_type"]
          
        - name: auth_latency_seconds
          type: histogram
          help: "Authentication latency"
          buckets: [0.001, 0.005, 0.01, 0.05, 0.1]
```

### Metric Aggregation

```yaml
gateway:
  telemetry:
    metrics:
      aggregation:
        # Cardinality limits
        maxCardinality: 10000
        
        # Label sanitization
        sanitizeLabels:
          path:
            # Group similar paths
            - pattern: "/api/users/[0-9]+"
              replacement: "/api/users/{id}"
            - pattern: "/api/orders/[a-f0-9-]+"
              replacement: "/api/orders/{uuid}"
```

## Distributed Tracing

### Tracing Configuration

```yaml
gateway:
  telemetry:
    tracing:
      enabled: true
      
      # Sampling configuration
      sampling:
        type: adaptive  # adaptive, probabilistic, always
        rate: 0.1      # 10% for probabilistic
        
        # Adaptive sampling
        adaptive:
          maxTracesPerSecond: 100
          targetSamplesPerSecond: 10
          
      # Trace export
      exporter:
        type: otlp
        endpoint: jaeger:4317
        headers:
          X-Trace-Source: gateway
        
      # Trace propagation
      propagation:
        - tracecontext
        - baggage
        - b3multi
```

### Trace Enrichment

```yaml
gateway:
  telemetry:
    tracing:
      enrichment:
        # Add standard attributes
        attributes:
          service.name: api-gateway
          service.version: "${VERSION}"
          deployment.environment: "${ENV}"
          
        # Add request attributes
        request:
          - http.method
          - http.url
          - http.target
          - http.host
          - http.scheme
          - http.user_agent
          - http.request_content_length
          
        # Add response attributes
        response:
          - http.status_code
          - http.response_content_length
```

### Trace Context Propagation

```yaml
gateway:
  router:
    rules:
      - id: traced-api
        path: /api/*
        serviceName: api-service
        tracing:
          # Inject trace context
          injectContext: true
          
          # Custom span attributes
          spanAttributes:
            route.id: traced-api
            route.version: v1
            
          # Baggage items
          baggage:
            - key: user.tier
              value: "${header:X-User-Tier}"
            - key: request.priority
              value: "${header:X-Priority}"
```

## Logging

### Structured Logging

```yaml
gateway:
  logging:
    level: info
    format: json
    
    # Output configuration
    outputs:
      - type: stdout
        format: json
        level: info
        
      - type: file
        path: /var/log/gateway/gateway.log
        format: json
        level: debug
        rotation:
          maxSize: 100MB
          maxAge: 7d
          maxBackups: 5
          compress: true
          
      - type: syslog
        network: tcp
        address: syslog:514
        facility: local0
        level: warning
```

### Log Fields

```yaml
gateway:
  logging:
    fields:
      # Standard fields
      standard:
        - timestamp
        - level
        - message
        - logger
        
      # Request fields
      request:
        - request_id
        - method
        - path
        - remote_addr
        - user_agent
        - trace_id
        - span_id
        
      # Response fields
      response:
        - status_code
        - duration_ms
        - bytes_sent
        
      # Error fields
      error:
        - error_type
        - error_message
        - stack_trace
```

### Log Sampling

```yaml
gateway:
  logging:
    sampling:
      # Sample logs for high-volume endpoints
      rules:
        - path: /health
          rate: 0.01  # 1%
        - path: /metrics
          rate: 0.01
        - status: 200
          rate: 0.1   # 10% of successful requests
        - status: 5xx
          rate: 1.0   # 100% of errors
```

## Health Monitoring

### Health Endpoints

```yaml
gateway:
  health:
    endpoints:
      # Liveness probe
      liveness:
        path: /healthz
        checks:
          - type: goroutines
            threshold: 10000
          - type: memory
            threshold: 90  # percentage
            
      # Readiness probe
      readiness:
        path: /ready
        checks:
          - type: backend_connectivity
            timeout: 5s
          - type: config_loaded
          - type: middleware_ready
```

### Component Health

```yaml
gateway:
  health:
    components:
      # Monitor internal components
      - name: router
        check:
          type: internal
          interval: 10s
          
      - name: service_registry
        check:
          type: internal
          interval: 10s
          
      - name: rate_limiter
        check:
          type: redis
          address: redis:6379
          interval: 10s
```

## Dashboards

### Grafana Dashboard Configuration

```json
{
  "dashboard": {
    "title": "API Gateway Monitoring",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [{
          "expr": "sum(rate(gateway_requests_total[5m])) by (method, status)"
        }]
      },
      {
        "title": "Latency Percentiles",
        "targets": [{
          "expr": "histogram_quantile(0.99, rate(gateway_request_duration_seconds_bucket[5m]))"
        }]
      },
      {
        "title": "Error Rate",
        "targets": [{
          "expr": "sum(rate(gateway_requests_total{status=~\"5..\"}[5m])) / sum(rate(gateway_requests_total[5m]))"
        }]
      },
      {
        "title": "Backend Health",
        "targets": [{
          "expr": "up{job=\"gateway-backends\"}"
        }]
      }
    ]
  }
}
```

### Key Dashboard Panels

1. **Traffic Overview**
   - Request rate by endpoint
   - Response time distribution
   - Error rate by status code
   - Active connections

2. **Performance Metrics**
   - Latency percentiles (p50, p95, p99)
   - Backend response times
   - Cache hit rates
   - Connection pool utilization

3. **Resource Usage**
   - CPU utilization
   - Memory usage
   - Goroutine count
   - File descriptor usage

4. **Business Metrics**
   - API usage by client
   - Rate limit violations
   - Authentication failures
   - Circuit breaker status

## Alerting

### Alert Rules

```yaml
groups:
  - name: gateway_alerts
    interval: 30s
    rules:
      # High error rate
      - alert: HighErrorRate
        expr: |
          sum(rate(gateway_requests_total{status=~"5.."}[5m])) 
          / sum(rate(gateway_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }}"
          
      # High latency
      - alert: HighLatency
        expr: |
          histogram_quantile(0.99, 
            rate(gateway_request_duration_seconds_bucket[5m])
          ) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          description: "p99 latency is {{ $value }}s"
          
      # Backend down
      - alert: BackendDown
        expr: |
          gateway_backend_healthy == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Backend {{ $labels.service }} is down"
          
      # Memory pressure
      - alert: HighMemoryUsage
        expr: |
          go_memstats_alloc_bytes / go_memstats_sys_bytes > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
```

### Alert Routing

```yaml
gateway:
  alerting:
    providers:
      - type: pagerduty
        apiKey: ${PAGERDUTY_API_KEY}
        serviceKey: ${PAGERDUTY_SERVICE_KEY}
        severity:
          - critical
          
      - type: slack
        webhook: ${SLACK_WEBHOOK_URL}
        channel: "#gateway-alerts"
        severity:
          - warning
          - critical
          
      - type: email
        smtp:
          host: smtp.example.com
          port: 587
        recipients:
          - ops@example.com
        severity:
          - critical
```

## SLI/SLO Configuration

### Service Level Indicators

```yaml
gateway:
  sli:
    # Availability SLI
    availability:
      metric: |
        sum(rate(gateway_requests_total{status!~"5.."}[5m]))
        / sum(rate(gateway_requests_total[5m]))
      
    # Latency SLI
    latency:
      metric: |
        histogram_quantile(0.95,
          rate(gateway_request_duration_seconds_bucket[5m])
        ) < 0.5
      
    # Error rate SLI
    errors:
      metric: |
        sum(rate(gateway_requests_total{status=~"5.."}[5m]))
        / sum(rate(gateway_requests_total[5m])) < 0.01
```

### Service Level Objectives

```yaml
gateway:
  slo:
    # 99.9% availability
    availability:
      target: 0.999
      window: 30d
      
    # 95% of requests < 500ms
    latency:
      target: 0.95
      threshold: 500ms
      window: 30d
      
    # Error rate < 1%
    errors:
      target: 0.01
      window: 30d
```

### Error Budget Monitoring

```yaml
gateway:
  errorBudget:
    # Alert when 50% of error budget consumed
    alerts:
      - name: ErrorBudget50
        threshold: 0.5
        severity: warning
        
      - name: ErrorBudget80
        threshold: 0.8
        severity: critical
    
    # Automated responses
    automation:
      - trigger: ErrorBudget80
        action: enable_circuit_breakers
      - trigger: ErrorBudget90
        action: enable_read_only_mode
```

## Troubleshooting

### Debug Logging

```yaml
gateway:
  debug:
    # Enable debug mode
    enabled: true
    
    # Detailed logging for specific components
    components:
      - router
      - loadbalancer
      - middleware.auth
      
    # Request/response logging
    requests:
      enabled: true
      includeBody: true
      maxBodySize: 1024
      paths:
        - /api/debug/*
```

### Trace Sampling Override

```yaml
gateway:
  router:
    rules:
      - id: debug-route
        path: /api/debug/*
        tracing:
          # Force trace this route
          alwaysSample: true
          # Add debug attributes
          debugMode: true
```

### Metric Cardinality Analysis

```bash
# Find high cardinality metrics
curl -s http://gateway:9090/metrics | \
  grep -E "^[a-zA-Z_]+" | \
  cut -d'{' -f1 | \
  sort | uniq -c | \
  sort -rn | head -20
```

### Common Issues

1. **Missing Metrics**
   - Check telemetry configuration
   - Verify metrics endpoint accessibility
   - Check for cardinality limits

2. **Incomplete Traces**
   - Verify trace propagation headers
   - Check sampling configuration
   - Ensure all services are instrumented

3. **Log Volume Issues**
   - Review log sampling rules
   - Check log rotation settings
   - Consider log aggregation

4. **Alert Fatigue**
   - Review alert thresholds
   - Implement alert grouping
   - Use appropriate time windows