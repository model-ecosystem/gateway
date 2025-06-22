package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"gateway/internal/middleware/auth"
	"gateway/pkg/errors"
)

// Test RSA key pair for testing (2048-bit key)
const (
	testPrivateKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA2eWlMM1qOrNkJbhxJPXP7zwdLnnl3JT31ofPXOWaXAJLZlj8
g2OHdvZmbG12dWDmrqYtlhR8vG5d/OxDNL9ifirLSSVjLbPAxZIpQbYrZ1dyS6WC
/V+4CbfRbrg7uejfWBjRs8jXLESKLiW7ZbmSh3aQv2bqvLrJ7HW+8vOhyTQXHUl8
zA/cxNpmsxgAQLJfrigzHRkwbAhZI4PozVhnpajE75OX/2c6Z88CI9ED36vIzyBf
vGuflD5gLdo1WfWrUGLLYj+w9bDssAL/Z3Tc45JIWeSXKiBEN/VKxGgT5Gqdq2ze
eSsbrzn5miXGgk+4KyJstQsUCLqJ0RAWRvEWUQIDAQABAoIBADQCf5KNhWyrgx0J
0F/tGIvXAnQkfnxPRnT7h8B5wYVyusKcPPGzoRMBL2N0IVFVJtrLKZhoHXnwshh7
4HLHt9+7oTg9Z1XyUPIXuCdKL0QEeHCb+g82eLxBFwlhikgO0Li7e9p49vtHBOCM
+xUF3XbeEyDMlP0lbKs3U7Oz+YsHpjiP7Mr0xUZe9wvEnbw3Bjfw+vfEPz3pfsC3
DNTWYFlvucsDZzYTLeC/cegRTasP0NPHFki77RZU2I2dnbCt5eMLBhKgjGDPj87p
5RRBNCgBEthbEfQ9jf8tK/hmSmU9HiQZoP/I+4qdxjpBVC8tbOFKEugdAXe4JVBO
LAPROv0CgYEA6y/xJC1tCuReO5emsZ1FL92ByeWu8bWNW2Gpourw6a/1zm2ua2TJ
VRgm0KfQ09Ph071yla9juKVjlgGTW/FIiArDLFB7VbvfmVemlkEGva84BKHB+xqa
wFGUx+E229ppxo/iSeMQIWVaiwrTuPTJ5zRjx3hRTOSRVyO7RbN0OXMCgYEA7S4A
XIAKU1sKKXVseKJVf9MmkWWknotlMsbhRLELKCTcuRfg6mYtLGLdSZkkxKK3e50U
HpifRCk679aFRvwuoDy464RHuCQq5fPmc2zLsOqXU0q4HdTHWSzG8DazB9XGERoj
P87gWjYVB8Dl1uKLyeHlcarltXR8xjOVQWXK0CsCgYEAzSLc90wz/zsfwmTNPcDK
lyxix4JyLFvJ9znhJ7w68+nJwgtDBmM7hOBzAq5NZGY8ZF6q8kqv9V801KN9L8Xu
GNMiV6W/XhFnv62HHSmMwqhxeQDKXMZg0nyWBB25ptwERPA9VWsbJ7Xq2rpP39SL
wwGcQmD8sM/wwYvmDa6wImcCgYBcFgEz6M6ZgH5YjGu6BqUVhQCzcPhSSiLXbRon
VmnTg0RjZN8BgvxFAHmUSq5Y3ihJCTq3imBD0ZI9ble+sMjVk93kKy7BUuGI+IJg
DDylit+ICjmj82oWuGjg+QvXnetR1okbDBJVVCwkH4PdQ4YsstUnpcecBQcw2PQ5
OPFekwKBgEqgpzUwsvRCv4BTPuByYp9aA2IYkCSQV8FFfw2kmAKCcYuBRZTX2F7k
j9h+RQr5EORiZBmKWdZIfwqGOaUIo3qv5cVUQsbTfUVrQ0F4zdsexnqfG0iJ6BJD
pGX58phCUyUYX5WxRPFX93mozmnR1vmHcIo/ReDXfx6CgFmQq5yl
-----END RSA PRIVATE KEY-----`

	testPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA2eWlMM1qOrNkJbhxJPXP
7zwdLnnl3JT31ofPXOWaXAJLZlj8g2OHdvZmbG12dWDmrqYtlhR8vG5d/OxDNL9i
firLSSVjLbPAxZIpQbYrZ1dyS6WC/V+4CbfRbrg7uejfWBjRs8jXLESKLiW7ZbmS
h3aQv2bqvLrJ7HW+8vOhyTQXHUl8zA/cxNpmsxgAQLJfrigzHRkwbAhZI4PozVhn
pajE75OX/2c6Z88CI9ED36vIzyBfvGuflD5gLdo1WfWrUGLLYj+w9bDssAL/Z3Tc
45JIWeSXKiBEN/VKxGgT5Gqdq2zeeSsbrzn5miXGgk+4KyJstQsUCLqJ0RAWRvEW
UQIDAQAB
-----END PUBLIC KEY-----`

	testSecret = "test-secret-key-for-hmac-signing"
)

