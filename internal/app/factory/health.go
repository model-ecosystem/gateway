package factory

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/health"
)

// CreateHealthChecker creates a health checker with configured checks
func CreateHealthChecker(cfg *config.Health, registry core.ServiceRegistry, logger *slog.Logger) (*health.Checker, error) {
	checker := health.NewChecker()

	// Always register basic checks
	checker.RegisterCheck("gateway", func(ctx context.Context) error {
		// Basic self-check - always healthy if we can execute
		return nil
	})

	// Register registry check
	if registry != nil {
		checker.RegisterCheck("registry", health.RegistryCheck(registry))
	}

	// Register configured checks
	if cfg != nil && cfg.Checks != nil {
		for name, checkCfg := range cfg.Checks {
			check, err := createCheck(checkCfg)
			if err != nil {
				logger.Warn("Failed to create health check",
					"name", name,
					"type", checkCfg.Type,
					"error", err,
				)
				continue
			}
			checker.RegisterCheck(name, check)
		}
	}

	return checker, nil
}

// CreateHealthHandler creates the health check HTTP handler
func CreateHealthHandler(cfg *config.Health, checker *health.Checker, version, serviceID string) *health.Handler {
	return health.NewHandler(checker, version, serviceID)
}

// createCheck creates a specific type of health check
func createCheck(cfg config.Check) (health.Check, error) {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	switch cfg.Type {
	case "http":
		url := cfg.Config["url"]
		if url == "" {
			return nil, fmt.Errorf("http check requires 'url' in config")
		}
		return health.HTTPCheck(url, timeout), nil

	case "tcp":
		addr := cfg.Config["address"]
		if addr == "" {
			return nil, fmt.Errorf("tcp check requires 'address' in config")
		}
		return tcpCheck(addr, timeout), nil

	default:
		return nil, fmt.Errorf("unknown check type: %s", cfg.Type)
	}
}

// tcpCheck creates a TCP connectivity check
func tcpCheck(addr string, timeout time.Duration) health.Check {
	return func(ctx context.Context) error {
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("tcp dial failed: %w", err)
		}
		conn.Close()
		return nil
	}
}