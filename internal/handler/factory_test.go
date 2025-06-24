package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	grpcConnector "gateway/internal/connector/grpc"
	sseConnector "gateway/internal/connector/sse"
	wsConnector "gateway/internal/connector/websocket"
	"gateway/internal/core"
	gwerrors "gateway/pkg/errors"
	"gateway/pkg/factory"
)

// Mock implementations
type mockRouter struct {
	routeFn func(ctx context.Context, req core.Request) (*core.RouteResult, error)
}

func (m *mockRouter) Route(ctx context.Context, req core.Request) (*core.RouteResult, error) {
	if m.routeFn != nil {
		return m.routeFn(ctx, req)
	}
	return &core.RouteResult{ServiceName: "test-service"}, nil
}


type mockConnector struct {
	forwardFn func(ctx context.Context, req core.Request, route *core.RouteResult) (core.Response, error)
}

func (m *mockConnector) Forward(ctx context.Context, req core.Request, route *core.RouteResult) (core.Response, error) {
	if m.forwardFn != nil {
		return m.forwardFn(ctx, req, route)
	}
	return &mockResponse{statusCode: 200}, nil
}

type mockRequest struct {
	id      string
	method  string
	path    string
	url     string
	remote  string
	headers map[string][]string
	body    io.ReadCloser
}

func (m *mockRequest) ID() string                     { return m.id }
func (m *mockRequest) Method() string                 { return m.method }
func (m *mockRequest) Path() string                   { return m.path }
func (m *mockRequest) URL() string                    { 
	if m.url != "" {
		return m.url
	}
	return m.path 
}
func (m *mockRequest) RemoteAddr() string             { return m.remote }
func (m *mockRequest) Headers() map[string][]string   { return m.headers }
func (m *mockRequest) Body() io.ReadCloser           { 
	if m.body != nil {
		return m.body
	}
	return io.NopCloser(bytes.NewReader(nil))
}
func (m *mockRequest) Context() context.Context       { return context.Background() }

type mockResponse struct {
	statusCode int
	headers    map[string][]string
	body       []byte
}

