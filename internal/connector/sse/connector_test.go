package sse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gateway/internal/core"
	gwerrors "gateway/pkg/errors"
	"log/slog"
)

// Mock SSE server for testing
func createMockSSEServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify SSE headers
		if accept := r.Header.Get("Accept"); accept != "text/event-stream" {
			t.Errorf("Expected Accept: text/event-stream, got %s", accept)
		}

		// Set SSE response headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		if handler != nil {
			handler(w, r)
		}
	}))
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DialTimeout != 10*time.Second {
		t.Errorf("Expected dial timeout 10s, got %v", config.DialTimeout)
	}
	if config.ResponseTimeout != 30*time.Second {
		t.Errorf("Expected response timeout 30s, got %v", config.ResponseTimeout)
	}
	if config.KeepaliveTimeout != 30*time.Second {
		t.Errorf("Expected keepalive timeout 30s, got %v", config.KeepaliveTimeout)
	}
}

func TestNewConnector(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name   string
		config *Config
		client *http.Client
	}{
		{
			name:   "with nil config and client",
			config: nil,
			client: nil,
		},
		{
			name: "with custom config",
			config: &Config{
				DialTimeout:     5 * time.Second,
				ResponseTimeout: 20 * time.Second,
			},
			client: nil,
		},
		{
			name:   "with custom client",
			config: nil,
			client: &http.Client{Timeout: 15 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := NewConnector(tt.config, tt.client, logger)

			if connector == nil {
				t.Fatal("Expected connector, got nil")
			}
			if connector.logger != logger {
				t.Error("Logger not set correctly")
			}
			if connector.client == nil {
				t.Error("Client not created")
			}
		})
	}
}

func TestConnector_Connect(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		instance      *core.ServiceInstance
		path          string
		headers       http.Header
		wantError     bool
		errorContains string
		checkResponse func(*testing.T, *Connection)
	}{
		{
			name: "successful connection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// Send initial event
				fmt.Fprintf(w, "event: welcome\ndata: connected\n\n")
				w.(http.Flusher).Flush()
			},
			instance: &core.ServiceInstance{
				ID:      "test-1",
				Address: "127.0.0.1",
				Port:    0, // Will be set from server
			},
			path: "/events",
			checkResponse: func(t *testing.T, conn *Connection) {
				// Read the welcome event
				event, err := conn.ReadEvent()
				if err != nil {
					t.Fatalf("Failed to read event: %v", err)
				}
				if event.Type != "welcome" || event.Data != "connected" {
					t.Errorf("Expected welcome event, got %+v", event)
				}
			},
		},
		{
			name: "with custom headers",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// Echo back authorization header in event
				auth := r.Header.Get("Authorization")
				fmt.Fprintf(w, "event: auth\ndata: %s\n\n", auth)
				w.(http.Flusher).Flush()
			},
			instance: &core.ServiceInstance{
				ID:      "test-2",
				Address: "127.0.0.1",
				Port:    0,
			},
			path: "/secure",
			headers: http.Header{
				"Authorization": []string{"Bearer token123"},
			},
			checkResponse: func(t *testing.T, conn *Connection) {
				event, err := conn.ReadEvent()
				if err != nil {
					t.Fatalf("Failed to read event: %v", err)
				}
				if event.Data != "Bearer token123" {
					t.Errorf("Expected auth header in event, got %s", event.Data)
				}
			},
		},
		{
			name: "non-200 response",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
			instance: &core.ServiceInstance{
				ID:      "test-4",
				Address: "127.0.0.1",
				Port:    0,
			},
			path:          "/error",
			wantError:     true,
			errorContains: "SSE backend returned status",
		},
		{
			name: "connection failure",
			instance: &core.ServiceInstance{
				ID:      "test-5",
				Address: "127.0.0.1",
				Port:    9999, // Invalid port
			},
			path:          "/test",
			wantError:     true,
			errorContains: "Failed to connect to SSE backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.serverHandler != nil {
				server = createMockSSEServer(t, tt.serverHandler)
				defer server.Close()

				// Extract port from server URL
				serverURL := strings.TrimPrefix(server.URL, "http://")
				if tt.instance.Scheme == "https" {
					serverURL = strings.TrimPrefix(server.URL, "https://")
				}
				_, portStr, _ := net.SplitHostPort(serverURL)
				var port int
				_, _ = fmt.Sscanf(portStr, "%d", &port)

				if tt.instance.Port == 0 {
					tt.instance.Port = port
				}
			}

			connector := NewConnector(DefaultConfig(), nil, logger)
			ctx := context.Background()

			conn, err := connector.Connect(ctx, tt.instance, tt.path, tt.headers)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if conn == nil {
					t.Fatal("Expected connection, got nil")
				}
				defer conn.Close()

				// Verify connection properties
				if conn.instance != tt.instance {
					t.Error("Instance not set correctly")
				}

				// Run response check if provided
				if tt.checkResponse != nil {
					tt.checkResponse(t, conn)
				}
			}
		})
	}
}

