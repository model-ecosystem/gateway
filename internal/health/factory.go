package health

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// ComponentName is the name used to register this component
const ComponentName = "health"

// Component implements factory.Component for health checking
type Component struct {
	config          *config.Health
	registry        core.ServiceRegistry
	checker         *Checker
	handler         *Handler
	backendMonitor  *BackendMonitor
	logger          *slog.Logger
	version         string
	serviceID       string
}

// NewComponent creates a new health component
func NewComponent(registry core.ServiceRegistry, logger *slog.Logger, version, serviceID string) factory.Component {
	return &Component{
		registry:  registry,
		logger:    logger,
		version:   version,
		serviceID: serviceID,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse the health configuration
	var healthConfig config.Health
	if err := parser(&healthConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	c.config = &healthConfig

	// Create health checker
	checker, err := c.createHealthChecker()
	if err != nil {
		return fmt.Errorf("create health checker: %w", err)
	}
	c.checker = checker

	// Create health handler
	c.handler = NewHandler(c.checker, c.version, c.serviceID)

	// Create backend monitor if active health checking is enabled
	if c.config.Enabled && c.registry != nil {
		monitor, err := c.createBackendMonitor()
		if err != nil {
			return fmt.Errorf("create backend monitor: %w", err)
		}
		c.backendMonitor = monitor
	}

	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.checker == nil {
		return fmt.Errorf("health checker not initialized")
	}
	if c.handler == nil {
		return fmt.Errorf("health handler not initialized")
	}
	// Backend monitor is optional
	return nil
}

// Build returns the health handler
func (c *Component) Build() *Handler {
	if c.handler == nil {
		panic("Component not initialized")
	}
	return c.handler
}

// GetChecker returns the health checker
func (c *Component) GetChecker() *Checker {
	return c.checker
}

// GetBackendMonitor returns the backend monitor
func (c *Component) GetBackendMonitor() *BackendMonitor {
	return c.backendMonitor
}

// Start starts the backend monitor if configured
func (c *Component) Start() error {
	if c.backendMonitor != nil {
		ctx := context.Background()
		return c.backendMonitor.Start(ctx)
	}
	return nil
}

// Stop stops the backend monitor if configured
func (c *Component) Stop() error {
	if c.backendMonitor != nil {
		return c.backendMonitor.Stop()
	}
	return nil
}

// createHealthChecker creates a health checker with configured checks
func (c *Component) createHealthChecker() (*Checker, error) {
	checker := NewChecker()

	// Always register basic checks
	checker.RegisterCheck("gateway", func(ctx context.Context) error {
		// Basic self-check - always healthy if we can execute
		return nil
	})

	// Register registry check
	if c.registry != nil {
		checker.RegisterCheck("registry", RegistryCheck(c.registry))
	}

	// Register configured checks
	if c.config != nil && c.config.Checks != nil {
		for name, checkCfg := range c.config.Checks {
			check, err := c.createCheck(checkCfg)
			if err != nil {
				c.logger.Warn("Failed to create health check",
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

// createBackendMonitor creates a backend health monitor
func (c *Component) createBackendMonitor() (*BackendMonitor, error) {
	monitor := NewBackendMonitor(c.registry, c.config, c.logger)
	
	// The monitor will use the configured health checks to monitor backend services
	// It provides active health checking and updates service registry with health status
	
	return monitor, nil
}

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

// createCheck creates a specific type of health check
func (c *Component) createCheck(cfg config.Check) (Check, error) {
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
		return HTTPCheck(url, timeout), nil

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
func tcpCheck(addr string, timeout time.Duration) Check {
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
func grpcCheck(addr string, service string, timeout time.Duration) Check {
	return func(ctx context.Context) error {
		// Create a timeout context
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Connect to gRPC server
		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return fmt.Errorf("grpc client creation failed: %w", err)
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
func execCheck(cmdArgs []string, timeout time.Duration) Check {
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

// Ensure Component implements factory.Component and factory.Lifecycle
var (
	_ factory.Component = (*Component)(nil)
	_ factory.Lifecycle = (*Component)(nil)
)