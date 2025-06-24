package recovery

import (
	"fmt"
	"log/slog"

	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "recovery-middleware"

// Component implements factory.Component for recovery middleware
type Component struct {
	middleware core.Middleware
	logger     *slog.Logger
}

// NewComponent creates a new recovery middleware component
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
	// Recovery middleware doesn't need configuration
	// Create the middleware with the logger
	c.middleware = Default(c.logger)
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.middleware == nil {
		return fmt.Errorf("recovery middleware not initialized")
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