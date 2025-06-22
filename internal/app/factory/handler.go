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
		// Get route from context (set by route-aware middleware)
		var route *core.RouteResult
		if r := getRouteFromContext(ctx); r != nil {
			route = r
		} else {
			// Fallback: route if not in context
			var err error
			route, err = router.Route(ctx, req)
			if err != nil {
				return nil, err
			}
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

// routeContextKey is the key for storing route info in context
type routeContextKey struct{}

// getRouteFromContext retrieves the route from context
func getRouteFromContext(ctx context.Context) *core.RouteResult {
	if route, ok := ctx.Value(routeContextKey{}).(*core.RouteResult); ok {
		return route
	}
	return nil
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

	// Add route-aware auth middleware if configured
	if authMiddleware != nil {
		// Create route-aware auth that checks per-route requirements
		routeAuthMiddleware := createRouteAwareAuthMiddleware(authMiddleware, logger)
		middlewares = append([]core.Middleware{routeAuthMiddleware}, middlewares...)
	}

	return middleware.Chain(middlewares...)(handler)
}

// createRouteAwareAuthMiddleware creates middleware that checks per-route auth requirements
func createRouteAwareAuthMiddleware(authMiddleware *auth.Middleware, logger *slog.Logger) core.Middleware {
	authHandler := authMiddleware.Handler
	
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Get route from context
			route := getRouteFromContext(ctx)
			if route == nil {
				// No route info, apply auth globally
				return authHandler(next)(ctx, req)
			}

			// Check if route requires authentication
			authRequired := false
			if metadata := route.Rule.Metadata; metadata != nil {
				if val, ok := metadata["authRequired"].(bool); ok {
					authRequired = val
				}
			}

			if !authRequired {
				// Route doesn't require auth
				logger.Debug("Route does not require authentication",
					"route", route.Rule.ID,
					"path", req.Path(),
				)
				return next(ctx, req)
			}

			// Route requires auth, apply authentication
			logger.Debug("Route requires authentication", 
				"route", route.Rule.ID,
				"path", req.Path(),
			)
			return authHandler(next)(ctx, req)
		}
	}
}

// CreateRouteAwareHandler wraps a handler to add route information to context
func CreateRouteAwareHandler(router core.Router, handler core.Handler) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Route request first
		route, err := router.Route(ctx, req)
		if err != nil {
			return nil, err
		}

		// Store route in context
		ctx = context.WithValue(ctx, routeContextKey{}, route)

		// Call the handler
		return handler(ctx, req)
	}
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
