# Basic Gateway Example

This example demonstrates a basic gateway setup with simple HTTP routing.

## Quick Start

1. Start the example:
   ```bash
   docker-compose up
   ```

2. Test the gateway:
   ```bash
   # Route to service A
   curl http://localhost:8080/api/users
   
   # Route to service B
   curl http://localhost:8080/api/products
   ```

## Configuration

The gateway is configured with:
- Two backend services (service-a and service-b)
- Simple path-based routing
- Round-robin load balancing
- Basic health checks

See `config.yaml` for the complete configuration.