package config

import (
	"gateway/internal/core"
	"time"
)

// Config holds gateway configuration
type Config struct {
	Gateway Gateway `yaml:"gateway"`
}

// Gateway configuration
type Gateway struct {
	Frontend Frontend `yaml:"frontend"`
	Backend  Backend  `yaml:"backend"`
	Registry Registry `yaml:"registry"`
	Router   Router   `yaml:"router"`
}

// Frontend configuration
type Frontend struct {
	HTTP HTTP `yaml:"http"`
}

// HTTP configuration
type HTTP struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	ReadTimeout  int    `yaml:"readTimeout"`
	WriteTimeout int    `yaml:"writeTimeout"`
	TLS          *TLS   `yaml:"tls,omitempty"`
}

// TLS configuration
type TLS struct {
	Enabled    bool   `yaml:"enabled"`
	CertFile   string `yaml:"certFile"`
	KeyFile    string `yaml:"keyFile"`
	MinVersion string `yaml:"minVersion,omitempty"`
}

// Backend configuration
type Backend struct {
	HTTP HTTPBackend `yaml:"http"`
}

// HTTPBackend configuration for backend connections
type HTTPBackend struct {
	// Connection pool settings
	MaxIdleConns        int `yaml:"maxIdleConns"`
	MaxIdleConnsPerHost int `yaml:"maxIdleConnsPerHost"`
	IdleConnTimeout     int `yaml:"idleConnTimeout"`
	
	// Keep-alive settings
	KeepAlive        bool `yaml:"keepAlive"`
	KeepAliveTimeout int  `yaml:"keepAliveTimeout"`
	
	// Additional transport settings
	DialTimeout           int `yaml:"dialTimeout"`
	ResponseHeaderTimeout int `yaml:"responseHeaderTimeout"`
}

// Registry configuration
type Registry struct {
	Type   string         `yaml:"type"`
	Static *StaticRegistry `yaml:"static"`
}

// StaticRegistry configuration
type StaticRegistry struct {
	Services []Service `yaml:"services"`
}

// Service configuration
type Service struct {
	Name      string     `yaml:"name"`
	Instances []Instance `yaml:"instances"`
}

// Instance configuration
type Instance struct {
	ID       string         `yaml:"id"`
	Address  string         `yaml:"address"`
	Port     int            `yaml:"port"`
	Health   string         `yaml:"health"`
	Metadata map[string]any `yaml:"metadata"`
}

// Router configuration
type Router struct {
	Rules []RouteRule `yaml:"rules"`
}

// RouteRule configuration
type RouteRule struct {
	ID          string   `yaml:"id"`
	Path        string   `yaml:"path"`
	Methods     []string `yaml:"methods"`
	ServiceName string   `yaml:"serviceName"`
	LoadBalance string   `yaml:"loadBalance"`
	Timeout     int      `yaml:"timeout"`
}

// ToServiceInstance converts to core.ServiceInstance
func (i *Instance) ToServiceInstance(name string) core.ServiceInstance {
	return core.ServiceInstance{
		ID:       i.ID,
		Name:     name,
		Address:  i.Address,
		Port:     i.Port,
		Healthy:  i.Health == "healthy",
		Metadata: i.Metadata,
	}
}

// ToRouteRule converts to core.RouteRule
func (r *RouteRule) ToRouteRule() core.RouteRule {
	return core.RouteRule{
		ID:          r.ID,
		Path:        r.Path,
		Methods:     r.Methods,
		ServiceName: r.ServiceName,
		LoadBalance: core.LoadBalanceStrategy(r.LoadBalance),
		Timeout:     time.Duration(r.Timeout) * time.Second,
	}
}