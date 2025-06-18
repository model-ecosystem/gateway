package router

import (
	"context"
	"fmt"
	"gateway/internal/core"
	"testing"
)

func BenchmarkRouterRoute(b *testing.B) {
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"bench-service": {
				{ID: "bench-1", Address: "127.0.0.1", Port: 8001, Healthy: true},
			},
		},
	}

	router := NewRouter(registry)

	// Add many routes to test performance
	for i := 0; i < 1000; i++ {
		rule := core.RouteRule{
			ID:          fmt.Sprintf("bench-%d", i),
			Path:        fmt.Sprintf("/api/v%d/resource", i),
			ServiceName: "bench-service",
		}
		if err := router.AddRule(rule); err != nil {
			b.Fatalf("Failed to add rule: %v", err)
		}
	}

	ctx := context.Background()
	req := &mockRequest{method: "GET", path: "/api/v500/resource"}

	b.ResetTimer()
	b.Run("ServeMux", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			result, err := router.Route(ctx, req)
			if err != nil {
				b.Fatalf("Route failed: %v", err)
			}
			if result == nil {
				b.Fatal("Route returned nil result")
			}
		}
	})
}

func BenchmarkRouterRouteWithParams(b *testing.B) {
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"param-service": {
				{ID: "param-1", Address: "127.0.0.1", Port: 8001, Healthy: true},
			},
		},
	}

	router := NewRouter(registry)

	// Add routes with parameters
	rules := []core.RouteRule{
		{
			ID:          "users",
			Path:        "/api/users/:id",
			ServiceName: "param-service",
		},
		{
			ID:          "posts",
			Path:        "/api/posts/:id/comments/:commentId",
			ServiceName: "param-service",
		},
		{
			ID:          "files",
			Path:        "/api/files/*",
			ServiceName: "param-service",
		},
	}

	for _, rule := range rules {
		if err := router.AddRule(rule); err != nil {
			b.Fatalf("Failed to add rule: %v", err)
		}
	}

	ctx := context.Background()

	b.Run("SimpleParam", func(b *testing.B) {
		req := &mockRequest{method: "GET", path: "/api/users/12345"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := router.Route(ctx, req)
			if err != nil {
				b.Fatalf("Route failed: %v", err)
			}
			if result == nil {
				b.Fatal("Route returned nil result")
			}
		}
	})

	b.Run("MultipleParams", func(b *testing.B) {
		req := &mockRequest{method: "GET", path: "/api/posts/456/comments/789"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := router.Route(ctx, req)
			if err != nil {
				b.Fatalf("Route failed: %v", err)
			}
			if result == nil {
				b.Fatal("Route returned nil result")
			}
		}
	})

	b.Run("Wildcard", func(b *testing.B) {
		req := &mockRequest{method: "GET", path: "/api/files/docs/2024/report.pdf"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := router.Route(ctx, req)
			if err != nil {
				b.Fatalf("Route failed: %v", err)
			}
			if result == nil {
				b.Fatal("Route returned nil result")
			}
		}
	})
}

func BenchmarkRouterAddRule(b *testing.B) {
	registry := &mockRegistry{
		services: make(map[string][]core.ServiceInstance),
	}

	b.ResetTimer()
	b.Run("AddRules", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			router := NewRouter(registry)
			b.StartTimer()

			for j := 0; j < 100; j++ {
				rule := core.RouteRule{
					ID:          fmt.Sprintf("rule-%d", j),
					Path:        fmt.Sprintf("/api/endpoint-%d", j),
					ServiceName: "test-service",
				}
				if err := router.AddRule(rule); err != nil {
					b.Fatalf("Failed to add rule: %v", err)
				}
			}
		}
	})
}

func BenchmarkRouterConcurrentRoute(b *testing.B) {
	registry := &mockRegistry{
		services: map[string][]core.ServiceInstance{
			"concurrent-service": {
				{ID: "concurrent-1", Address: "127.0.0.1", Port: 8001, Healthy: true},
				{ID: "concurrent-2", Address: "127.0.0.1", Port: 8002, Healthy: true},
			},
		},
	}

	router := NewRouter(registry)

	// Add some routes
	for i := 0; i < 10; i++ {
		rule := core.RouteRule{
			ID:          fmt.Sprintf("concurrent-%d", i),
			Path:        fmt.Sprintf("/api/concurrent/%d", i),
			ServiceName: "concurrent-service",
		}
		if err := router.AddRule(rule); err != nil {
			b.Fatalf("Failed to add rule: %v", err)
		}
	}

	ctx := context.Background()
	req := &mockRequest{method: "GET", path: "/api/concurrent/5"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := router.Route(ctx, req)
			if err != nil {
				b.Fatalf("Route failed: %v", err)
			}
			if result == nil {
				b.Fatal("Route returned nil result")
			}
		}
	})
}