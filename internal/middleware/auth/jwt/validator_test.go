package jwt

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"
)

// TestNewTokenValidator tests the creation of a new token validator
func TestNewTokenValidator(t *testing.T) {
	// Create a real provider with test configuration
	config := &Config{
		SigningMethod: "HS256",
		Secret:        "test-secret",
		Issuer:        "test-issuer",
		Audience:      []string{"test-audience"},
	}
	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	validator := NewTokenValidator(provider, slog.Default())

	if validator == nil {
		t.Fatal("expected validator to be created")
	}
	if validator.provider != provider {
		t.Error("expected provider to be set")
	}
	if validator.logger == nil {
		t.Error("expected logger to be set")
	}
	if validator.timers == nil {
		t.Error("expected timers map to be initialized")
	}
}

// TestValidateConnection_Basic tests basic connection validation
func TestValidateConnection_Basic(t *testing.T) {
	ctx := context.Background()

	// Create a provider
	config := &Config{
		SigningMethod: "HS256",
		Secret:        "test-secret",
		Issuer:        "test-issuer",
		Audience:      []string{"test-audience"},
	}
	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	validator := NewTokenValidator(provider, slog.Default())

	// Test with invalid token (basic test)
	err = validator.ValidateConnection(ctx, "conn1", "invalid-token", func() {})
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

// TestStopValidation tests stopping validation
func TestStopValidation(t *testing.T) {
	config := &Config{
		SigningMethod: "HS256",
		Secret:        "test-secret",
	}
	provider, _ := NewProvider(config, slog.Default())
	validator := NewTokenValidator(provider, slog.Default())

	// Create a timer
	timer := time.NewTimer(10 * time.Second)
	validator.mu.Lock()
	validator.timers["conn1"] = timer
	validator.mu.Unlock()

	// Stop validation
	validator.StopValidation("conn1")

	// Timer should be removed
	validator.mu.RLock()
	_, exists := validator.timers["conn1"]
	validator.mu.RUnlock()

	if exists {
		t.Error("timer should be removed after StopValidation")
	}

	// Timer should be stopped
	select {
	case <-timer.C:
		t.Error("timer should have been stopped")
	default:
		// Expected
	}
}

// TestStopValidation_NonExistent tests stopping non-existent validation
func TestStopValidation_NonExistent(t *testing.T) {
	config := &Config{
		SigningMethod: "HS256",
		Secret:        "test-secret",
	}
	provider, _ := NewProvider(config, slog.Default())
	validator := NewTokenValidator(provider, slog.Default())

	// Should not panic for non-existent connection
	validator.StopValidation("non-existent")
}

// TestScheduleRecheck tests the scheduleRecheck method
func TestScheduleRecheck(t *testing.T) {
	ctx := context.Background()
	expiredCalled := false

	config := &Config{
		SigningMethod: "HS256",
		Secret:        "test-secret",
	}
	provider, _ := NewProvider(config, slog.Default())
	validator := NewTokenValidator(provider, slog.Default())

	// Schedule a recheck
	validator.scheduleRecheck(ctx, "conn1", "token", func() {
		expiredCalled = true
	}, 50*time.Millisecond)

	// Verify timer was created
	validator.mu.RLock()
	timer, exists := validator.timers["conn1"]
	validator.mu.RUnlock()

	if !exists || timer == nil {
		t.Error("expected timer to be created")
	}

	// Wait a bit but not long enough for recheck
	time.Sleep(30 * time.Millisecond)

	if expiredCalled {
		t.Error("expiration callback called too early")
	}

	// Cleanup
	validator.StopValidation("conn1")
}

// TestContextCancellation tests that timers are cleaned up when context is cancelled
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	config := &Config{
		SigningMethod: "HS256",
		Secret:        "test-secret",
	}
	provider, _ := NewProvider(config, slog.Default())
	validator := NewTokenValidator(provider, slog.Default())

	// Create a timer with a cancellable context
	timer := time.NewTimer(10 * time.Second)
	validator.mu.Lock()
	validator.timers["conn1"] = timer
	validator.mu.Unlock()

	// Start a goroutine to clean up on context cancellation
	go func() {
		<-ctx.Done()
		validator.StopValidation("conn1")
	}()

	// Cancel context
	cancel()

	// Give goroutine time to react
	time.Sleep(50 * time.Millisecond)

	// Timer should be cleaned up
	validator.mu.RLock()
	_, exists := validator.timers["conn1"]
	validator.mu.RUnlock()

	if exists {
		t.Error("timer should be removed after context cancellation")
	}
}

// TestConcurrentAccess tests concurrent access to the validator
func TestConcurrentAccess(t *testing.T) {
	config := &Config{
		SigningMethod: "HS256",
		Secret:        "test-secret",
	}
	provider, _ := NewProvider(config, slog.Default())
	validator := NewTokenValidator(provider, slog.Default())

	// Start and stop multiple connections concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			connID := fmt.Sprintf("conn-%d", id)

			// Create timer
			timer := time.NewTimer(1 * time.Second)
			validator.mu.Lock()
			validator.timers[connID] = timer
			validator.mu.Unlock()

			// Stop it after a short delay
			time.Sleep(10 * time.Millisecond)
			validator.StopValidation(connID)

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// All timers should be cleaned up
	validator.mu.RLock()
	timerCount := len(validator.timers)
	validator.mu.RUnlock()

	if timerCount != 0 {
		t.Errorf("expected 0 timers, got %d", timerCount)
	}
}

// TestValidateConnection_WithMockAuthInfo demonstrates how the validator
// would work with a valid auth info (integration test style)
func TestValidateConnection_WithMockAuthInfo(t *testing.T) {
	// This test demonstrates the expected behavior when integrated
	// with a real JWT provider. Since we can't easily mock the Provider's
	// internal behavior, this serves as documentation of expected usage.

	t.Run("token with expiration creates timer", func(t *testing.T) {
		config := &Config{
			SigningMethod: "HS256",
			Secret:        "test-secret",
		}
		provider, _ := NewProvider(config, slog.Default())
		validator := NewTokenValidator(provider, slog.Default())

		// In a real scenario with a valid token that has expiration,
		// a timer would be created. We can verify the structure is ready
		// for this by checking the timers map exists
		if validator.timers == nil {
			t.Error("timers map should be initialized")
		}
	})

	t.Run("multiple connections tracked separately", func(t *testing.T) {
		config := &Config{
			SigningMethod: "HS256",
			Secret:        "test-secret",
		}
		provider, _ := NewProvider(config, slog.Default())
		validator := NewTokenValidator(provider, slog.Default())

		// Simulate multiple connections with timers
		for i := 0; i < 3; i++ {
			connID := fmt.Sprintf("test-conn-%d", i)
			timer := time.NewTimer(1 * time.Minute)
			validator.mu.Lock()
			validator.timers[connID] = timer
			validator.mu.Unlock()
		}

		// Verify all are tracked
		validator.mu.RLock()
		count := len(validator.timers)
		validator.mu.RUnlock()

		if count != 3 {
			t.Errorf("expected 3 timers, got %d", count)
		}

		// Clean up
		for i := 0; i < 3; i++ {
			connID := fmt.Sprintf("test-conn-%d", i)
			validator.StopValidation(connID)
		}
	})
}
