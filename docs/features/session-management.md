# Session Management

The gateway provides advanced session management capabilities for maintaining client state, implementing sticky sessions, and managing stateful connections across your services.

## Overview

Session management features:
- Client session tracking
- Sticky session routing (session affinity)
- Session storage backends
- Session replication
- Automatic session cleanup
- WebSocket session persistence
- Cross-datacenter session sync

## Configuration

### Basic Setup

```yaml
gateway:
  session:
    enabled: true
    
    # Session identification
    identifier:
      type: cookie
      name: "gateway-session"
      
    # Session storage
    storage:
      type: memory  # memory, redis, database
      
    # Session options
    options:
      timeout: 30m
      rolling: true  # Reset timeout on activity
      secure: true   # HTTPS only
      httpOnly: true
```

### Session Identifiers

#### Cookie-Based Sessions

```yaml
gateway:
  session:
    identifier:
      type: cookie
      name: "GSESSIONID"
      domain: ".example.com"
      path: "/"
      sameSite: "strict"
      maxAge: 86400  # 24 hours
```

#### Header-Based Sessions

```yaml
gateway:
  session:
    identifier:
      type: header
      name: "X-Session-ID"
      generateIfMissing: true
```

#### Multiple Identifiers

```yaml
gateway:
  session:
    identifier:
      sources:
        - type: cookie
          name: "session"
          priority: 1
        
        - type: header
          name: "X-Session-ID"
          priority: 2
        
        - type: query
          name: "sid"
          priority: 3
```

## Session Storage

### Memory Storage

```yaml
gateway:
  session:
    storage:
      type: memory
      memory:
        maxSessions: 100000
        evictionPolicy: "lru"  # lru, lfu, ttl
```

### Redis Storage

```yaml
gateway:
  session:
    storage:
      type: redis
      redis:
        addresses:
          - "redis-1:6379"
          - "redis-2:6379"
        
        # Clustering
        cluster: true
        
        # Connection options
        password: "${REDIS_PASSWORD}"
        db: 0
        
        # Key prefix
        keyPrefix: "gateway:session:"
        
        # Replication
        replication:
          enabled: true
          async: true
```

### Database Storage

```yaml
gateway:
  session:
    storage:
      type: database
      database:
        driver: "postgres"
        dsn: "${DATABASE_URL}"
        
        # Table configuration
        table: "gateway_sessions"
        
        # Cleanup
        cleanupInterval: 5m
        cleanupBatchSize: 1000
```

## Sticky Sessions (Session Affinity)

### Configuration

```yaml
gateway:
  router:
    sessionAffinity:
      enabled: true
      
      # Affinity methods
      methods:
        - cookie    # Preferred
        - sourceIP  # Fallback
      
      # Cookie configuration
      cookie:
        name: "gateway-backend"
        path: "/"
        maxAge: 3600
        
      # How to encode backend info
      encoding: "secure"  # secure, hash, plain
```

### Per-Route Affinity

```yaml
gateway:
  router:
    rules:
      - id: chat-api
        path: /chat/*
        serviceName: chat-service
        sessionAffinity:
          enabled: true
          method: cookie
          duration: 1h
      
      - id: shopping-cart
        path: /cart/*
        serviceName: cart-service
        sessionAffinity:
          enabled: true
          method: header
          headerName: "X-Cart-Session"
```

### Affinity Algorithms

```yaml
gateway:
  router:
    sessionAffinity:
      algorithm:
        type: "consistent-hash"
        
        # Consistent hashing options
        consistentHash:
          replicas: 150
          
        # Or IP hash
        ipHash:
          useXForwardedFor: true
          
        # Or custom header hash
        headerHash:
          header: "X-User-ID"
```

## Session Data

### Basic Session Data

```yaml
gateway:
  session:
    data:
      # What to store in sessions
      fields:
        - userId
        - roles
        - preferences
        - lastActivity
      
      # Max session size
      maxSize: 8KB
      
      # Encryption
      encryption:
        enabled: true
        algorithm: "aes-256-gcm"
        keyRotation: 24h
```

### Session Attributes

```yaml
gateway:
  session:
    attributes:
      # Extract from authentication
      fromAuth:
        - claim: "sub"
          attribute: "userId"
        
        - claim: "roles"
          attribute: "userRoles"
      
      # Extract from headers
      fromHeaders:
        - header: "X-Tenant-ID"
          attribute: "tenantId"
      
      # Computed attributes
      computed:
        - attribute: "sessionAge"
          expression: "now() - session.created"
```

## WebSocket Sessions

### Persistent WebSocket Sessions

```yaml
gateway:
  websocket:
    sessions:
      enabled: true
      
      # Maintain session across reconnects
      persistence:
        enabled: true
        timeout: 5m  # Time to maintain after disconnect
      
      # Session recovery
      recovery:
        enabled: true
        messageBuffer: 1000  # Messages to buffer
```

