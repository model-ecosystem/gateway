# Deployment Guide

## Docker Deployment

### Using Pre-built Image

```bash
docker run -d \
  --name gateway \
  -p 8080:8080 \
  -v $(pwd)/gateway.yaml:/etc/gateway/gateway.yaml:ro \
  gateway:latest
```

### Building Custom Image

```bash
docker build -f deployments/docker/Dockerfile -t my-gateway:latest .
```

### Docker Compose

```yaml
version: '3.8'
services:
  gateway:
    image: gateway:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config:/etc/gateway:ro
    environment:
      - CONFIG_FILE=/etc/gateway/gateway.yaml
```

## Kubernetes Deployment

### Basic Deployment

```bash
kubectl apply -f deployments/kubernetes/
```

### Using Helm

```bash
helm install my-gateway deployments/helm/gateway/
```

### Custom Values

```bash
helm install my-gateway deployments/helm/gateway/ \
  --set image.tag=v1.0.0 \
  --set service.type=LoadBalancer
```

## Production Considerations

### High Availability

1. **Run multiple instances** behind a load balancer
2. **Use shared service registry** (Consul, etcd)
3. **Configure health checks**
4. **Set up proper monitoring**

### Performance Tuning

```yaml
gateway:
  frontend:
    http:
      readTimeout: 60
      writeTimeout: 60
  backend:
    http:
      maxIdleConns: 200
      maxIdleConnsPerHost: 20
      idleConnTimeout: 120
```

### Security

1. **Enable TLS/mTLS**
2. **Configure authentication**
3. **Set up rate limiting**
4. **Use network policies**
5. **Regular security updates**

### Monitoring

#### Prometheus Metrics

```yaml
metrics:
  enabled: true
  port: 9090
  path: /metrics
```

#### Health Checks

```yaml
health:
  enabled: true
  port: 8081
  path: /health
```

### Logging

```yaml
logging:
  level: info
  format: json
  output: stdout
```

## Deployment Checklist

- [ ] Configure appropriate resource limits
- [ ] Set up health checks
- [ ] Configure TLS certificates
- [ ] Set up monitoring and alerting
- [ ] Configure log aggregation
- [ ] Test failover scenarios
- [ ] Document runbooks
- [ ] Set up backup procedures

## Troubleshooting

### Common Issues

1. **Connection refused**
   - Check service discovery
   - Verify backend health
   - Check network policies

2. **High latency**
   - Review connection pool settings
   - Check backend performance
   - Enable HTTP/2

3. **Memory issues**
   - Adjust connection limits
   - Enable request/response streaming
   - Monitor goroutine leaks