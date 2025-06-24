package oauth2

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// Config represents OAuth2 middleware configuration
type Config struct {
	// Providers configuration
	Providers []ProviderConfig `yaml:"providers"`
	
	// Token extraction
	TokenHeader     string `yaml:"tokenHeader"`     // Default: Authorization
	TokenQuery      string `yaml:"tokenQuery"`      // Query parameter name
	TokenCookie     string `yaml:"tokenCookie"`     // Cookie name
	BearerPrefix    string `yaml:"bearerPrefix"`    // Default: Bearer
	
	// Validation options
	RequireScopes   []string `yaml:"requireScopes"`   // Required scopes
	RequireAudience []string `yaml:"requireAudience"` // Required audience
	
	// Context keys
	ClaimsKey string `yaml:"claimsKey"` // Context key for claims
	
	// Enable/disable
	Enabled bool `yaml:"enabled"`
}

// Middleware implements OAuth2/OIDC authentication
type Middleware struct {
	config    *Config
	providers map[string]*Provider
	logger    *slog.Logger
}

// NewMiddleware creates a new OAuth2 middleware
func NewMiddleware(config *Config, logger *slog.Logger) (*Middleware, error) {
	if logger == nil {
		logger = slog.Default()
	}
	
	// Set defaults
	if config.TokenHeader == "" {
		config.TokenHeader = "Authorization"
	}
	if config.BearerPrefix == "" {
		config.BearerPrefix = "Bearer"
	}
	if config.ClaimsKey == "" {
		config.ClaimsKey = "oauth2_claims"
	}
	
	// Initialize providers
	providers := make(map[string]*Provider)
	for _, providerConfig := range config.Providers {
		provider, err := NewProvider(providerConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize provider %s: %w", providerConfig.Name, err)
		}
		providers[providerConfig.Name] = provider
	}
	
	return &Middleware{
		config:    config,
		providers: providers,
		logger:    logger.With("middleware", "oauth2"),
	}, nil
}

// Middleware implements core.Middleware
func (m *Middleware) Middleware() core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Skip if disabled
			if !m.config.Enabled {
				return next(ctx, req)
			}
			
			// Extract token from headers
			token := m.extractTokenFromRequest(req)
			if token == "" {
				return nil, errors.NewError(errors.ErrorTypeUnauthorized, "missing authentication token")
			}
			
			// Try to validate with each provider
			var lastError error
			for _, provider := range m.providers {
				claims, err := provider.ValidateToken(token)
				if err != nil {
					lastError = err
					continue
				}
				
				// Validate required scopes
				if len(m.config.RequireScopes) > 0 {
					if !m.hasRequiredScopes(claims, m.config.RequireScopes) {
						lastError = fmt.Errorf("missing required scopes")
						continue
					}
				}
				
				// Validate required audience
				if len(m.config.RequireAudience) > 0 {
					if !m.hasRequiredAudience(claims, m.config.RequireAudience) {
						lastError = fmt.Errorf("invalid audience")
						continue
					}
				}
				
				// Success - add claims to context
				ctx = context.WithValue(ctx, m.config.ClaimsKey, claims)
				
				// Log successful authentication
				m.logger.Debug("Authentication successful",
					"provider", provider.Name,
					"subject", claims.Subject,
					"scopes", claims.Scopes,
				)
				
				return next(ctx, req)
			}
			
			// All providers failed
			if lastError != nil {
				return nil, errors.NewError(errors.ErrorTypeUnauthorized, "token validation failed").WithCause(lastError)
			}
			
			return nil, errors.NewError(errors.ErrorTypeUnauthorized, "no providers configured")
		}
	}
}

// extractTokenFromRequest extracts the token from the request
func (m *Middleware) extractTokenFromRequest(req core.Request) string {
	headers := req.Headers()
	
	// Try header first
	if m.config.TokenHeader != "" {
		if headerValues, ok := headers[m.config.TokenHeader]; ok && len(headerValues) > 0 {
			header := headerValues[0]
			// Remove bearer prefix if present
			if m.config.BearerPrefix != "" {
				header = strings.TrimPrefix(header, m.config.BearerPrefix+" ")
			}
			return strings.TrimSpace(header)
		}
	}
	
	// Try query parameter
	if m.config.TokenQuery != "" {
		// Parse URL to get query params
		if url := req.URL(); url != "" {
			if idx := strings.Index(url, "?"); idx != -1 {
				query := url[idx+1:]
				params := strings.Split(query, "&")
				for _, param := range params {
					parts := strings.SplitN(param, "=", 2)
					if len(parts) == 2 && parts[0] == m.config.TokenQuery {
						return parts[1]
					}
				}
			}
		}
	}
	
	// Try cookie
	if m.config.TokenCookie != "" {
		if cookieHeader, ok := headers["Cookie"]; ok && len(cookieHeader) > 0 {
			cookies := strings.Split(cookieHeader[0], "; ")
			for _, cookie := range cookies {
				parts := strings.SplitN(cookie, "=", 2)
				if len(parts) == 2 && parts[0] == m.config.TokenCookie {
					return parts[1]
				}
			}
		}
	}
	
	return ""
}

// hasRequiredScopes checks if claims contain all required scopes
func (m *Middleware) hasRequiredScopes(claims *Claims, required []string) bool {
	for _, scope := range required {
		if !claims.HasScope(scope) {
			return false
		}
	}
	return true
}

// hasRequiredAudience checks if claims contain valid audience
func (m *Middleware) hasRequiredAudience(claims *Claims, required []string) bool {
	for _, req := range required {
		for _, aud := range claims.Audience {
			if aud == req {
				return true
			}
		}
	}
	return false
}

// GetClaims retrieves claims from request context
func GetClaims(ctx context.Context, key string) (*Claims, bool) {
	claims, ok := ctx.Value(key).(*Claims)
	return claims, ok
}
