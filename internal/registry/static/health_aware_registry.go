package static

import (
	"fmt"
	"sync"
	
	"gateway/internal/config"
	"gateway/internal/core"
)

// HealthAwareRegistry provides static service discovery with health status updates
type HealthAwareRegistry struct {
	services map[string]map[string]*core.ServiceInstance // service -> instanceID -> instance
	mu       sync.RWMutex
}

// NewHealthAwareRegistry creates a health-aware static registry from config
func NewHealthAwareRegistry(cfg *config.StaticRegistry) (*HealthAwareRegistry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("static registry config is nil")
	}

	r := &HealthAwareRegistry{
		services: make(map[string]map[string]*core.ServiceInstance),
	}

	// Load services from config
	for _, svc := range cfg.Services {
		instanceMap := make(map[string]*core.ServiceInstance)
		for _, inst := range svc.Instances {
			instance := inst.ToServiceInstance(svc.Name)
			instanceMap[instance.ID] = &instance
		}
		r.services[svc.Name] = instanceMap
	}

	return r, nil
}

// GetService returns healthy instances for a service
func (r *HealthAwareRegistry) GetService(name string) ([]core.ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	instanceMap, ok := r.services[name]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", name)
	}
	
	// Return only healthy instances
	instances := make([]core.ServiceInstance, 0, len(instanceMap))
	for _, inst := range instanceMap {
		if inst.Healthy {
			instances = append(instances, *inst)
		}
	}
	
	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instances for service: %s", name)
	}
	
	return instances, nil
}

// GetAllInstances returns all instances (healthy and unhealthy) for a service
func (r *HealthAwareRegistry) GetAllInstances(name string) ([]core.ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	instanceMap, ok := r.services[name]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", name)
	}
	
	instances := make([]core.ServiceInstance, 0, len(instanceMap))
	for _, inst := range instanceMap {
		instances = append(instances, *inst)
	}
	
	return instances, nil
}

// UpdateInstanceHealth updates the health status of an instance
func (r *HealthAwareRegistry) UpdateInstanceHealth(serviceName, instanceID string, healthy bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	instanceMap, ok := r.services[serviceName]
	if !ok {
		return fmt.Errorf("service not found: %s", serviceName)
	}
	
	instance, ok := instanceMap[instanceID]
	if !ok {
		return fmt.Errorf("instance not found: %s/%s", serviceName, instanceID)
	}
	
	instance.Healthy = healthy
	return nil
}

// RegisterHealthUpdateCallback returns a callback function for the backend monitor
func (r *HealthAwareRegistry) RegisterHealthUpdateCallback() func(string, *core.ServiceInstance, bool) {
	return func(serviceName string, instance *core.ServiceInstance, healthy bool) {
		_ = r.UpdateInstanceHealth(serviceName, instance.ID, healthy)
	}
}