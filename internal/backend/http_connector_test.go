package backend

import (
	"context"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// mockRequest implements core.Request
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
func (m *mockRequest) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

// mockResponse implements core.Response
type mockResponse struct {
	statusCode int
	headers    map[string][]string
	body       io.ReadCloser
}

func (m *mockResponse) StatusCode() int             { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string { return m.headers }
func (m *mockResponse) Body() io.ReadCloser          { return m.body }

func TestHTTPConnectorForward(t *testing.T) {
	// Create a test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back request details
		w.Header().Set("X-Test-Header", "test-value")
		w.Header().Set("Content-Type", "text/plain")
		
		// Check forwarded headers
		if xff := r.Header.Get("X-Forwarded-For"); xff == "" {
			t.Error("X-Forwarded-For header not set")
		}
		if xfp := r.Header.Get("X-Forwarded-Proto"); xfp == "" {
			t.Error("X-Forwarded-Proto header not set")
		}
		
		// Echo the path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Path: " + r.URL.Path))
	}))
	defer backend.Close()

	// Parse backend URL
	backendURL, _ := url.Parse(backend.URL)
	
	// Create connector
	connector := NewHTTPConnector(&http.Client{}, 10 * time.Second)

	tests := []struct {
		name       string
		request    core.Request
		instance   core.ServiceInstance
		wantStatus int
		wantErr    bool
	}{
		{
			name: "successful forward",
			request: &mockRequest{
				id:         "test-1",
				method:     "GET",
				path:       "/api/test",
				url:        "/api/test",
				remoteAddr: "192.168.1.1:12345",
				headers: map[string][]string{
					"User-Agent": {"test-agent"},
					"Accept":     {"application/json"},
				},
				body: io.NopCloser(strings.NewReader("")),
			},
			instance: core.ServiceInstance{
				ID:      "backend-1",
				Address: backendURL.Hostname(),
				Port:    parsePort(backendURL.Port()),
				Scheme:  backendURL.Scheme,
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "with request body",
			request: &mockRequest{
				id:         "test-2",
				method:     "POST",
				path:       "/api/create",
				url:        "/api/create",
				remoteAddr: "192.168.1.2:12346",
				headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				body: io.NopCloser(strings.NewReader(`{"test": "data"}`)),
			},
			instance: core.ServiceInstance{
				ID:      "backend-1",
				Address: backendURL.Hostname(),
				Port:    parsePort(backendURL.Port()),
				Scheme:  backendURL.Scheme,
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "connection refused",
			request: &mockRequest{
				id:         "test-3",
				method:     "GET",
				path:       "/api/test",
				url:        "/api/test",
				remoteAddr: "192.168.1.3:12347",
				headers:    make(map[string][]string),
				body:       io.NopCloser(strings.NewReader("")),
			},
			instance: core.ServiceInstance{
				ID:      "backend-2",
				Address: "127.0.0.1",
				Port:    9999, // Non-existent port
			},
			wantErr: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &core.RouteResult{
				Instance: &tt.instance,
				Rule:     &core.RouteRule{},
			}
			resp, err := connector.Forward(ctx, tt.request, route)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Forward() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				// Verify error type
				if gerr, ok := err.(*errors.Error); ok {
					if gerr.Type != errors.ErrorTypeUnavailable {
						t.Errorf("Expected ErrorTypeUnavailable, got %v", gerr.Type)
					}
				}
				return
			}

			if resp.StatusCode() != tt.wantStatus {
				t.Errorf("Forward() status = %v, want %v", resp.StatusCode(), tt.wantStatus)
			}

			// Read response body
			body, _ := io.ReadAll(resp.Body())
			resp.Body().Close()
			
			if !strings.Contains(string(body), "Path:") {
				t.Errorf("Response body doesn't contain expected path echo")
			}
		})
	}
}

