package router

import (
	"context"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"io"
	"testing"
)

// mockRegistry for testing
type mockRegistry struct {
	services map[string][]core.ServiceInstance
}

func (m *mockRegistry) RegisterService(name string, instance core.ServiceInstance) error {
	if m.services == nil {
		m.services = make(map[string][]core.ServiceInstance)
	}
	m.services[name] = append(m.services[name], instance)
	return nil
}

func (m *mockRegistry) UnregisterService(name string, instanceID string) error {
	if instances, ok := m.services[name]; ok {
		var filtered []core.ServiceInstance
		for _, inst := range instances {
			if inst.ID != instanceID {
				filtered = append(filtered, inst)
			}
		}
		m.services[name] = filtered
	}
	return nil
}

func (m *mockRegistry) GetService(name string) ([]core.ServiceInstance, error) {
	instances, ok := m.services[name]
	if !ok {
		return nil, errors.NewError(errors.ErrorTypeNotFound, "service not found")
	}
	// Filter healthy instances
	var healthy []core.ServiceInstance
	for _, inst := range instances {
		if inst.Healthy {
			healthy = append(healthy, inst)
		}
	}
	return healthy, nil
}

func (m *mockRegistry) ListServices() ([]string, error) {
	var names []string
	for name := range m.services {
		names = append(names, name)
	}
	return names, nil
}

// mockRequest for testing
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

func TestRouterAddRule(t *testing.T) {
	registry := &mockRegistry{
		services: make(map[string][]core.ServiceInstance),
	}
	router := NewRouter(registry)

	tests := []struct {
		name    string
		rule    core.RouteRule
		wantErr bool
	}{
		{
			name: "simple path",
			rule: core.RouteRule{
				ID:          "test-1",
				Path:        "/api/users",
				ServiceName: "user-service",
				Methods:     []string{"GET", "POST"},
			},
			wantErr: false,
		},
		{
			name: "path with parameter",
			rule: core.RouteRule{
				ID:          "test-2",
				Path:        "/api/users/:id",
				ServiceName: "user-service",
				Methods:     []string{"GET"},
			},
			wantErr: false,
		},
		{
			name: "wildcard path",
			rule: core.RouteRule{
				ID:          "test-3",
				Path:        "/api/files/*",
				ServiceName: "file-service",
			},
			wantErr: false,
		},
		{
			name: "duplicate id",
			rule: core.RouteRule{
				ID:          "test-1", // duplicate
				Path:        "/api/products",
				ServiceName: "product-service",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := router.AddRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRouterRoute(t *testing.T) {
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"user-service": {
				{ID: "user-1", Address: "127.0.0.1", Port: 8001, Healthy: true},
				{ID: "user-2", Address: "127.0.0.1", Port: 8002, Healthy: true},
			},
			"file-service": {
				{ID: "file-1", Address: "127.0.0.1", Port: 8003, Healthy: true},
			},
			"unhealthy-service": {
				{ID: "unhealthy-1", Address: "127.0.0.1", Port: 8004, Healthy: false},
			},
		},
	}

	router := NewRouter(registry)
	
	// Add rules
	rules := []core.RouteRule{
		{
			ID:          "users-get",
			Path:        "/api/users",
			ServiceName: "user-service",
			Methods:     []string{"GET"},
		},
		{
			ID:          "users-post",
			Path:        "/api/users",
			ServiceName: "user-service",
			Methods:     []string{"POST"},
		},
		{
			ID:          "user-by-id",
			Path:        "/api/users/:id",
			ServiceName: "user-service",
			Methods:     []string{"GET", "PUT", "DELETE"},
		},
		{
			ID:          "files-wildcard",
			Path:        "/api/files/*",
			ServiceName: "file-service",
		},
		{
			ID:          "unhealthy",
			Path:        "/api/unhealthy",
			ServiceName: "unhealthy-service",
		},
	}

	for _, rule := range rules {
		if err := router.AddRule(rule); err != nil {
			t.Fatalf("Failed to add rule: %v", err)
		}
	}

	tests := []struct {
		name     string
		request  core.Request
		wantErr  bool
		errorType errors.ErrorType
		service  string
	}{
		{
			name:     "exact match GET",
			request:  &mockRequest{method: "GET", path: "/api/users"},
			wantErr:  false,
			service:  "user-service",
		},
		{
			name:     "exact match POST",
			request:  &mockRequest{method: "POST", path: "/api/users"},
			wantErr:  false,
			service:  "user-service",
		},
		{
			name:     "method not allowed",
			request:  &mockRequest{method: "DELETE", path: "/api/users"},
			wantErr:  true,
			errorType: errors.ErrorTypeNotFound,
		},
		{
			name:     "path parameter",
			request:  &mockRequest{method: "GET", path: "/api/users/123"},
			wantErr:  false,
			service:  "user-service",
		},
		{
			name:     "wildcard match",
			request:  &mockRequest{method: "GET", path: "/api/files/documents/report.pdf"},
			wantErr:  false,
			service:  "file-service",
		},
		{
			name:     "route not found",
			request:  &mockRequest{method: "GET", path: "/api/unknown"},
			wantErr:  true,
			errorType: errors.ErrorTypeNotFound,
		},
		{
			name:     "no healthy instances",
			request:  &mockRequest{method: "GET", path: "/api/unhealthy"},
			wantErr:  true,
			errorType: errors.ErrorTypeUnavailable,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := router.Route(ctx, tt.request)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Route() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if gerr, ok := err.(*errors.Error); ok {
					if gerr.Type != tt.errorType {
						t.Errorf("Route() error type = %v, want %v", gerr.Type, tt.errorType)
					}
				}
				return
			}

			if !tt.wantErr && result != nil {
				// Verify we got an instance from the expected service
				instances, _ := registry.GetService(tt.service)
				found := false
				for _, inst := range instances {
					if inst.ID == result.Instance.ID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Route() returned instance from wrong service")
				}
			}
		})
	}
}

