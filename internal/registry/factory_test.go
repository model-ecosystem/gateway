package registry

import (
	"errors"
	"log/slog"
	"testing"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// Mock service registry
type mockServiceRegistry struct {
	getServiceFn func(name string) ([]core.ServiceInstance, error)
}

func (m *mockServiceRegistry) GetService(name string) ([]core.ServiceInstance, error) {
	if m.getServiceFn != nil {
		return m.getServiceFn(name)
	}
	return []core.ServiceInstance{}, nil
}

// Mock lifecycle component
type mockLifecycle struct {
	startCalled bool
	stopCalled  bool
	startErr    error
	stopErr     error
}

func (m *mockLifecycle) Init(parser factory.ConfigParser) error {
	return nil
}

func (m *mockLifecycle) Name() string {
	return "mock-lifecycle"
}

func (m *mockLifecycle) Validate() error {
	return nil
}

func (m *mockLifecycle) Start() error {
	m.startCalled = true
	return m.startErr
}

func (m *mockLifecycle) Stop() error {
	m.stopCalled = true
	return m.stopErr
}

func TestNewComponent(t *testing.T) {
	logger := slog.Default()
	component := NewComponent(logger)

	if component == nil {
		t.Fatal("expected non-nil component")
	}

	if component.Name() != ComponentName {
		t.Errorf("expected component name %s, got %s", ComponentName, component.Name())
	}
}

func TestComponent_Init(t *testing.T) {
	tests := []struct {
		name          string
		config        interface{}
		expectError   bool
		errorContains string
		expectedType  string
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
									ID:      "instance-1",
									Address: "127.0.0.1",
									Port:    8080,
								},
							},
						},
					},
				},
			},
			expectError:  false,
			expectedType: "static",
		},
		// Note: Docker tests are commented out as they require Docker to be running
		// {
		// 	name: "docker registry",
		// 	config: &config.Registry{
		// 		Type: "docker",
		// 		Docker: &config.DockerRegistry{
		// 			Host:            "unix:///var/run/docker.sock",
		// 			RefreshInterval: 10,
		// 		},
		// 	},
		// 	expectError:  false,
		// 	expectedType: "docker",
		// },
		// {
		// 	name: "docker-compose registry",
		// 	config: &config.Registry{
		// 		Type: "docker-compose",
		// 		DockerCompose: &config.DockerComposeRegistry{
		// 			ProjectName:     "test-project",
		// 			RefreshInterval: 10,
		// 		},
		// 	},
		// 	expectError:  false,
		// 	expectedType: "docker-compose",
		// },
		{
			name: "default to static when type not specified",
			config: &config.Registry{
				Static: &config.StaticRegistry{
					Services: []config.Service{
						{
							Name: "test-service",
							Instances: []config.Instance{
								{
									ID:      "instance-1",
									Address: "127.0.0.1",
									Port:    8080,
								},
							},
						},
					},
				},
			},
			expectError:  false,
			expectedType: "static",
		},
		{
			name: "unknown registry type",
			config: &config.Registry{
				Type: "unknown",
			},
			expectError:   true,
			errorContains: "unknown registry type: unknown",
		},
		{
			name: "invalid config parsing",
			config: "invalid-config",
			expectError:   true,
			errorContains: "parse config:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			component := NewComponent(logger).(*Component)

			parser := func(v interface{}) error {
				switch target := v.(type) {
				case *config.Registry:
					if cfg, ok := tt.config.(*config.Registry); ok {
						*target = *cfg
						return nil
					}
					return errors.New("invalid config type")
				default:
					return errors.New("unexpected parser call")
				}
			}

			err := component.Init(parser)

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
				if component.registryType != tt.expectedType {
					t.Errorf("expected registry type %s, got %s", tt.expectedType, component.registryType)
				}
			}
		})
	}
}

func TestComponent_Validate(t *testing.T) {
	tests := []struct {
		name         string
		setupComp    func(*Component)
		expectError  bool
		errorContains string
	}{
		{
			name: "valid component",
			setupComp: func(c *Component) {
				c.registry = &mockServiceRegistry{}
			},
			expectError: false,
		},
		{
			name:          "uninitialized registry",
			setupComp:     func(c *Component) {},
			expectError:   true,
			errorContains: "registry not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			component := NewComponent(logger).(*Component)

			tt.setupComp(component)

			err := component.Validate()

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
			}
		})
	}
}

func TestComponent_Build(t *testing.T) {
	t.Run("valid build", func(t *testing.T) {
		logger := slog.Default()
		component := NewComponent(logger).(*Component)
		mockReg := &mockServiceRegistry{}
		component.registry = mockReg

		reg := component.Build()

		if reg != mockReg {
			t.Error("expected Build to return the initialized registry")
		}
	})

	t.Run("panic on uninitialized", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for uninitialized component")
			}
		}()

		logger := slog.Default()
		component := NewComponent(logger).(*Component)
		component.Build()
	})
}

