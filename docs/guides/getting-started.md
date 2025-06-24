# Getting Started

Welcome! This guide will get you up and running with the gateway in less than 5 minutes.

## Installation

### Option 1: Download Binary

```bash
# macOS/Linux
curl -L https://github.com/model-ecosystem/gateway/releases/latest/download/gateway-$(uname -s)-$(uname -m) -o gateway
chmod +x gateway

# Or using go install
go install github.com/model-ecosystem/gateway@latest
```

### Option 2: Build from Source

```bash
git clone https://github.com/model-ecosystem/gateway.git
cd gateway
make build
```

## Zero-Config Start

The gateway works out of the box with zero configuration:

```bash
./gateway
```

This starts the gateway on `http://localhost:8080` with:
- Built-in health endpoint: `/_gateway/health`
- Built-in echo endpoint: `/_gateway/echo`
- Default proxy to httpbin.org for testing

### Try It Now

```bash
# Check gateway health
curl http://localhost:8080/_gateway/health

# Echo endpoint - see your request details
curl http://localhost:8080/_gateway/echo

# Proxy to httpbin (default backend)
curl http://localhost:8080/get
curl http://localhost:8080/post -d "hello=world"
```

## Your First Configuration

Create a `gateway.yaml` file to proxy to your own backend:

```yaml
gateway:
  router:
    rules:
      - path: /*
        serviceName: my-app
  registry:
    static:
      services:
        - name: my-app
          instances:
            - address: localhost
              port: 3000
```

Start with your config:

```bash
./gateway -config gateway.yaml
```

Now all requests go to your app at `localhost:3000`.

## Adding Features

### Step 1: Multiple Routes

Route different paths to different services:

```yaml
gateway:
  router:
    rules:
      - path: /api/*
        serviceName: api-service
      - path: /web/*
        serviceName: web-service
  registry:
    static:
      services:
        - name: api-service
          instances:
            - address: localhost
              port: 3001
        - name: web-service
          instances:
            - address: localhost
              port: 3002
```

### Step 2: Add Authentication

Protect your API endpoints:

```yaml
gateway:
  auth:
    jwt:
      publicKey: |
        -----BEGIN PUBLIC KEY-----
        MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
        -----END PUBLIC KEY-----
  
  router:
    rules:
      - path: /api/*
        serviceName: api-service
        authRequired: true  # This route now requires JWT
      - path: /public/*
        serviceName: api-service
        # No auth required
```

Test it:

```bash
# Without token - returns 401
curl http://localhost:8080/api/users

# With token - works
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" http://localhost:8080/api/users
```

### Step 3: Rate Limiting

Prevent abuse and overload:

```yaml
gateway:
  router:
    rules:
      - path: /api/*
        serviceName: api-service
        authRequired: true
        rateLimit: 100      # 100 requests per second per IP
        rateLimitBurst: 200 # Allow short bursts
```

### Step 4: Load Balancing

Scale with multiple backend instances:

```yaml
gateway:
  registry:
    static:
      services:
        - name: api-service
          instances:
            - address: localhost
              port: 3001
            - address: localhost
              port: 3002
            - address: localhost
              port: 3003
```

The gateway automatically load balances between healthy instances.

## Real-World Example

Here's a complete example for a typical web application:

```yaml
gateway:
  frontend:
    http:
      port: 80
  
  router:
    rules:
      # Static assets - no auth needed
      - path: /static/*
        serviceName: nginx
        
      # Public API endpoints
      - path: /api/public/*
        serviceName: api
        rateLimit: 1000
        
      # Protected API endpoints
      - path: /api/*
        serviceName: api
        authRequired: true
        rateLimit: 100
        
      # Admin panel
      - path: /admin/*
        serviceName: admin
        authRequired: true
        rateLimit: 10
  
  registry:
    static:
      services:
        - name: nginx
          instances:
            - address: nginx.local
              port: 80
        - name: api
          instances:
            - address: api-1.local
              port: 3000
            - address: api-2.local
              port: 3000
        - name: admin
          instances:
            - address: admin.local
              port: 4000
```

## What's Next?

### Essential Guides
- [Configuration Guide](configuration.md) - All configuration options
- [Authentication Guide](authentication.md) - Secure your APIs
- [TLS Setup](tls-setup.md) - Enable HTTPS

### Advanced Features
- [WebSocket Support](configuration.md#protocol-support) - Real-time connections
- [gRPC Support](grpc.md) - High-performance RPC
- [Health Checks](health-checks.md) - Monitor backend health
- [Metrics](metrics.md) - Observability with Prometheus

### Examples

Check out the `/examples` directory:
- `basic/` - Simple HTTP routing
- `websocket-chat/` - Real-time chat application
- `microservices/` - Complete microservices setup
- `kubernetes/` - Kubernetes deployment

## Troubleshooting

### Gateway won't start
- Check port 8080 is available: `lsof -i :8080`
- Verify config syntax: `./gateway -config gateway.yaml -validate`

### Requests return 404
- Check your route paths match exactly
- Use `/*` for catch-all routes
- Verify service names match between routes and registry

### Connection refused
- Ensure backend services are running
- Check backend addresses and ports
- Look at gateway logs for connection errors

### Need Help?

- Read the [FAQ](../README.md#faq)
- Check [example configurations](../../configs/examples/)
- Open an issue on GitHub