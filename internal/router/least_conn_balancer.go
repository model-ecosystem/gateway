package router

import (
	"sync"
	"sync/atomic"
	
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// LeastConnectionsBalancer implements least connections load balancing
type LeastConnectionsBalancer struct {
	mu          sync.RWMutex
	connections map[string]*atomic.Int64 // instance ID -> connection count
}

// NewLeastConnectionsBalancer creates a new least connections balancer
func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		connections: make(map[string]*atomic.Int64),
	}
}

// Select selects the instance with least active connections
func (b *LeastConnectionsBalancer) Select(instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	var selected *core.ServiceInstance
	var minConnections int64 = -1
	
	// Find healthy instance with least connections
	for i := range instances {
		inst := &instances[i]
		
		// Skip unhealthy instances
		if !inst.Healthy {
			continue
		}
		
		// Get connection count
		count := b.getConnectionCount(inst.ID)
		
		// Select instance with least connections
		if minConnections == -1 || count < minConnections {
			selected = inst
			minConnections = count
		}
	}
	
	if selected == nil {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no healthy instances")
	}
	
	// Increment connection count
	b.incrementConnections(selected.ID)
	
	return selected, nil
}

// SelectForRequest selects instance and tracks the request
func (b *LeastConnectionsBalancer) SelectForRequest(req core.Request, instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	return b.Select(instances)
}

// getConnectionCount gets the current connection count for an instance
func (b *LeastConnectionsBalancer) getConnectionCount(instanceID string) int64 {
	if counter, exists := b.connections[instanceID]; exists {
		return counter.Load()
	}
	return 0
}

// incrementConnections increments the connection count
func (b *LeastConnectionsBalancer) incrementConnections(instanceID string) {
	// Need to upgrade read lock to write lock
	// First check if counter exists under read lock
	counter, exists := b.connections[instanceID]
	
	if !exists {
		// Need to create counter - upgrade to write lock
		b.mu.RUnlock()
		b.mu.Lock()
		
		// Double-check after acquiring write lock
		counter, exists = b.connections[instanceID]
		if !exists {
			counter = &atomic.Int64{}
			b.connections[instanceID] = counter
		}
		
		// Downgrade back to read lock
		b.mu.Unlock()
		b.mu.RLock()
	}
	
	// Increment is atomic, safe under read lock
	counter.Add(1)
}

// DecrementConnections decrements the connection count (called when request completes)
func (b *LeastConnectionsBalancer) DecrementConnections(instanceID string) {
	b.mu.RLock()
	if counter, exists := b.connections[instanceID]; exists {
		counter.Add(-1)
	}
	b.mu.RUnlock()
}

// Reset resets all connection counts
func (b *LeastConnectionsBalancer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Reset all counters
	for _, counter := range b.connections {
		counter.Store(0)
	}
}

// GetConnectionCounts returns current connection counts for monitoring
func (b *LeastConnectionsBalancer) GetConnectionCounts() map[string]int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	counts := make(map[string]int64, len(b.connections))
	for id, counter := range b.connections {
		counts[id] = counter.Load()
	}
	
	return counts
}

// ConnectionTracker wraps a load balancer to track connections
type ConnectionTracker struct {
	balancer  core.LoadBalancer
	leastConn *LeastConnectionsBalancer
}

// NewConnectionTracker creates a connection tracking wrapper
func NewConnectionTracker(balancer core.LoadBalancer) *ConnectionTracker {
	if lc, ok := balancer.(*LeastConnectionsBalancer); ok {
		return &ConnectionTracker{
			balancer:  balancer,
			leastConn: lc,
		}
	}
	return &ConnectionTracker{
		balancer: balancer,
	}
}

// StartRequest marks the start of a request
func (t *ConnectionTracker) StartRequest(instanceID string) {
	if t.leastConn != nil {
		// Already incremented in Select
	}
}

// EndRequest marks the end of a request
func (t *ConnectionTracker) EndRequest(instanceID string) {
	if t.leastConn != nil {
		t.leastConn.DecrementConnections(instanceID)
	}
}