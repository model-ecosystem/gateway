package integration

import (
	"context"
	"io"
	"testing"
	"time"

	"gateway/internal/core"
	"gateway/internal/middleware/ratelimit"
	"gateway/internal/storage"
	"gateway/internal/storage/memory"
)

func TestRateLimitMiddlewareSimple(t *testing.T) {
	// Create a simple rate limiter with 2 requests per second, burst of 3
	store := memory.NewStore(storage.DefaultConfig())
	cfg := &ratelimit.Config{
		Rate:    2,
		Burst:   3,
		KeyFunc: ratelimit.ByIP,
		Store:   store,
	}

	// Counter for successful requests
	successCount := 0
	
	// Create a handler that just counts successes
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		successCount++
		return &mockResponse{}, nil
	}

	// Apply rate limiting middleware
	limitedHandler := ratelimit.Middleware(cfg)(handler)

	// Create mock request
	mockReq := &mockRequest{path: "/test"}

	// Should allow burst of 3 requests immediately
	for i := 0; i < 3; i++ {
		_, err := limitedHandler(context.Background(), mockReq)
		if err != nil {
			t.Errorf("Request %d should have succeeded: %v", i+1, err)
		}
	}

	// 4th request should be rate limited
	_, err := limitedHandler(context.Background(), mockReq)
	if err == nil {
		t.Error("4th request should have been rate limited")
	}

	if successCount != 3 {
		t.Errorf("Expected 3 successful requests, got %d", successCount)
	}

	// Wait for tokens to refill (at 2/sec, we should get 1 token after 500ms)
	time.Sleep(600 * time.Millisecond)

	// Should now allow one more request
	_, err = limitedHandler(context.Background(), mockReq)
	if err != nil {
		t.Errorf("Request after refill should have succeeded: %v", err)
	}

	if successCount != 4 {
		t.Errorf("Expected 4 successful requests after refill, got %d", successCount)
	}
}

func TestPerRouteRateLimit(t *testing.T) {
	// Create per-route configuration
	store1 := memory.NewStore(storage.DefaultConfig())
	store2 := memory.NewStore(storage.DefaultConfig())
	rules := map[string]*ratelimit.Config{
		"/api/*": {
			Rate:    1,
			Burst:   2,
			KeyFunc: ratelimit.ByIP,
			Store:   store1,
		},
		"/public/*": {
			Rate:    10,
			Burst:   20,
			KeyFunc: ratelimit.ByIP,
			Store:   store2,
		},
	}

	// Counter for successful requests
	successCount := 0
	
	// Create a handler that just counts successes
	handler := func(ctx context.Context, req core.Request) (core.Response, error) {
		successCount++
		return &mockResponse{}, nil
	}

	// Apply per-route rate limiting
	limitedHandler := ratelimit.PerRoute(rules)(handler)

	t.Run("api route limited", func(t *testing.T) {
		successCount = 0
		// Create request for /api/ route
		apiReq := &mockRequest{path: "/api/users"}

		// Should allow burst of 2
		for i := 0; i < 2; i++ {
			_, err := limitedHandler(context.Background(), apiReq)
			if err != nil {
				t.Errorf("API request %d should have succeeded: %v", i+1, err)
			}
		}

		// 3rd request should be rate limited
		_, err := limitedHandler(context.Background(), apiReq)
		if err == nil {
			t.Error("3rd API request should have been rate limited")
		}

		if successCount != 2 {
			t.Errorf("Expected 2 successful API requests, got %d", successCount)
		}
	})

	t.Run("public route higher limit", func(t *testing.T) {
		successCount = 0
		// Create request for /public/ route
		publicReq := &mockRequest{path: "/public/info"}

		// Should allow burst of 20
		for i := 0; i < 20; i++ {
			_, err := limitedHandler(context.Background(), publicReq)
			if err != nil {
				t.Errorf("Public request %d should have succeeded: %v", i+1, err)
			}
		}

		// 21st request should be rate limited
		_, err := limitedHandler(context.Background(), publicReq)
		if err == nil {
			t.Error("21st public request should have been rate limited")
		}

		if successCount != 20 {
			t.Errorf("Expected 20 successful public requests, got %d", successCount)
		}
	})

	t.Run("unmatched route not limited", func(t *testing.T) {
		successCount = 0
		// Create request for unmatched route
		otherReq := &mockRequest{path: "/other/path"}

		// Should allow many requests (no rate limit)
		for i := 0; i < 50; i++ {
			_, err := limitedHandler(context.Background(), otherReq)
			if err != nil {
				t.Errorf("Other request %d should have succeeded: %v", i+1, err)
			}
		}

		if successCount != 50 {
			t.Errorf("Expected 50 successful other requests, got %d", successCount)
		}
	})
}

// Mock implementations for testing

type mockResponse struct{}

func (m *mockResponse) StatusCode() int              { return 200 }
func (m *mockResponse) Headers() map[string][]string { return nil }
func (m *mockResponse) Body() io.ReadCloser          { return nil }

type mockRequest struct {
	path string
}

func (m *mockRequest) ID() string                   { return "test-id" }
func (m *mockRequest) Method() string               { return "GET" }
func (m *mockRequest) Path() string                 { return m.path }
func (m *mockRequest) URL() string                  { return "http://example.com" + m.path }
func (m *mockRequest) RemoteAddr() string           { return "127.0.0.1:12345" }
func (m *mockRequest) Headers() map[string][]string { return nil }
func (m *mockRequest) Body() io.ReadCloser          { return nil }
func (m *mockRequest) Context() context.Context     { return context.Background() }