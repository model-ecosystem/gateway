package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	// GatewayGroup is the API group for gateway CRDs
	GatewayGroup = "gateway.io"
	// GatewayVersion is the API version
	GatewayVersion = "v1alpha1"
)

// GatewayRoute represents a custom resource for gateway routes
type GatewayRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GatewayRouteSpec   `json:"spec"`
	Status            GatewayRouteStatus `json:"status,omitempty"`
}

// GatewayRouteSpec defines the desired state of a gateway route
type GatewayRouteSpec struct {
	// Path pattern for the route
	Path string `json:"path"`
	// Service name to route to
	ServiceName string `json:"serviceName"`
	// Service port (optional)
	ServicePort int32 `json:"servicePort,omitempty"`
	// Protocol (http, grpc, websocket, sse)
	Protocol string `json:"protocol,omitempty"`
	// Load balancing algorithm
	LoadBalance string `json:"loadBalance,omitempty"`
	// Timeout in seconds
	Timeout int `json:"timeout,omitempty"`
	// Authentication configuration
	Auth *AuthConfig `json:"auth,omitempty"`
	// Rate limiting configuration
	RateLimit *RateLimitConfig `json:"rateLimit,omitempty"`
	// Middleware to apply
	Middleware []string `json:"middleware,omitempty"`
	// Additional metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// AuthConfig defines authentication settings
type AuthConfig struct {
	// Type of authentication (jwt, apikey, oauth2)
	Type string `json:"type"`
	// Whether authentication is required
	Required bool `json:"required"`
	// Additional auth parameters
	Parameters map[string]string `json:"parameters,omitempty"`
}

// RateLimitConfig defines rate limiting settings
type RateLimitConfig struct {
	// Requests per second
	RPS int `json:"rps"`
	// Burst size
	Burst int `json:"burst"`
	// Rate limit key (client_ip, header, etc.)
	Key string `json:"key,omitempty"`
}

// GatewayRouteStatus defines the observed state of a gateway route
type GatewayRouteStatus struct {
	// Current state of the route
	State string `json:"state,omitempty"`
	// Last update time
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
	// Error message if any
	Error string `json:"error,omitempty"`
	// Number of active instances
	ActiveInstances int `json:"activeInstances,omitempty"`
}

// CRDWatcher watches for GatewayRoute custom resources
type CRDWatcher struct {
	dynamicClient dynamic.Interface
	namespace     string
	routeHandler  func(*GatewayRoute, watch.EventType)
}

// NewCRDWatcher creates a new CRD watcher
func NewCRDWatcher(config *rest.Config, namespace string) (*CRDWatcher, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &CRDWatcher{
		dynamicClient: dynamicClient,
		namespace:     namespace,
	}, nil
}

// SetRouteHandler sets the handler for route events
func (w *CRDWatcher) SetRouteHandler(handler func(*GatewayRoute, watch.EventType)) {
	w.routeHandler = handler
}

// Start starts watching for CRD changes
func (w *CRDWatcher) Start(ctx context.Context) error {
	gvr := schema.GroupVersionResource{
		Group:    GatewayGroup,
		Version:  GatewayVersion,
		Resource: "gatewayroutes",
	}

	// Create watch options
	watchOptions := metav1.ListOptions{
		Watch: true,
	}

	// Start watching
	var watcher watch.Interface
	var err error

	if w.namespace != "" {
		watcher, err = w.dynamicClient.Resource(gvr).Namespace(w.namespace).Watch(ctx, watchOptions)
	} else {
		watcher, err = w.dynamicClient.Resource(gvr).Watch(ctx, watchOptions)
	}

	if err != nil {
		return fmt.Errorf("failed to create CRD watcher: %w", err)
	}

	go w.handleEvents(ctx, watcher)
	return nil
}

// handleEvents processes watch events
func (w *CRDWatcher) handleEvents(ctx context.Context, watcher watch.Interface) {
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}

			unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}

			// Convert to GatewayRoute
			route := &GatewayRoute{}
			err := fromUnstructured(unstructuredObj, route)
			if err != nil {
				continue
			}

			// Call handler if set
			if w.routeHandler != nil {
				w.routeHandler(route, event.Type)
			}
		}
	}
}

// fromUnstructured converts an unstructured object to a typed object
func fromUnstructured(u *unstructured.Unstructured, obj interface{}) error {
	// This is a simplified conversion - in production you'd use proper conversion
	// For now, we'll just extract the key fields
	route, ok := obj.(*GatewayRoute)
	if !ok {
		return fmt.Errorf("invalid object type")
	}

	// Extract metadata
	route.Name = u.GetName()
	route.Namespace = u.GetNamespace()
	route.UID = u.GetUID()

	// Extract spec
	spec, found, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil || !found {
		return fmt.Errorf("spec not found")
	}

	// Path
	if path, ok := spec["path"].(string); ok {
		route.Spec.Path = path
	}

	// ServiceName
	if serviceName, ok := spec["serviceName"].(string); ok {
		route.Spec.ServiceName = serviceName
	}

	// ServicePort
	if port, ok := spec["servicePort"].(int64); ok {
		route.Spec.ServicePort = int32(port)
	}

	// Protocol
	if protocol, ok := spec["protocol"].(string); ok {
		route.Spec.Protocol = protocol
	}

	// LoadBalance
	if lb, ok := spec["loadBalance"].(string); ok {
		route.Spec.LoadBalance = lb
	}

	// Timeout
	if timeout, ok := spec["timeout"].(int64); ok {
		route.Spec.Timeout = int(timeout)
	}

	// Auth
	if authMap, ok := spec["auth"].(map[string]interface{}); ok {
		route.Spec.Auth = &AuthConfig{}
		if authType, ok := authMap["type"].(string); ok {
			route.Spec.Auth.Type = authType
		}
		if required, ok := authMap["required"].(bool); ok {
			route.Spec.Auth.Required = required
		}
	}

	// RateLimit
	if rlMap, ok := spec["rateLimit"].(map[string]interface{}); ok {
		route.Spec.RateLimit = &RateLimitConfig{}
		if rps, ok := rlMap["rps"].(int64); ok {
			route.Spec.RateLimit.RPS = int(rps)
		}
		if burst, ok := rlMap["burst"].(int64); ok {
			route.Spec.RateLimit.Burst = int(burst)
		}
		if key, ok := rlMap["key"].(string); ok {
			route.Spec.RateLimit.Key = key
		}
	}

	return nil
}

// Example CRD YAML:
/*
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gatewayroutes.gateway.io
spec:
  group: gateway.io
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            required: ["path", "serviceName"]
            properties:
              path:
                type: string
              serviceName:
                type: string
              servicePort:
                type: integer
              protocol:
                type: string
                enum: ["http", "grpc", "websocket", "sse"]
              loadBalance:
                type: string
                enum: ["round_robin", "least_connection", "weighted", "sticky"]
              timeout:
                type: integer
              auth:
                type: object
                properties:
                  type:
                    type: string
                  required:
                    type: boolean
                  parameters:
                    type: object
                    additionalProperties:
                      type: string
              rateLimit:
                type: object
                properties:
                  rps:
                    type: integer
                  burst:
                    type: integer
                  key:
                    type: string
              middleware:
                type: array
                items:
                  type: string
              metadata:
                type: object
                additionalProperties:
                  type: string
          status:
            type: object
            properties:
              state:
                type: string
              lastUpdated:
                type: string
              error:
                type: string
              activeInstances:
                type: integer
  scope: Namespaced
  names:
    plural: gatewayroutes
    singular: gatewayroute
    kind: GatewayRoute
    shortNames:
    - gwr
*/