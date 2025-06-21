package apikey

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gateway/internal/middleware/auth"
	"gateway/pkg/errors"
)

// Config represents API key provider configuration
type Config struct {
	// Keys is a map of API key ID to key configuration
	Keys map[string]*KeyConfig `yaml:"keys"`
	// HashKeys indicates if keys should be hashed before comparison
	HashKeys bool `yaml:"hashKeys"`
	// DefaultScopes are scopes granted to all keys
	DefaultScopes []string `yaml:"defaultScopes"`
}

// KeyConfig represents configuration for a single API key
type KeyConfig struct {
	// Key is the actual API key (or its hash if HashKeys is true)
	Key string `yaml:"key"`
	// Subject is the key owner
	Subject string `yaml:"subject"`
	// Type is the subject type
	Type string `yaml:"type"`
	// Scopes are the granted scopes
	Scopes []string `yaml:"scopes"`
	// ExpiresAt is when the key expires (optional)
	ExpiresAt *time.Time `yaml:"expiresAt"`
	// Metadata contains additional information
	Metadata map[string]interface{} `yaml:"metadata"`
	// Disabled marks the key as disabled
	Disabled bool `yaml:"disabled"`
}

// Provider implements API key authentication
type Provider struct {
	config *Config
	logger *slog.Logger
	keys   map[string]*KeyConfig
	mu     sync.RWMutex
}

// NewProvider creates a new API key authentication provider
func NewProvider(config *Config, logger *slog.Logger) (*Provider, error) {
	if config.Keys == nil {
		config.Keys = make(map[string]*KeyConfig)
	}

	// Validate keys
	for id, key := range config.Keys {
		if key.Key == "" {
			return nil, fmt.Errorf("API key %s has empty key value", id)
		}
		if key.Subject == "" {
			key.Subject = id
		}
		if key.Type == "" {
			key.Type = "service"
		}
	}

	return &Provider{
		config: config,
		logger: logger,
		keys:   config.Keys,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "apikey"
}

// Authenticate validates an API key
func (p *Provider) Authenticate(ctx context.Context, credentials auth.Credentials) (*auth.AuthInfo, error) {
	apiKeyCreds, ok := credentials.(*auth.APIKeyCredentials)
	if !ok {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"invalid credential type for API key provider",
		)
	}

	// Hash the key if needed
	key := apiKeyCreds.Key
	if p.config.HashKeys {
		hash := sha256.Sum256([]byte(key))
		key = hex.EncodeToString(hash[:])
	}

	// Look up the key
	p.mu.RLock()
	defer p.mu.RUnlock()

	var keyConfig *KeyConfig
	var keyID string

	// Try to find the key
	for id, cfg := range p.keys {
		if cfg.Key == key {
			keyConfig = cfg
			keyID = id
			break
		}
	}

	if keyConfig == nil {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"invalid API key",
		)
	}

	// Check if key is disabled
	if keyConfig.Disabled {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"API key is disabled",
		).WithDetail("keyId", keyID)
	}

	// Check expiration
	if keyConfig.ExpiresAt != nil && time.Now().After(*keyConfig.ExpiresAt) {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"API key has expired",
		).WithDetail("keyId", keyID)
	}

	// Determine subject type
	subjectType := auth.SubjectTypeService
	switch keyConfig.Type {
	case "user":
		subjectType = auth.SubjectTypeUser
	case "device":
		subjectType = auth.SubjectTypeDevice
	}

	// Build scopes
	scopes := append([]string{}, p.config.DefaultScopes...)
	scopes = append(scopes, keyConfig.Scopes...)

	// Build auth info
	authInfo := &auth.AuthInfo{
		Subject:   keyConfig.Subject,
		Type:      subjectType,
		Scopes:    scopes,
		Metadata:  make(map[string]interface{}),
		ExpiresAt: keyConfig.ExpiresAt,
		Token:     keyID, // Store key ID as token
	}

	// Copy metadata
	for k, v := range keyConfig.Metadata {
		authInfo.Metadata[k] = v
	}

	// Add key ID to metadata
	authInfo.Metadata["keyId"] = keyID

	p.logger.Debug("API key authenticated",
		"keyId", keyID,
		"subject", keyConfig.Subject,
		"type", keyConfig.Type,
	)

	return authInfo, nil
}

// Refresh is not supported for API keys
func (p *Provider) Refresh(ctx context.Context, token string) (*auth.AuthInfo, error) {
	return nil, errors.NewError(
		errors.ErrorTypeBadRequest,
		"API key refresh not supported",
	)
}

// AddKey adds a new API key dynamically
func (p *Provider) AddKey(id string, config *KeyConfig) error {
	if config.Key == "" {
		return fmt.Errorf("key value cannot be empty")
	}

	// Hash the key if needed
	if p.config.HashKeys {
		hash := sha256.Sum256([]byte(config.Key))
		config.Key = hex.EncodeToString(hash[:])
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check for duplicate
	if _, exists := p.keys[id]; exists {
		return fmt.Errorf("key %s already exists", id)
	}

	p.keys[id] = config
	p.logger.Info("API key added", "keyId", id, "subject", config.Subject)

	return nil
}

// RemoveKey removes an API key
func (p *Provider) RemoveKey(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.keys[id]; !exists {
		return fmt.Errorf("key %s not found", id)
	}

	delete(p.keys, id)
	p.logger.Info("API key removed", "keyId", id)

	return nil
}

// DisableKey disables an API key
func (p *Provider) DisableKey(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	key, exists := p.keys[id]
	if !exists {
		return fmt.Errorf("key %s not found", id)
	}

	key.Disabled = true
	p.logger.Info("API key disabled", "keyId", id)

	return nil
}

// EnableKey enables an API key
func (p *Provider) EnableKey(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	key, exists := p.keys[id]
	if !exists {
		return fmt.Errorf("key %s not found", id)
	}

	key.Disabled = false
	p.logger.Info("API key enabled", "keyId", id)

	return nil
}
