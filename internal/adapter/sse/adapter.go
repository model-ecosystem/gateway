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
	config  *Config
	handler core.Handler
	logger  *slog.Logger
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

	// Create SSE writer
	sseWriter := newWriter(w)
	defer sseWriter.Close()

	// Create SSE request with GET method for routing compatibility
	req := &sseRequest{
		BaseRequest: request.NewBase(r.Header.Get("X-Request-ID"), r, "GET", "sse"),
		writer:      sseWriter,
	}

	// Start keepalive goroutine
	ctx := r.Context()
	keepaliveCtx, cancelKeepalive := context.WithCancel(ctx)
	defer cancelKeepalive()

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
		
		// Try to send error event
		sseWriter.WriteEvent(&core.SSEEvent{
			Type: "error",
			Data: err.Error(),
		})
		return
	}

	// Check if handler successfully processed the SSE request
	if resp != nil && resp.StatusCode() == http.StatusOK {
		a.logger.Debug("SSE connection handled", "path", r.URL.Path)
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
				a.logger.Debug("SSE keepalive failed", "error", err)
				return
			}
		}
	}
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