package circuitbreaker

import (
	"fmt"
	"log/slog"
	"time"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/circuitbreaker"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "circuitbreaker"

// Component implements factory.Component for circuit breaker middleware
type Component struct {
	config     Config
	middleware *Middleware
	logger     *slog.Logger
}

// NewComponent creates a new circuit breaker component
func NewComponent(logger *slog.Logger) factory.Component {
	return &Component{
		logger: logger,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse the circuit breaker configuration
	var cbConfig config.CircuitBreaker
	if err := parser(&cbConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Skip if circuit breaker is not enabled
	if !cbConfig.Enabled {
		return fmt.Errorf("circuit breaker middleware is not enabled")
	}
	
	// Convert config to middleware config
	c.config = Config{
		Default:  convertCircuitBreakerConfig(cbConfig.Default),
		Routes:   make(map[string]circuitbreaker.Config),
		Services: make(map[string]circuitbreaker.Config),
	}
	
	// Convert route-specific configs
	for route, routeCfg := range cbConfig.Routes {
		c.config.Routes[route] = convertCircuitBreakerConfig(routeCfg)
	}
	
	// Convert service-specific configs
	for service, serviceCfg := range cbConfig.Services {
		c.config.Services[service] = convertCircuitBreakerConfig(serviceCfg)
	}
	
	// Create middleware
	c.middleware = New(c.config, c.logger)
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.middleware == nil {
		return fmt.Errorf("middleware not initialized")
	}
	
	// Validate default config
	if c.config.Default.MaxFailures <= 0 {
		return fmt.Errorf("invalid max failures in default config")
	}
	
	return nil
}

// Build returns the middleware handler
func (c *Component) Build() core.Middleware {
	if c.middleware == nil {
		panic("Component not initialized")
	}
	
	return c.middleware.Apply()
}

// convertCircuitBreakerConfig converts from config to internal circuit breaker config
func convertCircuitBreakerConfig(cfg config.CircuitBreakerConfig) circuitbreaker.Config {
	// Set defaults if not specified
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.FailureThreshold <= 0 || cfg.FailureThreshold > 1 {
		cfg.FailureThreshold = 0.5
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60
	}
	if cfg.MaxRequests <= 0 {
		cfg.MaxRequests = 1
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 60
	}
	
	return circuitbreaker.Config{
		MaxFailures:      cfg.MaxFailures,
		FailureThreshold: cfg.FailureThreshold,
		Timeout:          time.Duration(cfg.Timeout) * time.Second,
		MaxRequests:      cfg.MaxRequests,
		Interval:         time.Duration(cfg.Interval) * time.Second,
	}
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)