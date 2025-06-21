package websocket

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	wsConnector "gateway/internal/connector/websocket"
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// Handler handles WebSocket requests by routing them to backend services
type Handler struct {
	router    core.Router
	connector *wsConnector.Connector
	logger    *slog.Logger
}

// NewHandler creates a new WebSocket handler
func NewHandler(router core.Router, connector *wsConnector.Connector, logger *slog.Logger) *Handler {
	return &Handler{
		router:    router,
		connector: connector,
		logger:    logger,
	}
}

// Handle processes WebSocket requests
func (h *Handler) Handle(ctx context.Context, req core.Request) (core.Response, error) {
	// Extract WebSocket connection from request context
	wsConn, ok := getWebSocketConn(req)
	if !ok {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"Not a WebSocket request",
		)
	}

	// Route the request
	result, err := h.router.Route(ctx, req)
	if err != nil {
		h.logger.Error("Failed to route WebSocket request",
			"path", req.Path(),
			"error", err,
		)
		return nil, err
	}

	// Connect to backend
	headers := make(http.Header)
	for k, v := range req.Headers() {
		// Skip hop-by-hop headers
		if isHopByHopHeader(k) {
			continue
		}
		headers[k] = v
	}

	// Add X-Forwarded headers
	headers.Set("X-Forwarded-For", req.RemoteAddr())
	headers.Set("X-Forwarded-Proto", "ws")
	headers.Set("X-Forwarded-Host", req.Headers()["Host"][0])

	// Establish backend connection
	backendConn, err := h.connector.Connect(ctx, result.Instance, req.Path(), headers)
	if err != nil {
		h.logger.Error("Failed to connect to WebSocket backend",
			"instance", result.Instance.ID,
			"error", err,
		)
		return nil, err
	}

	// Start proxying in a goroutine
	go func() {
		if err := backendConn.Proxy(ctx, wsConn); err != nil {
			h.logger.Debug("WebSocket proxy ended",
				"path", req.Path(),
				"instance", result.Instance.ID,
				"error", err,
			)
		}
	}()

	// Return response indicating WebSocket is being handled
	return newResponse(wsConn, http.StatusSwitchingProtocols), nil
}

// getWebSocketConn extracts WebSocket connection from request
func getWebSocketConn(req core.Request) (core.WebSocketConn, bool) {
	// Check if this is a wsRequest type
	if wsReq, ok := req.(*wsRequest); ok {
		return wsReq.conn, true
	}
	return nil, false
}

// isHopByHopHeader checks if a header is hop-by-hop
func isHopByHopHeader(header string) bool {
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"TE",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}
	
	header = strings.ToLower(header)
	for _, h := range hopByHopHeaders {
		if strings.ToLower(h) == header {
			return true
		}
	}
	return false
}