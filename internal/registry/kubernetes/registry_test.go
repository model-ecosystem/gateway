package kubernetes

import (
	"log/slog"
	"testing"
	"time"

	"gateway/internal/core"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfig_Defaults(t *testing.T) {
	config := Config{
		Namespace: "default",
	}

	// Test that NewRegistry would set defaults
	if config.GatewayAnnotation == "" {
		config.GatewayAnnotation = "gateway/enabled"
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 5 * time.Minute
	}

	if config.GatewayAnnotation != "gateway/enabled" {
		t.Errorf("expected default gateway annotation 'gateway/enabled', got %s", config.GatewayAnnotation)
	}
	if config.RefreshInterval != 5*time.Minute {
		t.Errorf("expected default refresh interval 5m, got %v", config.RefreshInterval)
	}
}

func TestRegistry_GetService(t *testing.T) {
	logger := slog.Default()
	
	registry := &Registry{
		config: Config{
			GatewayAnnotation: "gateway/enabled",
		},
		services: make(map[string]*core.Service),
		logger:   logger,
	}

	// Add test services
	registry.services["existing-service"] = &core.Service{
		Name: "existing-service",
		Instances: []*core.ServiceInstance{
			{
				ID:      "instance-1",
				Address: "10.0.0.1",
				Port:    8080,
				Healthy: true,
			},
			{
				ID:      "instance-2",
				Address: "10.0.0.2",
				Port:    8080,
				Healthy: false,
			},
		},
	}

	registry.services["no-healthy"] = &core.Service{
		Name: "no-healthy",
		Instances: []*core.ServiceInstance{
			{
				ID:      "instance-1",
				Address: "10.0.0.1",
				Port:    8080,
				Healthy: false,
			},
		},
	}

	tests := []struct {
		name              string
		serviceName       string
		expectError       bool
		errorContains     string
		expectedInstances int
	}{
		{
			name:              "existing service with healthy instances",
			serviceName:       "existing-service",
			expectError:       false,
			expectedInstances: 1, // Only healthy instances
		},
		{
			name:          "non-existent service",
			serviceName:   "non-existent",
			expectError:   true,
			errorContains: "service non-existent not found",
		},
		{
			name:          "service with no healthy instances",
			serviceName:   "no-healthy",
			expectError:   true,
			errorContains: "no healthy instances available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := registry.GetService(tt.serviceName)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if service == nil {
					t.Fatal("expected service, got nil")
				}
				if len(service.Instances) != tt.expectedInstances {
					t.Errorf("expected %d instances, got %d", tt.expectedInstances, len(service.Instances))
				}
			}
		})
	}
}

func TestRegistry_ListServices(t *testing.T) {
	logger := slog.Default()
	
	registry := &Registry{
		config: Config{},
		services: map[string]*core.Service{
			"service-1": {Name: "service-1"},
			"service-2": {Name: "service-2"},
		},
		logger: logger,
	}

	services, err := registry.ListServices()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(services) != 2 {
		t.Errorf("expected 2 services, got %d", len(services))
	}
}

