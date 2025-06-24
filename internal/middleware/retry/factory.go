package retry

import (
	"fmt"
	"log/slog"
	"time"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
	"gateway/pkg/retry"
)

// ComponentName is the name used to register this component
const ComponentName = "retry"

// Component implements factory.Component for retry middleware
type Component struct {
	config     Config
	middleware *Middleware
	logger     *slog.Logger
}

// NewComponent creates a new retry component
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
	// Parse the retry configuration
	var retryConfig config.Retry
	if err := parser(&retryConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Skip if retry is not enabled
	if !retryConfig.Enabled {
		return fmt.Errorf("retry middleware is not enabled")
	}
	
	// Convert config to middleware config
	c.config = Config{
		Default:  convertRetryConfig(retryConfig.Default),
		Routes:   make(map[string]retry.Config),
		Services: make(map[string]retry.Config),
	}
	
	// Convert route-specific configs
	for route, routeCfg := range retryConfig.Routes {
		c.config.Routes[route] = convertRetryConfig(routeCfg)
	}
	
	// Convert service-specific configs
	for service, serviceCfg := range retryConfig.Services {
		c.config.Services[service] = convertRetryConfig(serviceCfg)
	}
	
	// Create middleware with budget
	budgetRatio := retryConfig.Default.BudgetRatio
	if budgetRatio <= 0 {
		budgetRatio = 0.1 // Default to 10%
	}
	
	c.middleware = NewWithBudget(c.config, budgetRatio, c.logger)
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.middleware == nil {
		return fmt.Errorf("middleware not initialized")
	}
	
	// Validate default config
	if c.config.Default.MaxAttempts <= 0 {
		return fmt.Errorf("invalid max attempts in default config")
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

// convertRetryConfig converts from config to internal retry config
func convertRetryConfig(cfg config.RetryConfig) retry.Config {
	// Set defaults if not specified
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.InitialDelay == 0 {
		cfg.InitialDelay = 100 // 100ms
	}
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = 30000 // 30s
	}
	if cfg.Multiplier == 0 {
		cfg.Multiplier = 2.0
	}
	
	return retry.Config{
		MaxAttempts:   cfg.MaxAttempts,
		InitialDelay:  time.Duration(cfg.InitialDelay) * time.Millisecond,
		MaxDelay:      time.Duration(cfg.MaxDelay) * time.Millisecond,
		Multiplier:    cfg.Multiplier,
		Jitter:        cfg.Jitter,
		RetryableFunc: retry.DefaultRetryableFunc,
	}
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)