package memory

import (
	"context"
	"sync"
	"time"

	"gateway/internal/storage"
)

// entry represents a rate limit entry
type entry struct {
	tokens    int
	lastReset time.Time
	mu        sync.Mutex
}

// Store implements LimiterStore using in-memory storage
type Store struct {
	entries map[string]*entry
	mu      sync.RWMutex
	config  *storage.LimiterStoreConfig
	done    chan struct{}
}

// NewStore creates a new memory store
func NewStore(config *storage.LimiterStoreConfig) *Store {
	if config == nil {
		config = storage.DefaultConfig()
	}

	s := &Store{
		entries: make(map[string]*entry),
		config:  config,
		done:    make(chan struct{}),
	}

	// Start cleanup routine
	if config.CleanupInterval > 0 {
		go s.cleanup()
	}

	return s
}

// Allow checks if a request is allowed
func (s *Store) Allow(ctx context.Context, key string, limit, burst int, window time.Duration) (bool, int, time.Time, error) {
	return s.AllowN(ctx, key, 1, limit, burst, window)
}

// AllowN checks if n requests are allowed
func (s *Store) AllowN(ctx context.Context, key string, n, limit, burst int, window time.Duration) (bool, int, time.Time, error) {
	now := time.Now()
	resetAt := now.Add(window)

	// Get or create entry
	s.mu.Lock()
	e, exists := s.entries[key]
	if !exists {
		// Check max entries limit
		if s.config.MaxEntries > 0 && len(s.entries) >= s.config.MaxEntries {
			s.mu.Unlock()
			// Evict oldest entry
			s.evictOldest()
			s.mu.Lock()
		}

		e = &entry{
			tokens:    burst,
			lastReset: now,
		}
		s.entries[key] = e
	}
	s.mu.Unlock()

	// Update tokens
	e.mu.Lock()
	defer e.mu.Unlock()

	// Calculate tokens to add based on time elapsed
	elapsed := now.Sub(e.lastReset)
	if elapsed >= window {
		// Full reset
		e.tokens = burst
		e.lastReset = now
	} else {
		// Add tokens based on rate
		tokensToAdd := int(float64(limit) * elapsed.Seconds() / window.Seconds())
		e.tokens = min(e.tokens+tokensToAdd, burst)
		if tokensToAdd > 0 {
			e.lastReset = now
		}
	}

	// Check if we have enough tokens
	if e.tokens >= n {
		e.tokens -= n
		return true, e.tokens, resetAt, nil
	}

	return false, e.tokens, resetAt, nil
}

// Reset resets the counter for a key
func (s *Store) Reset(ctx context.Context, key string) error {
	s.mu.Lock()
	delete(s.entries, key)
	s.mu.Unlock()
	return nil
}

// Close closes the store
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.done:
		// Already closed
		return nil
	default:
		close(s.done)
		return nil
	}
}

// cleanup periodically removes old entries
func (s *Store) cleanup() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.removeOldEntries()
		}
	}
}

// removeOldEntries removes entries that haven't been used recently
func (s *Store) removeOldEntries() {
	now := time.Now()
	threshold := 24 * time.Hour // Remove entries not used in 24 hours

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, e := range s.entries {
		e.mu.Lock()
		if now.Sub(e.lastReset) > threshold {
			delete(s.entries, key)
		}
		e.mu.Unlock()
	}
}

// evictOldest removes the oldest entry
func (s *Store) evictOldest() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) == 0 {
		return
	}

	// Find oldest entry
	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, e := range s.entries {
		e.mu.Lock()
		if first || e.lastReset.Before(oldestTime) {
			oldestKey = key
			oldestTime = e.lastReset
			first = false
		}
		e.mu.Unlock()
	}

	if oldestKey != "" {
		delete(s.entries, oldestKey)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
