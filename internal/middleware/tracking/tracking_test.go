package tracking

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"gateway/internal/core"
	"gateway/internal/router"
)

// Mock implementations
type mockRequest struct {
	id         string
	method     string
	path       string
	url        string
	remoteAddr string
	headers    map[string][]string
	body       string
	ctx        context.Context
}

func (r *mockRequest) ID() string                    { return r.id }
func (r *mockRequest) Method() string                { return r.method }
func (r *mockRequest) Path() string                  { return r.path }
func (r *mockRequest) URL() string                   { return r.url }
func (r *mockRequest) RemoteAddr() string            { return r.remoteAddr }
func (r *mockRequest) Headers() map[string][]string  { return r.headers }
func (r *mockRequest) Body() io.ReadCloser {
	return io.NopCloser(strings.NewReader(r.body))
}
func (r *mockRequest) Context() context.Context {
	if r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

type mockResponse struct {
	statusCode int
	headers    map[string]string
	body       string
}

func (r *mockResponse) StatusCode() int                        { return r.statusCode }
func (r *mockResponse) Header(key string) string               { return r.headers[key] }
func (r *mockResponse) Headers() map[string][]string {
	headers := make(map[string][]string)
	for k, v := range r.headers {
		headers[k] = []string{v}
	}
	return headers
}
func (r *mockResponse) Body() io.ReadCloser {
	return io.NopCloser(strings.NewReader(r.body))
}
func (r *mockResponse) WithStatusCode(code int) core.Response {
	return &mockResponse{
		statusCode: code,
		headers:    r.headers,
		body:       r.body,
	}
}
func (r *mockResponse) WithHeader(key, value string) core.Response {
	newHeaders := make(map[string]string)
	for k, v := range r.headers {
		newHeaders[k] = v
	}
	newHeaders[key] = value
	return &mockResponse{
		statusCode: r.statusCode,
		headers:    newHeaders,
		body:       r.body,
	}
}
func (r *mockResponse) WithBody(body io.ReadCloser) core.Response {
	data, _ := io.ReadAll(body)
	return &mockResponse{
		statusCode: r.statusCode,
		headers:    r.headers,
		body:       string(data),
	}
}

func TestNewMiddleware(t *testing.T) {
	m := NewMiddleware()
	if m == nil {
		t.Fatal("Expected non-nil middleware")
	}
	if m.connectionTracker == nil {
		t.Error("Expected initialized connection tracker map")
	}
	if m.responseTime == nil {
		t.Error("Expected initialized response time map")
	}
}

func TestMiddleware_WrapHandler_NoRouteContext(t *testing.T) {
	m := NewMiddleware()
	
	handlerCalled := false
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		handlerCalled = true
		return &mockResponse{statusCode: 200}, nil
	}
	
	wrapped := m.WrapHandler("test", handler)
	
	// Call without route context
	ctx := context.Background()
	req := &mockRequest{}
	
	resp, err := wrapped(ctx, req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("Expected handler to be called")
	}
	if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}
}

func TestMiddleware_WrapHandler_WithLeastConnections(t *testing.T) {
	m := NewMiddleware()
	
	// Create a least connections balancer
	balancer := router.NewLeastConnectionsBalancer()
	instance := &core.ServiceInstance{ID: "instance-1", Address: "localhost", Port: 8080}
	
	routeResult := &core.RouteResult{
		Rule: &core.RouteRule{
			ID:          "route-1",
			LoadBalance: core.LoadBalanceLeastConnections,
			Balancer:    balancer,
		},
		Instance: instance,
	}
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		return &mockResponse{statusCode: 200}, nil
	}
	
	wrapped := m.WrapHandler("test", handler)
	
	// Create context with route result
	ctx := context.WithValue(context.Background(), routeContextKey{}, routeResult)
	req := &mockRequest{}
	
	// First request should track connection
	resp, err := wrapped(ctx, req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}
	
	// Verify connection tracker was created
	if _, exists := m.connectionTracker["route-1"]; !exists {
		t.Error("Expected connection tracker to be created for route")
	}
}

func TestMiddleware_WrapHandler_WithResponseTime(t *testing.T) {
	m := NewMiddleware()
	
	// Create a response time balancer
	balancer := router.NewResponseTimeBalancer()
	instance := &core.ServiceInstance{ID: "instance-1", Address: "localhost", Port: 8080}
	
	routeResult := &core.RouteResult{
		Rule: &core.RouteRule{
			ID:          "route-1",
			LoadBalance: core.LoadBalanceResponseTime,
			Balancer:    balancer,
		},
		Instance: instance,
	}
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		return &mockResponse{statusCode: 200}, nil
	}
	
	wrapped := m.WrapHandler("test", handler)
	
	// Create context with route result
	ctx := context.WithValue(context.Background(), routeContextKey{}, routeResult)
	req := &mockRequest{}
	
	// Execute request
	resp, err := wrapped(ctx, req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}
	
	// The response time should have been recorded in the balancer
	// (the balancer itself tracks this internally)
}

