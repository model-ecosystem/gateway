package factory

import (
	"log/slog"
	"net/http"

	httpAdapter "gateway/internal/adapter/http"
	sseAdapter "gateway/internal/adapter/sse"
	wsAdapter "gateway/internal/adapter/websocket"
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/health"
	"gateway/internal/metrics"
	"gateway/internal/middleware/auth/jwt"
	"gateway/pkg/errors"
)

// AdapterFactory creates frontend adapter instances
type AdapterFactory struct {
	BaseComponentFactory
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory(logger *slog.Logger) *AdapterFactory {
	return &AdapterFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// CreateHTTPAdapter creates an HTTP frontend adapter
func (f *AdapterFactory) CreateHTTPAdapter(cfg config.HTTP, handler core.Handler) (*httpAdapter.Adapter, error) {
	httpAdapterComponent := httpAdapter.NewComponent(handler, f.logger)
	if err := httpAdapterComponent.Init(func(v interface{}) error {
		return f.ParseConfig(cfg, v)
	}); err != nil {
		return nil, err
	}
	
	httpAdapterComp := httpAdapterComponent.(*httpAdapter.Component)
	
	if err := httpAdapterComp.Validate(); err != nil {
		return nil, err
	}
	
	return httpAdapterComp.Build(), nil
}

// CreateSSEAdapter creates an SSE adapter that integrates with HTTP
func (f *AdapterFactory) CreateSSEAdapter(
	cfg *config.SSE,
	handler core.Handler,
	httpAdapterInstance *httpAdapter.Adapter,
	authConfig *config.Auth,
	metrics *metrics.Metrics,
	providerFactory *ProviderFactory,
) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	sseAdapterComponent := sseAdapter.NewComponent(handler, f.logger)
	if err := sseAdapterComponent.Init(func(v interface{}) error {
		return f.ParseConfig(*cfg, v)
	}); err != nil {
		return err
	}
	
	sseAdapterComp := sseAdapterComponent.(*sseAdapter.Component)
	sseAdapterInstance := sseAdapterComp.Build()

	// Add metrics if provided
	if metrics != nil {
		// TODO: Add SSE metrics
	}

	// Add JWT token validator if JWT auth is enabled
	if authConfig != nil && authConfig.JWT != nil && authConfig.JWT.Enabled && providerFactory != nil {
		jwtProvider, err := providerFactory.GetJWTProvider(authConfig.JWT)
		if err != nil {
			// Fail closed - return error to prevent insecure startup
			return errors.NewError(errors.ErrorTypeInternal, "failed to get JWT provider for SSE").WithCause(err)
		}
		// Create token validator
		tokenValidator := jwt.NewTokenValidator(jwtProvider, f.logger)
		sseAdapterInstance.WithTokenValidator(tokenValidator)
		f.logger.Info("JWT token validation enabled for SSE connections")
	}

	httpAdapterInstance.WithSSEHandler(sseAdapterInstance)
	return nil
}

// CreateWebSocketAdapter creates a WebSocket frontend adapter
func (f *AdapterFactory) CreateWebSocketAdapter(
	cfg *config.WebSocket,
	handler core.Handler,
	authConfig *config.Auth,
	metrics *metrics.Metrics,
	providerFactory *ProviderFactory,
) (*wsAdapter.Adapter, error) {
	if cfg == nil {
		return nil, nil
	}

	wsAdapterComponent := wsAdapter.NewComponent(handler, f.logger)
	if err := wsAdapterComponent.Init(func(v interface{}) error {
		return f.ParseConfig(*cfg, v)
	}); err != nil {
		return nil, err
	}
	
	wsAdapterComp := wsAdapterComponent.(*wsAdapter.Component)
	adapter := wsAdapterComp.Build()

	// Add metrics if provided
	if metrics != nil {
		// TODO: Add WebSocket metrics
	}

	// Add JWT token validator if JWT auth is enabled
	if authConfig != nil && authConfig.JWT != nil && authConfig.JWT.Enabled && providerFactory != nil {
		jwtProvider, err := providerFactory.GetJWTProvider(authConfig.JWT)
		if err != nil {
			// Fail closed - return error to prevent insecure startup
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to get JWT provider for WebSocket").WithCause(err)
		}
		// Create token validator
		tokenValidator := jwt.NewTokenValidator(jwtProvider, f.logger)
		adapter.WithTokenValidator(tokenValidator)
		f.logger.Info("JWT token validation enabled for WebSocket connections")
	}

	return adapter, nil
}

// CreateMetricsHandler creates a metrics handler
func (f *AdapterFactory) CreateMetricsHandler(metricsInstance *metrics.Metrics) http.HandlerFunc {
	// Return the Prometheus metrics handler
	return func(w http.ResponseWriter, r *http.Request) {
		metrics.Handler().ServeHTTP(w, r)
	}
}

// CreateHealthConfig creates health configuration for HTTP adapter
func (f *AdapterFactory) CreateHealthConfig(cfg *config.Health, healthHandler *health.Handler) httpAdapter.HealthConfig {
	if cfg == nil || !cfg.Enabled {
		return httpAdapter.HealthConfig{}
	}
	
	return httpAdapter.HealthConfig{
		Enabled:       true,
		HealthPath:    cfg.HealthPath,
		ReadyPath:     cfg.ReadyPath,
		LivePath:      cfg.LivePath,
		HealthHandler: healthHandler,
	}
}