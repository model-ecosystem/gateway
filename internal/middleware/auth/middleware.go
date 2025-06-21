package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"gateway/internal/core"
	"gateway/pkg/errors"
)

// Config represents authentication middleware configuration
type Config struct {
	// Required indicates if authentication is required
	Required bool `yaml:"required"`
	// Providers is the list of auth providers to use
	Providers []string `yaml:"providers"`
	// SkipPaths are paths that don't require authentication
	SkipPaths []string `yaml:"skipPaths"`
	// RequiredScopes are scopes required for all requests
	RequiredScopes []string `yaml:"requiredScopes"`
	// StoreAuthInfo indicates if auth info should be stored in context
	StoreAuthInfo bool `yaml:"storeAuthInfo"`
}

// Middleware provides authentication middleware
type Middleware struct {
	config     *Config
	logger     *slog.Logger
	providers  map[string]Provider
	extractors []Extractor
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(config *Config, logger *slog.Logger) *Middleware {
	if config.StoreAuthInfo {
		config.StoreAuthInfo = true // Default to true
	}

	return &Middleware{
		config:     config,
		logger:     logger,
		providers:  make(map[string]Provider),
		extractors: make([]Extractor, 0),
	}
}

// AddProvider adds an authentication provider
func (m *Middleware) AddProvider(provider Provider) {
	m.providers[provider.Name()] = provider
	m.logger.Info("Added auth provider", "name", provider.Name())
}

// AddExtractor adds a credential extractor
func (m *Middleware) AddExtractor(extractor Extractor) {
	m.extractors = append(m.extractors, extractor)
}

// Handler returns the middleware handler
func (m *Middleware) Handler(next core.Handler) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Check if path should skip auth
		if m.shouldSkipAuth(req.Path()) {
			return next(ctx, req)
		}

		// Extract and authenticate
		authInfo, err := m.authenticateRequest(ctx, req)
		if err != nil {
			return nil, err
		}

		// Continue without auth if not required and no auth info
		if authInfo == nil {
			return next(ctx, req)
		}

		// Validate scopes
		if err := m.validateScopes(authInfo); err != nil {
			return nil, err
		}

		// Store auth info in context if configured
		if m.config.StoreAuthInfo {
			ctx = WithAuthInfo(ctx, authInfo)
		}

		// Call next handler and wrap response
		return m.callNextWithAuth(ctx, req, next, authInfo)
	}
}

// authenticateRequest extracts credentials and attempts authentication
func (m *Middleware) authenticateRequest(ctx context.Context, req core.Request) (*AuthInfo, error) {
	// Extract credentials
	credentials, err := m.extractCredentials(ctx, req)
	if err != nil {
		if m.config.Required {
			return nil, errors.NewError(
				errors.ErrorTypeBadRequest,
				"authentication required",
			).WithCause(err)
		}
		// Auth not required, continue without auth
		return nil, nil
	}

	// Try authentication with providers
	authInfo, err := m.tryAuthentication(ctx, credentials)
	if err != nil {
		if m.config.Required {
			return nil, err
		}
		// Auth not required, continue without auth
		return nil, nil
	}

	return authInfo, nil
}

// tryAuthentication attempts authentication with configured providers
func (m *Middleware) tryAuthentication(ctx context.Context, credentials Credentials) (*AuthInfo, error) {
	var lastErr error

	for _, providerName := range m.config.Providers {
		provider, ok := m.providers[providerName]
		if !ok {
			continue
		}

		// Check if provider can handle these credentials
		if !m.canHandleCredentials(provider, credentials) {
			continue
		}

		info, err := provider.Authenticate(ctx, credentials)
		if err == nil {
			m.logger.Debug("Authentication successful",
				"provider", provider.Name(),
				"subject", info.Subject,
				"type", info.Type,
			)
			return info, nil
		}
		lastErr = err
	}

	// All providers failed
	err := errors.NewError(
		errors.ErrorTypeBadRequest,
		"authentication failed",
	)
	if lastErr != nil {
		err = err.WithCause(lastErr)
	}
	return nil, err
}

// validateScopes checks if the auth info has required scopes
func (m *Middleware) validateScopes(authInfo *AuthInfo) error {
	if len(m.config.RequiredScopes) > 0 {
		if !m.hasRequiredScopes(authInfo.Scopes) {
			return errors.NewError(
				errors.ErrorTypeBadRequest,
				"insufficient permissions",
			).WithDetail("required", m.config.RequiredScopes).
				WithDetail("actual", authInfo.Scopes)
		}
	}
	return nil
}

