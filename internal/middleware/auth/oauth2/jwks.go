package oauth2

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"` // Key type
	Use string `json:"use"` // Key use
	Kid string `json:"kid"` // Key ID
	Alg string `json:"alg"` // Algorithm
	N   string `json:"n"`   // Modulus (RSA)
	E   string `json:"e"`   // Exponent (RSA)
	X5c []string `json:"x5c"` // X.509 Certificate Chain
}

// JWKSet represents a JSON Web Key Set
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// JWKS handles JSON Web Key Set operations
type JWKS struct {
	endpoint   string
	httpClient *http.Client
	keys       map[string]interface{}
	mu         sync.RWMutex
	lastFetch  time.Time
	logger     *slog.Logger
}

// NewJWKS creates a new JWKS handler
func NewJWKS(endpoint string, httpClient *http.Client, logger *slog.Logger) (*JWKS, error) {
	if logger == nil {
		logger = slog.Default()
	}
	
	j := &JWKS{
		endpoint:   endpoint,
		httpClient: httpClient,
		keys:       make(map[string]interface{}),
		logger:     logger.With("component", "jwks"),
	}
	
	// Initial fetch
	if err := j.refresh(); err != nil {
		return nil, fmt.Errorf("failed to fetch initial JWKS: %w", err)
	}
	
	return j, nil
}

// GetKey retrieves a key by ID
func (j *JWKS) GetKey(kid string) (interface{}, error) {
	j.mu.RLock()
	key, exists := j.keys[kid]
	j.mu.RUnlock()
	
	if exists {
		return key, nil
	}
	
	// Refresh if key not found and last fetch was more than 5 minutes ago
	if time.Since(j.lastFetch) > 5*time.Minute {
		if err := j.refresh(); err != nil {
			j.logger.Error("Failed to refresh JWKS", "error", err)
			return nil, fmt.Errorf("key %s not found", kid)
		}
		
		// Try again after refresh
		j.mu.RLock()
		key, exists = j.keys[kid]
		j.mu.RUnlock()
		
		if exists {
			return key, nil
		}
	}
	
	return nil, fmt.Errorf("key %s not found", kid)
}

// refresh fetches the latest JWKS
func (j *JWKS) refresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", j.endpoint, nil)
	if err != nil {
		return err
	}
	
	resp, err := j.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS fetch failed with status: %d", resp.StatusCode)
	}
	
	var jwks JWKSet
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}
	
	// Parse keys
	newKeys := make(map[string]interface{})
	for _, jwk := range jwks.Keys {
		key, err := j.parseJWK(jwk)
		if err != nil {
			j.logger.Warn("Failed to parse JWK", "kid", jwk.Kid, "error", err)
			continue
		}
		newKeys[jwk.Kid] = key
	}
	
	// Update keys atomically
	j.mu.Lock()
	j.keys = newKeys
	j.lastFetch = time.Now()
	j.mu.Unlock()
	
	j.logger.Info("JWKS refreshed", "keys", len(newKeys))
	return nil
}

// parseJWK parses a JWK into a crypto key
func (j *JWKS) parseJWK(jwk JWK) (interface{}, error) {
	switch jwk.Kty {
	case "RSA":
		return j.parseRSAKey(jwk)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}
}

// parseRSAKey parses an RSA JWK
func (j *JWKS) parseRSAKey(jwk JWK) (*rsa.PublicKey, error) {
	// Try X.509 certificate first if available
	if len(jwk.X5c) > 0 {
		certDER, err := base64.StdEncoding.DecodeString(jwk.X5c[0])
		if err != nil {
			return nil, fmt.Errorf("failed to decode certificate: %w", err)
		}
		
		cert, err := x509.ParseCertificate(certDER)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
		
		rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("certificate does not contain RSA public key")
		}
		
		return rsaKey, nil
	}
	
	// Parse modulus and exponent
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}
	
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}
	
	// Convert exponent bytes to int
	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}

// ParsePEMPublicKey parses a PEM-encoded public key
func ParsePEMPublicKey(pemData []byte) (interface{}, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}
	
	switch block.Type {
	case "PUBLIC KEY":
		return x509.ParsePKIXPublicKey(block.Bytes)
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		return cert.PublicKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}
