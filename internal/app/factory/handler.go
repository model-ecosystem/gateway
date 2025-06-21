package factory

import (
	"context"
	"log/slog"

	sseAdapter "gateway/internal/adapter/sse"
	wsAdapter "gateway/internal/adapter/websocket"
	"gateway/internal/config"
	"gateway/internal/connector"
	grpcConnector "gateway/internal/connector/grpc"
	sseConnector "gateway/internal/connector/sse"
	wsConnector "gateway/internal/connector/websocket"
	"gateway/internal/core"
	"gateway/internal/middleware"
	"gateway/internal/middleware/auth"
	"gateway/internal/middleware/recovery"
	"gateway/internal/router"
	"gateway/pkg/errors"
)

// ConnectorSelector holds connectors for different protocols
type ConnectorSelector struct {
	httpConnector connector.Connector
	grpcConnector *grpcConnector.Connector
}

// CreateBaseHandler creates the base request handler
func CreateBaseHandler(router core.Router, conn connector.Connector) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Route request
		route, err := router.Route(ctx, req)
		if err != nil {
			return nil, err
		}

		// Forward request using connector
		return conn.Forward(ctx, req, route)
	}
}

// CreateMultiProtocolHandler creates a handler that supports multiple protocols
func CreateMultiProtocolHandler(router core.Router, httpConn connector.Connector, grpcConn *grpcConnector.Connector) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Route request
		route, err := router.Route(ctx, req)
		if err != nil {
			return nil, err
		}

		// Select connector based on protocol
		switch route.Rule.Protocol {
		case "grpc":
			if grpcConn == nil {
				return nil, errors.NewError(errors.ErrorTypeInternal, "gRPC connector not configured")
			}
			return grpcConn.Forward(ctx, req, route)
		case "http", "":
			// Default to HTTP for backward compatibility
			return httpConn.Forward(ctx, req, route)
		default:
			return nil, errors.NewError(errors.ErrorTypeBadRequest, "unsupported protocol: "+route.Rule.Protocol)
		}
	}
}

// CreateSSEHandler creates an SSE-specific handler
func CreateSSEHandler(
	router core.Router,
	connector *sseConnector.Connector,
	logger *slog.Logger,
) core.Handler {
	handler := sseAdapter.NewHandler(router, connector, logger)
	return handler.Handle
}

// CreateWebSocketHandler creates a WebSocket-specific handler
func CreateWebSocketHandler(
	router core.Router,
	connector *wsConnector.Connector,
	logger *slog.Logger,
) core.Handler {
	handler := wsAdapter.NewHandler(router, connector, logger)
	return handler.Handle
}

// ApplyMiddleware applies middleware chain to a handler
func ApplyMiddleware(handler core.Handler, logger *slog.Logger, authMiddleware *auth.Middleware) core.Handler {
	// Create recovery middleware with proper configuration
	recoveryMiddleware := recovery.Default(logger)

	middlewares := []core.Middleware{
		recoveryMiddleware,
		middleware.Logging(logger),
	}

	// Add auth middleware if configured
	if authMiddleware != nil {
		middlewares = append([]core.Middleware{authMiddleware.Handler}, middlewares...)
	}

	return middleware.Chain(middlewares...)(handler)
}

// CreateRouter creates and configures a router
func CreateRouter(registry core.ServiceRegistry, rules []core.RouteRule) (core.Router, error) {
	r := router.NewRouter(registry)

	for _, rule := range rules {
		if err := r.AddRule(rule); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// CreateRouterFromConfig creates a router from configuration
func CreateRouterFromConfig(registry core.ServiceRegistry, cfg *config.Router) (core.Router, error) {
	rules := make([]core.RouteRule, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		rules = append(rules, rule.ToRouteRule())
	}
	return CreateRouter(registry, rules)
}
