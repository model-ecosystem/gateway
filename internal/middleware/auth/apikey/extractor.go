package apikey

import (
	"context"
	"strings"

	"gateway/internal/middleware/auth"
	"gateway/pkg/errors"
)

// Extractor extracts API keys from requests
type Extractor struct {
	// HeaderName is the header to extract key from (default: X-API-Key)
	HeaderName string
	// QueryParam is the query parameter to extract key from (optional)
	QueryParam string
	// Scheme is an optional scheme prefix (e.g., "APIKey")
	Scheme string
}

// NewExtractor creates a new API key extractor
func NewExtractor() *Extractor {
	return &Extractor{
		HeaderName: "X-API-Key",
	}
}

// Extract extracts API key credentials from request
func (e *Extractor) Extract(ctx context.Context, headers map[string][]string) (auth.Credentials, error) {
	// Try header first
	if e.HeaderName != "" {
		// Check exact header name
		if keys, ok := headers[e.HeaderName]; ok && len(keys) > 0 {
			key := e.extractFromHeader(keys[0])
			if key != "" {
				return &auth.APIKeyCredentials{Key: key}, nil
			}
		}

		// Check case-insensitive
		for name, values := range headers {
			if strings.EqualFold(name, e.HeaderName) && len(values) > 0 {
				key := e.extractFromHeader(values[0])
				if key != "" {
					return &auth.APIKeyCredentials{Key: key}, nil
				}
			}
		}
	}

	// Try Authorization header with scheme
	if e.Scheme != "" {
		if authHeaders, ok := headers["Authorization"]; ok && len(authHeaders) > 0 {
			for _, header := range authHeaders {
				if key := e.extractFromAuthHeader(header); key != "" {
					return &auth.APIKeyCredentials{Key: key}, nil
				}
			}
		}
	}

	// Query parameter extraction would require the full request URL
	// For now, we'll skip this as it requires modifying the Extract interface

	return nil, errors.NewError(
		errors.ErrorTypeBadRequest,
		"no API key found",
	)
}

// extractFromHeader extracts key from header value
func (e *Extractor) extractFromHeader(value string) string {
	return strings.TrimSpace(value)
}

// extractFromAuthHeader extracts key from Authorization header with scheme
func (e *Extractor) extractFromAuthHeader(header string) string {
	if e.Scheme == "" {
		return ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return ""
	}

	if strings.EqualFold(parts[0], e.Scheme) {
		return strings.TrimSpace(parts[1])
	}

	return ""
}
