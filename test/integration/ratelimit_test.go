package integration

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"gateway/internal/app"
	"gateway/internal/config"
)

func TestRateLimitingIntegration(t *testing.T) {
	// Create a test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	// Extract backend host and port
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("Failed to parse backend URL: %v", err)
	}
	host := backendURL.Hostname()
	portStr := backendURL.Port()
	if portStr == "" {
		portStr = "80"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("Failed to parse port: %v", err)
	}

	// Create gateway configuration with rate limiting
	cfg := &config.Config{
		Gateway: config.Gateway{
			Frontend: config.Frontend{
				HTTP: config.HTTP{
					Host:         "127.0.0.1",
					Port:         18080, // Use fixed port for testing
					ReadTimeout:  10,
					WriteTimeout: 10,
				},
			},
			Backend: config.Backend{
				HTTP: config.HTTPBackend{
					MaxIdleConns:        10,
					MaxIdleConnsPerHost: 10,
					IdleConnTimeout:     30,
					DialTimeout:         5,
				},
			},
			Registry: config.Registry{
				Type: "static",
				Static: &config.StaticRegistry{
					Services: []config.Service{
						{
							Name: "test-service",
							Instances: []config.Instance{
								{
									ID:      "test-1",
									Address: host,
									Port:    port,
									Health:  "healthy",
								},
							},
						},
					},
				},
			},
			Router: config.Router{
				Rules: []config.RouteRule{
					{
						ID:             "rate-limited",
						Path:           "/limited/*",
						ServiceName:    "test-service",
						LoadBalance:    "round_robin",
						Timeout:        10,
						RateLimit:      5,  // 5 requests per second
						RateLimitBurst: 10, // Allow burst of 10
					},
					{
						ID:          "unlimited",
						Path:        "/unlimited/*",
						ServiceName: "test-service",
						LoadBalance: "round_robin",
						Timeout:     5,
						// No rate limit
					},
				},
			},
		},
	}

	// Build and start the gateway
	logger := slog.Default()
	builder := app.NewBuilder(cfg, logger)
	server, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build gateway: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the gateway
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start gateway: %v", err)
	}
	defer server.Stop(context.Background())

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Get the gateway URL - using the configured port
	gatewayURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Gateway.Frontend.HTTP.Port)

	t.Run("rate limited endpoint", func(t *testing.T) {
		// Should allow burst of 10 requests immediately
		for i := 0; i < 10; i++ {
			resp, err := http.Get(gatewayURL + "/limited/test")
			if err != nil {
				t.Fatalf("Request %d failed: %v", i+1, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Request %d: expected status 200, got %d", i+1, resp.StatusCode)
			}
		}

		// 11th request should be rate limited
		resp, err := http.Get(gatewayURL + "/limited/test")
		if err != nil {
			t.Fatalf("Request 11 failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Request 11: expected status 429, got %d. Body: %s", resp.StatusCode, body)
		}
	})

	t.Run("unlimited endpoint", func(t *testing.T) {
		// Should allow many requests without rate limiting
		for i := 0; i < 20; i++ {
			resp, err := http.Get(gatewayURL + "/unlimited/test")
			if err != nil {
				t.Fatalf("Request %d failed: %v", i+1, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Request %d: expected status 200, got %d", i+1, resp.StatusCode)
			}
		}
	})

	t.Run("rate limit recovery", func(t *testing.T) {
		// First exhaust the rate limit
		for i := 0; i < 11; i++ {
			resp, err := http.Get(gatewayURL + "/limited/recovery")
			if err != nil {
				t.Fatalf("Request %d failed: %v", i+1, err)
			}
			resp.Body.Close()
		}

		// Wait for tokens to refill (at 5/sec rate, we should get 1 token after 200ms)
		time.Sleep(250 * time.Millisecond)

		// Should now allow one more request
		resp, err := http.Get(gatewayURL + "/limited/recovery")
		if err != nil {
			t.Fatalf("Recovery request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Recovery request: expected status 200, got %d", resp.StatusCode)
		}
	})
}
