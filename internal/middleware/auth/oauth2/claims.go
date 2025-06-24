package oauth2

import (
	"time"
	
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the standard OAuth2/OIDC claims
type Claims struct {
	// Standard JWT claims
	Issuer    string    `json:"iss,omitempty"`
	Subject   string    `json:"sub,omitempty"`
	Audience  []string  `json:"aud,omitempty"`
	ExpiresAt time.Time `json:"exp,omitempty"`
	NotBefore time.Time `json:"nbf,omitempty"`
	IssuedAt  time.Time `json:"iat,omitempty"`
	JWTID     string    `json:"jti,omitempty"`
	
	// OpenID Connect standard claims
	Email             string `json:"email,omitempty"`
	EmailVerified     bool   `json:"email_verified,omitempty"`
	Name              string `json:"name,omitempty"`
	GivenName         string `json:"given_name,omitempty"`
	FamilyName        string `json:"family_name,omitempty"`
	MiddleName        string `json:"middle_name,omitempty"`
	Nickname          string `json:"nickname,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Profile           string `json:"profile,omitempty"`
	Picture           string `json:"picture,omitempty"`
	Website           string `json:"website,omitempty"`
	Gender            string `json:"gender,omitempty"`
	Birthdate         string `json:"birthdate,omitempty"`
	Zoneinfo          string `json:"zoneinfo,omitempty"`
	Locale            string `json:"locale,omitempty"`
	PhoneNumber       string `json:"phone_number,omitempty"`
	PhoneVerified     bool   `json:"phone_number_verified,omitempty"`
	UpdatedAt         int64  `json:"updated_at,omitempty"`
	
	// Authorization claims
	Scopes []string `json:"scope,omitempty"`
	Groups []string `json:"groups,omitempty"`
	Roles  []string `json:"roles,omitempty"`
	
	// Custom claims
	Custom map[string]interface{} `json:"-"`
	
	// Raw claims for access to all data
	Raw jwt.MapClaims `json:"-"`
}

// Valid implements jwt.Claims interface
func (c Claims) Valid() error {
	now := time.Now()
	
	// Check expiration
	if !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt) {
		return jwt.ErrTokenExpired
	}
	
	// Check not before
	if !c.NotBefore.IsZero() && now.Before(c.NotBefore) {
		return jwt.ErrTokenNotValidYet
	}
	
	return nil
}

// HasScope checks if the claims contain a specific scope
func (c *Claims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasGroup checks if the claims contain a specific group
func (c *Claims) HasGroup(group string) bool {
	for _, g := range c.Groups {
		if g == group {
			return true
		}
	}
	return false
}

// HasRole checks if the claims contain a specific role
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// GetCustomClaim retrieves a custom claim by key
func (c *Claims) GetCustomClaim(key string) (interface{}, bool) {
	if c.Custom != nil {
		val, ok := c.Custom[key]
		return val, ok
	}
	if c.Raw != nil {
		val, ok := c.Raw[key]
		return val, ok
	}
	return nil, false
}

// TokenResponse represents the OAuth2 token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// ErrorResponse represents an OAuth2 error response
type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}