func TestConnection_ReadEvent(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name           string
		serverEvents   []string
		expectedEvents []core.SSEEvent
	}{
		{
			name: "simple events",
			serverEvents: []string{
				"event: test\ndata: hello\n\n",
				"data: world\n\n",
			},
			expectedEvents: []core.SSEEvent{
				{Type: "test", Data: "hello"},
				{Type: "message", Data: "world"}, // Default type
			},
		},
		{
			name: "events with ID",
			serverEvents: []string{
				"id: 123\nevent: update\ndata: first\n\n",
				"id: 124\nevent: update\ndata: second\n\n",
			},
			expectedEvents: []core.SSEEvent{
				{ID: "123", Type: "update", Data: "first"},
				{ID: "124", Type: "update", Data: "second"},
			},
		},
		{
			name: "multiline data",
			serverEvents: []string{
				"event: multiline\ndata: line1\ndata: line2\ndata: line3\n\n",
			},
			expectedEvents: []core.SSEEvent{
				{Type: "multiline", Data: "line1\nline2\nline3"},
			},
		},
		{
			name: "empty data",
			serverEvents: []string{
				"event: empty\ndata:\n\n",
				"event: ping\n\n",
			},
			expectedEvents: []core.SSEEvent{
				{Type: "empty", Data: ""},
				{Type: "ping", Data: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createMockSSEServer(t, func(w http.ResponseWriter, r *http.Request) {
				for _, event := range tt.serverEvents {
					fmt.Fprint(w, event)
					w.(http.Flusher).Flush()
				}
			})
			defer server.Close()

			// Extract port
			serverURL := strings.TrimPrefix(server.URL, "http://")
			_, portStr, _ := net.SplitHostPort(serverURL)
			var port int
			_, _ = fmt.Sscanf(portStr, "%d", &port)

			instance := &core.ServiceInstance{
				ID:      "test",
				Address: "127.0.0.1",
				Port:    port,
			}

			connector := NewConnector(DefaultConfig(), nil, logger)
			conn, err := connector.Connect(context.Background(), instance, "/", nil)
			if err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}
			defer conn.Close()

			// Read events
			for i, expected := range tt.expectedEvents {
				event, err := conn.ReadEvent()
				if err != nil {
					t.Fatalf("Failed to read event %d: %v", i, err)
				}

				if event.ID != expected.ID {
					t.Errorf("Event %d: expected ID %s, got %s", i, expected.ID, event.ID)
				}
				if event.Type != expected.Type {
					t.Errorf("Event %d: expected type %s, got %s", i, expected.Type, event.Type)
				}
				if event.Data != expected.Data {
					t.Errorf("Event %d: expected data %s, got %s", i, expected.Data, event.Data)
				}
			}
		})
	}
}

