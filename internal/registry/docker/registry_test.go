package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gateway/internal/core"
	"log/slog"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	if cfg.LabelPrefix != "gateway" {
		t.Errorf("Expected label prefix 'gateway', got '%s'", cfg.LabelPrefix)
	}

	if cfg.RefreshInterval != 10 {
		t.Errorf("Expected refresh interval 10, got %d", cfg.RefreshInterval)
	}
}

func TestRegistry_GetService(t *testing.T) {
	logger := slog.Default()

	// Create a registry with mock data
	registry := &Registry{
		config: DefaultConfig(),
		logger: logger,
		services: map[string][]core.ServiceInstance{
			"test-service": {
				{
					ID:      "instance-1",
					Name:    "test-service",
					Address: "10.0.0.1",
					Port:    8080,
					Scheme:  "http",
					Healthy: true,
				},
				{
					ID:      "instance-2",
					Name:    "test-service",
					Address: "10.0.0.2",
					Port:    8080,
					Scheme:  "http",
					Healthy: true,
				},
			},
		},
	}

	tests := []struct {
		name        string
		serviceName string
		wantErr     bool
		wantCount   int
	}{
		{
			name:        "existing service",
			serviceName: "test-service",
			wantErr:     false,
			wantCount:   2,
		},
		{
			name:        "non-existent service",
			serviceName: "non-existent",
			wantErr:     true,
			wantCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instances, err := registry.GetService(tt.serviceName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(instances) != tt.wantCount {
				t.Errorf("GetService() returned %d instances, want %d", len(instances), tt.wantCount)
			}
		})
	}
}

