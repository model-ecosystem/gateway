package ratelimit

import (
	"fmt"
	"log/slog"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/storage"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "ratelimit"

// Component implements factory.Component for rate limit middleware
type Component struct {
	routeConfigs map[string]*Config
	store        storage.LimiterStore
	logger       *slog.Logger
}

// NewComponent creates a new rate limit component
func NewComponent(logger *slog.Logger, store storage.LimiterStore) factory.Component {
	return &Component{
		logger: logger,
		store:  store,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse router configuration to get rate limit rules
	var routerConfig config.Router
	if err := parser(&routerConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Check if any route has rate limiting enabled
	hasRateLimit := false
	c.routeConfigs = make(map[string]*Config)
	
	for _, rule := range routerConfig.Rules {
		if rule.RateLimit > 0 {
			hasRateLimit = true
			c.routeConfigs[rule.Path] = &Config{
				Rate:   rule.RateLimit,
				Burst:  rule.RateLimitBurst,
				Store:  c.store,
				Logger: c.logger,
			}
		}
	}
	
	if !hasRateLimit {
		return fmt.Errorf("no rate limit rules configured")
	}
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if len(c.routeConfigs) == 0 {
		return fmt.Errorf("no route configurations")
	}
	
	for path, cfg := range c.routeConfigs {
		if cfg.Rate <= 0 {
			return fmt.Errorf("invalid rate for path %s", path)
		}
		if cfg.Burst < cfg.Rate {
			cfg.Burst = cfg.Rate // Set burst to at least rate
		}
	}
	
	return nil
}

// Build returns the middleware handler
func (c *Component) Build() core.Middleware {
	if len(c.routeConfigs) == 0 {
		panic("Component not initialized")
	}
	
	return PerRoute(c.routeConfigs)
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)