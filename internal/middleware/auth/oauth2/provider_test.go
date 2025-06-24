package oauth2

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	
	"github.com/golang-jwt/jwt/v5"
)

func TestProviderDiscovery(t *testing.T) {
	// Create mock OIDC discovery server
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			wellKnown := WellKnown{
				Issuer:                serverURL,
				AuthorizationEndpoint: serverURL + "/auth",
				TokenEndpoint:         serverURL + "/token",
				UserInfoEndpoint:      serverURL + "/userinfo",
				JWKSUri:               serverURL + "/jwks",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(wellKnown)
		} else if r.URL.Path == "/jwks" {
			// Return a valid JWKS with a test key
			jwks := JWKSet{Keys: []JWK{
				{
					Kty: "RSA",
					Use: "sig",
					Kid: "test-key",
					Alg: "RS256",
					N:   "test-n",
					E:   "AQAB",
				},
			}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jwks)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	
	config := ProviderConfig{
		Name:         "test",
		IssuerURL:    server.URL,
		UseDiscovery: true,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}
	
	provider, err := NewProvider(config, nil)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	
	// Check if endpoints were discovered
	expectedAuthURL := server.URL + "/auth"
	if provider.config.AuthorizationURL != expectedAuthURL {
		t.Errorf("Expected authorization URL to be %s, got %s", expectedAuthURL, provider.config.AuthorizationURL)
	}
	expectedTokenURL := server.URL + "/token"
	if provider.config.TokenURL != expectedTokenURL {
		t.Errorf("Expected token URL to be %s, got %s", expectedTokenURL, provider.config.TokenURL)
	}
}

func TestTokenValidation(t *testing.T) {
	// Generate test RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	
	// Create provider with manual configuration
	provider := &Provider{
		config: ProviderConfig{
			Name:             "test",
			ClientID:         "test-client",
			ClientSecret:     "test-secret",
			IssuerURL:        "https://example.com",
			ValidateIssuer:   true,
			ValidateAudience: true,
			Audience:         []string{"test-client"},
		},
		jwks: &JWKS{
			keys: map[string]interface{}{
				"test-key": &privateKey.PublicKey,
			},
		},
	}
	
	// Create test token
	claims := jwt.MapClaims{
		"iss":   "https://example.com",
		"sub":   "user123",
		"aud":   "test-client",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"email": "user@example.com",
		"scope": "openid email profile",
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key"
	
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}
	
	// Validate token
	validatedClaims, err := provider.ValidateToken(tokenString)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}
	
	// Check claims
	if validatedClaims.Subject != "user123" {
		t.Errorf("Expected subject user123, got %s", validatedClaims.Subject)
	}
	if validatedClaims.Email != "user@example.com" {
		t.Errorf("Expected email user@example.com, got %s", validatedClaims.Email)
	}
	if !validatedClaims.HasScope("email") {
		t.Error("Expected token to have email scope")
	}
}

func TestGetAuthorizationURL(t *testing.T) {
	provider := &Provider{
		config: ProviderConfig{
			Name:             "test",
			ClientID:         "test-client",
			AuthorizationURL: "https://example.com/auth",
			Scopes:           []string{"openid", "email", "profile"},
		},
	}
	
	state := "test-state-123"
	redirectURI := "https://myapp.com/callback"
	
	authURL := provider.GetAuthorizationURL(state, redirectURI)
	
	// Check URL components
	if !strings.Contains(authURL, "response_type=code") {
		t.Error("Expected response_type=code in auth URL")
	}
	if !strings.Contains(authURL, "client_id=test-client") {
		t.Error("Expected client_id in auth URL")
	}
	if !strings.Contains(authURL, "state=test-state-123") {
		t.Error("Expected state in auth URL")
	}
	if !strings.Contains(authURL, "scope=openid email profile") {
		t.Error("Expected scopes in auth URL")
	}
}