func TestMiddleware_WrapHandler_WithAdaptiveBalancer(t *testing.T) {
	m := NewMiddleware()
	
	// Create an adaptive balancer
	balancer := router.NewAdaptiveBalancer()
	instance := &core.ServiceInstance{ID: "instance-1", Address: "localhost", Port: 8080}
	
	routeResult := &core.RouteResult{
		Rule: &core.RouteRule{
			ID:          "route-1",
			LoadBalance: core.LoadBalanceStrategy("adaptive"),
			Balancer:    balancer,
		},
		Instance: instance,
	}
	
	tests := []struct {
		name         string
		handlerResp  *mockResponse
		handlerErr   error
		expectSuccess bool
	}{
		{
			name:          "successful request",
			handlerResp:   &mockResponse{statusCode: 200},
			expectSuccess: true,
		},
		{
			name:          "client error",
			handlerResp:   &mockResponse{statusCode: 400},
			expectSuccess: true, // 4xx is not a server error
		},
		{
			name:          "server error",
			handlerResp:   &mockResponse{statusCode: 500},
			expectSuccess: false,
		},
		{
			name:          "handler error",
			handlerErr:    errors.New("connection failed"),
			expectSuccess: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := func(ctx context.Context, req core.Request) (core.Response, error) {
				time.Sleep(5 * time.Millisecond)
				return tt.handlerResp, tt.handlerErr
			}
			
			wrapped := m.WrapHandler("test", handler)
			ctx := context.WithValue(context.Background(), routeContextKey{}, routeResult)
			req := &mockRequest{}
			
			resp, err := wrapped(ctx, req)
			
			if tt.handlerErr != nil {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp.StatusCode() != tt.handlerResp.statusCode {
					t.Errorf("Expected status %d, got %d", tt.handlerResp.statusCode, resp.StatusCode())
				}
			}
		})
	}
}

func TestMiddleware_getBalancerType(t *testing.T) {
	m := NewMiddleware()
	
	tests := []struct {
		strategy core.LoadBalanceStrategy
		expected string
	}{
		{core.LoadBalanceRoundRobin, "round_robin"},
		{core.LoadBalanceLeastConnections, "least_connections"},
		{core.LoadBalanceResponseTime, "response_time"},
		{core.LoadBalanceStrategy("custom"), "custom"},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			result := m.getBalancerType(tt.strategy)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMiddleware_MultipleRoutes(t *testing.T) {
	m := NewMiddleware()
	
	// Create different balancers for different routes
	lcBalancer1 := router.NewLeastConnectionsBalancer()
	lcBalancer2 := router.NewLeastConnectionsBalancer()
	
	// Test that different routes get different trackers
	routes := []struct {
		routeID  string
		balancer core.LoadBalancer
	}{
		{"route-1", lcBalancer1},
		{"route-2", lcBalancer2},
	}
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{statusCode: 200}, nil
	}
	
	wrapped := m.WrapHandler("test", handler)
	
	for _, route := range routes {
		routeResult := &core.RouteResult{
			Rule: &core.RouteRule{
				ID:          route.routeID,
				LoadBalance: core.LoadBalanceLeastConnections,
				Balancer:    route.balancer,
			},
			Instance: &core.ServiceInstance{ID: "instance-1"},
		}
		
		ctx := context.WithValue(context.Background(), routeContextKey{}, routeResult)
		req := &mockRequest{}
		
		_, err := wrapped(ctx, req)
		if err != nil {
			t.Errorf("Unexpected error for route %s: %v", route.routeID, err)
		}
		
		// Verify separate tracker for each route
		if _, exists := m.connectionTracker[route.routeID]; !exists {
			t.Errorf("Expected connection tracker for route %s", route.routeID)
		}
	}
	
	// Verify we have trackers for both routes
	if len(m.connectionTracker) != 2 {
		t.Errorf("Expected 2 connection trackers, got %d", len(m.connectionTracker))
	}
}

func TestMiddleware_ConcurrentAccess(t *testing.T) {
	m := NewMiddleware()
	
	balancer := router.NewLeastConnectionsBalancer()
	routeResult := &core.RouteResult{
		Rule: &core.RouteRule{
			ID:          "route-1",
			LoadBalance: core.LoadBalanceLeastConnections,
			Balancer:    balancer,
		},
		Instance: &core.ServiceInstance{ID: "instance-1"},
	}
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		time.Sleep(time.Millisecond) // Simulate work
		return &mockResponse{statusCode: 200}, nil
	}
	
	wrapped := m.WrapHandler("test", handler)
	ctx := context.WithValue(context.Background(), routeContextKey{}, routeResult)
	
	// Run multiple concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := &mockRequest{}
			_, err := wrapped(ctx, req)
			if err != nil {
				t.Errorf("Concurrent request failed: %v", err)
			}
			done <- true
		}()
	}
	
	// Wait for all requests to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}