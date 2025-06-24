package factory

import (
	"log/slog"
	"time"

	"gateway/internal/config"
	"gateway/internal/middleware/auth/apikey"
	"gateway/internal/middleware/auth/jwt"
	"gateway/pkg/errors"
)

// ProviderFactory creates authentication provider instances
type ProviderFactory struct {
	BaseComponentFactory
	jwtProvider    *jwt.Provider
	apiKeyProvider *apikey.Provider
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(logger *slog.Logger) *ProviderFactory {
	return &ProviderFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// GetJWTProvider returns the JWT provider, creating it if necessary
func (f *ProviderFactory) GetJWTProvider(cfg *config.JWTConfig) (*jwt.Provider, error) {
	if f.jwtProvider != nil {
		return f.jwtProvider, nil
	}
	
	// Check if JWT is configured
	if cfg == nil || !cfg.Enabled {
		return nil, errors.NewError(errors.ErrorTypeInternal, "JWT provider requested but JWT auth is not enabled")
	}
	
	provider, err := f.createJWTProvider(cfg)
	if err != nil {
		return nil, err
	}
	
	f.jwtProvider = provider
	return provider, nil
}

// GetAPIKeyProvider returns the API key provider, creating it if necessary
func (f *ProviderFactory) GetAPIKeyProvider(cfg *config.APIKeyConfig) (*apikey.Provider, error) {
	if f.apiKeyProvider != nil {
		return f.apiKeyProvider, nil
	}
	
	// Check if API key is configured
	if cfg == nil || !cfg.Enabled {
		return nil, errors.NewError(errors.ErrorTypeInternal, "API key provider requested but API key auth is not enabled")
	}
	
	provider, err := f.createAPIKeyProvider(cfg)
	if err != nil {
		return nil, err
	}
	
	f.apiKeyProvider = provider
	return provider, nil
}

// createJWTProvider creates a JWT provider from configuration
func (f *ProviderFactory) createJWTProvider(cfg *config.JWTConfig) (*jwt.Provider, error) {
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

	return jwt.NewProvider(jwtConfig, f.logger)
}

// createAPIKeyProvider creates an API key provider from configuration
func (f *ProviderFactory) createAPIKeyProvider(cfg *config.APIKeyConfig) (*apikey.Provider, error) {
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

	return apikey.NewProvider(apiKeyConfig, f.logger)
}