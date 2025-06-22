package factory

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/health"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// allowedExecCommands is a whitelist of allowed commands for exec health checks
// This prevents arbitrary command execution vulnerabilities
var allowedExecCommands = map[string][]string{
	"check-db-postgres": {"/usr/bin/pg_isready", "-h", "localhost"},
	"check-db-mysql":    {"/usr/bin/mysqladmin", "ping"},
	"check-disk-space":  {"/bin/df", "-h", "/"},
	"check-memory":      {"/usr/bin/free", "-m"},
	"check-redis":       {"/usr/bin/redis-cli", "ping"},
	"check-custom-app":  {"/usr/local/bin/health-check"},
}

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

	case "grpc":
		addr := cfg.Config["address"]
		if addr == "" {
			return nil, fmt.Errorf("grpc check requires 'address' in config")
		}
		service := cfg.Config["service"]
		// Default to standard health service if not specified
		if service == "" {
			service = "grpc.health.v1.Health"
		}
		return grpcCheck(addr, service, timeout), nil

	case "exec":
		commandKey := cfg.Config["command"]
		if commandKey == "" {
			return nil, fmt.Errorf("exec check requires 'command' in config")
		}
		// Check if command is in whitelist
		cmdArgs, ok := allowedExecCommands[commandKey]
		if !ok {
			return nil, fmt.Errorf("exec command '%s' is not in whitelist", commandKey)
		}
		// Parse any additional args from config
		args := cfg.Config["args"]
		if args != "" {
			// Note: In production, you'd want to properly parse and validate these args
			// For now, we'll just append them
			cmdArgs = append(cmdArgs, args)
		}
		return execCheck(cmdArgs, timeout), nil

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

// grpcCheck creates a gRPC health check
func grpcCheck(addr string, service string, timeout time.Duration) health.Check {
	return func(ctx context.Context) error {
		// Create a timeout context
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Connect to gRPC server
		conn, err := grpc.DialContext(checkCtx, addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return fmt.Errorf("grpc dial failed: %w", err)
		}
		defer conn.Close()

		// Create health check client
		client := healthpb.NewHealthClient(conn)

		// Perform health check
		resp, err := client.Check(checkCtx, &healthpb.HealthCheckRequest{
			Service: service,
		})
		if err != nil {
			return fmt.Errorf("grpc health check failed: %w", err)
		}

		// Check response status
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			return fmt.Errorf("grpc service unhealthy: %s", resp.Status.String())
		}

		return nil
	}
}

// execCheck creates an exec command health check
func execCheck(cmdArgs []string, timeout time.Duration) health.Check {
	return func(ctx context.Context) error {
		if len(cmdArgs) == 0 {
			return fmt.Errorf("exec check requires command arguments")
		}

		// Create command with timeout context
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Create command - first arg is the command, rest are arguments
		cmd := exec.CommandContext(checkCtx, cmdArgs[0], cmdArgs[1:]...)

		// Run the command and check exit code
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Check if it was a timeout
			if checkCtx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("exec check timed out after %v", timeout)
			}
			// Check if it was an exit error
			if exitErr, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("exec check failed with exit code %d: %s", exitErr.ExitCode(), string(output))
			}
			return fmt.Errorf("exec check failed: %w", err)
		}

		// Command succeeded
		return nil
	}
}
