package factory

import (
	"log/slog"

	"gateway/internal/connector"
	grpcConnector "gateway/internal/connector/grpc"
	sseConnector "gateway/internal/connector/sse"
	wsConnector "gateway/internal/connector/websocket"
	"gateway/internal/core"
	"gateway/internal/handler"
)

// HandlerFactory creates handler instances
type HandlerFactory struct {
	BaseComponentFactory
}

// NewHandlerFactory creates a new handler factory
func NewHandlerFactory(logger *slog.Logger) *HandlerFactory {
	return &HandlerFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// CreateMultiProtocolHandler creates a handler that supports multiple protocols
func (f *HandlerFactory) CreateMultiProtocolHandler(router core.Router, httpConn connector.Connector, grpcConn *grpcConnector.Connector) core.Handler {
	handlerComponent := handler.NewComponent(f.logger)
	handlerComp := handlerComponent.(*handler.Component)
	handlerComp.SetDependencies(router, httpConn, grpcConn, nil, nil)
	return handlerComp.CreateMultiProtocolHandler()
}

// CreateRouteAwareHandler creates a handler that caches routing decisions
func (f *HandlerFactory) CreateRouteAwareHandler(router core.Router, baseHandler core.Handler) core.Handler {
	handlerComponent := handler.NewComponent(f.logger)
	handlerComp := handlerComponent.(*handler.Component)
	handlerComp.SetDependencies(router, nil, nil, nil, nil)
	return handlerComp.CreateRouteAwareHandler(baseHandler)
}

// CreateSSEHandler creates an SSE-specific handler
func (f *HandlerFactory) CreateSSEHandler(router core.Router, sseConn *sseConnector.Connector) core.Handler {
	handlerComponent := handler.NewComponent(f.logger)
	handlerComp := handlerComponent.(*handler.Component)
	handlerComp.SetDependencies(router, nil, nil, sseConn, nil)
	return handlerComp.CreateSSEHandler()
}

// CreateWebSocketHandler creates a WebSocket-specific handler
func (f *HandlerFactory) CreateWebSocketHandler(router core.Router, wsConn *wsConnector.Connector) core.Handler {
	handlerComponent := handler.NewComponent(f.logger)
	handlerComp := handlerComponent.(*handler.Component)
	handlerComp.SetDependencies(router, nil, nil, nil, wsConn)
	return handlerComp.CreateWebSocketHandler()
}

// ApplyMiddleware applies middleware to a handler
func (f *HandlerFactory) ApplyMiddleware(h core.Handler, middlewares ...core.Middleware) core.Handler {
	return handler.ApplyMiddleware(h, f.logger, middlewares...)
}