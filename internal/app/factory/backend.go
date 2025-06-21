package factory

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"time"

	"gateway/internal/config"
	"gateway/internal/connector"
	grpcConnector "gateway/internal/connector/grpc"
	sseConnector "gateway/internal/connector/sse"
	wsConnector "gateway/internal/connector/websocket"
	"gateway/pkg/errors"
)

// CreateHTTPClient creates an optimized HTTP client from configuration
func CreateHTTPClient(cfg config.HTTPBackend) *http.Client {
	// Create dialer with keep-alive settings
	dialer := &net.Dialer{
		Timeout: time.Duration(cfg.DialTimeout) * time.Second,
	}

	if cfg.KeepAlive {
		dialer.KeepAlive = time.Duration(cfg.KeepAliveTimeout) * time.Second
	} else {
		dialer.KeepAlive = -1 // Disable keep-alive
	}

	// Create transport with connection pooling
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(cfg.IdleConnTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(cfg.ResponseHeaderTimeout) * time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    false,
	}

	// Configure TLS if enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsConfig, err := createBackendTLSConfig(cfg.TLS)
		if err != nil {
			// Log error but continue with default TLS
			slog.Default().Error("Failed to create backend TLS config", "error", err)
		} else {
			transport.TLSClientConfig = tlsConfig
		}
	}

	return &http.Client{
		Transport: transport,
		// No timeout here, we use context timeout per request
	}
}

// CreateHTTPConnector creates an HTTP backend connector
func CreateHTTPConnector(client *http.Client, cfg config.HTTPBackend) connector.Connector {
	// Use response header timeout as default timeout, fallback to 30s
	defaultTimeout := time.Duration(cfg.ResponseHeaderTimeout) * time.Second
	if defaultTimeout == 0 {
		defaultTimeout = 30 * time.Second
	}
	return connector.NewHTTPConnector(client, defaultTimeout)
}

// CreateSSEConnector creates an SSE backend connector
func CreateSSEConnector(cfg *config.SSEBackend, client *http.Client, logger *slog.Logger) *sseConnector.Connector {
	sseConfig := &sseConnector.Config{
		DialTimeout:      10 * time.Second,
		ResponseTimeout:  30 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
	}

	if cfg != nil {
		if cfg.ConnectTimeout > 0 {
			sseConfig.DialTimeout = time.Duration(cfg.ConnectTimeout) * time.Second
		}
		if cfg.ReadTimeout > 0 {
			sseConfig.ResponseTimeout = time.Duration(cfg.ReadTimeout) * time.Second
		}
		// Use connect timeout as keepalive for now
		if cfg.ConnectTimeout > 0 {
			sseConfig.KeepaliveTimeout = time.Duration(cfg.ConnectTimeout) * time.Second
		}
	}

	return sseConnector.NewConnector(sseConfig, client, logger)
}

// CreateWebSocketConnector creates a WebSocket backend connector
func CreateWebSocketConnector(cfg *config.WebSocketBackend, logger *slog.Logger) *wsConnector.Connector {
	wsConfig := &wsConnector.Config{
		HandshakeTimeout:  10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		MaxMessageSize:    1024 * 1024,
		MaxConnections:    10,
		ConnectionTimeout: 10 * time.Second,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
		CloseTimeout:      5 * time.Second,
		EnableCompression: false,
		CompressionLevel:  0,
	}

	if cfg != nil {
		if cfg.ConnectionTimeout > 0 {
			wsConfig.ConnectionTimeout = time.Duration(cfg.ConnectionTimeout) * time.Second
		}
		if cfg.HandshakeTimeout > 0 {
			wsConfig.HandshakeTimeout = time.Duration(cfg.HandshakeTimeout) * time.Second
		}
		if cfg.ReadBufferSize > 0 {
			wsConfig.ReadBufferSize = cfg.ReadBufferSize
		}
		if cfg.WriteBufferSize > 0 {
			wsConfig.WriteBufferSize = cfg.WriteBufferSize
		}
		if cfg.MaxMessageSize > 0 {
			wsConfig.MaxMessageSize = cfg.MaxMessageSize
		}
		if cfg.MaxConnections > 0 {
			wsConfig.MaxConnections = cfg.MaxConnections
		}
	}

	return wsConnector.NewConnector(wsConfig, logger)
}

// createBackendTLSConfig creates TLS configuration for backend connections
func createBackendTLSConfig(cfg *config.BackendTLS) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.ServerName,
	}

	// Load client certificate for mTLS
	if cfg.ClientCertFile != "" && cfg.ClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to load client certificate").WithCause(err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate for server verification
	if cfg.RootCAFile != "" {
		// Note: For now, just set InsecureSkipVerify based on config
		// Full CA loading implementation would require additional setup
		tlsConfig.InsecureSkipVerify = cfg.InsecureSkipVerify
	}

	// Set minimum version
	if cfg.MinVersion != "" {
		switch cfg.MinVersion {
		case "1.0":
			tlsConfig.MinVersion = tls.VersionTLS10
		case "1.1":
			tlsConfig.MinVersion = tls.VersionTLS11
		case "1.2":
			tlsConfig.MinVersion = tls.VersionTLS12
		case "1.3":
			tlsConfig.MinVersion = tls.VersionTLS13
		default:
			tlsConfig.MinVersion = tls.VersionTLS12
		}
	}

	// Set maximum version
	if cfg.MaxVersion != "" {
		switch cfg.MaxVersion {
		case "1.0":
			tlsConfig.MaxVersion = tls.VersionTLS10
		case "1.1":
			tlsConfig.MaxVersion = tls.VersionTLS11
		case "1.2":
			tlsConfig.MaxVersion = tls.VersionTLS12
		case "1.3":
			tlsConfig.MaxVersion = tls.VersionTLS13
		}
	}

	// Set basic security settings
	tlsConfig.PreferServerCipherSuites = cfg.PreferServerCipher
	tlsConfig.Renegotiation = tls.RenegotiateNever
	if cfg.Renegotiation {
		tlsConfig.Renegotiation = tls.RenegotiateFreelyAsClient
	}

	return tlsConfig, nil
}

// CreateGRPCConnector creates a gRPC backend connector
func CreateGRPCConnector(logger *slog.Logger) *grpcConnector.Connector {
	grpcConfig := &grpcConnector.Config{
		MaxConcurrentStreams:  100,
		InitialConnWindowSize: 1024 * 1024,
		InitialWindowSize:     1024 * 1024,
		KeepAliveTime:         30 * time.Second,
		KeepAliveTimeout:      10 * time.Second,
		MaxRetryAttempts:      3,
		RetryTimeout:          5 * time.Second,
		TLS:                   false,
	}

	return grpcConnector.New(grpcConfig, logger)
}
