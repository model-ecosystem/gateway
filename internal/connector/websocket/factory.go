package websocket

import (
	"fmt"
	"log/slog"
	"time"
	
	"gateway/internal/config"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "websocket-connector"

// Component implements factory.Component for WebSocket connector
type Component struct {
	config    *Config
	connector *Connector
	logger    *slog.Logger
}

// NewComponent creates a new WebSocket connector component
func NewComponent(logger *slog.Logger) factory.Component {
	return &Component{
		logger: logger,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse the WebSocket backend configuration
	var wsConfig config.WebSocketBackend
	if err := parser(&wsConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Convert to internal config
	c.config = &Config{
		HandshakeTimeout:  time.Duration(wsConfig.HandshakeTimeout) * time.Second,
		ReadTimeout:       time.Duration(wsConfig.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(wsConfig.WriteTimeout) * time.Second,
		ReadBufferSize:    wsConfig.ReadBufferSize,
		WriteBufferSize:   wsConfig.WriteBufferSize,
		MaxMessageSize:    wsConfig.MaxMessageSize,
		MaxConnections:    wsConfig.MaxConnections,
		ConnectionTimeout: time.Duration(wsConfig.ConnectionTimeout) * time.Second,
		PingInterval:      time.Duration(wsConfig.PingInterval) * time.Second,
		PongTimeout:       time.Duration(wsConfig.PongTimeout) * time.Second,
		CloseTimeout:      time.Duration(wsConfig.CloseTimeout) * time.Second,
		EnableCompression: wsConfig.EnableCompression,
		CompressionLevel:  wsConfig.CompressionLevel,
	}
	
	// Set defaults if not configured
	if c.config.HandshakeTimeout == 0 {
		c.config.HandshakeTimeout = 10 * time.Second
	}
	if c.config.ReadBufferSize == 0 {
		c.config.ReadBufferSize = 4096
	}
	if c.config.WriteBufferSize == 0 {
		c.config.WriteBufferSize = 4096
	}
	if c.config.MaxMessageSize == 0 {
		c.config.MaxMessageSize = 1024 * 1024 // 1MB
	}
	if c.config.MaxConnections == 0 {
		c.config.MaxConnections = 10
	}
	
	// Create connector
	c.connector = NewConnector(c.config, c.logger)
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.connector == nil {
		return fmt.Errorf("WebSocket connector not initialized")
	}
	
	// Validate configuration
	if c.config.HandshakeTimeout <= 0 {
		return fmt.Errorf("invalid handshake timeout")
	}
	if c.config.ReadBufferSize <= 0 {
		return fmt.Errorf("invalid read buffer size")
	}
	if c.config.WriteBufferSize <= 0 {
		return fmt.Errorf("invalid write buffer size")
	}
	
	return nil
}

// Build returns the WebSocket connector
func (c *Component) Build() *Connector {
	if c.connector == nil {
		panic("Component not initialized")
	}
	return c.connector
}

// GetConnector returns the WebSocket connector (for specific use cases)
func (c *Component) GetConnector() *Connector {
	return c.connector
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)