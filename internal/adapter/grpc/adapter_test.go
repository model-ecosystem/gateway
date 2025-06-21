package grpc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	grpcConnector "gateway/internal/connector/grpc"
	"gateway/internal/core"
	"log/slog"
)

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

func (m *mockRequest) ID() string                     { return m.id }
func (m *mockRequest) Method() string                 { return m.method }
func (m *mockRequest) Path() string                   { return m.path }
func (m *mockRequest) URL() string                    { return m.url }
func (m *mockRequest) RemoteAddr() string             { return m.remoteAddr }
func (m *mockRequest) Headers() map[string][]string   { return m.headers }
func (m *mockRequest) Body() io.ReadCloser            { return m.body }
func (m *mockRequest) Context() context.Context       { return m.ctx }

// Mock response for testing
type mockResponse struct {
	statusCode int
	headers    map[string][]string
	body       []byte
}

func (m *mockResponse) StatusCode() int                { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string   { return m.headers }
func (m *mockResponse) Body() io.ReadCloser           { return io.NopCloser(bytes.NewReader(m.body)) }

func TestNewAdapter(t *testing.T) {
	logger := slog.Default()
	adapter := NewAdapter(logger)
	
	if adapter == nil {
		t.Fatal("Expected adapter, got nil")
	}
	if adapter.logger != logger {
		t.Error("Logger not set correctly")
	}
	if adapter.transcoder == nil {
		t.Error("Transcoder not created")
	}
}

func TestAdapter_TranscodeRequest(t *testing.T) {
	logger := slog.Default()
	adapter := NewAdapter(logger)
	
	tests := []struct {
		name        string
		request     core.Request
		wantError   bool
		wantGRPC    bool
		checkResult func(*testing.T, core.Request)
	}{
		{
			name: "gRPC-Web request passthrough",
			request: &mockRequest{
				path: "/package.Service/Method",
				headers: map[string][]string{
					"Content-Type": {"application/grpc-web"},
				},
			},
			wantGRPC: false, // Should pass through as-is
		},
		{
			name: "gRPC-Web with proto",
			request: &mockRequest{
				path: "/package.Service/Method",
				headers: map[string][]string{
					"Content-Type": {"application/grpc-web+proto"},
				},
			},
			wantGRPC: false,
		},
		{
			name: "gRPC-Web with json",
			request: &mockRequest{
				path: "/package.Service/Method",
				headers: map[string][]string{
					"Content-Type": {"application/grpc-web+json"},
				},
			},
			wantGRPC: false,
		},
		{
			name: "non-gRPC path",
			request: &mockRequest{
				path: "/api/v1/users",
				headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
			wantGRPC: false,
		},
		{
			name: "gRPC path for transcoding",
			request: &mockRequest{
				path:   "/package.Service/Method",
				method: "POST",
				headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				body: io.NopCloser(bytes.NewReader([]byte(`{"key": "value"}`))),
			},
			wantGRPC: true,
			checkResult: func(t *testing.T, req core.Request) {
				// Should be wrapped in transcodedRequest
				if _, ok := req.(*transcodedRequest); !ok {
					t.Error("Expected transcodedRequest type")
				}
			},
		},
		{
			name: "invalid gRPC path format",
			request: &mockRequest{
				path: "/invalid",
				headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
			wantGRPC: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.TranscodeRequest(tt.request)
			
			if tt.wantError && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if err == nil {
				if tt.wantGRPC && result == tt.request {
					t.Error("Expected request to be transcoded")
				} else if !tt.wantGRPC && result != tt.request {
					t.Error("Expected request to pass through unchanged")
				}
				
				if tt.checkResult != nil {
					tt.checkResult(t, result)
				}
			}
		})
	}
}

func TestAdapter_TranscodeResponse(t *testing.T) {
	logger := slog.Default()
	adapter := NewAdapter(logger)
	
	tests := []struct {
		name      string
		response  core.Response
		wantError bool
	}{
		{
			name: "non-200 response passthrough",
			response: &mockResponse{
				statusCode: 404,
				headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				body: []byte(`{"error": "not found"}`),
			},
			wantError: false,
		},
		{
			name: "gRPC response for transcoding",
			response: &mockResponse{
				statusCode: 200,
				headers: map[string][]string{
					"Content-Type": {"application/grpc"},
				},
				body: []byte("gRPC binary data"),
			},
			wantError: false,
		},
		{
			name: "empty gRPC response",
			response: &mockResponse{
				statusCode: 200,
				headers:    map[string][]string{},
				body:       []byte{},
			},
			wantError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.TranscodeResponse(tt.response)
			
			if tt.wantError && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if err == nil && result == nil {
				t.Error("Expected non-nil response")
			}
		})
	}
}

func TestIsGRPCPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/package.Service/Method", true},
		{"/com.example.Service/GetUser", true},
		{"/grpc.health.v1.Health/Check", true},
		{"/Package.Service/Method", true},
		{"/api/v1/users", false},
		{"/health", false},
		{"/", false},
		{"", false},
		{"package.Service/Method", false}, // Missing leading slash
		{"/packageService/Method", false},  // Missing dot
		{"/package/Service/Method", false}, // Too many slashes
	}
	
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isGRPCPath(tt.path)
			if result != tt.expected {
				t.Errorf("isGRPCPath(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestHandleGRPCWebRequest(t *testing.T) {
	logger := slog.Default()
	adapter := NewAdapter(logger)
	
	tests := []struct {
		name    string
		method  string
		path    string
		headers map[string][]string
		wantStatus int
	}{
		{
			name:   "preflight request",
			method: "OPTIONS",
			path:   "/package.Service/Method",
			headers: map[string][]string{
				"Origin": {"http://localhost:3000"},
				"Access-Control-Request-Method": {"POST"},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "gRPC-Web POST request",
			method: "POST",
			path:   "/package.Service/Method",
			headers: map[string][]string{
				"Content-Type": {"application/grpc-web"},
			},
			wantStatus: http.StatusOK,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test preflight handling
			if tt.method == "OPTIONS" {
				headers := adapter.HandleGRPCWebPreflight()
				
				// Check CORS headers
				if headers["Access-Control-Allow-Origin"][0] != "*" {
					t.Error("Expected wildcard origin")
				}
				if len(headers["Access-Control-Allow-Methods"]) == 0 {
					t.Error("Expected allowed methods")
				}
				if len(headers["Access-Control-Allow-Headers"]) == 0 {
					t.Error("Expected allowed headers")
				}
			}
		})
	}
}

// Test transcoded request wrapper
func TestTranscodedRequest(t *testing.T) {
	original := &mockRequest{
		id:         "test-123",
		method:     "POST",
		path:       "/api/users",
		url:        "http://example.com/api/users",
		remoteAddr: "192.168.1.1",
		headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		body: io.NopCloser(bytes.NewReader([]byte(`{"name": "test"}`))),
		ctx:  context.Background(),
	}
	
	grpcReq := &grpcConnector.GRPCRequest{
		Service: "package.Service",
		Method:  "CreateUser",
		Headers: map[string][]string{
			"Content-Type": {"application/grpc"},
		},
		Body: []byte("grpc data"),
	}
	
	transcoded := &transcodedRequest{
		original: original,
		grpcReq:  grpcReq,
	}
	
	// Test that transcoded request preserves original metadata
	if transcoded.ID() != original.ID() {
		t.Errorf("Expected ID %s, got %s", original.ID(), transcoded.ID())
	}
	if transcoded.RemoteAddr() != original.RemoteAddr() {
		t.Errorf("Expected RemoteAddr %s, got %s", original.RemoteAddr(), transcoded.RemoteAddr())
	}
	if transcoded.Context() != original.Context() {
		t.Error("Expected same context")
	}
	
	// Test that transcoded request uses gRPC data
	if transcoded.Method() != "POST" {
		t.Errorf("Expected method POST, got %s", transcoded.Method())
	}
	expectedPath := "/package.Service/CreateUser"
	if transcoded.Path() != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, transcoded.Path())
	}
}