package provider

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"gateway/internal/config"
	"gateway/internal/connector"
	httpConnector "gateway/internal/connector/http"
	grpcConnector "gateway/internal/connector/grpc"
	sseConnector "gateway/internal/connector/sse"
	wsConnector "gateway/internal/connector/websocket"
	"gateway/internal/middleware/auth/apikey"
	"gateway/internal/middleware/auth/jwt"
	"gateway/pkg/errors"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "provider"

// Component implements factory.Component for dependency provider
type Component struct {
	mu sync.RWMutex
	
	// Core components
	httpClient     *http.Client
	httpConnectorInst  connector.Connector
	sseConnectorInst   *sseConnector.Connector
	wsConnectorInst    *wsConnector.Connector
	grpcConnectorInst  *grpcConnector.Connector
	
	// Auth providers
	jwtProvider    *jwt.Provider
	apiKeyProvider *apikey.Provider
	
	// Component references
	httpConnectorComp  *httpConnector.Component
	sseConnectorComp   *sseConnector.Component
	wsConnectorComp    *wsConnector.Component
	grpcConnectorComp  *grpcConnector.Component
	
	// Configuration
	config *config.Config
	logger *slog.Logger
}

// NewComponent creates a new provider component
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
	// Parse the full configuration
	var cfg config.Config
	if err := parser(&cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	
	c.config = &cfg
	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.config == nil {
		return fmt.Errorf("configuration not initialized")
	}
	return nil
}

// SetConnectorComponents sets the connector components for lazy initialization
func (c *Component) SetConnectorComponents(
	httpComp *httpConnector.Component,
	sseComp *sseConnector.Component,
	wsComp *wsConnector.Component,
	grpcComp *grpcConnector.Component,
) {
	c.httpConnectorComp = httpComp
	c.sseConnectorComp = sseComp
	c.wsConnectorComp = wsComp
	c.grpcConnectorComp = grpcComp
}

