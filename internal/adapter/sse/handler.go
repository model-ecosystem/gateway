package sse

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"

	sseConnector "gateway/internal/connector/sse"
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// Handler handles SSE requests by routing them to backend services
type Handler struct {
	router    core.Router
	connector *sseConnector.Connector
	logger    *slog.Logger
}

// NewHandler creates a new SSE handler
func NewHandler(router core.Router, connector *sseConnector.Connector, logger *slog.Logger) *Handler {
	return &Handler{
		router:    router,
		connector: connector,
		logger:    logger,
	}
}

// Handle processes SSE requests
func (h *Handler) Handle(ctx context.Context, req core.Request) (core.Response, error) {
	// Extract SSE writer from request
	sseWriter, ok := getSSEWriter(req)
	if !ok {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"Not an SSE request",
		)
	}

	// Route the request
	result, err := h.router.Route(ctx, req)
	if err != nil {
		h.logger.Error("Failed to route SSE request",
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
	headers.Set("X-Forwarded-Proto", "http")
	if host := req.Headers()["Host"]; len(host) > 0 {
		headers.Set("X-Forwarded-Host", host[0])
	}

	// Establish backend connection
	backendConn, err := h.connector.Connect(ctx, result.Instance, req.Path(), headers)
	if err != nil {
		h.logger.Error("Failed to connect to SSE backend",
			"instance", result.Instance.ID,
			"error", err,
		)
		return nil, err
	}
	defer backendConn.Close()

	// Start proxying
	if err := backendConn.Proxy(ctx, sseWriter); err != nil {
		h.logger.Debug("SSE proxy ended",
			"path", req.Path(),
			"instance", result.Instance.ID,
			"error", err,
		)
		return nil, err
	}

	// Return response indicating SSE was handled
	return &sseResponse{statusCode: http.StatusOK}, nil
}

// getSSEWriter extracts SSE writer from request
func getSSEWriter(req core.Request) (core.SSEWriter, bool) {
	// Check if this is an sseRequest type
	if sseReq, ok := req.(*sseRequest); ok {
		return sseReq.SSEWriter(), true
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

// sseResponse implements core.Response for SSE
type sseResponse struct {
	statusCode int
}

func (r *sseResponse) StatusCode() int {
	return r.statusCode
}

func (r *sseResponse) Headers() map[string][]string {
	return map[string][]string{
		"Content-Type":      {"text/event-stream"},
		"Cache-Control":     {"no-cache"},
		"Connection":        {"keep-alive"},
		"X-Accel-Buffering": {"no"},
	}
}

func (r *sseResponse) Body() io.ReadCloser {
	return io.NopCloser(nil)
}