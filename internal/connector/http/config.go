package http

import "time"

// Config holds HTTP connector configuration
type Config struct {
	// Connection pool settings
	MaxIdleConns        int           `yaml:"maxIdleConns"`
	MaxIdleConnsPerHost int           `yaml:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int           `yaml:"maxConnsPerHost"`
	IdleConnTimeout     time.Duration `yaml:"idleConnTimeout"`
	
	// Connection settings
	KeepAlive           bool          `yaml:"keepAlive"`
	KeepAliveTimeout    time.Duration `yaml:"keepAliveTimeout"`
	DisableCompression  bool          `yaml:"disableCompression"`
	DisableHTTP2        bool          `yaml:"disableHTTP2"`
	
	// Timeout settings
	DialTimeout           time.Duration `yaml:"dialTimeout"`
	ResponseHeaderTimeout time.Duration `yaml:"responseHeaderTimeout"`
	ExpectContinueTimeout time.Duration `yaml:"expectContinueTimeout"`
	TLSHandshakeTimeout   time.Duration `yaml:"tlsHandshakeTimeout"`
	
	// Retry settings
	MaxRetries     int           `yaml:"maxRetries"`
	RetryDelay     time.Duration `yaml:"retryDelay"`
	RetryCondition string        `yaml:"retryCondition"`
}