func TestRouterLoadBalancing(t *testing.T) {
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"lb-service": {
				{ID: "lb-1", Address: "127.0.0.1", Port: 8001, Healthy: true},
				{ID: "lb-2", Address: "127.0.0.1", Port: 8002, Healthy: true},
				{ID: "lb-3", Address: "127.0.0.1", Port: 8003, Healthy: true},
			},
		},
	}

	router := NewRouter(registry)
	
	rule := core.RouteRule{
		ID:          "lb-test",
		Path:        "/api/lb",
		ServiceName: "lb-service",
		LoadBalance: core.LoadBalanceRoundRobin,
	}
	
	if err := router.AddRule(rule); err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	// Track which instances we get
	instanceCounts := make(map[string]int)
	ctx := context.Background()
	req := &mockRequest{method: "GET", path: "/api/lb"}

	// Make multiple requests
	for i := 0; i < 9; i++ {
		result, err := router.Route(ctx, req)
		if err != nil {
			t.Fatalf("Route() failed: %v", err)
		}
		instanceCounts[result.Instance.ID]++
	}

	// With round-robin, each instance should get 3 requests
	for id, count := range instanceCounts {
		if count != 3 {
			t.Errorf("Instance %s got %d requests, expected 3", id, count)
		}
	}
}

func TestRouterMethodMatching(t *testing.T) {
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"method-service": {
				{ID: "method-1", Address: "127.0.0.1", Port: 8001, Healthy: true},
			},
		},
	}

	router := NewRouter(registry)
	
	// Add rule with no methods (should match all)
	rule := core.RouteRule{
		ID:          "all-methods",
		Path:        "/api/all",
		ServiceName: "method-service",
		// Methods is empty, should match all
	}
	
	if err := router.AddRule(rule); err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	// Test various HTTP methods
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	ctx := context.Background()

	for _, method := range methods {
		t.Run("method_"+method, func(t *testing.T) {
			req := &mockRequest{method: method, path: "/api/all"}
			result, err := router.Route(ctx, req)
			
			if err != nil {
				t.Errorf("Route() failed for method %s: %v", method, err)
			}
			if result == nil || result.Instance.ID != "method-1" {
				t.Errorf("Route() did not return expected instance for method %s", method)
			}
		})
	}
}

func TestRouterPathConversion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/users", "/api/users"},
		{"/api/users/:id", "/api/users/{id}"},
		{"/api/users/:id/posts/:postId", "/api/users/{id}/posts/{postId}"},
		{"/api/files/*", "/api/files/{path...}"},
		{"/api/static/*", "/api/static/{path...}"},
		{"/api/:version/users", "/api/{version}/users"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Create a new router for each test to avoid conflicts
			router := NewRouter(nil)
			
			rule := core.RouteRule{
				ID:          "test",
				Path:        tt.input,
				ServiceName: "test-service",
				// No methods specified, so it will register without method prefix
			}
			
			if err := router.AddRule(rule); err != nil {
				t.Fatalf("Failed to add rule: %v", err)
			}
			
			// Check if the pattern was registered correctly
			// Since no methods are specified, it should be registered without method prefix
			found := false
			for pattern := range router.routes {
				if pattern == tt.expected {
					found = true
					break
				}
			}
			
			if !found {
				t.Errorf("Expected pattern %s not found in routes (got: %v)", tt.expected, router.routes)
			}
		})
	}
}