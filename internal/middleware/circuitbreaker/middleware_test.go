package circuitbreaker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"gateway/internal/core"
	gwerrors "gateway/pkg/errors"
	"gateway/pkg/circuitbreaker"
)

// Mock request for testing
type mockRequest struct {
	path string
}

func (m *mockRequest) ID() string                   { return "test-id" }
func (m *mockRequest) Method() string               { return "GET" }
func (m *mockRequest) Path() string                 { return m.path }
func (m *mockRequest) URL() string                  { return m.path }
func (m *mockRequest) RemoteAddr() string           { return "127.0.0.1:12345" }
func (m *mockRequest) Headers() map[string][]string { return nil }
func (m *mockRequest) Body() io.ReadCloser          { return nil }
func (m *mockRequest) Context() context.Context     { return context.Background() }

// Mock response for testing
type mockResponse struct {
	statusCode int
}

func (m *mockResponse) StatusCode() int              { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string { return nil }
func (m *mockResponse) Body() io.ReadCloser          { return nil }

func TestNew(t *testing.T) {
	config := Config{
		Default: circuitbreaker.Config{
			MaxFailures: 5,
			Timeout:     time.Second,
		},
	}
	logger := slog.Default()

	middleware := New(config, logger)

	if middleware == nil {
		t.Fatal("Expected middleware instance, got nil")
	}

	if middleware.logger == nil {
		t.Error("Logger not set")
	}
}

func TestMiddleware_Apply(t *testing.T) {
	config := Config{
		Default: circuitbreaker.Config{
			MaxFailures: 2,
			Timeout:     time.Second,
			Interval:    time.Second,
		},
	}
	logger := slog.Default()
	middleware := New(config, logger)

	failCount := 0
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		if failCount < 2 {
			failCount++
			return nil, errors.New("temporary failure")
		}
		return &mockResponse{statusCode: 200}, nil
	}

	wrapped := middleware.Apply()(handler)

	// First two requests should fail
	req := &mockRequest{path: "/test"}
	
	_, err1 := wrapped(context.Background(), req)
	if err1 == nil {
		t.Error("Expected first request to fail")
	}

	_, err2 := wrapped(context.Background(), req)
	if err2 == nil {
		t.Error("Expected second request to fail")
	}

	// Third request should be blocked by circuit breaker
	_, err3 := wrapped(context.Background(), req)
	if err3 == nil {
		t.Error("Expected circuit breaker to be open")
	}

	var gwErr *gwerrors.Error
	if errors.As(err3, &gwErr) {
		if gwErr.Type != gwerrors.ErrorTypeUnavailable {
			t.Errorf("Expected ErrorTypeUnavailable, got %v", gwErr.Type)
		}
		if gwErr.Message != "Service temporarily unavailable" {
			t.Errorf("Unexpected error message: %s", gwErr.Message)
		}
	} else {
		t.Error("Expected gateway error")
	}
}

func TestMiddleware_RouteBasedCircuitBreaker(t *testing.T) {
	config := Config{
		Default: circuitbreaker.Config{
			MaxFailures: 5,
			Timeout:     time.Second,
		},
		Routes: map[string]circuitbreaker.Config{
			"api-route": {
				MaxFailures: 1, // More sensitive for API route
				Timeout:     500 * time.Millisecond,
			},
		},
	}
	logger := slog.Default()
	middleware := New(config, logger)

	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return nil, errors.New("failure")
	}

	wrapped := middleware.Apply()(handler)

	// Create context with route info
	routeResult := &core.RouteResult{
		Rule: &core.RouteRule{
			ID: "api-route",
		},
	}
	ctx := context.WithValue(context.Background(), routeContextKey{}, routeResult)

	req := &mockRequest{path: "/api/test"}

	// First failure
	_, err1 := wrapped(ctx, req)
	if err1 == nil {
		t.Error("Expected first request to fail")
	}

	// Second request should be blocked (route config has MaxFailures: 1)
	_, err2 := wrapped(ctx, req)
	if err2 == nil {
		t.Error("Expected circuit breaker to be open")
	}

	var gwErr *gwerrors.Error
	if errors.As(err2, &gwErr) && gwErr.Type != gwerrors.ErrorTypeUnavailable {
		t.Error("Expected circuit breaker to block request")
	}
}

func TestMiddleware_ServiceBasedCircuitBreaker(t *testing.T) {
	config := Config{
		Default: circuitbreaker.Config{
			MaxFailures: 5,
			Timeout:     time.Second,
		},
		Services: map[string]circuitbreaker.Config{
			"payment-service": {
				MaxFailures: 2,
				Timeout:     2 * time.Second,
			},
		},
	}
	logger := slog.Default()
	middleware := New(config, logger)

	// Create context with service info
	routeResult := &core.RouteResult{
		Rule: &core.RouteRule{
			ServiceName: "payment-service",
		},
	}
	ctx := context.WithValue(context.Background(), routeContextKey{}, routeResult)

	// Get the breaker that will be created
	key := "service:payment-service"
	
	// Call the middleware to create the breaker
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{statusCode: 200}, nil
	}
	wrapped := middleware.Apply()(handler)
	req := &mockRequest{path: "/payment"}
	
	_, _ = wrapped(ctx, req)

	// Now get the breaker
	breaker := middleware.GetBreaker(key)
	if breaker == nil {
		t.Fatal("Expected circuit breaker for payment service")
	}

	// Verify it uses service-specific config
	stats := breaker.Stats()
	if stats.Failures > 0 {
		t.Error("Expected no failures initially")
	}
}

