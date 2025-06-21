package factory

import (
	"crypto/tls"
	"log/slog"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
	httpAdapter "gateway/internal/adapter/http"
	sseAdapter "gateway/internal/adapter/sse"
	wsAdapter "gateway/internal/adapter/websocket"
	"gateway/pkg/errors"
	tlsutil "gateway/pkg/tls"
)

// CreateHTTPAdapter creates an HTTP frontend adapter
func CreateHTTPAdapter(cfg config.HTTP, handler core.Handler, logger *slog.Logger) *httpAdapter.Adapter {
	httpConfig := httpAdapter.Config{
		Host:           cfg.Host,
		Port:           cfg.Port,
		ReadTimeout:    time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.WriteTimeout) * time.Second,
		MaxRequestSize: cfg.MaxRequestSize,
	}

	// Add TLS config if enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsConfig, err := createTLSConfig(cfg.TLS)
		if err != nil {
			logger.Error("Failed to create TLS config", "error", err)
			// Continue without TLS rather than failing
		} else {
			httpConfig.TLSConfig = tlsConfig
			httpConfig.TLS = &httpAdapter.TLSConfig{
				Enabled:    true,
				CertFile:   cfg.TLS.CertFile,
				KeyFile:    cfg.TLS.KeyFile,
				MinVersion: cfg.TLS.MinVersion,
			}
		}
	}

	return httpAdapter.New(httpConfig, handler)
}

// CreateSSEAdapter creates an SSE adapter that integrates with HTTP
func CreateSSEAdapter(
	cfg *config.SSE,
	handler core.Handler,
	httpAdapterInstance *httpAdapter.Adapter,
	logger *slog.Logger,
) {
	if cfg == nil || !cfg.Enabled {
		return
	}

	sseConfig := &sseAdapter.Config{
		Enabled:          cfg.Enabled,
		WriteTimeout:     cfg.WriteTimeout,
		KeepaliveTimeout: cfg.KeepaliveTimeout,
	}

	sse := sseAdapter.NewAdapter(sseConfig, handler, logger)
	httpAdapterInstance.WithSSEHandler(sse)
}

// CreateWebSocketAdapter creates a WebSocket frontend adapter
func CreateWebSocketAdapter(
	cfg *config.WebSocket,
	handler core.Handler,
	logger *slog.Logger,
) *wsAdapter.Adapter {
	if cfg == nil {
		return nil
	}

	wsConfig := &wsAdapter.Config{
		Host:              cfg.Host,
		Port:              cfg.Port,
		ReadTimeout:       time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(cfg.WriteTimeout) * time.Second,
		HandshakeTimeout:  time.Duration(cfg.HandshakeTimeout) * time.Second,
		ReadBufferSize:    cfg.ReadBufferSize,
		WriteBufferSize:   cfg.WriteBufferSize,
		EnableCompression: cfg.EnableCompression,
		MaxMessageSize:    cfg.MaxMessageSize,
		CheckOrigin:       cfg.CheckOrigin,
		AllowedOrigins:    cfg.AllowedOrigins,
	}

	// TLS configuration would be added here if needed
	// For now, using default non-TLS configuration

	return wsAdapter.NewAdapter(wsConfig, handler, logger)
}

// createTLSConfig creates a tls.Config from configuration
func createTLSConfig(cfg *config.TLS) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	// Load server certificate
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to load TLS certificate").WithCause(err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Set minimum version
	if cfg.MinVersion != "" {
		tlsConfig.MinVersion = tlsutil.ParseTLSVersion(cfg.MinVersion)
	}

	// Set maximum version
	if cfg.MaxVersion != "" {
		tlsConfig.MaxVersion = tlsutil.ParseTLSVersion(cfg.MaxVersion)
	}

	// Set prefer server cipher suites
	tlsConfig.PreferServerCipherSuites = cfg.PreferServerCipher

	return tlsConfig, nil
}