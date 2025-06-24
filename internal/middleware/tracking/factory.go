package tracking

import (
	"fmt"
	"log/slog"

	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "tracking-middleware"

// Component implements factory.Component for tracking middleware
type Component struct {
	middleware *Middleware
	logger     *slog.Logger
}

// NewComponent creates a new tracking middleware component
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
	// Tracking middleware doesn't need configuration
	c.middleware = NewMiddleware()
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.middleware == nil {
		return fmt.Errorf("tracking middleware not initialized")
	}
	return nil
}

// Build returns the middleware
func (c *Component) Build() *Middleware {
	if c.middleware == nil {
		panic("Component not initialized")
	}
	return c.middleware
}

// BuildHandler wraps a handler with tracking
func (c *Component) BuildHandler(name string, handler core.Handler) core.Handler {
	if c.middleware == nil {
		panic("Component not initialized")
	}
	return c.middleware.WrapHandler(name, handler)
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)