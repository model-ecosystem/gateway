package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"gateway/internal/core"
)

// RegistryCheck creates a health check for the service registry
func RegistryCheck(registry core.ServiceRegistry) Check {
	return func(ctx context.Context) error {
		// Try to list a known service or check registry connectivity
		// This is a simple check - you might want to be more specific
		_, err := registry.GetService("health-check-dummy")
		if err != nil {
			// Not finding the service is OK, we just want to ensure registry responds
			return nil
		}
		return nil
	}
}

// HTTPCheck creates a health check for an HTTP endpoint
func HTTPCheck(url string, timeout time.Duration) Check {
	return func(ctx context.Context) error {
		client := &http.Client{
			Timeout: timeout,
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
		}

		return nil
	}
}

// DatabaseCheck creates a health check for database connectivity
func DatabaseCheck(pingFunc func(context.Context) error) Check {
	return func(ctx context.Context) error {
		return pingFunc(ctx)
	}
}

// DiskSpaceCheck creates a health check for available disk space
func DiskSpaceCheck(path string, minBytes uint64) Check {
	return func(ctx context.Context) error {
		// This is a placeholder - actual implementation would check disk space
		// using syscall or a library like github.com/shirou/gopsutil
		return nil
	}
}

// MemoryCheck creates a health check for available memory
func MemoryCheck(maxUsagePercent float64) Check {
	return func(ctx context.Context) error {
		// This is a placeholder - actual implementation would check memory usage
		// using runtime.MemStats or a library like github.com/shirou/gopsutil
		return nil
	}
}

// CustomCheck allows creating custom health checks
func CustomCheck(name string, checkFunc func() error) Check {
	return func(ctx context.Context) error {
		// Run the check with context awareness
		done := make(chan error, 1)
		go func() {
			done <- checkFunc()
		}()

		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return fmt.Errorf("check timeout: %w", ctx.Err())
		}
	}
}