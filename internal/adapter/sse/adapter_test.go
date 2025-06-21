package sse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gateway/internal/core"
	"gateway/pkg/errors"
	"gateway/pkg/request"
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
	
	// Write some SSE events if this is an SSE request
	if sseReq, ok := req.(*sseRequest); ok {
		// Write test events
		sseReq.writer.WriteEvent(&core.SSEEvent{
			Type: "test",
			Data: "hello",
		})
		sseReq.writer.WriteEvent(&core.SSEEvent{
			ID:   "123",
			Type: "message",
			Data: "world",
		})
	}
	
	return m.resp, m.err
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.Enabled {
		t.Error("Expected SSE to be disabled by default")
	}
	if config.WriteTimeout != 60 {
		t.Errorf("Expected write timeout 60s, got %d", config.WriteTimeout)
	}
	if config.KeepaliveTimeout != 30 {
		t.Errorf("Expected keepalive timeout 30s, got %d", config.KeepaliveTimeout)
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
				Enabled:          true,
				WriteTimeout:     120,
				KeepaliveTimeout: 60,
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
		})
	}
}

func TestAdapter_HandleSSE(t *testing.T) {
	logger := slog.Default()
	
	tests := []struct {
		name           string
		handler        core.Handler
		headers        map[string]string
		wantStatus     int
		wantHeaders    map[string]string
		wantBody       string
		wantBodyPrefix string
	}{
		{
			name: "successful SSE stream",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				// Verify request properties
				if req.Method() != "GET" {
					t.Errorf("Expected method GET, got %s", req.Method())
				}
				
				// Write SSE event
				if sseReq, ok := req.(*sseRequest); ok {
					sseReq.writer.WriteEvent(&core.SSEEvent{
						Type: "test",
						Data: "hello world",
					})
				}
				
				return nil, nil
			},
			headers: map[string]string{
				"Accept": "text/event-stream",
			},
			wantStatus: http.StatusOK,
			wantHeaders: map[string]string{
				"Content-Type":      "text/event-stream",
				"Cache-Control":     "no-cache",
				"Connection":        "keep-alive",
				"X-Accel-Buffering": "no",
			},
			wantBodyPrefix: "event: test\ndata: hello world\n\n",
		},
		{
			name: "with request ID",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				if req.ID() != "test-request-id" {
					t.Errorf("Expected request ID 'test-request-id', got %s", req.ID())
				}
				return nil, nil
			},
			headers: map[string]string{
				"Accept":       "text/event-stream",
				"X-Request-ID": "test-request-id",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "handler error",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				return nil, errors.NewError(errors.ErrorTypeInternal, "handler error")
			},
			headers: map[string]string{
				"Accept": "text/event-stream",
			},
			wantStatus:     http.StatusOK,
			wantBodyPrefix: "event: error\ndata: internal: handler error\n\n",
		},
		{
			name: "not acceptable",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				return nil, nil
			},
			headers: map[string]string{
				"Accept": "application/json",
			},
			wantStatus: http.StatusNotAcceptable,
			wantBody:   "SSE not accepted\n",
		},
		{
			name: "wildcard accept",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				return nil, nil
			},
			headers: map[string]string{
				"Accept": "*/*",
			},
			wantStatus: http.StatusOK,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAdapter(&Config{KeepaliveTimeout: 0}, tt.handler, logger)
			
			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			
			// Record response
			w := httptest.NewRecorder()
			
			// Handle SSE
			adapter.HandleSSE(w, req)
			
			// Check status
			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
			
			// Check headers
			for k, v := range tt.wantHeaders {
				if got := w.Header().Get(k); got != v {
					t.Errorf("Expected header %s=%s, got %s", k, v, got)
				}
			}
			
			// Check body
			body := w.Body.String()
			if tt.wantBody != "" && body != tt.wantBody {
				t.Errorf("Expected body %q, got %q", tt.wantBody, body)
			}
			if tt.wantBodyPrefix != "" && !strings.HasPrefix(body, tt.wantBodyPrefix) {
				t.Errorf("Expected body to start with %q, got %q", tt.wantBodyPrefix, body)
			}
		})
	}
}

func TestAdapter_HandleSSE_Keepalive(t *testing.T) {
	logger := slog.Default()
	
	// Create a context we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Keep connection open for keepalive test
		<-ctx.Done()
		return nil, ctx.Err()
	}
	
	adapter := NewAdapter(&Config{
		KeepaliveTimeout: 1, // 1 second for faster test
	}, handler, logger)
	
	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")
	
	w := httptest.NewRecorder()
	
	// Handle in goroutine
	done := make(chan bool)
	go func() {
		adapter.HandleSSE(w, req)
		done <- true
	}()
	
	// Wait longer than keepalive timeout to capture keepalive
	time.Sleep(1200 * time.Millisecond)
	
	// Cancel context to stop handler
	cancel()
	
	// Wait for completion
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Handler did not complete in time")
	}
	
	// Check that we got keepalive comments
	body := w.Body.String()
	if !strings.Contains(body, ": keepalive") {
		t.Errorf("Expected keepalive comments in response, got: %s", body)
	}
}

