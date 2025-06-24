# Docker Compose Service Discovery

The gateway can automatically discover services from Docker Compose environments using container labels.

## Features

- **Automatic discovery**: Services are discovered when containers start
- **Label-based configuration**: Configure services using Docker Compose labels
- **Project filtering**: Optionally filter by Docker Compose project name
- **Health monitoring**: Automatic health checks for discovered services
- **Dynamic updates**: Services are added/removed as containers start/stop

## Configuration

```yaml
gateway:
  registry:
    type: docker-compose
    dockerCompose:
      projectName: "myapp"      # Optional: filter by project
      labelPrefix: "gateway"    # Label prefix for configuration
      refreshInterval: 10       # Refresh interval in seconds
```

## Docker Compose Labels

Configure services for gateway discovery using labels:

```yaml
version: '3.8'
services:
  api:
    image: myapp/api:latest
    labels:
      # Enable gateway discovery
      gateway.enable: "true"
      # Specify the internal port
      gateway.port: "3000"
      # Optional: override scheme (default: http)
      gateway.scheme: "http"
      # Optional: add custom metadata
      gateway.version: "v1"
      gateway.region: "us-east-1"
```

## Label Reference

| Label | Description | Required |
|-------|-------------|----------|
| `gateway.enable` | Enable service discovery | No (implicit if port specified) |
| `gateway.port` | Internal service port | Yes |
| `gateway.scheme` | Protocol scheme (http/https) | No (default: http) |
| `gateway.*` | Custom metadata | No |

## Example

See `configs/examples/docker-compose.yaml` for a complete example.

```bash
# Start your Docker Compose project
docker-compose up -d

# Start the gateway with Docker Compose discovery
./gateway -config configs/examples/docker-compose.yaml

# The gateway will automatically discover services
# Access services through the gateway
curl http://localhost:8080/api/users
```

## How It Works

1. Gateway connects to Docker daemon
2. Lists containers filtered by compose labels
3. Extracts service configuration from labels
4. Creates service instances with container IPs
5. Monitors container lifecycle for updates

## Advanced Features

### Multiple Projects

Discover services from all Docker Compose projects by omitting the project name:

```yaml
dockerCompose:
  # No projectName specified - discovers all compose services
  labelPrefix: "gateway"
```

### Custom Networks

Services on the same Docker network can communicate using container names:

```yaml
services:
  api:
    networks:
      - myapp_network
    labels:
      gateway.enable: "true"
      gateway.port: "3000"
```

## Troubleshooting

- Ensure Docker daemon is accessible
- Check container labels are correctly formatted
- Verify containers are running and healthy
- Review gateway logs for discovery errors