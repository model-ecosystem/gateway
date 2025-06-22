package sse

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"gateway/internal/core"
	"gateway/pkg/request"
)

// Config represents SSE adapter configuration
type Config struct {
	Enabled          bool `yaml:"enabled"`
	WriteTimeout     int  `yaml:"writeTimeout"`     // Write timeout in seconds
	KeepaliveTimeout int  `yaml:"keepaliveTimeout"` // Keepalive interval in seconds
}

// TokenValidator interface for JWT token validation
type TokenValidator interface {
	ValidateConnection(ctx context.Context, connectionID string, token string, onExpired func()) error
	StopValidation(connectionID string)
}

// DefaultConfig returns default SSE configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:          false,
		WriteTimeout:     60,
		KeepaliveTimeout: 30,
	}
}

// Adapter handles SSE requests as part of HTTP adapter
type Adapter struct {
	config         *Config
	handler        core.Handler
	logger         *slog.Logger
	tokenValidator TokenValidator
	metrics        *SSEMetrics
}

// NewAdapter creates a new SSE adapter
func NewAdapter(config *Config, handler core.Handler, logger *slog.Logger) *Adapter {
	if config == nil {
		config = DefaultConfig()
	}

	return &Adapter{
		config:  config,
		handler: handler,
		logger:  logger,
	}
}

// WithTokenValidator sets the token validator for the adapter
func (a *Adapter) WithTokenValidator(validator TokenValidator) *Adapter {
	a.tokenValidator = validator
	return a
}

// WithMetrics sets the metrics for the adapter
func (a *Adapter) WithMetrics(metrics *SSEMetrics) *Adapter {
	a.metrics = metrics
	return a
}

// HandleSSE handles an SSE request
func (a *Adapter) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Check if client accepts SSE
	accept := r.Header.Get("Accept")
	if accept != "" && accept != "text/event-stream" && accept != "*/*" {
		http.Error(w, "SSE not accepted", http.StatusNotAcceptable)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering

	// Create SSE writer with disconnect detection
	sseWriter := newWriter(w, r.Context(), a.metrics)
	defer func() {
		if err := sseWriter.Close(); err != nil {
			a.logger.Debug("SSE writer close error", "error", err)
		}
	}()

	// Track SSE connection
	if a.metrics != nil {
		if a.metrics.ConnectionsTotal != nil {
			a.metrics.ConnectionsTotal.WithLabelValues("", "established").Inc()
		}
		if a.metrics.Connections != nil {
			a.metrics.Connections.Inc()
			defer a.metrics.Connections.Dec()
		}
	}

	// Create SSE request with GET method for routing compatibility
	req := &sseRequest{
		BaseRequest: request.NewBase(r.Header.Get("X-Request-ID"), r, "GET", "sse"),
		writer:      sseWriter,
	}

	// Create a cancellable context for the entire SSE connection
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start keepalive goroutine
	keepaliveCtx, cancelKeepalive := context.WithCancel(ctx)
	defer cancelKeepalive()

	// Start disconnect monitor
	disconnectCtx, cancelDisconnect := context.WithCancel(ctx)
	defer cancelDisconnect()
	go a.monitorDisconnect(disconnectCtx, sseWriter, r.RemoteAddr)

	// Start JWT validation if configured
	if a.tokenValidator != nil {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token := authHeader[7:]
			connectionID := r.Header.Get("X-Request-ID")
			if connectionID == "" {
				connectionID = r.RemoteAddr
			}

			// Start token validation
			err := a.tokenValidator.ValidateConnection(ctx, connectionID, token, func() {
				// Token expired, close the connection
				a.logger.Info("JWT token expired, closing SSE connection",
					"connectionID", connectionID,
					"remote", r.RemoteAddr,
				)

				// Send error event and close
				_ = sseWriter.WriteEvent(&core.SSEEvent{
					Type: "error",
					Data: "authentication expired",
				})
				_ = sseWriter.Close()

				// Cancel all contexts to stop the handler and keepalive
				cancel()
			})

			if err != nil {
				// Initial validation failed
				a.logger.Error("JWT validation failed for SSE connection",
					"error", err,
					"remote", r.RemoteAddr,
				)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Stop validation when connection closes
			defer a.tokenValidator.StopValidation(connectionID)
		}
	}

	if a.config.KeepaliveTimeout > 0 {
		go a.keepalive(keepaliveCtx, sseWriter)
	}

	// Handle the request through the handler chain
	resp, err := a.handler(ctx, req)
	if err != nil {
		a.logger.Error("SSE handler error",
			"error", err,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
		)

		// Try to send error event (ignore error if client disconnected)
		_ = sseWriter.WriteEvent(&core.SSEEvent{
			Type: "error",
			Data: err.Error(),
		})
		return
	}

	// Check if handler successfully processed the SSE request
	if resp != nil && resp.StatusCode() == http.StatusOK {
		a.logger.Debug("SSE connection completed",
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"disconnected", sseWriter.IsDisconnected(),
		)
	}
}

// keepalive sends periodic comments to keep connection alive
func (a *Adapter) keepalive(ctx context.Context, writer core.SSEWriter) {
	ticker := time.NewTicker(time.Duration(a.config.KeepaliveTimeout) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := writer.WriteComment("keepalive"); err != nil {
				a.logger.Debug("SSE keepalive failed, client likely disconnected", "error", err)
				return
			}
		}
	}
}

// monitorDisconnect monitors for client disconnection
func (a *Adapter) monitorDisconnect(ctx context.Context, writer core.SSEWriter, remoteAddr string) {
	// For now, just monitor context cancellation
	// The actual disconnect detection happens in the writer methods
	<-ctx.Done()
	a.logger.Info("SSE connection context cancelled", "remote", remoteAddr)
}

// sseRequest implements core.Request for SSE
type sseRequest struct {
	*request.BaseRequest
	writer core.SSEWriter
}

// SSEWriter returns the SSE writer
func (r *sseRequest) SSEWriter() core.SSEWriter {
	return r.writer
}
