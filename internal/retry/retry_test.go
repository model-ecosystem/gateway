package retry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetrier_Do(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		config := Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
		}
		r := New(config)

		attempts := 0
		err := r.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got: %d", attempts)
		}
	})

	t.Run("success after retry", func(t *testing.T) {
		config := Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
		}
		r := New(config)

		attempts := 0
		err := r.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary error")
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got: %d", attempts)
		}
	})

	t.Run("exhausted retries", func(t *testing.T) {
		config := Config{
			MaxAttempts:  2,
			InitialDelay: 10 * time.Millisecond,
		}
		r := New(config)

		attempts := 0
		testErr := errors.New("persistent error")
		err := r.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return testErr
		})

		var retryErr *Error
		if !errors.As(err, &retryErr) {
			t.Errorf("Expected retry error, got: %v", err)
		}
		if retryErr.Attempts != 3 { // MaxAttempts + 1
			t.Errorf("Expected 3 attempts, got: %d", retryErr.Attempts)
		}
		if !errors.Is(err, testErr) {
			t.Error("Expected error to wrap original error")
		}
		if attempts != 3 {
			t.Errorf("Expected 3 actual attempts, got: %d", attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		config := Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
		}
		r := New(config)

		attempts := 0
		nonRetryableErr := NewNonRetryableError(errors.New("non-retryable"))
		err := r.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return nonRetryableErr
		})

		if err != nonRetryableErr {
			t.Errorf("Expected non-retryable error, got: %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got: %d", attempts)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		config := Config{
			MaxAttempts:  3,
			InitialDelay: 100 * time.Millisecond,
		}
		r := New(config)

		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := r.Do(ctx, func(ctx context.Context) error {
			attempts++
			return errors.New("error")
		})

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context canceled error, got: %v", err)
		}
		if attempts > 2 {
			t.Errorf("Expected at most 2 attempts, got: %d", attempts)
		}
	})
}

func TestRetrier_DoWithData(t *testing.T) {
	config := Config{
		MaxAttempts:  2,
		InitialDelay: 10 * time.Millisecond,
	}
	r := New(config)

	t.Run("success with data", func(t *testing.T) {
		attempts := 0
		data, err := r.DoWithData(context.Background(), func(ctx context.Context) (interface{}, error) {
			attempts++
			if attempts < 2 {
				return nil, errors.New("temporary error")
			}
			return "success", nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if data != "success" {
			t.Errorf("Expected 'success', got: %v", data)
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got: %d", attempts)
		}
	})
}

func TestRetrier_calculateDelay(t *testing.T) {
	t.Run("exponential backoff", func(t *testing.T) {
		config := Config{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       false,
		}
		r := New(config)

		delays := []time.Duration{
			100 * time.Millisecond,  // attempt 0
			200 * time.Millisecond,  // attempt 1
			400 * time.Millisecond,  // attempt 2
			800 * time.Millisecond,  // attempt 3
			1600 * time.Millisecond, // attempt 4
		}

		for i, expected := range delays {
			actual := r.calculateDelay(i)
			if actual != expected {
				t.Errorf("Attempt %d: expected %v, got %v", i, expected, actual)
			}
		}
	})

	t.Run("max delay cap", func(t *testing.T) {
		config := Config{
			InitialDelay: 1 * time.Second,
			MaxDelay:     3 * time.Second,
			Multiplier:   2.0,
			Jitter:       false,
		}
		r := New(config)

		// Should be capped at 3 seconds
		delay := r.calculateDelay(10)
		if delay != 3*time.Second {
			t.Errorf("Expected max delay of 3s, got: %v", delay)
		}
	})

	t.Run("jitter", func(t *testing.T) {
		config := Config{
			InitialDelay: 100 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       true,
		}
		r := New(config)

		// Run multiple times to ensure jitter is applied
		seen := make(map[time.Duration]bool)
		for i := 0; i < 10; i++ {
			delay := r.calculateDelay(1)
			seen[delay] = true

			// Should be around 200ms ± 25%
			if delay < 150*time.Millisecond || delay > 250*time.Millisecond {
				t.Errorf("Delay outside expected range: %v", delay)
			}
		}

		// Should have different values due to jitter
		if len(seen) < 2 {
			t.Error("Expected jitter to produce different delays")
		}
	})
}

func TestDefaultRetryableFunc(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "regular error",
			err:       errors.New("some error"),
			retryable: true,
		},
		{
			name:      "context canceled",
			err:       context.Canceled,
			retryable: false,
		},
		{
			name:      "context deadline exceeded",
			err:       context.DeadlineExceeded,
			retryable: false,
		},
		{
			name:      "non-retryable error",
			err:       NewNonRetryableError(errors.New("no retry")),
			retryable: false,
		},
		{
			name:      "wrapped context error",
			err:       errors.Join(errors.New("wrapper"), context.Canceled),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DefaultRetryableFunc(tt.err)
			if result != tt.retryable {
				t.Errorf("Expected retryable=%v for %v, got %v", tt.retryable, tt.err, result)
			}
		})
	}
}

func TestRetrier_Concurrent(t *testing.T) {
	config := Config{
		MaxAttempts:  2,
		InitialDelay: 10 * time.Millisecond,
	}
	r := New(config)

	var totalAttempts int32
	concurrency := 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			err := r.Do(context.Background(), func(ctx context.Context) error {
				atomic.AddInt32(&totalAttempts, 1)
				if id%2 == 0 {
					return errors.New("error")
				}
				return nil
			})

			if id%2 == 0 && err == nil {
				t.Errorf("Goroutine %d: expected error", id)
			}
			if id%2 != 0 && err != nil {
				t.Errorf("Goroutine %d: unexpected error: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Verify attempts
	attempts := atomic.LoadInt32(&totalAttempts)
	// 5 successful (1 attempt each) + 5 failed (3 attempts each) = 20
	if attempts != 20 {
		t.Errorf("Expected 20 total attempts, got: %d", attempts)
	}
}