func TestRegistry_HTTPDiscovery(t *testing.T) {
	// Create a mock Docker API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_ping":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))

		case "/containers/json":
			// Return mock container data
			containers := []Container{
				{
					ID:    "abc123def456789",
					Names: []string{"/service1"},
					State: "running",
					Labels: map[string]string{
						"gateway.service":      "test-service",
						"gateway.port":         "8080",
						"gateway.scheme":       "http",
						"gateway.health":       "healthy",
						"gateway.meta.version": "1.0.0",
					},
					NetworkSettings: struct {
						Networks map[string]struct {
							IPAddress string `json:"IPAddress"`
						} `json:"Networks"`
					}{
						Networks: map[string]struct {
							IPAddress string `json:"IPAddress"`
						}{
							"bridge": {IPAddress: "172.17.0.2"},
						},
					},
				},
				{
					ID:    "def456ghi789012",
					Names: []string{"/service2"},
					State: "running",
					Labels: map[string]string{
						"gateway.service": "test-service",
						"gateway.port":    "8080",
						"gateway.scheme":  "https",
					},
					NetworkSettings: struct {
						Networks map[string]struct {
							IPAddress string `json:"IPAddress"`
						} `json:"Networks"`
					}{
						Networks: map[string]struct {
							IPAddress string `json:"IPAddress"`
						}{
							"bridge": {IPAddress: "172.17.0.3"},
						},
					},
				},
				{
					ID:    "ghi789jkl012345",
					Names: []string{"/other-service"},
					State: "running",
					Labels: map[string]string{
						"some.other.label": "value",
					},
					NetworkSettings: struct {
						Networks map[string]struct {
							IPAddress string `json:"IPAddress"`
						} `json:"Networks"`
					}{
						Networks: map[string]struct {
							IPAddress string `json:"IPAddress"`
						}{
							"bridge": {IPAddress: "172.17.0.4"},
						},
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(containers)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create registry with mock server
	cfg := DefaultConfig()
	cfg.Host = server.URL
	cfg.RefreshInterval = 0 // Disable auto-refresh for test

	registry, err := NewRegistry(cfg, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Close()

	// Test service discovery
	instances, err := registry.GetService("test-service")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if len(instances) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(instances))
	}

	// Verify first instance
	if instances[0].ID != "abc123def456" {
		t.Errorf("Expected instance ID 'abc123def456', got '%s'", instances[0].ID)
	}
	if instances[0].Address != "172.17.0.2" {
		t.Errorf("Expected address '172.17.0.2', got '%s'", instances[0].Address)
	}
	if instances[0].Port != 8080 {
		t.Errorf("Expected port 8080, got %d", instances[0].Port)
	}
	if instances[0].Scheme != "http" {
		t.Errorf("Expected scheme 'http', got '%s'", instances[0].Scheme)
	}
	if instances[0].Healthy != true {
		t.Errorf("Expected healthy=true, got %v", instances[0].Healthy)
	}
	if instances[0].Metadata["version"] != "1.0.0" {
		t.Errorf("Expected metadata version='1.0.0', got '%v'", instances[0].Metadata["version"])
	}

	// Verify second instance
	if instances[1].Scheme != "https" {
		t.Errorf("Expected scheme 'https', got '%s'", instances[1].Scheme)
	}
}

func TestRegistry_ContainerFiltering(t *testing.T) {
	// Test various container states and label configurations
	testCases := []struct {
		name          string
		container     Container
		expectInclude bool
		expectError   bool
	}{
		{
			name: "valid container",
			container: Container{
				ID:    "abc123",
				State: "running",
				Labels: map[string]string{
					"gateway.service": "test",
					"gateway.port":    "8080",
				},
				NetworkSettings: struct {
					Networks map[string]struct {
						IPAddress string `json:"IPAddress"`
					} `json:"Networks"`
				}{
					Networks: map[string]struct {
						IPAddress string `json:"IPAddress"`
					}{
						"bridge": {IPAddress: "172.17.0.2"},
					},
				},
			},
			expectInclude: true,
		},
		{
			name: "stopped container",
			container: Container{
				ID:    "def456",
				State: "exited",
				Labels: map[string]string{
					"gateway.service": "test",
					"gateway.port":    "8080",
				},
			},
			expectInclude: false,
		},
		{
			name: "missing service label",
			container: Container{
				ID:    "ghi789",
				State: "running",
				Labels: map[string]string{
					"gateway.port": "8080",
				},
			},
			expectInclude: false,
		},
		{
			name: "missing port label",
			container: Container{
				ID:    "jkl012",
				State: "running",
				Labels: map[string]string{
					"gateway.service": "test",
				},
			},
			expectInclude: false,
		},
		{
			name: "invalid port",
			container: Container{
				ID:    "mno345",
				State: "running",
				Labels: map[string]string{
					"gateway.service": "test",
					"gateway.port":    "not-a-number",
				},
			},
			expectInclude: false,
		},
		{
			name: "no IP address",
			container: Container{
				ID:    "pqr678",
				State: "running",
				Labels: map[string]string{
					"gateway.service": "test",
					"gateway.port":    "8080",
				},
				NetworkSettings: struct {
					Networks map[string]struct {
						IPAddress string `json:"IPAddress"`
					} `json:"Networks"`
				}{
					Networks: map[string]struct {
						IPAddress string `json:"IPAddress"`
					}{},
				},
			},
			expectInclude: false,
		},
	}

	// Test each case with a mock server
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/_ping":
					w.WriteHeader(http.StatusOK)
				case "/containers/json":
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode([]Container{tc.container})
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			cfg := DefaultConfig()
			cfg.Host = server.URL
			cfg.RefreshInterval = 0

			registry, err := NewRegistry(cfg, slog.Default())
			if err != nil {
				t.Fatalf("Failed to create registry: %v", err)
			}
			defer registry.Close()

			instances, err := registry.GetService("test")

			if tc.expectInclude {
				if err != nil {
					t.Errorf("Expected to find service, got error: %v", err)
				}
				if len(instances) != 1 {
					t.Errorf("Expected 1 instance, got %d", len(instances))
				}
			} else {
				if err == nil && len(instances) > 0 {
					t.Errorf("Expected no instances, got %d", len(instances))
				}
			}
		})
	}
}

