package docker

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
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
	// This test doesn't require actual Docker connection
	logger := slog.Default()
	cfg := DefaultConfig()
	
	registry := &Registry{
		config:   cfg,
		logger:   logger,
		services: make(map[string][]core.ServiceInstance),
	}
	
	// Populate test data
	registry.services["test-service"] = []core.ServiceInstance{
		{
			ID:      "container1",
			Name:    "test-service",
			Address: "172.17.0.2",
			Port:    8080,
			Healthy: true,
		},
		{
			ID:      "container2",
			Name:    "test-service",
			Address: "172.17.0.3",
			Port:    8080,
			Healthy: true,
		},
	}
	
	// Test existing service
	instances, err := registry.GetService("test-service")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(instances))
	}
	
	// Test non-existing service
	instances, err = registry.GetService("unknown-service")
	if err == nil {
		t.Error("Expected error for unknown service")
	}
	if len(instances) != 0 {
		t.Errorf("Expected 0 instances for unknown service, got %d", len(instances))
	}
}

func TestRegistry_countInstances(t *testing.T) {
	logger := slog.Default()
	cfg := DefaultConfig()
	
	registry := &Registry{
		config: cfg,
		logger: logger,
	}
	
	tests := []struct {
		name     string
		services map[string][]core.ServiceInstance
		want     int
	}{
		{
			name:     "empty services",
			services: map[string][]core.ServiceInstance{},
			want:     0,
		},
		{
			name: "single service",
			services: map[string][]core.ServiceInstance{
				"service1": {
					{ID: "1", Name: "service1"},
					{ID: "2", Name: "service1"},
				},
			},
			want: 2,
		},
		{
			name: "multiple services",
			services: map[string][]core.ServiceInstance{
				"service1": {
					{ID: "1", Name: "service1"},
					{ID: "2", Name: "service1"},
				},
				"service2": {
					{ID: "3", Name: "service2"},
				},
				"service3": {
					{ID: "4", Name: "service3"},
					{ID: "5", Name: "service3"},
					{ID: "6", Name: "service3"},
				},
			},
			want: 6,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.countInstances(tt.services)
			if got != tt.want {
				t.Errorf("countInstances() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRegistry_Close(t *testing.T) {
	// Skip if Docker is not available
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available, skipping test")
		return
	}
	dockerClient.Close()
	
	logger := slog.Default()
	cfg := DefaultConfig()
	cfg.RefreshInterval = 0 // Disable refresh loop for this test
	
	registry, err := NewRegistry(cfg, logger)
	if err != nil {
		t.Skip("Failed to create registry (Docker not running), skipping test")
		return
	}
	
	// Close the registry
	err = registry.Close()
	if err != nil {
		t.Errorf("Unexpected error on close: %v", err)
	}
}

// Integration tests that require Docker would go here, but are skipped
// as they require Docker daemon to be running

func TestNewRegistry_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Check if Docker is available
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available, skipping integration test")
		return
	}
	dockerClient.Close()
	
	logger := slog.Default()
	cfg := DefaultConfig()
	cfg.RefreshInterval = 1 // Short interval for testing
	
	registry, err := NewRegistry(cfg, logger)
	if err != nil {
		t.Skip("Failed to create registry (Docker not running), skipping integration test")
		return
	}
	defer registry.Close()
	
	// Give it time to complete initial discovery
	time.Sleep(100 * time.Millisecond)
	
	// The actual service discovery would depend on having
	// properly labeled containers running
	// For now, just verify the registry is working
	
	// Try to get a non-existent service
	_, err = registry.GetService("non-existent-service")
	if err == nil {
		t.Error("Expected error for non-existent service")
	}
}

func TestRegistry_getContainerIP(t *testing.T) {
	logger := slog.Default()
	cfg := DefaultConfig()
	
	registry := &Registry{
		config: cfg,
		logger: logger,
	}
	
	// Mock container with various network configurations
	tests := []struct {
		name      string
		container types.Container
		network   string
		wantIP    string
	}{
		{
			name: "bridge network",
			container: types.Container{
				NetworkSettings: &types.SummaryNetworkSettings{
					Networks: map[string]*network.EndpointSettings{
						"bridge": {
							IPAddress: "172.17.0.2",
						},
					},
				},
			},
			network: "",
			wantIP:  "172.17.0.2",
		},
		{
			name: "specific network",
			container: types.Container{
				NetworkSettings: &types.SummaryNetworkSettings{
					Networks: map[string]*network.EndpointSettings{
						"custom": {
							IPAddress: "10.0.0.2",
						},
					},
				},
			},
			network: "custom",
			wantIP:  "10.0.0.2",
		},
		{
			name: "no matching network",
			container: types.Container{
				NetworkSettings: &types.SummaryNetworkSettings{
					Networks: map[string]*network.EndpointSettings{
						"bridge": {
							IPAddress: "172.17.0.2",
						},
					},
				},
			},
			network: "nonexistent",
			wantIP:  "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.getContainerIP(tt.container, tt.network)
			if got != tt.wantIP {
				t.Errorf("getContainerIP() = %s, want %s", got, tt.wantIP)
			}
		})
	}
}