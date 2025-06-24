package http

import (
	"context"
	"fmt"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"gateway/pkg/requestid"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// mockHandler for testing
type mockHandler struct {
	handleFunc func(context.Context, core.Request) (core.Response, error)
}

func (m *mockHandler) Handle(ctx context.Context, req core.Request) (core.Response, error) {
	if m.handleFunc != nil {
		return m.handleFunc(ctx, req)
	}
	return nil, nil
}

// mockResponse for testing
type mockResponse struct {
	statusCode int
	headers    map[string][]string
	body       io.ReadCloser
}

func (m *mockResponse) StatusCode() int              { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string { return m.headers }
func (m *mockResponse) Body() io.ReadCloser          { return m.body }

func TestAdapterServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		handler        core.Handler
		request        *http.Request
		expectedStatus int
		expectedBody   string
		expectedHeader map[string]string
	}{
		{
			name: "successful request",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				return &mockResponse{
					statusCode: http.StatusOK,
					headers: map[string][]string{
						"Content-Type": {"application/json"},
						"X-Custom":     {"test-value"},
					},
					body: io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
				}, nil
			},
			request:        httptest.NewRequest("GET", "/api/test", nil),
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"ok"}`,
			expectedHeader: map[string]string{
				"Content-Type": "application/json",
				"X-Custom":     "test-value",
			},
		},
		{
			name: "request with body",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				// Echo the request body
				body, _ := io.ReadAll(req.Body())
				return &mockResponse{
					statusCode: http.StatusOK,
					headers:    map[string][]string{"Content-Type": {"text/plain"}},
					body:       io.NopCloser(strings.NewReader(string(body))),
				}, nil
			},
			request:        httptest.NewRequest("POST", "/api/echo", strings.NewReader("test data")),
			expectedStatus: http.StatusOK,
			expectedBody:   "test data",
		},
		{
			name: "error handling - not found",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				return nil, errors.NewError(errors.ErrorTypeNotFound, "route not found").
					WithDetail("path", req.Path())
			},
			request:        httptest.NewRequest("GET", "/api/unknown", nil),
			expectedStatus: http.StatusNotFound,
			expectedBody:   "route not found",
		},
		{
			name: "error handling - internal error",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				return nil, errors.NewError(errors.ErrorTypeInternal, "internal server error")
			},
			request:        httptest.NewRequest("GET", "/api/error", nil),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error",
		},
		{
			name: "error handling - timeout",
			handler: func(ctx context.Context, req core.Request) (core.Response, error) {
				return nil, errors.NewError(errors.ErrorTypeTimeout, "request timeout")
			},
			request:        httptest.NewRequest("GET", "/api/timeout", nil),
			expectedStatus: http.StatusRequestTimeout,
			expectedBody:   "request timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Host: "127.0.0.1", Port: 8080}
			adapter := New(cfg, tt.handler)

			// Create response recorder
			recorder := httptest.NewRecorder()

			// Serve the request
			adapter.ServeHTTP(recorder, tt.request)

			// Check status code
			if recorder.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, want %d", recorder.Code, tt.expectedStatus)
			}

			// Check body
			body := recorder.Body.String()
			// For error responses, http.Error adds a newline
			if tt.expectedStatus >= 400 && body != tt.expectedBody+"\n" {
				t.Errorf("Body = %q, want %q", body, tt.expectedBody+"\n")
			} else if tt.expectedStatus < 400 && body != tt.expectedBody {
				t.Errorf("Body = %q, want %q", body, tt.expectedBody)
			}

			// Check headers
			for key, value := range tt.expectedHeader {
				if got := recorder.Header().Get(key); got != value {
					t.Errorf("Header[%s] = %q, want %q", key, got, value)
				}
			}
		})
	}
}

func TestAdapterRequestConversion(t *testing.T) {
	var capturedReq core.Request
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		capturedReq = req
		return &mockResponse{
			statusCode: http.StatusOK,
			headers:    make(map[string][]string),
			body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}

	cfg := Config{Host: "127.0.0.1", Port: 8080}
	adapter := New(cfg, handler)

	// Create test request with various attributes
	req := httptest.NewRequest("POST", "/api/test?query=value", strings.NewReader("request body"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.RemoteAddr = "192.168.1.100:12345"

	// Serve the request
	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, req)

	// Verify request conversion
	if capturedReq == nil {
		t.Fatal("Handler was not called")
	}

	// Check method
	if capturedReq.Method() != "POST" {
		t.Errorf("Method = %s, want POST", capturedReq.Method())
	}

	// Check path
	if capturedReq.Path() != "/api/test" {
		t.Errorf("Path = %s, want /api/test", capturedReq.Path())
	}

	// Check URL (should include query)
	if capturedReq.URL() != "/api/test?query=value" {
		t.Errorf("URL = %s, want /api/test?query=value", capturedReq.URL())
	}

	// Check remote address
	if capturedReq.RemoteAddr() != "192.168.1.100:12345" {
		t.Errorf("RemoteAddr = %s, want 192.168.1.100:12345", capturedReq.RemoteAddr())
	}

	// Check headers
	headers := capturedReq.Headers()
	if ct := headers["Content-Type"]; len(ct) == 0 || ct[0] != "application/json" {
		t.Error("Content-Type header not preserved")
	}
	if ch := headers["X-Custom-Header"]; len(ch) == 0 || ch[0] != "custom-value" {
		t.Error("X-Custom-Header not preserved")
	}

	// Check request ID format (timestamp-randomhex)
	if capturedReq.ID() == "" {
		t.Error("Request ID should be generated")
	}
	// Verify format: should contain a hyphen separating timestamp and random hex
	if !strings.Contains(capturedReq.ID(), "-") {
		t.Errorf("Request ID format invalid: %s, expected format: timestamp-randomhex", capturedReq.ID())
	}
}

func TestAdapterContextPropagation(t *testing.T) {
	var capturedCtx context.Context
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		capturedCtx = ctx
		// Simulate work
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return &mockResponse{
				statusCode: http.StatusOK,
				headers:    make(map[string][]string),
				body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}
	}

	cfg := Config{Host: "127.0.0.1", Port: 8080}
	adapter := New(cfg, handler)

	// Create request with context
	req := httptest.NewRequest("GET", "/api/test", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Serve the request
	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, req)

	// Verify context was propagated
	if capturedCtx == nil {
		t.Fatal("Context was not propagated")
	}

	// Check that context has deadline
	if _, ok := capturedCtx.Deadline(); !ok {
		t.Error("Context deadline was not propagated")
	}
}

func TestAdapterStreamingResponse(t *testing.T) {
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Create a pipe for streaming
		pr, pw := io.Pipe()

		// Write data asynchronously
		go func() {
			defer pw.Close()
			for i := 0; i < 3; i++ {
				_, _ = pw.Write([]byte("chunk\n"))
				time.Sleep(5 * time.Millisecond)
			}
		}()

		return &mockResponse{
			statusCode: http.StatusOK,
			headers:    map[string][]string{"Content-Type": {"text/plain"}},
			body:       pr,
		}, nil
	}

	cfg := Config{Host: "127.0.0.1", Port: 8080}
	adapter := New(cfg, handler)

	// Create request
	req := httptest.NewRequest("GET", "/stream", nil)
	recorder := httptest.NewRecorder()

	// Serve the request
	adapter.ServeHTTP(recorder, req)

	// Check response
	if recorder.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	if expected := "chunk\nchunk\nchunk\n"; body != expected {
		t.Errorf("Body = %q, want %q", body, expected)
	}
}

func TestAdapterRequestIDUniqueness(t *testing.T) {
	// Generate multiple request IDs
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := requestid.GenerateRequestID()

		// Check format
		parts := strings.Split(id, "-")
		if len(parts) != 2 {
			t.Errorf("Invalid request ID format: %s", id)
			continue
		}

		// Verify timestamp part is numeric
		if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
			t.Errorf("Timestamp part should be numeric: %s", parts[0])
		}

		// Verify random part is hex
		if len(parts[1]) != 8 { // 4 bytes = 8 hex chars
			t.Errorf("Random part should be 8 hex characters: %s", parts[1])
		}

		// Check uniqueness
		if ids[id] {
			t.Errorf("Duplicate request ID generated: %s", id)
		}
		ids[id] = true
	}

	// All IDs should be unique
	if len(ids) != 100 {
		t.Errorf("Expected 100 unique IDs, got %d", len(ids))
	}
}

func TestAdapterPanicRecovery(t *testing.T) {
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		panic("test panic")
	}

	cfg := Config{Host: "127.0.0.1", Port: 8080}
	adapter := New(cfg, handler)

	// Create request
	req := httptest.NewRequest("GET", "/panic", nil)
	recorder := httptest.NewRecorder()

	// The adapter doesn't have built-in panic recovery
	// So this should panic (as expected in production, recovery would be done by middleware)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected ServeHTTP to panic, but it didn't")
		} else if r != "test panic" {
			t.Errorf("Expected panic with 'test panic', got: %v", r)
		}
	}()

	adapter.ServeHTTP(recorder, req)

	// This line should not be reached
	t.Error("ServeHTTP should have panicked")
}

func TestAdapterBuiltinEndpoints(t *testing.T) {
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Should not be called for builtin endpoints
		t.Error("Handler should not be called for builtin endpoints")
		return nil, nil
	}

	cfg := Config{Host: "127.0.0.1", Port: 8080}
	adapter := New(cfg, handler)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		checkBody      func(t *testing.T, body string)
	}{
		{
			name:           "gateway health",
			path:           "/_gateway/health",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, `"status":"healthy"`) {
					t.Error("Expected healthy status in response")
				}
				if !strings.Contains(body, `"service":"gateway"`) {
					t.Error("Expected gateway service in response")
				}
			},
		},
		{
			name:           "gateway echo",
			path:           "/_gateway/echo",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				// Echo response is JSON formatted with spaces due to SetIndent
				if !strings.Contains(body, `"method": "GET"`) {
					t.Errorf("Expected method in echo response, got: %s", body)
				}
				if !strings.Contains(body, `"path": "/_gateway/echo"`) {
					t.Errorf("Expected path in echo response, got: %s", body)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			recorder := httptest.NewRecorder()

			adapter.ServeHTTP(recorder, req)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", recorder.Code, tt.expectedStatus)
			}

			if tt.checkBody != nil {
				tt.checkBody(t, recorder.Body.String())
			}
		})
	}
}

func TestAdapterMaxRequestSize(t *testing.T) {
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Should not be called for oversized requests
		t.Error("Handler should not be called for oversized requests")
		return nil, nil
	}

	cfg := Config{
		Host:           "127.0.0.1",
		Port:           8080,
		MaxRequestSize: 1024, // 1KB limit
	}
	adapter := New(cfg, handler)

	// Create oversized request
	largeBody := strings.Repeat("x", 2048) // 2KB
	req := httptest.NewRequest("POST", "/api/upload", strings.NewReader(largeBody))
	req.Header.Set("Content-Length", strconv.Itoa(len(largeBody)))
	
	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}

	if !strings.Contains(recorder.Body.String(), "Request body too large") {
		t.Error("Expected 'Request body too large' error message")
	}
}

func TestAdapterHealthEndpoints(t *testing.T) {
	// Create mock health handler
	healthCalled := make(map[string]bool)
	mockHealth := &mockHealthHandler{
		healthFunc: func(w http.ResponseWriter, r *http.Request) {
			healthCalled["health"] = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("health ok"))
		},
		readyFunc: func(w http.ResponseWriter, r *http.Request) {
			healthCalled["ready"] = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready ok"))
		},
		liveFunc: func(w http.ResponseWriter, r *http.Request) {
			healthCalled["live"] = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("live ok"))
		},
	}

	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Should not be called for health endpoints
		t.Error("Handler should not be called for health endpoints")
		return nil, nil
	}

	cfg := Config{Host: "127.0.0.1", Port: 8080}
	adapter := New(cfg, handler).
		WithHealthHandler(mockHealth).
		WithHealthConfig(HealthConfig{
			Enabled:    true,
			HealthPath: "/health",
			ReadyPath:  "/ready",
			LivePath:   "/live",
		})

	tests := []struct {
		path     string
		key      string
		expected string
	}{
		{"/health", "health", "health ok"},
		{"/ready", "ready", "ready ok"},
		{"/live", "live", "live ok"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			recorder := httptest.NewRecorder()

			adapter.ServeHTTP(recorder, req)

			if !healthCalled[tt.key] {
				t.Errorf("Health handler for %s was not called", tt.key)
			}

			if recorder.Body.String() != tt.expected {
				t.Errorf("Body = %q, want %q", recorder.Body.String(), tt.expected)
			}
		})
	}
}

// mockHealthHandler implements HealthHandler for testing
type mockHealthHandler struct {
	healthFunc func(w http.ResponseWriter, r *http.Request)
	readyFunc  func(w http.ResponseWriter, r *http.Request)
	liveFunc   func(w http.ResponseWriter, r *http.Request)
}

func (m *mockHealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	if m.healthFunc != nil {
		m.healthFunc(w, r)
	}
}

func (m *mockHealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if m.readyFunc != nil {
		m.readyFunc(w, r)
	}
}

func (m *mockHealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	if m.liveFunc != nil {
		m.liveFunc(w, r)
	}
}

func TestAdapterMetricsEndpoint(t *testing.T) {
	metricsCalled := false
	mockMetrics := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metricsCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("metrics data"))
	})

	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Should not be called for metrics endpoint
		t.Error("Handler should not be called for metrics endpoint")
		return nil, nil
	}

	cfg := Config{Host: "127.0.0.1", Port: 8080}
	adapter := New(cfg, handler).
		WithMetricsHandler(mockMetrics)

	// Test default metrics path
	req := httptest.NewRequest("GET", "/metrics", nil)
	recorder := httptest.NewRecorder()

	adapter.ServeHTTP(recorder, req)

	if !metricsCalled {
		t.Error("Metrics handler was not called")
	}

	if recorder.Body.String() != "metrics data" {
		t.Errorf("Body = %q, want %q", recorder.Body.String(), "metrics data")
	}

	// Test custom metrics path
	adapter = New(cfg, handler).
		WithMetricsHandler(mockMetrics).
		WithMetricsPath("/custom/metrics")

	metricsCalled = false
	req = httptest.NewRequest("GET", "/custom/metrics", nil)
	recorder = httptest.NewRecorder()

	adapter.ServeHTTP(recorder, req)

	if !metricsCalled {
		t.Error("Metrics handler was not called for custom path")
	}
}

func TestAdapterSSERequest(t *testing.T) {
	sseCalled := false
	mockSSE := &mockSSEHandler{
		handleFunc: func(w http.ResponseWriter, r *http.Request) {
			sseCalled = true
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("data: test\n\n"))
		},
	}

	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Should not be called for SSE requests
		t.Error("Handler should not be called for SSE requests")
		return nil, nil
	}

	cfg := Config{Host: "127.0.0.1", Port: 8080}
	adapter := New(cfg, handler).
		WithSSEHandler(mockSSE)

	req := httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Accept", "text/event-stream")
	recorder := httptest.NewRecorder()

	adapter.ServeHTTP(recorder, req)

	if !sseCalled {
		t.Error("SSE handler was not called")
	}

	if ct := recorder.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

// mockSSEHandler implements SSEHandler for testing
type mockSSEHandler struct {
	handleFunc func(w http.ResponseWriter, r *http.Request)
}

func (m *mockSSEHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	if m.handleFunc != nil {
		m.handleFunc(w, r)
	}
}

func TestAdapterStartStop(t *testing.T) {
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{
			statusCode: http.StatusOK,
			headers:    make(map[string][]string),
			body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	}

	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	cfg := Config{
		Host:         "127.0.0.1",
		Port:         port,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	}
	adapter := New(cfg, handler)

	// Start server
	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	// Test that server is running
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/test", port))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Stop server
	stopCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if err := adapter.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Verify server stopped
	time.Sleep(10 * time.Millisecond)
	_, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/test", port))
	if err == nil {
		t.Error("Server should not be accessible after stop")
	}
}

func TestAdapterWithMethods(t *testing.T) {
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{
			statusCode: http.StatusOK,
			headers:    make(map[string][]string),
			body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	}

	cfg := Config{Host: "127.0.0.1", Port: 8080}
	adapter := New(cfg, handler)

	// Test chaining
	mockHealth := &mockHealthHandler{}
	mockSSE := &mockSSEHandler{}
	mockMetrics := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	mockCORS := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	result := adapter.
		WithHealthHandler(mockHealth).
		WithSSEHandler(mockSSE).
		WithMetricsHandler(mockMetrics).
		WithCORSHandler(mockCORS).
		WithHealthConfig(HealthConfig{Enabled: true}).
		WithMetricsPath("/metrics")

	// Verify adapter is returned for chaining
	if result != adapter {
		t.Error("With methods should return the adapter for chaining")
	}

	// Verify fields are set
	if adapter.healthHandler != mockHealth {
		t.Error("Health handler not set correctly")
	}
	if adapter.sseHandler != mockSSE {
		t.Error("SSE handler not set correctly")
	}
	// Can't compare functions directly, just check they're not nil
	if adapter.metricsHandler == nil {
		t.Error("Metrics handler not set")
	}
	if adapter.corsHandler == nil {
		t.Error("CORS handler not set")
	}
	if !adapter.healthConfig.Enabled {
		t.Error("Health config not set correctly")
	}
	if adapter.config.MetricsPath != "/metrics" {
		t.Error("Metrics path not set correctly")
	}
}
