package jwt

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	
	"gateway/internal/middleware/auth"
	"gateway/pkg/errors"
)

// Config represents JWT provider configuration
type Config struct {
	// Issuer is the expected token issuer
	Issuer string `yaml:"issuer"`
	// Audience is the expected audience
	Audience []string `yaml:"audience"`
	// SigningMethod is the signing algorithm (RS256, HS256, etc)
	SigningMethod string `yaml:"signingMethod"`
	// PublicKey for RS256/RS512 validation
	PublicKey string `yaml:"publicKey"`
	// Secret for HS256/HS512 validation
	Secret string `yaml:"secret"`
	// JWKS endpoint for key discovery
	JWKSEndpoint string `yaml:"jwksEndpoint"`
	// JWKSCacheDuration is how long to cache JWKS
	JWKSCacheDuration time.Duration `yaml:"jwksCacheDuration"`
	// ClaimsMapping maps JWT claims to auth metadata
	ClaimsMapping map[string]string `yaml:"claimsMapping"`
	// ScopeClaim is the claim containing scopes/permissions
	ScopeClaim string `yaml:"scopeClaim"`
	// SubjectClaim is the claim containing the subject
	SubjectClaim string `yaml:"subjectClaim"`
}

// Provider implements JWT authentication
type Provider struct {
	config     *Config
	logger     *slog.Logger
	publicKey  interface{}
	jwks       *jwksCache
	httpClient *http.Client
}

// NewProvider creates a new JWT authentication provider
func NewProvider(config *Config, logger *slog.Logger) (*Provider, error) {
	if config.SigningMethod == "" {
		config.SigningMethod = "RS256"
	}
	if config.ScopeClaim == "" {
		config.ScopeClaim = "scope"
	}
	if config.SubjectClaim == "" {
		config.SubjectClaim = "sub"
	}
	if config.JWKSCacheDuration == 0 {
		config.JWKSCacheDuration = 1 * time.Hour
	}

	p := &Provider{
		config: config,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Initialize signing key
	if strings.HasPrefix(config.SigningMethod, "RS") {
		if config.PublicKey != "" {
			key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(config.PublicKey))
			if err != nil {
				return nil, errors.NewError(errors.ErrorTypeInternal, "failed to parse RSA public key").WithCause(err)
			}
			p.publicKey = key
		} else if config.JWKSEndpoint == "" {
			return nil, errors.NewError(errors.ErrorTypeInternal, "RSA signing requires publicKey or jwksEndpoint")
		}
	} else if strings.HasPrefix(config.SigningMethod, "HS") {
		if config.Secret == "" {
			return nil, errors.NewError(errors.ErrorTypeInternal, "HMAC signing requires secret")
		}
		p.publicKey = []byte(config.Secret)
	}

	// Initialize JWKS cache if endpoint provided
	if config.JWKSEndpoint != "" {
		p.jwks = newJWKSCache(config.JWKSEndpoint, config.JWKSCacheDuration, p.httpClient)
	}

	return p, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "jwt"
}

// Authenticate validates a JWT token
func (p *Provider) Authenticate(ctx context.Context, credentials auth.Credentials) (*auth.AuthInfo, error) {
	bearerCreds, ok := credentials.(*auth.BearerCredentials)
	if !ok {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"invalid credential type for JWT provider",
		)
	}

	// Parse token
	token, err := jwt.Parse(bearerCreds.Token, p.keyFunc)
	if err != nil {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"invalid token",
		).WithCause(err)
	}

	if !token.Valid {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"token validation failed",
		)
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.NewError(
			errors.ErrorTypeInternal,
			"invalid token claims",
		)
	}

	// Validate issuer
	if p.config.Issuer != "" {
		issuer, _ := claims["iss"].(string)
		if issuer != p.config.Issuer {
			return nil, errors.NewError(
				errors.ErrorTypeBadRequest,
				"invalid token issuer",
			).WithDetail("expected", p.config.Issuer).WithDetail("actual", issuer)
		}
	}

	// Validate audience
	if len(p.config.Audience) > 0 {
		if !p.validateAudience(claims) {
			return nil, errors.NewError(
				errors.ErrorTypeBadRequest,
				"invalid token audience",
			)
		}
	}

	// Extract subject
	subject, _ := claims[p.config.SubjectClaim].(string)
	if subject == "" {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"missing subject claim",
		)
	}

	// Extract scopes
	scopes := p.extractScopes(claims)

	// Extract expiration
	var expiresAt *time.Time
	if exp, ok := claims["exp"].(float64); ok {
		t := time.Unix(int64(exp), 0)
		expiresAt = &t
	}

	// Build auth info
	authInfo := &auth.AuthInfo{
		Subject:   subject,
		Type:      auth.SubjectTypeUser,
		Scopes:    scopes,
		Metadata:  make(map[string]interface{}),
		ExpiresAt: expiresAt,
		Token:     bearerCreds.Token,
	}

	// Map additional claims
	if p.config.ClaimsMapping != nil {
		for jwtClaim, metaKey := range p.config.ClaimsMapping {
			if value, ok := claims[jwtClaim]; ok {
				authInfo.Metadata[metaKey] = value
			}
		}
	}

	// Determine subject type from claims
	if typ, ok := claims["typ"].(string); ok {
		switch typ {
		case "service":
			authInfo.Type = auth.SubjectTypeService
		case "device":
			authInfo.Type = auth.SubjectTypeDevice
		}
	}

	return authInfo, nil
}

