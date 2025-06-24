package router

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "router"

// Component implements factory.Component for router
type Component struct {
	config   *config.Router
	registry core.ServiceRegistry
	router   *Router
	logger   *slog.Logger
}

// NewComponent creates a new router component
func NewComponent(registry core.ServiceRegistry, logger *slog.Logger) factory.Component {
	return &Component{
		registry: registry,
		logger:   logger,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse the router configuration
	var routerConfig config.Router
	if err := parser(&routerConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	c.config = &routerConfig

	// Create router
	router := NewRouter(c.registry, c.logger)

	// Add all configured routes
	for _, rule := range c.config.Rules {
		if err := router.AddRule(rule.ToRouteRule()); err != nil {
			return fmt.Errorf("add route %s: %w", rule.ID, err)
		}
	}

	c.router = router

	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.router == nil {
		return fmt.Errorf("router not initialized")
	}
	if c.registry == nil {
		return fmt.Errorf("service registry not provided")
	}
	if len(c.config.Rules) == 0 {
		return fmt.Errorf("no routes configured")
	}
	return nil
}

// Build returns the router
func (c *Component) Build() core.Router {
	if c.router == nil {
		panic("Component not initialized")
	}
	return c.router
}

// Close closes the router and cleans up resources
func (c *Component) Close() error {
	if c.router != nil {
		return c.router.Close()
	}
	return nil
}

// GetRouter returns the concrete router implementation
func (c *Component) GetRouter() *Router {
	return c.router
}

// CreateFromRules creates a router from a list of route rules
func CreateFromRules(registry core.ServiceRegistry, rules []core.RouteRule, logger *slog.Logger) (core.Router, error) {
	r := NewRouter(registry, logger)

	for _, rule := range rules {
		if err := r.AddRule(rule); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)