package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"gateway/internal/storage"
)

// mockClient implements the Client interface for testing
type mockClient struct {
	evalFunc func(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
	delFunc  func(ctx context.Context, keys ...string) error
	closed   bool
}

func (m *mockClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	if m.evalFunc != nil {
		return m.evalFunc(ctx, script, keys, args...)
	}
	// Default behavior - simulate successful rate limit
	return []interface{}{int64(1), int64(5)}, nil
}

func (m *mockClient) Del(ctx context.Context, keys ...string) error {
	if m.delFunc != nil {
		return m.delFunc(ctx, keys...)
	}
	return nil
}

func (m *mockClient) Close() error {
	m.closed = true
	return nil
}

func TestNewStore(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		client := &mockClient{}
		store := NewStore(client, nil)

		if store == nil {
			t.Fatal("expected store to be created")
		}
		if store.config == nil {
			t.Fatal("expected default config to be used")
		}
		if store.script == "" {
			t.Fatal("expected Lua script to be set")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		client := &mockClient{}
		config := &storage.LimiterStoreConfig{
			CleanupInterval: 1 * time.Minute,
			MaxEntries:      5000,
		}
		store := NewStore(client, config)

		if store.config != config {
			t.Error("expected custom config to be used")
		}
	})
}

func TestStore_Allow(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		evalResult    interface{}
		evalErr       error
		wantAllowed   bool
		wantRemaining int
		wantErr       bool
	}{
		{
			name:          "request allowed",
			evalResult:    []interface{}{int64(1), int64(10)},
			wantAllowed:   true,
			wantRemaining: 10,
		},
		{
			name:          "request denied",
			evalResult:    []interface{}{int64(0), int64(0)},
			wantAllowed:   false,
			wantRemaining: 0,
		},
		{
			name:    "redis error",
			evalErr: errors.New("redis connection failed"),
			wantErr: true,
		},
		{
			name:       "invalid result type",
			evalResult: "invalid",
			wantErr:    true,
		},
		{
			name:       "invalid result length",
			evalResult: []interface{}{int64(1)},
			wantErr:    true,
		},
		{
			name:       "invalid allowed type",
			evalResult: []interface{}{"1", int64(10)},
			wantErr:    true,
		},
		{
			name:       "invalid remaining type",
			evalResult: []interface{}{int64(1), "10"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockClient{
				evalFunc: func(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
					if tt.evalErr != nil {
						return nil, tt.evalErr
					}
					return tt.evalResult, nil
				},
			}
			store := NewStore(client, nil)

			allowed, remaining, resetAt, err := store.Allow(ctx, "test-key", 10, 20, time.Second)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if allowed != tt.wantAllowed {
				t.Errorf("expected allowed=%v, got %v", tt.wantAllowed, allowed)
			}
			if remaining != tt.wantRemaining {
				t.Errorf("expected remaining=%d, got %d", tt.wantRemaining, remaining)
			}
			if resetAt.Before(time.Now()) {
				t.Error("expected resetAt to be in the future")
			}
		})
	}
}

func TestStore_AllowN(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates to Eval with correct parameters", func(t *testing.T) {
		var capturedArgs []interface{}
		client := &mockClient{
			evalFunc: func(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
				capturedArgs = args
				return []interface{}{int64(1), int64(15)}, nil
			},
		}
		store := NewStore(client, nil)

		allowed, remaining, _, err := store.AllowN(ctx, "test-key", 5, 10, 20, time.Second)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Error("expected request to be allowed")
		}
		if remaining != 15 {
			t.Errorf("expected remaining=15, got %d", remaining)
		}

		// Check that n=5 was passed to Lua script
		if len(capturedArgs) < 5 || capturedArgs[4] != 5 {
			t.Errorf("expected n=5 to be passed to Lua script, got %v", capturedArgs)
		}
	})
}

func TestStore_Reset(t *testing.T) {
	ctx := context.Background()

	t.Run("successful reset", func(t *testing.T) {
		var capturedKey string
		client := &mockClient{
			delFunc: func(ctx context.Context, keys ...string) error {
				if len(keys) > 0 {
					capturedKey = keys[0]
				}
				return nil
			},
		}
		store := NewStore(client, nil)

		err := store.Reset(ctx, "test-key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedKey != "ratelimit:test-key" {
			t.Errorf("expected key 'ratelimit:test-key', got '%s'", capturedKey)
		}
	})

	t.Run("reset error", func(t *testing.T) {
		client := &mockClient{
			delFunc: func(ctx context.Context, keys ...string) error {
				return errors.New("redis error")
			},
		}
		store := NewStore(client, nil)

		err := store.Reset(ctx, "test-key")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Close(t *testing.T) {
	t.Run("closes client", func(t *testing.T) {
		client := &mockClient{}
		store := NewStore(client, nil)

		err := store.Close()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !client.closed {
			t.Error("expected client to be closed")
		}
	})

	t.Run("handles nil client", func(t *testing.T) {
		store := &Store{}
		err := store.Close()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestStore_LuaScript(t *testing.T) {
	ctx := context.Background()

	t.Run("script parameters", func(t *testing.T) {
		var capturedScript string
		var capturedKeys []string
		var capturedArgs []interface{}

		client := &mockClient{
			evalFunc: func(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
				capturedScript = script
				capturedKeys = keys
				capturedArgs = args
				return []interface{}{int64(1), int64(10)}, nil
			},
		}
		store := NewStore(client, nil)

		now := time.Now()
		_, _, _, err := store.AllowN(ctx, "test-key", 2, 10, 20, time.Second)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify script is set
		if capturedScript == "" {
			t.Error("expected Lua script to be passed")
		}

		// Verify key
		if len(capturedKeys) != 1 || capturedKeys[0] != "ratelimit:test-key" {
			t.Errorf("expected keys=['ratelimit:test-key'], got %v", capturedKeys)
		}

		// Verify args
		if len(capturedArgs) != 5 {
			t.Fatalf("expected 5 args, got %d", len(capturedArgs))
		}

		// Check timestamp (should be close to now)
		timestamp, ok := capturedArgs[0].(int64)
		if !ok {
			t.Errorf("expected args[0] to be int64, got %T", capturedArgs[0])
		}
		if timestamp < now.UnixMilli()-100 || timestamp > now.UnixMilli()+100 {
			t.Error("timestamp not within expected range")
		}

		// Check other args
		expectedArgs := []interface{}{
			1,  // window in seconds
			10, // limit
			20, // burst
			2,  // n
		}
		for i := 1; i < 5; i++ {
			if capturedArgs[i] != expectedArgs[i-1] {
				t.Errorf("args[%d]: expected %v, got %v", i, expectedArgs[i-1], capturedArgs[i])
			}
		}
	})
}