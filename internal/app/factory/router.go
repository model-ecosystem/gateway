package factory

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/router"
)

// RouterFactory creates router instances
type RouterFactory struct {
	BaseComponentFactory
}

// NewRouterFactory creates a new router factory
func NewRouterFactory(logger *slog.Logger) *RouterFactory {
	return &RouterFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// CreateRouter creates a router based on configuration
func (f *RouterFactory) CreateRouter(cfg *config.Router, registry core.ServiceRegistry) (core.Router, error) {
	routerComponent := router.NewComponent(registry, f.logger)
	if err := routerComponent.Init(func(v interface{}) error {
		return f.ParseConfig(*cfg, v)
	}); err != nil {
		return nil, fmt.Errorf("initializing router: %w", err)
	}
	
	if err := routerComponent.Validate(); err != nil {
		return nil, fmt.Errorf("validating router: %w", err)
	}
	
	routerComp := routerComponent.(*router.Component)
	return routerComp.Build(), nil
}