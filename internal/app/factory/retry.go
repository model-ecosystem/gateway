package factory

import (
	"log/slog"
	"time"

	"gateway/internal/config"
	retryMiddleware "gateway/internal/middleware/retry"
	"gateway/internal/retry"
)

// CreateRetryMiddleware creates retry middleware from config
func CreateRetryMiddleware(cfg *config.Retry, logger *slog.Logger) *retryMiddleware.Middleware {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	// Convert config to middleware config
	middlewareConfig := retryMiddleware.Config{
		Default:  convertRetryConfig(cfg.Default),
		Routes:   make(map[string]retry.Config),
		Services: make(map[string]retry.Config),
	}

	// Convert route-specific configs
	for route, routeCfg := range cfg.Routes {
		middlewareConfig.Routes[route] = convertRetryConfig(routeCfg)
	}

	// Convert service-specific configs
	for service, serviceCfg := range cfg.Services {
		middlewareConfig.Services[service] = convertRetryConfig(serviceCfg)
	}

	return retryMiddleware.New(middlewareConfig, logger)
}

// convertRetryConfig converts from config to internal retry config
func convertRetryConfig(cfg config.RetryConfig) retry.Config {
	// Set defaults if not specified
	if cfg.MaxAttempts < 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 100
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30000
	}
	if cfg.Multiplier <= 1 {
		cfg.Multiplier = 2.0
	}

	return retry.Config{
		MaxAttempts:   cfg.MaxAttempts,
		InitialDelay:  time.Duration(cfg.InitialDelay) * time.Millisecond,
		MaxDelay:      time.Duration(cfg.MaxDelay) * time.Millisecond,
		Multiplier:    cfg.Multiplier,
		Jitter:        cfg.Jitter,
		RetryableFunc: retry.DefaultRetryableFunc,
	}
}
