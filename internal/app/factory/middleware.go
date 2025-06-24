package factory

import (
	"fmt"
	"log/slog"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/metrics"
	"gateway/internal/middleware/auth"
	"gateway/internal/middleware/auth/oauth2"
	"gateway/internal/middleware/authz/rbac"
	"gateway/internal/middleware/circuitbreaker"
	metricsMiddleware "gateway/internal/middleware/metrics"
	"gateway/internal/middleware/ratelimit"
	"gateway/internal/middleware/retry"
	"gateway/internal/middleware/tracking"
	"gateway/internal/telemetry"
	pkgCircuitbreaker "gateway/pkg/circuitbreaker"
	pkgRetry "gateway/pkg/retry"
)

// MiddlewareFactory creates middleware instances
type MiddlewareFactory struct {
	BaseComponentFactory
}

// NewMiddlewareFactory creates a new middleware factory
func NewMiddlewareFactory(logger *slog.Logger) *MiddlewareFactory {
	return &MiddlewareFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// CreateAuthMiddleware creates authentication middleware from config
func (f *MiddlewareFactory) CreateAuthMiddleware(cfg *config.Auth) (*auth.Middleware, error) {
	if cfg == nil || len(cfg.Providers) == 0 {
		return nil, nil
	}

	authComponent := auth.NewComponent(f.logger)
	if err := authComponent.Init(func(v interface{}) error {
		return f.ParseConfig(*cfg, v)
	}); err != nil {
		return nil, err
	}
	
	if authComp, ok := authComponent.(*auth.Component); ok {
		return authComp.GetMiddleware(), nil
	}
	
	return nil, fmt.Errorf("failed to create auth middleware")
}

// CreateOAuth2Middleware creates OAuth2/OIDC authentication middleware
func (f *MiddlewareFactory) CreateOAuth2Middleware(cfg *config.OAuth2Config) (*oauth2.Middleware, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	oauth2Component := oauth2.NewComponent(f.logger)
	if err := oauth2Component.Init(func(v interface{}) error {
		return f.ParseConfig(*cfg, v)
	}); err != nil {
		return nil, err
	}
	
	if oauth2Comp, ok := oauth2Component.(*oauth2.Component); ok {
		if oauth2Comp.IsEnabled() {
			return oauth2Comp.GetMiddleware(), nil
		}
	}
	
	return nil, nil
}

// CreateAuthzMiddlewares creates authorization middlewares from config
func (f *MiddlewareFactory) CreateAuthzMiddlewares(cfg *config.MiddlewareAuthz) ([]core.Middleware, error) {
	var middlewares []core.Middleware
	
	// Add RBAC middleware if configured
	if cfg.RBAC != nil && cfg.RBAC.Enabled {
		rbacComponent := rbac.NewComponent(f.logger)
		if err := rbacComponent.Init(func(v interface{}) error {
			return f.ParseConfig(*cfg.RBAC, v)
		}); err == nil {
			if rbacComp, ok := rbacComponent.(*rbac.Component); ok {
				if mw := rbacComp.Build(); mw != nil {
					middlewares = append(middlewares, mw)
				}
			}
		}
	}
	
	return middlewares, nil
}

// CreateCircuitBreakerMiddleware creates circuit breaker middleware from config
func (f *MiddlewareFactory) CreateCircuitBreakerMiddleware(cfg *config.CircuitBreaker) *circuitbreaker.Middleware {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	// Create circuit breaker config
	cbConfig := circuitbreaker.Config{
		Default: pkgCircuitbreaker.Config{
			MaxFailures:      cfg.Default.MaxFailures,
			FailureThreshold: cfg.Default.FailureThreshold,
			Timeout:          time.Duration(cfg.Default.Timeout) * time.Second,
			MaxRequests:      cfg.Default.MaxRequests,
			Interval:         time.Duration(cfg.Default.Interval) * time.Second,
		},
		Routes:   make(map[string]pkgCircuitbreaker.Config),
		Services: make(map[string]pkgCircuitbreaker.Config),
	}

	// Add per-route configurations if any
	for route, routeCfg := range cfg.Routes {
		cbConfig.Routes[route] = pkgCircuitbreaker.Config{
			MaxFailures:      routeCfg.MaxFailures,
			FailureThreshold: routeCfg.FailureThreshold,
			Timeout:          time.Duration(routeCfg.Timeout) * time.Second,
			MaxRequests:      routeCfg.MaxRequests,
			Interval:         time.Duration(routeCfg.Interval) * time.Second,
		}
	}

	// Add per-service configurations if any
	for service, serviceCfg := range cfg.Services {
		cbConfig.Services[service] = pkgCircuitbreaker.Config{
			MaxFailures:      serviceCfg.MaxFailures,
			FailureThreshold: serviceCfg.FailureThreshold,
			Timeout:          time.Duration(serviceCfg.Timeout) * time.Second,
			MaxRequests:      serviceCfg.MaxRequests,
			Interval:         time.Duration(serviceCfg.Interval) * time.Second,
		}
	}

	return circuitbreaker.New(cbConfig, f.logger)
}

// CreateRetryMiddleware creates retry middleware from config
func (f *MiddlewareFactory) CreateRetryMiddleware(cfg *config.Retry) *retry.Middleware {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	// Create retry config
	retryConfig := retry.Config{
		Default: pkgRetry.Config{
			MaxAttempts:  cfg.Default.MaxAttempts,
			InitialDelay: time.Duration(cfg.Default.InitialDelay) * time.Millisecond,
			MaxDelay:     time.Duration(cfg.Default.MaxDelay) * time.Millisecond,
			Multiplier:   cfg.Default.Multiplier,
			Jitter:       cfg.Default.Jitter,
		},
		Routes:   make(map[string]pkgRetry.Config),
		Services: make(map[string]pkgRetry.Config),
	}

	// Add per-route configurations if any
	for route, routeCfg := range cfg.Routes {
		retryConfig.Routes[route] = pkgRetry.Config{
			MaxAttempts:  routeCfg.MaxAttempts,
			InitialDelay: time.Duration(routeCfg.InitialDelay) * time.Millisecond,
			MaxDelay:     time.Duration(routeCfg.MaxDelay) * time.Millisecond,
			Multiplier:   routeCfg.Multiplier,
			Jitter:       routeCfg.Jitter,
		}
	}

	// Add per-service configurations if any
	for service, serviceCfg := range cfg.Services {
		retryConfig.Services[service] = pkgRetry.Config{
			MaxAttempts:  serviceCfg.MaxAttempts,
			InitialDelay: time.Duration(serviceCfg.InitialDelay) * time.Millisecond,
			MaxDelay:     time.Duration(serviceCfg.MaxDelay) * time.Millisecond,
			Multiplier:   serviceCfg.Multiplier,
			Jitter:       serviceCfg.Jitter,
		}
	}

	// Create middleware with custom budget ratio if specified
	if cfg.Default.BudgetRatio > 0 {
		return retry.NewWithBudget(retryConfig, cfg.Default.BudgetRatio, f.logger)
	}

	return retry.New(retryConfig, f.logger)
}

// CreateRateLimitMiddleware creates rate limiting middleware
func (f *MiddlewareFactory) CreateRateLimitMiddleware(routerCfg *config.Router, gatewayCfg *config.Gateway) core.Middleware {
	if gatewayCfg == nil || routerCfg == nil {
		return nil
	}

	// Check if any route has rate limiting enabled
	hasRateLimit := false
	for _, rule := range routerCfg.Rules {
		if rule.RateLimit > 0 {
			hasRateLimit = true
			break
		}
	}

	if !hasRateLimit {
		return nil
	}

	// Build per-route configurations
	routeConfigs := make(map[string]*ratelimit.Config)
	for _, rule := range routerCfg.Rules {
		if rule.RateLimit > 0 {
			routeConfigs[rule.Path] = &ratelimit.Config{
				Rate:  rule.RateLimit,
				Burst: rule.RateLimitBurst,
			}
		}
	}

	// Use per-route middleware
	return ratelimit.PerRoute(routeConfigs)
}

// CreateMetricsMiddleware creates metrics middleware
func (f *MiddlewareFactory) CreateMetricsMiddleware(metricsInstance *metrics.Metrics) core.Middleware {
	return metricsMiddleware.Middleware(metricsInstance)
}

// CreateTrackingMiddleware creates tracking middleware
func (f *MiddlewareFactory) CreateTrackingMiddleware() *tracking.Middleware {
	return tracking.NewMiddleware()
}

// CreateTelemetryMiddleware creates telemetry middleware
func (f *MiddlewareFactory) CreateTelemetryMiddleware(telemetryInstance *telemetry.Telemetry, metrics *telemetry.Metrics) *telemetry.Middleware {
	return telemetry.NewMiddleware(telemetryInstance, metrics)
}