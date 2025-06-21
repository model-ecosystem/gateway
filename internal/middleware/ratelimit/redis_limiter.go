package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gateway/pkg/errors"
)

// RedisLimiter implements rate limiting using Redis
type RedisLimiter struct {
	client *redis.Client
	limit  int
	burst  int
	logger *slog.Logger
	mu     sync.RWMutex
	
	// Fallback to in-memory limiter if Redis is unavailable
	fallback Limiter
}

// NewRedisLimiter creates a new Redis-based rate limiter
func NewRedisLimiter(client *redis.Client, limit int, burst int, logger *slog.Logger) *RedisLimiter {
	return &RedisLimiter{
		client:   client,
		limit:    limit,
		burst:    burst,
		logger:   logger,
		fallback: NewTokenBucketLimiter(limit, burst), // Fallback to in-memory
	}
}

// Allow checks if a request is allowed for the given key
func (l *RedisLimiter) Allow(ctx context.Context, key string) error {
	// Use sliding window algorithm with Redis
	now := time.Now()
	windowStart := now.Add(-time.Second)
	
	// Create a unique member ID for this request
	member := fmt.Sprintf("%d:%s", now.UnixNano(), key)
	
	// Redis pipeline for atomic operations
	pipe := l.client.Pipeline()
	
	// Add current request to sorted set with timestamp as score
	pipe.ZAdd(ctx, l.redisKey(key), redis.Z{
		Score:  float64(now.UnixNano()),
		Member: member,
	})
	
	// Remove old entries outside the window
	pipe.ZRemRangeByScore(ctx, l.redisKey(key), "-inf", fmt.Sprintf("%d", windowStart.UnixNano()))
	
	// Count requests in current window
	countCmd := pipe.ZCard(ctx, l.redisKey(key))
	
	// Set expiration on the key (2x window size to be safe)
	pipe.Expire(ctx, l.redisKey(key), 2*time.Second)
	
	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		l.logger.Warn("Redis rate limit error, falling back to in-memory",
			"error", err,
			"key", key,
		)
		// Fall back to in-memory limiter
		return l.fallback.Allow(ctx, key)
	}
	
	// Check if count exceeds limit
	count := countCmd.Val()
	if count > int64(l.limit) {
		// Check if we can use burst capacity
		if count <= int64(l.burst) {
			l.logger.Debug("Request allowed using burst capacity",
				"key", key,
				"count", count,
				"limit", l.limit,
				"burst", l.burst,
			)
			return nil
		}
		
		return errors.NewError(errors.ErrorTypeRateLimit, "rate limit exceeded").
			WithDetail("key", key).
			WithDetail("count", count).
			WithDetail("limit", l.limit)
	}
	
	return nil
}

// AllowN checks if n requests are allowed
func (l *RedisLimiter) AllowN(ctx context.Context, key string, n int) error {
	// For simplicity, check if current count + n would exceed limit
	now := time.Now()
	windowStart := now.Add(-time.Second)
	
	// Count current requests in window
	count, err := l.client.ZCount(ctx, l.redisKey(key), 
		fmt.Sprintf("%d", windowStart.UnixNano()),
		fmt.Sprintf("%d", now.UnixNano()),
	).Result()
	
	if err != nil {
		l.logger.Warn("Redis rate limit error, falling back to in-memory",
			"error", err,
			"key", key,
		)
		return l.fallback.AllowN(ctx, key, n)
	}
	
	// Check if adding n requests would exceed limit
	newCount := count + int64(n)
	if newCount > int64(l.limit) {
		// Check burst
		if newCount <= int64(l.burst) {
			// Allow using burst, but add all n requests
			for i := 0; i < n; i++ {
				member := fmt.Sprintf("%d:%s:%d", now.UnixNano(), key, i)
				l.client.ZAdd(ctx, l.redisKey(key), redis.Z{
					Score:  float64(now.UnixNano()),
					Member: member,
				})
			}
			return nil
		}
		
		return errors.NewError(errors.ErrorTypeRateLimit, "rate limit exceeded").
			WithDetail("key", key).
			WithDetail("requested", n).
			WithDetail("available", l.limit-int(count))
	}
	
	// Add all n requests
	pipe := l.client.Pipeline()
	for i := 0; i < n; i++ {
		member := fmt.Sprintf("%d:%s:%d", now.UnixNano(), key, i)
		pipe.ZAdd(ctx, l.redisKey(key), redis.Z{
			Score:  float64(now.UnixNano()),
			Member: member,
		})
	}
	pipe.Expire(ctx, l.redisKey(key), 2*time.Second)
	_, err = pipe.Exec(ctx)
	
	return err
}

// Wait blocks until a request is allowed
func (l *RedisLimiter) Wait(ctx context.Context, key string) error {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Try to get token immediately
	if err := l.Allow(ctx, key); err == nil {
		return nil
	}
	
	// Calculate wait time based on current rate
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := l.Allow(ctx, key); err == nil {
				return nil
			}
		}
	}
}

// WaitN blocks until n requests are allowed
func (l *RedisLimiter) WaitN(ctx context.Context, key string, n int) error {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Try to get tokens immediately
	if err := l.AllowN(ctx, key, n); err == nil {
		return nil
	}
	
	// Wait with exponential backoff
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := l.AllowN(ctx, key, n); err == nil {
				return nil
			}
		}
	}
}

// Limit returns the rate limit
func (l *RedisLimiter) Limit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.limit
}

// Burst returns the burst limit
func (l *RedisLimiter) Burst() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.burst
}

// SetLimit updates the rate limit
func (l *RedisLimiter) SetLimit(limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = limit
	l.fallback.SetLimit(limit)
}

// SetBurst updates the burst limit
func (l *RedisLimiter) SetBurst(burst int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.burst = burst
	l.fallback.SetBurst(burst)
}

// redisKey generates the Redis key for a given rate limit key
func (l *RedisLimiter) redisKey(key string) string {
	return fmt.Sprintf("ratelimit:%s", key)
}

// Ensure RedisLimiter implements Limiter interface
var _ Limiter = (*RedisLimiter)(nil)