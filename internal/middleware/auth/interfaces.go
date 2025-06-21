package auth

import (
	"context"
	"time"
)

// Provider defines the authentication provider interface
type Provider interface {
	// Name returns the provider name
	Name() string
	// Authenticate validates credentials and returns auth info
	Authenticate(ctx context.Context, credentials Credentials) (*AuthInfo, error)
	// Refresh refreshes an authentication token
	Refresh(ctx context.Context, token string) (*AuthInfo, error)
}

// Credentials represents authentication credentials
type Credentials interface {
	// Type returns the credential type (bearer, apikey, etc)
	Type() string
}

// BearerCredentials represents bearer token credentials
type BearerCredentials struct {
	Token string
}

// Type returns the credential type for bearer tokens
func (c *BearerCredentials) Type() string {
	return "bearer"
}

// APIKeyCredentials represents API key credentials
type APIKeyCredentials struct {
	Key string
}

// Type returns the credential type for API keys
func (c *APIKeyCredentials) Type() string {
	return "apikey"
}

// AuthInfo contains authentication information
type AuthInfo struct {
	// Subject is the authenticated subject (user, service, etc)
	Subject string
	// Type is the subject type (user, service, device)
	Type SubjectType
	// Scopes are the granted permissions/scopes
	Scopes []string
	// Metadata contains additional information
	Metadata map[string]interface{}
	// ExpiresAt is when the auth expires
	ExpiresAt *time.Time
	// Token is the auth token (for refresh)
	Token string
}

// SubjectType represents the type of authenticated subject
type SubjectType string

const (
	// SubjectTypeUser represents a human user subject
	SubjectTypeUser SubjectType = "user"
	// SubjectTypeService represents a service-based subject
	SubjectTypeService SubjectType = "service"
	// SubjectTypeDevice represents a device-based subject
	SubjectTypeDevice SubjectType = "device"
)

// Extractor extracts credentials from a request
type Extractor interface {
	// Extract extracts credentials from request context
	Extract(ctx context.Context, headers map[string][]string) (Credentials, error)
}