### WebSocket Session Affinity

```yaml
gateway:
  websocket:
    sessionAffinity:
      enabled: true
      
      # Ensure reconnects go to same backend
      stickyReconnect: true
      
      # Transfer session on backend failure
      failover:
        enabled: true
        transferSession: true
```

## Session Replication

### Cross-Instance Sync

```yaml
gateway:
  session:
    replication:
      enabled: true
      
      # Sync method
      method: "gossip"  # gossip, broadcast, redis
      
      # Gossip protocol settings
      gossip:
        interval: 1s
        fanout: 3
        
      # What to replicate
      replicate:
        - sessionData
        - affinityMappings
        - activeConnections
```

### Cross-Datacenter Sync

```yaml
gateway:
  session:
    replication:
      crossRegion:
        enabled: true
        
        regions:
          - name: "us-east"
            primary: true
            endpoints:
              - "gateway-us-east-1:7946"
              - "gateway-us-east-2:7946"
          
          - name: "eu-west"
            endpoints:
              - "gateway-eu-west-1:7946"
              - "gateway-eu-west-2:7946"
        
        # Conflict resolution
        conflictResolution: "last-write-wins"
```

## Session Security

### Session Validation

```yaml
gateway:
  session:
    security:
      # Validate session integrity
      validation:
        checkIP: true
        checkUserAgent: true
        checkFingerprint: true
      
      # Session hijacking protection
      antiHijacking:
        enabled: true
        rotateId: true  # Rotate ID periodically
        rotateInterval: 15m
```

### Session Encryption

```yaml
gateway:
  session:
    security:
      encryption:
        enabled: true
        
        # Encrypt session data
        data:
          algorithm: "aes-256-gcm"
          key: "${SESSION_ENCRYPTION_KEY}"
        
        # Encrypt session ID
        id:
          enabled: true
          algorithm: "hmac-sha256"
          secret: "${SESSION_ID_SECRET}"
```

## Session Lifecycle

### Session Creation

```yaml
gateway:
  session:
    lifecycle:
      # When to create sessions
      creation:
        triggers:
          - "authentication"
          - "firstRequest"
          - "explicitCreate"
        
        # Initial session data
        defaults:
          created: "${timestamp}"
          lastAccess: "${timestamp}"
          requestCount: 0
```

### Session Cleanup

```yaml
gateway:
  session:
    lifecycle:
      cleanup:
        # Cleanup strategies
        strategies:
          - type: "ttl"
            ttl: 24h
          
          - type: "inactive"
            inactiveTime: 30m
          
          - type: "maxSessions"
            maxSessions: 1000000
            evictPolicy: "lru"
        
        # Cleanup execution
        interval: 5m
        batchSize: 1000
```

## Monitoring

### Session Metrics

```yaml
gateway:
  session:
    metrics:
      enabled: true
      
      # Available metrics
      collect:
        - active_sessions
        - session_creates
        - session_destroys
        - session_hits
        - session_misses
        - affinity_hits
        - affinity_misses
```

### Session Events

```yaml
gateway:
  session:
    events:
      # Emit events for monitoring
      emit:
        - sessionCreated
        - sessionDestroyed
        - sessionExpired
        - affinityEstablished
        - affinityBroken
      
      # Event handlers
      handlers:
        webhook:
          url: "https://monitoring.example.com/sessions"
```

## Best Practices

1. **Choose Storage Wisely**: Use Redis/Database for production
2. **Secure Sessions**: Always use encryption and HTTPS
3. **Monitor Usage**: Track session metrics and growth
4. **Plan Capacity**: Size storage for peak sessions
5. **Test Failover**: Ensure session persistence during failures
6. **Regular Cleanup**: Prevent session accumulation

## Examples

### E-commerce Session

```yaml
gateway:
  session:
    enabled: true
    identifier:
      type: cookie
      name: "shop-session"
    
    storage:
      type: redis
      redis:
        cluster: true
    
    data:
      fields:
        - userId
        - cartId
        - preferences
    
    options:
      timeout: 2h
      rolling: true
```

### Real-time Chat

```yaml
gateway:
  websocket:
    sessions:
      enabled: true
      persistence:
        enabled: true
      
    sessionAffinity:
      enabled: true
      stickyReconnect: true
      
  session:
    storage:
      type: memory  # Fast for real-time
      memory:
        maxSessions: 50000
```

### Multi-tenant SaaS

```yaml
gateway:
  session:
    enabled: true
    
    attributes:
      fromHeaders:
        - header: "X-Tenant-ID"
          attribute: "tenantId"
    
    security:
      validation:
        checkTenant: true
    
    storage:
      type: database
      database:
        # Partition by tenant
        partitionKey: "tenantId"
```

This comprehensive session management system enables sophisticated stateful routing and session handling across your distributed services.