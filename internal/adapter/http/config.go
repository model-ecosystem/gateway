package http

import (
	"crypto/tls"
	"time"
)

// Config holds HTTP adapter configuration
type Config struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxRequestSize  int64 // Maximum request body size in bytes (0 = no limit)
	TLS             *TLSConfig
	TLSConfig       *tls.Config // Full TLS configuration
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled    bool
	CertFile   string
	KeyFile    string
	MinVersion string
}