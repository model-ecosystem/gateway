# Troubleshooting Guide

This guide helps diagnose and resolve common issues with the gateway.

## Table of Contents

- [Diagnostic Tools](#diagnostic-tools)
- [Common Issues](#common-issues)
- [Performance Problems](#performance-problems)
- [Connection Issues](#connection-issues)
- [Configuration Problems](#configuration-problems)
- [Authentication/Authorization Issues](#authenticationauthorization-issues)
- [Backend Service Issues](#backend-service-issues)
- [Memory and Resource Issues](#memory-and-resource-issues)
- [Debugging Techniques](#debugging-techniques)
- [Emergency Procedures](#emergency-procedures)

## Diagnostic Tools

### Built-in Diagnostics

```yaml
gateway:
  diagnostics:
    enabled: true
    endpoints:
      # Runtime profiling
      pprof:
        enabled: true
        path: /debug/pprof
        auth: required
        
      # Configuration dump
      config:
        enabled: true
        path: /debug/config
        sanitize: true  # Remove secrets
        
      # Route information
      routes:
        enabled: true
        path: /debug/routes
        
      # Health details
      health:
        enabled: true
        path: /debug/health
        verbose: true
```

### Debug Logging

Enable debug logging for specific components:

```bash
# Via environment variable
GATEWAY_LOG_LEVEL=debug ./gateway

# Via API
curl -X POST http://gateway:9001/debug/log-level \
  -H "Content-Type: application/json" \
  -d '{"level": "debug", "components": ["router", "auth"]}'
```

### Trace Requests

Force tracing for specific requests:

```bash
# Add debug headers
curl http://gateway:8080/api/test \
  -H "X-Debug: true" \
  -H "X-Force-Trace: true" \
  -H "X-Request-ID: debug-12345"
```

## Common Issues

### Gateway Won't Start

1. **Port Already in Use**
   ```bash
   # Check if port is in use
   lsof -i :8080
   netstat -an | grep 8080
   
   # Solution: Change port or kill process
   gateway:
     frontend:
       http:
         port: 8081
   ```

2. **Configuration Errors**
   ```bash
   # Validate configuration
   ./gateway validate -c config.yaml
   
   # Common issues:
   # - YAML syntax errors
   # - Missing required fields
   # - Invalid values
   ```

3. **Permission Issues**
   ```bash
   # Check file permissions
   ls -la /etc/gateway/
   
   # Fix permissions
   sudo chown -R gateway:gateway /etc/gateway/
   sudo chmod 640 /etc/gateway/*.yaml
   ```

### No Response from Gateway

1. **Check Gateway Health**
   ```bash
   curl -v http://localhost:8080/health
   ```

2. **Check Logs**
   ```bash
   # Check for panic or fatal errors
   tail -f /var/log/gateway/gateway.log | grep -E "panic|fatal|error"
   ```

3. **Verify Network Connectivity**
   ```bash
   # Test from inside container/host
   curl -v http://localhost:8080/
   
   # Test from outside
   telnet gateway-host 8080
   ```

## Performance Problems

### High Latency

1. **Identify Slow Endpoints**
   ```promql
   # Find slowest endpoints
   topk(10, 
     histogram_quantile(0.99, 
       rate(gateway_request_duration_seconds_bucket[5m])
     ) by (path)
   )
   ```

2. **Check Backend Performance**
   ```bash
   # Enable backend timing logs
   gateway:
     logging:
       backendDuration: true
   ```

3. **Analyze Middleware Overhead**
   ```yaml
   gateway:
     telemetry:
       metrics:
         middlewareDuration: true
   ```

### High CPU Usage

1. **Profile CPU Usage**
   ```bash
   # Get CPU profile
   curl http://localhost:9001/debug/pprof/profile?seconds=30 > cpu.prof
   go tool pprof cpu.prof
   
   # Interactive analysis
   (pprof) top10
   (pprof) web
   ```

2. **Check for Regex Performance**
   ```yaml
   # Use exact matches where possible
   gateway:
     router:
       rules:
         - path: /api/v1/users  # Faster than regex
   ```

3. **Review JSON Processing**
   ```yaml
   # Use streaming for large payloads
   gateway:
     performance:
       json:
         streamThreshold: 10KB
   ```

## Connection Issues

### Connection Refused

1. **Verify Service is Running**
   ```bash
   systemctl status gateway
   docker ps | grep gateway
   ```

2. **Check Firewall Rules**
   ```bash
   # List firewall rules
   iptables -L -n
   ufw status
   
   # Allow gateway port
   ufw allow 8080/tcp
   ```

3. **Check Binding Address**
   ```yaml
   gateway:
     frontend:
       http:
         host: "0.0.0.0"  # Bind to all interfaces
   ```

### Connection Timeout

1. **Check Backend Health**
   ```bash
   # View backend status
   curl http://localhost:9001/services | jq
   ```

2. **Increase Timeouts**
   ```yaml
   gateway:
     backend:
       http:
         dialTimeout: 30s
         responseHeaderTimeout: 30s
     router:
       rules:
         - id: slow-api
           timeout: 60s
   ```

3. **Check Connection Limits**
   ```bash
   # Check file descriptor limits
   ulimit -n
   
   # Increase limits
   ulimit -n 65536
   ```

### Connection Reset

1. **Check for Panics**
   ```bash
   grep -i panic /var/log/gateway/gateway.log
   ```

2. **Enable Recovery Middleware**
   ```yaml
   gateway:
     middleware:
       recovery:
         enabled: true
         logPanics: true
         returnError: true
   ```

## Configuration Problems

### Configuration Not Loading

1. **Check File Path**
   ```bash
   # Verify file exists
   ls -la /etc/gateway/config.yaml
   
   # Check environment variable
   echo $GATEWAY_CONFIG_PATH
   ```

2. **Validate YAML Syntax**
   ```bash
   # Online validator or
   python -c "import yaml; yaml.safe_load(open('config.yaml'))"
   ```

3. **Check for Merge Conflicts**
   ```yaml
   gateway:
     config:
       debug:
         showMerged: true
         logSources: true
   ```

### Hot Reload Not Working

1. **Verify Watch is Enabled**
   ```yaml
   gateway:
     config:
       reload:
         enabled: true
       sources:
         - type: file
           path: /etc/gateway/config.yaml
           watch: true
   ```

2. **Check File System Events**
   ```bash
   # Test inotify (Linux)
   inotifywait -m /etc/gateway/
   ```

## Authentication/Authorization Issues

### 401 Unauthorized

1. **Check Token Validity**
   ```bash
   # Decode JWT
   echo $TOKEN | cut -d. -f2 | base64 -d | jq
   
   # Check expiration
   exp=$(echo $TOKEN | cut -d. -f2 | base64 -d | jq -r .exp)
   date -d @$exp
   ```

2. **Verify Auth Configuration**
   ```yaml
   gateway:
     middleware:
       auth:
         jwt:
           secret: ${JWT_SECRET}
           algorithm: RS256
           publicKeyFile: /certs/public.key
   ```

3. **Check Auth Headers**
   ```bash
   curl -v http://gateway:8080/api/test \
     -H "Authorization: Bearer $TOKEN"
   ```

### 403 Forbidden

1. **Check RBAC Rules**
   ```yaml
   gateway:
     middleware:
       authz:
         rbac:
           debug: true  # Log permission checks
   ```

2. **Verify User Permissions**
   ```bash
   # Check token claims
   echo $TOKEN | cut -d. -f2 | base64 -d | jq '.permissions'
   ```

## Backend Service Issues

### Service Unavailable

1. **Check Service Registration**
   ```bash
   curl http://localhost:9001/services/api-service | jq
   ```

2. **Verify Health Checks**
   ```yaml
   gateway:
     health:
       debug: true
       logResults: true
   ```

3. **Check Circuit Breaker Status**
   ```bash
   curl http://localhost:9001/circuit-breakers | jq
   ```

### Wrong Backend Selected

1. **Enable Load Balancer Debugging**
   ```yaml
   gateway:
     loadbalancer:
       debug:
         logSelection: true
         logReason: true
   ```

2. **Check Session Affinity**
   ```bash
   # Check cookie
   curl -v http://gateway:8080/api/test \
     -H "Cookie: GATEWAY_SESSION=xyz123"
   ```

## Memory and Resource Issues

### Out of Memory

1. **Check Memory Usage**
   ```bash
   # Get memory stats
   curl http://localhost:9001/debug/pprof/heap > heap.prof
   go tool pprof heap.prof
   
   # Top memory consumers
   (pprof) top
   ```

2. **Identify Memory Leaks**
   ```bash
   # Compare heap profiles
   curl http://localhost:9001/debug/pprof/heap > heap1.prof
   # Wait 5 minutes
   curl http://localhost:9001/debug/pprof/heap > heap2.prof
   go tool pprof -base heap1.prof heap2.prof
   ```

3. **Tune GC Settings**
   ```bash
   # Set memory limit
   GOMEMLIMIT=2GiB ./gateway
   
   # Adjust GC percentage
   GOGC=50 ./gateway
   ```

### Too Many Goroutines

1. **Check Goroutine Count**
   ```bash
   curl http://localhost:9001/debug/pprof/goroutine?debug=1
   ```

2. **Identify Goroutine Leaks**
   ```bash
   # Get goroutine profile
   curl http://localhost:9001/debug/pprof/goroutine > goroutine.prof
   go tool pprof goroutine.prof
   ```

## Debugging Techniques

### Enable Verbose Logging

```yaml
gateway:
  logging:
    level: debug
    verboseComponents:
      - router: trace
      - auth: debug
      - backend: debug
    
    # Log all requests/responses
    requests:
      enabled: true
      includeHeaders: true
      includeBody: true
      bodyLimit: 1024
```

### Use Request Tracing

```bash
# Trace a specific request
curl http://gateway:8080/api/test \
  -H "X-Request-ID: debug-$(date +%s)" \
  -H "X-Debug: true" \
  -H "X-Verbose: true"

# Follow in logs
tail -f /var/log/gateway/gateway.log | grep "debug-"
```

### Reproduce Issues

```yaml
gateway:
  debug:
    # Record and replay requests
    replay:
      enabled: true
      record:
        enabled: true
        path: /var/lib/gateway/replay
        filter:
          status: 5xx
      playback:
        enabled: true
        speed: 1x
```

## Emergency Procedures

### Circuit Breaker Tripped

```bash
# Reset circuit breaker
curl -X POST http://localhost:9001/circuit-breakers/api-service/reset

# Or via configuration
gateway:
  middleware:
    circuitbreaker:
      forceOpen: false
      forceClose: true  # Temporary override
```

### Rate Limit Exhausted

```bash
# Temporarily increase limits
curl -X PATCH http://localhost:9001/rate-limits/api-limit \
  -d '{"limit": 10000, "burst": 1000}'

# Clear rate limit counters (Redis)
redis-cli --scan --pattern "ratelimit:*" | xargs redis-cli del
```

### Rollback Configuration

```bash
# List configuration versions
curl http://localhost:9001/config/versions

# Rollback to previous version
curl -X POST http://localhost:9001/config/rollback \
  -d '{"version": "v1.2.3"}'
```

### Emergency Mode

```yaml
gateway:
  emergency:
    enabled: true
    mode: readonly  # readonly, bypass, minimal
    
    # Bypass all middleware
    bypass:
      - auth
      - ratelimit
      - transform
    
    # Use minimal configuration
    minimal:
      routes:
        - path: /*
          serviceName: fallback-service
```

## Getting Help

### Collect Diagnostic Information

```bash
# Generate diagnostic bundle
./gateway diagnostic-bundle \
  --include-config \
  --include-logs \
  --include-metrics \
  --include-profiles \
  --output gateway-diag-$(date +%Y%m%d-%H%M%S).tar.gz
```

### Important Information to Provide

1. Gateway version
2. Configuration (sanitized)
3. Error messages and stack traces
4. Steps to reproduce
5. Expected vs actual behavior
6. Recent changes
7. Environment details