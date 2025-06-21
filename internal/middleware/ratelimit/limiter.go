package ratelimit

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"gateway/internal/core"
	"gateway/internal/storage"
	"gateway/pkg/errors"
)

// Limiter defines the interface for rate limiting
type Limiter interface {
	// Allow checks if a request is allowed
	Allow(ctx context.Context, key string) error
	// AllowN checks if n requests are allowed
	AllowN(ctx context.Context, key string, n int) error
	// Wait blocks until a request is allowed
	Wait(ctx context.Context, key string) error
	// WaitN blocks until n requests are allowed
	WaitN(ctx context.Context, key string, n int) error
	// Limit returns the rate limit
	Limit() int
	// Burst returns the burst limit
	Burst() int
	// SetLimit updates the rate limit
	SetLimit(limit int)
	// SetBurst updates the burst limit
	SetBurst(burst int)
}

// StoreLimiter wraps a storage backend to implement the Limiter interface
type StoreLimiter struct {
	store  storage.LimiterStore
	limit  int
	burst  int
	window time.Duration
}

// NewStoreLimiter creates a new limiter with the given storage backend
func NewStoreLimiter(store storage.LimiterStore, limit, burst int) *StoreLimiter {
	return &StoreLimiter{
		store:  store,
		limit:  limit,
		burst:  burst,
		window: time.Second, // 1 second window for rate limiting
	}
}

// Allow checks if a request is allowed
func (l *StoreLimiter) Allow(ctx context.Context, key string) error {
	allowed, _, _, err := l.store.Allow(ctx, key, l.limit, l.burst, l.window)
	if err != nil {
		return fmt.Errorf("rate limit check failed: %w", err)
	}
	if !allowed {
		return errors.NewError(
			errors.ErrorTypeRateLimit,
			"rate limit exceeded",
		).WithDetail("key", key)
	}
	return nil
}

// AllowN checks if n requests are allowed
func (l *StoreLimiter) AllowN(ctx context.Context, key string, n int) error {
	allowed, _, _, err := l.store.AllowN(ctx, key, n, l.limit, l.burst, l.window)
	if err != nil {
		return fmt.Errorf("rate limit check failed: %w", err)
	}
	if !allowed {
		return errors.NewError(
			errors.ErrorTypeRateLimit,
			"rate limit exceeded",
		).WithDetail("key", key).WithDetail("requested", n)
	}
	return nil
}

// Wait blocks until a request is allowed
func (l *StoreLimiter) Wait(ctx context.Context, key string) error {
	// For storage-backed limiters, we don't support blocking wait
	// Just check if allowed
	return l.Allow(ctx, key)
}

// WaitN blocks until n requests are allowed
func (l *StoreLimiter) WaitN(ctx context.Context, key string, n int) error {
	// For storage-backed limiters, we don't support blocking wait
	// Just check if allowed
	return l.AllowN(ctx, key, n)
}

// Limit returns the rate limit
func (l *StoreLimiter) Limit() int {
	return l.limit
}

// Burst returns the burst limit
func (l *StoreLimiter) Burst() int {
	return l.burst
}

// SetLimit updates the rate limit
func (l *StoreLimiter) SetLimit(limit int) {
	l.limit = limit
}

// SetBurst updates the burst limit
func (l *StoreLimiter) SetBurst(burst int) {
	l.burst = burst
}

// TokenBucket implements token bucket algorithm (deprecated - use StoreLimiter with memory storage)
type TokenBucket struct {
	rate   int
	burst  int
	tokens map[string]*bucket
	mu     sync.RWMutex
	ticker *time.Ticker
	done   chan struct{}
}

type bucket struct {
	tokens int
	last   time.Time
}

// NewTokenBucket creates a new token bucket rate limiter
// Deprecated: Use NewStoreLimiter with memory storage instead
func NewTokenBucket(rate, burst int) *TokenBucket {
	tb := &TokenBucket{
		rate:   rate,
		burst:  burst,
		tokens: make(map[string]*bucket),
		ticker: time.NewTicker(time.Second),
		done:   make(chan struct{}),
	}

	go tb.refill()
	return tb
}

