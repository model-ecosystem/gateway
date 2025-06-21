package websocket

import "time"

// Config holds WebSocket connector configuration
type Config struct {
	// Connection settings
	HandshakeTimeout time.Duration `yaml:"handshakeTimeout"`
	ReadTimeout      time.Duration `yaml:"readTimeout"`
	WriteTimeout     time.Duration `yaml:"writeTimeout"`

	// Buffer settings
	ReadBufferSize  int `yaml:"readBufferSize"`
	WriteBufferSize int `yaml:"writeBufferSize"`

	// Message settings
	MaxMessageSize int64 `yaml:"maxMessageSize"`

	// Connection pool settings
	MaxConnections        int           `yaml:"maxConnections"`
	ConnectionTimeout     time.Duration `yaml:"connectionTimeout"`
	IdleConnectionTimeout time.Duration `yaml:"idleConnectionTimeout"`

	// Keepalive settings
	PingInterval time.Duration `yaml:"pingInterval"`
	PongTimeout  time.Duration `yaml:"pongTimeout"`
	CloseTimeout time.Duration `yaml:"closeTimeout"`

	// Compression
	EnableCompression bool `yaml:"enableCompression"`
	CompressionLevel  int  `yaml:"compressionLevel"`
}
