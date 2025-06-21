# WebSocket Gateway Examples

This directory contains examples of using the gateway with WebSocket support.

## Quick Start

1. Start the echo server:
```bash
cd test/servers/websocket
go run echo_server.go
```

2. Start the gateway with WebSocket configuration:
```bash
./build/gateway -config configs/gateway-websocket.yaml
```

3. Connect to the WebSocket endpoint through the gateway:
```bash
# Using wscat (install with: npm install -g wscat)
wscat -c ws://localhost:8081/ws/echo

# Send messages and see them echoed back
> Hello, WebSocket!
< Hello, WebSocket!
```

## WebSocket Configuration

The gateway supports WebSocket with the following features:

### Frontend Configuration
```yaml
frontend:
  websocket:
    enabled: true
    host: "0.0.0.0"
    port: 8081
    readTimeout: 60
    writeTimeout: 60
    handshakeTimeout: 10
    maxMessageSize: 1048576  # 1MB
    checkOrigin: true
    allowedOrigins: ["*"]
```

### Routing Configuration
```yaml
router:
  rules:
    # Basic WebSocket route
    - id: websocket-echo
      path: /ws/echo
      methods: ["WEBSOCKET", "GET"]
      serviceName: websocket-service
      loadBalance: round_robin
      
    # WebSocket with sticky sessions
    - id: websocket-chat
      path: /ws/chat/*
      methods: ["WEBSOCKET", "GET"]
      serviceName: chat-service
      loadBalance: sticky_session
      stickySession:
        enabled: true
        ttl: 3600  # 1 hour
```

## Sticky Sessions

For stateful WebSocket services (like chat applications), use sticky sessions to ensure all messages from a client go to the same backend instance:

```yaml
loadBalance: sticky_session
stickySession:
  enabled: true
  ttl: 3600  # Session TTL in seconds
```

The gateway uses the following methods to identify sessions:
1. `GATEWAY_SESSION` cookie
2. `X-Session-Id` header
3. Client IP address (for WebSocket connections without explicit session ID)

## Testing

Run the integration tests:
```bash
./scripts/test-websocket.sh
```

## JavaScript Client Example

```javascript
// Connect to WebSocket through gateway
const ws = new WebSocket('ws://localhost:8081/ws/echo');

ws.onopen = () => {
    console.log('Connected to WebSocket gateway');
    ws.send('Hello from JavaScript!');
};

ws.onmessage = (event) => {
    console.log('Received:', event.data);
};

ws.onerror = (error) => {
    console.error('WebSocket error:', error);
};

ws.onclose = () => {
    console.log('Disconnected from WebSocket gateway');
};
```

## Load Testing

You can load test the WebSocket gateway using tools like [ws-load-test](https://github.com/observing/ws-load-test) or [artillery](https://artillery.io/):

```bash
# Using artillery
artillery quick --count 100 --num 10 ws://localhost:8081/ws/echo
```

## Troubleshooting

1. **Connection Refused**: Ensure both the backend service and gateway are running
2. **403 Forbidden**: Check `checkOrigin` and `allowedOrigins` settings
3. **Connection Drops**: Verify timeout settings and keep-alive configuration
4. **Messages Not Routed**: Check that route methods include both "WEBSOCKET" and "GET"