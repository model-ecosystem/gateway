package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	httpAdapter "gateway/internal/adapter/http"
	wsAdapter "gateway/internal/adapter/websocket"
	"gateway/internal/app/factory"
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/health"
	"gateway/internal/management"
	"gateway/internal/metrics"
	"gateway/internal/middleware/auth"
	"gateway/internal/registry/static"
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
	// Create factories
	registryFactory := factory.NewRegistryFactory(b.logger)
	routerFactory := factory.NewRouterFactory(b.logger)
	handlerFactory := factory.NewHandlerFactory(b.logger)
	middlewareFactory := factory.NewMiddlewareFactory(b.logger)
	connectorFactory := factory.NewConnectorFactory(b.logger)
	adapterFactory := factory.NewAdapterFactory(b.logger)
	telemetryFactory := factory.NewTelemetryFactory(b.logger)
	healthFactory := factory.NewHealthFactory(b.logger)
	managementFactory := factory.NewManagementFactory(b.logger)
	providerFactory := factory.NewProviderFactory(b.logger)

	// Initialize telemetry if enabled
	gatewayTelemetry, telemetryMetrics, err := telemetryFactory.CreateTelemetry(b.config.Gateway.Telemetry)
	if err != nil {
		return nil, err
	}

	// Create service registry - use health-aware registry if health checks are enabled
	var registry core.ServiceRegistry
	var backendMonitor *health.BackendMonitor
	useHealthAware := b.config.Gateway.Health != nil && b.config.Gateway.Health.Enabled
	
	if useHealthAware {
		registry, backendMonitor, err = registryFactory.CreateHealthAwareRegistry(&b.config.Gateway.Registry, b.config.Gateway.Health)
		if err != nil {
			return nil, fmt.Errorf("creating health-aware registry: %w", err)
		}
	} else {
		registry, err = registryFactory.CreateRegistry(&b.config.Gateway.Registry)
		if err != nil {
			return nil, fmt.Errorf("creating registry: %w", err)
		}
	}

	// Create router
	gatewayRouter, err := routerFactory.CreateRouter(&b.config.Gateway.Router, registry)
	if err != nil {
		return nil, err
	}

	// Create auth middleware if configured
	var authMiddleware *auth.Middleware
	if b.config.Gateway.Auth != nil {
		authMiddleware, err = middlewareFactory.CreateAuthMiddleware(b.config.Gateway.Auth)
		if err != nil {
			return nil, fmt.Errorf("creating auth middleware: %w", err)
		}
	}
	
	// Create OAuth2 middleware if configured
	var oauth2Middleware core.Middleware
	if b.config.Gateway.Middleware != nil && b.config.Gateway.Middleware.Auth != nil && b.config.Gateway.Middleware.Auth.OAuth2 != nil {
		oauth2MW, err := middlewareFactory.CreateOAuth2Middleware(b.config.Gateway.Middleware.Auth.OAuth2)
		if err != nil {
			return nil, fmt.Errorf("creating OAuth2 middleware: %w", err)
		}
		if oauth2MW != nil {
			oauth2Middleware = oauth2MW.Middleware()
		}
	}

	// Create HTTP client and connector
	httpClient, err := connectorFactory.CreateHTTPClient(b.config.Gateway.Backend.HTTP)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP client: %w", err)
	}
	httpConnector := connectorFactory.CreateHTTPConnector(httpClient, b.config.Gateway.Backend.HTTP)

	// Create gRPC connector
	grpcConnector := connectorFactory.CreateGRPCConnector()

	// Create base handler with multi-protocol support
	baseHandler := handlerFactory.CreateMultiProtocolHandler(gatewayRouter, httpConnector, grpcConnector)
	
	// Wrap handler to add route context for middleware
	baseHandler = handlerFactory.CreateRouteAwareHandler(gatewayRouter, baseHandler)
	
	// Add tracking middleware for load balancers
	trackingMiddleware := middlewareFactory.CreateTrackingMiddleware()
	baseHandler = trackingMiddleware.WrapHandler("gateway.tracking", baseHandler)
	
	// Add telemetry middleware if enabled
	if gatewayTelemetry != nil && telemetryMetrics != nil {
		telemetryMiddleware := middlewareFactory.CreateTelemetryMiddleware(gatewayTelemetry, telemetryMetrics)
		baseHandler = telemetryMiddleware.WrapHandler("gateway.handler", baseHandler)
		b.logger.Info("Telemetry middleware enabled")
	}

	// Add metrics middleware if enabled
	var gatewayMetrics *metrics.Metrics
	if telemetryFactory.ShouldEnableMetrics(b.config.Gateway.Metrics) {
		gatewayMetrics = telemetryFactory.CreateMetrics(b.config.Gateway.Metrics)
		metricsMiddleware := middlewareFactory.CreateMetricsMiddleware(gatewayMetrics)
		baseHandler = metricsMiddleware(baseHandler)
		b.logger.Info("Metrics enabled", "path", b.config.Gateway.Metrics.Path)
	}

	// Add circuit breaker middleware if enabled
	if cbMiddleware := middlewareFactory.CreateCircuitBreakerMiddleware(b.config.Gateway.CircuitBreaker); cbMiddleware != nil {
		baseHandler = cbMiddleware.Apply()(baseHandler)
		b.logger.Info("Circuit breaker enabled")
	}

	// Add retry middleware if enabled
	if retryMiddleware := middlewareFactory.CreateRetryMiddleware(b.config.Gateway.Retry); retryMiddleware != nil {
		baseHandler = retryMiddleware.Apply()(baseHandler)
		b.logger.Info("Retry enabled")
	}

	// Apply base middleware (recovery, logging, auth)
	var middlewares []core.Middleware
	if authMiddleware != nil {
		middlewares = append(middlewares, authMiddleware.Handler)
	}
	baseHandler = handlerFactory.ApplyMiddleware(baseHandler, middlewares...)
	
	// Apply OAuth2 middleware if configured
	if oauth2Middleware != nil {
		baseHandler = oauth2Middleware(baseHandler)
		b.logger.Info("OAuth2 authentication enabled")
	}
	
	// Apply authorization middlewares
	if b.config.Gateway.Middleware != nil && b.config.Gateway.Middleware.Authz != nil {
		authzMiddlewares, err := middlewareFactory.CreateAuthzMiddlewares(b.config.Gateway.Middleware.Authz)
		if err != nil {
			return nil, fmt.Errorf("creating authorization middlewares: %w", err)
		}
		for _, mw := range authzMiddlewares {
			baseHandler = mw(baseHandler)
		}
		if len(authzMiddlewares) > 0 {
			b.logger.Info("Authorization middlewares enabled", "count", len(authzMiddlewares))
		}
	}

	// TODO: Add versioning middleware at HTTP adapter level
	// Versioning middleware works with http.Handler, not core.Handler

	// Add rate limiting middleware after basic middleware but before business logic
	if rateLimitMiddleware := middlewareFactory.CreateRateLimitMiddleware(&b.config.Gateway.Router, &b.config.Gateway); rateLimitMiddleware != nil {
		baseHandler = rateLimitMiddleware(baseHandler)
		b.logger.Info("Rate limiting enabled for configured routes")
	}

	// Create HTTP adapter
	httpAdapterInstance, err := adapterFactory.CreateHTTPAdapter(b.config.Gateway.Frontend.HTTP, baseHandler)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP adapter: %w", err)
	}

	// Add health check support if enabled
	var healthHandler *health.Handler
	if cfg := b.config.Gateway.Health; cfg != nil && cfg.Enabled {
		// Use a simple service ID - could be enhanced to use hostname or config
		serviceID := fmt.Sprintf("gateway-%d", time.Now().Unix())
		version := "1.0.0" // Could be injected via build flags

		// Create health handler
		healthHandler, _, err = healthFactory.CreateHealthHandler(cfg, registry, version, serviceID)
		if err != nil {
			return nil, err
		}
		
		// Get backend monitor if health-aware registry is used and we didn't get it earlier
		if useHealthAware && backendMonitor != nil {
			// Register health update callback if registry supports it
			if healthRegistry, ok := registry.(*static.HealthAwareRegistry); ok {
				backendMonitor.RegisterUpdateCallback(healthRegistry.RegisterHealthUpdateCallback())
			}
			
			// Start backend monitoring
			if err := backendMonitor.Start(context.Background()); err != nil {
				return nil, fmt.Errorf("starting backend monitor: %w", err)
			}
			
			b.logger.Info("Backend health monitoring enabled")
		}
		
		// Set both health handler and health config
		healthCfg := adapterFactory.CreateHealthConfig(cfg, healthHandler)
		httpAdapterInstance.WithHealthHandler(healthHandler).WithHealthConfig(healthCfg)

		b.logger.Info("Health checks enabled",
			"health", cfg.HealthPath,
			"ready", cfg.ReadyPath,
			"live", cfg.LivePath,
		)
	}

	// Add metrics endpoint if enabled
	var metricsServer *http.Server
	if telemetryFactory.ShouldEnableMetrics(b.config.Gateway.Metrics) {
		metricsHandler := adapterFactory.CreateMetricsHandler(gatewayMetrics)
		
		// Check if metrics should be on a separate port
		if b.config.Gateway.Metrics.Port > 0 {
			// Create separate metrics server with path routing
			mux := http.NewServeMux()
			metricsPath := b.config.Gateway.Metrics.Path
			if metricsPath == "" {
				metricsPath = "/metrics"
			}
			mux.Handle(metricsPath, metricsHandler)
			
			metricsServer = &http.Server{
				Addr:    fmt.Sprintf(":%d", b.config.Gateway.Metrics.Port),
				Handler: mux,
			}
			b.logger.Info("Metrics server configured on separate port", 
				"port", b.config.Gateway.Metrics.Port,
				"path", metricsPath)
		} else {
			// Add metrics to main HTTP server
			httpAdapterInstance.WithMetricsHandler(metricsHandler)
			if b.config.Gateway.Metrics.Path != "" {
				httpAdapterInstance.WithMetricsPath(b.config.Gateway.Metrics.Path)
			}
			b.logger.Info("Metrics enabled on main server", 
				"path", b.config.Gateway.Metrics.Path)
		}
	}

	// Add CORS support if enabled
	// TODO: Apply CORS at HTTP adapter level  
	// CORS middleware works with http.Handler, not core.Handler
	if b.config.Gateway.CORS != nil && b.config.Gateway.CORS.Enabled {
		b.logger.Info("CORS enabled")
	}

	// Add SSE support if enabled
	if cfg := b.config.Gateway.Frontend.SSE; cfg != nil && cfg.Enabled {
		if err := b.addSSESupport(httpAdapterInstance, gatewayRouter, httpClient, authMiddleware, gatewayMetrics, connectorFactory, adapterFactory, middlewareFactory, handlerFactory, providerFactory); err != nil {
			return nil, fmt.Errorf("creating SSE adapter: %w", err)
		}
	}

	// Create WebSocket adapter if enabled
	var wsAdapter *wsAdapter.Adapter
	if cfg := b.config.Gateway.Frontend.WebSocket; cfg != nil && cfg.Enabled {
		var err error
		wsAdapter, err = b.createWebSocketAdapter(gatewayRouter, authMiddleware, gatewayMetrics, connectorFactory, adapterFactory, middlewareFactory, handlerFactory, providerFactory)
		if err != nil {
			return nil, fmt.Errorf("creating WebSocket adapter: %w", err)
		}
	}

	// Create Management API if enabled
	var managementAPI *management.API
	if cfg := b.config.Gateway.Management; cfg != nil && cfg.Enabled {
		managementAPI, err = managementFactory.CreateManagementAPI(cfg)
		if err != nil {
			return nil, err
		}
		if managementAPI != nil {
			// Connect managed components
			managementAPI.SetRegistry(registry)
			
			// Cast router to the expected interface
			if r, ok := gatewayRouter.(interface{ GetRoutes() []core.RouteRule }); ok {
				managementAPI.SetRouter(r)
			}
			// TODO: Set other components as they implement the required interfaces
		}
	}

	// Store router and registry in server for cleanup
	var routerCloser interface{ Close() error }
	if r, ok := gatewayRouter.(interface{ Close() error }); ok {
		routerCloser = r
	}
	
	var registryCloser interface{ Close() error }
	if r, ok := registry.(interface{ Close() error }); ok {
		registryCloser = r
	}
	
	// Only set backendMonitor interface if the concrete type is not nil
	var backendMonitorInterface interface{ Stop() error }
	if backendMonitor != nil {
		backendMonitorInterface = backendMonitor
	}
	
	// Only set telemetry interface if the concrete type is not nil
	var telemetryInterface interface{ Shutdown(context.Context) error }
	if gatewayTelemetry != nil {
		telemetryInterface = gatewayTelemetry
	}

	// Only set managementAPI interface if the concrete type is not nil
	var managementAPIInterface interface{ Start(context.Context) error; Stop(context.Context) error }
	if managementAPI != nil {
		managementAPIInterface = managementAPI
	}

	return &Server{
		config:         b.config,
		httpAdapter:    httpAdapterInstance,
		wsAdapter:      wsAdapter,
		metricsServer:  metricsServer,
		managementAPI:  managementAPIInterface,
		router:         routerCloser,
		registry:       registryCloser,
		telemetry:      telemetryInterface,
		backendMonitor: backendMonitorInterface,
		logger:         b.logger,
	}, nil
}

