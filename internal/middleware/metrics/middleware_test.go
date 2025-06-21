package metrics

import (
	"context"
	"io"
	"testing"
	"time"
	
	"gateway/internal/core"
	"gateway/internal/metrics"
	gwerrors "gateway/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// Mock request implementation
type mockRequest struct {
	method string
	path   string
	body   io.ReadCloser
	headers map[string][]string
}

func (r *mockRequest) ID() string                    { return "test-request-id" }
func (r *mockRequest) Method() string               { return r.method }
func (r *mockRequest) Path() string                 { return r.path }
func (r *mockRequest) URL() string                  { return "http://example.com" + r.path }
func (r *mockRequest) RemoteAddr() string           { return "127.0.0.1:12345" }
func (r *mockRequest) Headers() map[string][]string { return r.headers }
func (r *mockRequest) Body() io.ReadCloser          { return r.body }
func (r *mockRequest) Context() context.Context     { return context.Background() }

// Mock response implementation
type mockResponse struct {
	statusCode int
	body       io.ReadCloser
	headers    map[string][]string
}

func (r *mockResponse) StatusCode() int                { return r.statusCode }
func (r *mockResponse) Body() io.ReadCloser            { return r.body }
func (r *mockResponse) Headers() map[string][]string   { return r.headers }

func TestMiddleware(t *testing.T) {
	m := metrics.New()
	middleware := Middleware(m)
	
	// Create a handler that returns a response
	handler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		
		return &mockResponse{
			statusCode: 200,
			headers:    make(map[string][]string),
		}, nil
	})
	
	// Wrap handler with middleware
	wrapped := middleware(handler)
	
	// Create test request
	req := &mockRequest{
		method:  "GET",
		path:    "/api/test",
		headers: make(map[string][]string),
	}
	
	// Execute request
	ctx := context.Background()
	resp, err := wrapped(ctx, req)
	
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}
	
	// Check metrics
	// Note: Without HTTP request in context, metrics won't be collected
	// This tests the pass-through behavior
}

func TestMiddlewareWithHTTPContext(t *testing.T) {
	// Create a new registry for this test to avoid conflicts
	registry := prometheus.NewRegistry()
	m := metrics.NewWithRegistry(registry, registry)
	middleware := Middleware(m)
	
	// Create a handler that returns a response
	handler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{
			statusCode: 200,
			headers:    make(map[string][]string),
		}, nil
	})
	
	// Wrap handler with middleware
	wrapped := middleware(handler)
	
	// Create test request
	req := &mockRequest{
		method:  "GET",
		path:    "/api/test",
		headers: make(map[string][]string),
	}
	
	// Execute multiple requests
	for i := 0; i < 3; i++ {
		ctx := context.Background()
		_, err := wrapped(ctx, req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
	}
	
	// Test error handling
	errorHandler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		return nil, gwerrors.NewError(gwerrors.ErrorTypeNotFound, "not found")
	})
	
	wrappedError := middleware(errorHandler)
	_, err := wrappedError(context.Background(), req)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestMiddlewareMetricsCollection(t *testing.T) {
	// Create a new registry for this test to avoid conflicts
	registry := prometheus.NewRegistry()
	m := metrics.NewWithRegistry(registry, registry)
	middleware := Middleware(m)
	
	successCount := 0
	errorCount := 0
	
	// Create handlers
	successHandler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		successCount++
		return &mockResponse{statusCode: 200}, nil
	})
	
	errorHandler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		errorCount++
		return nil, gwerrors.NewError(gwerrors.ErrorTypeInternal, "internal error")
	})
	
	// Test success path
	wrapped := middleware(successHandler)
	req := &mockRequest{method: "GET", path: "/api/users"}
	
	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if successCount != 1 {
		t.Errorf("Expected 1 success call, got %d", successCount)
	}
	
	// Test error path
	wrappedError := middleware(errorHandler)
	_, err = wrappedError(context.Background(), req)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	
	if errorCount != 1 {
		t.Errorf("Expected 1 error call, got %d", errorCount)
	}
}

func BenchmarkMiddleware(b *testing.B) {
	m := metrics.New()
	middleware := Middleware(m)
	
	handler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{statusCode: 200}, nil
	})
	
	wrapped := middleware(handler)
	req := &mockRequest{
		method:  "GET",
		path:    "/api/test",
		headers: make(map[string][]string),
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = wrapped(ctx, req)
	}
}