func TestJWTProvider_NewProvider(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		errorType errors.ErrorType
	}{
		{
			name: "valid RS256 with public key",
			config: &Config{
				SigningMethod: "RS256",
				PublicKey:     testPublicKeyPEM,
				Issuer:        "test-issuer",
				Audience:      []string{"test-audience"},
			},
			wantErr: false,
		},
		{
			name: "valid HS256 with secret",
			config: &Config{
				SigningMethod: "HS256",
				Secret:        testSecret,
			},
			wantErr: false,
		},
		{
			name: "valid with JWKS endpoint",
			config: &Config{
				SigningMethod: "RS256",
				JWKSEndpoint:  "https://example.com/.well-known/jwks.json",
			},
			wantErr: false,
		},
		{
			name: "invalid RSA public key",
			config: &Config{
				SigningMethod: "RS256",
				PublicKey:     "invalid-key",
			},
			wantErr:   true,
			errorType: errors.ErrorTypeInternal,
		},
		{
			name: "RS256 without key or JWKS",
			config: &Config{
				SigningMethod: "RS256",
			},
			wantErr:   true,
			errorType: errors.ErrorTypeInternal,
		},
		{
			name: "HS256 without secret",
			config: &Config{
				SigningMethod: "HS256",
			},
			wantErr:   true,
			errorType: errors.ErrorTypeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.config, logger)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				if gwErr, ok := err.(*errors.Error); ok {
					if gwErr.Type != tt.errorType {
						t.Errorf("Expected error type %s, got %s", tt.errorType, gwErr.Type)
					}
				} else {
					t.Errorf("Expected gateway error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if provider == nil {
				t.Error("Expected provider but got nil")
				return
			}

			if provider.Name() != "jwt" {
				t.Errorf("Expected provider name 'jwt', got %s", provider.Name())
			}
		})
	}
}

