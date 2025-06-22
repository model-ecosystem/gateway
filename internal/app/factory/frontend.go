package factory

import (
	"crypto/tls"
	"log/slog"
	"time"

	httpAdapter "gateway/internal/adapter/http"
	sseAdapter "gateway/internal/adapter/sse"
	wsAdapter "gateway/internal/adapter/websocket"
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/metrics"
	"gateway/internal/middleware/auth/jwt"
	"gateway/pkg/errors"
	tlsutil "gateway/pkg/tls"
)

// CreateHTTPAdapter creates an HTTP frontend adapter
func CreateHTTPAdapter(cfg config.HTTP, handler core.Handler, logger *slog.Logger) (*httpAdapter.Adapter, error) {
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
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to create TLS configuration").WithCause(err)
		}
		httpConfig.TLSConfig = tlsConfig
		httpConfig.TLS = &httpAdapter.TLSConfig{
			Enabled:    true,
			CertFile:   cfg.TLS.CertFile,
			KeyFile:    cfg.TLS.KeyFile,
			MinVersion: cfg.TLS.MinVersion,
		}
	}

	return httpAdapter.New(httpConfig, handler), nil
}

// CreateSSEAdapter creates an SSE adapter that integrates with HTTP
func CreateSSEAdapter(
	cfg *config.SSE,
	handler core.Handler,
	httpAdapterInstance *httpAdapter.Adapter,
	authConfig *config.Auth,
	logger *slog.Logger,
	metrics *metrics.Metrics,
) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	sseConfig := &sseAdapter.Config{
		Enabled:          cfg.Enabled,
		WriteTimeout:     cfg.WriteTimeout,
		KeepaliveTimeout: cfg.KeepaliveTimeout,
	}

	sse := sseAdapter.NewAdapter(sseConfig, handler, logger)

	// Add metrics if provided
	if metrics != nil {
		// Extract SSE-specific metrics
		var sseMetrics *sseAdapter.SSEMetrics
		if metrics.SSEConnections != nil {
			sseMetrics = sseAdapter.NewSSEMetrics(
				metrics.SSEConnections.WithLabelValues(""),
				metrics.SSEConnectionsTotal,
				metrics.SSEEventsSent.WithLabelValues(""),
			)
			sse.WithMetrics(sseMetrics)
		}
	}

	// Add JWT token validator if JWT auth is enabled
	if authConfig != nil && authConfig.JWT != nil && authConfig.JWT.Enabled {
		jwtProvider, err := createJWTProvider(authConfig.JWT, logger)
		if err != nil {
			// Fail closed - return error to prevent insecure startup
			return errors.NewError(errors.ErrorTypeInternal, "failed to create JWT provider for SSE").WithCause(err)
		}
		// Create token validator
		tokenValidator := jwt.NewTokenValidator(jwtProvider, logger)
		sse.WithTokenValidator(tokenValidator)
		logger.Info("JWT token validation enabled for SSE connections")
	}

	httpAdapterInstance.WithSSEHandler(sse)
	return nil
}

// CreateWebSocketAdapter creates a WebSocket frontend adapter
func CreateWebSocketAdapter(
	cfg *config.WebSocket,
	handler core.Handler,
	authConfig *config.Auth,
	logger *slog.Logger,
	metrics *metrics.Metrics,
) (*wsAdapter.Adapter, error) {
	if cfg == nil {
		return nil, nil
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

	adapter := wsAdapter.NewAdapter(wsConfig, handler, logger)

	// Add metrics if provided
	if metrics != nil {
		// Extract WebSocket-specific metrics
		var wsMetrics *wsAdapter.WebSocketMetrics
		if metrics.WebSocketConnections != nil {
			// Find the gauge for this service (empty label for now)
			wsMetrics = wsAdapter.NewWebSocketMetrics(
				metrics.WebSocketConnections.WithLabelValues(""),
				metrics.WebSocketConnectionsTotal,
				metrics.WebSocketMessagesSent.WithLabelValues(""),
				metrics.WebSocketMessagesReceived.WithLabelValues(""),
			)
			adapter.WithMetrics(wsMetrics)
		}
	}

	// Add JWT token validator if JWT auth is enabled
	if authConfig != nil && authConfig.JWT != nil && authConfig.JWT.Enabled {
		jwtProvider, err := createJWTProvider(authConfig.JWT, logger)
		if err != nil {
			// Fail closed - return error to prevent insecure startup
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to create JWT provider for WebSocket").WithCause(err)
		}
		// Create token validator
		tokenValidator := jwt.NewTokenValidator(jwtProvider, logger)
		adapter.WithTokenValidator(tokenValidator)
		logger.Info("JWT token validation enabled for WebSocket connections")
	}

	return adapter, nil
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

	// Set minimum version - default to TLS 1.2 for security
	if cfg.MinVersion != "" {
		tlsConfig.MinVersion = tlsutil.ParseTLSVersion(cfg.MinVersion)
	} else {
		tlsConfig.MinVersion = tls.VersionTLS12
	}

	// Set maximum version
	if cfg.MaxVersion != "" {
		tlsConfig.MaxVersion = tlsutil.ParseTLSVersion(cfg.MaxVersion)
	}

	// PreferServerCipherSuites is deprecated since Go 1.18 and ignored

	return tlsConfig, nil
}
