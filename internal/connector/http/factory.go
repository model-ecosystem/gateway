package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
	
	"gateway/internal/config"
	"gateway/internal/connector"
	"gateway/pkg/errors"
	"gateway/pkg/factory"
	tlsutil "gateway/pkg/tls"
)

// ComponentName is the name used to register this component
const ComponentName = "http-connector"

// Component implements factory.Component for HTTP connector
type Component struct {
	config    config.HTTPBackend
	client    *http.Client
	connector *HTTPConnector
}

// NewComponent creates a new HTTP connector component
func NewComponent() factory.Component {
	return &Component{}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse the HTTP backend configuration
	if err := parser(&c.config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Create HTTP client
	client, err := c.createHTTPClient()
	if err != nil {
		return fmt.Errorf("create HTTP client: %w", err)
	}
	
	c.client = client
	
	// Create connector with default timeout
	// Use response header timeout as default timeout
	defaultTimeout := time.Duration(c.config.ResponseHeaderTimeout) * time.Second
	if defaultTimeout == 0 {
		defaultTimeout = 30 * time.Second
	}
	
	c.connector = NewHTTPConnector(client, defaultTimeout)
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.client == nil {
		return fmt.Errorf("HTTP client not initialized")
	}
	if c.connector == nil {
		return fmt.Errorf("HTTP connector not initialized")
	}
	
	// Validate configuration
	if c.config.MaxIdleConns <= 0 {
		c.config.MaxIdleConns = 100
	}
	if c.config.MaxIdleConnsPerHost <= 0 {
		c.config.MaxIdleConnsPerHost = 10
	}
	
	return nil
}

// Build returns the connector
func (c *Component) Build() connector.Connector {
	if c.connector == nil {
		panic("Component not initialized")
	}
	return c.connector
}

// GetClient returns the HTTP client (for backward compatibility)
func (c *Component) GetClient() *http.Client {
	return c.client
}

// createHTTPClient creates an optimized HTTP client from configuration
func (c *Component) createHTTPClient() (*http.Client, error) {
	// Create dialer with keep-alive settings
	dialer := &net.Dialer{
		Timeout: time.Duration(c.config.DialTimeout) * time.Second,
	}
	
	if c.config.KeepAlive {
		dialer.KeepAlive = time.Duration(c.config.KeepAliveTimeout) * time.Second
	} else {
		dialer.KeepAlive = -1 // Disable keep-alive
	}
	
	// Create transport with connection pooling
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          c.config.MaxIdleConns,
		MaxIdleConnsPerHost:   c.config.MaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(c.config.IdleConnTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(c.config.ResponseHeaderTimeout) * time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    false,
	}
	
	// Configure TLS if enabled
	if c.config.TLS != nil && c.config.TLS.Enabled {
		tlsConfig, err := c.createBackendTLSConfig(c.config.TLS)
		if err != nil {
			// Fail closed - do not proceed with an insecure configuration
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to create secure backend HTTP client").WithCause(err)
		}
		transport.TLSClientConfig = tlsConfig
	}
	
	return &http.Client{
		Transport: transport,
		// Don't follow redirects automatically
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

// createBackendTLSConfig creates TLS configuration for backend connections
func (c *Component) createBackendTLSConfig(cfg *config.BackendTLS) (*tls.Config, error) {
	// Create base TLS config with security defaults
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12, // Default to TLS 1.2 minimum
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	
	// Set minimum version if specified
	if cfg.MinVersion != "" {
		if v := tlsutil.ParseTLSVersion(cfg.MinVersion); v != 0 {
			tlsConfig.MinVersion = v
		}
	}
	
	// Set maximum version if specified
	if cfg.MaxVersion != "" {
		if v := tlsutil.ParseTLSVersion(cfg.MaxVersion); v != 0 {
			tlsConfig.MaxVersion = v
		}
	}
	
	// Set server name for SNI
	if cfg.ServerName != "" {
		tlsConfig.ServerName = cfg.ServerName
	}
	
	// Load CA certificate if provided
	if cfg.RootCAFile != "" {
		caCertPEM, err := os.ReadFile(cfg.RootCAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate: %w", err)
		}
		
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCertPEM); !ok {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}
	
	// Load client certificate for mutual TLS
	if cfg.ClientCertFile != "" && cfg.ClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	
	// Handle renegotiation settings
	if cfg.Renegotiation {
		// Warning: Client-initiated renegotiation is a security risk
		tlsConfig.Renegotiation = tls.RenegotiateFreelyAsClient
	}
	
	return tlsConfig, nil
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)