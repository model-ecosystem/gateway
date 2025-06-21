# Session Affinity Examples

This directory contains examples demonstrating protocol-agnostic session affinity in the gateway.

## Overview

The gateway now supports flexible session affinity (sticky sessions) that works across all protocols (HTTP, WebSocket, SSE, gRPC) using configurable session extraction methods:

- **Cookie-based**: Extract session ID from HTTP cookies
- **Header-based**: Extract session ID from custom headers
- **Query parameter-based**: Extract session ID from URL query parameters

## Configuration

Session affinity is configured per route in the `sessionAffinity` section:

```yaml
sessionAffinity:
  enabled: true
  ttl: 3600  # Session TTL in seconds
  source: cookie | header | query
  cookieName: SESSION_ID  # For cookie source
  headerName: X-Session-Id  # For header source
  queryParam: sid  # For query source
```

## Examples

### 1. HTTP Web Application (Cookie-based)

Traditional web applications typically use cookies for session management:

```yaml
- id: web-app
  path: /app/*
  serviceName: stateful-service
  loadBalance: sticky_session
  sessionAffinity:
    enabled: true
    ttl: 1800  # 30 minutes
    source: cookie
    cookieName: SESSION_ID
```

### 2. REST API (Header-based)

APIs and mobile applications often use custom headers:

```yaml
- id: api-route
  path: /api/v1/*
  serviceName: api-service
  loadBalance: sticky_session
  sessionAffinity:
    enabled: true
    ttl: 3600  # 1 hour
    source: header
    headerName: X-Auth-Token
```

### 3. WebSocket (Multiple Options)

WebSocket connections can use any session source:

```yaml
# Cookie-based (when browser WebSocket API is used)
- id: ws-chat-cookie
  path: /ws/chat
  methods: ["WEBSOCKET", "GET"]
  serviceName: chat-service
  loadBalance: sticky_session
  sessionAffinity:
    enabled: true
    ttl: 3600
    source: cookie
    cookieName: CHAT_SESSION

# Header-based (for custom WebSocket clients)
- id: ws-chat-header
  path: /ws/api
  methods: ["WEBSOCKET", "GET"]
  serviceName: chat-service
  loadBalance: sticky_session
  sessionAffinity:
    enabled: true
    ttl: 3600
    source: header
    headerName: X-Client-Id
```

### 4. Server-Sent Events (Header/Query)

SSE connections often can't send cookies after initial connection:

```yaml
# Header-based
- id: sse-notifications
  path: /notifications/*
  methods: ["GET", "SSE"]
  serviceName: event-service
  loadBalance: sticky_session
  sessionAffinity:
    enabled: true
    ttl: 3600
    source: header
    headerName: X-Client-Id

# Query parameter-based
- id: sse-events
  path: /events
  methods: ["GET", "SSE"]
  serviceName: event-service
  loadBalance: sticky_session
  sessionAffinity:
    enabled: true
    ttl: 600
    source: query
    queryParam: client_id
```

### 5. File Download Service (Query-based)

For services where adding headers is difficult:

```yaml
- id: download-service
  path: /download/*
  serviceName: storage-service
  loadBalance: sticky_session
  sessionAffinity:
    enabled: true
    ttl: 600  # 10 minutes
    source: query
    queryParam: session
```

## Client Examples

### Cookie-based (JavaScript)
```javascript
// Browser automatically sends cookies
fetch('/app/data')
  .then(response => response.json())
  .then(data => console.log(data));
```

### Header-based (JavaScript)
```javascript
fetch('/api/v1/users', {
  headers: {
    'X-Auth-Token': 'my-session-token-123'
  }
})
.then(response => response.json())
.then(data => console.log(data));
```

### Query-based (Any client)
```bash
curl http://gateway/download/file.pdf?session=abc123
```

### WebSocket with Header
```javascript
const ws = new WebSocket('ws://gateway/ws/api', [], {
  headers: {
    'X-Client-Id': 'client-123'
  }
});
```

### SSE with Header
```javascript
const eventSource = new EventSource('/notifications/user123', {
  headers: {
    'X-Client-Id': 'client-123'
  }
});
```

## Benefits

1. **Protocol Agnostic**: Same configuration pattern works for all protocols
2. **Flexible**: Choose the best session source for your use case
3. **No Hardcoding**: Session extraction is configuration-driven
4. **Consistent**: All protocols use the same session affinity implementation

## Migration from Old Configuration

If you were using the old `stickySession` configuration:

```yaml
# Old format
stickySession:
  enabled: true
  ttl: 3600

# New format
sessionAffinity:
  enabled: true
  ttl: 3600
  source: cookie  # Explicitly specify source
  cookieName: GATEWAY_SESSION  # Configure cookie name
```

The new format is more explicit and supports multiple session sources.