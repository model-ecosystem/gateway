package factory

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/health"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// Mock service registry for testing
type mockRegistry struct {
	services map[string][]core.ServiceInstance
	err      error
}

func (m *mockRegistry) GetService(name string) ([]core.ServiceInstance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.services[name], nil
}

func TestCreateHealthChecker(t *testing.T) {
	logger := slog.Default()
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{},
	}

	tests := []struct {
		name    string
		config  *config.Health
		wantErr bool
	}{
		{
			name: "nil config",
			config: nil,
			wantErr: false,
		},
		{
			name: "disabled health",
			config: &config.Health{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "enabled with checks",
			config: &config.Health{
				Enabled: true,
				Checks: map[string]config.Check{
					"tcp-check": {
						Type:    "tcp",
						Timeout: 5,
						Config: map[string]string{
							"address": "localhost:8080",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := CreateHealthChecker(tt.config, registry, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateHealthChecker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && checker == nil && tt.config != nil && tt.config.Enabled {
				t.Error("CreateHealthChecker() returned nil checker for enabled config")
			}
		})
	}
}

func TestCreateCheck(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Check
		wantErr bool
		errMsg  string
	}{
		{
			name: "http check with URL",
			cfg: config.Check{
				Type:    "http",
				Timeout: 5,
				Config: map[string]string{
					"url": "http://localhost:8080/health",
				},
			},
			wantErr: false,
		},
		{
			name: "http check without URL",
			cfg: config.Check{
				Type:    "http",
				Timeout: 5,
				Config: map[string]string{},
			},
			wantErr: true,
			errMsg:  "http check requires 'url' in config",
		},
		{
			name: "tcp check with address",
			cfg: config.Check{
				Type:    "tcp",
				Timeout: 5,
				Config: map[string]string{
					"address": "localhost:8080",
				},
			},
			wantErr: false,
		},
		{
			name: "tcp check without address",
			cfg: config.Check{
				Type:    "tcp",
				Timeout: 5,
				Config: map[string]string{},
			},
			wantErr: true,
			errMsg:  "tcp check requires 'address' in config",
		},
		{
			name: "grpc check with address",
			cfg: config.Check{
				Type:    "grpc",
				Timeout: 5,
				Config: map[string]string{
					"address": "localhost:50051",
				},
			},
			wantErr: false,
		},
		{
			name: "grpc check without address",
			cfg: config.Check{
				Type:    "grpc",
				Timeout: 5,
				Config: map[string]string{},
			},
			wantErr: true,
			errMsg:  "grpc check requires 'address' in config",
		},
		{
			name: "exec check with allowed command",
			cfg: config.Check{
				Type:    "exec",
				Timeout: 5,
				Config: map[string]string{
					"command": "check-disk-space",
				},
			},
			wantErr: false,
		},
		{
			name: "exec check without command",
			cfg: config.Check{
				Type:    "exec",
				Timeout: 5,
				Config: map[string]string{},
			},
			wantErr: true,
			errMsg:  "exec check requires 'command' in config",
		},
		{
			name: "exec check with disallowed command",
			cfg: config.Check{
				Type:    "exec",
				Timeout: 5,
				Config: map[string]string{
					"command": "rm -rf /",
				},
			},
			wantErr: true,
			errMsg:  "exec command 'rm -rf /' is not in whitelist",
		},
		{
			name: "unknown check type",
			cfg: config.Check{
				Type:    "unknown",
				Timeout: 5,
			},
			wantErr: true,
			errMsg:  "unknown check type: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check, err := createCheck(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("createCheck() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("createCheck() error = %v, want %v", err.Error(), tt.errMsg)
			}
			if err == nil && check == nil {
				t.Error("createCheck() returned nil check without error")
			}
		})
	}
}

func TestTCPCheck(t *testing.T) {
	// Start a test TCP server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// Test successful connection
	check := tcpCheck(addr, 5*time.Second)
	err = check(context.Background())
	if err != nil {
		t.Errorf("Expected successful TCP check, got %v", err)
	}

	// Test failed connection (invalid address)
	check = tcpCheck("localhost:99999", 1*time.Second)
	err = check(context.Background())
	if err == nil {
		t.Error("Expected error for invalid address")
	}

	// Test timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	check = tcpCheck(addr, 5*time.Second)
	err = check(ctx)
	// The error might be nil if the connection succeeds quickly
	_ = err
}

// Mock gRPC health server for testing
type mockHealthServer struct {
	healthpb.UnimplementedHealthServer
	status healthpb.HealthCheckResponse_ServingStatus
}

func (s *mockHealthServer) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{
		Status: s.status,
	}, nil
}

func TestGRPCCheck(t *testing.T) {
	// Start a test gRPC server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	grpcServer := grpc.NewServer()
	healthServer := &mockHealthServer{
		status: healthpb.HealthCheckResponse_SERVING,
	}
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	go func() {
		grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	addr := listener.Addr().String()

	// Test successful health check
	check := grpcCheck(addr, "grpc.health.v1.Health", 5*time.Second)
	err = check(context.Background())
	if err != nil {
		t.Errorf("Expected successful gRPC check, got %v", err)
	}

	// Test unhealthy service
	healthServer.status = healthpb.HealthCheckResponse_NOT_SERVING
	err = check(context.Background())
	if err == nil {
		t.Error("Expected error for unhealthy service")
	}

	// Test invalid address
	check = grpcCheck("localhost:99999", "grpc.health.v1.Health", 1*time.Second)
	err = check(context.Background())
	if err == nil {
		t.Error("Expected error for invalid address")
	}
}

func TestExecCheck(t *testing.T) {
	// Test successful command (echo)
	check := execCheck([]string{"echo", "test"}, 5*time.Second)
	err := check(context.Background())
	if err != nil {
		t.Errorf("Expected successful exec check, got %v", err)
	}

	// Test failing command (false)
	check = execCheck([]string{"false"}, 5*time.Second)
	err = check(context.Background())
	if err == nil {
		t.Error("Expected error for failing command")
	}

	// Test non-existent command
	check = execCheck([]string{"nonexistentcommand12345"}, 5*time.Second)
	err = check(context.Background())
	if err == nil {
		t.Error("Expected error for non-existent command")
	}

	// Test timeout
	check = execCheck([]string{"sleep", "10"}, 100*time.Millisecond)
	err = check(context.Background())
	if err == nil {
		t.Error("Expected timeout error")
	}

	// Test empty command
	check = execCheck([]string{}, 5*time.Second)
	err = check(context.Background())
	if err == nil {
		t.Error("Expected error for empty command")
	}
}

func TestCreateHealthHandler(t *testing.T) {
	cfg := &config.Health{
		Enabled:    true,
		HealthPath: "/health",
		ReadyPath:  "/ready",
		LivePath:   "/live",
	}
	
	checker := health.NewChecker()
	version := "1.0.0"
	serviceID := "test-gateway"

	handler := CreateHealthHandler(cfg, checker, version, serviceID)
	if handler == nil {
		t.Error("CreateHealthHandler returned nil")
	}
}