package redis

import (
	"context"
	
	"github.com/redis/go-redis/v9"
)

// ClientAdapter adapts go-redis client to our interface
type ClientAdapter struct {
	client redis.UniversalClient
}

// NewClientAdapter creates a new client adapter
func NewClientAdapter(client redis.UniversalClient) *ClientAdapter {
	return &ClientAdapter{client: client}
}

// Eval executes a Lua script
func (c *ClientAdapter) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return c.client.Eval(ctx, script, keys, args...).Result()
}

// Del deletes keys
func (c *ClientAdapter) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Close closes the connection
func (c *ClientAdapter) Close() error {
	return c.client.Close()
}