// addSSESupport adds SSE capabilities to the HTTP adapter
func (b *Builder) addSSESupport(
	httpAdapterInstance *httpAdapter.Adapter,
	router core.Router,
	httpClient *http.Client,
	authMiddleware *auth.Middleware,
	metrics *metrics.Metrics,
	connectorFactory *factory.ConnectorFactory,
	adapterFactory *factory.AdapterFactory,
	middlewareFactory *factory.MiddlewareFactory,
	handlerFactory *factory.HandlerFactory,
	providerFactory *factory.ProviderFactory,
) error {
	sseConnector := connectorFactory.CreateSSEConnector(b.config.Gateway.Backend.SSE, httpClient)
	sseHandler := handlerFactory.CreateSSEHandler(router, sseConnector)

	// Apply rate limiting if configured
	if rateLimitMiddleware := middlewareFactory.CreateRateLimitMiddleware(&b.config.Gateway.Router, &b.config.Gateway); rateLimitMiddleware != nil {
		sseHandler = rateLimitMiddleware(sseHandler)
	}

	var middlewares []core.Middleware
	if authMiddleware != nil {
		middlewares = append(middlewares, authMiddleware.Handler)
	}
	sseHandler = handlerFactory.ApplyMiddleware(sseHandler, middlewares...)
	
	return adapterFactory.CreateSSEAdapter(b.config.Gateway.Frontend.SSE, sseHandler, httpAdapterInstance, b.config.Gateway.Auth, metrics, providerFactory)
}