func TestMiddleware_NonRetryableErrors(t *testing.T) {
	config := Config{
		Default: circuitbreaker.Config{
			MaxFailures: 1,
			Timeout:     time.Second,
		},
	}
	logger := slog.Default()
	middleware := New(config, logger)

	nonRetryableErrors := []gwerrors.ErrorType{
		gwerrors.ErrorTypeBadRequest,
		gwerrors.ErrorTypeUnauthorized,
		gwerrors.ErrorTypeForbidden,
		gwerrors.ErrorTypeNotFound,
	}

	for _, errType := range nonRetryableErrors {
		t.Run(string(errType), func(t *testing.T) {
			callCount := 0
			handler := func(ctx context.Context, req core.Request) (core.Response, error) {
				callCount++
				return nil, gwerrors.NewError(errType, "test error")
			}

			wrapped := middleware.Apply()(handler)
			req := &mockRequest{path: "/test-" + string(errType)}

			// Make multiple requests with non-retryable errors
			for i := 0; i < 3; i++ {
				_, err := wrapped(context.Background(), req)
				if err == nil {
					t.Error("Expected error")
				}
			}

			// All requests should go through (circuit breaker should not open)
			if callCount != 3 {
				t.Errorf("Expected 3 calls, got %d", callCount)
			}
		})
	}
}

func TestMiddleware_SuccessResetsFailures(t *testing.T) {
	config := Config{
		Default: circuitbreaker.Config{
			MaxFailures:      3,
			FailureThreshold: 0.8, // Higher threshold to prevent early opening
			Timeout:          time.Second,
		},
	}
	logger := slog.Default()
	middleware := New(config, logger)

	shouldFail := true
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		if shouldFail {
			return nil, errors.New("failure")
		}
		return &mockResponse{statusCode: 200}, nil
	}

	wrapped := middleware.Apply()(handler)
	req := &mockRequest{path: "/test"}

	// First, have some successes to prevent 100% failure rate
	shouldFail = false
	wrapped(context.Background(), req)
	
	// Now two failures (2 failures out of 3 total = 66.7% < 80% threshold)
	shouldFail = true
	wrapped(context.Background(), req)
	wrapped(context.Background(), req)

	// Circuit should still be closed, success should work
	shouldFail = false
	resp, err := wrapped(context.Background(), req)
	if err != nil {
		t.Error("Expected success")
	}
	if resp != nil && resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}

	// More failures should be allowed
	shouldFail = true
	for i := 0; i < 2; i++ {
		_, err := wrapped(context.Background(), req)
		if err == nil {
			t.Error("Expected failure")
		}
	}
}

func TestMiddleware_ResetAll(t *testing.T) {
	config := Config{
		Default: circuitbreaker.Config{
			MaxFailures: 1,
			Timeout:     time.Second,
		},
	}
	logger := slog.Default()
	middleware := New(config, logger)

	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return nil, errors.New("failure")
	}

	wrapped := middleware.Apply()(handler)

	// Create multiple circuit breakers
	paths := []string{"/api/v1", "/api/v2", "/api/v3"}
	
	for _, path := range paths {
		req := &mockRequest{path: path}
		// Trigger circuit breaker
		wrapped(context.Background(), req)
		wrapped(context.Background(), req) // Should open the breaker
	}

	// Reset all breakers
	middleware.ResetAll()

	// All requests should succeed again
	successHandler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{statusCode: 200}, nil
	}
	wrapped = middleware.Apply()(successHandler)

	for _, path := range paths {
		req := &mockRequest{path: path}
		resp, err := wrapped(context.Background(), req)
		if err != nil {
			t.Errorf("Expected success after reset for path %s", path)
		}
		if resp.StatusCode() != 200 {
			t.Errorf("Expected status 200 for path %s", path)
		}
	}
}

func TestMiddleware_GetBreaker(t *testing.T) {
	config := Config{
		Default: circuitbreaker.Config{
			MaxFailures: 5,
			Timeout:     time.Second,
		},
	}
	logger := slog.Default()
	middleware := New(config, logger)

	// Initially no breaker
	breaker := middleware.GetBreaker("path:/test")
	if breaker != nil {
		t.Error("Expected no breaker initially")
	}

	// Trigger creation
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{statusCode: 200}, nil
	}
	wrapped := middleware.Apply()(handler)
	req := &mockRequest{path: "/test"}
	wrapped(context.Background(), req)

	// Now breaker should exist
	breaker = middleware.GetBreaker("path:/test")
	if breaker == nil {
		t.Error("Expected breaker to exist after request")
	}
}

func TestMiddleware_StateChangeCallback(t *testing.T) {
	stateChanged := false
	var mu sync.Mutex
	
	config := Config{
		Default: circuitbreaker.Config{
			MaxFailures: 1,
			Timeout:     100 * time.Millisecond,
			OnStateChange: func(from, to circuitbreaker.State) {
				mu.Lock()
				stateChanged = true
				mu.Unlock()
			},
		},
	}
	logger := slog.Default()
	middleware := New(config, logger)

	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return nil, errors.New("failure")
	}

	wrapped := middleware.Apply()(handler)
	req := &mockRequest{path: "/test"}

	// Trigger state change
	wrapped(context.Background(), req)
	wrapped(context.Background(), req) // Should open the breaker

	// Wait a bit for the goroutine to execute
	time.Sleep(10 * time.Millisecond)
	
	mu.Lock()
	changed := stateChanged
	mu.Unlock()
	
	if !changed {
		t.Error("Expected state change callback to be called")
	}
}