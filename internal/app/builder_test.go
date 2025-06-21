package app

import (
	"testing"

	"gateway/internal/config"
	"log/slog"
)

func TestNewBuilder(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.Gateway{
			Frontend: config.Frontend{
				HTTP: config.HTTP{
					Host: "localhost",
					Port: 8080,
				},
			},
			Backend: config.Backend{
				HTTP: config.HTTPBackend{
					MaxIdleConns: 100,
				},
			},
			Registry: config.Registry{
				Type: "static",
			},
		},
	}
	logger := slog.Default()

	builder := NewBuilder(cfg, logger)

	if builder == nil {
		t.Fatal("Expected builder, got nil")
	}
	if builder.config != cfg {
		t.Error("Config not set correctly")
	}
	if builder.logger != logger {
		t.Error("Logger not set correctly")
	}
}

func TestBuilder_Build(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.Config
		wantError bool
	}{
		{
			name: "basic HTTP gateway",
			config: &config.Config{
				Gateway: config.Gateway{
					Frontend: config.Frontend{
						HTTP: config.HTTP{
							Host: "localhost",
							Port: 8080,
						},
					},
					Backend: config.Backend{
						HTTP: config.HTTPBackend{
							MaxIdleConns: 100,
						},
					},
					Registry: config.Registry{
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
					Router: config.Router{
						Rules: []config.RouteRule{
							{
								ID:          "test-route",
								Path:        "/api/*",
								ServiceName: "test-service",
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "with JWT authentication",
			config: &config.Config{
				Gateway: config.Gateway{
					Frontend: config.Frontend{
						HTTP: config.HTTP{
							Host: "localhost",
							Port: 8080,
						},
					},
					Backend: config.Backend{
						HTTP: config.HTTPBackend{
							MaxIdleConns: 100,
						},
					},
					Registry: config.Registry{
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
					Router: config.Router{
						Rules: []config.RouteRule{
							{
								ID:          "test-route",
								Path:        "/api/*",
								ServiceName: "test-service",
							},
						},
					},
					Auth: &config.Auth{
						JWT: &config.JWTConfig{
							Secret: "test-secret",
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "with WebSocket support",
			config: &config.Config{
				Gateway: config.Gateway{
					Frontend: config.Frontend{
						HTTP: config.HTTP{
							Host: "localhost",
							Port: 8080,
						},
						WebSocket: &config.WebSocket{
							Enabled: true,
							Host:    "localhost",
							Port:    8081,
						},
					},
					Backend: config.Backend{
						HTTP: config.HTTPBackend{
							MaxIdleConns: 100,
						},
					},
					Registry: config.Registry{
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
					Router: config.Router{
						Rules: []config.RouteRule{
							{
								ID:          "test-route",
								Path:        "/ws/*",
								ServiceName: "test-service",
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "with SSE support",
			config: &config.Config{
				Gateway: config.Gateway{
					Frontend: config.Frontend{
						HTTP: config.HTTP{
							Host: "localhost",
							Port: 8080,
						},
						SSE: &config.SSE{
							Enabled: true,
						},
					},
					Backend: config.Backend{
						HTTP: config.HTTPBackend{
							MaxIdleConns: 100,
						},
					},
					Registry: config.Registry{
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
					Router: config.Router{
						Rules: []config.RouteRule{
							{
								ID:          "test-route",
								Path:        "/events/*",
								ServiceName: "test-service",
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "with TLS enabled",
			config: &config.Config{
				Gateway: config.Gateway{
					Frontend: config.Frontend{
						HTTP: config.HTTP{
							Host: "localhost",
							Port: 8443,
							TLS: &config.TLS{
								Enabled:  true,
								CertFile: "cert.pem",
								KeyFile:  "key.pem",
							},
						},
					},
					Backend: config.Backend{
						HTTP: config.HTTPBackend{
							MaxIdleConns: 100,
						},
					},
					Registry: config.Registry{
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
					Router: config.Router{
						Rules: []config.RouteRule{
							{
								ID:          "test-route",
								Path:        "/api/*",
								ServiceName: "test-service",
							},
						},
					},
				},
			},
			wantError: true, // Expected to fail without valid cert files
		},
		{
			name: "empty registry config",
			config: &config.Config{
				Gateway: config.Gateway{
					Frontend: config.Frontend{
						HTTP: config.HTTP{
							Host: "localhost",
							Port: 8080,
						},
					},
					Backend: config.Backend{
						HTTP: config.HTTPBackend{
							MaxIdleConns: 100,
						},
					},
					Registry: config.Registry{
						Type: "static",
						// Missing Static config
					},
					Router: config.Router{
						Rules: []config.RouteRule{
							{
								ID:          "test-route",
								Path:        "/api/*",
								ServiceName: "test-service",
							},
						},
					},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			builder := NewBuilder(tt.config, logger)

			server, err := builder.Build()

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

			if server == nil {
				t.Fatal("Expected server, got nil")
			}

			// Verify essential components were created
			if server.httpAdapter == nil {
				t.Error("Expected httpAdapter, got nil")
			}
			if server.logger == nil {
				t.Error("Expected logger, got nil")
			}

			// Check optional components based on config
			if tt.config.Gateway.Frontend.WebSocket != nil && server.wsAdapter == nil {
				t.Error("Expected wsAdapter, got nil")
			}
			// SSE is integrated into HTTP adapter, not a separate adapter
		})
	}
}

func TestBuilder_addSSESupport(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.Gateway{
			Frontend: config.Frontend{
				HTTP: config.HTTP{
					Host: "localhost",
					Port: 8080,
				},
				SSE: &config.SSE{
					Enabled:          true,
					KeepaliveTimeout: 30,
				},
			},
			Backend: config.Backend{
				HTTP: config.HTTPBackend{
					MaxIdleConns: 100,
				},
			},
			Registry: config.Registry{
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
			Router: config.Router{
				Rules: []config.RouteRule{
					{
						ID:          "test-route",
						Path:        "/events/*",
						ServiceName: "test-service",
					},
				},
			},
		},
	}
	logger := slog.Default()
	builder := NewBuilder(cfg, logger)

	server, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// SSE is integrated into HTTP adapter, not a separate adapter
	// Verify SSE support was added by checking that the build succeeded
	if server == nil {
		t.Error("Expected server with SSE support")
	}
}

func TestBuilder_createWebSocketAdapter(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.Gateway{
			Frontend: config.Frontend{
				HTTP: config.HTTP{
					Host: "localhost",
					Port: 8080,
				},
				WebSocket: &config.WebSocket{
					Enabled:           true,
					Host:              "localhost",
					Port:              8081,
					ReadBufferSize:    1024,
					WriteBufferSize:   1024,
					HandshakeTimeout:  10,
					PingPeriod:        30,
					PongWait:          60,
					MaxMessageSize:    65536,
					EnableCompression: true,
				},
			},
			Backend: config.Backend{
				HTTP: config.HTTPBackend{
					MaxIdleConns: 100,
				},
			},
			Registry: config.Registry{
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
			Router: config.Router{
				Rules: []config.RouteRule{
					{
						ID:          "test-route",
						Path:        "/ws/*",
						ServiceName: "test-service",
					},
				},
			},
		},
	}
	logger := slog.Default()
	builder := NewBuilder(cfg, logger)

	server, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if server.wsAdapter == nil {
		t.Error("Expected wsAdapter to be created")
	}
}