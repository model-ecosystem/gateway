package http

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/core"
	gwerrors "gateway/pkg/errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// Config holds HTTP adapter configuration
type Config struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	TLS          *TLSConfig
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled    bool
	CertFile   string
	KeyFile    string
	MinVersion string
}

// Adapter handles HTTP requests
type Adapter struct {
	config  Config
	server  *http.Server
	handler core.Handler
	reqNum  atomic.Uint64
	logger  *slog.Logger
}

// New creates a new HTTP adapter
func New(cfg Config, handler core.Handler) *Adapter {
	return &Adapter{
		config:  cfg,
		handler: handler,
		logger:  slog.Default().With("component", "http"),
	}
}

// Start starts the HTTP server
func (a *Adapter) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
	
	a.server = &http.Server{
		Addr:         addr,
		Handler:      a,
		ReadTimeout:  a.config.ReadTimeout,
		WriteTimeout: a.config.WriteTimeout,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	
	a.logger.Info("starting server", "addr", addr)
	
	go func() {
		var err error
		if a.config.TLS != nil && a.config.TLS.Enabled {
			a.logger.Info("starting TLS server", "cert", a.config.TLS.CertFile)
			err = a.server.ListenAndServeTLS(a.config.TLS.CertFile, a.config.TLS.KeyFile)
		} else {
			err = a.server.ListenAndServe()
		}
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
	reqID := fmt.Sprintf("%d", a.reqNum.Add(1))
	
	// Copy headers
	headers := make(map[string][]string, len(r.Header))
	for k, v := range r.Header {
		headers[k] = v
	}
	
	// Create request
	req := core.NewRequest(reqID, r.Method, r.URL.Path, r.URL.String(), r.RemoteAddr, headers, r.Body, r.Context())
	
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

// handleError handles errors by mapping them to appropriate HTTP responses
func (a *Adapter) handleError(w http.ResponseWriter, reqID string, err error) {
	var gwErr *gwerrors.Error
	var statusCode int
	var message string

	if errors.As(err, &gwErr) {
		// Structured error with proper status code
		statusCode = gwErr.HTTPStatusCode()
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