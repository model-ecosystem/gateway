# Configuration Loading

The gateway supports flexible configuration loading from multiple sources with hot-reloading, validation, and templating capabilities.

## Overview

Configuration loading features:
- Multiple source support (files, HTTP, environment, etc.)
- Dynamic reloading without downtime
- Configuration validation and schemas
- Template support with variable substitution
- Configuration merging and overrides
- Version control and rollback

## Configuration Sources

### File Sources

Load from local files:

```yaml
gateway:
  config:
    sources:
      - type: file
        path: /etc/gateway/config.yaml
        watch: true
        
      - type: file
        path: /etc/gateway/routes/*.yaml
        glob: true
        
      - type: file
        path: ${CONFIG_DIR}/overrides.yaml
        optional: true
```

### HTTP Sources

Load from remote endpoints:

```yaml
gateway:
  config:
    sources:
      - type: http
        url: https://config-server/gateway/config
        headers:
          Authorization: Bearer ${CONFIG_TOKEN}
        interval: 30s
        timeout: 10s
        tls:
          verify: true
          certFile: /certs/client.crt
          keyFile: /certs/client.key
```

### Environment Variables

Load from environment:

```yaml
gateway:
  config:
    sources:
      - type: env
        prefix: GATEWAY_
        delimiter: __
        lowercase: true
        # GATEWAY_SERVER__PORT becomes server.port
```

### Kubernetes ConfigMaps

Load from ConfigMaps:

```yaml
gateway:
  config:
    sources:
      - type: k8s-configmap
        namespace: default
        name: gateway-config
        key: config.yaml
        watch: true
```

### Consul KV

Load from Consul:

```yaml
gateway:
  config:
    sources:
      - type: consul
        address: consul:8500
        prefix: gateway/config
        datacenter: dc1
        token: ${CONSUL_TOKEN}
        watch: true
```

### etcd

Load from etcd:

```yaml
gateway:
  config:
    sources:
      - type: etcd
        endpoints:
          - etcd-1:2379
          - etcd-2:2379
        prefix: /gateway/config
        tls:
          enabled: true
          certFile: /certs/etcd-client.crt
```

### AWS Systems Manager

Load from Parameter Store:

```yaml
gateway:
  config:
    sources:
      - type: aws-ssm
        region: us-east-1
        prefix: /gateway/
        decrypt: true
        credentials:
          role: arn:aws:iam::123456789012:role/gateway
```

### HashiCorp Vault

Load from Vault:

```yaml
gateway:
  config:
    sources:
      - type: vault
        address: https://vault:8200
        path: secret/gateway
        auth:
          method: kubernetes
          role: gateway
        renewable: true
```

## Configuration Structure

### Schema Definition

Define configuration schema:

```yaml
gateway:
  config:
    schema:
      version: "1.0"
      strict: true  # Reject unknown fields
      definitions:
        - path: gateway.router.rules
          type: array
          items:
            type: object
            required: [id, path, serviceName]
            properties:
              id:
                type: string
                pattern: "^[a-z0-9-]+$"
              path:
                type: string
              serviceName:
                type: string
```

### Validation Rules

Custom validation:

```yaml
gateway:
  config:
    validation:
      rules:
        - name: unique-route-ids
          type: unique
          field: gateway.router.rules[*].id
          
        - name: valid-ports
          type: range
          field: "**.port"
          min: 1
          max: 65535
          
        - name: timeout-limits
          type: custom
          script: |
            if (timeout > 300) {
              return "Timeout cannot exceed 300 seconds"
            }
```

## Hot Reloading

### Reload Strategy

```yaml
gateway:
  config:
    reload:
      enabled: true
      strategy: graceful  # graceful, immediate
      validation: strict  # strict, warn, none
      rollback: true     # Rollback on error
      
      # Gradual rollout
      canary:
        enabled: true
        percentage: 10
        duration: 5m
        metrics:
          - error_rate < 0.01
          - latency_p99 < 500ms
```

### Change Detection

```yaml
gateway:
  config:
    watch:
      # File watching
      files:
        interval: 1s
        useNotify: true  # Use OS notifications
        
      # Remote source polling
      remote:
        interval: 30s
        jitter: 5s      # Add randomness
        
      # Debouncing
      debounce:
        wait: 2s        # Wait before applying
        maxWait: 10s    # Max wait time
```

## Template Support

### Variable Substitution

```yaml
gateway:
  server:
    host: ${HOST:0.0.0.0}
    port: ${PORT:8080}
    
  backend:
    url: ${BACKEND_URL}
    timeout: ${TIMEOUT:30s}
    
  auth:
    secret: ${JWT_SECRET|file:/run/secrets/jwt}
```

### Template Functions

```yaml
gateway:
  config:
    templates:
      functions:
        - env         # Environment variables
        - file        # Read from file
        - vault       # Vault secrets
        - base64      # Base64 encoding
        - json        # JSON parsing
        - default     # Default values
        
  # Example usage
  database:
    password: '{{ vault "secret/db/password" }}'
    config: '{{ file "/etc/db/config.json" | json }}'
```

### Conditional Configuration

