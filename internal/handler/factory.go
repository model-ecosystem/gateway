package handler

import (
	"context"
	"fmt"
	"log/slog"

	"gateway/internal/connector"
	grpcConnector "gateway/internal/connector/grpc"
	sseConnector "gateway/internal/connector/sse"
	wsConnector "gateway/internal/connector/websocket"
	"gateway/internal/core"
	"gateway/internal/middleware/recovery"
	"gateway/pkg/errors"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "handler"

// Component implements factory.Component for handler creation
type Component struct {
	router        core.Router
	httpConnector connector.Connector
	grpcConnector *grpcConnector.Connector
	sseConnector  *sseConnector.Connector
	wsConnector   *wsConnector.Connector
	logger        *slog.Logger
}

// NewComponent creates a new handler component
func NewComponent(logger *slog.Logger) factory.Component {
	return &Component{
		logger: logger,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Handler component doesn't have its own configuration
	// It relies on injected dependencies
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.router == nil {
		return fmt.Errorf("router not set")
	}
	if c.httpConnector == nil {
		return fmt.Errorf("HTTP connector not set")
	}
	return nil
}

// SetDependencies sets the required dependencies
func (c *Component) SetDependencies(
	router core.Router,
	httpConnector connector.Connector,
	grpcConnector *grpcConnector.Connector,
	sseConnector *sseConnector.Connector,
	wsConnector *wsConnector.Connector,
) {
	c.router = router
	c.httpConnector = httpConnector
	c.grpcConnector = grpcConnector
	c.sseConnector = sseConnector
	c.wsConnector = wsConnector
}

// CreateBaseHandler creates the base request handler
func (c *Component) CreateBaseHandler() core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Route request
		route, err := c.router.Route(ctx, req)
		if err != nil {
			return nil, err
		}

		// Forward request using connector
		return c.httpConnector.Forward(ctx, req, route)
	}
}

// CreateMultiProtocolHandler creates a handler that supports multiple protocols
func (c *Component) CreateMultiProtocolHandler() core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Get route from context (set by route-aware middleware)
		var route *core.RouteResult
		if r := getRouteFromContext(ctx); r != nil {
			route = r
		} else {
			// Fallback: route if not in context
			var err error
			route, err = c.router.Route(ctx, req)
			if err != nil {
				return nil, err
			}
		}

		// For now, we only support HTTP through the standard connector
		// gRPC support would require protocol detection from request headers
		return c.httpConnector.Forward(ctx, req, route)
	}
}

// CreateSSEHandler creates an SSE-specific handler
func (c *Component) CreateSSEHandler() core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Route request
		_, err := c.router.Route(ctx, req)
		if err != nil {
			return nil, err
		}

		// SSE requires special handling, not a simple forward
		// This would typically be handled by the SSE adapter
		return nil, errors.NewError(errors.ErrorTypeInternal, "SSE handling should be done through SSE adapter")
	}
}

// CreateWebSocketHandler creates a WebSocket-specific handler
func (c *Component) CreateWebSocketHandler() core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Route request
		_, err := c.router.Route(ctx, req)
		if err != nil {
			return nil, err
		}

		// WebSocket requires special handling, not a simple forward
		// This would typically be handled by the WebSocket adapter
		return nil, errors.NewError(errors.ErrorTypeInternal, "WebSocket handling should be done through WebSocket adapter")
	}
}

// CreateRouteAwareHandler creates a handler that caches routing decisions
func (c *Component) CreateRouteAwareHandler(baseHandler core.Handler) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Check if route is already in context
		if getRouteFromContext(ctx) != nil {
			// Route already determined, just call handler
			return baseHandler(ctx, req)
		}

		// Route the request
		route, err := c.router.Route(ctx, req)
		if err != nil {
			return nil, err
		}

		// Store route in context for downstream use
		ctx = setRouteInContext(ctx, route)

		// Call the base handler with the enhanced context
		return baseHandler(ctx, req)
	}
}

// ApplyMiddleware applies middleware to a handler
func ApplyMiddleware(handler core.Handler, logger *slog.Logger, middlewares ...core.Middleware) core.Handler {
	// Always add recovery middleware first
	recoveryConfig := recovery.Config{
		StackTrace: true,
	}
	handler = recovery.Middleware(recoveryConfig, logger)(handler)

	// Apply other middleware in reverse order
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			handler = middlewares[i](handler)
		}
	}

	return handler
}

// routeContextKey is the context key for storing route results
type routeContextKey struct{}

// setRouteInContext stores the route result in the context
func setRouteInContext(ctx context.Context, route *core.RouteResult) context.Context {
	return context.WithValue(ctx, routeContextKey{}, route)
}

// getRouteFromContext retrieves the route result from the context
func getRouteFromContext(ctx context.Context) *core.RouteResult {
	if route, ok := ctx.Value(routeContextKey{}).(*core.RouteResult); ok {
		return route
	}
	return nil
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)