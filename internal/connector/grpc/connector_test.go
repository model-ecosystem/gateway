package grpc

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"gateway/internal/core"
	"gateway/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
)

func TestNew(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "with empty config",
			config: &Config{},
		},
		{
			name: "with custom config",
			config: &Config{
				MaxConcurrentStreams: 100,
				KeepAliveTime:        60 * time.Second,
				KeepAliveTimeout:     20 * time.Second,
				TLS:                  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := New(tt.config, logger)

			if connector == nil {
				t.Fatal("Expected connector, got nil")
			}
			if connector.logger != logger {
				t.Error("Logger not set correctly")
			}
			if connector.clients == nil {
				t.Error("Clients map not initialized")
			}

			// Check defaults
			if tt.config.KeepAliveTime == 0 && connector.config.KeepAliveTime != 30*time.Second {
				t.Errorf("Expected default KeepAliveTime 30s, got %v", connector.config.KeepAliveTime)
			}
			if tt.config.KeepAliveTimeout == 0 && connector.config.KeepAliveTimeout != 10*time.Second {
				t.Errorf("Expected default KeepAliveTimeout 10s, got %v", connector.config.KeepAliveTimeout)
			}
		})
	}
}

func TestConnector_Type(t *testing.T) {
	connector := &Connector{}
	if connector.Type() != "grpc" {
		t.Errorf("Expected type 'grpc', got '%s'", connector.Type())
	}
}

func TestConnector_Forward_NoRoute(t *testing.T) {
	logger := slog.Default()
	connector := New(&Config{}, logger)

	req := &mockRequest{
		path: "/package.Service/Method",
	}

	// Test with nil route
	_, err := connector.Forward(context.Background(), req, nil)
	if err == nil {
		t.Error("Expected error for nil route")
	}

	// Test with nil instance
	_, err = connector.Forward(context.Background(), req, &core.RouteResult{})
	if err == nil {
		t.Error("Expected error for nil instance")
	}
}

func TestConnector_Close(t *testing.T) {
	logger := slog.Default()
	connector := New(&Config{}, logger)

	// Add mock client connections
	// In real test, we would add actual gRPC connections

	err := connector.Close()
	if err != nil {
		t.Errorf("Unexpected error on close: %v", err)
	}

	// Verify clients map is empty
	connector.clientsMu.RLock()
	if len(connector.clients) != 0 {
		t.Error("Expected clients map to be empty after close")
	}
	connector.clientsMu.RUnlock()
}

func TestHandleGRPCError(t *testing.T) {
	logger := slog.Default()
	connector := New(&Config{}, logger)
	tests := []struct {
		name         string
		err          error
		expectedType errors.ErrorType
		expectedMsg  string
	}{
		{
			name:         "deadline exceeded",
			err:          status.Error(codes.DeadlineExceeded, "deadline exceeded"),
			expectedType: errors.ErrorTypeTimeout,
			expectedMsg:  "deadline exceeded",
		},
		{
			name:         "not found",
			err:          status.Error(codes.NotFound, "method not found"),
			expectedType: errors.ErrorTypeNotFound,
			expectedMsg:  "method not found",
		},
		{
			name:         "unavailable",
			err:          status.Error(codes.Unavailable, "service unavailable"),
			expectedType: errors.ErrorTypeUnavailable,
			expectedMsg:  "service unavailable",
		},
		{
			name:         "invalid argument",
			err:          status.Error(codes.InvalidArgument, "invalid input"),
			expectedType: errors.ErrorTypeBadRequest,
			expectedMsg:  "invalid input",
		},
		{
			name:         "internal error",
			err:          status.Error(codes.Internal, "internal error"),
			expectedType: errors.ErrorTypeInternal,
			expectedMsg:  "internal error",
		},
		{
			name:         "unhandled code",
			err:          status.Error(codes.PermissionDenied, "access denied"),
			expectedType: errors.ErrorTypeInternal,
			expectedMsg:  "access denied",
		},
		{
			name:         "non-grpc error",
			err:          fmt.Errorf("some error"),
			expectedType: errors.ErrorTypeInternal,
			expectedMsg:  "unknown gRPC error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := connector.handleGRPCError(tt.err)

			if result == nil {
				t.Error("Expected non-nil error")
				return
			}

			gatewayErr, ok := result.(*errors.Error)
			if !ok {
				t.Error("Expected *errors.Error type")
				return
			}

			if gatewayErr.Type != tt.expectedType {
				t.Errorf("Expected error type %v, got %v", tt.expectedType, gatewayErr.Type)
			}

			if gatewayErr.Message != tt.expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, gatewayErr.Message)
			}
		})
	}
}

// Mock request for testing
type mockRequest struct {
	id         string
	method     string
	path       string
	url        string
	remoteAddr string
	headers    map[string][]string
	body       io.ReadCloser
	ctx        context.Context
}

func (m *mockRequest) ID() string                   { return m.id }
func (m *mockRequest) Method() string               { return m.method }
func (m *mockRequest) Path() string                 { return m.path }
func (m *mockRequest) URL() string                  { return m.url }
func (m *mockRequest) RemoteAddr() string           { return m.remoteAddr }
func (m *mockRequest) Headers() map[string][]string { return m.headers }
func (m *mockRequest) Body() io.ReadCloser          { return m.body }
func (m *mockRequest) Context() context.Context     { return m.ctx }
