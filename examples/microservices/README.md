# Docker Service Discovery Examples

This directory contains examples of using the gateway with Docker service discovery.

## Quick Start

1. Start services with docker-compose:
```bash
docker-compose up -d
```

2. Test the gateway:
```bash
# HTTP requests are load balanced across backend instances
curl http://localhost:8080/get
curl http://localhost:8080/headers

# WebSocket connection
wscat -c ws://localhost:8081/ws/echo

# SSE stream
curl -N -H "Accept: text/event-stream" http://localhost:8080/events
```

3. View discovered services:
```bash
docker ps --filter "label=gateway.enable=true"
```

## How It Works

The gateway discovers services by querying the Docker API for containers with specific labels:

### Required Labels
- `gateway.enable=true` - Enables discovery for this container
- `gateway.service=<name>` - Service name for routing
- `gateway.port=<port>` - Port the service listens on

### Optional Labels
- `gateway.scheme=<scheme>` - URL scheme (http, https, ws, wss)
- `gateway.meta.*` - Additional metadata

## Configuration

### Docker Registry Configuration
```yaml
registry:
  type: docker
  docker:
    host: "unix:///var/run/docker.sock"  # Docker socket
    labelPrefix: "gateway"                # Label prefix
    network: ""                          # Docker network (empty = any)
    refreshInterval: 10                  # Refresh interval in seconds
```

### Service Labels Example
```yaml
services:
  backend:
    image: myapp:latest
    labels:
      - "gateway.enable=true"
      - "gateway.service=myapp-service"
      - "gateway.port=8080"
      - "gateway.meta.version=v1"
      - "gateway.meta.region=us-east"
```

## Advanced Features

### 1. Multiple Instances
Docker Compose automatically creates multiple instances with scaling:
```bash
docker-compose up -d --scale backend=3
```

### 2. Network Isolation
Specify a Docker network for service discovery:
```yaml
docker:
  network: "myapp_network"
```

### 3. Dynamic Updates
The gateway automatically discovers new containers and removes stopped ones based on the refresh interval.

### 4. Health Checks
Only running containers are included in service discovery. Add Docker health checks for more control:
```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
  interval: 30s
  timeout: 10s
  retries: 3
```

## Development Workflow

1. **Local Development**: Use docker-compose for local service dependencies
2. **Integration Testing**: Test service discovery with multiple instances
3. **Debugging**: Check gateway logs for discovery events

## Troubleshooting

### No Services Found
- Check container labels with `docker inspect <container>`
- Ensure containers are running: `docker ps`
- Verify Docker socket access permissions

### Connection Refused
- Check if service port matches the label
- Ensure services are on the same network
- Verify firewall rules

### Stale Services
- Reduce `refreshInterval` for faster updates
- Check container health status
- Manual refresh not supported (by design)

## Security Considerations

1. **Docker Socket Access**: The gateway needs access to Docker socket
2. **Network Isolation**: Use Docker networks to isolate services
3. **Label Validation**: Only trust containers you control

## Example docker-compose.yml

See the main `docker-compose.yaml` in the project root for a complete example with:
- Multiple backend instances
- WebSocket service
- SSE service
- Proper labeling
- Network configuration