func TestRegistry_APIErrors(t *testing.T) {
	testCases := []struct {
		name           string
		pingStatus     int
		listStatus     int
		listResponse   string
		expectNewError bool
	}{
		{
			name:           "ping fails",
			pingStatus:     http.StatusInternalServerError,
			expectNewError: true,
		},
		{
			name:         "list fails",
			pingStatus:   http.StatusOK,
			listStatus:   http.StatusInternalServerError,
			listResponse: "Internal Server Error",
		},
		{
			name:         "invalid JSON response",
			pingStatus:   http.StatusOK,
			listStatus:   http.StatusOK,
			listResponse: "not-json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/_ping":
					w.WriteHeader(tc.pingStatus)
				case "/containers/json":
					w.WriteHeader(tc.listStatus)
					if tc.listResponse != "" {
						w.Write([]byte(tc.listResponse))
					}
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			cfg := DefaultConfig()
			cfg.Host = server.URL
			cfg.RefreshInterval = 0

			registry, err := NewRegistry(cfg, slog.Default())

			if tc.expectNewError {
				if err == nil {
					t.Error("Expected error creating registry, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error creating registry: %v", err)
			}
			defer registry.Close()

			// Try to get a service - should handle errors gracefully
			_, err = registry.GetService("test")
			if err == nil {
				t.Error("Expected error getting service after API failure")
			}
		})
	}
}