```yaml
gateway:
  router:
    rules:
      {{ if eq .ENV "production" }}
      - id: prod-route
        path: /api/*
        serviceName: api-prod
        rateLimit: 1000
      {{ else }}
      - id: dev-route
        path: /api/*
        serviceName: api-dev
        rateLimit: 100
      {{ end }}
```

## Configuration Merging

### Merge Strategy

```yaml
gateway:
  config:
    merge:
      strategy: deep      # deep, shallow, replace
      arrays: append      # append, merge, replace
      conflicts: last     # last, first, error
      
    # Source priority (higher overwrites lower)
    sources:
      - type: file
        path: /etc/gateway/base.yaml
        priority: 100
        
      - type: file
        path: /etc/gateway/overrides.yaml
        priority: 200
        
      - type: env
        priority: 300
```

### Partial Updates

```yaml
gateway:
  config:
    updates:
      # Only update specific paths
      paths:
        - gateway.router.rules
        - gateway.middleware
      
      # Ignore certain paths
      ignore:
        - gateway.telemetry
        - gateway.storage
```

## Configuration Management

### Version Control

```yaml
gateway:
  config:
    versioning:
      enabled: true
      backend: git
      repository: /var/lib/gateway/config
      
      # Auto-commit changes
      autoCommit:
        enabled: true
        message: "Config update: {{.Timestamp}}"
        author: "gateway-bot"
        
      # Track history
      history:
        retain: 100
        compress: true
```

### Backup and Restore

```yaml
gateway:
  config:
    backup:
      enabled: true
      schedule: "0 * * * *"  # Hourly
      destination:
        type: s3
        bucket: gateway-config-backup
        prefix: configs/
      retention: 30d
      
    restore:
      # Restore point
      point: "2024-01-15T10:30:00Z"
      # Or specific version
      version: "v1.2.3"
```

### Diff and Audit

```yaml
gateway:
  config:
    audit:
      enabled: true
      log:
        changes: true    # Log all changes
        diff: true       # Include diff
        source: true     # Log source info
      
      # Webhook notifications
      webhooks:
        - url: https://audit.example.com/config
          events: [change, error]
```

## Advanced Features

### Configuration Profiles

```yaml
gateway:
  config:
    profiles:
      active: ${PROFILE:default}
      
      definitions:
        default:
          server.port: 8080
          log.level: info
          
        development:
          server.port: 8080
          log.level: debug
          debug: true
          
        production:
          server.port: 80
          log.level: warn
          tls.enabled: true
```

### Feature Flags

```yaml
gateway:
  config:
    features:
      source:
        type: http
        url: https://feature-flags/gateway
        interval: 60s
      
      flags:
        newRouter:
          enabled: ${FEATURE_NEW_ROUTER:false}
        experimentalMetrics:
          enabled: true
          rollout: 25  # Percentage
```

### Dynamic Routing Rules

```yaml
gateway:
  config:
    dynamic:
      routes:
        source:
          type: http
          url: https://api-catalog/routes
          transform: |
            routes.map(r => ({
              id: r.name,
              path: r.basePath + "/*",
              serviceName: r.service,
              methods: r.methods
            }))
```

## Security

### Encryption

```yaml
gateway:
  config:
    encryption:
      # Encrypt sensitive values
      enabled: true
      algorithm: aes-256-gcm
      
      # Encrypted fields
      fields:
        - "**.password"
        - "**.secret"
        - "**.token"
        - "**.key"
      
      # Key management
      keyProvider:
        type: kms
        keyId: ${KMS_KEY_ID}
```

### Access Control

```yaml
gateway:
  config:
    access:
      # Who can modify config
      writers:
        - role: admin
        - user: config-service
      
      # Who can read config
      readers:
        - role: operator
        - role: developer
      
      # Audit access
      audit: true
```

## Performance

### Caching

```yaml
gateway:
  config:
    cache:
      enabled: true
      ttl: 5m
      size: 100MB
      
      # Cache remote configs
      remote:
        enabled: true
        ttl: 1m
        
      # Preload on startup
      preload:
        enabled: true
        timeout: 30s
```

### Lazy Loading

```yaml
gateway:
  config:
    lazy:
      enabled: true
      # Load configs on demand
      sections:
        - path: gateway.router.rules
          trigger: startup
        - path: gateway.plugins
          trigger: ondemand
```

## Best Practices

1. **Use Schema Validation**: Define schemas for all configuration
2. **Environment-Specific Configs**: Separate configs by environment
3. **Secure Sensitive Data**: Encrypt passwords and secrets
4. **Version Control**: Track all configuration changes
5. **Test Changes**: Validate configs before applying
6. **Monitor Reloads**: Track reload success/failure
7. **Document Configuration**: Maintain config documentation

## Troubleshooting

### Configuration Errors

```yaml
gateway:
  config:
    debug:
      enabled: true
      # Log all source attempts
      logSources: true
      # Show merged result
      showMerged: true
      # Validation details
      verboseValidation: true
```

### Common Issues

1. **Merge Conflicts**
   - Check source priorities
   - Review merge strategy
   - Use explicit overrides

2. **Validation Failures**
   - Check schema definitions
   - Review validation rules
   - Test with minimal config

3. **Reload Failures**
   - Check file permissions
   - Verify network access
   - Review rollback logs