package websocket

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"gateway/pkg/request"
	"log/slog"
	"sync"
)

// Mock handler for testing
type mockHandler struct {
	called bool
	err    error
	resp   core.Response
}

func (m *mockHandler) Handle(ctx context.Context, req core.Request) (core.Response, error) {
	m.called = true
	return m.resp, m.err
}

// Mock response for testing
type mockResponse struct {
	statusCode int
	headers    map[string][]string
	body       []byte
}

func (m *mockResponse) StatusCode() int                { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string  { return m.headers }
func (m *mockResponse) Body() io.ReadCloser           { return io.NopCloser(bytes.NewReader(m.body)) }

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.Host != "0.0.0.0" {
		t.Errorf("Expected host 0.0.0.0, got %s", config.Host)
	}
	if config.Port != 8081 {
		t.Errorf("Expected port 8081, got %d", config.Port)
	}
	if config.MaxMessageSize != 1024*1024 {
		t.Errorf("Expected max message size 1MB, got %d", config.MaxMessageSize)
	}
}

func TestNewAdapter(t *testing.T) {
	logger := slog.Default()
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return nil, nil
	}
	
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "with nil config",
			config: nil,
		},
		{
			name:   "with custom config",
			config: &Config{
				Host: "localhost",
				Port: 9999,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAdapter(tt.config, handler, logger)
			
			if adapter == nil {
				t.Fatal("Expected adapter, got nil")
			}
			if adapter.logger != logger {
				t.Error("Logger not set correctly")
			}
			if adapter.handler == nil {
				t.Error("Handler not set correctly")
			}
			if adapter.upgrader == nil {
				t.Error("Upgrader not created")
			}
		})
	}
}

func TestAdapter_StartStop(t *testing.T) {
	logger := slog.Default()
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
	}
	
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name: "basic start/stop",
			config: &Config{
				Host:        "127.0.0.1",
				Port:        0, // Use random port
				ReadTimeout: 10,
			},
		},
		{
			name: "with TLS",
			config: &Config{
				Host: "127.0.0.1",
				Port: 0,
				TLS: &TLSConfig{
					Enabled: true,
				},
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					Certificates: []tls.Certificate{
						{}, // Empty cert will still allow listener creation for testing
					},
				},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAdapter(tt.config, handler, logger)
			
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			
			// Start adapter
			err := adapter.Start(ctx)
			if (err != nil) != tt.wantError {
				t.Fatalf("Start() error = %v, wantError %v", err, tt.wantError)
			}
			
			if err == nil {
				// Get actual port
				if adapter.listener != nil {
					addr := adapter.listener.Addr()
					t.Logf("Adapter listening on %s", addr.String())
				}
				
				// Test double start
				err = adapter.Start(ctx)
				if err == nil {
					t.Error("Expected error on double start")
				}
				
				// Stop adapter
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer stopCancel()
				
				err = adapter.Stop(stopCtx)
				if err != nil {
					t.Errorf("Stop() error = %v", err)
				}
				
				// Test double stop
				err = adapter.Stop(stopCtx)
				if err != nil {
					t.Error("Expected no error on double stop")
				}
			}
		})
	}
}

func TestAdapter_Type(t *testing.T) {
	adapter := &Adapter{}
	if adapter.Type() != "websocket" {
		t.Errorf("Expected type 'websocket', got '%s'", adapter.Type())
	}
}

func TestAdapter_handleWebSocket(t *testing.T) {
	logger := slog.Default()
	
	tests := []struct {
		name          string
		handler       core.Handler
		requestHeader http.Header
		upgradeError  bool
		wantUpgrade   bool
	}{
		{
			name: "successful upgrade",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				// Verify request properties
				if req.ID() == "" {
					t.Error("Expected request ID")
				}
				if req.Method() != "WEBSOCKET" {
					t.Errorf("Expected method WEBSOCKET, got %s", req.Method())
				}
				// Protocol is set in BaseRequest
				base := req.(*wsRequest).BaseRequest
				if base.Protocol() != "websocket" {
					t.Errorf("Expected protocol websocket, got %s", base.Protocol())
				}
				return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
			},
			requestHeader: http.Header{
				"Connection": []string{"Upgrade"},
				"Sec-WebSocket-Key": []string{"dGhlIHNhbXBsZSBub25jZQ=="},
				"Sec-WebSocket-Version": []string{"13"},
			},
			wantUpgrade: true,
		},
		{
			name: "handler error",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				return nil, errors.NewError(errors.ErrorTypeInternal, "handler error")
			},
			requestHeader: http.Header{
				"Connection": []string{"Upgrade"},
				"Sec-WebSocket-Key": []string{"dGhlIHNhbXBsZSBub25jZQ=="},
				"Sec-WebSocket-Version": []string{"13"},
			},
			wantUpgrade: true,
		},
		{
			name: "with generated request ID",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				// Verify request ID was generated
				if req.ID() == "" {
					t.Error("Expected generated request ID")
				}
				return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
			},
			requestHeader: http.Header{
				"Connection":    []string{"Upgrade"},
				"Sec-WebSocket-Key": []string{"dGhlIHNhbXBsZSBub25jZQ=="},
				"Sec-WebSocket-Version": []string{"13"},
			},
			wantUpgrade: true,
		},
		{
			name: "invalid upgrade",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				return nil, nil
			},
			requestHeader: http.Header{}, // Missing upgrade headers
			upgradeError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAdapter(DefaultConfig(), tt.handler, logger)
			
			// Start test server
			server := &http.Server{
				Handler: http.HandlerFunc(adapter.handleWebSocket),
			}
			
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			
			go server.Serve(listener)
			defer server.Close()
			
			// Create WebSocket client or regular HTTP client
			addr := listener.Addr().String()
			
			if tt.upgradeError {
				// Make a regular HTTP request instead of WebSocket
				resp, err := http.Get(fmt.Sprintf("http://%s/test", addr))
				if err != nil {
					t.Fatalf("HTTP request failed: %v", err)
				}
				defer resp.Body.Close()
				
				// Should get a non-101 status since it's not a proper WebSocket upgrade
				if resp.StatusCode == http.StatusSwitchingProtocols {
					t.Error("Expected non-101 status for invalid upgrade")
				}
			} else {
				// Make WebSocket connection
				url := fmt.Sprintf("ws://%s/test", addr)
				dialer := websocket.DefaultDialer
				dialer.HandshakeTimeout = 2 * time.Second
				
				conn, resp, err := dialer.Dial(url, nil)
				if tt.wantUpgrade {
					if err != nil {
						t.Fatalf("Failed to connect: %v", err)
					}
					defer conn.Close()
					
					if resp.StatusCode != http.StatusSwitchingProtocols {
						t.Errorf("Expected status 101, got %d", resp.StatusCode)
					}
				}
			}
		})
	}
}

