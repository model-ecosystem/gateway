# Kubernetes Service Discovery

The gateway supports native Kubernetes service discovery, allowing automatic discovery and routing to services running in Kubernetes clusters using Custom Resource Definitions (CRDs).

## Overview

Kubernetes discovery provides:
- Automatic service discovery via Kubernetes API
- Custom Resource Definition (CRD) support
- Real-time updates via watch API
- Namespace isolation
- Label and annotation filtering
- Health status from readiness probes
- Multi-cluster support

## Configuration

### Basic Setup

```yaml
gateway:
  registry:
    type: kubernetes
    kubernetes:
      # In-cluster configuration (when running inside K8s)
      inCluster: true
      
      # Or external configuration
      kubeconfig: /path/to/kubeconfig
      
      # Namespace to watch (default: all namespaces)
      namespace: default
      
      # Label selector for filtering
      labelSelector: "app=myapp,tier=backend"
```

### Advanced Configuration

```yaml
gateway:
  registry:
    type: kubernetes
    kubernetes:
      # Watch specific namespaces
      namespaces:
        - default
        - production
        - staging
      
      # Service filtering
      serviceSelector:
        labels:
          gateway.enabled: "true"
          tier: backend
        
        annotations:
          gateway.route: "/api/*"
      
      # Endpoint configuration
      endpointMode: "pod"  # or "service" for ClusterIP
      
      # Health check integration
      useReadinessProbe: true
      
      # Update frequency
      resyncPeriod: 30s
      
      # Connection settings
      qps: 50
      burst: 100
```

## Custom Resource Definition (CRD)

### Gateway Service CRD

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gatewayservices.gateway.io
spec:
  group: gateway.io
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              service:
                type: string
              port:
                type: integer
              weight:
                type: integer
              healthCheck:
                type: object
                properties:
                  path:
                    type: string
                  interval:
                    type: string
              metadata:
                type: object
                additionalProperties:
                  type: string
  scope: Namespaced
  names:
    plural: gatewayservices
    singular: gatewayservice
    kind: GatewayService
```

### Using the CRD

```yaml
apiVersion: gateway.io/v1
kind: GatewayService
metadata:
  name: user-service
  namespace: default
spec:
  service: user-service
  port: 8080
  weight: 100
  healthCheck:
    path: /health
    interval: 10s
  metadata:
    version: "v2"
    region: "us-east"
```

## Service Discovery Modes

### 1. Service-Based Discovery

```yaml
gateway:
  registry:
    kubernetes:
      endpointMode: "service"
      # Uses Kubernetes Service ClusterIP
```

Routes to Kubernetes Service:
- Leverages kube-proxy load balancing
- Works with all service types
- Simple configuration

### 2. Pod-Based Discovery

```yaml
gateway:
  registry:
    kubernetes:
      endpointMode: "pod"
      # Direct pod discovery
```

Routes directly to pods:
- Bypasses kube-proxy
- More control over load balancing
- Better for sticky sessions

### 3. Headless Service Discovery

```yaml
# Kubernetes headless service
apiVersion: v1
kind: Service
metadata:
  name: headless-service
spec:
  clusterIP: None
  selector:
    app: myapp
```

Gateway configuration:
```yaml
gateway:
  registry:
    kubernetes:
      # Automatically detects headless services
      endpointMode: "auto"
```

## Label-Based Routing

### Service Labels

```yaml
# Kubernetes service
apiVersion: v1
kind: Service
metadata:
  name: user-service
  labels:
    gateway.enabled: "true"
    gateway.route: "/users"
    gateway.weight: "100"
```

### Gateway Configuration

```yaml
gateway:
  router:
    kubernetes:
      autoRouting: true
      routeLabelPrefix: "gateway."
      
      # Creates routes from labels automatically
      # gateway.route: "/users" → path: /users/*
      # gateway.weight: "100" → weight: 100