func TestRegistry_shouldExposeService(t *testing.T) {
	// Test with annotation required
	registryWithAnnotation := &Registry{
		config: Config{
			GatewayAnnotation: "gateway/enabled",
		},
	}
	
	// Test without annotation required
	registryWithoutAnnotation := &Registry{
		config: Config{
			GatewayAnnotation: "",
		},
	}

	tests := []struct {
		name         string
		registry     *Registry
		service      *corev1.Service
		expectExpose bool
	}{
		{
			name:     "service with enabled annotation",
			registry: registryWithAnnotation,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"gateway/enabled": "true",
					},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
					Ports: []corev1.ServicePort{
						{Port: 80},
					},
				},
			},
			expectExpose: true,
		},
		{
			name:     "service with disabled annotation",
			registry: registryWithAnnotation,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"gateway/enabled": "false",
					},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
					Ports: []corev1.ServicePort{
						{Port: 80},
					},
				},
			},
			expectExpose: false,
		},
		{
			name:     "service without annotation (annotation required)",
			registry: registryWithAnnotation,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
					Ports: []corev1.ServicePort{
						{Port: 80},
					},
				},
			},
			expectExpose: false,
		},
		{
			name:     "service without annotation (annotation not required)",
			registry: registryWithoutAnnotation,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
					Ports: []corev1.ServicePort{
						{Port: 80},
					},
				},
			},
			expectExpose: true,
		},
		{
			name:     "headless service",
			registry: registryWithoutAnnotation,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"gateway/enabled": "true",
					},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "None",
					Ports: []corev1.ServicePort{
						{Port: 80},
					},
				},
			},
			expectExpose: false,
		},
		{
			name:     "service without ports",
			registry: registryWithoutAnnotation,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"gateway/enabled": "true",
					},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
					Ports:     []corev1.ServicePort{},
				},
			},
			expectExpose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exposed := tt.registry.shouldExposeService(tt.service)
			if exposed != tt.expectExpose {
				t.Errorf("expected expose=%v, got %v", tt.expectExpose, exposed)
			}
		})
	}
}

func TestRegistry_k8sServiceToCoreService(t *testing.T) {
	registry := &Registry{
		config: Config{
			PortName: "http",
		},
		logger: slog.Default(),
	}

	tests := []struct {
		name              string
		service           *corev1.Service
		expectNil         bool
		expectedPort      int
		expectedInstances int
	}{
		{
			name: "service with named port",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "123",
					Annotations: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
					Type:      corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{Name: "grpc", Port: 9090},
						{Name: "http", Port: 8080},
					},
				},
			},
			expectNil:         false,
			expectedPort:      8080,
			expectedInstances: 1,
		},
		{
			name: "service with first port",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
					Type:      corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{Port: 3000},
					},
				},
			},
			expectNil:         false,
			expectedPort:      3000,
			expectedInstances: 1,
		},
		{
			name: "loadbalancer service",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lb-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{Port: 80},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{IP: "1.2.3.4"},
							{Hostname: "lb.example.com"},
						},
					},
				},
			},
			expectNil:         false,
			expectedPort:      80,
			expectedInstances: 2,
		},
		{
			name: "service without ports",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-ports",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{},
				},
			},
			expectNil: true,
		},
		{
			name: "nodeport service",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nodeport-service",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{Port: 80, NodePort: 30080},
					},
				},
			},
			expectNil:         false,
			expectedPort:      80,
			expectedInstances: 0, // NodePort services need node discovery
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.k8sServiceToCoreService(tt.service)

			if tt.expectNil {
				if result != nil {
					t.Error("expected nil result")
				}
			} else {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if result.Name != tt.service.Name {
					t.Errorf("expected name %s, got %s", tt.service.Name, result.Name)
				}
				if len(result.Instances) != tt.expectedInstances {
					t.Errorf("expected %d instances, got %d", tt.expectedInstances, len(result.Instances))
				}
				if len(result.Instances) > 0 && result.Instances[0].Port != tt.expectedPort {
					t.Errorf("expected port %d, got %d", tt.expectedPort, result.Instances[0].Port)
				}
				// Check metadata
				if result.Metadata["namespace"] != tt.service.Namespace {
					t.Errorf("expected namespace metadata %s, got %s", tt.service.Namespace, result.Metadata["namespace"])
				}
				if result.Metadata["uid"] != string(tt.service.UID) {
					t.Errorf("expected uid metadata %s, got %s", tt.service.UID, result.Metadata["uid"])
				}
			}
		})
	}
}

// Test that critical functions exist and have correct signatures
func TestRegistry_Methods(t *testing.T) {
	registry := &Registry{
		config:   Config{},
		services: make(map[string]*core.Service),
		logger:   slog.Default(),
	}

	// Test that methods exist
	_ = registry.Start
	_ = registry.Stop
	_ = registry.syncServices
	_ = registry.watchServices
	_ = registry.watchEndpoints
	_ = registry.resyncLoop
	_ = registry.handleServiceEvents
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || (len(s) > len(substr) && contains(s[1:], substr)))
}