func TestMakeCheckOrigin(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		origin         string
		host           string
		expectedResult bool
	}{
		{
			name: "origin check disabled",
			config: &Config{
				CheckOrigin: false,
			},
			origin:         "http://evil.com",
			expectedResult: true,
		},
		{
			name: "no origin header",
			config: &Config{
				CheckOrigin: true,
			},
			origin:         "",
			expectedResult: true,
		},
		{
			name: "same origin http",
			config: &Config{
				CheckOrigin: true,
			},
			origin:         "http://localhost:8080",
			host:           "localhost:8080",
			expectedResult: true,
		},
		{
			name: "same origin https",
			config: &Config{
				CheckOrigin: true,
			},
			origin:         "https://localhost:8080",
			host:           "localhost:8080",
			expectedResult: true,
		},
		{
			name: "allowed origin",
			config: &Config{
				CheckOrigin:    true,
				AllowedOrigins: []string{"http://trusted.com", "https://app.com"},
			},
			origin:         "http://trusted.com",
			expectedResult: true,
		},
		{
			name: "wildcard origin",
			config: &Config{
				CheckOrigin:    true,
				AllowedOrigins: []string{"*"},
			},
			origin:         "http://any.com",
			expectedResult: true,
		},
		{
			name: "disallowed origin",
			config: &Config{
				CheckOrigin:    true,
				AllowedOrigins: []string{"http://trusted.com"},
			},
			origin:         "http://evil.com",
			expectedResult: false,
		},
		{
			name: "no allowed origins",
			config: &Config{
				CheckOrigin:    true,
				AllowedOrigins: []string{},
			},
			origin:         "http://any.com",
			expectedResult: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkOrigin := makeCheckOrigin(tt.config)
			
			req := &http.Request{
				Header: make(http.Header),
				Host:   tt.host,
			}
			
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			
			result := checkOrigin(req)
			if result != tt.expectedResult {
				t.Errorf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestAdapter_Concurrent(t *testing.T) {
	logger := slog.Default()
	
	// Counter for concurrent connections
	var connCount int
	var mu sync.Mutex
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		mu.Lock()
		connCount++
		mu.Unlock()
		
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		
		return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
	}
	
	adapter := NewAdapter(&Config{
		Host:        "127.0.0.1",
		Port:        0,
		ReadTimeout: 10,
	}, handler, logger)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := adapter.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	
	addr := adapter.listener.Addr().String()
	
	// Launch multiple concurrent connections
	const numConnections = 10
	errChan := make(chan error, numConnections)
	
	for i := 0; i < numConnections; i++ {
		go func(i int) {
			url := fmt.Sprintf("ws://%s/test%d", addr, i)
			conn, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				errChan <- err
				return
			}
			conn.Close()
			errChan <- nil
		}(i)
	}
	
	// Wait for all connections
	for i := 0; i < numConnections; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Connection error: %v", err)
		}
	}
	
	// Verify all connections were handled
	mu.Lock()
	if connCount != numConnections {
		t.Errorf("Expected %d connections, got %d", numConnections, connCount)
	}
	mu.Unlock()
}

func TestWsRequest(t *testing.T) {
	// Create a mock HTTP request
	httpReq, _ := http.NewRequest("GET", "/test", nil)
	httpReq.RemoteAddr = "192.168.1.1:12345"
	
	// Create base request
	baseReq := request.NewBase("test-id", httpReq, "WEBSOCKET", "websocket")
	
	// Create mock connection
	mockConn := newConn(nil, "192.168.1.1:12345")
	
	// Create WebSocket request
	wsReq := &wsRequest{
		BaseRequest: baseReq,
		conn:        mockConn,
	}
	
	// Verify properties
	if wsReq.ID() != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", wsReq.ID())
	}
	if wsReq.Method() != "WEBSOCKET" {
		t.Errorf("Expected method 'WEBSOCKET', got %s", wsReq.Method())
	}
	if wsReq.Protocol() != "websocket" {
		t.Errorf("Expected protocol 'websocket', got %s", wsReq.Protocol())
	}
}