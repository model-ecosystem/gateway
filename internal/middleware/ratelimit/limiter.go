package ratelimit

import (
	"fmt"
	"gateway/internal/core"
	"sync"
	"time"
)

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	Allow(key string) bool
}

// TokenBucket implements token bucket algorithm
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
	return req.RemoteAddr()
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
