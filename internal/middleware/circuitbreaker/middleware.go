package circuitbreaker

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"gateway/pkg/circuitbreaker"
	"gateway/internal/core"
	gwerrors "gateway/pkg/errors"
)

// Config holds circuit breaker middleware configuration
type Config struct {
	// Default circuit breaker config for all routes
	Default circuitbreaker.Config
	// Per-route circuit breaker configs
	Routes map[string]circuitbreaker.Config
	// Per-service circuit breaker configs
	Services map[string]circuitbreaker.Config
}

// Middleware implements circuit breaker pattern for backend requests
type Middleware struct {
	config   Config
	breakers sync.Map // map[string]*circuitbreaker.CircuitBreaker
	logger   *slog.Logger
}

// New creates a new circuit breaker middleware
func New(config Config, logger *slog.Logger) *Middleware {
	return &Middleware{
		config: config,
		logger: logger.With("component", "circuitbreaker"),
	}
}

// Apply returns a middleware function that applies circuit breaking
func (m *Middleware) Apply() core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Get circuit breaker key based on route or service
			key := m.getCircuitBreakerKey(ctx, req)

			// Get or create circuit breaker for this key
			cb := m.getOrCreateBreaker(key)

			// Check if request is allowed
			if !cb.Allow() {
				m.logger.Warn("circuit breaker open",
					"key", key,
					"state", cb.State().String(),
				)

				return nil, &gwerrors.Error{
					Type:    gwerrors.ErrorTypeUnavailable,
					Message: "Service temporarily unavailable",
					Details: map[string]interface{}{
						"circuit_breaker": "open",
						"key":             key,
					},
				}
			}

			// Execute the request
			resp, err := next(ctx, req)

			// Record result
			if err != nil {
				// Check if error is retryable
				if m.isRetryableError(err) {
					cb.Failure()
					m.logger.Debug("request failed",
						"key", key,
						"error", err,
						"stats", cb.Stats(),
					)
				}
			} else {
				cb.Success()
			}

			return resp, err
		}
	}
}

// routeContextKey is the key for storing route info in context
type routeContextKey struct{}

// getCircuitBreakerKey determines the circuit breaker key for a request
func (m *Middleware) getCircuitBreakerKey(ctx context.Context, req core.Request) string {
	// Try to get route result from context (set by route-aware middleware)
	if route, ok := ctx.Value(routeContextKey{}).(*core.RouteResult); ok && route != nil {
		// Prefer route ID if available
		if route.Rule != nil && route.Rule.ID != "" {
			return "route:" + route.Rule.ID
		}
		// Fall back to service name
		if route.Rule != nil && route.Rule.ServiceName != "" {
			return "service:" + route.Rule.ServiceName
		}
	}

	// Default to path-based key
	return "path:" + req.Path()
}

// getOrCreateBreaker gets or creates a circuit breaker for the given key
func (m *Middleware) getOrCreateBreaker(key string) *circuitbreaker.CircuitBreaker {
	// Try to get existing breaker
	if breaker, ok := m.breakers.Load(key); ok {
		return breaker.(*circuitbreaker.CircuitBreaker)
	}

	// Create new breaker with appropriate config
	config := m.getConfig(key)

	// Add state change logging
	originalOnChange := config.OnStateChange
	config.OnStateChange = func(from, to circuitbreaker.State) {
		m.logger.Info("circuit breaker state changed",
			"key", key,
			"from", from.String(),
			"to", to.String(),
		)
		if originalOnChange != nil {
			originalOnChange(from, to)
		}
	}

	breaker := circuitbreaker.New(config)

	// Store and return
	actual, _ := m.breakers.LoadOrStore(key, breaker)
	return actual.(*circuitbreaker.CircuitBreaker)
}

// getConfig returns the configuration for a given key
func (m *Middleware) getConfig(key string) circuitbreaker.Config {
	// Check for specific route config
	if len(key) > 6 && key[:6] == "route:" {
		routeID := key[6:]
		if config, ok := m.config.Routes[routeID]; ok {
			return config
		}
	}

	// Check for specific service config
	if len(key) > 8 && key[:8] == "service:" {
		serviceName := key[8:]
		if config, ok := m.config.Services[serviceName]; ok {
			return config
		}
	}

	// Return default config
	return m.config.Default
}

// isRetryableError determines if an error should count as a circuit breaker failure
func (m *Middleware) isRetryableError(err error) bool {
	// Don't count client errors as failures
	var gwErr *gwerrors.Error
	if errors.As(err, &gwErr) {
		switch gwErr.Type {
		case gwerrors.ErrorTypeBadRequest,
			gwerrors.ErrorTypeUnauthorized,
			gwerrors.ErrorTypeForbidden,
			gwerrors.ErrorTypeNotFound:
			return false
		}
	}

	// Count timeouts, service unavailable, and internal errors as failures
	return true
}

// GetBreaker returns the circuit breaker for a specific key (for testing/monitoring)
func (m *Middleware) GetBreaker(key string) *circuitbreaker.CircuitBreaker {
	if breaker, ok := m.breakers.Load(key); ok {
		return breaker.(*circuitbreaker.CircuitBreaker)
	}
	return nil
}

// ResetAll resets all circuit breakers
func (m *Middleware) ResetAll() {
	m.breakers.Range(func(key, value interface{}) bool {
		if breaker, ok := value.(*circuitbreaker.CircuitBreaker); ok {
			breaker.Reset()
		}
		return true
	})
}
