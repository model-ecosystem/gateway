package retry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gateway/internal/core"
	"gateway/internal/retry"
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
	config   Config
	retriers map[string]*retry.Retrier
	logger   *slog.Logger
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

	return &Middleware{
		config:   config,
		retriers: retriers,
		logger:   logger.With("component", "retry"),
	}
}

// Apply returns a middleware function that applies retry logic
func (m *Middleware) Apply() core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Get retrier based on route or service
			retrier := m.getRetrier(ctx)

			var resp core.Response
			var lastErr error
			startTime := time.Now()

			// Use retrier to execute the request
			err := retrier.Do(ctx, func(ctx context.Context) error {
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

// getRetrier returns the appropriate retrier for the request
func (m *Middleware) getRetrier(ctx context.Context) *retry.Retrier {
	// Try route-specific retrier
	if routeID, ok := ctx.Value("route_id").(string); ok {
		if retrier, exists := m.retriers["route:"+routeID]; exists {
			return retrier
		}
	}

	// Try service-specific retrier
	if serviceName, ok := ctx.Value("service_name").(string); ok {
		if retrier, exists := m.retriers["service:"+serviceName]; exists {
			return retrier
		}
	}

	// Return default retrier
	return m.retriers["default"]
}

// isRetryableError determines if an error should trigger a retry
func (m *Middleware) isRetryableError(err error) bool {
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
