package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Provider represents an OAuth2/OIDC provider
type Provider struct {
	config        ProviderConfig
	
	// Internal state
	jwks          *JWKS
	httpClient    *http.Client
	logger        *slog.Logger
	mu            sync.RWMutex
	lastDiscovery time.Time
}

// WellKnown represents OIDC discovery document
type WellKnown struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserInfoEndpoint      string   `json:"userinfo_endpoint"`
	JWKSUri               string   `json:"jwks_uri"`
	ScopesSupported       []string `json:"scopes_supported"`
	ResponseTypesSupported []string `json:"response_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// NewProvider creates a new OAuth2/OIDC provider
func NewProvider(config ProviderConfig, logger *slog.Logger) (*Provider, error) {
	if logger == nil {
		logger = slog.Default()
	}
	
	p := &Provider{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With("provider", config.Name),
	}
	
	// Set default scopes if not provided
	if len(p.config.Scopes) == 0 {
		p.config.Scopes = []string{"openid", "profile", "email"}
	}
	
	// Use discovery if enabled
	if p.config.UseDiscovery && p.config.IssuerURL != "" {
		if err := p.discoverEndpoints(); err != nil {
			return nil, fmt.Errorf("failed to discover endpoints: %w", err)
		}
	}
	
	// Initialize JWKS if endpoint is available
	if p.config.JWKSEndpoint != "" {
		jwks, err := NewJWKS(p.config.JWKSEndpoint, p.httpClient, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize JWKS: %w", err)
		}
		p.jwks = jwks
	}
	
	return p, nil
}

// Config returns the provider configuration
func (p *Provider) Config() ProviderConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// Name returns the provider name
func (p *Provider) Name() string {
	return p.config.Name
}

// ClientID returns the client ID
func (p *Provider) ClientID() string {
	return p.config.ClientID
}

// ClientSecret returns the client secret
func (p *Provider) ClientSecret() string {
	return p.config.ClientSecret
}

// discoverEndpoints performs OIDC discovery
func (p *Provider) discoverEndpoints() error {
	discoveryURL := p.config.DiscoveryURL
	if discoveryURL == "" && p.config.IssuerURL != "" {
		// Build discovery URL from issuer
		discoveryURL = strings.TrimRight(p.config.IssuerURL, "/") + "/.well-known/openid-configuration"
	}
	
	req, err := http.NewRequest("GET", discoveryURL, nil)
	if err != nil {
		return err
	}
	
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discovery failed with status: %d", resp.StatusCode)
	}
	
	var wellKnown WellKnown
	if err := json.NewDecoder(resp.Body).Decode(&wellKnown); err != nil {
		return err
	}
	
	// Update endpoints from discovery
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if wellKnown.AuthorizationEndpoint != "" {
		p.config.AuthorizationURL = wellKnown.AuthorizationEndpoint
	}
	if wellKnown.TokenEndpoint != "" {
		p.config.TokenURL = wellKnown.TokenEndpoint
	}
	if wellKnown.UserInfoEndpoint != "" {
		p.config.UserInfoURL = wellKnown.UserInfoEndpoint
	}
	if wellKnown.JWKSUri != "" {
		p.config.JWKSEndpoint = wellKnown.JWKSUri
	}
	if wellKnown.Issuer != "" && p.config.IssuerURL == "" {
		p.config.IssuerURL = wellKnown.Issuer
	}
	
	p.lastDiscovery = time.Now()
	p.logger.Info("OIDC discovery completed",
		"issuer", wellKnown.Issuer,
		"jwks", wellKnown.JWKSUri,
	)
	
	return nil
}

// ValidateToken validates an OAuth2/OIDC token
func (p *Provider) ValidateToken(tokenString string) (*Claims, error) {
	// Re-discover if needed (every hour)
	if p.config.UseDiscovery && time.Since(p.lastDiscovery) > time.Hour {
		if err := p.discoverEndpoints(); err != nil {
			p.logger.Error("Failed to refresh discovery", "error", err)
		}
	}
	
	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		switch token.Method.(type) {
		case *jwt.SigningMethodRSA, *jwt.SigningMethodECDSA:
			// Use JWKS for public key
			if p.jwks == nil {
				return nil, fmt.Errorf("JWKS not configured")
			}
			
			kid, ok := token.Header["kid"].(string)
			if !ok {
				return nil, fmt.Errorf("kid not found in token header")
			}
			
			return p.jwks.GetKey(kid)
		case *jwt.SigningMethodHMAC:
			// Use client secret for HMAC
			return []byte(p.config.ClientSecret), nil
		default:
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
	})
	
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	
	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}
	
	// Extract and validate claims
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	
	claims := &Claims{
		Raw: mapClaims,
	}
	
	// Validate issuer
	if p.config.ValidateIssuer && p.config.IssuerURL != "" {
		iss, _ := mapClaims["iss"].(string)
		if iss != p.config.IssuerURL {
			return nil, fmt.Errorf("invalid issuer: expected %s, got %s", p.config.IssuerURL, iss)
		}
		claims.Issuer = iss
	}
	
	// Validate audience
	if p.config.ValidateAudience && len(p.config.Audience) > 0 {
		aud := extractAudience(mapClaims)
		if !containsAudience(aud, p.config.Audience) {
			return nil, fmt.Errorf("invalid audience")
		}
		claims.Audience = aud
	}
	
	// Extract standard claims
	claims.Subject, _ = mapClaims["sub"].(string)
	claims.Email, _ = mapClaims["email"].(string)
	claims.Name, _ = mapClaims["name"].(string)
	claims.PreferredUsername, _ = mapClaims["preferred_username"].(string)
	
	// Extract scopes
	if scope, ok := mapClaims["scope"].(string); ok {
		claims.Scopes = strings.Split(scope, " ")
	}
	
	// Extract groups
	if groups, ok := mapClaims["groups"].([]interface{}); ok {
		claims.Groups = make([]string, len(groups))
		for i, g := range groups {
			claims.Groups[i], _ = g.(string)
		}
	}
	
	// Extract custom claims based on mapping
	claims.Custom = make(map[string]interface{})
	for from, to := range p.config.ClaimsMapping {
		if value, ok := mapClaims[from]; ok {
			claims.Custom[to] = value
		}
	}
	
	// Extract expiration
	if exp, ok := mapClaims["exp"].(float64); ok {
		claims.ExpiresAt = time.Unix(int64(exp), 0)
	}
	
	return claims, nil
}

// GetAuthorizationURL generates the authorization URL for OAuth2 flow
func (p *Provider) GetAuthorizationURL(state, redirectURI string) string {
	params := []string{
		"response_type=code",
		"client_id=" + p.config.ClientID,
		"redirect_uri=" + redirectURI,
		"state=" + state,
		"scope=" + strings.Join(p.config.Scopes, " "),
	}
	
	separator := "?"
	if strings.Contains(p.config.AuthorizationURL, "?") {
		separator = "&"
	}
	
	return p.config.AuthorizationURL + separator + strings.Join(params, "&")
}

// ExchangeCode exchanges an authorization code for tokens
func (p *Provider) ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	data := strings.NewReader(fmt.Sprintf(
		"grant_type=authorization_code&code=%s&redirect_uri=%s&client_id=%s&client_secret=%s",
		code, redirectURI, p.config.ClientID, p.config.ClientSecret,
	))
	
	req, err := http.NewRequestWithContext(ctx, "POST", p.config.TokenURL, data)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}
	
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	
	return &tokenResp, nil
}

// GetUserInfo retrieves user information using an access token
func (p *Provider) GetUserInfo(ctx context.Context, accessToken string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Authorization", "Bearer "+accessToken)
	
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status: %d", resp.StatusCode)
	}
	
	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}
	
	return userInfo, nil
}

// Helper functions

func extractAudience(claims jwt.MapClaims) []string {
	switch aud := claims["aud"].(type) {
	case string:
		return []string{aud}
	case []interface{}:
		audience := make([]string, len(aud))
		for i, a := range aud {
			audience[i], _ = a.(string)
		}
		return audience
	default:
		return nil
	}
}

func containsAudience(tokenAud, expectedAud []string) bool {
	for _, expected := range expectedAud {
		for _, actual := range tokenAud {
			if actual == expected {
				return true
			}
		}
	}
	return false
}