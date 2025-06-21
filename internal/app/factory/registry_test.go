package factory

import (
	"testing"

	"gateway/internal/config"
	"log/slog"
)

func TestCreateRegistry(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name      string
		config    *config.Registry
		wantError bool
		wantType  string
	}{
		{
			name: "static registry",
			config: &config.Registry{
				Type: "static",
				Static: &config.StaticRegistry{
					Services: []config.Service{
						{
							Name: "test-service",
							Instances: []config.Instance{
								{
									ID:      "test-1",
									Address: "localhost",
									Port:    3000,
									Health:  "healthy",
								},
							},
						},
					},
				},
			},
			wantError: false,
			wantType:  "static",
		},
		// Skip docker registry test as it requires Docker daemon to be running
		// {
		// 	name: "docker registry",
		// 	config: &config.Registry{
		// 		Type: "docker",
		// 		Docker: &config.DockerRegistry{
		// 			Host:            "unix:///var/run/docker.sock",
		// 			Version:         "1.41",
		// 			LabelPrefix:     "gateway",
		// 			RefreshInterval: 30,
		// 		},
		// 	},
		// 	wantError: false,
		// 	wantType:  "docker",
		// },
		{
			name: "unknown registry type",
			config: &config.Registry{
				Type: "unknown",
			},
			wantError: true,
		},
		{
			name: "static registry without config",
			config: &config.Registry{
				Type: "static",
			},
			wantError: true,
		},
		{
			name: "docker registry without config",
			config: &config.Registry{
				Type: "docker",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := CreateRegistry(tt.config, logger)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if registry == nil {
				t.Error("Expected registry, got nil")
			}
		})
	}
}

func TestCreateStaticRegistry(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name      string
		config    *config.StaticRegistry
		wantError bool
	}{
		{
			name: "valid config",
			config: &config.StaticRegistry{
				Services: []config.Service{
					{
						Name: "service1",
						Instances: []config.Instance{
							{
								ID:      "instance1",
								Address: "localhost",
								Port:    8080,
								Health:  "healthy",
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "empty services",
			config: &config.StaticRegistry{
				Services: []config.Service{},
			},
			wantError: false,
		},
		{
			name: "multiple services",
			config: &config.StaticRegistry{
				Services: []config.Service{
					{
						Name: "service1",
						Instances: []config.Instance{
							{
								ID:      "instance1",
								Address: "localhost",
								Port:    8080,
								Health:  "healthy",
							},
						},
					},
					{
						Name: "service2",
						Instances: []config.Instance{
							{
								ID:      "instance2",
								Address: "localhost",
								Port:    8081,
								Health:  "healthy",
							},
							{
								ID:      "instance3",
								Address: "localhost",
								Port:    8082,
								Health:  "unhealthy",
							},
						},
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := createStaticRegistry(tt.config, logger)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if registry == nil {
				t.Error("Expected registry, got nil")
			}

			// Verify services can be retrieved
			for _, svc := range tt.config.Services {
				instances, err := registry.GetService(svc.Name)
				if err != nil {
					t.Errorf("Failed to get service %s: %v", svc.Name, err)
					continue
				}
				if len(instances) != len(svc.Instances) {
					t.Errorf("Service %s: expected %d instances, got %d", svc.Name, len(svc.Instances), len(instances))
				}
			}
		})
	}
}