```

## Annotation-Based Configuration

### Service Annotations

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-service
  annotations:
    gateway.io/timeout: "30s"
    gateway.io/retry: "3"
    gateway.io/circuit-breaker: "true"
    gateway.io/rate-limit: "100/minute"
```

### Automatic Middleware Application

```yaml
gateway:
  registry:
    kubernetes:
      annotationPrefix: "gateway.io/"
      applyAnnotations: true
```

## Multi-Cluster Support

### Configuration

```yaml
gateway:
  registry:
    kubernetes:
      clusters:
        - name: primary
          kubeconfig: /configs/primary.yaml
          weight: 70
        
        - name: secondary
          kubeconfig: /configs/secondary.yaml  
          weight: 30
        
        - name: dr-site
          kubeconfig: /configs/dr.yaml
          weight: 0  # Standby
```

### Cross-Cluster Load Balancing

```yaml
gateway:
  router:
    rules:
      - id: multi-cluster
        path: /api/*
        kubernetes:
          service: user-service
          clusters:
            - primary
            - secondary
          loadBalance: weighted_round_robin
```

## Health Integration

### Readiness Probe Integration

```yaml
# Kubernetes deployment
spec:
  containers:
  - name: app
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      periodSeconds: 10
```

Gateway uses readiness status:
- Only routes to ready pods
- Automatic health updates
- No additional health checks needed

### Custom Health Checks

```yaml
gateway:
  registry:
    kubernetes:
      healthCheck:
        enabled: true
        # Override readiness probe
        customChecks:
          - service: critical-service
            path: /health/detailed
            interval: 5s
            timeout: 2s
```

## RBAC Requirements

### ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gateway
  namespace: gateway-system
```

### ClusterRole

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gateway
rules:
- apiGroups: [""]
  resources: ["services", "endpoints", "pods"]
  verbs: ["get", "watch", "list"]
- apiGroups: ["gateway.io"]
  resources: ["gatewayservices"]
  verbs: ["get", "watch", "list"]
```

### ClusterRoleBinding

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: gateway
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gateway
subjects:
- kind: ServiceAccount
  name: gateway
  namespace: gateway-system
```

## Deployment Example

### Gateway Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
  namespace: gateway-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      labels:
        app: gateway
    spec:
      serviceAccountName: gateway
      containers:
      - name: gateway
        image: gateway:latest
        env:
        - name: GATEWAY_REGISTRY_TYPE
          value: kubernetes
        - name: GATEWAY_KUBERNETES_INCLUSTER
          value: "true"
```

## Best Practices

1. **Use Namespaces**: Isolate services by namespace
2. **Label Consistently**: Establish labeling conventions
3. **RBAC Minimal**: Grant only required permissions
4. **Health Checks**: Leverage readiness probes
5. **Resource Limits**: Set appropriate QPS/burst
6. **Monitor Discovery**: Track discovery latency

## Troubleshooting

### Common Issues

1. **Services Not Discovered**
   - Check RBAC permissions
   - Verify label selectors
   - Review namespace configuration

2. **Connection Refused**
   - Ensure correct port configuration
   - Check network policies
   - Verify service endpoints

3. **Stale Endpoints**
   - Check resync period
   - Verify watch connection
   - Review event processing

### Debug Logging

```yaml
gateway:
  logging:
    level: debug
    modules:
      registry.kubernetes: debug
```

## Integration with Gateway Features

### Automatic Route Creation

```yaml
gateway:
  router:
    kubernetes:
      enabled: true
      template:
        path: "/{namespace}/{service}/*"
        stripPrefix: true
```

Creates routes like:
- `/default/user-service/*` → `user-service.default.svc.cluster.local`
- `/prod/api-service/*` → `api-service.prod.svc.cluster.local`

### Service Mesh Integration

Works with:
- Istio
- Linkerd
- Consul Connect

```yaml
gateway:
  registry:
    kubernetes:
      serviceMesh:
        enabled: true
        type: istio
        # Uses mesh metadata
```