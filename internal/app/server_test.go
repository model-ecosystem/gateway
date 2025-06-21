package app

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"gateway/internal/config"
	"log/slog"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.Gateway{
			Frontend: config.Frontend{
				HTTP: config.HTTP{
					Host: "localhost",
					Port: 0, // Use 0 to get random available port
				},
			},
			Backend: config.Backend{
				HTTP: config.HTTPBackend{
					MaxIdleConns: 10,
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
	}
	logger := slog.Default()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("Expected server, got nil")
	}
	if server.httpAdapter == nil {
		t.Error("Expected httpAdapter, got nil")
	}
	if server.logger != logger {
		t.Error("Logger not set correctly")
	}
}

func TestServer_StartStop(t *testing.T) {
	// Find available port
	httpPort := findAvailablePort(t)
	
	cfg := &config.Config{
		Gateway: config.Gateway{
			Frontend: config.Frontend{
				HTTP: config.HTTP{
					Host:         "localhost",
					Port:         httpPort,
					ReadTimeout:  5,
					WriteTimeout: 5,
				},
			},
			Backend: config.Backend{
				HTTP: config.HTTPBackend{
					MaxIdleConns: 10,
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
	}
	logger := slog.Default()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- server.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Check if server is listening
	conn, err := net.Dial("tcp", net.JoinHostPort("localhost", fmt.Sprintf("%d", httpPort)))
	if err != nil {
		t.Errorf("Server not listening on expected port: %v", err)
	} else {
		conn.Close()
	}

	// Stop server
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	err = server.Stop(stopCtx)
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	// Check that Start() returns after Stop()
	select {
	case err := <-startErrCh:
		if err != nil && err != context.Canceled {
			t.Errorf("Start returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after Stop")
	}
}

func TestServer_StartStop_WithWebSocket(t *testing.T) {
	// Find available ports
	httpPort := findAvailablePort(t)
	wsPort := findAvailablePort(t)
	
	cfg := &config.Config{
		Gateway: config.Gateway{
			Frontend: config.Frontend{
				HTTP: config.HTTP{
					Host:         "localhost",
					Port:         httpPort,
					ReadTimeout:  5,
					WriteTimeout: 5,
				},
				WebSocket: &config.WebSocket{
					Enabled: true,
					Host:    "localhost",
					Port:    wsPort,
				},
			},
			Backend: config.Backend{
				HTTP: config.HTTPBackend{
					MaxIdleConns: 10,
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

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- server.Start(ctx)
	}()

	// Wait for servers to start
	time.Sleep(100 * time.Millisecond)

	// Check if HTTP server is listening
	httpConn, err := net.Dial("tcp", net.JoinHostPort("localhost", fmt.Sprintf("%d", httpPort)))
	if err != nil {
		t.Errorf("HTTP server not listening on expected port: %v", err)
	} else {
		httpConn.Close()
	}

	// Check if WebSocket server is listening
	wsConn, err := net.Dial("tcp", net.JoinHostPort("localhost", fmt.Sprintf("%d", wsPort)))
	if err != nil {
		t.Errorf("WebSocket server not listening on expected port: %v", err)
	} else {
		wsConn.Close()
	}

	// Stop server
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	err = server.Stop(stopCtx)
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	// Check that Start() returns after Stop()
	select {
	case err := <-startErrCh:
		if err != nil && err != context.Canceled {
			t.Errorf("Start returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after Stop")
	}
}

func TestServer_BuildError(t *testing.T) {
	// Test configuration that should fail during build
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
					MaxIdleConns: 10,
				},
			},
			Registry: config.Registry{
				Type: "invalid-registry-type", // This should cause build to fail
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
	}
	logger := slog.Default()

	_, err := NewServer(cfg, logger)
	if err == nil {
		t.Error("Expected error when creating server with invalid registry type")
	}
}

// Helper function to find an available port
func findAvailablePort(t *testing.T) int {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	defer listener.Close()
	
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}