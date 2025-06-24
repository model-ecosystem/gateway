package sse

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
	
	"gateway/internal/config"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "sse-connector"

// Component implements factory.Component for SSE connector
type Component struct {
	config    *Config
	connector *Connector
	client    *http.Client
	logger    *slog.Logger
}

// NewComponent creates a new SSE connector component
func NewComponent(client *http.Client, logger *slog.Logger) factory.Component {
	return &Component{
		client: client,
		logger: logger,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse the SSE backend configuration
	var sseConfig config.SSEBackend
	if err := parser(&sseConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Convert to internal config
	c.config = &Config{
		DialTimeout:      time.Duration(sseConfig.ConnectTimeout) * time.Second,
		ResponseTimeout:  time.Duration(sseConfig.ReadTimeout) * time.Second,
		KeepaliveTimeout: 30 * time.Second, // Default keepalive
	}
	
	// Set defaults if not configured
	if c.config.DialTimeout == 0 {
		c.config.DialTimeout = 10 * time.Second
	}
	if c.config.ResponseTimeout == 0 {
		c.config.ResponseTimeout = 30 * time.Second
	}
	if c.config.KeepaliveTimeout == 0 {
		c.config.KeepaliveTimeout = 30 * time.Second
	}
	
	// Create connector
	c.connector = NewConnector(c.config, c.client, c.logger)
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.connector == nil {
		return fmt.Errorf("SSE connector not initialized")
	}
	
	// Validate configuration
	if c.config.DialTimeout <= 0 {
		return fmt.Errorf("invalid dial timeout")
	}
	if c.config.ResponseTimeout <= 0 {
		return fmt.Errorf("invalid response timeout")
	}
	
	return nil
}

// Build returns the SSE connector
func (c *Component) Build() *Connector {
	if c.connector == nil {
		panic("Component not initialized")
	}
	return c.connector
}

// GetConnector returns the SSE connector (for specific use cases)
func (c *Component) GetConnector() *Connector {
	return c.connector
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)