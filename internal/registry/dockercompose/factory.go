package dockercompose

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "dockercompose-registry"

// Component implements factory.Component for Docker Compose registry
type Component struct {
	config   *Config
	registry *Registry
	logger   *slog.Logger
}

// NewComponent creates a new Docker Compose registry component
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
	// Parse the Docker Compose registry configuration
	var composeConfig config.DockerComposeRegistry
	if err := parser(&composeConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Convert to internal config
	c.config = &Config{
		ProjectName:     composeConfig.ProjectName,
		LabelPrefix:     composeConfig.LabelPrefix,
		RefreshInterval: time.Duration(composeConfig.RefreshInterval) * time.Second,
		DockerHost:      composeConfig.DockerHost,
		APIVersion:      composeConfig.APIVersion,
	}
	
	// Set defaults
	if c.config.LabelPrefix == "" {
		c.config.LabelPrefix = "gateway"
	}
	if c.config.RefreshInterval == 0 {
		c.config.RefreshInterval = 10 * time.Second
	}
	if c.config.DockerHost == "" {
		c.config.DockerHost = "unix:///var/run/docker.sock"
	}
	
	// Create registry
	registry, err := NewRegistry(c.config, c.logger)
	if err != nil {
		return fmt.Errorf("create Docker Compose registry: %w", err)
	}
	
	c.registry = registry
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.registry == nil {
		return fmt.Errorf("Docker Compose registry not initialized")
	}
	
	// Validate configuration
	if c.config.ProjectName == "" {
		return fmt.Errorf("project name is required")
	}
	if c.config.RefreshInterval < time.Second {
		return fmt.Errorf("refresh interval too short: %v", c.config.RefreshInterval)
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

// Start starts the registry (implements Lifecycle)
func (c *Component) Start() error {
	if c.registry == nil {
		return fmt.Errorf("registry not initialized")
	}
	ctx := context.Background()
	return c.registry.Start(ctx)
}

// Stop stops the registry (implements Lifecycle)
func (c *Component) Stop() error {
	if c.registry == nil {
		return nil
	}
	ctx := context.Background()
	return c.registry.Stop(ctx)
}

// GetRegistry returns the Docker Compose registry (for specific use cases)
func (c *Component) GetRegistry() *Registry {
	return c.registry
}

// Ensure Component implements factory.Component and factory.Lifecycle
var (
	_ factory.Component = (*Component)(nil)
	_ factory.Lifecycle = (*Component)(nil)
)