// Allow checks if request is allowed
func (tb *TokenBucket) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, exists := tb.tokens[key]
	if !exists {
		b = &bucket{
			tokens: tb.burst,
			last:   time.Now(),
		}
		tb.tokens[key] = b
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(b.last)
	// Use milliseconds for more precision
	tokensToAdd := int(elapsed.Milliseconds() * int64(tb.rate) / 1000)
	if tokensToAdd > 0 {
		b.tokens = min(b.tokens+tokensToAdd, tb.burst)
		b.last = now
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// Stop stops the token bucket
func (tb *TokenBucket) Stop() {
	close(tb.done)
	tb.ticker.Stop()
}

// refill periodically cleans up old buckets
func (tb *TokenBucket) refill() {
	for {
		select {
		case <-tb.ticker.C:
			tb.cleanup()
		case <-tb.done:
			return
		}
	}
}

// cleanup removes inactive buckets
func (tb *TokenBucket) cleanup() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	for key, b := range tb.tokens {
		if now.Sub(b.last) > 5*time.Minute {
			delete(tb.tokens, key)
		}
	}
}

// KeyFunc extracts rate limit key from request
type KeyFunc func(core.Request) string

// ByIP rate limits by IP address
func ByIP(req core.Request) string {
	// Extract IP from remote address (remove port)
	host, _, err := net.SplitHostPort(req.RemoteAddr())
	if err != nil {
		// If SplitHostPort fails, use the whole address
		return req.RemoteAddr()
	}
	return host
}

// ByPath rate limits by request path
func ByPath(req core.Request) string {
	return req.Path()
}

// ByIPAndPath rate limits by IP and path combination
func ByIPAndPath(req core.Request) string {
	return fmt.Sprintf("%s:%s", req.RemoteAddr(), req.Path())
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TokenBucketLimiter wraps TokenBucket to implement Limiter interface
type TokenBucketLimiter struct {
	tb *TokenBucket
	mu sync.RWMutex
}

// NewTokenBucketLimiter creates a new token bucket limiter
func NewTokenBucketLimiter(rate, burst int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		tb: NewTokenBucket(rate, burst),
	}
}

// Allow checks if a request is allowed
func (l *TokenBucketLimiter) Allow(ctx context.Context, key string) error {
	if !l.tb.Allow(key) {
		return errors.NewError(errors.ErrorTypeRateLimit, "rate limit exceeded").
			WithDetail("key", key)
	}
	return nil
}

// AllowN checks if n requests are allowed
func (l *TokenBucketLimiter) AllowN(ctx context.Context, key string, n int) error {
	// For simplicity, check each request individually
	for i := 0; i < n; i++ {
		if !l.tb.Allow(key) {
			return errors.NewError(errors.ErrorTypeRateLimit, "rate limit exceeded").
				WithDetail("key", key).
				WithDetail("requested", n).
				WithDetail("allowed", i)
		}
	}
	return nil
}

// Wait blocks until a request is allowed
func (l *TokenBucketLimiter) Wait(ctx context.Context, key string) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if l.tb.Allow(key) {
				return nil
			}
		}
	}
}

// WaitN blocks until n requests are allowed
func (l *TokenBucketLimiter) WaitN(ctx context.Context, key string, n int) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Try to get all tokens at once
			allowed := 0
			for i := 0; i < n; i++ {
				if l.tb.Allow(key) {
					allowed++
				} else {
					// Return tokens we took
					for j := 0; j < allowed; j++ {
						// Can't return tokens with current implementation
						// This is a limitation of the simple token bucket
					}
					break
				}
			}
			if allowed == n {
				return nil
			}
		}
	}
}

// Limit returns the rate limit
func (l *TokenBucketLimiter) Limit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tb.rate
}

// Burst returns the burst limit
func (l *TokenBucketLimiter) Burst() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tb.burst
}

// SetLimit updates the rate limit
func (l *TokenBucketLimiter) SetLimit(limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tb.rate = limit
}

// SetBurst updates the burst limit
func (l *TokenBucketLimiter) SetBurst(burst int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tb.burst = burst
}

// Ensure TokenBucketLimiter implements Limiter
var _ Limiter = (*TokenBucketLimiter)(nil)
