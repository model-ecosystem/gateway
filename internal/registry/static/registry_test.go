package static

import (
	"gateway/internal/config"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.StaticRegistry
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &config.StaticRegistry{
				Services: []config.Service{
					{
						Name: "test-service",
						Instances: []config.Instance{
							{
								ID:      "instance-1",
								Address: "127.0.0.1",
								Port:    8001,
								Health:  "healthy",
							},
							{
								ID:      "instance-2",
								Address: "127.0.0.1",
								Port:    8002,
								Health:  "unhealthy",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty services",
			config: &config.StaticRegistry{
				Services: []config.Service{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := NewRegistry(tt.config)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRegistry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && registry == nil {
				t.Error("NewRegistry() returned nil registry")
			}
		})
	}
}

func TestRegistryGetService(t *testing.T) {
	cfg := &config.StaticRegistry{
		Services: []config.Service{
			{
				Name: "test-service",
				Instances: []config.Instance{
					{
						ID:      "instance-1",
						Address: "10.0.0.1",
						Port:    8080,
						Health:  "healthy",
					},
					{
						ID:      "instance-2",
						Address: "10.0.0.2",
						Port:    8080,
						Health:  "healthy",
					},
					{
						ID:      "instance-3",
						Address: "10.0.0.3",
						Port:    8080,
						Health:  "unhealthy",
					},
				},
			},
			{
				Name:      "empty-service",
				Instances: []config.Instance{},
			},
		},
	}

	registry, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	tests := []struct {
		name         string
		serviceName  string
		wantCount    int
		wantErr      bool
		checkHealthy bool
	}{
		{
			name:         "existing service",
			serviceName:  "test-service",
			wantCount:    3, // All instances returned, health filtering happens at registry interface level
			wantErr:      false,
			checkHealthy: true,
		},
		{
			name:        "non-existent service",
			serviceName: "non-existent",
			wantCount:   0,
			wantErr:     true,
		},
		{
			name:        "empty service",
			serviceName: "empty-service",
			wantCount:   0,
			wantErr:     false,
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
			
			// Verify instance conversion
			if tt.checkHealthy && len(instances) > 0 {
				for _, inst := range instances {
					if inst.Name != tt.serviceName {
						t.Errorf("Instance has wrong service name: got %s, want %s", inst.Name, tt.serviceName)
					}
				}
			}
		})
	}
}

func TestInstanceConversion(t *testing.T) {
	tests := []struct {
		name           string
		instance       config.Instance
		serviceName    string
		expectedHealthy bool
	}{
		{
			name: "healthy instance",
			instance: config.Instance{
				ID:      "test-1",
				Address: "192.168.1.1",
				Port:    8080,
				Health:  "healthy",
			},
			serviceName:    "test-service",
			expectedHealthy: true,
		},
		{
			name: "unhealthy instance",
			instance: config.Instance{
				ID:      "test-2",
				Address: "192.168.1.2",
				Port:    8080,
				Health:  "unhealthy",
			},
			serviceName:    "test-service",
			expectedHealthy: false,
		},
		{
			name: "unknown health status",
			instance: config.Instance{
				ID:      "test-3",
				Address: "192.168.1.3",
				Port:    8080,
				Health:  "unknown",
			},
			serviceName:    "test-service",
			expectedHealthy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceInstance := tt.instance.ToServiceInstance(tt.serviceName)
			
			if serviceInstance.ID != tt.instance.ID {
				t.Errorf("ID mismatch: got %s, want %s", serviceInstance.ID, tt.instance.ID)
			}
			
			if serviceInstance.Name != tt.serviceName {
				t.Errorf("Name mismatch: got %s, want %s", serviceInstance.Name, tt.serviceName)
			}
			
			if serviceInstance.Address != tt.instance.Address {
				t.Errorf("Address mismatch: got %s, want %s", serviceInstance.Address, tt.instance.Address)
			}
			
			if serviceInstance.Port != tt.instance.Port {
				t.Errorf("Port mismatch: got %d, want %d", serviceInstance.Port, tt.instance.Port)
			}
			
			if serviceInstance.Healthy != tt.expectedHealthy {
				t.Errorf("Healthy mismatch: got %v, want %v", serviceInstance.Healthy, tt.expectedHealthy)
			}
		})
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	cfg := &config.StaticRegistry{
		Services: []config.Service{
			{
				Name: "concurrent-service",
				Instances: []config.Instance{
					{
						ID:      "concurrent-1",
						Address: "127.0.0.1",
						Port:    8080,
						Health:  "healthy",
					},
				},
			},
		},
	}

	registry, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Run concurrent GetService calls
	done := make(chan bool)
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		go func() {
			_, err := registry.GetService("concurrent-service")
			if err != nil {
				errors <- err
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Check for errors
	select {
	case err := <-errors:
		t.Fatalf("Concurrent access failed: %v", err)
	default:
		// No errors, test passed
	}
}