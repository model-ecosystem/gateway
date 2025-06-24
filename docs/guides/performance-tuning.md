# Performance Tuning Guide

This guide provides comprehensive performance tuning recommendations for optimizing the gateway's throughput, latency, and resource utilization.

## Table of Contents

- [Benchmarking](#benchmarking)
- [Connection Management](#connection-management)
- [Memory Optimization](#memory-optimization)
- [CPU Optimization](#cpu-optimization)
- [Network Tuning](#network-tuning)
- [Caching Strategies](#caching-strategies)
- [Load Balancing Optimization](#load-balancing-optimization)
- [Middleware Performance](#middleware-performance)
- [Monitoring and Profiling](#monitoring-and-profiling)

## Benchmarking

### Baseline Performance Testing

Before tuning, establish baseline metrics:

```bash
# HTTP benchmarking with wrk
wrk -t12 -c400 -d30s --latency http://gateway:8080/api/test

# HTTP/2 benchmarking with h2load
h2load -n100000 -c100 -t10 http://gateway:8080/api/test

# gRPC benchmarking with ghz
ghz --insecure --proto api.proto --call api.Service/Method \
    -n 10000 -c 50 gateway:9090
```

### Key Metrics to Track

```yaml
gateway:
  telemetry:
    metrics:
      histograms:
        - name: request_duration
          buckets: [0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5]
        - name: backend_duration
          buckets: [0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5]
      counters:
        - name: requests_total
        - name: errors_total
        - name: connections_active
```

## Connection Management

### Frontend Connection Pool

```yaml
gateway:
  frontend:
    http:
      # Maximum concurrent connections
      maxConnections: 10000
      
      # Keep-alive settings
      keepAlive:
        enabled: true
        maxIdleConnections: 1000
        idleTimeout: 90s
      
      # Read/Write timeouts
      readTimeout: 30s
      writeTimeout: 30s
      idleTimeout: 120s
      
      # Header limits
      maxHeaderBytes: 1048576  # 1MB
      
      # HTTP/2 settings
      http2:
        enabled: true
        maxConcurrentStreams: 250
        maxFrameSize: 16384
        maxHeaderListSize: 8192
```

### Backend Connection Pool

```yaml
gateway:
  backend:
    http:
      # Connection pool per host
      maxIdleConns: 1000
      maxIdleConnsPerHost: 100
      maxConnsPerHost: 0  # unlimited
      
      # Timeouts
      dialTimeout: 10s
      keepAliveTimeout: 30s
      idleConnTimeout: 90s
      responseHeaderTimeout: 10s
      expectContinueTimeout: 1s
      
      # TCP settings
      tcp:
        noDelay: true  # Disable Nagle's algorithm
        keepAlive: 30s
        
      # Buffer sizes
      readBufferSize: 65536   # 64KB
      writeBufferSize: 65536  # 64KB
```

### WebSocket Optimization

```yaml
gateway:
  adapter:
    websocket:
      # Message buffer sizes
      readBufferSize: 4096
      writeBufferSize: 4096
      
      # Compression
      enableCompression: true
      compressionLevel: 1  # Fast compression
      
      # Timeouts
      handshakeTimeout: 10s
      
      # Connection limits
      maxMessageSize: 1048576  # 1MB
      maxConnections: 10000
```

## Memory Optimization

### Buffer Pool Configuration

```yaml
gateway:
  performance:
    bufferPool:
      enabled: true
      sizes:
        - size: 1024    # 1KB
          count: 10000
        - size: 4096    # 4KB
          count: 5000
        - size: 16384   # 16KB
          count: 1000
        - size: 65536   # 64KB
          count: 500
```

### Request Body Handling

```yaml
gateway:
  performance:
    request:
      # Stream large bodies instead of buffering
      maxInMemorySize: 1048576  # 1MB
      
      # Temporary file directory for large uploads
      tempDir: /tmp/gateway
      
      # Cleanup settings
      cleanupInterval: 5m
      cleanupAge: 1h
```

### Garbage Collection Tuning

```yaml
# In deployment configuration
env:
  - name: GOGC
    value: "100"  # Default GC target percentage
  - name: GOMEMLIMIT
    value: "4GiB"  # Memory limit
  - name: GOMAXPROCS
    value: "8"  # Number of CPU cores
```

## CPU Optimization

### Goroutine Pool

```yaml
gateway:
  performance:
    goroutinePool:
      # Worker pool for CPU-bound tasks
      workers: 100
      queueSize: 1000
      
      # Separate pools for different tasks
      pools:
        - name: compute
          workers: 50
          priority: high
        - name: io
          workers: 200
          priority: normal
```

### JSON Processing

```yaml
gateway:
  performance:
    json:
      # Use faster JSON library
      encoder: sonic  # sonic, standard
      decoder: sonic
      
      # Streaming for large responses
      streamThreshold: 10240  # 10KB
      
      # Pre-allocated buffers
      bufferSize: 4096
```

### Regex Caching

```yaml
gateway:
  performance:
    regex:
      # Cache compiled regexes
      cacheSize: 1000
      
      # Precompile common patterns
      precompile:
        - pattern: "^/api/v[0-9]+/"
        - pattern: "^Bearer [A-Za-z0-9-._~+/]+=*$"
```

## Network Tuning

### TCP Optimization

```yaml
gateway:
  network:
    tcp:
      # Socket options
      reusePort: true
      fastOpen: true
      noDelay: true
      
      # Buffer sizes (OS level)
      receiveBuffer: 2097152  # 2MB
      sendBuffer: 2097152     # 2MB
      
      # Congestion control
      congestionAlgorithm: bbr  # bbr, cubic
```

### HTTP/2 Tuning

```yaml
gateway:
  frontend:
    http2:
      # Connection settings
      initialWindowSize: 1048576  # 1MB
      initialConnWindowSize: 2097152  # 2MB
      
      # Frame settings
      maxFrameSize: 32768  # 32KB
      
      # Stream settings
      maxConcurrentStreams: 1000
      maxDecoderHeaderTableSize: 4096
      maxEncoderHeaderTableSize: 4096
      
      # Ping settings
      pingInterval: 30s
      pingTimeout: 10s
```

### TLS Optimization

```yaml
gateway:
  tls:
    # Session resumption
    sessionCache:
      enabled: true
      size: 10000
      ttl: 3600s
    
    # OCSP stapling
    ocsp:
      enabled: true
      cacheSize: 1000
      cacheTTL: 3600s
    
    # Cipher suites (prefer fast ones)
    cipherSuites:
      - TLS_AES_128_GCM_SHA256
      - TLS_CHACHA20_POLY1305_SHA256
      - TLS_AES_256_GCM_SHA384
    
    # TLS 1.3 only for better performance
    minVersion: "TLS1.3"
```

## Caching Strategies

### Response Caching

```yaml
gateway:
  cache:
    response:
      enabled: true
      
      # Memory cache
      memory:
        size: 1GB
        ttl: 300s
        
      # Disk cache for larger responses
      disk:
        enabled: true
        path: /var/cache/gateway
        maxSize: 10GB
        
      # Cache key generation
      key:
        includeHost: true
        includeMethod: true
        includeQuery: true
        includeHeaders: ["Accept", "Accept-Encoding"]
        
      # Vary headers
      vary: ["Accept-Encoding", "Accept"]
```

### Backend Response Caching

```yaml
gateway:
  router:
    rules:
      - id: cacheable-api
        path: /api/static/*
        serviceName: api-service
        cache:
          enabled: true
          ttl: 3600s
          staleWhileRevalidate: 60s
          staleIfError: 300s
          # Cache based on these headers
          varyBy:
            - Authorization
            - X-API-Version
```

### DNS Caching

```yaml
gateway:
  performance:
    dns:
      # Cache DNS lookups
      cache:
        enabled: true
        size: 10000
        ttl: 300s
        negativeTTL: 30s
      
      # DNS resolver settings
      resolver:
        timeout: 5s
        attempts: 3
        # Use multiple resolvers
        servers:
          - 8.8.8.8:53
          - 8.8.4.4:53
```

## Load Balancing Optimization

### Connection Reuse

```yaml
gateway:
  loadbalancer:
    # Prefer existing connections
    connectionReuse:
      enabled: true
      strategy: least_recently_used
      maxAge: 300s
      
    # Health check optimization
    healthCheck:
      # Passive health checking
      passive:
        enabled: true
        failureThreshold: 5
        successThreshold: 2
      
      # Reduce active check frequency
      active:
        interval: 30s
        fastInterval: 1s  # During failures
```

### Smart Routing

```yaml
gateway:
  loadbalancer:
    # Latency-based routing
    adaptiveRouting:
      enabled: true
      # Route based on p95 latency
      metric: latency_p95
      # Update every 10s
      updateInterval: 10s
      
    # Locality preference
    locality:
      enabled: true
      # Prefer same zone
      zonePreference: 2.0
      # Prefer same region
      regionPreference: 1.5
```

## Middleware Performance

### Middleware Ordering

```yaml
gateway:
  middleware:
    # Order matters for performance
    chain:
      - recovery      # Catch panics first
      - metrics       # Measure everything
      - cache         # Return cached responses early
      - ratelimit     # Reject over-limit requests
      - auth          # Authenticate
      - compression   # Compress responses
      - transform     # Transform last
```

### Conditional Middleware

```yaml
gateway:
  middleware:
    # Skip middleware for certain paths
    skipPaths:
      metrics: ["/health", "/ready"]
      auth: ["/public/*", "/health"]
      compression: ["/stream/*", "/ws/*"]
    
    # Apply middleware conditionally
    conditions:
      compression:
        minSize: 1024  # Only compress > 1KB
        contentTypes: ["text/*", "application/json"]
```

### Rate Limiting Optimization

```yaml
gateway:
  middleware:
    ratelimit:
      # Use sliding window for accuracy
      algorithm: sliding_window
      
      # Distributed rate limiting
      storage:
        type: redis
        redis:
          # Use pipelining
          pipeline: true
          pipelineSize: 100
          
      # Local cache for performance
      localCache:
        enabled: true
        size: 10000
        ttl: 1s
```

## Monitoring and Profiling

### Continuous Profiling

```yaml
gateway:
  profiling:
    enabled: true
    
    # CPU profiling
    cpu:
      enabled: true
      rate: 100  # Hz
      duration: 30s
      
    # Memory profiling
    memory:
      enabled: true
      interval: 5m
      
    # Goroutine profiling
    goroutine:
      enabled: true
      interval: 1m
    
    # Block profiling
    block:
      enabled: true
      rate: 1
```

### Performance Metrics

```yaml
gateway:
  telemetry:
    metrics:
      # Detailed histograms
      detailedHistograms:
        - path: /api/critical/*
          buckets: [0.0001, 0.0005, 0.001, 0.005, 0.01]
      
      # Custom metrics
      custom:
        - name: connection_pool_efficiency
          type: gauge
          help: "Connection reuse percentage"
```

### Real-time Monitoring

```yaml
gateway:
  monitoring:
    realtime:
      enabled: true
      
      # Alerts for performance degradation
      alerts:
        - name: high_latency
          condition: p99_latency > 500ms
          window: 1m
          
        - name: low_cache_hit_rate
          condition: cache_hit_rate < 0.8
          window: 5m
```

## Performance Checklist

### Before Deployment

- [ ] Run load tests to establish baseline
- [ ] Configure connection pools appropriately
- [ ] Enable HTTP/2 and connection reuse
- [ ] Set up response caching
- [ ] Configure compression
- [ ] Tune garbage collection
- [ ] Enable profiling endpoints

### After Deployment

- [ ] Monitor key metrics (latency, throughput, errors)
- [ ] Check connection pool utilization
- [ ] Verify cache hit rates
- [ ] Analyze slow query logs
- [ ] Review profiling data
- [ ] Adjust based on real traffic patterns

## Common Performance Issues

### High Latency

1. Check backend response times
2. Review connection pool settings
3. Analyze middleware overhead
4. Verify DNS resolution speed
5. Check for network congestion

### High CPU Usage

1. Profile CPU usage
2. Check for regex performance
3. Review JSON processing
4. Optimize middleware chain
5. Check for goroutine leaks

### High Memory Usage

1. Check for memory leaks
2. Review buffer sizes
3. Analyze cache usage
4. Check request body handling
5. Review connection pools

### Connection Errors

1. Increase connection limits
2. Check file descriptor limits
3. Review timeout settings
4. Verify backend capacity
5. Check for connection leaks