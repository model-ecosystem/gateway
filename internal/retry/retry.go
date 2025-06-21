package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Config holds retry configuration
type Config struct {
	// MaxAttempts is the maximum number of retry attempts (0 = no retry)
	MaxAttempts int
	// InitialDelay is the initial delay between retries
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the backoff multiplier (e.g., 2 for exponential backoff)
	Multiplier float64
	// Jitter adds randomness to delays to avoid thundering herd
	Jitter bool
	// RetryableFunc determines if an error is retryable
	RetryableFunc func(error) bool
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		Multiplier:    2.0,
		Jitter:        true,
		RetryableFunc: DefaultRetryableFunc,
	}
}

// DefaultRetryableFunc is the default function to determine if an error is retryable
func DefaultRetryableFunc(err error) bool {
	// Don't retry context errors
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	
	// Don't retry non-retryable errors
	var nonRetryable *NonRetryableError
	if errors.As(err, &nonRetryable) {
		return false
	}
	
	// Retry all other errors by default
	return true
}

// Retrier provides retry functionality with exponential backoff
type Retrier struct {
	config Config
}

// New creates a new retrier with the given configuration
func New(config Config) *Retrier {
	// Validate and set defaults
	if config.MaxAttempts < 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.Multiplier <= 1 {
		config.Multiplier = 2.0
	}
	if config.RetryableFunc == nil {
		config.RetryableFunc = DefaultRetryableFunc
	}
	
	return &Retrier{
		config: config,
	}
}

// Do executes the given function with retry logic
func (r *Retrier) Do(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	
	for attempt := 0; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context before attempt
		if err := ctx.Err(); err != nil {
			return err
		}
		
		// Execute the function
		err := fn(ctx)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if we should retry
		if attempt >= r.config.MaxAttempts {
			break
		}
		
		if !r.config.RetryableFunc(err) {
			return err
		}
		
		// Calculate delay
		delay := r.calculateDelay(attempt)
		
		// Wait for the delay or context cancellation
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	return &Error{
		Err:      lastErr,
		Attempts: r.config.MaxAttempts + 1,
	}
}

// DoWithData executes the given function with retry logic and returns data
func (r *Retrier) DoWithData(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	var lastErr error
	
	for attempt := 0; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context before attempt
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		
		// Execute the function
		data, err := fn(ctx)
		if err == nil {
			return data, nil
		}
		
		lastErr = err
		
		// Check if we should retry
		if attempt >= r.config.MaxAttempts {
			break
		}
		
		if !r.config.RetryableFunc(err) {
			return nil, err
		}
		
		// Calculate delay
		delay := r.calculateDelay(attempt)
		
		// Wait for the delay or context cancellation
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	return nil, &Error{
		Err:      lastErr,
		Attempts: r.config.MaxAttempts + 1,
	}
}

// calculateDelay calculates the delay for the given attempt
func (r *Retrier) calculateDelay(attempt int) time.Duration {
	// Calculate base delay using exponential backoff
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt))
	
	// Cap at max delay
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}
	
	// Add jitter if enabled
	if r.config.Jitter {
		// Add ±25% jitter
		jitter := delay * 0.25
		delay = delay + (rand.Float64()*2-1)*jitter
	}
	
	return time.Duration(delay)
}

// Error represents a retry error with additional information
type Error struct {
	Err      error
	Attempts int
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Err
}

// Is checks if the target error matches
func (e *Error) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// As attempts to convert to the target type
func (e *Error) As(target interface{}) bool {
	return errors.As(e.Err, target)
}

// NonRetryableError wraps an error to indicate it should not be retried
type NonRetryableError struct {
	err error
}

// NewNonRetryableError creates a new non-retryable error
func NewNonRetryableError(err error) error {
	return &NonRetryableError{err: err}
}

// Error implements the error interface
func (e *NonRetryableError) Error() string {
	return e.err.Error()
}

// Unwrap returns the underlying error
func (e *NonRetryableError) Unwrap() error {
	return e.err
}