func TestComponent_Lifecycle(t *testing.T) {
	tests := []struct {
		name        string
		setupComp   func(*Component, *mockLifecycle)
		method      string
		expectCall  bool
		expectError bool
	}{
		{
			name: "start with lifecycle",
			setupComp: func(c *Component, m *mockLifecycle) {
				c.lifecycle = m
			},
			method:     "start",
			expectCall: true,
		},
		{
			name: "start without lifecycle",
			setupComp: func(c *Component, m *mockLifecycle) {
				// No lifecycle set
			},
			method:     "start",
			expectCall: false,
		},
		{
			name: "stop with lifecycle",
			setupComp: func(c *Component, m *mockLifecycle) {
				c.lifecycle = m
			},
			method:     "stop",
			expectCall: true,
		},
		{
			name: "stop without lifecycle",
			setupComp: func(c *Component, m *mockLifecycle) {
				// No lifecycle set
			},
			method:     "stop",
			expectCall: false,
		},
		{
			name: "start with error",
			setupComp: func(c *Component, m *mockLifecycle) {
				c.lifecycle = m
				m.startErr = errors.New("start failed")
			},
			method:      "start",
			expectCall:  true,
			expectError: true,
		},
		{
			name: "stop with error",
			setupComp: func(c *Component, m *mockLifecycle) {
				c.lifecycle = m
				m.stopErr = errors.New("stop failed")
			},
			method:      "stop",
			expectCall:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			component := NewComponent(logger).(*Component)
			mockLife := &mockLifecycle{}

			tt.setupComp(component, mockLife)

			var err error
			switch tt.method {
			case "start":
				err = component.Start()
				if tt.expectCall != mockLife.startCalled {
					t.Errorf("expected Start called=%v, got %v", tt.expectCall, mockLife.startCalled)
				}
			case "stop":
				err = component.Stop()
				if tt.expectCall != mockLife.stopCalled {
					t.Errorf("expected Stop called=%v, got %v", tt.expectCall, mockLife.stopCalled)
				}
			}

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConfigParser(t *testing.T) {
	tests := []struct {
		name        string
		source      interface{}
		target      interface{}
		expectError bool
		errorContains string
	}{
		{
			name: "static registry config",
			source: &config.StaticRegistry{
				Services: []config.Service{
					{Name: "test"},
				},
			},
			target: &config.StaticRegistry{},
			expectError: false,
		},
		{
			name: "docker registry config",
			source: &config.DockerRegistry{
				Host: "unix:///var/run/docker.sock",
			},
			target: &config.DockerRegistry{},
			expectError: false,
		},
		{
			name: "docker-compose registry config",
			source: &config.DockerComposeRegistry{
				ProjectName: "test",
			},
			target: &config.DockerComposeRegistry{},
			expectError: false,
		},
		{
			name:          "nil source for static",
			source:        (*config.StaticRegistry)(nil),
			target:        &config.StaticRegistry{},
			expectError:   true,
			errorContains: "invalid static registry config",
		},
		{
			name:          "wrong type for static",
			source:        "invalid",
			target:        &config.StaticRegistry{},
			expectError:   true,
			errorContains: "invalid static registry config",
		},
		{
			name:          "unsupported target type",
			source:        &config.StaticRegistry{},
			target:        &struct{}{},
			expectError:   true,
			errorContains: "unsupported config type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := configParser(tt.source)
			err := parser(tt.target)

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
			}
		})
	}
}

func TestNewHealthAwareComponent(t *testing.T) {
	logger := slog.Default()
	component := NewHealthAwareComponent(logger)

	if component == nil {
		t.Fatal("expected non-nil component")
	}

	expectedName := "health-aware-" + ComponentName
	if component.Name() != expectedName {
		t.Errorf("expected component name %s, got %s", expectedName, component.Name())
	}
}

func TestHealthAwareComponent_Init(t *testing.T) {
	tests := []struct {
		name        string
		config      interface{}
		expectError bool
	}{
		{
			name: "static registry with health aware",
			config: &config.Registry{
				Type: "static",
				Static: &config.StaticRegistry{
					Services: []config.Service{
						{
							Name: "test-service",
							Instances: []config.Instance{
								{
									ID:      "instance-1",
									Address: "127.0.0.1",
									Port:    8080,
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		// Note: Docker test commented out as it requires Docker to be running
		// {
		// 	name: "docker registry with health aware",
		// 	config: &config.Registry{
		// 		Type: "docker",
		// 		Docker: &config.DockerRegistry{
		// 			Host: "unix:///var/run/docker.sock",
		// 		},
		// 	},
		// 	expectError: false,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			component := NewHealthAwareComponent(logger).(*HealthAwareComponent)

			parser := func(v interface{}) error {
				switch target := v.(type) {
				case *config.Registry:
					if cfg, ok := tt.config.(*config.Registry); ok {
						*target = *cfg
						return nil
					}
					return errors.New("invalid config type")
				default:
					return errors.New("unexpected parser call")
				}
			}

			err := component.Init(parser)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestInterfaceImplementation(t *testing.T) {
	logger := slog.Default()
	
	// Test Component implements factory.Component
	var _ factory.Component = NewComponent(logger)
	
	// Test Component implements factory.Lifecycle
	component := NewComponent(logger).(*Component)
	var _ factory.Lifecycle = component
	
	// Test HealthAwareComponent implements factory.Component
	var _ factory.Component = NewHealthAwareComponent(logger)
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || (len(s) > len(substr) && contains(s[1:], substr)))
}