func (m *mockResponse) StatusCode() int               { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string  { 
	if m.headers == nil {
		m.headers = make(map[string][]string)
	}
	return m.headers 
}
func (m *mockResponse) Body() io.ReadCloser          { 
	return io.NopCloser(bytes.NewReader(m.body))
}


func TestNewComponent(t *testing.T) {
	logger := slog.Default()
	component := NewComponent(logger)
	
	if component == nil {
		t.Fatal("expected non-nil component")
	}
	
	if component.Name() != ComponentName {
		t.Errorf("expected component name %s, got %s", ComponentName, component.Name())
	}
}

func TestComponent_Init(t *testing.T) {
	logger := slog.Default()
	component := NewComponent(logger).(*Component)
	
	parser := func(v any) error {
		return nil
	}
	err := component.Init(parser)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComponent_Validate(t *testing.T) {
	tests := []struct {
		name          string
		setupComp     func(*Component)
		expectError   bool
		errorContains string
	}{
		{
			name: "valid component",
			setupComp: func(c *Component) {
				c.router = &mockRouter{}
				c.httpConnector = &mockConnector{}
			},
			expectError: false,
		},
		{
			name:          "missing router",
			setupComp:     func(c *Component) {
				c.httpConnector = &mockConnector{}
			},
			expectError:   true,
			errorContains: "router not set",
		},
		{
			name:          "missing HTTP connector",
			setupComp:     func(c *Component) {
				c.router = &mockRouter{}
			},
			expectError:   true,
			errorContains: "HTTP connector not set",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			component := NewComponent(logger).(*Component)
			
			tt.setupComp(component)
			
			err := component.Validate()
			
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestComponent_SetDependencies(t *testing.T) {
	logger := slog.Default()
	component := NewComponent(logger).(*Component)
	
	router := &mockRouter{}
	httpConnector := &mockConnector{}
	grpcConn := &grpcConnector.Connector{}
	sseConn := &sseConnector.Connector{}
	wsConn := &wsConnector.Connector{}
	
	component.SetDependencies(router, httpConnector, grpcConn, sseConn, wsConn)
	
	if component.router != router {
		t.Error("router not set correctly")
	}
	if component.httpConnector != httpConnector {
		t.Error("HTTP connector not set correctly")
	}
	if component.grpcConnector != grpcConn {
		t.Error("gRPC connector not set correctly")
	}
	if component.sseConnector != sseConn {
		t.Error("SSE connector not set correctly")
	}
	if component.wsConnector != wsConn {
		t.Error("WebSocket connector not set correctly")
	}
}

func TestComponent_CreateBaseHandler(t *testing.T) {
	tests := []struct {
		name         string
		routeError   error
		forwardError error
		expectError  bool
	}{
		{
			name:        "successful request",
			expectError: false,
		},
		{
			name:        "routing error",
			routeError:  errors.New("route not found"),
			expectError: true,
		},
		{
			name:         "forward error",
			forwardError: errors.New("backend unavailable"),
			expectError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			component := NewComponent(logger).(*Component)
			
			router := &mockRouter{
				routeFn: func(ctx context.Context, req core.Request) (*core.RouteResult, error) {
					if tt.routeError != nil {
						return nil, tt.routeError
					}
					return &core.RouteResult{ServiceName: "test-service"}, nil
				},
			}
			
			httpConnector := &mockConnector{
				forwardFn: func(ctx context.Context, req core.Request, route *core.RouteResult) (core.Response, error) {
					if tt.forwardError != nil {
						return nil, tt.forwardError
					}
					return &mockResponse{statusCode: 200}, nil
				},
			}
			
			component.SetDependencies(router, httpConnector, nil, nil, nil)
			
			handler := component.CreateBaseHandler()
			req := &mockRequest{method: "GET", path: "/test"}
			resp, err := handler(context.Background(), req)
			
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("expected response, got nil")
				}
			}
		})
	}
}

func TestComponent_CreateMultiProtocolHandler(t *testing.T) {
	tests := []struct {
		name          string
		withRouteCtx  bool
		routeError    error
		forwardError  error
		expectError   bool
	}{
		{
			name:         "with route in context",
			withRouteCtx: true,
			expectError:  false,
		},
		{
			name:         "without route in context",
			withRouteCtx: false,
			expectError:  false,
		},
		{
			name:        "routing error",
			routeError:  errors.New("route not found"),
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			component := NewComponent(logger).(*Component)
			
			router := &mockRouter{
				routeFn: func(ctx context.Context, req core.Request) (*core.RouteResult, error) {
					if tt.routeError != nil {
						return nil, tt.routeError
					}
					return &core.RouteResult{ServiceName: "test-service"}, nil
				},
			}
			
			httpConnector := &mockConnector{
				forwardFn: func(ctx context.Context, req core.Request, route *core.RouteResult) (core.Response, error) {
					if tt.forwardError != nil {
						return nil, tt.forwardError
					}
					return &mockResponse{statusCode: 200}, nil
				},
			}
			
			component.SetDependencies(router, httpConnector, nil, nil, nil)
			
			handler := component.CreateMultiProtocolHandler()
			req := &mockRequest{method: "GET", path: "/test"}
			
			ctx := context.Background()
			if tt.withRouteCtx {
				route := &core.RouteResult{ServiceName: "context-service"}
				ctx = setRouteInContext(ctx, route)
			}
			
			resp, err := handler(ctx, req)
			
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("expected response, got nil")
				}
			}
		})
	}
}

func TestComponent_CreateSSEHandler(t *testing.T) {
	logger := slog.Default()
	component := NewComponent(logger).(*Component)
	
	router := &mockRouter{}
	httpConnector := &mockConnector{}
	component.SetDependencies(router, httpConnector, nil, nil, nil)
	
	handler := component.CreateSSEHandler()
	req := &mockRequest{method: "GET", path: "/sse"}
	
	_, err := handler(context.Background(), req)
	
	if err == nil {
		t.Error("expected error for SSE handler")
	}
	
	gwErr, ok := err.(*gwerrors.Error)
	if !ok {
		t.Errorf("expected gateway error, got %T", err)
	} else if gwErr.Type != gwerrors.ErrorTypeInternal {
		t.Errorf("expected internal error type, got %v", gwErr.Type)
	}
}

func TestComponent_CreateWebSocketHandler(t *testing.T) {
	logger := slog.Default()
	component := NewComponent(logger).(*Component)
	
	router := &mockRouter{}
	httpConnector := &mockConnector{}
	component.SetDependencies(router, httpConnector, nil, nil, nil)
	
	handler := component.CreateWebSocketHandler()
	req := &mockRequest{method: "GET", path: "/ws"}
	
	_, err := handler(context.Background(), req)
	
	if err == nil {
		t.Error("expected error for WebSocket handler")
	}
	
	gwErr, ok := err.(*gwerrors.Error)
	if !ok {
		t.Errorf("expected gateway error, got %T", err)
	} else if gwErr.Type != gwerrors.ErrorTypeInternal {
		t.Errorf("expected internal error type, got %v", gwErr.Type)
	}
}

