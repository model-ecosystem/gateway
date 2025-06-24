package factory

import (
	"fmt"
	"sync"
)

// Creator is a function that creates a new instance of a component
type Creator func() Component

// Registry manages component creators
type Registry struct {
	mu       sync.RWMutex
	creators map[string]Creator
}

// NewRegistry creates a new component registry
func NewRegistry() *Registry {
	return &Registry{
		creators: make(map[string]Creator),
	}
}

// Register registers a component creator
func (r *Registry) Register(name string, creator Creator) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.creators[name]; exists {
		return fmt.Errorf("component %s already registered", name)
	}
	
	r.creators[name] = creator
	return nil
}

// Create creates and initializes a component by name
func (r *Registry) Create(name string, config any) (Component, error) {
	r.mu.RLock()
	creator, exists := r.creators[name]
	r.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("component %s not registered", name)
	}
	
	component := creator()
	return Build(component, config)
}

// List returns all registered component names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.creators))
	for name := range r.creators {
		names = append(names, name)
	}
	return names
}

// Global registry for convenience
var globalRegistry = NewRegistry()

// Register registers a component creator in the global registry
func Register(name string, creator Creator) error {
	return globalRegistry.Register(name, creator)
}

// Create creates a component from the global registry
func Create(name string, config any) (Component, error) {
	return globalRegistry.Create(name, config)
}

// List returns all globally registered component names
func List() []string {
	return globalRegistry.List()
}