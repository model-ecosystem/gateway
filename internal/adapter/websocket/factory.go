package websocket

import (
	"fmt"
	"log/slog"
	"time"
	
	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "websocket-adapter"

// Component implements factory.Component for WebSocket adapter
type Component struct {
	config  *Config
	adapter *Adapter
	handler core.Handler
	logger  *slog.Logger
}

// NewComponent creates a new WebSocket adapter component
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
	// Parse the WebSocket configuration
	var wsConfig config.WebSocket
	if err := parser(&wsConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	// Skip if not enabled
	if !wsConfig.Enabled {
		return fmt.Errorf("websocket adapter is not enabled")
	}
	
	// Convert to internal config
	c.config = &Config{
		Host:              wsConfig.Host,
		Port:              wsConfig.Port,
		ReadTimeout:       time.Duration(wsConfig.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(wsConfig.WriteTimeout) * time.Second,
		HandshakeTimeout:  time.Duration(wsConfig.HandshakeTimeout) * time.Second,
		MaxMessageSize:    wsConfig.MaxMessageSize,
		ReadBufferSize:    wsConfig.ReadBufferSize,
		WriteBufferSize:   wsConfig.WriteBufferSize,
		CheckOrigin:       wsConfig.CheckOrigin,
		AllowedOrigins:    wsConfig.AllowedOrigins,
		EnableCompression: wsConfig.EnableCompression,
		CompressionLevel:  wsConfig.CompressionLevel,
		Subprotocols:      wsConfig.Subprotocols,
		WriteDeadline:     time.Duration(wsConfig.WriteDeadline) * time.Second,
		PongWait:          time.Duration(wsConfig.PongWait) * time.Second,
		PingPeriod:        time.Duration(wsConfig.PingPeriod) * time.Second,
		CloseGracePeriod:  time.Duration(wsConfig.CloseGracePeriod) * time.Second,
	}
	
	// Set defaults
	if c.config.ReadTimeout == 0 {
		c.config.ReadTimeout = 60 * time.Second
	}
	if c.config.WriteTimeout == 0 {
		c.config.WriteTimeout = 60 * time.Second
	}
	if c.config.HandshakeTimeout == 0 {
		c.config.HandshakeTimeout = 10 * time.Second
	}
	if c.config.MaxMessageSize == 0 {
		c.config.MaxMessageSize = 1024 * 1024 // 1MB
	}
	if c.config.ReadBufferSize == 0 {
		c.config.ReadBufferSize = 4096
	}
	if c.config.WriteBufferSize == 0 {
		c.config.WriteBufferSize = 4096
	}
	if c.config.PongWait == 0 {
		c.config.PongWait = 60 * time.Second
	}
	if c.config.PingPeriod == 0 {
		c.config.PingPeriod = (c.config.PongWait * 9) / 10
	}
	if c.config.CloseGracePeriod == 0 {
		c.config.CloseGracePeriod = 10 * time.Second
	}
	
	// Create adapter
	c.adapter = NewAdapter(c.config, c.handler, c.logger)
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.adapter == nil {
		return fmt.Errorf("WebSocket adapter not initialized")
	}
	
	// Validate configuration
	if c.config.ReadBufferSize <= 0 {
		return fmt.Errorf("invalid read buffer size")
	}
	if c.config.WriteBufferSize <= 0 {
		return fmt.Errorf("invalid write buffer size")
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

// GetConfig returns the configuration (for testing)
func (c *Component) GetConfig() *Config {
	return c.config
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)