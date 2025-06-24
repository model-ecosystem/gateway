package grpc

import (
	"fmt"
	"log/slog"
	
	"gateway/internal/connector"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "grpc-connector"

// Component implements factory.Component for gRPC connector
type Component struct {
	connector *Connector
	logger    *slog.Logger
}

// NewComponent creates a new gRPC connector component
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
	// gRPC connector doesn't have specific configuration
	// It's created with default settings
	
	// Create connector
	grpcConfig := &Config{
		MaxConcurrentStreams:  100,
		InitialConnWindowSize: 1024 * 1024,
		InitialWindowSize:     1024 * 1024,
		KeepAliveTime:         30,
		KeepAliveTimeout:      10,
		MaxRetryAttempts:      3,
		RetryTimeout:          5,
		TLS:                   false,
	}
	c.connector = New(grpcConfig, c.logger)
	
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.connector == nil {
		return fmt.Errorf("gRPC connector not initialized")
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

// GetConnector returns the gRPC connector (for specific use cases)
func (c *Component) GetConnector() *Connector {
	return c.connector
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)