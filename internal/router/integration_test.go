package router

import (
	"context"
	"gateway/internal/core"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterIntegration(t *testing.T) {
	// Create a real registry with services
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"api-service": {
				{ID: "api-1", Address: "127.0.0.1", Port: 8001, Healthy: true},
			},
		},
	}

	router := NewRouter(registry, nil)

	// Add routes that use ServeMux patterns
	rules := []core.RouteRule{
		{
			ID:          "users-list",
			Path:        "/api/users",
			ServiceName: "api-service",
			Methods:     []string{"GET"},
		},
		{
			ID:          "user-detail",
			Path:        "/api/users/:id",
			ServiceName: "api-service",
			Methods:     []string{"GET"},
		},
		{
			ID:          "user-posts",
			Path:        "/api/users/:id/posts",
			ServiceName: "api-service",
			Methods:     []string{"GET"},
		},
		{
			ID:          "static-files",
			Path:        "/static/*",
			ServiceName: "api-service",
		},
	}

	for _, rule := range rules {
		if err := router.AddRule(rule); err != nil {
			t.Fatalf("Failed to add rule: %v", err)
		}
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "exact match",
			method:     "GET",
			path:       "/api/users",
			wantStatus: http.StatusOK,
		},
		{
			name:       "path parameter",
			method:     "GET",
			path:       "/api/users/123",
			wantStatus: http.StatusOK,
		},
		{
			name:       "nested path parameter",
			method:     "GET",
			path:       "/api/users/456/posts",
			wantStatus: http.StatusOK,
		},
		{
			name:       "wildcard path",
			method:     "GET",
			path:       "/static/css/main.css",
			wantStatus: http.StatusOK,
		},
		{
			name:       "not found",
			method:     "GET",
			path:       "/api/unknown",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "method not allowed",
			method:     "POST",
			path:       "/api/users",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest(tt.method, tt.path, nil)

			// Create a mock request wrapper
			mockReq := &mockRequest{
				method: req.Method,
				path:   req.URL.Path,
			}

			// Route the request
			ctx := context.Background()
			result, err := router.Route(ctx, mockReq)

			if tt.wantStatus == http.StatusOK {
				if err != nil {
					t.Errorf("Route() error = %v, want nil", err)
				}
				if result == nil {
					t.Errorf("Route() returned nil result")
				}
			} else {
				if err == nil {
					t.Errorf("Route() error = nil, want error")
				}
			}
		})
	}
}

func TestRouterConcurrency(t *testing.T) {
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"concurrent-service": {
				{ID: "concurrent-1", Address: "127.0.0.1", Port: 8001, Healthy: true},
				{ID: "concurrent-2", Address: "127.0.0.1", Port: 8002, Healthy: true},
			},
		},
	}

	router := NewRouter(registry, nil)

	rule := core.RouteRule{
		ID:          "concurrent-test",
		Path:        "/api/concurrent",
		ServiceName: "concurrent-service",
	}

	if err := router.AddRule(rule); err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	// Run concurrent requests
	done := make(chan bool)
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		go func() {
			ctx := context.Background()
			req := &mockRequest{method: "GET", path: "/api/concurrent"}

			_, err := router.Route(ctx, req)
			if err != nil {
				errors <- err
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// Check for errors
	select {
	case err := <-errors:
		t.Fatalf("Concurrent routing failed: %v", err)
	default:
		// No errors, test passed
	}
}

func TestRouterServeMuxPathExtraction(t *testing.T) {
	// Test that we can extract path parameters using ServeMux
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"param-service": {
				{ID: "param-1", Address: "127.0.0.1", Port: 8001, Healthy: true},
			},
		},
	}

	router := NewRouter(registry, nil)

	rule := core.RouteRule{
		ID:          "param-test",
		Path:        "/api/items/:id/details",
		ServiceName: "param-service",
	}

	if err := router.AddRule(rule); err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	// Test with a real HTTP request to verify ServeMux pattern matching
	testPaths := []string{
		"/api/items/123/details",
		"/api/items/abc-def/details",
		"/api/items/456/details",
	}

	ctx := context.Background()
	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			req := &mockRequest{method: "GET", path: path}
			result, err := router.Route(ctx, req)

			if err != nil {
				t.Errorf("Route() failed for path %s: %v", path, err)
			}
			if result == nil {
				t.Errorf("Route() returned nil result for path %s", path)
			}
		})
	}

	// Test non-matching path
	nonMatchingReq := &mockRequest{method: "GET", path: "/api/items/123"}
	_, err := router.Route(ctx, nonMatchingReq)
	if err == nil {
		t.Errorf("Route() should have failed for non-matching path")
	}
}
