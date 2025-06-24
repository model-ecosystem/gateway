package router

import (
	"crypto/md5"
	"fmt"
	"hash/crc32"
	"sort"
	"sync"

	"gateway/internal/core"
	"gateway/pkg/errors"
)

// ConsistentHashBalancer implements consistent hashing load balancing
type ConsistentHashBalancer struct {
	mu               sync.RWMutex
	replicas         int
	hashFunc         HashFunc
	ring             map[uint32]string // hash -> instance ID
	sortedHashes     []uint32
	instances        map[string]*core.ServiceInstance
	virtualNodes     map[string][]uint32 // instance ID -> virtual node hashes
}

// HashFunc defines the hash function type
type HashFunc func(data []byte) uint32

// DefaultHashFunc uses CRC32 for hashing
func DefaultHashFunc(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// MD5HashFunc uses MD5 for better distribution
func MD5HashFunc(data []byte) uint32 {
	h := md5.Sum(data)
	return uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
}

// NewConsistentHashBalancer creates a new consistent hash balancer
func NewConsistentHashBalancer(replicas int) *ConsistentHashBalancer {
	if replicas <= 0 {
		replicas = 150 // Default number of virtual nodes per instance
	}
	
	return &ConsistentHashBalancer{
		replicas:     replicas,
		hashFunc:     DefaultHashFunc,
		ring:         make(map[uint32]string),
		instances:    make(map[string]*core.ServiceInstance),
		virtualNodes: make(map[string][]uint32),
	}
}

// NewConsistentHashBalancerWithHash creates a balancer with custom hash function
func NewConsistentHashBalancerWithHash(replicas int, hashFunc HashFunc) *ConsistentHashBalancer {
	b := NewConsistentHashBalancer(replicas)
	b.hashFunc = hashFunc
	return b
}

// Select selects an instance using the request for consistent hashing
func (b *ConsistentHashBalancer) Select(instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	// This method shouldn't be called for consistent hash
	// Use SelectForRequest instead
	return nil, errors.NewError(errors.ErrorTypeInternal, 
		"consistent hash balancer requires request context, use SelectForRequest")
}

// SelectForRequest selects an instance based on request key
func (b *ConsistentHashBalancer) SelectForRequest(req core.Request, instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Update ring if instances changed
	if b.instancesChanged(instances) {
		b.updateRing(instances)
	}
	
	if len(b.sortedHashes) == 0 {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no healthy instances")
	}
	
	// Get hash key from request
	key := b.getHashKey(req)
	hash := b.hashFunc([]byte(key))
	
	// Find the first node with hash >= key hash
	idx := sort.Search(len(b.sortedHashes), func(i int) bool {
		return b.sortedHashes[i] >= hash
	})
	
	// Wrap around if necessary
	if idx == len(b.sortedHashes) {
		idx = 0
	}
	
	// Get instance ID from ring
	instanceID := b.ring[b.sortedHashes[idx]]
	instance, ok := b.instances[instanceID]
	if !ok {
		return nil, errors.NewError(errors.ErrorTypeInternal, "instance not found in ring")
	}
	
	// Check if instance is healthy
	if !instance.Healthy {
		// Try next instances in the ring
		for i := 1; i < len(b.sortedHashes); i++ {
			nextIdx := (idx + i) % len(b.sortedHashes)
			nextID := b.ring[b.sortedHashes[nextIdx]]
			nextInstance, ok := b.instances[nextID]
			if ok && nextInstance.Healthy {
				return nextInstance, nil
			}
		}
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no healthy instances")
	}
	
	return instance, nil
}

// getHashKey extracts the key for hashing from the request
func (b *ConsistentHashBalancer) getHashKey(req core.Request) string {
	// Priority order for hash key:
	// 1. Session ID from header (for session affinity)
	// 2. Client IP from RemoteAddr
	// 3. Request path
	
	// Check for session ID in headers
	headers := req.Headers()
	if sessionIDs, ok := headers["X-Session-Id"]; ok && len(sessionIDs) > 0 {
		return sessionIDs[0]
	}
	
	// Check for session in Cookie header
	if cookies, ok := headers["Cookie"]; ok && len(cookies) > 0 {
		// Simple cookie parsing for session
		for _, cookie := range cookies {
			if len(cookie) > 8 && cookie[:8] == "session=" {
				return cookie[8:]
			}
		}
	}
	
	// Use remote address (client IP)
	if remoteAddr := req.RemoteAddr(); remoteAddr != "" {
		// Extract IP from IP:port format
		if idx := len(remoteAddr) - 1; idx >= 0 {
			for i := idx; i >= 0; i-- {
				if remoteAddr[i] == ':' {
					return remoteAddr[:i]
				}
			}
		}
		return remoteAddr
	}
	
	// Fallback to path
	return req.Path()
}

// instancesChanged checks if the instances list has changed
func (b *ConsistentHashBalancer) instancesChanged(instances []core.ServiceInstance) bool {
	if len(instances) != len(b.instances) {
		return true
	}
	
	for _, inst := range instances {
		existing, ok := b.instances[inst.ID]
		if !ok || existing.Healthy != inst.Healthy {
			return true
		}
	}
	
	return false
}

// updateRing rebuilds the consistent hash ring
func (b *ConsistentHashBalancer) updateRing(instances []core.ServiceInstance) {
	// Clear existing ring
	b.ring = make(map[uint32]string)
	b.sortedHashes = nil
	b.instances = make(map[string]*core.ServiceInstance)
	b.virtualNodes = make(map[string][]uint32)
	
	// Add instances to ring
	for i := range instances {
		inst := &instances[i]
		if !inst.Healthy {
			continue
		}
		
		b.instances[inst.ID] = inst
		virtualHashes := make([]uint32, 0, b.replicas)
		
		// Create virtual nodes
		for j := 0; j < b.replicas; j++ {
			virtualKey := fmt.Sprintf("%s#%d", inst.ID, j)
			hash := b.hashFunc([]byte(virtualKey))
			
			// Handle hash collision
			if _, exists := b.ring[hash]; exists {
				// Try with a different key
				for k := 0; k < 10; k++ {
					virtualKey = fmt.Sprintf("%s#%d#%d", inst.ID, j, k)
					hash = b.hashFunc([]byte(virtualKey))
					if _, exists := b.ring[hash]; !exists {
						break
					}
				}
			}
			
			b.ring[hash] = inst.ID
			virtualHashes = append(virtualHashes, hash)
		}
		
		b.virtualNodes[inst.ID] = virtualHashes
	}
	
	// Sort hashes for binary search
	b.sortedHashes = make([]uint32, 0, len(b.ring))
	for hash := range b.ring {
		b.sortedHashes = append(b.sortedHashes, hash)
	}
	sort.Slice(b.sortedHashes, func(i, j int) bool {
		return b.sortedHashes[i] < b.sortedHashes[j]
	})
}

// GetStats returns statistics about the hash ring
func (b *ConsistentHashBalancer) GetStats() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	distribution := make(map[string]int)
	for _, instanceID := range b.ring {
		distribution[instanceID]++
	}
	
	return map[string]interface{}{
		"instances":     len(b.instances),
		"virtual_nodes": len(b.ring),
		"replicas":      b.replicas,
		"distribution":  distribution,
	}
}

// RemoveInstance removes an instance from the ring
func (b *ConsistentHashBalancer) RemoveInstance(instanceID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Remove instance
	delete(b.instances, instanceID)
	
	// Remove virtual nodes
	if hashes, ok := b.virtualNodes[instanceID]; ok {
		for _, hash := range hashes {
			delete(b.ring, hash)
		}
		delete(b.virtualNodes, instanceID)
	}
	
	// Rebuild sorted hashes
	b.sortedHashes = make([]uint32, 0, len(b.ring))
	for hash := range b.ring {
		b.sortedHashes = append(b.sortedHashes, hash)
	}
	sort.Slice(b.sortedHashes, func(i, j int) bool {
		return b.sortedHashes[i] < b.sortedHashes[j]
	})
}