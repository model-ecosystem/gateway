# JWT Validation for Long-Lived Connections

This guide explains how JWT token validation works for long-lived connections like Server-Sent Events (SSE) and WebSocket in the gateway.

## Overview

Long-lived connections present unique challenges for authentication:
- Connections can remain open for extended periods (hours or days)
- JWT tokens typically have expiration times
- Need to handle token expiration gracefully without disrupting service

The gateway provides automatic JWT validation for:
- **SSE (Server-Sent Events)**: Unidirectional streaming connections
- **WebSocket**: Bidirectional real-time connections

## How It Works

### Initial Authentication

1. Client establishes SSE/WebSocket connection with JWT token in Authorization header
2. Gateway validates the token before upgrading the connection
3. If valid, connection is established; if invalid, connection is rejected

### Periodic Validation

For tokens with expiration:
1. Gateway tracks token expiration time
2. Validates token periodically before expiration
3. If token expires, connection is gracefully closed
4. Client receives notification before disconnection

### Token Without Expiration

- Tokens without `exp` claim are considered valid indefinitely
- No periodic validation occurs
- Connection remains open until explicitly closed

## Configuration

### Enable JWT Authentication

```yaml
gateway:
  auth:
    jwt:
      enabled: true
      issuer: "https://auth.example.com"
      audience: "api.example.com"
      signingMethod: "RS256"
      jwksEndpoint: "https://auth.example.com/.well-known/jwks.json"
      jwksCacheDuration: 3600
```

### Configure SSE with JWT Validation

```yaml
gateway:
  frontend:
    sse:
      enabled: true
      writeTimeout: 300      # 5 minutes
      keepaliveTimeout: 30   # Send keepalive every 30 seconds
  
  router:
    rules:
      - id: secure-events
        path: /events/*
        serviceName: event-service
        loadBalance: sticky
        timeout: 300
        auth:
          required: true
          scopes:
            - "events:read"
```

### Configure WebSocket with JWT Validation

```yaml
gateway:
  frontend:
    websocket:
      enabled: true
      host: "0.0.0.0"
      port: 8081
      readTimeout: 60
      writeTimeout: 60
  
  router:
    rules:
      - id: secure-ws
        path: /ws/*
        serviceName: ws-service
        loadBalance: sticky
        timeout: 300
        auth:
          required: true
          scopes:
            - "ws:connect"
```

## Client Implementation

### SSE Client Example

```javascript
// JavaScript/Browser example
const token = localStorage.getItem('jwt_token');
const eventSource = new EventSource('/events/stream', {
  headers: {
    'Authorization': `Bearer ${token}`
  }
});

eventSource.onerror = (error) => {
  if (error.data === 'authentication expired') {
    // Token expired, need to reconnect with new token
    eventSource.close();
    refreshTokenAndReconnect();
  }
};
```

### WebSocket Client Example

```javascript
// JavaScript/Browser example
const token = localStorage.getItem('jwt_token');
const ws = new WebSocket('ws://localhost:8081/ws/chat', [], {
  headers: {
    'Authorization': `Bearer ${token}`
  }
});

ws.onclose = (event) => {
  if (event.reason === 'authentication expired') {
    // Token expired, need to reconnect with new token
    refreshTokenAndReconnect();
  }
};
```

## Token Validation Lifecycle

### SSE Connection

1. **Connection Established**: Token validated, SSE stream starts
2. **During Connection**: 
   - Keepalive events sent periodically
   - Token validation scheduled before expiration
3. **Token Expiration**:
   - Error event sent: `event: error\ndata: authentication expired\n\n`
   - Connection closed gracefully
4. **Client Reconnection**: Client should obtain new token and reconnect

### WebSocket Connection

1. **Connection Established**: Token validated, WebSocket upgraded
2. **During Connection**:
   - Ping/pong frames maintain connection
   - Token validation scheduled before expiration
3. **Token Expiration**:
   - Close frame sent with reason "authentication expired"
   - Connection closed with status 1000 (normal closure)
4. **Client Reconnection**: Client should obtain new token and reconnect

## Security Considerations

### Token Rotation

For enhanced security with long-lived connections:

