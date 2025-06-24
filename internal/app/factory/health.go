package factory

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/health"
)

// HealthFactory creates health check components
type HealthFactory struct {
	BaseComponentFactory
}

// NewHealthFactory creates a new health factory
func NewHealthFactory(logger *slog.Logger) *HealthFactory {
	return &HealthFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// CreateHealthHandler creates health check handler and backend monitor
func (f *HealthFactory) CreateHealthHandler(
	cfg *config.Health,
	registry core.ServiceRegistry,
	version string,
	serviceID string,
) (*health.Handler, *health.BackendMonitor, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil, nil
	}

	// Create health component using factory
	healthComponent := health.NewComponent(registry, f.logger, version, serviceID)
	if err := healthComponent.Init(func(v interface{}) error {
		return f.ParseConfig(cfg, v)
	}); err != nil {
		return nil, nil, fmt.Errorf("initializing health component: %w", err)
	}
	
	if err := healthComponent.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validating health component: %w", err)
	}
	
	// Cast to concrete type to access Build and other methods
	healthComp := healthComponent.(*health.Component)
	healthHandler := healthComp.Build()
	backendMonitor := healthComp.GetBackendMonitor()
	
	return healthHandler, backendMonitor, nil
}