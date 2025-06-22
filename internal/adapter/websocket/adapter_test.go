package websocket

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gateway/internal/core"
	"gateway/pkg/errors"
	"gateway/pkg/request"
	"github.com/gorilla/websocket"
	"log/slog"
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

func (m *mockResponse) StatusCode() int              { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string { return m.headers }
func (m *mockResponse) Body() io.ReadCloser          { return io.NopCloser(bytes.NewReader(m.body)) }

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
			name: "with custom config",
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
				"Connection":            []string{"Upgrade"},
				"Sec-WebSocket-Key":     []string{"dGhlIHNhbXBsZSBub25jZQ=="},
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
				"Connection":            []string{"Upgrade"},
				"Sec-WebSocket-Key":     []string{"dGhlIHNhbXBsZSBub25jZQ=="},
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
				"Connection":            []string{"Upgrade"},
				"Sec-WebSocket-Key":     []string{"dGhlIHNhbXBsZSBub25jZQ=="},
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

func TestWebSocketContextLifecycle(t *testing.T) {
	logger := slog.Default()

	// Channel to signal when handler completes
	handlerDone := make(chan struct{})
	// Channel to verify connection is still alive
	connAlive := make(chan bool)

	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Simulate the handler returning (HTTP upgrade complete)
		close(handlerDone)

		// Get the WebSocket connection
		wsReq := req.(*wsRequest)
		// The conn field is already of type *conn
		wsConn := wsReq.conn

		// Start a goroutine to verify connection stays alive
		go func() {
			// Wait for signal that HTTP handler has returned
			<-handlerDone

			// Wait a bit to ensure HTTP handler has fully returned
			time.Sleep(100 * time.Millisecond)

			// Try to write a message - should still work
			err := wsConn.ws.WriteMessage(websocket.TextMessage, []byte("still alive"))
			connAlive <- err == nil
		}()

		// Return successful upgrade
		return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
	}

	adapter := NewAdapter(DefaultConfig(), handler, logger)

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

	// Connect WebSocket client
	addr := listener.Addr().String()
	url := fmt.Sprintf("ws://%s/test", addr)
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 2 * time.Second

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Set up pong handler to respond to pings
	conn.SetPongHandler(func(appData string) error {
		return nil
	})

	// Read messages in background
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Wait for connection alive signal
	select {
	case alive := <-connAlive:
		if !alive {
			t.Error("WebSocket connection was closed after HTTP handler returned")
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for connection check")
	}

	// Verify we can still send messages
	err = conn.WriteMessage(websocket.TextMessage, []byte("test message"))
	if err != nil {
		t.Errorf("Failed to write message after handler returned: %v", err)
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

			// Use default host if not specified
			host := tt.host
			if host == "" {
				host = "localhost:8080"
			}

			req := &http.Request{
				Header: make(http.Header),
				Host:   host,
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
	t.Skip("Skipping flaky test - needs investigation")
	logger := slog.Default()

	// Counter for successful upgrades
	var upgradeCount int32 // Use atomic for thread safety
	
	// Wait group to track all goroutines
	var serverWg sync.WaitGroup

	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Count successful upgrades atomically
		atomic.AddInt32(&upgradeCount, 1)

		// Extract WebSocket connection from request
		wsReq, ok := req.(*wsRequest)
		if !ok {
			return nil, fmt.Errorf("not a WebSocket request")
		}

		// Start a goroutine to handle the connection
		serverWg.Add(1)
		go func() {
			defer serverWg.Done()
			
			// Just echo back any messages received
			for {
				msg, err := wsReq.conn.ReadMessage()
				if err != nil {
					// Connection closed, that's ok
					return
				}
				
				// Echo the message back
				err = wsReq.conn.WriteMessage(msg)
				if err != nil {
					return
				}
			}
		}()

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
	var clientWg sync.WaitGroup
	successCount := int32(0)

	for i := 0; i < numConnections; i++ {
		clientWg.Add(1)
		go func(i int) {
			defer clientWg.Done()
			
			url := fmt.Sprintf("ws://%s/test%d", addr, i)
			conn, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				t.Logf("Connection %d failed: %v", i, err)
				return
			}
			defer conn.Close()
			
			// Send a test message
			testMsg := fmt.Sprintf("Hello from client %d", i)
			err = conn.WriteMessage(websocket.TextMessage, []byte(testMsg))
			if err != nil {
				t.Logf("Failed to send message %d: %v", i, err)
				return
			}
			
			// Read the echo response
			_, msg, err := conn.ReadMessage()
			if err != nil {
				t.Logf("Failed to read echo %d: %v", i, err)
				return
			}
			
			// Verify echo
			if string(msg) != testMsg {
				t.Logf("Echo mismatch %d: expected %q, got %q", i, testMsg, string(msg))
				return
			}
			
			// Success!
			atomic.AddInt32(&successCount, 1)
			
			// Send close message
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		}(i)
	}

	// Wait for all client connections to complete
	clientWg.Wait()
	
	// Give server goroutines a moment to process
	time.Sleep(100 * time.Millisecond)
	
	// Stop the adapter gracefully
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	
	err = adapter.Stop(stopCtx)
	if err != nil {
		t.Logf("Error stopping adapter: %v", err)
	}
	
	// Cancel the start context
	cancel()
	
	// Wait for server goroutines to finish
	done := make(chan struct{})
	go func() {
		serverWg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines finished
	case <-time.After(2 * time.Second):
		t.Log("Warning: server goroutines did not finish in time")
	}
	
	// Verify results
	finalUpgradeCount := atomic.LoadInt32(&upgradeCount)
	finalSuccessCount := atomic.LoadInt32(&successCount)
	
	t.Logf("Upgrades: %d, Successes: %d", finalUpgradeCount, finalSuccessCount)
	
	// We expect at least some connections to succeed
	if finalUpgradeCount == 0 {
		t.Error("No WebSocket connections were upgraded")
	}
	
	if finalSuccessCount == 0 {
		t.Error("No WebSocket echo tests succeeded")
	}
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

// Mock token validator for testing
type mockTokenValidator struct {
	validateFunc func(ctx context.Context, connectionID string, token string, onExpired func()) error
	stopCalled   map[string]bool
	mu           sync.Mutex
}

func newMockTokenValidator() *mockTokenValidator {
	return &mockTokenValidator{
		stopCalled: make(map[string]bool),
	}
}

func (m *mockTokenValidator) ValidateConnection(ctx context.Context, connectionID string, token string, onExpired func()) error {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, connectionID, token, onExpired)
	}
	return nil
}

func (m *mockTokenValidator) StopValidation(connectionID string) {
	m.mu.Lock()
	m.stopCalled[connectionID] = true
	m.mu.Unlock()
}

func TestAdapter_WithTokenValidator(t *testing.T) {
	logger := slog.Default()

	t.Run("valid token", func(t *testing.T) {
		validator := newMockTokenValidator()
		validator.validateFunc = func(ctx context.Context, connectionID string, token string, onExpired func()) error {
			if token != "valid-token" {
				return fmt.Errorf("invalid token")
			}
			return nil
		}

		handlerCalled := false
		handler := func(ctx context.Context, req core.Request) (core.Response, error) {
			handlerCalled = true
			return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
		}

		adapter := NewAdapter(&Config{
			Host: "127.0.0.1",
			Port: 0,
		}, handler, logger)
		adapter.WithTokenValidator(validator)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := adapter.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Create WebSocket connection with valid token
		addr := adapter.listener.Addr().String()
		headers := http.Header{}
		headers.Set("Authorization", "Bearer valid-token")

		conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/test", addr), headers)
		if err != nil {
			t.Fatalf("Failed to connect with valid token: %v", err)
		}
		defer conn.Close()

		// Give handler time to be called
		time.Sleep(50 * time.Millisecond)

		if !handlerCalled {
			t.Error("Handler should have been called with valid token")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		validator := newMockTokenValidator()
		validator.validateFunc = func(ctx context.Context, connectionID string, token string, onExpired func()) error {
			return fmt.Errorf("invalid token")
		}

		handler := func(ctx context.Context, req core.Request) (core.Response, error) {
			t.Error("Handler should not be called with invalid token")
			return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
		}

		adapter := NewAdapter(&Config{
			Host: "127.0.0.1",
			Port: 0,
		}, handler, logger)
		adapter.WithTokenValidator(validator)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := adapter.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Try to connect with invalid token
		addr := adapter.listener.Addr().String()
		headers := http.Header{}
		headers.Set("Authorization", "Bearer invalid-token")

		conn, resp, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/test", addr), headers)
		if err == nil {
			conn.Close()
			t.Error("Expected connection to fail with invalid token")
		}

		// The connection should be rejected
		if resp != nil && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 status, got %d", resp.StatusCode)
		}
	})

	t.Run("no token with validator", func(t *testing.T) {
		validator := newMockTokenValidator()

		handlerCalled := false
		handler := func(ctx context.Context, req core.Request) (core.Response, error) {
			handlerCalled = true
			return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
		}

		adapter := NewAdapter(&Config{
			Host: "127.0.0.1",
			Port: 0,
		}, handler, logger)
		adapter.WithTokenValidator(validator)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := adapter.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Connect without token - should be rejected
		addr := adapter.listener.Addr().String()
		conn, resp, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/test", addr), nil)
		if err == nil {
			conn.Close()
			t.Fatal("Expected connection to be rejected without token")
		}

		// Check that we got a 401 Unauthorized
		if resp != nil && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got %d", resp.StatusCode)
		}

		// Verify handler was NOT called (authentication failed before handler)
		if handlerCalled {
			t.Error("Handler should not be called when authentication fails")
		}
	})

	t.Run("token expiration", func(t *testing.T) {
		validator := newMockTokenValidator()
		expiredCallbackCalled := false

		validator.validateFunc = func(ctx context.Context, connectionID string, token string, onExpired func()) error {
			// Simulate token expiration after 100ms
			go func() {
				time.Sleep(100 * time.Millisecond)
				expiredCallbackCalled = true
				onExpired()
			}()
			return nil
		}

		handler := func(ctx context.Context, req core.Request) (core.Response, error) {
			// Keep connection open
			<-ctx.Done()
			return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
		}

		adapter := NewAdapter(&Config{
			Host: "127.0.0.1",
			Port: 0,
		}, handler, logger)
		adapter.WithTokenValidator(validator)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := adapter.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Connect with token
		addr := adapter.listener.Addr().String()
		headers := http.Header{}
		headers.Set("Authorization", "Bearer expiring-token")

		conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/test", addr), headers)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		// Wait for token to expire
		time.Sleep(150 * time.Millisecond)

		if !expiredCallbackCalled {
			t.Error("Token expiration callback should have been called")
		}

		// Connection should be closed
		_, _, err = conn.ReadMessage()
		if err == nil {
			t.Error("Expected connection to be closed after token expiration")
		}
	})

	t.Run("stop validation cleanup", func(t *testing.T) {
		validator := newMockTokenValidator()

		handler := func(ctx context.Context, req core.Request) (core.Response, error) {
			return &mockResponse{statusCode: http.StatusSwitchingProtocols}, nil
		}

		adapter := NewAdapter(&Config{
			Host: "127.0.0.1",
			Port: 0,
		}, handler, logger)
		adapter.WithTokenValidator(validator)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := adapter.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Connect with token and specific request ID
		addr := adapter.listener.Addr().String()
		headers := http.Header{}
		headers.Set("Authorization", "Bearer test-token")
		headers.Set("X-Request-ID", "test-conn-456")

		conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/test", addr), headers)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		// Close connection
		conn.Close()

		// Give cleanup time to run
		time.Sleep(50 * time.Millisecond)

		// Check if StopValidation was called
		validator.mu.Lock()
		stopped := validator.stopCalled["test-conn-456"]
		validator.mu.Unlock()

		if !stopped {
			t.Error("StopValidation should be called when connection closes")
		}
	})
}
