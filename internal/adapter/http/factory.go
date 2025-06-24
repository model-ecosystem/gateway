package http

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
	tlsutil "gateway/pkg/tls"
)

// ComponentName is the name used to register this component
const ComponentName = "http-adapter"

// Component implements factory.Component for HTTP adapter
type Component struct {
	config  Config
	adapter *Adapter
	handler core.Handler
	logger  *slog.Logger
}

// NewComponent creates a new HTTP adapter component
func NewComponent(handler core.Handler, logger *slog.Logger) factory.Component {
	return &Component{
		handler: handler,
		logger:  logger,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse the HTTP configuration
	var httpConfig config.HTTP
	if err := parser(&httpConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Convert to internal config
	c.config = Config{
		Host:           httpConfig.Host,
		Port:           httpConfig.Port,
		ReadTimeout:    time.Duration(httpConfig.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(httpConfig.WriteTimeout) * time.Second,
		MaxRequestSize: httpConfig.MaxRequestSize,
	}
	
	// Set defaults
	if c.config.Host == "" {
		c.config.Host = "0.0.0.0"
	}
	if c.config.Port == 0 {
		c.config.Port = 8080
	}
	if c.config.ReadTimeout == 0 {
		c.config.ReadTimeout = 30 * time.Second
	}
	if c.config.WriteTimeout == 0 {
		c.config.WriteTimeout = 30 * time.Second
	}
	if c.config.MaxRequestSize == 0 {
		c.config.MaxRequestSize = 10 * 1024 * 1024 // 10MB
	}
	
	// Add TLS config if enabled
	if httpConfig.TLS != nil && httpConfig.TLS.Enabled {
		tlsConfig, err := c.createTLSConfig(httpConfig.TLS)
		if err != nil {
			return fmt.Errorf("create TLS config: %w", err)
		}
		c.config.TLSConfig = tlsConfig
		c.config.TLS = &TLSConfig{
			Enabled:    true,
			CertFile:   httpConfig.TLS.CertFile,
			KeyFile:    httpConfig.TLS.KeyFile,
			MinVersion: httpConfig.TLS.MinVersion,
		}
	}
	
	// Create adapter
	c.adapter = New(c.config, c.handler)
	if c.logger != nil {
		c.adapter.logger = c.logger
	}
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.adapter == nil {
		return fmt.Errorf("HTTP adapter not initialized")
	}
	
	// Validate configuration
	if c.config.Port <= 0 || c.config.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", c.config.Port)
	}
	
	return nil
}

// Build returns the adapter
func (c *Component) Build() *Adapter {
	if c.adapter == nil {
		panic("Component not initialized")
	}
	return c.adapter
}

// createTLSConfig creates TLS configuration for frontend connections
func (c *Component) createTLSConfig(cfg *config.TLS) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tlsutil.ParseTLSVersion(cfg.MinVersion),
		MaxVersion: tls.VersionTLS13,
	}
	
	// Load certificate and key
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	
	// Configure cipher suites if specified
	if len(cfg.CipherSuites) > 0 {
		tlsConfig.CipherSuites = make([]uint16, 0, len(cfg.CipherSuites))
		for _, suite := range cfg.CipherSuites {
			// Cipher suites are already numeric IDs in the config
			tlsConfig.CipherSuites = append(tlsConfig.CipherSuites, uint16(suite))
		}
	}
	
	// Set other TLS options
	tlsConfig.PreferServerCipherSuites = cfg.PreferServerCipher
	
	return tlsConfig, nil
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)