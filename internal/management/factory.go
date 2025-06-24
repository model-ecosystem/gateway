package management

import (
	"context"
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "management-api"

// Component implements factory.Component for management API
type Component struct {
	config *config.Management
	api    *API
	logger *slog.Logger
}

// NewComponent creates a new management API component
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
	// Parse the management configuration
	var mgmtConfig config.Management
	if err := parser(&mgmtConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	c.config = &mgmtConfig

	// Only create API if enabled
	if c.config.Enabled {
		c.api = NewAPI(c.config, c.logger)
	}

	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.config == nil {
		return fmt.Errorf("management config not initialized")
	}
	
	// API can be nil if management is disabled
	if c.config.Enabled && c.api == nil {
		return fmt.Errorf("management enabled but API not created")
	}

	// Validate configuration
	if c.config.Enabled {
		if c.config.Port == 0 {
			return fmt.Errorf("management port not specified")
		}
		
		// Validate auth configuration if present
		if c.config.Auth != nil {
			switch c.config.Auth.Type {
			case "token":
				if c.config.Auth.Token == "" {
					return fmt.Errorf("management auth token not specified")
				}
			case "basic":
				if len(c.config.Auth.Users) == 0 {
					return fmt.Errorf("no users configured for basic auth")
				}
			default:
				return fmt.Errorf("invalid auth type: %s", c.config.Auth.Type)
			}
		}
	}

	return nil
}

// Build returns the management API
func (c *Component) Build() *API {
	// Can return nil if management is disabled
	return c.api
}

// Start starts the management API server
func (c *Component) Start() error {
	if c.api != nil {
		ctx := context.Background()
		return c.api.Start(ctx)
	}
	return nil
}

// Stop stops the management API server
func (c *Component) Stop() error {
	if c.api != nil {
		ctx := context.Background()
		return c.api.Stop(ctx)
	}
	return nil
}

// IsEnabled returns whether management API is enabled
func (c *Component) IsEnabled() bool {
	return c.config != nil && c.config.Enabled
}

// Ensure Component implements factory.Component and factory.Lifecycle
var (
	_ factory.Component = (*Component)(nil)
	_ factory.Lifecycle = (*Component)(nil)
)