package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	
	"gateway/internal/core"
	"gopkg.in/yaml.v3"
)

func TestConfig_LoadFromYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		check   func(t *testing.T, cfg *Config)
	}{
		{
			name: "minimal valid config",
			yaml: `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
      maxIdleConnsPerHost: 10
  registry:
    type: static
    static:
      services:
        - name: test-service
          instances:
            - id: instance-1
              address: "127.0.0.1"
              port: 8081
              health: healthy
  router:
    rules:
      - id: rule-1
        path: /api/*
        serviceName: test-service
        loadBalance: round_robin
`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Gateway.Frontend.HTTP.Port != 8080 {
					t.Errorf("Expected port 8080, got %d", cfg.Gateway.Frontend.HTTP.Port)
				}
				if len(cfg.Gateway.Registry.Static.Services) != 1 {
					t.Errorf("Expected 1 service, got %d", len(cfg.Gateway.Registry.Static.Services))
				}
				if len(cfg.Gateway.Router.Rules) != 1 {
					t.Errorf("Expected 1 route rule, got %d", len(cfg.Gateway.Router.Rules))
				}
			},
		},
		{
			name: "config with TLS",
			yaml: `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8443
      readTimeout: 30
      writeTimeout: 30
      tls:
        enabled: true
        certFile: "/path/to/cert.pem"
        keyFile: "/path/to/key.pem"
        minVersion: "1.2"
        maxVersion: "1.3"
  backend:
    http:
      maxIdleConns: 100
      tls:
        enabled: true
        insecureSkipVerify: false
        serverName: "backend.example.com"
  registry:
    type: static
    static:
      services: []
  router:
    rules: []
`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Gateway.Frontend.HTTP.TLS.Enabled {
					t.Error("Expected TLS to be enabled")
				}
				if cfg.Gateway.Frontend.HTTP.TLS.CertFile != "/path/to/cert.pem" {
					t.Errorf("Expected cert file /path/to/cert.pem, got %s", cfg.Gateway.Frontend.HTTP.TLS.CertFile)
				}
				if !cfg.Gateway.Backend.HTTP.TLS.Enabled {
					t.Error("Expected backend TLS to be enabled")
				}
			},
		},
		{
			name: "config with authentication",
			yaml: `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
  registry:
    type: static
    static:
      services: []
  router:
    rules: []
  auth:
    required: true
    providers: ["jwt", "apikey"]
    skipPaths: ["/health", "/metrics"]
    requiredScopes: ["read"]
    jwt:
      enabled: true
      issuer: "https://auth.example.com"
      audience: ["api.example.com"]
      signingMethod: "RS256"
      publicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA..."
      scopeClaim: "scope"
      subjectClaim: "sub"
    apikey:
      enabled: true
      hashKeys: true
      defaultScopes: ["basic"]
      keys:
        key1:
          key: "hashed-key-value"
          subject: "service1"
          type: "service"
          scopes: ["admin"]
`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Gateway.Auth.Required {
					t.Error("Expected auth to be required")
				}
				if len(cfg.Gateway.Auth.Providers) != 2 {
					t.Errorf("Expected 2 providers, got %d", len(cfg.Gateway.Auth.Providers))
				}
				if !cfg.Gateway.Auth.JWT.Enabled {
					t.Error("Expected JWT to be enabled")
				}
				if cfg.Gateway.Auth.JWT.Issuer != "https://auth.example.com" {
					t.Errorf("Expected issuer https://auth.example.com, got %s", cfg.Gateway.Auth.JWT.Issuer)
				}
				if !cfg.Gateway.Auth.APIKey.Enabled {
					t.Error("Expected API Key to be enabled")
				}
			},
		},
		{
			name: "config with WebSocket",
			yaml: `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
      readTimeout: 30
      writeTimeout: 30
    websocket:
      host: "0.0.0.0"
      port: 8081
      readTimeout: 60
      writeTimeout: 60
      handshakeTimeout: 10
      maxMessageSize: 1048576
      checkOrigin: true
      allowedOrigins: ["https://example.com"]
  backend:
    http:
      maxIdleConns: 100
    websocket:
      handshakeTimeout: 10
      maxConnections: 1000
      pingInterval: 30
      pongTimeout: 60
  registry:
    type: static
    static:
      services: []
  router:
    rules: []
`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Gateway.Frontend.WebSocket == nil {
					t.Fatal("Expected WebSocket config")
				}
				if cfg.Gateway.Frontend.WebSocket.Port != 8081 {
					t.Errorf("Expected WebSocket port 8081, got %d", cfg.Gateway.Frontend.WebSocket.Port)
				}
				if cfg.Gateway.Backend.WebSocket == nil {
					t.Fatal("Expected backend WebSocket config")
				}
				if cfg.Gateway.Backend.WebSocket.MaxConnections != 1000 {
					t.Errorf("Expected max connections 1000, got %d", cfg.Gateway.Backend.WebSocket.MaxConnections)
				}
			},
		},
		{
			name: "config with Docker registry",
			yaml: `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
  registry:
    type: docker
    docker:
      host: "unix:///var/run/docker.sock"
      version: "1.43"
      labelPrefix: "gateway"
      network: "gateway-network"
      refreshInterval: 30
  router:
    rules: []
`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Gateway.Registry.Type != "docker" {
					t.Errorf("Expected registry type docker, got %s", cfg.Gateway.Registry.Type)
				}
				if cfg.Gateway.Registry.Docker == nil {
					t.Fatal("Expected Docker registry config")
				}
				if cfg.Gateway.Registry.Docker.Host != "unix:///var/run/docker.sock" {
					t.Errorf("Expected Docker host unix:///var/run/docker.sock, got %s", cfg.Gateway.Registry.Docker.Host)
				}
			},
		},
		{
			name: "config with session affinity",
			yaml: `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
  registry:
    type: static
    static:
      services:
        - name: test-service
          instances:
            - id: instance-1
              address: "127.0.0.1"
              port: 8081
              health: healthy
  router:
    rules:
      - id: rule-1
        path: /api/*
        serviceName: test-service
        loadBalance: sticky
        sessionAffinity:
          enabled: true
          ttl: 3600
          source: cookie
          cookieName: "SESSIONID"
`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.Gateway.Router.Rules) != 1 {
					t.Fatal("Expected 1 route rule")
				}
				rule := cfg.Gateway.Router.Rules[0]
				if rule.SessionAffinityConfig == nil {
					t.Fatal("Expected session affinity config")
				}
				if !rule.SessionAffinityConfig.Enabled {
					t.Error("Expected session affinity to be enabled")
				}
				if rule.SessionAffinityConfig.CookieName != "SESSIONID" {
					t.Errorf("Expected cookie name SESSIONID, got %s", rule.SessionAffinityConfig.CookieName)
				}
			},
		},
		{
			name: "config with rate limiting",
			yaml: `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
  registry:
    type: static
    static:
      services: []
  router:
    rules:
      - id: rule-1
        path: /api/*
        serviceName: test-service
        loadBalance: round_robin
        rateLimit: 100
        rateLimitBurst: 10
        rateLimitExpiration: 60
`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.Gateway.Router.Rules) != 1 {
					t.Fatal("Expected 1 route rule")
				}
				rule := cfg.Gateway.Router.Rules[0]
				if rule.RateLimit != 100 {
					t.Errorf("Expected rate limit 100, got %d", rule.RateLimit)
				}
				if rule.RateLimitBurst != 10 {
					t.Errorf("Expected rate limit burst 10, got %d", rule.RateLimitBurst)
				}
			},
		},
		{
			name: "invalid YAML",
			yaml: `
gateway:
  frontend:
    http:
      port: "should be int"
`,
			wantErr: true,
		},
		{
			name: "empty config",
			yaml: ``,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				// Should have zero values
				if cfg.Gateway.Frontend.HTTP.Port != 0 {
					t.Errorf("Expected port 0, got %d", cfg.Gateway.Frontend.HTTP.Port)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			err := yaml.Unmarshal([]byte(tt.yaml), &cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.check != nil {
				tt.check(t, &cfg)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create test config file
	configContent := `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
  registry:
    type: static
    static:
      services:
        - name: test-service
          instances:
            - id: instance-1
              address: "127.0.0.1"
              port: 8081
              health: healthy
  router:
    rules:
      - id: rule-1
        path: /api/*
        serviceName: test-service
        loadBalance: round_robin
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test loading
	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify
	if cfg.Gateway.Frontend.HTTP.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", cfg.Gateway.Frontend.HTTP.Port)
	}

	// Test loading non-existent file
	_, err = LoadFromFile(filepath.Join(tmpDir, "nonexistent.yaml"))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestInstance_ToServiceInstance(t *testing.T) {
	tests := []struct {
		name     string
		instance Instance
		svcName  string
		want     core.ServiceInstance
	}{
		{
			name: "healthy instance",
			instance: Instance{
				ID:      "instance-1",
				Address: "192.168.1.100",
				Port:    8080,
				Health:  "healthy",
				Tags:    []string{"tag1", "tag2"},
			},
			svcName: "test-service",
			want: core.ServiceInstance{
				ID:       "instance-1",
				Name:     "test-service",
				Address:  "192.168.1.100",
				Port:     8080,
				Healthy:  true,
				Metadata: nil,
			},
		},
		{
			name: "unhealthy instance",
			instance: Instance{
				ID:      "instance-2",
				Address: "192.168.1.101",
				Port:    8081,
				Health:  "unhealthy",
			},
			svcName: "test-service",
			want: core.ServiceInstance{
				ID:       "instance-2",
				Name:     "test-service",
				Address:  "192.168.1.101",
				Port:     8081,
				Healthy:  false,
				Metadata: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.instance.ToServiceInstance(tt.svcName)
			
			if got.ID != tt.want.ID {
				t.Errorf("ID: got %s, want %s", got.ID, tt.want.ID)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name: got %s, want %s", got.Name, tt.want.Name)
			}
			if got.Address != tt.want.Address {
				t.Errorf("Address: got %s, want %s", got.Address, tt.want.Address)
			}
			if got.Port != tt.want.Port {
				t.Errorf("Port: got %d, want %d", got.Port, tt.want.Port)
			}
			if got.Healthy != tt.want.Healthy {
				t.Errorf("Healthy: got %v, want %v", got.Healthy, tt.want.Healthy)
			}
		})
	}
}

func TestRouteRule_ToRouteRule(t *testing.T) {
	tests := []struct {
		name string
		rule RouteRule
		want core.RouteRule
	}{
		{
			name: "basic rule",
			rule: RouteRule{
				ID:          "rule-1",
				Path:        "/api/*",
				ServiceName: "test-service",
				LoadBalance: "round_robin",
				Timeout:     30,
			},
			want: core.RouteRule{
				ID:          "rule-1",
				Path:        "/api/*",
				ServiceName: "test-service",
				LoadBalance: core.LoadBalanceRoundRobin,
				Timeout:     30 * time.Second,
			},
		},
		{
			name: "rule with session affinity",
			rule: RouteRule{
				ID:          "rule-2",
				Path:        "/app/*",
				ServiceName: "app-service",
				LoadBalance: "sticky",
				SessionAffinityConfig: &SessionAffinityConfig{
					Enabled:    true,
					TTL:        3600,
					Source:     "cookie",
					CookieName: "SESSIONID",
				},
			},
			want: core.RouteRule{
				ID:          "rule-2",
				Path:        "/app/*",
				ServiceName: "app-service",
				LoadBalance: "sticky",
				SessionAffinity: &core.SessionAffinityConfig{
					Enabled:    true,
					TTL:        3600 * time.Second,
					Source:     core.SessionSourceCookie,
					CookieName: "SESSIONID",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.ToRouteRule()
			
			if got.ID != tt.want.ID {
				t.Errorf("ID: got %s, want %s", got.ID, tt.want.ID)
			}
			if got.Path != tt.want.Path {
				t.Errorf("Path: got %s, want %s", got.Path, tt.want.Path)
			}
			if got.ServiceName != tt.want.ServiceName {
				t.Errorf("ServiceName: got %s, want %s", got.ServiceName, tt.want.ServiceName)
			}
			if got.LoadBalance != tt.want.LoadBalance {
				t.Errorf("LoadBalance: got %s, want %s", got.LoadBalance, tt.want.LoadBalance)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("Timeout: got %v, want %v", got.Timeout, tt.want.Timeout)
			}
			
			// Check session affinity
			if tt.want.SessionAffinity != nil {
				if got.SessionAffinity == nil {
					t.Error("Expected session affinity config but got nil")
				} else {
					if got.SessionAffinity.Enabled != tt.want.SessionAffinity.Enabled {
						t.Errorf("SessionAffinity.Enabled: got %v, want %v", got.SessionAffinity.Enabled, tt.want.SessionAffinity.Enabled)
					}
					if got.SessionAffinity.CookieName != tt.want.SessionAffinity.CookieName {
						t.Errorf("SessionAffinity.CookieName: got %s, want %s", got.SessionAffinity.CookieName, tt.want.SessionAffinity.CookieName)
					}
				}
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	// Test that zero values work correctly
	cfg := &Config{}
	
	// Frontend HTTP should have zero values
	if cfg.Gateway.Frontend.HTTP.Port != 0 {
		t.Errorf("Expected port 0, got %d", cfg.Gateway.Frontend.HTTP.Port)
	}
	
	// Optional configs should be nil
	if cfg.Gateway.Frontend.WebSocket != nil {
		t.Error("Expected WebSocket to be nil")
	}
	if cfg.Gateway.Auth != nil {
		t.Error("Expected Auth to be nil")
	}
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}