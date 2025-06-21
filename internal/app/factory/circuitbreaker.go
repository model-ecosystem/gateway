package factory

import (
	"log/slog"
	"time"
	
	"gateway/internal/circuitbreaker"
	"gateway/internal/config"
	cbMiddleware "gateway/internal/middleware/circuitbreaker"
)

// CreateCircuitBreakerMiddleware creates circuit breaker middleware from config
func CreateCircuitBreakerMiddleware(cfg *config.CircuitBreaker, logger *slog.Logger) *cbMiddleware.Middleware {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	
	// Convert config to middleware config
	middlewareConfig := cbMiddleware.Config{
		Default:  convertCircuitBreakerConfig(cfg.Default),
		Routes:   make(map[string]circuitbreaker.Config),
		Services: make(map[string]circuitbreaker.Config),
	}
	
	// Convert route-specific configs
	for route, routeCfg := range cfg.Routes {
		middlewareConfig.Routes[route] = convertCircuitBreakerConfig(routeCfg)
	}
	
	// Convert service-specific configs
	for service, serviceCfg := range cfg.Services {
		middlewareConfig.Services[service] = convertCircuitBreakerConfig(serviceCfg)
	}
	
	return cbMiddleware.New(middlewareConfig, logger)
}

// convertCircuitBreakerConfig converts from config to internal circuit breaker config
func convertCircuitBreakerConfig(cfg config.CircuitBreakerConfig) circuitbreaker.Config {
	// Set defaults if not specified
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.FailureThreshold <= 0 || cfg.FailureThreshold > 1 {
		cfg.FailureThreshold = 0.5
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60
	}
	if cfg.MaxRequests <= 0 {
		cfg.MaxRequests = 1
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 60
	}
	
	return circuitbreaker.Config{
		MaxFailures:      cfg.MaxFailures,
		FailureThreshold: cfg.FailureThreshold,
		Timeout:          time.Duration(cfg.Timeout) * time.Second,
		MaxRequests:      cfg.MaxRequests,
		Interval:         time.Duration(cfg.Interval) * time.Second,
	}
}