func TestRegistry_NetworkFiltering(t *testing.T) {
	containers := []Container{
		{
			ID:    "container1",
			State: "running",
			Labels: map[string]string{
				"gateway.service": "test",
				"gateway.port":    "8080",
			},
			NetworkSettings: struct {
				Networks map[string]struct {
					IPAddress string `json:"IPAddress"`
				} `json:"Networks"`
			}{
				Networks: map[string]struct {
					IPAddress string `json:"IPAddress"`
				}{
					"bridge": {IPAddress: "172.17.0.2"},
					"custom": {IPAddress: "10.0.0.2"},
				},
			},
		},
		{
			ID:    "container2",
			State: "running",
			Labels: map[string]string{
				"gateway.service": "test",
				"gateway.port":    "8080",
			},
			NetworkSettings: struct {
				Networks map[string]struct {
					IPAddress string `json:"IPAddress"`
				} `json:"Networks"`
			}{
				Networks: map[string]struct {
					IPAddress string `json:"IPAddress"`
				}{
					"custom": {IPAddress: "10.0.0.3"},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_ping":
			w.WriteHeader(http.StatusOK)
		case "/containers/json":
			// Check if network filter is applied
			filters := r.URL.Query().Get("filters")
			if filters != "" && strings.Contains(filters, "\"network\":[\"custom\"]") {
				// When custom network filter is applied, only return container2
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]Container{containers[1]})
			} else {
				// Return all containers
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(containers)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test without network filter
	cfg1 := DefaultConfig()
	cfg1.Host = server.URL
	cfg1.RefreshInterval = 0

	registry1, err := NewRegistry(cfg1, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry1.Close()

	instances1, err := registry1.GetService("test")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if len(instances1) != 2 {
		t.Errorf("Without network filter: expected 2 instances, got %d", len(instances1))
	}

	// Test with network filter
	cfg2 := DefaultConfig()
	cfg2.Host = server.URL
	cfg2.Network = "custom"
	cfg2.RefreshInterval = 0

	registry2, err := NewRegistry(cfg2, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create registry with network filter: %v", err)
	}
	defer registry2.Close()

	instances2, err := registry2.GetService("test")
	if err != nil {
		t.Fatalf("Failed to get service with network filter: %v", err)
	}

	if len(instances2) != 1 {
		t.Errorf("With network filter: expected 1 instance, got %d", len(instances2))
	}

	if len(instances2) > 0 && instances2[0].Address != "10.0.0.3" {
		t.Errorf("Expected instance from custom network (10.0.0.3), got %s", instances2[0].Address)
	}
}

func TestRegistry_RefreshLoop(t *testing.T) {
	updateCount := 0
	containers := []Container{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_ping":
			w.WriteHeader(http.StatusOK)
		case "/containers/json":
			updateCount++
			// Return different data on each refresh
			if updateCount == 1 {
				containers = []Container{
					{
						ID:    "initial",
						State: "running",
						Labels: map[string]string{
							"gateway.service": "test",
							"gateway.port":    "8080",
						},
						NetworkSettings: struct {
							Networks map[string]struct {
								IPAddress string `json:"IPAddress"`
							} `json:"Networks"`
						}{
							Networks: map[string]struct {
								IPAddress string `json:"IPAddress"`
							}{
								"bridge": {IPAddress: "172.17.0.2"},
							},
						},
					},
				}
			} else {
				containers = append(containers, Container{
					ID:    fmt.Sprintf("new-%d", updateCount),
					State: "running",
					Labels: map[string]string{
						"gateway.service": "test",
						"gateway.port":    "8080",
					},
					NetworkSettings: struct {
						Networks map[string]struct {
							IPAddress string `json:"IPAddress"`
						} `json:"Networks"`
					}{
						Networks: map[string]struct {
							IPAddress string `json:"IPAddress"`
						}{
							"bridge": {IPAddress: fmt.Sprintf("172.17.0.%d", updateCount+1)},
						},
					},
				})
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(containers)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Host = server.URL
	cfg.RefreshInterval = 1 // 1 second refresh

	registry, err := NewRegistry(cfg, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Close()

	// Initial check
	instances1, err := registry.GetService("test")
	if err != nil {
		t.Fatalf("Failed to get service initially: %v", err)
	}
	if len(instances1) != 1 {
		t.Errorf("Initial: expected 1 instance, got %d", len(instances1))
	}

	// Wait for refresh
	time.Sleep(1500 * time.Millisecond)

	// Check again - should have more instances
	instances2, err := registry.GetService("test")
	if err != nil {
		t.Fatalf("Failed to get service after refresh: %v", err)
	}
	if len(instances2) <= len(instances1) {
		t.Errorf("After refresh: expected more than %d instances, got %d", len(instances1), len(instances2))
	}

	if updateCount < 2 {
		t.Errorf("Expected at least 2 updates, got %d", updateCount)
	}
}

func TestRegistry_Close(t *testing.T) {
	logger := slog.Default()
	cfg := DefaultConfig()
	cfg.RefreshInterval = 0 // Disable refresh loop for this test

	// Create a minimal registry without Docker connection
	registry := &Registry{
		config:   cfg,
		logger:   logger,
		services: make(map[string][]core.ServiceInstance),
		stopCh:   make(chan struct{}),
	}

	// Close the registry
	err := registry.Close()
	if err != nil {
		t.Errorf("Unexpected error on close: %v", err)
	}
}

func TestContainer_Parsing(t *testing.T) {
	// Test container JSON parsing
	container := Container{
		ID:    "abc123def456",
		Names: []string{"/my-service"},
		Labels: map[string]string{
			"gateway.service": "my-service",
			"gateway.port":    "8080",
			"gateway.health":  "healthy",
		},
		State: "running",
	}

	// Test short ID
	if len(container.ID) >= 12 {
		shortID := container.ID[:12]
		if len(shortID) != 12 {
			t.Errorf("Expected short ID length 12, got %d", len(shortID))
		}
	}

	// Test label extraction
	serviceName := container.Labels["gateway.service"]
	if serviceName != "my-service" {
		t.Errorf("Expected service name 'my-service', got '%s'", serviceName)
	}
}

func TestRegistry_countInstances(t *testing.T) {
	registry := &Registry{}

	services := map[string][]core.ServiceInstance{
		"service1": {
			{ID: "1", Name: "service1"},
			{ID: "2", Name: "service1"},
		},
		"service2": {
			{ID: "3", Name: "service2"},
		},
		"service3": {},
	}

	count := registry.countInstances(services)
	if count != 3 {
		t.Errorf("Expected 3 instances, got %d", count)
	}
}
