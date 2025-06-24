package factory

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/health"
	"gateway/internal/registry"
)

// RegistryFactory creates service registry instances
type RegistryFactory struct {
	BaseComponentFactory
}

// NewRegistryFactory creates a new registry factory
func NewRegistryFactory(logger *slog.Logger) *RegistryFactory {
	return &RegistryFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// CreateRegistry creates a service registry based on configuration
func (f *RegistryFactory) CreateRegistry(cfg *config.Registry) (core.ServiceRegistry, error) {
	registryComponent := registry.NewComponent(f.logger)
	if err := registryComponent.Init(func(v interface{}) error {
		return f.ParseConfig(*cfg, v)
	}); err != nil {
		return nil, fmt.Errorf("initializing registry: %w", err)
	}
	
	if err := registryComponent.Validate(); err != nil {
		return nil, fmt.Errorf("validating registry: %w", err)
	}
	
	registryComp := registryComponent.(*registry.Component)
	return registryComp.Build(), nil
}

// CreateHealthAwareRegistry creates a health-aware service registry
func (f *RegistryFactory) CreateHealthAwareRegistry(cfg *config.Registry, healthCfg *config.Health) (core.ServiceRegistry, *health.BackendMonitor, error) {
	// First create the base registry
	baseRegistry, err := f.CreateRegistry(cfg)
	if err != nil {
		return nil, nil, err
	}
	
	// If health is not enabled, just return the base registry
	if healthCfg == nil || !healthCfg.Enabled {
		return baseRegistry, nil, nil
	}
	
	// For now, just return the base registry without health monitoring
	// Health monitoring is handled by the health component separately
	return baseRegistry, nil, nil
}