// callNextWithAuth calls the next handler and wraps the response with auth info
func (m *Middleware) callNextWithAuth(ctx context.Context, req core.Request, next core.Handler, authInfo *AuthInfo) (core.Response, error) {
	// Call next handler
	resp, err := next(ctx, req)
	if err != nil {
		return nil, err
	}

	// Add auth headers to response
	return &authResponseWrapper{
		Response: resp,
		authInfo: authInfo,
	}, nil
}

// shouldSkipAuth checks if the path should skip authentication
func (m *Middleware) shouldSkipAuth(path string) bool {
	for _, skip := range m.config.SkipPaths {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

// extractCredentials extracts credentials from the request
func (m *Middleware) extractCredentials(ctx context.Context, req core.Request) (Credentials, error) {
	// Get headers from request
	headers := req.Headers()

	// Try all extractors
	var lastErr error
	for _, extractor := range m.extractors {
		creds, err := extractor.Extract(ctx, headers)
		if err == nil {
			return creds, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.NewError(
		errors.ErrorTypeBadRequest,
		"no credentials found",
	)
}

// canHandleCredentials checks if a provider can handle the given credentials
func (m *Middleware) canHandleCredentials(provider Provider, creds Credentials) bool {
	switch provider.Name() {
	case "jwt":
		return creds.Type() == "bearer"
	case "apikey":
		return creds.Type() == "apikey"
	default:
		return true
	}
}

// hasRequiredScopes checks if the auth info has all required scopes
func (m *Middleware) hasRequiredScopes(scopes []string) bool {
	scopeMap := make(map[string]bool)
	for _, scope := range scopes {
		scopeMap[scope] = true
	}

	for _, required := range m.config.RequiredScopes {
		if !scopeMap[required] {
			return false
		}
	}

	return true
}

// authResponseWrapper wraps a response with auth info
type authResponseWrapper struct {
	core.Response
	authInfo *AuthInfo
}

// Context keys
type contextKey string

const authInfoKey contextKey = "authInfo"

// WithAuthInfo stores auth info in context
func WithAuthInfo(ctx context.Context, info *AuthInfo) context.Context {
	return context.WithValue(ctx, authInfoKey, info)
}

// GetAuthInfo retrieves auth info from context
func GetAuthInfo(ctx context.Context) (*AuthInfo, bool) {
	info, ok := ctx.Value(authInfoKey).(*AuthInfo)
	return info, ok
}

// HTTPMiddleware creates an HTTP middleware handler
func (m *Middleware) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if path should skip auth
		if m.shouldSkipAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Extract headers
		headers := make(map[string][]string)
		for key, values := range r.Header {
			headers[key] = values
		}

		// Try to extract credentials
		var credentials Credentials
		var lastErr error

		for _, extractor := range m.extractors {
			creds, err := extractor.Extract(r.Context(), headers)
			if err == nil {
				credentials = creds
				break
			}
			lastErr = err
		}

		if credentials == nil {
			if m.config.Required {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
			// Auth not required, continue
			next.ServeHTTP(w, r)
			return
		}

		// Try authentication
		var authInfo *AuthInfo
		for _, providerName := range m.config.Providers {
			provider, ok := m.providers[providerName]
			if !ok || !m.canHandleCredentials(provider, credentials) {
				continue
			}

			info, err := provider.Authenticate(r.Context(), credentials)
			if err == nil {
				authInfo = info
				break
			}
			lastErr = err
		}

		if authInfo == nil {
			if m.config.Required {
				if lastErr != nil {
					m.logger.Warn("Authentication failed", "error", lastErr)
				}
				http.Error(w, "Authentication failed", http.StatusUnauthorized)
				return
			}
			// Auth not required, continue
			next.ServeHTTP(w, r)
			return
		}

		// Check required scopes
		if len(m.config.RequiredScopes) > 0 && !m.hasRequiredScopes(authInfo.Scopes) {
			http.Error(w, "Insufficient permissions", http.StatusForbidden)
			return
		}

		// Store auth info in context
		if m.config.StoreAuthInfo {
			r = r.WithContext(WithAuthInfo(r.Context(), authInfo))
		}

		// Add auth info headers
		w.Header().Set("X-Auth-Subject", authInfo.Subject)
		w.Header().Set("X-Auth-Type", string(authInfo.Type))

		next.ServeHTTP(w, r)
	})
}