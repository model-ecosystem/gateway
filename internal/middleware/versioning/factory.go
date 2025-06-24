package versioning

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "versioning-middleware"

// Component implements factory.Component for versioning middleware
type Component struct {
	config     *config.VersioningConfig
	middleware *VersioningMiddleware
	logger     *slog.Logger
}

// NewComponent creates a new versioning middleware component
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
	// Parse the versioning configuration
	var versioningConfig config.VersioningConfig
	if err := parser(&versioningConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	c.config = &versioningConfig

	// Only create middleware if enabled
	if c.config.Enabled {
		c.middleware = NewVersioningMiddleware(c.config, c.logger)
	}

	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.config == nil {
		return fmt.Errorf("versioning config not initialized")
	}
	
	// Middleware can be nil if versioning is disabled
	if c.config.Enabled && c.middleware == nil {
		return fmt.Errorf("versioning enabled but middleware not created")
	}

	// Validate configuration
	if c.config.Enabled {
		if c.config.Strategy == "" {
			return fmt.Errorf("versioning strategy not specified")
		}
		
		validStrategies := map[string]bool{
			"path":   true,
			"header": true,
			"query":  true,
			"accept": true,
		}
		
		if !validStrategies[c.config.Strategy] {
			return fmt.Errorf("invalid versioning strategy: %s", c.config.Strategy)
		}
		
		if c.config.DefaultVersion == "" {
			return fmt.Errorf("default version not specified")
		}
	}

	return nil
}

// Build returns the versioning middleware
func (c *Component) Build() *VersioningMiddleware {
	// Can return nil if versioning is disabled
	return c.middleware
}

// IsEnabled returns whether versioning is enabled
func (c *Component) IsEnabled() bool {
	return c.config != nil && c.config.Enabled
}

// GetRouteModifier returns a route modifier if versioning is enabled
func (c *Component) GetRouteModifier() *VersionRouteModifier {
	if c.IsEnabled() {
		return NewVersionRouteModifier(c.config)
	}
	return nil
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)