func TestJWTProvider_Authenticate_RS256(t *testing.T) {
	// Parse private key for signing
	block, _ := pem.Decode([]byte(testPrivateKeyPEM))
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	config := &Config{
		SigningMethod: "RS256",
		PublicKey:     testPublicKeyPEM,
		Issuer:        "test-issuer",
		Audience:      []string{"test-audience"},
		ScopeClaim:    "scope",
		SubjectClaim:  "sub",
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		name        string
		token       func() string
		wantErr     bool
		errorType   errors.ErrorType
		wantSubject string
		wantScopes  []string
	}{
		{
			name: "valid token",
			token: func() string {
				claims := jwt.MapClaims{
					"iss":   "test-issuer",
					"aud":   "test-audience",
					"sub":   "user123",
					"scope": "read write",
					"exp":   time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				tokenString, _ := token.SignedString(privateKey)
				return tokenString
			},
			wantErr:     false,
			wantSubject: "user123",
			wantScopes:  []string{"read", "write"},
		},
		{
			name: "expired token",
			token: func() string {
				claims := jwt.MapClaims{
					"iss":   "test-issuer",
					"aud":   "test-audience",
					"sub":   "user123",
					"scope": "read",
					"exp":   time.Now().Add(-time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				tokenString, _ := token.SignedString(privateKey)
				return tokenString
			},
			wantErr:   true,
			errorType: errors.ErrorTypeBadRequest,
		},
		{
			name: "invalid issuer",
			token: func() string {
				claims := jwt.MapClaims{
					"iss":   "wrong-issuer",
					"aud":   "test-audience",
					"sub":   "user123",
					"scope": "read",
					"exp":   time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				tokenString, _ := token.SignedString(privateKey)
				return tokenString
			},
			wantErr:   true,
			errorType: errors.ErrorTypeBadRequest,
		},
		{
			name: "invalid audience",
			token: func() string {
				claims := jwt.MapClaims{
					"iss":   "test-issuer",
					"aud":   "wrong-audience",
					"sub":   "user123",
					"scope": "read",
					"exp":   time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				tokenString, _ := token.SignedString(privateKey)
				return tokenString
			},
			wantErr:   true,
			errorType: errors.ErrorTypeBadRequest,
		},
		{
			name: "missing subject",
			token: func() string {
				claims := jwt.MapClaims{
					"iss":   "test-issuer",
					"aud":   "test-audience",
					"scope": "read",
					"exp":   time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				tokenString, _ := token.SignedString(privateKey)
				return tokenString
			},
			wantErr:   true,
			errorType: errors.ErrorTypeBadRequest,
		},
		{
			name: "scopes as array",
			token: func() string {
				claims := jwt.MapClaims{
					"iss":   "test-issuer",
					"aud":   "test-audience",
					"sub":   "user123",
					"scope": []string{"read", "write", "admin"},
					"exp":   time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				tokenString, _ := token.SignedString(privateKey)
				return tokenString
			},
			wantErr:     false,
			wantSubject: "user123",
			wantScopes:  []string{"read", "write", "admin"},
		},
		{
			name: "audience as array",
			token: func() string {
				claims := jwt.MapClaims{
					"iss":   "test-issuer",
					"aud":   []string{"test-audience", "other-audience"},
					"sub":   "user123",
					"scope": "read",
					"exp":   time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				tokenString, _ := token.SignedString(privateKey)
				return tokenString
			},
			wantErr:     false,
			wantSubject: "user123",
			wantScopes:  []string{"read"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentials := &auth.BearerCredentials{
				Token: tt.token(),
			}

			authInfo, err := provider.Authenticate(context.Background(), credentials)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				if gwErr, ok := err.(*errors.Error); ok {
					if gwErr.Type != tt.errorType {
						t.Errorf("Expected error type %s, got %s", tt.errorType, gwErr.Type)
					}
				} else {
					t.Errorf("Expected gateway error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if authInfo == nil {
				t.Error("Expected auth info but got nil")
				return
			}

			if authInfo.Subject != tt.wantSubject {
				t.Errorf("Expected subject %s, got %s", tt.wantSubject, authInfo.Subject)
			}

			if len(authInfo.Scopes) != len(tt.wantScopes) {
				t.Errorf("Expected %d scopes, got %d", len(tt.wantScopes), len(authInfo.Scopes))
			} else {
				for i, scope := range tt.wantScopes {
					if authInfo.Scopes[i] != scope {
						t.Errorf("Expected scope %s at index %d, got %s", scope, i, authInfo.Scopes[i])
					}
				}
			}

			if authInfo.Type != auth.SubjectTypeUser {
				t.Errorf("Expected subject type %s, got %s", auth.SubjectTypeUser, authInfo.Type)
			}
		})
	}
}

func TestJWTProvider_Authenticate_HS256(t *testing.T) {
	config := &Config{
		SigningMethod: "HS256",
		Secret:        testSecret,
		Issuer:        "test-issuer",
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create valid token
	claims := jwt.MapClaims{
		"iss": "test-issuer",
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	credentials := &auth.BearerCredentials{
		Token: tokenString,
	}

	authInfo, err := provider.Authenticate(context.Background(), credentials)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if authInfo.Subject != "user123" {
		t.Errorf("Expected subject user123, got %s", authInfo.Subject)
	}
}

func TestJWTProvider_Authenticate_InvalidCredentials(t *testing.T) {
	config := &Config{
		SigningMethod: "RS256",
		PublicKey:     testPublicKeyPEM,
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test with wrong credential type
	credentials := &auth.APIKeyCredentials{
		Key: "test-key",
	}

	_, err = provider.Authenticate(context.Background(), credentials)
	if err == nil {
		t.Error("Expected error for invalid credential type")
		return
	}

	gwErr, ok := err.(*errors.Error)
	if !ok {
		t.Errorf("Expected gateway error, got %v", err)
		return
	}

	if gwErr.Type != errors.ErrorTypeBadRequest {
		t.Errorf("Expected error type %s, got %s", errors.ErrorTypeBadRequest, gwErr.Type)
	}
}

func TestJWTProvider_ClaimsMapping(t *testing.T) {
	// Parse private key for signing
	block, _ := pem.Decode([]byte(testPrivateKeyPEM))
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	config := &Config{
		SigningMethod: "RS256",
		PublicKey:     testPublicKeyPEM,
		ClaimsMapping: map[string]string{
			"email": "email",
			"name":  "full_name",
		},
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	claims := jwt.MapClaims{
		"sub":   "user123",
		"email": "user@example.com",
		"name":  "John Doe",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)

	credentials := &auth.BearerCredentials{
		Token: tokenString,
	}

	authInfo, err := provider.Authenticate(context.Background(), credentials)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if authInfo.Metadata["email"] != "user@example.com" {
		t.Errorf("Expected email user@example.com, got %v", authInfo.Metadata["email"])
	}

	if authInfo.Metadata["full_name"] != "John Doe" {
		t.Errorf("Expected full_name John Doe, got %v", authInfo.Metadata["full_name"])
	}
}

func TestJWTProvider_SubjectTypes(t *testing.T) {
	// Parse private key for signing
	block, _ := pem.Decode([]byte(testPrivateKeyPEM))
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	config := &Config{
		SigningMethod: "RS256",
		PublicKey:     testPublicKeyPEM,
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		name        string
		subjectType string
		wantType    auth.SubjectType
	}{
		{"user type", "user", auth.SubjectTypeUser},
		{"service type", "service", auth.SubjectTypeService},
		{"device type", "device", auth.SubjectTypeDevice},
		{"default type", "", auth.SubjectTypeUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := jwt.MapClaims{
				"sub": "test123",
				"exp": time.Now().Add(time.Hour).Unix(),
			}
			if tt.subjectType != "" {
				claims["typ"] = tt.subjectType
			}

			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, _ := token.SignedString(privateKey)

			credentials := &auth.BearerCredentials{
				Token: tokenString,
			}

			authInfo, err := provider.Authenticate(context.Background(), credentials)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if authInfo.Type != tt.wantType {
				t.Errorf("Expected subject type %s, got %s", tt.wantType, authInfo.Type)
			}
		})
	}
}

func TestJWTProvider_JWKS(t *testing.T) {
	// Generate a test RSA key pair for JWKS
	_, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create mock JWKS server
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create JWKS response with the test key
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kid": "test-key-id",
					"kty": "RSA",
					"alg": "RS256",
					"use": "sig",
					"n":   "test-modulus",
					"e":   "AQAB",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer jwksServer.Close()

	config := &Config{
		SigningMethod:     "RS256",
		JWKSEndpoint:      jwksServer.URL,
		JWKSCacheDuration: 1 * time.Hour,
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Verify JWKS cache was initialized
	if provider.jwks == nil {
		t.Error("Expected JWKS cache to be initialized")
	}

	if provider.jwks.endpoint != jwksServer.URL {
		t.Errorf("Expected JWKS endpoint %s, got %s", jwksServer.URL, provider.jwks.endpoint)
	}
}

func TestJWTProvider_Refresh(t *testing.T) {
	// Parse private key for signing
	block, _ := pem.Decode([]byte(testPrivateKeyPEM))
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	config := &Config{
		SigningMethod: "RS256",
		PublicKey:     testPublicKeyPEM,
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create valid token
	claims := jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)

	// Test refresh (should re-validate the token)
	authInfo, err := provider.Refresh(context.Background(), tokenString)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if authInfo.Subject != "user123" {
		t.Errorf("Expected subject user123, got %s", authInfo.Subject)
	}
}

func TestJWTProvider_MalformedToken(t *testing.T) {
	config := &Config{
		SigningMethod: "RS256",
		PublicKey:     testPublicKeyPEM,
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	credentials := &auth.BearerCredentials{
		Token: "invalid.token.here",
	}

	_, err = provider.Authenticate(context.Background(), credentials)
	if err == nil {
		t.Error("Expected error for malformed token")
		return
	}

	gwErr, ok := err.(*errors.Error)
	if !ok {
		t.Errorf("Expected gateway error, got %v", err)
		return
	}

	if gwErr.Type != errors.ErrorTypeBadRequest {
		t.Errorf("Expected error type %s, got %s", errors.ErrorTypeBadRequest, gwErr.Type)
	}
}

func TestJWTProvider_WrongSigningMethod(t *testing.T) {
	config := &Config{
		SigningMethod: "RS256",
		PublicKey:     testPublicKeyPEM,
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create token with wrong signing method (HS256 instead of RS256)
	claims := jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("wrong-secret"))

	credentials := &auth.BearerCredentials{
		Token: tokenString,
	}

	_, err = provider.Authenticate(context.Background(), credentials)
	if err == nil {
		t.Error("Expected error for wrong signing method")
		return
	}

	gwErr, ok := err.(*errors.Error)
	if !ok {
		t.Errorf("Expected gateway error, got %v", err)
		return
	}

	if gwErr.Type != errors.ErrorTypeBadRequest {
		t.Errorf("Expected error type %s, got %s", errors.ErrorTypeBadRequest, gwErr.Type)
	}
}