func TestComponent_CreateRouteAwareHandler(t *testing.T) {
	tests := []struct {
		name          string
		withRouteCtx  bool
		routeError    error
		handlerError  error
		expectError   bool
	}{
		{
			name:         "route already in context",
			withRouteCtx: true,
			expectError:  false,
		},
		{
			name:         "route not in context",
			withRouteCtx: false,
			expectError:  false,
		},
		{
			name:        "routing error",
			routeError:  errors.New("route not found"),
			expectError: true,
		},
		{
			name:         "handler error",
			handlerError: errors.New("handler failed"),
			expectError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			component := NewComponent(logger).(*Component)
			
			router := &mockRouter{
				routeFn: func(ctx context.Context, req core.Request) (*core.RouteResult, error) {
					if tt.routeError != nil {
						return nil, tt.routeError
					}
					return &core.RouteResult{ServiceName: "test-service"}, nil
				},
			}
			
			component.SetDependencies(router, nil, nil, nil, nil)
			
			// Base handler that checks for route in context
			baseHandler := func(ctx context.Context, req core.Request) (core.Response, error) {
				if tt.handlerError != nil {
					return nil, tt.handlerError
				}
				
				route := getRouteFromContext(ctx)
				if route == nil && !tt.withRouteCtx {
					// Route should have been set by CreateRouteAwareHandler
					return nil, errors.New("route not found in context")
				}
				
				return &mockResponse{statusCode: 200}, nil
			}
			
			handler := component.CreateRouteAwareHandler(baseHandler)
			req := &mockRequest{method: "GET", path: "/test"}
			
			ctx := context.Background()
			if tt.withRouteCtx {
				route := &core.RouteResult{ServiceName: "context-service"}
				ctx = setRouteInContext(ctx, route)
			}
			
			resp, err := handler(ctx, req)
			
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("expected response, got nil")
				}
			}
		})
	}
}

func TestApplyMiddleware(t *testing.T) {
	logger := slog.Default()
	
	// Base handler that returns success
	baseHandler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{statusCode: 200}, nil
	}
	
	// Middleware that adds a header
	middleware1 := func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}
			// Mark that this middleware was called by modifying headers
			if mr, ok := resp.(*mockResponse); ok {
				headers := mr.Headers()
				headers["X-Middleware-1"] = []string{"applied"}
			}
			return resp, nil
		}
	}
	
	// Another middleware
	middleware2 := func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}
			// Mark that this middleware was called by modifying headers
			if mr, ok := resp.(*mockResponse); ok {
				headers := mr.Headers()
				headers["X-Middleware-2"] = []string{"applied"}
			}
			return resp, nil
		}
	}
	
	// Apply middleware
	handler := ApplyMiddleware(baseHandler, logger, middleware1, middleware2)
	
	req := &mockRequest{method: "GET", path: "/test"}
	resp, err := handler(context.Background(), req)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	
	// Check that both middlewares were applied
	if mr, ok := resp.(*mockResponse); ok {
		headers := mr.Headers()
		if v, ok := headers["X-Middleware-1"]; !ok || len(v) == 0 || v[0] != "applied" {
			t.Error("middleware 1 was not applied")
		}
		if v, ok := headers["X-Middleware-2"]; !ok || len(v) == 0 || v[0] != "applied" {
			t.Error("middleware 2 was not applied")
		}
	} else {
		t.Error("expected mockResponse type")
	}
}

func TestApplyMiddleware_WithPanic(t *testing.T) {
	logger := slog.Default()
	
	// Handler that panics
	panicHandler := func(ctx context.Context, req core.Request) (core.Response, error) {
		panic("test panic")
	}
	
	// Apply middleware (recovery middleware should be added automatically)
	handler := ApplyMiddleware(panicHandler, logger)
	
	req := &mockRequest{method: "GET", path: "/test"}
	resp, err := handler(context.Background(), req)
	
	// Recovery middleware should catch the panic and return an error
	if err == nil {
		t.Error("expected error from panic recovery, got nil")
	}
	
	if resp != nil {
		t.Error("expected nil response after panic")
	}
}

func TestRouteContext(t *testing.T) {
	ctx := context.Background()
	route := &core.RouteResult{ServiceName: "test-service"}
	
	// Test setting route in context
	ctx = setRouteInContext(ctx, route)
	
	// Test getting route from context
	retrieved := getRouteFromContext(ctx)
	if retrieved == nil {
		t.Fatal("expected route in context, got nil")
	}
	
	if retrieved.ServiceName != route.ServiceName {
		t.Errorf("expected service name %s, got %s", route.ServiceName, retrieved.ServiceName)
	}
	
	// Test getting route from empty context
	emptyCtx := context.Background()
	retrieved = getRouteFromContext(emptyCtx)
	if retrieved != nil {
		t.Error("expected nil route from empty context")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}

// Test that Component implements factory.Component interface
func TestComponentInterface(t *testing.T) {
	logger := slog.Default()
	var component factory.Component = NewComponent(logger)
	
	if component == nil {
		t.Error("expected Component to implement factory.Component interface")
	}
}