func TestConnection_Proxy(t *testing.T) {
	logger := slog.Default()

	// Create backend server that sends periodic events
	server := createMockSSEServer(t, func(w http.ResponseWriter, r *http.Request) {
		events := []string{
			"event: start\ndata: begin\n\n",
			"id: 1\nevent: data\ndata: first\n\n",
			"id: 2\nevent: data\ndata: second\n\n",
			"event: end\ndata: done\n\n",
		}

		for _, event := range events {
			fmt.Fprint(w, event)
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	})
	defer server.Close()

	// Extract port
	serverURL := strings.TrimPrefix(server.URL, "http://")
	_, portStr, _ := net.SplitHostPort(serverURL)
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)

	instance := &core.ServiceInstance{
		ID:      "backend",
		Address: "127.0.0.1",
		Port:    port,
	}

	// Connect to backend
	connector := NewConnector(DefaultConfig(), nil, logger)
	conn, err := connector.Connect(context.Background(), instance, "/", nil)
	if err != nil {
		t.Fatalf("Failed to connect to backend: %v", err)
	}
	defer conn.Close()

	// Create mock client writer
	clientWriter := httptest.NewRecorder()
	mockWriter := &mockSSEWriter{
		w:      clientWriter,
		events: []core.SSEEvent{},
	}

	// Proxy in background
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	proxyDone := make(chan error, 1)
	go func() {
		proxyDone <- conn.Proxy(ctx, mockWriter)
	}()

	// Wait for proxy to complete
	select {
	case err := <-proxyDone:
		// EOF is expected when the server closes the connection after sending all events
		if err != nil && err != context.DeadlineExceeded && err != io.EOF {
			// Check if it's a wrapped EOF error
			var gwErr *gwerrors.Error
			if errors.As(err, &gwErr) && gwErr.Cause != nil && gwErr.Cause == io.EOF {
				// This is expected - server closed connection after sending all events
			} else {
				t.Errorf("Proxy error: %v", err)
			}
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Proxy did not complete in time")
	}

	// Verify events were proxied
	if len(mockWriter.events) < 4 {
		t.Errorf("Expected at least 4 events, got %d", len(mockWriter.events))
	}

	// Check first event
	if len(mockWriter.events) > 0 {
		if mockWriter.events[0].Type != "start" || mockWriter.events[0].Data != "begin" {
			t.Errorf("First event mismatch: %+v", mockWriter.events[0])
		}
	}
}

func TestConnection_ContextCancellation(t *testing.T) {
	logger := slog.Default()

	// Create server that sends events slowly
	server := createMockSSEServer(t, func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 100; i++ {
			fmt.Fprintf(w, "event: tick\ndata: %d\n\n", i)
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
	})
	defer server.Close()

	// Extract port
	serverURL := strings.TrimPrefix(server.URL, "http://")
	_, portStr, _ := net.SplitHostPort(serverURL)
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)

	instance := &core.ServiceInstance{
		ID:      "test",
		Address: "127.0.0.1",
		Port:    port,
	}

	// Connect with cancelable context
	ctx, cancel := context.WithCancel(context.Background())

	connector := NewConnector(DefaultConfig(), nil, logger)
	conn, err := connector.Connect(ctx, instance, "/", nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read first event
	event, err := conn.ReadEvent()
	if err != nil {
		t.Fatalf("Failed to read first event: %v", err)
	}
	if event.Type != "tick" {
		t.Errorf("Expected tick event, got %s", event.Type)
	}

	// Cancel context
	cancel()

	// Try to read more events - should fail
	_, err = conn.ReadEvent()
	if err == nil {
		t.Error("Expected error after context cancellation")
	}
}

// Mock SSE writer for testing
type mockSSEWriter struct {
	w      http.ResponseWriter
	events []core.SSEEvent
}

func (m *mockSSEWriter) WriteEvent(event *core.SSEEvent) error {
	m.events = append(m.events, *event)

	// Write to actual response writer
	if event.ID != "" {
		fmt.Fprintf(m.w, "id: %s\n", event.ID)
	}
	if event.Type != "" && event.Type != "message" {
		fmt.Fprintf(m.w, "event: %s\n", event.Type)
	}
	fmt.Fprintf(m.w, "data: %s\n\n", event.Data)

	if flusher, ok := m.w.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

func (m *mockSSEWriter) WriteComment(comment string) error {
	fmt.Fprintf(m.w, ":%s\n", comment)
	if flusher, ok := m.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (m *mockSSEWriter) Flush() error {
	if flusher, ok := m.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (m *mockSSEWriter) Close() error {
	return nil
}
