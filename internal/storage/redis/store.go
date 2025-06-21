package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gateway/internal/storage"
)

// Client defines the interface for Redis operations
type Client interface {
	// Eval executes a Lua script
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
	// Del deletes keys
	Del(ctx context.Context, keys ...string) error
	// Close closes the connection
	Close() error
}

// Store implements LimiterStore using Redis
type Store struct {
	client Client
	config *storage.LimiterStoreConfig
	script string // Lua script for atomic rate limiting
}

// NewStore creates a new Redis store
func NewStore(client Client, config *storage.LimiterStoreConfig) *Store {
	if config == nil {
		config = storage.DefaultConfig()
	}

	// Lua script for atomic rate limiting with sliding window
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local burst = tonumber(ARGV[4])
		local n = tonumber(ARGV[5])
		
		-- Clean old entries
		redis.call('ZREMRANGEBYSCORE', key, 0, now - window * 1000)
		
		-- Count current requests
		local current = redis.call('ZCARD', key)
		
		-- Check if we can allow the request
		if current + n <= burst then
			-- Add new entries
			for i = 1, n do
				redis.call('ZADD', key, now, now .. ':' .. i .. ':' .. math.random())
			end
			-- Set expiration
			redis.call('EXPIRE', key, window + 1)
			return {1, burst - current - n}
		else
			return {0, burst - current}
		end
	`

	return &Store{
		client: client,
		config: config,
		script: script,
	}
}

// Allow checks if a request is allowed
func (s *Store) Allow(ctx context.Context, key string, limit, burst int, window time.Duration) (bool, int, time.Time, error) {
	return s.AllowN(ctx, key, 1, limit, burst, window)
}

// AllowN checks if n requests are allowed
func (s *Store) AllowN(ctx context.Context, key string, n, limit, burst int, window time.Duration) (bool, int, time.Time, error) {
	now := time.Now()
	resetAt := now.Add(window)

	// Use Redis key with prefix
	redisKey := fmt.Sprintf("ratelimit:%s", key)

	// Execute Lua script
	result, err := s.client.Eval(ctx, s.script, []string{redisKey},
		now.UnixMilli(),       // current time in milliseconds
		int(window.Seconds()), // window in seconds
		limit,                 // requests per window
		burst,                 // burst capacity
		n,                     // number of requests
	)

	if err != nil {
		return false, 0, resetAt, fmt.Errorf("failed to execute rate limit script: %w", err)
	}

	// Parse result
	res, ok := result.([]interface{})
	if !ok || len(res) != 2 {
		return false, 0, resetAt, errors.New("invalid rate limit script result")
	}

	allowed, ok1 := res[0].(int64)
	remaining, ok2 := res[1].(int64)
	if !ok1 || !ok2 {
		return false, 0, resetAt, errors.New("invalid rate limit script result types")
	}

	return allowed == 1, int(remaining), resetAt, nil
}

// Reset resets the counter for a key
func (s *Store) Reset(ctx context.Context, key string) error {
	redisKey := fmt.Sprintf("ratelimit:%s", key)
	return s.client.Del(ctx, redisKey)
}

// Close closes the store
func (s *Store) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
