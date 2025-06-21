package factory

import (
	"log/slog"
	"time"

	"gateway/internal/config"
	"gateway/internal/middleware/auth"
	"gateway/internal/middleware/auth/apikey"
	"gateway/internal/middleware/auth/jwt"
	"gateway/pkg/errors"
)

// CreateAuthMiddleware creates authentication middleware from config
func CreateAuthMiddleware(cfg *config.Auth, logger *slog.Logger) (*auth.Middleware, error) {
	if cfg == nil || len(cfg.Providers) == 0 {
		return nil, nil
	}

	// Create middleware config
	authConfig := &auth.Config{
		Required:       cfg.Required,
		Providers:      cfg.Providers,
		SkipPaths:      cfg.SkipPaths,
		RequiredScopes: cfg.RequiredScopes,
		StoreAuthInfo:  true,
	}

	// Create middleware
	middleware := auth.NewMiddleware(authConfig, logger)

	// Add JWT provider if configured
	if cfg.JWT != nil && cfg.JWT.Enabled {
		provider, err := createJWTProvider(cfg.JWT, logger)
		if err != nil {
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to create JWT provider").WithCause(err)
		}
		middleware.AddProvider(provider)

		// Add JWT extractor
		extractor := jwt.NewExtractor()
		if cfg.JWT.HeaderName != "" {
			extractor.HeaderName = cfg.JWT.HeaderName
		}
		if cfg.JWT.CookieName != "" {
			extractor.CookieName = cfg.JWT.CookieName
		}
		middleware.AddExtractor(extractor)
	}

	// Add API key provider if configured
	if cfg.APIKey != nil && cfg.APIKey.Enabled {
		provider, err := createAPIKeyProvider(cfg.APIKey, logger)
		if err != nil {
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to create API key provider").WithCause(err)
		}
		middleware.AddProvider(provider)

		// Add API key extractor
		extractor := apikey.NewExtractor()
		if cfg.APIKey.HeaderName != "" {
			extractor.HeaderName = cfg.APIKey.HeaderName
		}
		if cfg.APIKey.QueryParam != "" {
			extractor.QueryParam = cfg.APIKey.QueryParam
		}
		if cfg.APIKey.Scheme != "" {
			extractor.Scheme = cfg.APIKey.Scheme
		}
		middleware.AddExtractor(extractor)
	}

	return middleware, nil
}

// createJWTProvider creates a JWT authentication provider
func createJWTProvider(cfg *config.JWTConfig, logger *slog.Logger) (*jwt.Provider, error) {
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

	return jwt.NewProvider(jwtConfig, logger)
}

// createAPIKeyProvider creates an API key authentication provider
func createAPIKeyProvider(cfg *config.APIKeyConfig, logger *slog.Logger) (*apikey.Provider, error) {
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

	return apikey.NewProvider(apiKeyConfig, logger)
}