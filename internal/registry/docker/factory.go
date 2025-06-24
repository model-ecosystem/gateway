package docker

import (
	"fmt"
	"log/slog"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "docker-registry"

// Component implements factory.Component for Docker registry
type Component struct {
	config   *Config
	registry *Registry
	logger   *slog.Logger
}

// NewComponent creates a new Docker registry component
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
	// Parse the Docker registry configuration
	var dockerConfig config.DockerRegistry
	if err := parser(&dockerConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Convert to internal config
	c.config = &Config{
		Host:            dockerConfig.Host,
		Version:         dockerConfig.Version,
		CertPath:        dockerConfig.CertPath,
		LabelPrefix:     dockerConfig.LabelPrefix,
		Network:         dockerConfig.Network,
		RefreshInterval: dockerConfig.RefreshInterval,
	}
	
	// Set defaults
	if c.config.Host == "" {
		c.config.Host = "unix:///var/run/docker.sock"
	}
	if c.config.LabelPrefix == "" {
		c.config.LabelPrefix = "gateway"
	}
	if c.config.RefreshInterval == 0 {
		c.config.RefreshInterval = 10 // 10 seconds
	}
	
	// Create registry
	registry, err := NewRegistry(c.config, c.logger)
	if err != nil {
		return fmt.Errorf("create Docker registry: %w", err)
	}
	
	c.registry = registry
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.registry == nil {
		return fmt.Errorf("Docker registry not initialized")
	}
	
	// Validate configuration
	if c.config.RefreshInterval < 1 {
		return fmt.Errorf("refresh interval too short: %d seconds", c.config.RefreshInterval)
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
	// Docker registry starts automatically in NewRegistry
	return nil
}

// Stop stops the registry (implements Lifecycle)
func (c *Component) Stop() error {
	if c.registry == nil {
		return nil
	}
	return c.registry.Close()
}

// GetRegistry returns the Docker registry (for specific use cases)
func (c *Component) GetRegistry() *Registry {
	return c.registry
}

// Ensure Component implements factory.Component and factory.Lifecycle
var (
	_ factory.Component = (*Component)(nil)
	_ factory.Lifecycle = (*Component)(nil)
)