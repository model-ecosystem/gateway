package auth

import (
	"fmt"
	"log/slog"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "auth"

// Component implements factory.Component for auth middleware
type Component struct {
	config     *config.Auth
	middleware *Middleware
	logger     *slog.Logger
}

// NewComponent creates a new auth component
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
	// Parse the auth configuration
	var authConfig config.Auth
	if err := parser(&authConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Store config for later use
	c.config = &authConfig
	
	// Validate configuration
	if len(authConfig.Providers) == 0 {
		return fmt.Errorf("no auth providers configured")
	}
	
	// Configuration will be used to create middleware during Build
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.config == nil {
		return fmt.Errorf("configuration not initialized")
	}
	
	// Check that at least one provider is configured and enabled
	hasEnabledProvider := false
	for _, providerName := range c.config.Providers {
		switch providerName {
		case "jwt":
			if c.config.JWT != nil && c.config.JWT.Enabled {
				hasEnabledProvider = true
			}
		case "apikey":
			if c.config.APIKey != nil && c.config.APIKey.Enabled {
				hasEnabledProvider = true
			}
		}
	}
	
	if !hasEnabledProvider {
		return fmt.Errorf("no enabled providers found")
	}
	
	return nil
}

// Build returns the middleware handler
func (c *Component) Build() core.Middleware {
	if c.config == nil {
		panic("Component not initialized")
	}
	
	// Create middleware config
	authConfig := &Config{
		Required:       c.config.Required,
		Providers:      c.config.Providers,
		SkipPaths:      c.config.SkipPaths,
		RequiredScopes: c.config.RequiredScopes,
		StoreAuthInfo:  true,
	}
	
	// Create middleware instance
	c.middleware = NewMiddleware(authConfig, c.logger)
	
	// Note: In a real implementation, we would initialize providers here
	// For now, we return a middleware that will need providers added externally
	
	return c.middleware.Handler
}

// GetMiddleware returns the created middleware instance for external provider setup
func (c *Component) GetMiddleware() *Middleware {
	return c.middleware
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)