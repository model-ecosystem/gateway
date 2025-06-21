package storage

import (
	"context"
	"time"
)

// LimiterStore defines the interface for rate limiter storage
type LimiterStore interface {
	// Allow checks if a request is allowed for the given key
	Allow(ctx context.Context, key string, limit, burst int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, err error)

	// AllowN checks if n requests are allowed for the given key
	AllowN(ctx context.Context, key string, n, limit, burst int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, err error)

	// Reset resets the counter for the given key
	Reset(ctx context.Context, key string) error

	// Close closes the store and releases resources
	Close() error
}

// LimiterStoreConfig defines common configuration for limiter stores
type LimiterStoreConfig struct {
	// CleanupInterval is how often to clean up expired entries
	CleanupInterval time.Duration
	// MaxEntries is the maximum number of entries to keep (0 = unlimited)
	MaxEntries int
}

// DefaultConfig returns default configuration
func DefaultConfig() *LimiterStoreConfig {
	return &LimiterStoreConfig{
		CleanupInterval: 5 * time.Minute,
		MaxEntries:      10000, // Prevent unbounded memory growth
	}
}
