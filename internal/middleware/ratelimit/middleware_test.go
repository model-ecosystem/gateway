package ratelimit

import (
	"context"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"io"
	"testing"
	"time"
)

type mockRequest struct {
	method     string
	path       string
	remoteAddr string
}

func (m *mockRequest) ID() string                   { return "test-id" }
func (m *mockRequest) Method() string               { return m.method }
func (m *mockRequest) Path() string                 { return m.path }
func (m *mockRequest) URL() string                  { return "http://test.com" + m.path }
func (m *mockRequest) RemoteAddr() string           { return m.remoteAddr }
func (m *mockRequest) Headers() map[string][]string { return nil }
func (m *mockRequest) Body() io.ReadCloser          { return nil }
func (m *mockRequest) Context() context.Context     { return context.Background() }

type mockResponse struct {
	status int
}

func (m *mockResponse) StatusCode() int              { return m.status }
func (m *mockResponse) Headers() map[string][]string { return nil }
func (m *mockResponse) Body() io.ReadCloser          { return nil }

func TestTokenBucket(t *testing.T) {
	t.Run("allows requests within rate", func(t *testing.T) {
		limiter := NewTokenBucket(10, 10)
		defer limiter.Stop()

		for i := 0; i < 10; i++ {
			if !limiter.Allow("test") {
				t.Fatalf("expected request %d to be allowed", i)
			}
		}

		// 11th request should be denied
		if limiter.Allow("test") {
			t.Fatal("expected request to be denied")
		}
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		// Higher rate for faster refill in tests
		limiter := NewTokenBucket(100, 2)
		defer limiter.Stop()

		// Use up all tokens
		limiter.Allow("test")
		limiter.Allow("test")

		if limiter.Allow("test") {
			t.Fatal("expected request to be denied")
		}

		// Wait for refill (100 tokens/sec means 1 token per 10ms)
		time.Sleep(20 * time.Millisecond)

		if !limiter.Allow("test") {
			t.Fatal("expected request to be allowed after refill")
		}
	})

	t.Run("different keys have separate buckets", func(t *testing.T) {
		limiter := NewTokenBucket(10, 1)
		defer limiter.Stop()

		if !limiter.Allow("key1") {
			t.Fatal("expected first request for key1 to be allowed")
		}

		if !limiter.Allow("key2") {
			t.Fatal("expected first request for key2 to be allowed")
		}

		if limiter.Allow("key1") {
			t.Fatal("expected second request for key1 to be denied")
		}

		if limiter.Allow("key2") {
			t.Fatal("expected second request for key2 to be denied")
		}
	})
}

func TestMiddleware(t *testing.T) {
	t.Run("allows requests within rate", func(t *testing.T) {
		cfg := &Config{
			Rate:  10,
			Burst: 10,
		}

		mw := Middleware(cfg)

		handler := mw(func(ctx context.Context, req core.Request) (core.Response, error) {
			return &mockResponse{status: 200}, nil
		})

		req := &mockRequest{
			method:     "GET",
			path:       "/test",
			remoteAddr: "127.0.0.1:1234",
		}

		// Should allow 10 requests
		for i := 0; i < 10; i++ {
			resp, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("expected no error on request %d, got: %v", i, err)
			}
			if resp.StatusCode() != 200 {
				t.Fatalf("expected status 200, got: %d", resp.StatusCode())
			}
		}

		// 11th request should be rate limited
		_, err := handler(context.Background(), req)
		if err == nil {
			t.Fatal("expected rate limit error")
		}

		rateLimitErr, ok := err.(*errors.Error)
		if !ok {
			t.Fatalf("expected errors.Error, got: %T", err)
		}

		if rateLimitErr.Type != errors.ErrorTypeRateLimit {
			t.Fatalf("expected ErrorTypeRateLimit, got: %v", rateLimitErr.Type)
		}
	})

	t.Run("uses custom key function", func(t *testing.T) {
		cfg := &Config{
			Rate:    10,
			Burst:   1,
			KeyFunc: ByPath,
		}

		mw := Middleware(cfg)

		handler := mw(func(ctx context.Context, req core.Request) (core.Response, error) {
			return &mockResponse{status: 200}, nil
		})

		req1 := &mockRequest{
			method:     "GET",
			path:       "/path1",
			remoteAddr: "127.0.0.1:1234",
		}

		req2 := &mockRequest{
			method:     "GET",
			path:       "/path2",
			remoteAddr: "127.0.0.1:1234",
		}

		// Different paths should have separate limits
		resp1, err1 := handler(context.Background(), req1)
		if err1 != nil {
			t.Fatalf("expected no error for path1, got: %v", err1)
		}
		if resp1.StatusCode() != 200 {
			t.Fatalf("expected status 200, got: %d", resp1.StatusCode())
		}

		resp2, err2 := handler(context.Background(), req2)
		if err2 != nil {
			t.Fatalf("expected no error for path2, got: %v", err2)
		}
		if resp2.StatusCode() != 200 {
			t.Fatalf("expected status 200, got: %d", resp2.StatusCode())
		}
	})

	t.Run("per route configuration", func(t *testing.T) {
		rules := map[string]*Config{
			"/api/": {
				Rate:    5,
				Burst:   5,
				KeyFunc: ByIP,
			},
			"/public/": {
				Rate:    100,
				Burst:   100,
				KeyFunc: ByIP,
			},
		}

		mw := PerRoute(rules)

		handler := mw(func(ctx context.Context, req core.Request) (core.Response, error) {
			return &mockResponse{status: 200}, nil
		})

		apiReq := &mockRequest{
			method:     "GET",
			path:       "/api/users",
			remoteAddr: "127.0.0.1:1234",
		}

		publicReq := &mockRequest{
			method:     "GET",
			path:       "/public/docs",
			remoteAddr: "127.0.0.1:1234",
		}

		// API should allow 5 requests
		for i := 0; i < 5; i++ {
			_, err := handler(context.Background(), apiReq)
			if err != nil {
				t.Fatalf("expected no error on API request %d, got: %v", i, err)
			}
		}

		// 6th API request should fail
		_, err := handler(context.Background(), apiReq)
		if err == nil {
			t.Fatal("expected rate limit error for API")
		}

		// Public should still allow requests
		for i := 0; i < 10; i++ {
			_, err := handler(context.Background(), publicReq)
			if err != nil {
				t.Fatalf("expected no error on public request %d, got: %v", i, err)
			}
		}
	})
}
