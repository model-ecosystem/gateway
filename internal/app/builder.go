package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	httpAdapter "gateway/internal/adapter/http"
	wsAdapter "gateway/internal/adapter/websocket"
	"gateway/internal/app/factory"
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/metrics"
	"gateway/internal/middleware/auth"
)

// Builder builds the gateway application
type Builder struct {
	config *config.Config
	logger *slog.Logger
}

// NewBuilder creates a new application builder
func NewBuilder(cfg *config.Config, logger *slog.Logger) *Builder {
	return &Builder{
		config: cfg,
		logger: logger,
	}
}

// Build constructs the gateway server
func (b *Builder) Build() (*Server, error) {
	// Create service registry
	registry, err := factory.CreateRegistry(&b.config.Gateway.Registry, b.logger)
	if err != nil {
		return nil, fmt.Errorf("creating registry: %w", err)
	}

	// Create router
	router, err := factory.CreateRouterFromConfig(registry, &b.config.Gateway.Router)
	if err != nil {
		return nil, fmt.Errorf("creating router: %w", err)
	}

	// Create auth middleware if configured
	var authMiddleware *auth.Middleware
	if b.config.Gateway.Auth != nil {
		authMiddleware, err = factory.CreateAuthMiddleware(b.config.Gateway.Auth, b.logger)
		if err != nil {
			return nil, fmt.Errorf("creating auth middleware: %w", err)
		}
	}

	// Create HTTP client and connector
	httpClient := factory.CreateHTTPClient(b.config.Gateway.Backend.HTTP)
	httpConnector := factory.CreateHTTPConnector(httpClient, b.config.Gateway.Backend.HTTP)

	// Create base handler with middleware
	baseHandler := factory.CreateBaseHandler(router, httpConnector)
	
	// Add metrics middleware if enabled
	var gatewayMetrics *metrics.Metrics
	if factory.ShouldEnableMetrics(b.config.Gateway.Metrics) {
		gatewayMetrics = factory.CreateMetrics()
		metricsMiddleware := factory.CreateMetricsMiddleware(gatewayMetrics)
		baseHandler = metricsMiddleware(baseHandler)
		b.logger.Info("Metrics enabled", "path", b.config.Gateway.Metrics.Path)
	}
	
	// Add circuit breaker middleware if enabled
	if cbMiddleware := factory.CreateCircuitBreakerMiddleware(b.config.Gateway.CircuitBreaker, b.logger); cbMiddleware != nil {
		baseHandler = cbMiddleware.Apply()(baseHandler)
		b.logger.Info("Circuit breaker enabled")
	}
	
	// Add retry middleware if enabled
	if retryMiddleware := factory.CreateRetryMiddleware(b.config.Gateway.Retry, b.logger); retryMiddleware != nil {
		baseHandler = retryMiddleware.Apply()(baseHandler)
		b.logger.Info("Retry enabled")
	}
	
	baseHandler = factory.ApplyMiddleware(baseHandler, b.logger, authMiddleware)

	// Create HTTP adapter
	httpAdapter := factory.CreateHTTPAdapter(b.config.Gateway.Frontend.HTTP, baseHandler, b.logger)

	// Add health check support if enabled
	if cfg := b.config.Gateway.Health; cfg != nil && cfg.Enabled {
		healthChecker, err := factory.CreateHealthChecker(cfg, registry, b.logger)
		if err != nil {
			return nil, fmt.Errorf("creating health checker: %w", err)
		}
		
		// Use a simple service ID - could be enhanced to use hostname or config
		serviceID := fmt.Sprintf("gateway-%d", time.Now().Unix())
		version := "1.0.0" // Could be injected via build flags
		
		healthHandler := factory.CreateHealthHandler(cfg, healthChecker, version, serviceID)
		httpAdapter.WithHealthHandler(healthHandler)
		
		b.logger.Info("Health checks enabled",
			"health", cfg.HealthPath,
			"ready", cfg.ReadyPath,
			"live", cfg.LivePath,
		)
	}
	
	// Add metrics endpoint if enabled
	if factory.ShouldEnableMetrics(b.config.Gateway.Metrics) {
		metricsHandler := factory.CreateMetricsHandler()
		httpAdapter.WithMetricsHandler(metricsHandler)
	}
	
	// Add CORS support if enabled
	if corsHandler := factory.CreateCORSHandler(b.config.Gateway.CORS, httpAdapter); corsHandler != nil {
		httpAdapter.WithCORSHandler(corsHandler)
		b.logger.Info("CORS enabled")
	}

	// Add SSE support if enabled
	if cfg := b.config.Gateway.Frontend.SSE; cfg != nil && cfg.Enabled {
		b.addSSESupport(httpAdapter, router, httpClient, authMiddleware)
	}

	// Create WebSocket adapter if enabled
	var wsAdapter *wsAdapter.Adapter
	if cfg := b.config.Gateway.Frontend.WebSocket; cfg != nil {
		wsAdapter = b.createWebSocketAdapter(router, authMiddleware)
	}

	return &Server{
		config:      b.config,
		httpAdapter: httpAdapter,
		wsAdapter:   wsAdapter,
		logger:      b.logger,
	}, nil
}

// addSSESupport adds SSE capabilities to the HTTP adapter
func (b *Builder) addSSESupport(
	httpAdapter *httpAdapter.Adapter,
	router core.Router,
	httpClient *http.Client,
	authMiddleware *auth.Middleware,
) {
	sseConnector := factory.CreateSSEConnector(b.config.Gateway.Backend.SSE, httpClient, b.logger)
	sseHandler := factory.CreateSSEHandler(router, sseConnector, b.logger)
	sseHandler = factory.ApplyMiddleware(sseHandler, b.logger, authMiddleware)
	factory.CreateSSEAdapter(b.config.Gateway.Frontend.SSE, sseHandler, httpAdapter, b.logger)
}

// createWebSocketAdapter creates the WebSocket adapter
func (b *Builder) createWebSocketAdapter(
	router core.Router,
	authMiddleware *auth.Middleware,
) *wsAdapter.Adapter {
	wsConnector := factory.CreateWebSocketConnector(b.config.Gateway.Backend.WebSocket, b.logger)
	wsHandler := factory.CreateWebSocketHandler(router, wsConnector, b.logger)
	wsHandler = factory.ApplyMiddleware(wsHandler, b.logger, authMiddleware)
	return factory.CreateWebSocketAdapter(b.config.Gateway.Frontend.WebSocket, wsHandler, b.logger)
}