// GetHTTPClient returns the HTTP client, creating it if necessary
func (c *Component) GetHTTPClient() (*http.Client, error) {
	c.mu.RLock()
	if c.httpClient != nil {
		c.mu.RUnlock()
		return c.httpClient, nil
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Double-check after acquiring write lock
	if c.httpClient != nil {
		return c.httpClient, nil
	}
	
	// For now, create HTTP client directly
	// TODO: Refactor HTTP connector to expose the client
	httpConfig := c.config.Gateway.Backend.HTTP
	httpClient, err := c.createHTTPClient(httpConfig)
	if err != nil {
		return nil, err
	}
	
	c.httpClient = httpClient
	return c.httpClient, nil
}

// GetHTTPConnector returns the HTTP connector, creating it if necessary
func (c *Component) GetHTTPConnector() (connector.Connector, error) {
	c.mu.RLock()
	if c.httpConnectorInst != nil {
		c.mu.RUnlock()
		return c.httpConnectorInst, nil
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Double-check after acquiring write lock
	if c.httpConnectorInst != nil {
		return c.httpConnectorInst, nil
	}
	
	// Get connector from component
	if c.httpConnectorComp != nil {
		c.httpConnectorInst = c.httpConnectorComp.Build()
		return c.httpConnectorInst, nil
	}
	
	return nil, fmt.Errorf("HTTP connector component not set")
}

// GetSSEConnector returns the SSE connector, creating it if necessary
func (c *Component) GetSSEConnector() (*sseConnector.Connector, error) {
	c.mu.RLock()
	if c.sseConnectorInst != nil {
		c.mu.RUnlock()
		return c.sseConnectorInst, nil
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Double-check after acquiring write lock
	if c.sseConnectorInst != nil {
		return c.sseConnectorInst, nil
	}
	
	// Get connector from component
	if c.sseConnectorComp != nil {
		c.sseConnectorInst = c.sseConnectorComp.Build()
		return c.sseConnectorInst, nil
	}
	
	return nil, fmt.Errorf("SSE connector component not set")
}

// GetWebSocketConnector returns the WebSocket connector, creating it if necessary
func (c *Component) GetWebSocketConnector() *wsConnector.Connector {
	c.mu.RLock()
	if c.wsConnectorInst != nil {
		c.mu.RUnlock()
		return c.wsConnectorInst
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Double-check after acquiring write lock
	if c.wsConnectorInst != nil {
		return c.wsConnectorInst
	}
	
	// Get connector from component
	if c.wsConnectorComp != nil {
		c.wsConnectorInst = c.wsConnectorComp.Build()
		return c.wsConnectorInst
	}
	
	return nil
}

// GetGRPCConnector returns the gRPC connector, creating it if necessary
func (c *Component) GetGRPCConnector() *grpcConnector.Connector {
	c.mu.RLock()
	if c.grpcConnectorInst != nil {
		c.mu.RUnlock()
		return c.grpcConnectorInst
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Double-check after acquiring write lock
	if c.grpcConnectorInst != nil {
		return c.grpcConnectorInst
	}
	
	// Get connector from component
	if c.grpcConnectorComp != nil {
		// gRPC connector's Build() returns the concrete type
		c.grpcConnectorInst = c.grpcConnectorComp.GetConnector()
		return c.grpcConnectorInst
	}
	
	return nil
}

// GetJWTProvider returns the JWT provider, creating it if necessary
func (c *Component) GetJWTProvider() (*jwt.Provider, error) {
	c.mu.RLock()
	if c.jwtProvider != nil {
		c.mu.RUnlock()
		return c.jwtProvider, nil
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Double-check after acquiring write lock
	if c.jwtProvider != nil {
		return c.jwtProvider, nil
	}
	
	// Check if JWT is configured
	if c.config.Gateway.Auth == nil || c.config.Gateway.Auth.JWT == nil || !c.config.Gateway.Auth.JWT.Enabled {
		return nil, errors.NewError(errors.ErrorTypeInternal, "JWT provider requested but JWT auth is not enabled")
	}
	
	provider, err := c.createJWTProvider(c.config.Gateway.Auth.JWT)
	if err != nil {
		return nil, err
	}
	
	c.jwtProvider = provider
	return provider, nil
}

// GetAPIKeyProvider returns the API key provider, creating it if necessary
func (c *Component) GetAPIKeyProvider() (*apikey.Provider, error) {
	c.mu.RLock()
	if c.apiKeyProvider != nil {
		c.mu.RUnlock()
		return c.apiKeyProvider, nil
	}
	c.mu.RUnlock()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Double-check after acquiring write lock
	if c.apiKeyProvider != nil {
		return c.apiKeyProvider, nil
	}
	
	// Check if API key is configured
	if c.config.Gateway.Auth == nil || c.config.Gateway.Auth.APIKey == nil || !c.config.Gateway.Auth.APIKey.Enabled {
		return nil, errors.NewError(errors.ErrorTypeInternal, "API key provider requested but API key auth is not enabled")
	}
	
	provider, err := c.createAPIKeyProvider(c.config.Gateway.Auth.APIKey)
	if err != nil {
		return nil, err
	}
	
	c.apiKeyProvider = provider
	return provider, nil
}

// createJWTProvider creates a JWT provider from configuration
func (c *Component) createJWTProvider(cfg *config.JWTConfig) (*jwt.Provider, error) {
	jwtConfig := &jwt.Config{
		Issuer:            cfg.Issuer,
		Audience:          cfg.Audience,
		SigningMethod:     cfg.SigningMethod,
		PublicKey:         cfg.PublicKey,
		Secret:            cfg.Secret,
		JWKSEndpoint:      cfg.JWKSEndpoint,
		JWKSCacheDuration: time.Duration(cfg.JWKSCacheDuration) * time.Second,
		ClaimsMapping:     cfg.ClaimsMapping,
		ScopeClaim:        cfg.ScopeClaim,
		SubjectClaim:      cfg.SubjectClaim,
	}

	// Set defaults
	if jwtConfig.JWKSCacheDuration == 0 {
		jwtConfig.JWKSCacheDuration = 1 * time.Hour
	}

	return jwt.NewProvider(jwtConfig, c.logger)
}

// createAPIKeyProvider creates an API key provider from configuration
func (c *Component) createAPIKeyProvider(cfg *config.APIKeyConfig) (*apikey.Provider, error) {
	// Convert config keys
	keys := make(map[string]*apikey.KeyConfig)
	for id, details := range cfg.Keys {
		keyConfig := &apikey.KeyConfig{
			Key:      details.Key,
			Subject:  details.Subject,
			Type:     details.Type,
			Scopes:   details.Scopes,
			Metadata: details.Metadata,
			Disabled: details.Disabled,
		}

		// Parse expiration time if provided
		if details.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, details.ExpiresAt)
			if err != nil {
				return nil, errors.NewError(errors.ErrorTypeInternal, "failed to parse API key expiration").WithCause(err).WithDetail("keyId", id)
			}
			keyConfig.ExpiresAt = &t
		}

		keys[id] = keyConfig
	}

	apiKeyConfig := &apikey.Config{
		Keys:          keys,
		HashKeys:      cfg.HashKeys,
		DefaultScopes: cfg.DefaultScopes,
	}

	return apikey.NewProvider(apiKeyConfig, c.logger)
}

// Close releases all resources held by the provider
func (c *Component) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Close connectors that support closing
	// Most connectors don't need explicit closing, but we check just in case
	
	// Note: HTTP client transport doesn't need explicit closing
	// as it manages connections automatically
	
	return nil
}

// createHTTPClient creates an HTTP client from configuration
func (c *Component) createHTTPClient(cfg config.HTTPBackend) (*http.Client, error) {
	// Create dialer with keep-alive settings
	dialer := &net.Dialer{
		Timeout: time.Duration(cfg.DialTimeout) * time.Second,
	}

	if cfg.KeepAlive {
		dialer.KeepAlive = time.Duration(cfg.KeepAliveTimeout) * time.Second
	} else {
		dialer.KeepAlive = -1 // Disable keep-alive
	}

	// Create transport with connection pooling
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(cfg.IdleConnTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(cfg.ResponseHeaderTimeout) * time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    false,
	}

	// TODO: Add TLS configuration if needed

	return &http.Client{
		Transport: transport,
	}, nil
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)