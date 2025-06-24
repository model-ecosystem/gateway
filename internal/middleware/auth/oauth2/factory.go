package oauth2

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "oauth2-middleware"

// Component implements factory.Component for OAuth2 middleware
type Component struct {
	config     *config.OAuth2Config
	middleware *Middleware
	logger     *slog.Logger
}

// NewComponent creates a new OAuth2 middleware component
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
	// Parse the OAuth2 configuration
	var oauth2Config config.OAuth2Config
	if err := parser(&oauth2Config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	c.config = &oauth2Config

	// Only create middleware if enabled
	if c.config.Enabled {
		// Convert provider configs
		var providers []ProviderConfig
		for _, p := range c.config.Providers {
			provider := ProviderConfig{
				Name:             p.Name,
				ClientID:         p.ClientID,
				ClientSecret:     p.ClientSecret,
				AuthorizationURL: p.AuthorizationURL,
				TokenURL:         p.TokenURL,
				UserInfoURL:      p.UserInfoURL,
				JWKSEndpoint:     p.JWKSEndpoint,
				IssuerURL:        p.IssuerURL,
				DiscoveryURL:     p.DiscoveryURL,
				UseDiscovery:     p.UseDiscovery,
				Scopes:           p.Scopes,
				ClaimsMapping:    p.ClaimsMapping,
			}
			providers = append(providers, provider)
		}

		// Create middleware config
		middlewareConfig := &Config{
			Providers:       providers,
			TokenHeader:     c.config.TokenHeader,
			TokenQuery:      c.config.TokenQuery,
			TokenCookie:     c.config.TokenCookie,
			BearerPrefix:    c.config.BearerPrefix,
			RequireScopes:   c.config.RequireScopes,
			RequireAudience: c.config.RequireAudience,
			ClaimsKey:       c.config.ClaimsKey,
		}

		// Create middleware
		var err error
		c.middleware, err = NewMiddleware(middlewareConfig, c.logger)
		if err != nil {
			return fmt.Errorf("create OAuth2 middleware: %w", err)
		}
	}

	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.config == nil {
		return fmt.Errorf("OAuth2 config not initialized")
	}
	
	// Middleware can be nil if OAuth2 is disabled
	if c.config.Enabled {
		if c.middleware == nil {
			return fmt.Errorf("OAuth2 enabled but middleware not created")
		}
		
		// Validate configuration
		if len(c.config.Providers) == 0 {
			return fmt.Errorf("OAuth2 enabled but no providers configured")
		}
		
		for i, provider := range c.config.Providers {
			if provider.Name == "" {
				return fmt.Errorf("provider[%d] name is required", i)
			}
			if !provider.UseDiscovery {
				if provider.TokenURL == "" {
					return fmt.Errorf("provider[%s] token URL is required when discovery is disabled", provider.Name)
				}
			}
		}
	}

	return nil
}

// Build returns the middleware
func (c *Component) Build() core.Middleware {
	// Can return nil if OAuth2 is disabled
	if c.middleware == nil {
		return nil
	}
	return c.middleware.Middleware()
}

// IsEnabled returns whether OAuth2 is enabled
func (c *Component) IsEnabled() bool {
	return c.config != nil && c.config.Enabled
}

// GetMiddleware returns the OAuth2 middleware instance
func (c *Component) GetMiddleware() *Middleware {
	return c.middleware
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)