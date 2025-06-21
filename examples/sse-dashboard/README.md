# Server-Sent Events (SSE) Gateway Examples

This directory contains examples of using the gateway with Server-Sent Events (SSE) support.

## Quick Start

1. Start the SSE server:
```bash
cd test/servers/sse
go run sse_server.go
```

2. Start the gateway with SSE configuration:
```bash
./build/gateway -config configs/gateway-sse.yaml
```

3. Connect to the SSE endpoint through the gateway:
```bash
# Basic event stream
curl -N -H "Accept: text/event-stream" http://localhost:8080/events

# User-specific notifications
curl -N -H "Accept: text/event-stream" -H "X-User-ID: alice" \
  http://localhost:8080/notifications/alice
```

## SSE Configuration

The gateway supports SSE with the following features:

### Frontend Configuration
```yaml
frontend:
  sse:
    enabled: true
    writeTimeout: 60      # Write timeout in seconds
    keepaliveTimeout: 30  # Keepalive interval in seconds
```

### Backend Configuration
```yaml
backend:
  sse:
    dialTimeout: 10
    responseTimeout: 0    # No timeout for long-running streams
    keepaliveTimeout: 30
```

### Routing Configuration
```yaml
router:
  rules:
    # Basic SSE route
    - id: sse-events
      path: /events
      methods: ["GET", "SSE"]
      serviceName: sse-service
      loadBalance: round_robin
      timeout: 0  # No timeout for SSE
      
    # SSE with sticky sessions
    - id: sse-notifications
      path: /notifications/*
      methods: ["GET", "SSE"]
      serviceName: event-service
      loadBalance: sticky_session
      timeout: 0
```

## Features

### Automatic Keepalive
The gateway automatically sends keepalive comments (`:keepalive`) at configured intervals to prevent connection timeouts.

### Sticky Sessions
For stateful SSE services, use sticky sessions to ensure all events for a user go to the same backend:
```yaml
loadBalance: sticky_session
stickySession:
  enabled: true
  ttl: 3600
```

### Event Proxying
The gateway transparently proxies all SSE event fields:
- `id:` - Event ID for client-side reconnection
- `event:` - Event type
- `data:` - Event data (supports multi-line)
- `retry:` - Reconnection time
- `:` - Comments (used for keepalive)

## JavaScript Client Example

```javascript
// Connect to SSE endpoint through gateway
const eventSource = new EventSource('http://localhost:8080/events');

eventSource.onopen = () => {
    console.log('Connected to SSE gateway');
};

eventSource.onmessage = (event) => {
    console.log('Default event:', event.data);
};

// Listen for specific event types
eventSource.addEventListener('tick', (event) => {
    console.log('Tick event:', event.data);
});

eventSource.addEventListener('status', (event) => {
    const status = JSON.parse(event.data);
    console.log('Status update:', status);
});

eventSource.onerror = (error) => {
    console.error('SSE error:', error);
    if (eventSource.readyState === EventSource.CLOSED) {
        console.log('Connection closed');
    }
};

// Clean up
// eventSource.close();
```

## Testing

Run the integration tests:
```bash
./scripts/test-sse.sh
```

## Use Cases

1. **Real-time Notifications**: Push notifications to web clients
2. **Live Updates**: Stock prices, sports scores, news feeds
3. **Progress Monitoring**: Long-running job status updates
4. **System Monitoring**: Real-time metrics and alerts
5. **Chat Applications**: One-way message delivery

## Advantages of SSE

- **Simple Protocol**: Uses standard HTTP, works through proxies
- **Automatic Reconnection**: Built-in reconnection with Last-Event-ID
- **Efficient**: Single long-lived connection
- **Text-Based**: Easy to debug and implement

## Limitations

- **One-Way**: Server to client only (use WebSocket for bidirectional)
- **Text Only**: Binary data must be encoded
- **Browser Limits**: ~6 connections per domain in browsers

## Troubleshooting

1. **No Events Received**: Check Accept header is `text/event-stream`
2. **Connection Drops**: Verify timeout settings and keepalive
3. **Events Buffered**: Ensure proxy/gateway disables buffering
4. **CORS Issues**: Configure appropriate CORS headers