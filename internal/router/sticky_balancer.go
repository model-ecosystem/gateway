package router

import (
	"sync"
	"time"
	
	"gateway/internal/core"
	"gateway/internal/session"
	"gateway/pkg/errors"
)

// SessionStore stores session-to-instance mappings
type SessionStore interface {
	// GetInstance gets the instance ID for a session
	GetInstance(sessionID string) (string, bool)
	// SetInstance sets the instance ID for a session
	SetInstance(sessionID string, instanceID string, ttl time.Duration)
	// RemoveInstance removes a session mapping
	RemoveInstance(sessionID string)
	// Cleanup removes expired sessions
	Cleanup()
}

// memorySessionStore is an in-memory session store
type memorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*sessionEntry
}

type sessionEntry struct {
	instanceID string
	expiresAt  time.Time
}

func newMemorySessionStore() *memorySessionStore {
	store := &memorySessionStore{
		sessions: make(map[string]*sessionEntry),
	}
	
	// Start cleanup goroutine
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			store.Cleanup()
		}
	}()
	
	return store
}

func (s *memorySessionStore) GetInstance(sessionID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	entry, ok := s.sessions[sessionID]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	
	return entry.instanceID, true
}

func (s *memorySessionStore) SetInstance(sessionID string, instanceID string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.sessions[sessionID] = &sessionEntry{
		instanceID: instanceID,
		expiresAt:  time.Now().Add(ttl),
	}
}

func (s *memorySessionStore) RemoveInstance(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.sessions, sessionID)
}

func (s *memorySessionStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	for sessionID, entry := range s.sessions {
		if now.After(entry.expiresAt) {
			delete(s.sessions, sessionID)
		}
	}
}

// StickySessionBalancer implements sticky session load balancing
type StickySessionBalancer struct {
	fallback  core.LoadBalancer
	store     SessionStore
	ttl       time.Duration
	extractor session.Extractor
}

// NewStickySessionBalancer creates a new sticky session balancer
func NewStickySessionBalancer(fallback core.LoadBalancer, config *core.SessionAffinityConfig) *StickySessionBalancer {
	// Use default configuration if none provided
	if config == nil {
		config = &core.SessionAffinityConfig{
			Enabled:    true,
			TTL:        time.Hour,
			Source:     core.SessionSourceCookie,
			CookieName: "GATEWAY_SESSION",
		}
	}
	
	// Ensure we have a TTL
	if config.TTL == 0 {
		config.TTL = time.Hour
	}
	
	return &StickySessionBalancer{
		fallback:  fallback,
		store:     newMemorySessionStore(),
		ttl:       config.TTL,
		extractor: session.NewExtractor(config),
	}
}

// SelectForRequest selects an instance based on the request
func (b *StickySessionBalancer) SelectForRequest(req core.Request, instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no healthy instances")
	}
	
	// Get session ID using configured extractor
	sessionID := b.extractor.Extract(req)
	if sessionID == "" {
		// No session, use fallback balancer
		return b.fallback.Select(instances)
	}
	
	// Check if we have a sticky session
	if instanceID, ok := b.store.GetInstance(sessionID); ok {
		// Find the instance
		for i := range instances {
			if instances[i].ID == instanceID && instances[i].Healthy {
				// Instance still healthy, use it
				return &instances[i], nil
			}
		}
		// Instance not found or not healthy, remove from store
		b.store.RemoveInstance(sessionID)
	}
	
	// Select new instance using fallback
	instance, err := b.fallback.Select(instances)
	if err != nil {
		return nil, err
	}
	
	// Store the mapping
	b.store.SetInstance(sessionID, instance.ID, b.ttl)
	
	return instance, nil
}

// Select implements core.LoadBalancer (delegates to fallback for non-sticky selection)
func (b *StickySessionBalancer) Select(instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	return b.fallback.Select(instances)
}