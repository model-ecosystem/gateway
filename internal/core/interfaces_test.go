package core_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"gateway/internal/core"
)

// Mock implementations for testing

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
func (m *mockRequest) Context() context.Context     { return m.ctx }

type mockResponse struct {
	statusCode int
	headers    map[string][]string
	body       io.ReadCloser
}

func (m *mockResponse) StatusCode() int              { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string { return m.headers }
func (m *mockResponse) Body() io.ReadCloser          { return m.body }

type mockService struct {
	name      string
	instances []core.ServiceInstance
}

func (m *mockService) GetInstances() []core.ServiceInstance { return m.instances }

type mockRegistry struct {
	services map[string][]core.ServiceInstance
}

func (m *mockRegistry) GetService(name string) ([]core.ServiceInstance, error) {
	if instances, ok := m.services[name]; ok {
		return instances, nil
	}
	return nil, nil
}

// Tests

func TestRequest(t *testing.T) {
	req := &mockRequest{
		id:         "test-123",
		method:     "GET",
		path:       "/api/test",
		url:        "https://example.com/api/test",
		remoteAddr: "192.168.1.100:12345",
		headers: map[string][]string{
			"Content-Type": {"application/json"},
			"User-Agent":   {"test-client/1.0"},
		},
		body: io.NopCloser(strings.NewReader("test body")),
		ctx:  context.Background(),
	}

	// Test all methods
	if req.ID() != "test-123" {
		t.Errorf("Expected ID test-123, got %s", req.ID())
	}

	if req.Method() != "GET" {
		t.Errorf("Expected method GET, got %s", req.Method())
	}

	if req.Path() != "/api/test" {
		t.Errorf("Expected path /api/test, got %s", req.Path())
	}

	if req.URL() != "https://example.com/api/test" {
		t.Errorf("Expected URL https://example.com/api/test, got %s", req.URL())
	}

	if req.RemoteAddr() != "192.168.1.100:12345" {
		t.Errorf("Expected RemoteAddr 192.168.1.100:12345, got %s", req.RemoteAddr())
	}

	headers := req.Headers()
	if len(headers) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(headers))
	}

	if req.Body() == nil {
		t.Error("Expected non-nil body")
	}

	if req.Context() == nil {
		t.Error("Expected non-nil context")
	}
}

func TestResponse(t *testing.T) {
	resp := &mockResponse{
		statusCode: 200,
		headers: map[string][]string{
			"Content-Type":   {"application/json"},
			"X-Custom-Header": {"value1", "value2"},
		},
		body: io.NopCloser(strings.NewReader("response body")),
	}

	if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}

	headers := resp.Headers()
	if len(headers) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(headers))
	}

	if resp.Body() == nil {
		t.Error("Expected non-nil body")
	}
}

func TestServiceInstance(t *testing.T) {
	instance := &core.ServiceInstance{
		ID:       "instance-1",
		Name:     "test-service",
		Address:  "192.168.1.10",
		Port:     8080,
		Scheme:   "http",
		Healthy:  true,
		Metadata: map[string]any{"zone": "us-east-1a"},
	}

	if instance.ID != "instance-1" {
		t.Errorf("Expected ID instance-1, got %s", instance.ID)
	}

	if instance.Name != "test-service" {
		t.Errorf("Expected name test-service, got %s", instance.Name)
	}

	if instance.Address != "192.168.1.10" {
		t.Errorf("Expected address 192.168.1.10, got %s", instance.Address)
	}

	if instance.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", instance.Port)
	}

	if instance.Scheme != "http" {
		t.Errorf("Expected scheme http, got %s", instance.Scheme)
	}

	if !instance.Healthy {
		t.Error("Expected instance to be healthy")
	}

	if instance.Metadata["zone"] != "us-east-1a" {
		t.Errorf("Expected zone us-east-1a, got %v", instance.Metadata["zone"])
	}
}

func TestService(t *testing.T) {
	service := &core.Service{
		Name: "test-service",
		Instances: []*core.ServiceInstance{
			{ID: "inst-1", Address: "10.0.0.1", Port: 8080, Healthy: true},
			{ID: "inst-2", Address: "10.0.0.2", Port: 8080, Healthy: true},
		},
		Metadata: map[string]string{"version": "v1"},
	}

	if service.Name != "test-service" {
		t.Errorf("Expected name test-service, got %s", service.Name)
	}

	if len(service.Instances) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(service.Instances))
	}

	if service.Metadata["version"] != "v1" {
		t.Errorf("Expected version v1, got %s", service.Metadata["version"])
	}
}


