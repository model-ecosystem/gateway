package retry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gateway/internal/core"
	gwerrors "gateway/pkg/errors"
	"gateway/pkg/retry"
)

// Mock request and response types
type mockRequest struct {
	method  string
	path    string
	headers map[string][]string
	body    string
}

func (m *mockRequest) ID() string                   { return "test-req-123" }
func (m *mockRequest) Method() string               { return m.method }
func (m *mockRequest) Path() string                 { return m.path }
func (m *mockRequest) URL() string                  { return "http://example.com" + m.path }
func (m *mockRequest) RemoteAddr() string           { return "127.0.0.1:12345" }
func (m *mockRequest) Headers() map[string][]string { return m.headers }
func (m *mockRequest) Body() io.ReadCloser          { return io.NopCloser(strings.NewReader(m.body)) }
func (m *mockRequest) Context() context.Context     { return context.Background() }

type mockResponse struct {
	statusCode int
	headers    map[string][]string
	body       string
}

func (m *mockResponse) StatusCode() int              { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string { return m.headers }
func (m *mockResponse) Body() io.ReadCloser          { return io.NopCloser(strings.NewReader(m.body)) }

func TestNew(t *testing.T) {
	config := Config{
		Default: retry.Config{
			MaxAttempts:  3,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     1 * time.Second,
			Multiplier:   2,
		},
	}
	
	logger := slog.Default()
	middleware := New(config, logger)
	
	if middleware == nil {
		t.Fatal("Expected non-nil middleware")
	}
	
	if middleware.logger == nil {
		t.Error("Expected logger to be set")
	}
	
	if middleware.config.Default.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts 3, got %d", middleware.config.Default.MaxAttempts)
	}
}

func TestMiddleware_Apply_Success(t *testing.T) {
	config := Config{
		Default: retry.Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
		},
	}
	
	middleware := New(config, slog.Default())
	
	// Handler that succeeds immediately
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{
			statusCode: 200,
			body:       "success",
		}, nil
	}
	
	wrapped := middleware.Apply()(handler)
	
	req := &mockRequest{
		method: "GET",
		path:   "/test",
	}
	
	resp, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}
}

func TestMiddleware_Apply_RetryOnFailure(t *testing.T) {
	config := Config{
		Default: retry.Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
		},
	}
	
	middleware := New(config, slog.Default())
	
	// Handler that fails twice then succeeds
	var attempts int32
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			return nil, errors.New("temporary failure")
		}
		return &mockResponse{
			statusCode: 200,
			body:       "success",
		}, nil
	}
	
	wrapped := middleware.Apply()(handler)
	
	req := &mockRequest{
		method: "GET",
		path:   "/test",
	}
	
	start := time.Now()
	resp, err := wrapped(context.Background(), req)
	duration := time.Since(start)
	
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	
	if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}
	
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
	
	// Should have some delay due to retries
	if duration < 20*time.Millisecond {
		t.Errorf("Expected retry delays, but duration was only %v", duration)
	}
}

func TestMiddleware_Apply_MaxAttemptsExceeded(t *testing.T) {
	config := Config{
		Default: retry.Config{
			MaxAttempts:  2,
			InitialDelay: 10 * time.Millisecond,
		},
	}
	
	middleware := New(config, slog.Default())
	
	// Handler that always fails
	var attempts int32
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, errors.New("persistent failure")
	}
	
	wrapped := middleware.Apply()(handler)
	
	req := &mockRequest{
		method: "GET",
		path:   "/test",
	}
	
	resp, err := wrapped(context.Background(), req)
	
	if err == nil {
		t.Fatal("Expected error after max attempts")
	}
	
	if resp != nil {
		t.Error("Expected nil response on failure")
	}
	
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("Expected 2 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestMiddleware_Apply_NonRetryableError(t *testing.T) {
	config := Config{
		Default: retry.Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
		},
	}
	
	middleware := New(config, slog.Default())
	
	// Handler that returns non-retryable error
	var attempts int32
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, gwerrors.NewError(gwerrors.ErrorTypeBadRequest, "invalid request")
	}
	
	wrapped := middleware.Apply()(handler)
	
	req := &mockRequest{
		method: "POST",
		path:   "/test",
	}
	
	_, err := wrapped(context.Background(), req)
	
	if err == nil {
		t.Fatal("Expected error")
	}
	
	// Should not retry on client errors
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("Expected 1 attempt (no retry), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestMiddleware_Apply_ContextCancellation(t *testing.T) {
	config := Config{
		Default: retry.Config{
			MaxAttempts:  5,
			InitialDelay: 100 * time.Millisecond,
		},
	}
	
	middleware := New(config, slog.Default())
	
	// Handler that always fails
	var attempts int32
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, errors.New("failure")
	}
	
	wrapped := middleware.Apply()(handler)
	
	req := &mockRequest{
		method: "GET",
		path:   "/test",
	}
	
	// Cancel context after short delay
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	_, err := wrapped(ctx, req)
	
	if err == nil {
		t.Fatal("Expected error due to context cancellation")
	}
	
	// Should have attempted at least once but not all retries
	attemptCount := atomic.LoadInt32(&attempts)
	if attemptCount < 1 || attemptCount >= 5 {
		t.Errorf("Expected 1-4 attempts due to context cancellation, got %d", attemptCount)
	}
}

