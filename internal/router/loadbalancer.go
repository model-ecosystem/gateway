package router

import (
	"gateway/internal/core"
	"gateway/pkg/errors"
	"sync/atomic"
)

// RoundRobinBalancer implements round-robin load balancing
type RoundRobinBalancer struct {
	counter atomic.Uint64
}

// NewRoundRobinBalancer creates a new round-robin balancer
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// Select selects the next healthy instance
func (b *RoundRobinBalancer) Select(instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	// Filter healthy instances
	var healthy []core.ServiceInstance
	for _, inst := range instances {
		if inst.Healthy {
			healthy = append(healthy, inst)
		}
	}

	if len(healthy) == 0 {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no healthy instances")
	}

	// Round-robin selection
	index := b.counter.Add(1) % uint64(len(healthy))
	return &healthy[index], nil
}
