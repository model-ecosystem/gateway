package factory

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/management"
)

// ManagementFactory creates management API instances
type ManagementFactory struct {
	BaseComponentFactory
}

// NewManagementFactory creates a new management factory
func NewManagementFactory(logger *slog.Logger) *ManagementFactory {
	return &ManagementFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// CreateManagementAPI creates management API from configuration
func (f *ManagementFactory) CreateManagementAPI(cfg *config.Management) (*management.API, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	// Create management component using factory
	managementComponent := management.NewComponent(f.logger)
	if err := managementComponent.Init(func(v interface{}) error {
		return f.ParseConfig(cfg, v)
	}); err != nil {
		return nil, fmt.Errorf("initializing management component: %w", err)
	}
	
	if err := managementComponent.Validate(); err != nil {
		return nil, fmt.Errorf("validating management component: %w", err)
	}
	
	// Cast to concrete type to access Build method
	mgmtComp := managementComponent.(*management.Component)
	return mgmtComp.Build(), nil
}