func TestRegistry(t *testing.T) {
	instances1 := []core.ServiceInstance{
		{ID: "inst-1", Address: "10.0.0.1", Port: 8080, Healthy: true},
	}

	instances2 := []core.ServiceInstance{
		{ID: "inst-2", Address: "10.0.0.2", Port: 8080, Healthy: true},
	}

	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"service-1": instances1,
			"service-2": instances2,
		},
	}

	// Test GetService
	instances, err := registry.GetService("service-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(instances) != 1 {
		t.Errorf("Expected 1 instance, got %d", len(instances))
	}

	// Test GetService not found
	instances, err = registry.GetService("service-3")
	if err != nil {
		t.Errorf("Expected no error for missing service, got %v", err)
	}
	if instances != nil {
		t.Error("Expected nil instances for missing service")
	}
}


func TestRouteRule(t *testing.T) {
	rule := &core.RouteRule{
		ID:          "test-route",
		Path:        "/api/*",
		ServiceName: "backend-service",
		Methods:     []string{"GET", "POST"},
		Timeout:     30 * time.Second,
			LoadBalance: "round_robin",
		Metadata: map[string]interface{}{
			"version": "v1",
			"owner":   "team-a",
		},
	}

	if rule.ID != "test-route" {
		t.Errorf("Expected ID test-route, got %s", rule.ID)
	}

	if rule.Path != "/api/*" {
		t.Errorf("Expected path /api/*, got %s", rule.Path)
	}

	if rule.ServiceName != "backend-service" {
		t.Errorf("Expected service backend-service, got %s", rule.ServiceName)
	}

	if len(rule.Methods) != 2 {
		t.Errorf("Expected 2 methods, got %d", len(rule.Methods))
	}

	if rule.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", rule.Timeout)
	}

	if rule.LoadBalance != core.LoadBalanceRoundRobin {
		t.Errorf("Expected load balance round_robin, got %s", rule.LoadBalance)
	}

	if rule.Metadata["version"] != "v1" {
		t.Errorf("Expected version v1, got %v", rule.Metadata["version"])
	}
}

func TestRouteResult(t *testing.T) {
	rule := &core.RouteRule{
		ID:          "test-route",
		ServiceName: "backend-service",
	}

	instance := &core.ServiceInstance{
		ID:      "inst-1",
		Address: "10.0.0.1",
		Port:    8080,
	}

	result := &core.RouteResult{
		Instance:    instance,
		Rule:        rule,
		ServiceName: "backend-service",
	}

	if result.Rule.ID != "test-route" {
		t.Errorf("Expected rule ID test-route, got %s", result.Rule.ID)
	}

	if result.ServiceName != "backend-service" {
		t.Errorf("Expected service backend-service, got %s", result.ServiceName)
	}

	if result.Instance.ID != "inst-1" {
		t.Errorf("Expected instance ID inst-1, got %s", result.Instance.ID)
	}
}

func TestHandler(t *testing.T) {
	// Test handler function
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{
			statusCode: 200,
			headers:    map[string][]string{"X-Handler": {"test"}},
			body:       io.NopCloser(strings.NewReader("OK")),
		}, nil
	}

	req := &mockRequest{
		method: "GET",
		path:   "/test",
		ctx:    context.Background(),
	}

	resp, err := handler(req.Context(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}

	if resp.Headers()["X-Handler"][0] != "test" {
		t.Errorf("Expected header X-Handler=test, got %v", resp.Headers()["X-Handler"])
	}
}

func TestMiddleware(t *testing.T) {
	// Test middleware that adds a header
	middleware := func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}
			
			// Simulate adding a header
			if mockResp, ok := resp.(*mockResponse); ok {
				if mockResp.headers == nil {
					mockResp.headers = make(map[string][]string)
				}
				mockResp.headers["X-Middleware"] = []string{"applied"}
			}
			
			return resp, nil
		}
	}

	// Base handler
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{
			statusCode: 200,
			headers:    make(map[string][]string),
		}, nil
	}

	// Apply middleware
	wrapped := middleware(handler)

	req := &mockRequest{
		method: "GET",
		path:   "/test",
		ctx:    context.Background(),
	}

	resp, err := wrapped(req.Context(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.Headers()["X-Middleware"][0] != "applied" {
		t.Error("Expected middleware to add header")
	}
}