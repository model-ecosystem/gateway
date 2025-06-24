package sse

import (
	"fmt"
	"log/slog"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "sse-adapter"

// Component implements factory.Component for SSE adapter
type Component struct {
	config  *Config
	adapter *Adapter
	handler core.Handler
	logger  *slog.Logger
}

// NewComponent creates a new SSE adapter component
func NewComponent(handler core.Handler, logger *slog.Logger) factory.Component {
	return &Component{
		handler: handler,
		logger:  logger,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse the SSE configuration
	var sseConfig config.SSE
	if err := parser(&sseConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Skip if not enabled
	if !sseConfig.Enabled {
		return fmt.Errorf("SSE adapter is not enabled")
	}
	
	// Convert to internal config
	c.config = &Config{
		Enabled:          sseConfig.Enabled,
		WriteTimeout:     sseConfig.WriteTimeout,
		KeepaliveTimeout: sseConfig.KeepaliveTimeout,
	}
	
	// Set defaults
	if c.config.WriteTimeout == 0 {
		c.config.WriteTimeout = 60
	}
	if c.config.KeepaliveTimeout == 0 {
		c.config.KeepaliveTimeout = 30
	}
	
	// Create adapter
	c.adapter = NewAdapter(c.config, c.handler, c.logger)
	
	// Note: Token validation configuration would be set through a separate method
	// if the adapter supports it
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.adapter == nil {
		return fmt.Errorf("SSE adapter not initialized")
	}
	
	// Validate configuration
	if c.config.WriteTimeout < 0 {
		return fmt.Errorf("invalid write timeout")
	}
	if c.config.KeepaliveTimeout < 0 {
		return fmt.Errorf("invalid keepalive timeout")
	}
	
	return nil
}

// Build returns the adapter
func (c *Component) Build() *Adapter {
	if c.adapter == nil {
		panic("Component not initialized")
	}
	return c.adapter
}

// GetConfig returns the configuration (for testing)
func (c *Component) GetConfig() *Config {
	return c.config
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)