func TestAdapter_HandleSSE_ContextCancellation(t *testing.T) {
	logger := slog.Default()
	
	handlerStarted := make(chan bool)
	handlerDone := make(chan bool)
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		handlerStarted <- true
		<-ctx.Done()
		handlerDone <- true
		return nil, ctx.Err()
	}
	
	adapter := NewAdapter(DefaultConfig(), handler, logger)
	
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")
	
	w := httptest.NewRecorder()
	
	// Handle in goroutine
	go func() {
		adapter.HandleSSE(w, req)
	}()
	
	// Wait for handler to start
	<-handlerStarted
	
	// Cancel context
	cancel()
	
	// Wait for handler to complete
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Error("Handler did not respond to context cancellation")
	}
}

func TestAdapter_HandleSSE_MultipleEvents(t *testing.T) {
	logger := slog.Default()
	
	events := []core.SSEEvent{
		{Type: "start", Data: "begin"},
		{ID: "1", Type: "data", Data: "first"},
		{ID: "2", Type: "data", Data: "second"},
		{Type: "end", Data: "done"},
	}
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		if sseReq, ok := req.(*sseRequest); ok {
			for _, event := range events {
				if err := sseReq.writer.WriteEvent(&event); err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	}
	
	adapter := NewAdapter(&Config{KeepaliveTimeout: 0}, handler, logger)
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "text/event-stream")
	
	w := httptest.NewRecorder()
	adapter.HandleSSE(w, req)
	
	body := w.Body.String()
	
	// Verify all events are in the response
	expectedParts := []string{
		"event: start\ndata: begin\n\n",
		"id: 1\nevent: data\ndata: first\n\n",
		"id: 2\nevent: data\ndata: second\n\n",
		"event: end\ndata: done\n\n",
	}
	
	for _, part := range expectedParts {
		if !strings.Contains(body, part) {
			t.Errorf("Expected to find %q in response body", part)
		}
	}
}

func TestSSERequest(t *testing.T) {
	// Create a mock HTTP request
	httpReq := httptest.NewRequest("GET", "/test", nil)
	httpReq.RemoteAddr = "192.168.1.1:12345"
	httpReq.Header.Set("X-Custom", "value")
	
	// Create mock writer
	w := httptest.NewRecorder()
	writer := newWriter(w)
	
	// Create SSE request
	req := &sseRequest{
		BaseRequest: request.NewBase("test-id", httpReq, "SSE", "sse"),
		writer:      writer,
	}
	
	// Verify properties
	if req.ID() != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", req.ID())
	}
	if req.Method() != "SSE" {
		t.Errorf("Expected method 'SSE', got %s", req.Method())
	}
	if req.Path() != "/test" {
		t.Errorf("Expected path '/test', got %s", req.Path())
	}
	
	// Test that Body returns a non-nil io.ReadCloser (even if empty)
	body := req.Body()
	if body == nil {
		t.Error("Expected Body() to return non-nil io.ReadCloser")
	} else {
		// Should be able to close it
		body.Close()
	}
}

// Test concurrent SSE connections
func TestAdapter_Concurrent(t *testing.T) {
	logger := slog.Default()
	
	connCount := 0
	var mu sync.Mutex
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		mu.Lock()
		connCount++
		current := connCount
		mu.Unlock()
		
		if sseReq, ok := req.(*sseRequest); ok {
			sseReq.writer.WriteEvent(&core.SSEEvent{
				Type: "connection",
				Data: fmt.Sprintf("conn-%d", current),
			})
		}
		
		// Simulate some work
		time.Sleep(50 * time.Millisecond)
		return nil, nil
	}
	
	adapter := NewAdapter(&Config{KeepaliveTimeout: 0}, handler, logger)
	
	// Launch concurrent connections
	const numConnections = 5
	results := make(chan string, numConnections)
	
	for i := 0; i < numConnections; i++ {
		go func(i int) {
			req := httptest.NewRequest("GET", fmt.Sprintf("/test%d", i), nil)
			req.Header.Set("Accept", "text/event-stream")
			
			w := httptest.NewRecorder()
			adapter.HandleSSE(w, req)
			
			results <- w.Body.String()
		}(i)
	}
	
	// Collect results
	for i := 0; i < numConnections; i++ {
		body := <-results
		if !strings.Contains(body, "event: connection") {
			t.Error("Expected connection event in response")
		}
	}
	
	// Verify all connections were handled
	mu.Lock()
	if connCount != numConnections {
		t.Errorf("Expected %d connections, got %d", numConnections, connCount)
	}
	mu.Unlock()
}