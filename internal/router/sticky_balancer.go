package router

import (
	"container/list"
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
	// Close stops any background goroutines
	Close() error
}

// memorySessionStore is an in-memory session store with size limits
type memorySessionStore struct {
	mu         sync.RWMutex
	sessions   map[string]*sessionEntry
	maxEntries int
	// Track access order for LRU eviction
	accessList *list.List
	accessMap  map[string]*list.Element
	stopCh     chan struct{}
}

type sessionEntry struct {
	sessionID  string
	instanceID string
	expiresAt  time.Time
}

func newMemorySessionStore(maxEntries int) *memorySessionStore {
	if maxEntries <= 0 {
		maxEntries = 10000 // Default max entries
	}

	store := &memorySessionStore{
		sessions:   make(map[string]*sessionEntry),
		maxEntries: maxEntries,
		accessList: list.New(),
		accessMap:  make(map[string]*list.Element),
		stopCh:     make(chan struct{}),
	}

	// Start cleanup goroutine
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				store.Cleanup()
			case <-store.stopCh:
				return
			}
		}
	}()

	return store
}

func (s *memorySessionStore) GetInstance(sessionID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.sessions[sessionID]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}

	// Update access order for LRU
	if elem, exists := s.accessMap[sessionID]; exists {
		s.accessList.MoveToFront(elem)
	}

	return entry.instanceID, true
}

func (s *memorySessionStore) SetInstance(sessionID string, instanceID string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we need to evict
	if len(s.sessions) >= s.maxEntries && s.sessions[sessionID] == nil {
		// Evict least recently used
		if s.accessList.Len() > 0 {
			oldest := s.accessList.Back()
			if oldest != nil {
				oldSessionID := oldest.Value.(string)
				delete(s.sessions, oldSessionID)
				delete(s.accessMap, oldSessionID)
				s.accessList.Remove(oldest)
			}
		}
	}

	entry := &sessionEntry{
		sessionID:  sessionID,
		instanceID: instanceID,
		expiresAt:  time.Now().Add(ttl),
	}

	s.sessions[sessionID] = entry

	// Update access tracking
	if elem, exists := s.accessMap[sessionID]; exists {
		s.accessList.MoveToFront(elem)
		elem.Value = sessionID
	} else {
		elem := s.accessList.PushFront(sessionID)
		s.accessMap[sessionID] = elem
	}
}

func (s *memorySessionStore) RemoveInstance(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)

	// Remove from access tracking
	if elem, exists := s.accessMap[sessionID]; exists {
		s.accessList.Remove(elem)
		delete(s.accessMap, sessionID)
	}
}

func (s *memorySessionStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for sessionID, entry := range s.sessions {
		if now.After(entry.expiresAt) {
			delete(s.sessions, sessionID)

			// Remove from access tracking
			if elem, exists := s.accessMap[sessionID]; exists {
				s.accessList.Remove(elem)
				delete(s.accessMap, sessionID)
			}
		}
	}
}

// Close stops the cleanup goroutine
func (s *memorySessionStore) Close() error {
	close(s.stopCh)
	return nil
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

	// Get max entries from config or use default
	maxEntries := config.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 10000
	}

	return &StickySessionBalancer{
		fallback:  fallback,
		store:     newMemorySessionStore(maxEntries),
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

// Close closes the balancer and cleans up resources
func (b *StickySessionBalancer) Close() error {
	return b.store.Close()
}
