package router

import (
	"math/rand"
	"sync"
	"sync/atomic"
	
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// WeightedRoundRobinBalancer implements weighted round-robin load balancing
type WeightedRoundRobinBalancer struct {
	mu                sync.RWMutex
	weightedInstances []weightedInstance
	totalWeight       int
	counter           atomic.Uint64
}

type weightedInstance struct {
	instance         core.ServiceInstance
	weight           int
	currentWeight    int
	effectiveWeight  int
}

// NewWeightedRoundRobinBalancer creates a new weighted round-robin balancer
func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{}
}

// Select selects the next healthy instance based on weights
func (b *WeightedRoundRobinBalancer) Select(instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Update weighted instances if needed
	b.updateWeightedInstances(instances)
	
	if len(b.weightedInstances) == 0 {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no healthy instances")
	}
	
	// Use smooth weighted round-robin algorithm
	var selected *weightedInstance
	totalWeight := 0
	
	for i := range b.weightedInstances {
		w := &b.weightedInstances[i]
		
		// Skip unhealthy instances
		if !w.instance.Healthy {
			w.currentWeight = 0
			continue
		}
		
		// Update current weight
		w.currentWeight += w.effectiveWeight
		totalWeight += w.effectiveWeight
		
		// Select instance with highest current weight
		if selected == nil || w.currentWeight > selected.currentWeight {
			selected = w
		}
	}
	
	if selected == nil {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no healthy instances")
	}
	
	// Reduce selected instance's current weight
	selected.currentWeight -= totalWeight
	
	return &selected.instance, nil
}

// updateWeightedInstances updates the weighted instance list
func (b *WeightedRoundRobinBalancer) updateWeightedInstances(instances []core.ServiceInstance) {
	// Check if instances changed
	if b.instancesEqual(instances) {
		return
	}
	
	// Rebuild weighted instances
	b.weightedInstances = make([]weightedInstance, 0, len(instances))
	b.totalWeight = 0
	
	for _, inst := range instances {
		weight := b.getWeight(inst)
		if weight > 0 {
			b.weightedInstances = append(b.weightedInstances, weightedInstance{
				instance:        inst,
				weight:          weight,
				currentWeight:   0,
				effectiveWeight: weight,
			})
			b.totalWeight += weight
		}
	}
}

// getWeight extracts weight from instance metadata
func (b *WeightedRoundRobinBalancer) getWeight(instance core.ServiceInstance) int {
	// Check metadata for weight
	if instance.Metadata != nil {
		if weight, ok := instance.Metadata["weight"].(int); ok && weight > 0 {
			return weight
		}
		// Also check float values
		if weight, ok := instance.Metadata["weight"].(float64); ok && weight > 0 {
			return int(weight)
		}
	}
	
	// Default weight is 1
	return 1
}

// instancesEqual checks if instances list has changed
func (b *WeightedRoundRobinBalancer) instancesEqual(instances []core.ServiceInstance) bool {
	if len(instances) != len(b.weightedInstances) {
		return false
	}
	
	// Simple check - could be optimized with instance ID map
	for i, inst := range instances {
		if i >= len(b.weightedInstances) || 
		   b.weightedInstances[i].instance.ID != inst.ID ||
		   b.weightedInstances[i].instance.Healthy != inst.Healthy {
			return false
		}
	}
	
	return true
}

// WeightedRandomBalancer implements weighted random load balancing
type WeightedRandomBalancer struct {
	mu   sync.RWMutex
	rand *rand.Rand
}

// NewWeightedRandomBalancer creates a new weighted random balancer
func NewWeightedRandomBalancer() *WeightedRandomBalancer {
	return &WeightedRandomBalancer{
		rand: rand.New(rand.NewSource(rand.Int63())),
	}
}

// Select randomly selects an instance based on weights
func (b *WeightedRandomBalancer) Select(instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Build weighted list of healthy instances
	var weightedList []weightedInstance
	totalWeight := 0
	
	for _, inst := range instances {
		if inst.Healthy {
			weight := b.getWeight(inst)
			if weight > 0 {
				weightedList = append(weightedList, weightedInstance{
					instance: inst,
					weight:   weight,
				})
				totalWeight += weight
			}
		}
	}
	
	if len(weightedList) == 0 {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no healthy instances")
	}
	
	// Random selection based on weights
	target := b.rand.Intn(totalWeight)
	current := 0
	
	for _, w := range weightedList {
		current += w.weight
		if target < current {
			return &w.instance, nil
		}
	}
	
	// Fallback (should not happen)
	return &weightedList[len(weightedList)-1].instance, nil
}

// getWeight extracts weight from instance metadata
func (b *WeightedRandomBalancer) getWeight(instance core.ServiceInstance) int {
	// Check metadata for weight
	if instance.Metadata != nil {
		if weight, ok := instance.Metadata["weight"].(int); ok && weight > 0 {
			return weight
		}
		// Also check float values
		if weight, ok := instance.Metadata["weight"].(float64); ok && weight > 0 {
			return int(weight)
		}
	}
	
	// Default weight is 1
	return 1
}