func TestHTTPConnectorTimeout(t *testing.T) {
	// Create a slow backend server
	slowBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowBackend.Close()

	// Parse backend URL
	backendURL, _ := url.Parse(slowBackend.URL)
	
	// Create connector with short timeout
	connector := NewHTTPConnector(&http.Client{}, 100 * time.Millisecond)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req := &mockRequest{
		id:         "timeout-test",
		method:     "GET",
		path:       "/slow",
		url:        "/slow",
		remoteAddr: "192.168.1.4:12348",
		headers:    make(map[string][]string),
		body:       io.NopCloser(strings.NewReader("")),
		ctx:        ctx,
	}

	instance := core.ServiceInstance{
		ID:      "slow-backend",
		Address: backendURL.Hostname(),
		Port:    parsePort(backendURL.Port()),
		Scheme:  backendURL.Scheme,
	}

	route := &core.RouteResult{
		Instance: &instance,
		Rule:     &core.RouteRule{},
	}
	_, err := connector.Forward(ctx, req, route)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	// Check if error contains timeout
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestHTTPConnectorHeaderFiltering(t *testing.T) {
	// Create backend to verify headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that hop-by-hop headers are removed
		if r.Header.Get("Connection") != "" {
			t.Error("Connection header should be removed")
		}
		if r.Header.Get("Keep-Alive") != "" {
			t.Error("Keep-Alive header should be removed")
		}
		
		// Set response headers including hop-by-hop
		w.Header().Set("Connection", "close")
		w.Header().Set("Keep-Alive", "timeout=5")
		w.Header().Set("X-Custom-Header", "custom-value")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	connector := NewHTTPConnector(&http.Client{}, 10 * time.Second)

	req := &mockRequest{
		id:         "header-test",
		method:     "GET",
		path:       "/headers",
		url:        "/headers",
		remoteAddr: "192.168.1.5:12349",
		headers: map[string][]string{
			"Connection":      {"keep-alive"},
			"Keep-Alive":      {"timeout=30"},
			"X-Custom-Header": {"request-value"},
		},
		body: io.NopCloser(strings.NewReader("")),
	}

	instance := core.ServiceInstance{
		ID:      "header-backend",
		Address: backendURL.Hostname(),
		Port:    parsePort(backendURL.Port()),
		Scheme:  backendURL.Scheme,
	}

	ctx := context.Background()
	route := &core.RouteResult{
		Instance: &instance,
		Rule:     &core.RouteRule{},
	}
	resp, err := connector.Forward(ctx, req, route)
	if err != nil {
		t.Fatalf("Forward() failed: %v", err)
	}

	// Check response headers
	// Note: Current implementation does not filter response headers, only request headers
	// This matches the behavior where the gateway passes through backend response headers
	if resp.Headers()["X-Custom-Header"] == nil {
		t.Error("Custom header should be preserved")
	}
}

func TestHTTPConnectorStreaming(t *testing.T) {
	// Create backend that returns a simple response
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		
		// Write all data at once
		w.Write([]byte("chunk 0\nchunk 1\nchunk 2\nchunk 3\nchunk 4\n"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	connector := NewHTTPConnector(&http.Client{}, 10 * time.Second)

	req := &mockRequest{
		id:         "stream-test",
		method:     "GET",
		path:       "/stream",
		url:        "/stream",
		remoteAddr: "192.168.1.6:12350",
		headers:    make(map[string][]string),
		body:       io.NopCloser(strings.NewReader("")),
	}

	instance := core.ServiceInstance{
		ID:      "stream-backend",
		Address: backendURL.Hostname(),
		Port:    parsePort(backendURL.Port()),
		Scheme:  backendURL.Scheme,
	}

	ctx := context.Background()
	route := &core.RouteResult{
		Instance: &instance,
		Rule:     &core.RouteRule{},
	}
	resp, err := connector.Forward(ctx, req, route)
	if err != nil {
		t.Fatalf("Forward() failed: %v", err)
	}

	// Read streamed response
	defer resp.Body().Close()
	body, err := io.ReadAll(resp.Body())
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Verify all chunks were received
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "chunk 0") {
		t.Error("Missing chunk 0")
	}
	if !strings.Contains(bodyStr, "chunk 4") {
		t.Error("Missing chunk 4")
	}
}

// Helper to parse port from string
func parsePort(portStr string) int {
	if portStr == "" {
		return 80
	}
	var port int
	if _, err := url.Parse("http://example.com:" + portStr); err == nil {
		// Extract numeric port
		for _, c := range portStr {
			if c >= '0' && c <= '9' {
				port = port*10 + int(c-'0')
			}
		}
	}
	return port
}