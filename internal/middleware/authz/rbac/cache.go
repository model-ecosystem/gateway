package rbac

import (
	"strings"
	"sync"
	"time"
)

// cacheEntry represents a cached permission check result
type cacheEntry struct {
	allowed   bool
	timestamp time.Time
}

// permissionCache caches permission check results
type permissionCache struct {
	entries  map[string]*cacheEntry
	mu       sync.RWMutex
	maxSize  int
	ttl      time.Duration
	lastClean time.Time
}

// newPermissionCache creates a new permission cache
func newPermissionCache(maxSize int, ttl time.Duration) *permissionCache {
	return &permissionCache{
		entries:   make(map[string]*cacheEntry),
		maxSize:   maxSize,
		ttl:       ttl,
		lastClean: time.Now(),
	}
}

// Get retrieves a cached permission check result
func (c *permissionCache) Get(key string) (bool, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()
	
	if !exists {
		return false, false
	}
	
	// Check if entry is expired
	if time.Since(entry.timestamp) > c.ttl {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return false, false
	}
	
	return entry.allowed, true
}

// Set stores a permission check result
func (c *permissionCache) Set(key string, allowed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Clean old entries if needed
	if time.Since(c.lastClean) > c.ttl {
		c.cleanExpiredLocked()
	}
	
	// Check size limit
	if len(c.entries) >= c.maxSize {
		// Remove oldest entry
		var oldestKey string
		var oldestTime time.Time
		first := true
		
		for k, entry := range c.entries {
			if first || entry.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = entry.timestamp
				first = false
			}
		}
		
		if oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}
	
	c.entries[key] = &cacheEntry{
		allowed:   allowed,
		timestamp: time.Now(),
	}
}

// Clear removes all entries from the cache
func (c *permissionCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

// ClearSubject removes all entries for a specific subject
func (c *permissionCache) ClearSubject(subject string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	prefix := subject + ":"
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
		}
	}
}

// cleanExpiredLocked removes expired entries (must be called with lock held)
func (c *permissionCache) cleanExpiredLocked() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.Sub(entry.timestamp) > c.ttl {
			delete(c.entries, key)
		}
	}
	c.lastClean = now
}