// Refresh is not supported for JWT (stateless)
func (p *Provider) Refresh(ctx context.Context, token string) (*auth.AuthInfo, error) {
	// JWT tokens are stateless, just re-validate
	return p.Authenticate(ctx, &auth.BearerCredentials{Token: token})
}

// keyFunc returns the key for validating the token
func (p *Provider) keyFunc(token *jwt.Token) (interface{}, error) {
	// Check signing method
	if token.Method.Alg() != p.config.SigningMethod {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
	}

	// If we have a static key, use it
	if p.publicKey != nil {
		return p.publicKey, nil
	}

	// Otherwise, fetch from JWKS
	if p.jwks != nil {
		kid, _ := token.Header["kid"].(string)
		return p.jwks.getKey(kid)
	}

	return nil, fmt.Errorf("no key available for token validation")
}

// validateAudience checks if token audience is valid
func (p *Provider) validateAudience(claims jwt.MapClaims) bool {
	audClaim, ok := claims["aud"]
	if !ok {
		return false
	}

	// Handle both string and []string audience
	switch aud := audClaim.(type) {
	case string:
		for _, expected := range p.config.Audience {
			if aud == expected {
				return true
			}
		}
	case []interface{}:
		for _, a := range aud {
			if audStr, ok := a.(string); ok {
				for _, expected := range p.config.Audience {
					if audStr == expected {
						return true
					}
				}
			}
		}
	}

	return false
}

// extractScopes extracts scopes from claims
func (p *Provider) extractScopes(claims jwt.MapClaims) []string {
	scopeClaim, ok := claims[p.config.ScopeClaim]
	if !ok {
		return nil
	}

	var scopes []string

	switch s := scopeClaim.(type) {
	case string:
		// Space-separated scopes
		scopes = strings.Fields(s)
	case []interface{}:
		// Array of scopes
		for _, scope := range s {
			if str, ok := scope.(string); ok {
				scopes = append(scopes, str)
			}
		}
	}

	return scopes
}

// jwksCache caches JWKS keys
type jwksCache struct {
	endpoint   string
	client     *http.Client
	keys       map[string]interface{}
	mu         sync.RWMutex
	lastUpdate time.Time
	ttl        time.Duration
}

func newJWKSCache(endpoint string, ttl time.Duration, client *http.Client) *jwksCache {
	return &jwksCache{
		endpoint: endpoint,
		client:   client,
		keys:     make(map[string]interface{}),
		ttl:      ttl,
	}
}

func (c *jwksCache) getKey(kid string) (interface{}, error) {
	c.mu.RLock()
	if time.Since(c.lastUpdate) < c.ttl {
		key, ok := c.keys[kid]
		c.mu.RUnlock()
		if ok {
			return key, nil
		}
	} else {
		c.mu.RUnlock()
	}

	// Refresh keys
	if err := c.refresh(); err != nil {
		return nil, err
	}

	c.mu.RLock()
	key, ok := c.keys[kid]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key %s not found in JWKS", kid)
	}

	return key, nil
}

func (c *jwksCache) refresh() error {
	resp, err := c.client.Get(c.endpoint)
	if err != nil {
		return errors.NewError(errors.ErrorTypeUnavailable, "failed to fetch JWKS").WithCause(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.NewError(errors.ErrorTypeUnavailable, fmt.Sprintf("JWKS endpoint returned status %d", resp.StatusCode))
	}

	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return errors.NewError(errors.ErrorTypeInternal, "failed to decode JWKS response").WithCause(err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear old keys
	c.keys = make(map[string]interface{})

	// Parse each key
	for _, keyData := range jwks.Keys {
		var keyInfo struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
		}

		if err := json.Unmarshal(keyData, &keyInfo); err != nil {
			continue
		}

		// Only process signing keys
		if keyInfo.Use != "" && keyInfo.Use != "sig" {
			continue
		}

		// Parse based on key type
		switch keyInfo.Kty {
		case "RSA":
			key, err := parseRSAKey(keyData)
			if err == nil && keyInfo.Kid != "" {
				c.keys[keyInfo.Kid] = key
			}
		}
	}

	c.lastUpdate = time.Now()
	return nil
}

// parseRSAKey parses an RSA key from JWKS
func parseRSAKey(data []byte) (*rsa.PublicKey, error) {
	var key struct {
		N string `json:"n"`
		E string `json:"e"`
	}

	if err := json.Unmarshal(data, &key); err != nil {
		return nil, err
	}

	// Decode base64url encoded modulus
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}

	// Decode base64url encoded exponent
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}

	// Convert to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	// Create RSA public key
	pubKey := &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}

	return pubKey, nil
}