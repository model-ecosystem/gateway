package middleware

import (
	"fmt"
	"log/slog"
	
	"gateway/internal/middleware/auth"
	"gateway/internal/middleware/circuitbreaker"
	"gateway/internal/middleware/cors"
	"gateway/internal/middleware/retry"
	"gateway/internal/middleware/ratelimit"
	"gateway/internal/storage"
	"gateway/pkg/factory"
)

// Registry manages middleware component registration
type Registry struct {
	registry *factory.Registry
	logger   *slog.Logger
}

// NewRegistry creates a new middleware registry
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		registry: factory.NewRegistry(),
		logger:   logger,
	}
}

// RegisterAll registers all built-in middleware components
func (r *Registry) RegisterAll(store storage.LimiterStore) error {
	// Register CORS middleware
	if err := r.registry.Register(cors.ComponentName, func() factory.Component {
		return cors.NewComponent()
	}); err != nil {
		return fmt.Errorf("register CORS: %w", err)
	}
	
	// Register Auth middleware
	if err := r.registry.Register(auth.ComponentName, func() factory.Component {
		return auth.NewComponent(r.logger)
	}); err != nil {
		return fmt.Errorf("register Auth: %w", err)
	}
	
	// Register Retry middleware
	if err := r.registry.Register(retry.ComponentName, func() factory.Component {
		return retry.NewComponent(r.logger)
	}); err != nil {
		return fmt.Errorf("register Retry: %w", err)
	}
	
	// Register CircuitBreaker middleware
	if err := r.registry.Register(circuitbreaker.ComponentName, func() factory.Component {
		return circuitbreaker.NewComponent(r.logger)
	}); err != nil {
		return fmt.Errorf("register CircuitBreaker: %w", err)
	}
	
	// Register RateLimit middleware (requires store)
	if err := r.registry.Register(ratelimit.ComponentName, func() factory.Component {
		return ratelimit.NewComponent(r.logger, store)
	}); err != nil {
		return fmt.Errorf("register RateLimit: %w", err)
	}
	
	r.logger.Info("Registered all middleware components",
		"components", r.registry.List(),
	)
	
	return nil
}

// Create creates a middleware component by name
func (r *Registry) Create(name string, config interface{}) (factory.Component, error) {
	return r.registry.Create(name, config)
}

// List returns all registered middleware names
func (r *Registry) List() []string {
	return r.registry.List()
}

// GetRegistry returns the underlying factory registry
func (r *Registry) GetRegistry() *factory.Registry {
	return r.registry
}