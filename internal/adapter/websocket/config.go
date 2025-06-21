package websocket

import (
	"crypto/tls"
	"time"
)

// Config holds WebSocket adapter configuration
type Config struct {
	Host                 string        `yaml:"host"`
	Port                 int           `yaml:"port"`
	ReadTimeout          time.Duration `yaml:"readTimeout"`
	WriteTimeout         time.Duration `yaml:"writeTimeout"`
	HandshakeTimeout     time.Duration `yaml:"handshakeTimeout"`
	MaxMessageSize       int64         `yaml:"maxMessageSize"`
	ReadBufferSize       int           `yaml:"readBufferSize"`
	WriteBufferSize      int           `yaml:"writeBufferSize"`
	CheckOrigin          bool          `yaml:"checkOrigin"`
	AllowedOrigins       []string      `yaml:"allowedOrigins"`
	EnableCompression    bool          `yaml:"enableCompression"`
	CompressionLevel     int           `yaml:"compressionLevel"`
	Subprotocols         []string      `yaml:"subprotocols"`
	WriteDeadline        time.Duration `yaml:"writeDeadline"`
	PongWait             time.Duration `yaml:"pongWait"`
	PingPeriod           time.Duration `yaml:"pingPeriod"`
	CloseGracePeriod     time.Duration `yaml:"closeGracePeriod"`
	TLS                  *TLSConfig    `yaml:"tls"`
	TLSConfig            *tls.Config   // Full TLS configuration
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled    bool   `yaml:"enabled"`
	CertFile   string `yaml:"certFile"`
	KeyFile    string `yaml:"keyFile"`
	MinVersion string `yaml:"minVersion"`
}