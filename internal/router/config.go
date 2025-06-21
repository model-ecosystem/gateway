package router

import "time"

// Config represents router configuration
type Config struct {
	Rules []RouteRule `yaml:"rules"`
}

// RouteRule represents a single routing rule
type RouteRule struct {
	ID                    string                 `yaml:"id"`
	Path                  string                 `yaml:"path"`
	ServiceName           string                 `yaml:"serviceName"`
	LoadBalance           string                 `yaml:"loadBalance"`
	Timeout               time.Duration          `yaml:"timeout"`
	SessionAffinityConfig *SessionAffinityConfig `yaml:"sessionAffinity"`
	// Authentication
	AuthRequired bool   `yaml:"authRequired"`
	AuthType     string `yaml:"authType"`
	// Rate limiting
	RateLimit           int           `yaml:"rateLimit"`
	RateLimitBurst      int           `yaml:"rateLimitBurst"`
	RateLimitExpiration time.Duration `yaml:"rateLimitExpiration"`
}

// SessionAffinityConfig represents session affinity configuration
type SessionAffinityConfig struct {
	Enabled    bool          `yaml:"enabled"`
	TTL        time.Duration `yaml:"ttl"`
	Source     string        `yaml:"source"`     // cookie, header, query
	CookieName string        `yaml:"cookieName"` // for cookie source
	HeaderName string        `yaml:"headerName"` // for header source
	QueryParam string        `yaml:"queryParam"` // for query source
}
