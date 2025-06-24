# Configuration Hot Reload

The gateway supports hot reloading of configuration files without service interruption. When enabled, the gateway monitors the configuration file for changes and automatically applies updates.

## Features

- **Zero-downtime reload**: Configuration changes are applied without dropping connections
- **Validation**: New configurations are validated before applying
- **Graceful transition**: Old configuration remains active until new one is successfully loaded
- **File watching**: Automatic detection of configuration file changes

## Usage

Enable hot reload by starting the gateway with the `-hot-reload` flag:

```bash
./gateway -config configs/gateway.yaml -hot-reload
```

## How It Works

1. The gateway watches the configuration file for changes
2. When a change is detected, the new configuration is loaded and validated
3. If validation passes, a new server instance is created with the new config
4. New requests are routed to the new server
5. The old server is gracefully shut down after existing requests complete

## Supported Changes

Most configuration changes can be applied via hot reload:

- Frontend settings (ports, timeouts)
- Backend settings (connection pools, timeouts)
- Service registry updates
- Route modifications
- Middleware configurations
- TLS settings

## Example

See `configs/examples/hotreload.yaml` for a working example.

```yaml
# Start the gateway with hot reload enabled
./gateway -config configs/examples/hotreload.yaml -hot-reload

# Modify the config file (e.g., change port, add services)
# The gateway will automatically reload

# Check logs for "Configuration reloaded successfully"
```

## Limitations

- The gateway binary cannot be updated via hot reload
- Some changes may require brief connection interruptions
- File watching may have platform-specific limitations