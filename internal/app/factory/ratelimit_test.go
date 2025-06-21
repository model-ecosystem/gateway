package factory

import (
	"log/slog"
	"testing"

	"gateway/internal/config"
	"gateway/internal/middleware/ratelimit"
)

func TestCreateRateLimitMiddleware(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name    string
		config  *config.Router
		wantNil bool
	}{
		{
			name: "no rate limiting configured",
			config: &config.Router{
				Rules: []config.RouteRule{
					{
						ID:          "test-route",
						Path:        "/api/*",
						ServiceName: "test-service",
					},
				},
			},
			wantNil: true,
		},
		{
			name: "single route with rate limiting",
			config: &config.Router{
				Rules: []config.RouteRule{
					{
						ID:             "rate-limited-route",
						Path:           "/api/*",
						ServiceName:    "test-service",
						RateLimit:      10,
						RateLimitBurst: 20,
					},
				},
			},
			wantNil: false,
		},
		{
			name: "multiple routes with mixed configuration",
			config: &config.Router{
				Rules: []config.RouteRule{
					{
						ID:          "unlimited-route",
						Path:        "/public/*",
						ServiceName: "public-service",
					},
					{
						ID:             "limited-route-1",
						Path:           "/api/v1/*",
						ServiceName:    "api-service",
						RateLimit:      50,
						RateLimitBurst: 100,
					},
					{
						ID:             "limited-route-2",
						Path:           "/admin/*",
						ServiceName:    "admin-service",
						RateLimit:      5,
						RateLimitBurst: 10,
					},
				},
			},
			wantNil: false,
		},
		{
			name: "rate limit with default burst",
			config: &config.Router{
				Rules: []config.RouteRule{
					{
						ID:          "default-burst-route",
						Path:        "/api/*",
						ServiceName: "test-service",
						RateLimit:   25,
						// RateLimitBurst not set, should default to RateLimit
					},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CreateRateLimitMiddleware(tt.config, nil, logger)

			if tt.wantNil && middleware != nil {
				t.Error("expected nil middleware, got non-nil")
			}

			if !tt.wantNil && middleware == nil {
				t.Error("expected non-nil middleware, got nil")
			}
		})
	}
}

func TestCreateGlobalRateLimitMiddleware(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name    string
		rate    int
		burst   int
		wantNil bool
	}{
		{
			name:    "zero rate",
			rate:    0,
			burst:   0,
			wantNil: true,
		},
		{
			name:    "negative rate",
			rate:    -1,
			burst:   10,
			wantNil: true,
		},
		{
			name:    "valid rate and burst",
			rate:    100,
			burst:   200,
			wantNil: false,
		},
		{
			name:    "valid rate with default burst",
			rate:    50,
			burst:   0, // Should default to rate
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CreateGlobalRateLimitMiddleware(tt.rate, tt.burst, nil, logger)

			if tt.wantNil && middleware != nil {
				t.Error("expected nil middleware, got non-nil")
			}

			if !tt.wantNil && middleware == nil {
				t.Error("expected non-nil middleware, got nil")
			}
		})
	}
}

func TestCreateRateLimitKeyFunc(t *testing.T) {
	tests := []struct {
		name     string
		keyType  string
		wantFunc ratelimit.KeyFunc
	}{
		{
			name:     "IP key type",
			keyType:  "ip",
			wantFunc: ratelimit.ByIP,
		},
		{
			name:     "path key type",
			keyType:  "path",
			wantFunc: ratelimit.ByPath,
		},
		{
			name:     "IP and path key type",
			keyType:  "ip_path",
			wantFunc: ratelimit.ByIPAndPath,
		},
		{
			name:     "default key type",
			keyType:  "unknown",
			wantFunc: ratelimit.ByIP,
		},
		{
			name:     "empty key type",
			keyType:  "",
			wantFunc: ratelimit.ByIP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyFunc := CreateRateLimitKeyFunc(tt.keyType)

			// We can't directly compare functions, but we can verify
			// we get a non-nil function
			if keyFunc == nil {
				t.Error("expected non-nil key function")
			}

			// Test the function with a mock request
			mockReq := &mockRequest{
				path: "/test",
			}

			key := keyFunc(mockReq)
			if key == "" {
				t.Error("key function returned empty key")
			}
		})
	}
}