// createWebSocketAdapter creates the WebSocket adapter
func (b *Builder) createWebSocketAdapter(
	router core.Router,
	authMiddleware *auth.Middleware,
	metrics *metrics.Metrics,
	connectorFactory *factory.ConnectorFactory,
	adapterFactory *factory.AdapterFactory,
	middlewareFactory *factory.MiddlewareFactory,
	handlerFactory *factory.HandlerFactory,
	providerFactory *factory.ProviderFactory,
) (*wsAdapter.Adapter, error) {
	wsConnector := connectorFactory.CreateWebSocketConnector(b.config.Gateway.Backend.WebSocket)
	wsHandler := handlerFactory.CreateWebSocketHandler(router, wsConnector)

	// Apply rate limiting if configured
	if rateLimitMiddleware := middlewareFactory.CreateRateLimitMiddleware(&b.config.Gateway.Router, &b.config.Gateway); rateLimitMiddleware != nil {
		wsHandler = rateLimitMiddleware(wsHandler)
	}

	var middlewares []core.Middleware
	if authMiddleware != nil {
		middlewares = append(middlewares, authMiddleware.Handler)
	}
	wsHandler = handlerFactory.ApplyMiddleware(wsHandler, middlewares...)
	
	return adapterFactory.CreateWebSocketAdapter(b.config.Gateway.Frontend.WebSocket, wsHandler, b.config.Gateway.Auth, metrics, providerFactory)
}