package jwt

import (
	"context"
	"strings"

	"gateway/internal/middleware/auth"
	"gateway/pkg/errors"
)

// Extractor extracts JWT tokens from requests
type Extractor struct {
	// HeaderName is the header to extract token from (default: Authorization)
	HeaderName string
	// Scheme is the auth scheme (default: Bearer)
	Scheme string
	// CookieName is an optional cookie to check for token
	CookieName string
}

// NewExtractor creates a new JWT token extractor
func NewExtractor() *Extractor {
	return &Extractor{
		HeaderName: "Authorization",
		Scheme:     "Bearer",
	}
}

// Extract extracts JWT credentials from request headers
func (e *Extractor) Extract(ctx context.Context, headers map[string][]string) (auth.Credentials, error) {
	// Try Authorization header first
	if authHeaders, ok := headers[e.HeaderName]; ok && len(authHeaders) > 0 {
		for _, header := range authHeaders {
			if token := e.extractFromAuthHeader(header); token != "" {
				return &auth.BearerCredentials{Token: token}, nil
			}
		}
	}

	// Try cookie if configured
	if e.CookieName != "" {
		if cookies, ok := headers["Cookie"]; ok && len(cookies) > 0 {
			for _, cookie := range cookies {
				if token := e.extractFromCookie(cookie); token != "" {
					return &auth.BearerCredentials{Token: token}, nil
				}
			}
		}
	}

	return nil, errors.NewError(
		errors.ErrorTypeBadRequest,
		"no authentication token found",
	)
}

// extractFromAuthHeader extracts token from Authorization header
func (e *Extractor) extractFromAuthHeader(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return ""
	}

	if strings.ToLower(parts[0]) != strings.ToLower(e.Scheme) {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

// extractFromCookie extracts token from cookie
func (e *Extractor) extractFromCookie(cookieHeader string) string {
	cookies := parseCookies(cookieHeader)
	return cookies[e.CookieName]
}

// parseCookies parses a cookie header
func parseCookies(header string) map[string]string {
	cookies := make(map[string]string)
	
	pairs := strings.Split(header, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			cookies[parts[0]] = parts[1]
		}
	}
	
	return cookies
}