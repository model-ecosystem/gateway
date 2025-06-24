package static

import (
	"fmt"
	"log/slog"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "static-registry"

// Component implements factory.Component for static registry
type Component struct {
	config   *config.StaticRegistry
	registry *Registry
	logger   *slog.Logger
}

// NewComponent creates a new static registry component
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
	// Parse the static registry configuration
	var staticConfig config.StaticRegistry
	if err := parser(&staticConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	c.config = &staticConfig
	
	// Create registry
	registry, err := NewRegistry(&staticConfig)
	if err != nil {
		return fmt.Errorf("create static registry: %w", err)
	}
	c.registry = registry
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.registry == nil {
		return fmt.Errorf("static registry not initialized")
	}
	
	// Validate that at least one service is configured
	if c.config == nil || len(c.config.Services) == 0 {
		return fmt.Errorf("no services configured")
	}
	
	// Validate each service has at least one instance
	for _, service := range c.config.Services {
		if len(service.Instances) == 0 {
			return fmt.Errorf("service %s has no instances", service.Name)
		}
	}
	
	return nil
}

// Build returns the registry
func (c *Component) Build() core.ServiceRegistry {
	if c.registry == nil {
		panic("Component not initialized")
	}
	return c.registry
}

// GetRegistry returns the static registry (for specific use cases)
func (c *Component) GetRegistry() *Registry {
	return c.registry
}

// HealthAwareComponent implements factory.Component for health-aware static registry
type HealthAwareComponent struct {
	Component
	healthAwareRegistry *HealthAwareRegistry
}

// NewHealthAwareComponent creates a new health-aware static registry component
func NewHealthAwareComponent(logger *slog.Logger) factory.Component {
	return &HealthAwareComponent{
		Component: Component{
			logger: logger,
		},
	}
}

// Name returns the component name
func (c *HealthAwareComponent) Name() string {
	return "health-aware-static-registry"
}

// Init initializes the health-aware component
func (c *HealthAwareComponent) Init(parser factory.ConfigParser) error {
	// First initialize the base component
	if err := c.Component.Init(parser); err != nil {
		return err
	}
	
	// Create health-aware wrapper
	healthAware, err := NewHealthAwareRegistry(c.config)
	if err != nil {
		return fmt.Errorf("create health-aware registry: %w", err)
	}
	c.healthAwareRegistry = healthAware
	
	return nil
}

// Build returns the health-aware registry
func (c *HealthAwareComponent) Build() core.ServiceRegistry {
	if c.healthAwareRegistry == nil {
		panic("Component not initialized")
	}
	return c.healthAwareRegistry
}

// GetHealthAwareRegistry returns the health-aware registry
func (c *HealthAwareComponent) GetHealthAwareRegistry() *HealthAwareRegistry {
	return c.healthAwareRegistry
}

// Ensure components implement factory.Component
var (
	_ factory.Component = (*Component)(nil)
	_ factory.Component = (*HealthAwareComponent)(nil)
)