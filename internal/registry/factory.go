package registry

import (
	"fmt"
	"log/slog"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/registry/docker"
	"gateway/internal/registry/dockercompose"
	"gateway/internal/registry/static"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "service-registry"

// Component implements factory.Component for service registry selection
type Component struct {
	registryType string
	registry     core.ServiceRegistry
	logger       *slog.Logger
	// For lifecycle management
	lifecycle factory.Lifecycle
}

// NewComponent creates a new registry component
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
	// Parse the registry configuration
	var registryConfig config.Registry
	if err := parser(&registryConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	c.registryType = registryConfig.Type
	if c.registryType == "" {
		c.registryType = "static" // Default to static
	}
	
	// Create appropriate registry based on type
	switch c.registryType {
	case "static":
		component := static.NewComponent(c.logger)
		if err := component.Init(configParser(registryConfig.Static)); err != nil {
			return fmt.Errorf("init static registry: %w", err)
		}
		if err := component.Validate(); err != nil {
			return fmt.Errorf("validate static registry: %w", err)
		}
		c.registry = component.(*static.Component).Build()
		
	case "docker":
		component := docker.NewComponent(c.logger)
		if err := component.Init(configParser(registryConfig.Docker)); err != nil {
			return fmt.Errorf("init docker registry: %w", err)
		}
		if err := component.Validate(); err != nil {
			return fmt.Errorf("validate docker registry: %w", err)
		}
		c.registry = component.(*docker.Component).Build()
		c.lifecycle = component.(factory.Lifecycle)
		
	case "docker-compose":
		component := dockercompose.NewComponent(c.logger)
		if err := component.Init(configParser(registryConfig.DockerCompose)); err != nil {
			return fmt.Errorf("init docker-compose registry: %w", err)
		}
		if err := component.Validate(); err != nil {
			return fmt.Errorf("validate docker-compose registry: %w", err)
		}
		c.registry = component.(*dockercompose.Component).Build()
		c.lifecycle = component.(factory.Lifecycle)
		
	default:
		return fmt.Errorf("unknown registry type: %s", c.registryType)
	}
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.registry == nil {
		return fmt.Errorf("registry not initialized")
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

// Start starts the registry if it implements Lifecycle
func (c *Component) Start() error {
	if c.lifecycle != nil {
		return c.lifecycle.Start()
	}
	return nil
}

// Stop stops the registry if it implements Lifecycle
func (c *Component) Stop() error {
	if c.lifecycle != nil {
		return c.lifecycle.Stop()
	}
	return nil
}

// configParser creates a ConfigParser for a specific config object
func configParser(cfg interface{}) factory.ConfigParser {
	return func(v interface{}) error {
		// Simple assignment since we already have the parsed config
		switch target := v.(type) {
		case *config.StaticRegistry:
			if src, ok := cfg.(*config.StaticRegistry); ok && src != nil {
				*target = *src
				return nil
			}
			return fmt.Errorf("invalid static registry config")
		case *config.DockerRegistry:
			if src, ok := cfg.(*config.DockerRegistry); ok && src != nil {
				*target = *src
				return nil
			}
			return fmt.Errorf("invalid docker registry config")
		case *config.DockerComposeRegistry:
			if src, ok := cfg.(*config.DockerComposeRegistry); ok && src != nil {
				*target = *src
				return nil
			}
			return fmt.Errorf("invalid docker-compose registry config")
		default:
			return fmt.Errorf("unsupported config type: %T", v)
		}
	}
}

// HealthAwareComponent creates a health-aware registry wrapper
type HealthAwareComponent struct {
	Component
}

// NewHealthAwareComponent creates a new health-aware registry component
func NewHealthAwareComponent(logger *slog.Logger) factory.Component {
	return &HealthAwareComponent{
		Component: Component{
			logger: logger,
		},
	}
}

// Name returns the component name
func (c *HealthAwareComponent) Name() string {
	return "health-aware-" + ComponentName
}

// Init initializes with health-aware wrapper
func (c *HealthAwareComponent) Init(parser factory.ConfigParser) error {
	// First initialize the base registry
	if err := c.Component.Init(parser); err != nil {
		return err
	}
	
	// For static registry, wrap with health-aware version
	if c.registryType == "static" {
		// The static registry already has health-aware support built-in
		// For other registry types, they handle health internally
	}
	
	return nil
}

// Ensure components implement factory.Component
var (
	_ factory.Component = (*Component)(nil)
	_ factory.Lifecycle = (*Component)(nil)
	_ factory.Component = (*HealthAwareComponent)(nil)
)