func TestMiddleware_Apply_RouteSpecificConfig(t *testing.T) {
	config := Config{
		Default: retry.Config{
			MaxAttempts:  2,
			InitialDelay: 100 * time.Millisecond,
		},
		Routes: map[string]retry.Config{
			"critical-route": {
				MaxAttempts:  5,
				InitialDelay: 10 * time.Millisecond,
			},
		},
	}
	
	middleware := New(config, slog.Default())
	
	// Handler that fails 3 times
	var attempts int32
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt <= 3 {
			return nil, errors.New("failure")
		}
		return &mockResponse{statusCode: 200}, nil
	}
	
	wrapped := middleware.Apply()(handler)
	
	// Add route context
	routeResult := &core.RouteResult{
		Rule: &core.RouteRule{
			ID: "critical-route",
		},
	}
	ctx := context.WithValue(context.Background(), routeContextKey{}, routeResult)
	
	req := &mockRequest{
		method: "GET",
		path:   "/critical",
	}
	
	resp, err := wrapped(ctx, req)
	
	if err != nil {
		t.Fatalf("Expected success with route-specific config, got error: %v", err)
	}
	
	if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}
	
	// Should succeed because route config allows 5 attempts
	if atomic.LoadInt32(&attempts) != 4 {
		t.Errorf("Expected 4 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestMiddleware_Apply_ExponentialBackoff(t *testing.T) {
	config := Config{
		Default: retry.Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			Multiplier:   2,
			MaxDelay:     50 * time.Millisecond,
		},
	}
	
	middleware := New(config, slog.Default())
	
	// Track delays between attempts
	var delays []time.Duration
	var lastAttemptTime time.Time
	var attempts int32
	
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		now := time.Now()
		if atomic.AddInt32(&attempts, 1) > 1 {
			delay := now.Sub(lastAttemptTime)
			delays = append(delays, delay)
		}
		lastAttemptTime = now
		return nil, errors.New("failure")
	}
	
	wrapped := middleware.Apply()(handler)
	
	req := &mockRequest{
		method: "GET",
		path:   "/test",
	}
	
	wrapped(context.Background(), req)
	
	if len(delays) != 2 {
		t.Fatalf("Expected 2 delays, got %d", len(delays))
	}
	
	// First retry delay should be ~10ms
	if delays[0] < 9*time.Millisecond || delays[0] > 15*time.Millisecond {
		t.Errorf("Expected first delay ~10ms, got %v", delays[0])
	}
	
	// Second retry delay should be ~20ms (doubled)
	if delays[1] < 18*time.Millisecond || delays[1] > 25*time.Millisecond {
		t.Errorf("Expected second delay ~20ms, got %v", delays[1])
	}
}

func TestIsRetryableError(t *testing.T) {
	middleware := &Middleware{}
	
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "generic error",
			err:       errors.New("some error"),
			retryable: true,
		},
		{
			name:      "timeout error",
			err:       context.DeadlineExceeded,
			retryable: true,
		},
		{
			name:      "bad request",
			err:       gwerrors.NewError(gwerrors.ErrorTypeBadRequest, "bad input"),
			retryable: false,
		},
		{
			name:      "unauthorized",
			err:       gwerrors.NewError(gwerrors.ErrorTypeUnauthorized, "no auth"),
			retryable: false,
		},
		{
			name:      "forbidden",
			err:       gwerrors.NewError(gwerrors.ErrorTypeForbidden, "forbidden"),
			retryable: false,
		},
		{
			name:      "not found",
			err:       gwerrors.NewError(gwerrors.ErrorTypeNotFound, "not found"),
			retryable: false,
		},
		{
			name:      "internal error",
			err:       gwerrors.NewError(gwerrors.ErrorTypeInternal, "internal"),
			retryable: true,
		},
		{
			name:      "unavailable",
			err:       gwerrors.NewError(gwerrors.ErrorTypeUnavailable, "unavailable"),
			retryable: true,
		},
		{
			name:      "timeout",
			err:       gwerrors.NewError(gwerrors.ErrorTypeTimeout, "timeout"),
			retryable: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := middleware.isRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("Expected retryable=%v for %v, got %v", tt.retryable, tt.err, result)
			}
		})
	}
}

