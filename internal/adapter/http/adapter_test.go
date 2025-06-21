package http

import (
	"context"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"gateway/pkg/requestid"
	"io"
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

func (m *mockResponse) StatusCode() int             { return m.statusCode }
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
				pw.Write([]byte("chunk\n"))
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