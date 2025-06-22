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

	// Create gRPC connector
	grpcConnector := factory.CreateGRPCConnector(b.logger)

	// Create base handler with multi-protocol support
	baseHandler := factory.CreateMultiProtocolHandler(router, httpConnector, grpcConnector)

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

	// Apply base middleware (recovery, logging, auth)
	baseHandler = factory.ApplyMiddleware(baseHandler, b.logger, authMiddleware)

	// Add rate limiting middleware after basic middleware but before business logic
	if rateLimitMiddleware := factory.CreateRateLimitMiddleware(&b.config.Gateway.Router, &b.config.Gateway, b.logger); rateLimitMiddleware != nil {
		baseHandler = rateLimitMiddleware(baseHandler)
		b.logger.Info("Rate limiting enabled for configured routes")
	}

	// Create HTTP adapter
	httpAdapterInstance, err := factory.CreateHTTPAdapter(b.config.Gateway.Frontend.HTTP, baseHandler, b.logger)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP adapter: %w", err)
	}

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
		
		// Set both health handler and health config
		healthCfg := httpAdapter.HealthConfig{
			Enabled:       true,
			HealthPath:    cfg.HealthPath,
			ReadyPath:     cfg.ReadyPath,
			LivePath:      cfg.LivePath,
			HealthHandler: healthHandler,
		}
		httpAdapterInstance.WithHealthHandler(healthHandler).WithHealthConfig(healthCfg)

		b.logger.Info("Health checks enabled",
			"health", cfg.HealthPath,
			"ready", cfg.ReadyPath,
			"live", cfg.LivePath,
		)
	}

	// Add metrics endpoint if enabled
	if factory.ShouldEnableMetrics(b.config.Gateway.Metrics) {
		metricsHandler := factory.CreateMetricsHandler()
		httpAdapterInstance.WithMetricsHandler(metricsHandler)
		if b.config.Gateway.Metrics.Path != "" {
			httpAdapterInstance.WithMetricsPath(b.config.Gateway.Metrics.Path)
		}
	}

	// Add CORS support if enabled
	if corsHandler := factory.CreateCORSHandler(b.config.Gateway.CORS, httpAdapterInstance); corsHandler != nil {
		httpAdapterInstance.WithCORSHandler(corsHandler)
		b.logger.Info("CORS enabled")
	}

	// Add SSE support if enabled
	if cfg := b.config.Gateway.Frontend.SSE; cfg != nil && cfg.Enabled {
		b.addSSESupport(httpAdapterInstance, router, httpClient, authMiddleware, gatewayMetrics)
	}

	// Create WebSocket adapter if enabled
	var wsAdapter *wsAdapter.Adapter
	if cfg := b.config.Gateway.Frontend.WebSocket; cfg != nil && cfg.Enabled {
		wsAdapter = b.createWebSocketAdapter(router, authMiddleware, gatewayMetrics)
	}

	return &Server{
		config:      b.config,
		httpAdapter: httpAdapterInstance,
		wsAdapter:   wsAdapter,
		logger:      b.logger,
	}, nil
}

// addSSESupport adds SSE capabilities to the HTTP adapter
func (b *Builder) addSSESupport(
	httpAdapterInstance *httpAdapter.Adapter,
	router core.Router,
	httpClient *http.Client,
	authMiddleware *auth.Middleware,
	metrics *metrics.Metrics,
) {
	sseConnector := factory.CreateSSEConnector(b.config.Gateway.Backend.SSE, httpClient, b.logger)
	sseHandler := factory.CreateSSEHandler(router, sseConnector, b.logger)

	// Apply rate limiting if configured
	if rateLimitMiddleware := factory.CreateRateLimitMiddleware(&b.config.Gateway.Router, &b.config.Gateway, b.logger); rateLimitMiddleware != nil {
		sseHandler = rateLimitMiddleware(sseHandler)
	}

	sseHandler = factory.ApplyMiddleware(sseHandler, b.logger, authMiddleware)
	factory.CreateSSEAdapter(b.config.Gateway.Frontend.SSE, sseHandler, httpAdapterInstance, b.config.Gateway.Auth, b.logger, metrics)
}

// createWebSocketAdapter creates the WebSocket adapter
func (b *Builder) createWebSocketAdapter(
	router core.Router,
	authMiddleware *auth.Middleware,
	metrics *metrics.Metrics,
) *wsAdapter.Adapter {
	wsConnector := factory.CreateWebSocketConnector(b.config.Gateway.Backend.WebSocket, b.logger)
	wsHandler := factory.CreateWebSocketHandler(router, wsConnector, b.logger)

	// Apply rate limiting if configured
	if rateLimitMiddleware := factory.CreateRateLimitMiddleware(&b.config.Gateway.Router, &b.config.Gateway, b.logger); rateLimitMiddleware != nil {
		wsHandler = rateLimitMiddleware(wsHandler)
	}

	wsHandler = factory.ApplyMiddleware(wsHandler, b.logger, authMiddleware)
	return factory.CreateWebSocketAdapter(b.config.Gateway.Frontend.WebSocket, wsHandler, b.config.Gateway.Auth, b.logger, metrics)
}
