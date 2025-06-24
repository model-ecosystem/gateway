package cors

import (
	"fmt"
	
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "cors"

// Component implements factory.Component for CORS middleware
type Component struct {
	config Config
	cors   *CORS
}

// NewComponent creates a new CORS component
func NewComponent() factory.Component {
	return &Component{}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Start with default configuration
	config := defaultConfig()
	
	// Parse user configuration
	if err := parser(&config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	
	// Store configuration and create CORS instance
	c.config = config
	c.cors = newCORS(config)
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.cors == nil {
		return fmt.Errorf("cors not initialized")
	}
	return nil
}

// Build returns the middleware function
func (c *Component) Build() core.Middleware {
	if c.cors == nil {
		panic("Component not initialized")
	}
	
	// Return the same middleware as the original Middleware function
	return Middleware(c.config)
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)