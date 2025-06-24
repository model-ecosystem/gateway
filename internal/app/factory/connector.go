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
	httpConnector "gateway/internal/connector/http"
	sseConnector "gateway/internal/connector/sse"
	wsConnector "gateway/internal/connector/websocket"
	"gateway/pkg/errors"
	tlsutil "gateway/pkg/tls"
)

// ConnectorFactory creates backend connector instances
type ConnectorFactory struct {
	BaseComponentFactory
}

// NewConnectorFactory creates a new connector factory
func NewConnectorFactory(logger *slog.Logger) *ConnectorFactory {
	return &ConnectorFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// CreateHTTPClient creates an optimized HTTP client from configuration
func (f *ConnectorFactory) CreateHTTPClient(cfg config.HTTPBackend) (*http.Client, error) {
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
		tlsConfig, err := f.createBackendTLSConfig(cfg.TLS)
		if err != nil {
			// Fail closed - do not proceed with an insecure configuration
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to create secure backend HTTP client").WithCause(err)
		}
		transport.TLSClientConfig = tlsConfig
	}

	return &http.Client{
		Transport: transport,
		// No timeout here, we use context timeout per request
	}, nil
}

// CreateHTTPConnector creates an HTTP backend connector
func (f *ConnectorFactory) CreateHTTPConnector(client *http.Client, cfg config.HTTPBackend) connector.Connector {
	// Use response header timeout as default timeout, fallback to 30s
	defaultTimeout := time.Duration(cfg.ResponseHeaderTimeout) * time.Second
	if defaultTimeout == 0 {
		defaultTimeout = 30 * time.Second
	}
	return httpConnector.NewHTTPConnector(client, defaultTimeout)
}

// CreateSSEConnector creates an SSE backend connector
func (f *ConnectorFactory) CreateSSEConnector(cfg *config.SSEBackend, client *http.Client) *sseConnector.Connector {
	sseConfig := &sseConnector.Config{
		DialTimeout:      10 * time.Second,
		ResponseTimeout:  30 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
	}

	if cfg != nil {
		if cfg.ConnectTimeout > 0 {
			sseConfig.DialTimeout = time.Duration(cfg.ConnectTimeout) * time.Second
		}
	}

	return sseConnector.NewConnector(sseConfig, client, f.logger)
}

// CreateWebSocketConnector creates a WebSocket backend connector
func (f *ConnectorFactory) CreateWebSocketConnector(cfg *config.WebSocketBackend) *wsConnector.Connector {
	wsConfig := &wsConnector.Config{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}

	if cfg != nil {
		if cfg.HandshakeTimeout > 0 {
			wsConfig.HandshakeTimeout = time.Duration(cfg.HandshakeTimeout) * time.Second
		}
		if cfg.ReadBufferSize > 0 {
			wsConfig.ReadBufferSize = cfg.ReadBufferSize
		}
		if cfg.WriteBufferSize > 0 {
			wsConfig.WriteBufferSize = cfg.WriteBufferSize
		}
	}

	return wsConnector.NewConnector(wsConfig, f.logger)
}

// CreateGRPCConnector creates a gRPC backend connector
func (f *ConnectorFactory) CreateGRPCConnector() *grpcConnector.Connector {
	grpcConfig := &grpcConnector.Config{
		MaxConcurrentStreams:  100,
		InitialConnWindowSize: 1024 * 1024,
		InitialWindowSize:     1024 * 1024,
		KeepAliveTime:         30,
		KeepAliveTimeout:      10,
		MaxRetryAttempts:      3,
		RetryTimeout:          5,
		TLS:                   false,
	}

	return grpcConnector.New(grpcConfig, f.logger)
}

// createBackendTLSConfig creates TLS configuration for backend connections
func (f *ConnectorFactory) createBackendTLSConfig(cfg *config.BackendTLS) (*tls.Config, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	// Create base TLS config with security defaults
	tlsConfig := f.createTLSConfigBase(cfg.MinVersion, cfg.MaxVersion)
	
	// Apply backend-specific settings
	tlsConfig.InsecureSkipVerify = cfg.InsecureSkipVerify
	tlsConfig.ServerName = cfg.ServerName

	// Load client certificate for mTLS
	if cfg.ClientCertFile != "" && cfg.ClientKeyFile != "" {
		cert, err := f.loadTLSCertificate(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to load client certificate for mTLS").WithCause(err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate for server verification
	if cfg.RootCAFile != "" {
		// Note: For now, just set InsecureSkipVerify based on config
		// Full CA loading implementation would require additional setup
		tlsConfig.InsecureSkipVerify = cfg.InsecureSkipVerify
	}

	// Handle renegotiation settings
	if cfg.Renegotiation {
		// Warning: Client-initiated renegotiation is a security risk
		tlsConfig.Renegotiation = tls.RenegotiateFreelyAsClient
	}

	return tlsConfig, nil
}

// createTLSConfigBase creates base TLS configuration with common settings
func (f *ConnectorFactory) createTLSConfigBase(minVersion, maxVersion string) *tls.Config {
	tlsConfig := &tls.Config{
		// Security best practices
		MinVersion:               tls.VersionTLS12, // Default to TLS 1.2 minimum
		PreferServerCipherSuites: false,            // Deprecated since Go 1.18, kept for clarity
		Renegotiation:           tls.RenegotiateNever,
	}

	// Set minimum version
	if minVersion != "" {
		tlsConfig.MinVersion = tlsutil.ParseTLSVersion(minVersion)
	}

	// Set maximum version
	if maxVersion != "" {
		tlsConfig.MaxVersion = tlsutil.ParseTLSVersion(maxVersion)
	}

	return tlsConfig
}

// loadTLSCertificate loads a TLS certificate from files
func (f *ConnectorFactory) loadTLSCertificate(certFile, keyFile string) (tls.Certificate, error) {
	if certFile == "" || keyFile == "" {
		return tls.Certificate{}, errors.NewError(errors.ErrorTypeInternal, "certificate and key files must be provided")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, errors.NewError(errors.ErrorTypeInternal, "failed to load TLS certificate").WithCause(err)
	}

	return cert, nil
}