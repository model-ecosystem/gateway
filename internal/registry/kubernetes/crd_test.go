package kubernetes

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestFromUnstructured(t *testing.T) {
	tests := []struct {
		name        string
		input       *unstructured.Unstructured
		expectError bool
		validate    func(*testing.T, *GatewayRoute)
	}{
		{
			name: "complete route",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-route",
						"namespace": "default",
						"uid":       "123-456",
					},
					"spec": map[string]interface{}{
						"path":        "/api/test",
						"serviceName": "test-service",
						"servicePort": int64(8080),
						"protocol":    "http",
						"loadBalance": "round_robin",
						"timeout":     int64(30),
						"auth": map[string]interface{}{
							"type":     "jwt",
							"required": true,
						},
						"rateLimit": map[string]interface{}{
							"rps":   int64(100),
							"burst": int64(200),
							"key":   "client_ip",
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, route *GatewayRoute) {
				if route.Name != "test-route" {
					t.Errorf("expected name test-route, got %s", route.Name)
				}
				if route.Namespace != "default" {
					t.Errorf("expected namespace default, got %s", route.Namespace)
				}
				if route.UID != types.UID("123-456") {
					t.Errorf("expected UID 123-456, got %s", route.UID)
				}
				if route.Spec.Path != "/api/test" {
					t.Errorf("expected path /api/test, got %s", route.Spec.Path)
				}
				if route.Spec.ServiceName != "test-service" {
					t.Errorf("expected serviceName test-service, got %s", route.Spec.ServiceName)
				}
				if route.Spec.ServicePort != 8080 {
					t.Errorf("expected servicePort 8080, got %d", route.Spec.ServicePort)
				}
				if route.Spec.Protocol != "http" {
					t.Errorf("expected protocol http, got %s", route.Spec.Protocol)
				}
				if route.Spec.LoadBalance != "round_robin" {
					t.Errorf("expected loadBalance round_robin, got %s", route.Spec.LoadBalance)
				}
				if route.Spec.Timeout != 30 {
					t.Errorf("expected timeout 30, got %d", route.Spec.Timeout)
				}
				if route.Spec.Auth == nil {
					t.Error("expected auth to be set")
				} else {
					if route.Spec.Auth.Type != "jwt" {
						t.Errorf("expected auth type jwt, got %s", route.Spec.Auth.Type)
					}
					if !route.Spec.Auth.Required {
						t.Error("expected auth required to be true")
					}
				}
				if route.Spec.RateLimit == nil {
					t.Error("expected rateLimit to be set")
				} else {
					if route.Spec.RateLimit.RPS != 100 {
						t.Errorf("expected rateLimit RPS 100, got %d", route.Spec.RateLimit.RPS)
					}
					if route.Spec.RateLimit.Burst != 200 {
						t.Errorf("expected rateLimit burst 200, got %d", route.Spec.RateLimit.Burst)
					}
					if route.Spec.RateLimit.Key != "client_ip" {
						t.Errorf("expected rateLimit key client_ip, got %s", route.Spec.RateLimit.Key)
					}
				}
			},
		},
		{
			name: "minimal route",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "minimal-route",
					},
					"spec": map[string]interface{}{
						"path":        "/api/minimal",
						"serviceName": "minimal-service",
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, route *GatewayRoute) {
				if route.Name != "minimal-route" {
					t.Errorf("expected name minimal-route, got %s", route.Name)
				}
				if route.Spec.Path != "/api/minimal" {
					t.Errorf("expected path /api/minimal, got %s", route.Spec.Path)
				}
				if route.Spec.ServiceName != "minimal-service" {
					t.Errorf("expected serviceName minimal-service, got %s", route.Spec.ServiceName)
				}
			},
		},
		{
			name: "missing spec",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "no-spec",
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid object type",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{},
				},
			},
			expectError: false, // Will extract metadata fields as empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &GatewayRoute{}
			err := fromUnstructured(tt.input, route)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, route)
				}
			}
		})
	}
}

func TestFromUnstructured_InvalidObjectType(t *testing.T) {
	input := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// Try to convert to wrong type
	var wrongType string
	err := fromUnstructured(input, &wrongType)
	
	if err == nil {
		t.Error("expected error for invalid object type")
	}
	if !contains(err.Error(), "invalid object type") {
		t.Errorf("expected 'invalid object type' error, got: %v", err)
	}
}

// Test structs
func TestGatewayRouteStructs(t *testing.T) {
	// Test creating a complete GatewayRoute
	route := &GatewayRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GatewayGroup + "/" + GatewayVersion,
			Kind:       "GatewayRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "default",
		},
		Spec: GatewayRouteSpec{
			Path:        "/api/test",
			ServiceName: "test-service",
			ServicePort: 8080,
			Protocol:    "http",
			LoadBalance: "round_robin",
			Timeout:     30,
			Auth: &AuthConfig{
				Type:     "jwt",
				Required: true,
				Parameters: map[string]string{
					"issuer": "https://example.com",
				},
			},
			RateLimit: &RateLimitConfig{
				RPS:   100,
				Burst: 200,
				Key:   "client_ip",
			},
			Middleware: []string{"cors", "logging"},
			Metadata: map[string]string{
				"version": "v1",
			},
		},
		Status: GatewayRouteStatus{
			State:           "active",
			LastUpdated:     metav1.Now(),
			ActiveInstances: 3,
		},
	}

	// Verify fields
	if route.Spec.Path != "/api/test" {
		t.Errorf("expected path /api/test, got %s", route.Spec.Path)
	}
	if route.Spec.Auth.Parameters["issuer"] != "https://example.com" {
		t.Error("auth parameters not set correctly")
	}
	if len(route.Spec.Middleware) != 2 {
		t.Errorf("expected 2 middleware, got %d", len(route.Spec.Middleware))
	}
}

// Test constants
func TestConstants(t *testing.T) {
	if GatewayGroup != "gateway.io" {
		t.Errorf("expected GatewayGroup 'gateway.io', got %s", GatewayGroup)
	}
	if GatewayVersion != "v1alpha1" {
		t.Errorf("expected GatewayVersion 'v1alpha1', got %s", GatewayVersion)
	}
}

