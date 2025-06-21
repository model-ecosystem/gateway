package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State represents the state of the circuit breaker
type State int

const (
	// StateClosed allows requests to pass through
	StateClosed State = iota
	// StateOpen blocks all requests
	StateOpen
	// StateHalfOpen allows limited requests to test if service recovered
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config holds circuit breaker configuration
type Config struct {
	// MaxFailures is the number of failures before opening the circuit
	MaxFailures int
	// FailureThreshold is the percentage of failures to trigger open state (0-1)
	FailureThreshold float64
	// Timeout is the duration of the open state before trying half-open
	Timeout time.Duration
	// MaxRequests is the number of requests allowed in half-open state
	MaxRequests int
	// Interval is the cyclic period to clear counts
	Interval time.Duration
	// OnStateChange is called when the state changes
	OnStateChange func(from, to State)
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		MaxFailures:      5,
		FailureThreshold: 0.5,
		Timeout:          60 * time.Second,
		MaxRequests:      1,
		Interval:         60 * time.Second,
		OnStateChange:    nil,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config Config
	
	mu              sync.RWMutex
	state           State
	failures        int
	successes       int
	requests        int
	lastFailureTime time.Time
	lastStateChange time.Time
	halfOpenSuccess int
	generation      uint64
}

// New creates a new circuit breaker
func New(config Config) *CircuitBreaker {
	// Validate config
	if config.MaxFailures <= 0 {
		config.MaxFailures = 5
	}
	if config.FailureThreshold <= 0 || config.FailureThreshold > 1 {
		config.FailureThreshold = 0.5
	}
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}
	if config.MaxRequests <= 0 {
		config.MaxRequests = 1
	}
	if config.Interval <= 0 {
		config.Interval = 60 * time.Second
	}
	
	cb := &CircuitBreaker{
		config:          config,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
	
	// Start the reset timer
	go cb.resetTimer()
	
	return cb
}

// State returns the current state
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Allow checks if a request is allowed to proceed
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	// Update state based on timeout
	cb.updateState()
	
	switch cb.state {
	case StateClosed:
		// Always allow in closed state
		return true
		
	case StateOpen:
		// Block all requests in open state
		return false
		
	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.requests < cb.config.MaxRequests {
			cb.requests++
			return true
		}
		return false
		
	default:
		return false
	}
}

// Success records a successful request
func (cb *CircuitBreaker) Success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.successes++
	
	switch cb.state {
	case StateHalfOpen:
		cb.halfOpenSuccess++
		// If we've had enough successful requests, close the circuit
		if cb.halfOpenSuccess >= cb.config.MaxRequests {
			cb.changeState(StateClosed)
		}
	}
}

// Failure records a failed request
func (cb *CircuitBreaker) Failure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failures++
	cb.lastFailureTime = time.Now()
	
	switch cb.state {
	case StateClosed:
		// Check if we should open the circuit
		if cb.shouldOpen() {
			cb.changeState(StateOpen)
		}
		
	case StateHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.changeState(StateOpen)
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(ctx context.Context, fn func(context.Context) error) error {
	if !cb.Allow() {
		return ErrCircuitOpen
	}
	
	err := fn(ctx)
	if err != nil {
		cb.Failure()
		return err
	}
	
	cb.Success()
	return nil
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failures = 0
	cb.successes = 0
	cb.requests = 0
	cb.halfOpenSuccess = 0
	cb.generation++
	
	if cb.state != StateClosed {
		cb.changeState(StateClosed)
	}
}

// Stats returns current statistics
func (cb *CircuitBreaker) Stats() Stats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return Stats{
		State:           cb.state,
		Failures:        cb.failures,
		Successes:       cb.successes,
		Requests:        cb.requests,
		LastFailureTime: cb.lastFailureTime,
		LastStateChange: cb.lastStateChange,
		Generation:      cb.generation,
	}
}

// updateState checks if the state should be updated based on timeouts
func (cb *CircuitBreaker) updateState() {
	if cb.state == StateOpen {
		if time.Since(cb.lastStateChange) > cb.config.Timeout {
			cb.changeState(StateHalfOpen)
		}
	}
}

// shouldOpen checks if the circuit should open based on failure criteria
func (cb *CircuitBreaker) shouldOpen() bool {
	total := cb.failures + cb.successes
	if total == 0 {
		return false
	}
	
	// Check absolute threshold
	if cb.failures >= cb.config.MaxFailures {
		return true
	}
	
	// Check percentage threshold
	failureRate := float64(cb.failures) / float64(total)
	return failureRate >= cb.config.FailureThreshold
}

// changeState changes the circuit breaker state
func (cb *CircuitBreaker) changeState(newState State) {
	if cb.state == newState {
		return
	}
	
	from := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()
	
	// Reset counters when entering closed or half-open state
	switch newState {
	case StateClosed:
		cb.failures = 0
		cb.successes = 0
		cb.requests = 0
		cb.halfOpenSuccess = 0
		cb.generation++
		
	case StateHalfOpen:
		cb.requests = 0
		cb.halfOpenSuccess = 0
	}
	
	// Call state change callback if configured
	if cb.config.OnStateChange != nil {
		// Call in goroutine to avoid blocking
		go cb.config.OnStateChange(from, newState)
	}
}

// resetTimer periodically resets counters in closed state
func (cb *CircuitBreaker) resetTimer() {
	ticker := time.NewTicker(cb.config.Interval)
	defer ticker.Stop()
	
	for range ticker.C {
		cb.mu.Lock()
		if cb.state == StateClosed {
			cb.failures = 0
			cb.successes = 0
			cb.generation++
		}
		cb.mu.Unlock()
	}
}

// Stats holds circuit breaker statistics
type Stats struct {
	State           State
	Failures        int
	Successes       int
	Requests        int
	LastFailureTime time.Time
	LastStateChange time.Time
	Generation      uint64
}

// Errors
var (
	// ErrCircuitOpen is returned when the circuit is open
	ErrCircuitOpen = errors.New("circuit breaker is open")
)