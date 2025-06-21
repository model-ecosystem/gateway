package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreakerStates(t *testing.T) {
	config := Config{
		MaxFailures:      3,
		FailureThreshold: 0.5,
		Timeout:          100 * time.Millisecond,
		MaxRequests:      1,
		Interval:         1 * time.Second,
	}
	
	cb := New(config)
	
	// Initial state should be closed
	if cb.State() != StateClosed {
		t.Errorf("Expected initial state to be closed, got %v", cb.State())
	}
	
	// Should allow requests in closed state
	if !cb.Allow() {
		t.Error("Expected to allow request in closed state")
	}
	
	// Record failures to open the circuit
	for i := 0; i < 3; i++ {
		cb.Failure()
	}
	
	// State should be open
	if cb.State() != StateOpen {
		t.Errorf("Expected state to be open after failures, got %v", cb.State())
	}
	
	// Should not allow requests in open state
	if cb.Allow() {
		t.Error("Expected to block request in open state")
	}
	
	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)
	
	// Should allow one request in half-open state
	if !cb.Allow() {
		t.Error("Expected to allow request in half-open state")
	}
	
	// Second request should be blocked
	if cb.Allow() {
		t.Error("Expected to block second request in half-open state")
	}
	
	// Success should close the circuit
	cb.Success()
	if cb.State() != StateClosed {
		t.Errorf("Expected state to be closed after success in half-open, got %v", cb.State())
	}
}

func TestCircuitBreakerCall(t *testing.T) {
	config := Config{
		MaxFailures:      3,
		FailureThreshold: 1.0, // Only use absolute threshold, not percentage
		Timeout:          100 * time.Millisecond,
		MaxRequests:      1,
	}
	
	cb := New(config)
	
	// Test successful call
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected successful call, got error: %v", err)
	}
	
	// Test failing calls - MaxFailures=3 means circuit opens when failures >= 3
	testErr := errors.New("test error")
	
	// First two failures should pass through
	for i := 0; i < 2; i++ {
		err = cb.Call(context.Background(), func(ctx context.Context) error {
			return testErr
		})
		if err != testErr {
			t.Errorf("Call %d: Expected test error, got: %v", i+1, err)
		}
		
		// Verify circuit is still closed
		if cb.State() != StateClosed {
			t.Errorf("After %d failures, expected circuit to be closed, got %v", i+1, cb.State())
		}
	}
	
	// Third failure should trigger the circuit to open
	err = cb.Call(context.Background(), func(ctx context.Context) error {
		return testErr
	})
	if err != testErr {
		t.Errorf("Expected test error on third failure, got: %v", err)
	}
	
	// Circuit should be open now
	if cb.State() != StateOpen {
		t.Errorf("Expected circuit to be open after 3 failures, got %v", cb.State())
	}
	
	// Next call should fail with ErrCircuitOpen
	err = cb.Call(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got: %v", err)
	}
}

