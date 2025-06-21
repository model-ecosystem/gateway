package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gateway/internal/storage"
)

func TestNewStore(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		store := NewStore(nil)
		defer store.Close()

		if store == nil {
			t.Fatal("expected store to be created")
		}
		if store.config == nil {
			t.Fatal("expected default config to be used")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &storage.LimiterStoreConfig{
			CleanupInterval: 1 * time.Minute,
			MaxEntries:      5000,
		}
		store := NewStore(config)
		defer store.Close()

		if store.config.CleanupInterval != config.CleanupInterval {
			t.Errorf("expected cleanup interval %v, got %v", config.CleanupInterval, store.config.CleanupInterval)
		}
		if store.config.MaxEntries != config.MaxEntries {
			t.Errorf("expected max entries %d, got %d", config.MaxEntries, store.config.MaxEntries)
		}
	})
}

func TestStore_Allow(t *testing.T) {
	ctx := context.Background()
	store := NewStore(storage.DefaultConfig())
	defer store.Close()

	tests := []struct {
		name      string
		key       string
		limit     int
		burst     int
		window    time.Duration
		wantAllow bool
	}{
		{
			name:      "first request allowed",
			key:       "test1",
			limit:     10,
			burst:     20,
			window:    time.Second,
			wantAllow: true,
		},
		{
			name:      "within burst allowed",
			key:       "test2",
			limit:     10,
			burst:     2,
			window:    time.Second,
			wantAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, remaining, resetAt, err := store.Allow(ctx, tt.key, tt.limit, tt.burst, tt.window)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if allowed != tt.wantAllow {
				t.Errorf("expected allowed=%v, got %v", tt.wantAllow, allowed)
			}
			if remaining < 0 {
				t.Errorf("expected remaining >= 0, got %d", remaining)
			}
			if resetAt.Before(time.Now()) {
				t.Errorf("expected resetAt to be in the future")
			}
		})
	}
}

func TestStore_AllowN(t *testing.T) {
	ctx := context.Background()
	store := NewStore(storage.DefaultConfig())
	defer store.Close()

	t.Run("allow multiple requests", func(t *testing.T) {
		key := "test-multi"
		limit := 10
		burst := 20
		window := time.Second

		// Request 5 at once
		allowed, remaining, _, err := store.AllowN(ctx, key, 5, limit, burst, window)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Error("expected request to be allowed")
		}
		if remaining != 15 { // 20 - 5
			t.Errorf("expected remaining=15, got %d", remaining)
		}
	})

	t.Run("deny when exceeding burst", func(t *testing.T) {
		key := "test-exceed"
		limit := 10
		burst := 5
		window := time.Second

		// Request more than burst
		allowed, remaining, _, err := store.AllowN(ctx, key, 10, limit, burst, window)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if allowed {
			t.Error("expected request to be denied")
		}
		if remaining != 5 {
			t.Errorf("expected remaining=5, got %d", remaining)
		}
	})
}

func TestStore_Reset(t *testing.T) {
	ctx := context.Background()
	store := NewStore(storage.DefaultConfig())
	defer store.Close()

	key := "test-reset"

	// Use some tokens
	_, _, _, err := store.AllowN(ctx, key, 5, 10, 10, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reset
	err = store.Reset(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have full burst available
	allowed, remaining, _, err := store.Allow(ctx, key, 10, 10, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected request to be allowed after reset")
	}
	if remaining != 9 { // 10 - 1
		t.Errorf("expected remaining=9 after reset, got %d", remaining)
	}
}

func TestStore_RateLimiting(t *testing.T) {
	ctx := context.Background()
	store := NewStore(storage.DefaultConfig())
	defer store.Close()

	key := "test-rate"
	limit := 2 // 2 requests per second
	burst := 3 // Allow burst of 3
	window := time.Second

	// Use up the burst
	for i := 0; i < burst; i++ {
		allowed, _, _, err := store.Allow(ctx, key, limit, burst, window)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Errorf("expected request %d to be allowed", i+1)
		}
	}

	// Next request should be denied
	allowed, _, _, err := store.Allow(ctx, key, limit, burst, window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected request to be denied after burst")
	}

	// Wait for tokens to replenish
	time.Sleep(600 * time.Millisecond) // Should get ~1 token back

	// Should be allowed again
	allowed, _, _, err = store.Allow(ctx, key, limit, burst, window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected request to be allowed after waiting")
	}
}

func TestStore_Cleanup(t *testing.T) {
	ctx := context.Background()
	config := &storage.LimiterStoreConfig{
		CleanupInterval: 100 * time.Millisecond,
		MaxEntries:      10,
	}
	store := NewStore(config)
	defer store.Close()

	// Create multiple entries
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("test-cleanup-%d", i)
		_, _, _, err := store.Allow(ctx, key, 10, 10, time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Verify entries exist
	store.mu.RLock()
	entryCount := len(store.entries)
	store.mu.RUnlock()

	if entryCount != 5 {
		t.Errorf("expected 5 entries, got %d", entryCount)
	}

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Old entries should still exist (not expired yet)
	store.mu.RLock()
	entryCount = len(store.entries)
	store.mu.RUnlock()

	if entryCount != 5 {
		t.Errorf("expected 5 entries after cleanup, got %d", entryCount)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	store := NewStore(storage.DefaultConfig())
	defer store.Close()

	key := "test-concurrent"
	limit := 100
	burst := 200
	window := time.Second

	// Run concurrent requests
	concurrency := 10
	requestsPerGoroutine := 5

	errChan := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < requestsPerGoroutine; j++ {
				_, _, _, err := store.Allow(ctx, key, limit, burst, window)
				if err != nil {
					errChan <- err
					return
				}
			}
			errChan <- nil
		}()
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent access error: %v", err)
		}
	}

	// Check final state
	_, remaining, _, err := store.Allow(ctx, key, limit, burst, window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedRemaining := burst - (concurrency*requestsPerGoroutine + 1)
	if remaining != expectedRemaining {
		t.Errorf("expected remaining=%d, got %d", expectedRemaining, remaining)
	}
}

func TestStore_Close(t *testing.T) {
	store := NewStore(storage.DefaultConfig())

	// Close should not error
	err := store.Close()
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}

	// Multiple closes should be safe
	err = store.Close()
	if err != nil {
		t.Errorf("unexpected error on second close: %v", err)
	}
}