1. **Short-Lived Access Tokens**: Use tokens with reasonable expiration (e.g., 1 hour)
2. **Refresh Tokens**: Implement refresh token flow for obtaining new access tokens
3. **Graceful Reconnection**: Design clients to handle token expiration gracefully

### Example Token Rotation Flow

```yaml
# Gateway validates tokens with these claims
jwt:
  enabled: true
  claimsMapping:
    subject: "sub"
    scopes: "scope"
    # Token must have 'exp' claim for periodic validation
```

Client implementation:
```javascript
class SecureEventStream {
  constructor(url) {
    this.url = url;
    this.reconnectDelay = 1000;
  }
  
  async connect() {
    const token = await this.getValidToken();
    this.eventSource = new EventSource(this.url, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    
    this.eventSource.onerror = async (error) => {
      if (error.data === 'authentication expired') {
        this.eventSource.close();
        await this.refreshAndReconnect();
      }
    };
  }
  
  async refreshAndReconnect() {
    await new Promise(resolve => setTimeout(resolve, this.reconnectDelay));
    await this.connect();
  }
  
  async getValidToken() {
    // Check if current token is still valid
    const currentToken = localStorage.getItem('access_token');
    if (this.isTokenValid(currentToken)) {
      return currentToken;
    }
    
    // Refresh token
    const refreshToken = localStorage.getItem('refresh_token');
    const response = await fetch('/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken })
    });
    
    const data = await response.json();
    localStorage.setItem('access_token', data.access_token);
    return data.access_token;
  }
}
```

## Monitoring and Debugging

### Log Messages

The gateway logs JWT validation events:

```
INFO JWT token validation enabled for SSE connections
INFO JWT token validation enabled for WebSocket connections
INFO JWT token expired, closing SSE connection connectionID=req-123 remote=192.168.1.100:54321
INFO JWT token expired, closing WebSocket connection connectionID=req-456 remote=192.168.1.101:54322
```

### Common Issues

1. **Token Not Provided**
   - SSE/WebSocket connections without auth header are allowed if route doesn't require auth
   - Connections are rejected if route requires authentication

2. **Token Validation Fails**
   - Initial connection is rejected with 401 Unauthorized
   - Error logged with details

3. **Token Expires During Connection**
   - Connection closed gracefully
   - Client notified before disconnection
   - Client should reconnect with new token

## Best Practices

1. **Use Appropriate Token Lifetimes**
   - Balance security with user experience
   - Shorter tokens (1-2 hours) for sensitive operations
   - Implement smooth token refresh flow

2. **Handle Disconnections Gracefully**
   - Implement automatic reconnection logic
   - Use exponential backoff for reconnection attempts
   - Maintain client state during reconnections

3. **Monitor Token Expiration**
   - Track metrics on token expiration disconnections
   - Adjust token lifetimes based on usage patterns

4. **Test Expiration Scenarios**
   - Test with short-lived tokens during development
   - Ensure clients handle expiration correctly
   - Verify graceful degradation

## Example: Complete Setup

```yaml
gateway:
  # Frontend configuration
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
    
    sse:
      enabled: true
      writeTimeout: 300
      keepaliveTimeout: 30
    
    websocket:
      enabled: true
      port: 8081
  
  # JWT authentication
  auth:
    jwt:
      enabled: true
      issuer: "https://auth.example.com"
      audience: "gateway.example.com"
      signingMethod: "RS256"
      jwksEndpoint: "https://auth.example.com/.well-known/jwks.json"
  
  # Routes with authentication
  router:
    rules:
      # Protected SSE endpoint
      - id: events
        path: /events/*
        serviceName: event-service
        loadBalance: sticky
        auth:
          required: true
          scopes: ["events:read"]
      
      # Protected WebSocket endpoint  
      - id: ws
        path: /ws/*
        serviceName: ws-service
        loadBalance: sticky
        auth:
          required: true
          scopes: ["ws:connect"]
      
      # Public endpoints (no auth)
      - id: public
        path: /public/*
        serviceName: public-service
```

This configuration ensures:
- JWT validation on connection establishment
- Periodic validation for tokens with expiration
- Graceful handling of token expiration
- Clear error messages for clients