func TestCircuitBreakerThreshold(t *testing.T) {
	config := Config{
		MaxFailures:      10,
		FailureThreshold: 0.5,
		Timeout:          100 * time.Millisecond,
	}
	
	cb := New(config)
	
	// 4 successes, 3 failures (failure rate = 42%)
	for i := 0; i < 4; i++ {
		cb.Success()
	}
	for i := 0; i < 3; i++ {
		cb.Failure()
	}
	
	// Should still be closed
	if cb.State() != StateClosed {
		t.Errorf("Expected state to be closed with 42%% failure rate, got %v", cb.State())
	}
	
	// One more failure brings it to 50%
	cb.Failure()
	
	// Should be open now
	if cb.State() != StateOpen {
		t.Errorf("Expected state to be open with 50%% failure rate, got %v", cb.State())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	config := Config{
		MaxFailures: 1,
		Timeout:     100 * time.Millisecond,
	}
	
	cb := New(config)
	
	// Open the circuit
	cb.Failure()
	if cb.State() != StateOpen {
		t.Error("Expected circuit to be open")
	}
	
	// Reset
	cb.Reset()
	
	// Should be closed
	if cb.State() != StateClosed {
		t.Error("Expected circuit to be closed after reset")
	}
	
	// Stats should be cleared
	stats := cb.Stats()
	if stats.Failures != 0 || stats.Successes != 0 {
		t.Error("Expected stats to be cleared after reset")
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	config := Config{
		MaxFailures: 100,
		Timeout:     100 * time.Millisecond,
		MaxRequests: 10,
	}
	
	cb := New(config)
	
	var wg sync.WaitGroup
	var allowed int32
	var blocked int32
	
	// Run concurrent requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if cb.Allow() {
				atomic.AddInt32(&allowed, 1)
				// Simulate some work
				time.Sleep(1 * time.Millisecond)
				if i%2 == 0 {
					cb.Success()
				} else {
					cb.Failure()
				}
			} else {
				atomic.AddInt32(&blocked, 1)
			}
		}()
	}
	
	wg.Wait()
	
	// All requests should have been processed
	total := atomic.LoadInt32(&allowed) + atomic.LoadInt32(&blocked)
	if total != 100 {
		t.Errorf("Expected 100 total requests, got %d", total)
	}
}

func TestCircuitBreakerStateCallback(t *testing.T) {
	var transitions []string
	var mu sync.Mutex
	
	config := Config{
		MaxFailures: 2,
		Timeout:     100 * time.Millisecond,
		OnStateChange: func(from, to State) {
			mu.Lock()
			transitions = append(transitions, from.String()+"->"+to.String())
			mu.Unlock()
		},
	}
	
	cb := New(config)
	
	// Trigger transitions
	cb.Failure()
	cb.Failure()
	
	// Wait for callback
	time.Sleep(10 * time.Millisecond)
	
	// Should have closed->open transition
	mu.Lock()
	if len(transitions) != 1 || transitions[0] != "closed->open" {
		t.Errorf("Expected closed->open transition, got %v", transitions)
	}
	mu.Unlock()
	
	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	cb.Allow() // This should trigger transition to half-open
	
	time.Sleep(10 * time.Millisecond)
	
	mu.Lock()
	if len(transitions) != 2 || transitions[1] != "open->half-open" {
		t.Errorf("Expected open->half-open transition, got %v", transitions)
	}
	mu.Unlock()
}

func TestCircuitBreakerAutoReset(t *testing.T) {
	config := Config{
		MaxFailures:      10, // High enough to not open the circuit
		FailureThreshold: 1.0, // Only use absolute threshold
		Interval:         100 * time.Millisecond,
	}
	
	cb := New(config)
	
	// Add more successes than failures to keep failure rate low
	// This ensures the circuit stays closed (failure rate = 3/9 = 33% < 100%)
	for i := 0; i < 6; i++ {
		cb.Success()
	}
	for i := 0; i < 3; i++ {
		cb.Failure()
	}
	
	stats := cb.Stats()
	if stats.Failures != 3 || stats.Successes != 6 {
		t.Errorf("Expected 3 failures and 6 successes, got %+v", stats)
	}
	
	// Verify circuit is still closed
	if stats.State != StateClosed {
		t.Errorf("Expected circuit to be closed, got %v", stats.State)
	}
	
	// Wait for auto-reset
	time.Sleep(150 * time.Millisecond)
	
	stats = cb.Stats()
	if stats.Failures != 0 || stats.Successes != 0 {
		t.Errorf("Expected counters to be reset, got %+v", stats)
	}
}

func BenchmarkCircuitBreaker(b *testing.B) {
	config := Config{
		MaxFailures: 100,
		Timeout:     1 * time.Second,
	}
	
	cb := New(config)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if cb.Allow() {
				cb.Success()
			}
		}
	})
}