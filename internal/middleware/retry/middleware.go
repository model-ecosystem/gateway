package retry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gateway/internal/core"
	"gateway/pkg/retry"
	gwerrors "gateway/pkg/errors"
)

// Config holds retry middleware configuration
type Config struct {
	// Default retry configuration for all routes
	Default retry.Config
	// Per-route retry configurations
	Routes map[string]retry.Config
	// Per-service retry configurations
	Services map[string]retry.Config
}

// Middleware implements retry logic for backend requests
type Middleware struct {
	config       Config
	retriers     map[string]*retry.Retrier
	retryBudget  *GlobalBudget
	logger       *slog.Logger
}

// New creates a new retry middleware
func New(config Config, logger *slog.Logger) *Middleware {
	// Create retriers for all configurations
	retriers := make(map[string]*retry.Retrier)

	// Create default retrier
	retriers["default"] = retry.New(config.Default)

	// Create route-specific retriers
	for route, cfg := range config.Routes {
		retriers["route:"+route] = retry.New(cfg)
	}

	// Create service-specific retriers
	for service, cfg := range config.Services {
		retriers["service:"+service] = retry.New(cfg)
	}

	// Create a global retry budget (10% of requests can retry by default)
	// This prevents retry storms when backends are failing
	retryBudget := NewGlobalBudget(0.1, 100, time.Minute)

	return &Middleware{
		config:      config,
		retriers:    retriers,
		retryBudget: retryBudget,
		logger:      logger.With("component", "retry"),
	}
}

// NewWithBudget creates a new retry middleware with custom budget
func NewWithBudget(config Config, budgetRatio float64, logger *slog.Logger) *Middleware {
	m := New(config, logger)
	m.retryBudget = NewGlobalBudget(budgetRatio, 100, time.Minute)
	return m
}

// Apply returns a middleware function that applies retry logic
func (m *Middleware) Apply() core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Record the request for budget tracking
			m.retryBudget.RecordRequest()
			
			// Get retrier based on route or service
			retrier := m.getRetrier(ctx)

			var resp core.Response
			var lastErr error
			startTime := time.Now()
			attemptCount := 0

			// Use retrier to execute the request
			err := retrier.Do(ctx, func(ctx context.Context) error {
				attemptCount++
				
				// Check retry budget before retrying (first attempt always allowed)
				if attemptCount > 1 {
					if !m.retryBudget.CanRetry() {
						m.logger.Debug("retry budget exhausted",
							"path", req.Path(),
							"budget_stats", m.retryBudget.Stats(),
						)
						return retry.NewNonRetryableError(lastErr)
					}
					// Record the retry
					m.retryBudget.RecordRetry()
				}
				
				var err error
				resp, err = next(ctx, req)

				if err != nil {
					// Check if error is retryable
					if !m.isRetryableError(err) {
						return retry.NewNonRetryableError(err)
					}
					lastErr = err
					return err
				}

				// Check response status for retryable conditions
				if resp != nil && m.isRetryableStatus(resp.StatusCode()) {
					lastErr = &gwerrors.Error{
						Type:    gwerrors.ErrorTypeInternal,
						Message: "Retryable HTTP status",
						Details: map[string]interface{}{
							"status": resp.StatusCode(),
						},
					}
					return lastErr
				}

				return nil
			})

			// Handle retry exhaustion
			if err != nil {
				var retryErr *retry.Error
				if errors.As(err, &retryErr) {
					m.logger.Warn("retry exhausted",
						"path", req.Path(),
						"attempts", retryErr.Attempts,
						"duration", time.Since(startTime),
						"error", retryErr.Err,
					)
				}

				// Return the last error
				if lastErr != nil {
					return nil, lastErr
				}
				return nil, err
			}

			// Log successful retry if it took more than one attempt
			if retryErr, ok := err.(*retry.Error); ok && retryErr.Attempts > 1 {
				m.logger.Info("request succeeded after retry",
					"path", req.Path(),
					"attempts", retryErr.Attempts,
					"duration", time.Since(startTime),
				)
			}

			return resp, nil
		}
	}
}

// routeContextKey is the key for storing route info in context
type routeContextKey struct{}

// getRetrier returns the appropriate retrier for the request
func (m *Middleware) getRetrier(ctx context.Context) *retry.Retrier {
	// Try to get route result from context (set by route-aware middleware)
	if route, ok := ctx.Value(routeContextKey{}).(*core.RouteResult); ok && route != nil {
		// Try route-specific retrier
		if route.Rule != nil && route.Rule.ID != "" {
			if retrier, exists := m.retriers["route:"+route.Rule.ID]; exists {
				return retrier
			}
		}
		
		// Try service-specific retrier
		if route.Rule != nil && route.Rule.ServiceName != "" {
			if retrier, exists := m.retriers["service:"+route.Rule.ServiceName]; exists {
				return retrier
			}
		}
	}

	// Return default retrier
	return m.retriers["default"]
}

// isRetryableError determines if an error should trigger a retry
func (m *Middleware) isRetryableError(err error) bool {
	// Don't retry nil errors
	if err == nil {
		return false
	}
	
	// Don't retry client errors
	var gwErr *gwerrors.Error
	if errors.As(err, &gwErr) {
		switch gwErr.Type {
		case gwerrors.ErrorTypeBadRequest,
			gwerrors.ErrorTypeUnauthorized,
			gwerrors.ErrorTypeForbidden,
			gwerrors.ErrorTypeNotFound,
			gwerrors.ErrorTypeRateLimit:
			return false
		}
	}

	// Retry timeouts, service unavailable, and internal errors
	return true
}

// isRetryableStatus determines if an HTTP status code should trigger a retry
func (m *Middleware) isRetryableStatus(status int) bool {
	// Retry on 5xx errors (except 501 Not Implemented)
	if status >= 500 && status != 501 {
		return true
	}

	// Retry on specific 4xx errors
	switch status {
	case 408, // Request Timeout
		429, // Too Many Requests
		503: // Service Unavailable
		return true
	}

	return false
}

// NonRetryableError wraps an error to prevent retry
type NonRetryableError struct {
	err error
}

// Error implements the error interface
func (e *NonRetryableError) Error() string {
	return e.err.Error()
}

// Unwrap returns the underlying error
func (e *NonRetryableError) Unwrap() error {
	return e.err
}

// GetBudgetStats returns current retry budget statistics
func (m *Middleware) GetBudgetStats() BudgetStats {
	return m.retryBudget.Stats()
}
