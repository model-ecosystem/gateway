package metrics

import (
	"fmt"
	"log/slog"

	"gateway/internal/core"
	"gateway/internal/metrics"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "metrics-middleware"

// Component implements factory.Component for metrics middleware
type Component struct {
	metrics    *metrics.Metrics
	middleware core.Middleware
	logger     *slog.Logger
}

// NewComponent creates a new metrics middleware component
func NewComponent(metricsInstance *metrics.Metrics, logger *slog.Logger) factory.Component {
	return &Component{
		metrics: metricsInstance,
		logger:  logger,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Metrics middleware doesn't need configuration
	// It uses the metrics instance passed in constructor
	
	if c.metrics == nil {
		return fmt.Errorf("metrics instance not provided")
	}
	
	// Create the middleware
	c.middleware = Middleware(c.metrics)
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.metrics == nil {
		return fmt.Errorf("metrics instance not initialized")
	}
	if c.middleware == nil {
		return fmt.Errorf("middleware function not initialized")
	}
	return nil
}

// Build returns the middleware
func (c *Component) Build() core.Middleware {
	if c.middleware == nil {
		panic("Component not initialized")
	}
	return c.middleware
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)