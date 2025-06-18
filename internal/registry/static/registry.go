package static

import (
	"fmt"
	"gateway/internal/config"
	"gateway/internal/core"
)

// Registry provides static service discovery
type Registry struct {
	services map[string][]core.ServiceInstance
}

// NewRegistry creates a static registry from config
func NewRegistry(cfg *config.StaticRegistry) (*Registry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("static registry config is nil")
	}

	r := &Registry{
		services: make(map[string][]core.ServiceInstance),
	}

	// Load services from config
	for _, svc := range cfg.Services {
		instances := make([]core.ServiceInstance, 0, len(svc.Instances))
		for _, inst := range svc.Instances {
			instances = append(instances, inst.ToServiceInstance(svc.Name))
		}
		r.services[svc.Name] = instances
	}

	return r, nil
}

// GetService returns instances for a service
func (r *Registry) GetService(name string) ([]core.ServiceInstance, error) {
	instances, ok := r.services[name]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", name)
	}
	return instances, nil
}