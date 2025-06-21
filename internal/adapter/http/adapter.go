package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"gateway/internal/core"
	gwerrors "gateway/pkg/errors"
	"gateway/pkg/requestid"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
)

// Adapter handles HTTP requests
type Adapter struct {
	config         Config
	server         *http.Server
	handler        core.Handler
	sseHandler     SSEHandler
	healthHandler  HealthHandler
	metricsHandler http.Handler
	corsHandler    http.Handler
	reqNum         atomic.Uint64
	logger         *slog.Logger
}

// HealthHandler handles health check requests
type HealthHandler interface {
	Health(w http.ResponseWriter, r *http.Request)
	Ready(w http.ResponseWriter, r *http.Request)
	Live(w http.ResponseWriter, r *http.Request)
}

// SSEHandler handles SSE requests
type SSEHandler interface {
	HandleSSE(w http.ResponseWriter, r *http.Request)
}

// New creates a new HTTP adapter
func New(cfg Config, handler core.Handler) *Adapter {
	return &Adapter{
		config:  cfg,
		handler: handler,
		logger:  slog.Default().With("component", "http"),
	}
}

// WithSSEHandler sets the SSE handler
func (a *Adapter) WithSSEHandler(handler SSEHandler) *Adapter {
	a.sseHandler = handler
	return a
}

// WithHealthHandler sets the health handler
func (a *Adapter) WithHealthHandler(handler HealthHandler) *Adapter {
	a.healthHandler = handler
	return a
}

// WithMetricsHandler sets the metrics handler
func (a *Adapter) WithMetricsHandler(handler http.Handler) *Adapter {
	a.metricsHandler = handler
	return a
}

// WithMetricsPath sets the metrics path
func (a *Adapter) WithMetricsPath(path string) *Adapter {
	a.config.MetricsPath = path
	return a
}

// WithCORSHandler sets the CORS handler
func (a *Adapter) WithCORSHandler(handler http.Handler) *Adapter {
	a.corsHandler = handler
	return a
}

// Start starts the HTTP server
func (a *Adapter) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)

	// Use CORS handler if configured, otherwise use the adapter directly
	var handler http.Handler = a
	if a.corsHandler != nil {
		handler = a.corsHandler
	}

	a.server = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  a.config.ReadTimeout,
		WriteTimeout: a.config.WriteTimeout,
		TLSConfig:    a.config.TLSConfig,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	// Create listener to detect bind errors early
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", addr, err)
	}

	// If TLS is enabled, wrap the listener
	if a.config.TLS != nil && a.config.TLS.Enabled {
		if a.config.TLSConfig == nil {
			listener.Close()
			return fmt.Errorf("TLS enabled but no TLS configuration provided")
		}
		a.logger.Info("starting TLS server", "addr", addr, "cert", a.config.TLS.CertFile)
		listener = tls.NewListener(listener, a.config.TLSConfig)
	} else {
		a.logger.Info("starting server", "addr", addr)
	}

	// Start server in goroutine
	go func() {
		err := a.server.Serve(listener)
		if err != http.ErrServerClosed {
			a.logger.Error("server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the server
func (a *Adapter) Stop(ctx context.Context) error {
	if a.server == nil {
		return nil
	}

	a.logger.Info("stopping server", "requests", a.reqNum.Load())
	return a.server.Shutdown(ctx)
}

// ServeHTTP implements http.Handler
func (a *Adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Increment request counter
	a.reqNum.Add(1)

	// Handle health check endpoints first (no request ID needed)
	if a.healthHandler != nil {
		switch r.URL.Path {
		case "/health":
			a.healthHandler.Health(w, r)
			return
		case "/ready":
			a.healthHandler.Ready(w, r)
			return
		case "/live":
			a.healthHandler.Live(w, r)
			return
		}
	}

	// Handle metrics endpoint (default to /metrics if not configured)
	metricsPath := a.config.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	if a.metricsHandler != nil && r.URL.Path == metricsPath {
		a.metricsHandler.ServeHTTP(w, r)
		return
	}

	reqID := requestid.GenerateRequestID()

	// Add request ID to headers for downstream handlers
	r.Header.Set("X-Request-ID", reqID)

	// Check if this is an SSE request
	if a.sseHandler != nil && isSSERequest(r) {
		a.sseHandler.HandleSSE(w, r)
		return
	}

	// Copy headers
	headers := make(map[string][]string, len(r.Header))
	for k, v := range r.Header {
		headers[k] = v
	}

	// Apply request size limit if configured
	if a.config.MaxRequestSize > 0 && r.ContentLength > a.config.MaxRequestSize {
		a.logger.Warn("request body too large",
			"request_id", reqID,
			"content_length", r.ContentLength,
			"max_size", a.config.MaxRequestSize,
		)
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Wrap body with size limiter if configured
	if a.config.MaxRequestSize > 0 && r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, a.config.MaxRequestSize)
	}

	// Create request
	req := newRequest(reqID, r)

	// Handle request
	resp, err := a.handler(r.Context(), req)
	if err != nil {
		a.handleError(w, reqID, err)
		return
	}

	// Write response
	for k, values := range resp.Headers() {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode())

	if body := resp.Body(); body != nil {
		defer body.Close()
		if _, err := io.Copy(w, body); err != nil {
			a.logger.Error("failed to copy response body",
				"error", err,
				"request_id", reqID,
				"path", req.Path())
			// Don't return error as headers are already sent
		}
	}
}

// errorTypeToHTTPStatus maps error types to HTTP status codes
func errorTypeToHTTPStatus(errType gwerrors.ErrorType) int {
	switch errType {
	case gwerrors.ErrorTypeNotFound:
		return http.StatusNotFound
	case gwerrors.ErrorTypeBadRequest:
		return http.StatusBadRequest
	case gwerrors.ErrorTypeUnauthorized:
		return http.StatusUnauthorized
	case gwerrors.ErrorTypeForbidden:
		return http.StatusForbidden
	case gwerrors.ErrorTypeTimeout:
		return http.StatusRequestTimeout
	case gwerrors.ErrorTypeUnavailable:
		return http.StatusServiceUnavailable
	case gwerrors.ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// handleError handles errors by mapping them to appropriate HTTP responses
func (a *Adapter) handleError(w http.ResponseWriter, reqID string, err error) {
	var gwErr *gwerrors.Error
	var statusCode int
	var message string

	if errors.As(err, &gwErr) {
		// Structured error with proper status code
		statusCode = errorTypeToHTTPStatus(gwErr.Type)
		message = gwErr.Message
		a.logger.Error("request failed",
			"id", reqID,
			"type", gwErr.Type,
			"error", gwErr.Error(),
			"details", gwErr.Details)
	} else {
		// Generic error
		statusCode = http.StatusInternalServerError
		message = "Internal Server Error"
		a.logger.Error("request failed", "id", reqID, "error", err)
	}

	http.Error(w, message, statusCode)
}

// isSSERequest checks if the request is for SSE
func isSSERequest(r *http.Request) bool {
	// Check Accept header
	accept := r.Header.Get("Accept")
	if accept == "text/event-stream" {
		return true
	}

	// Check if path indicates SSE (configurable)
	// This is a simple heuristic; real routing should be done by the router
	return false
}
