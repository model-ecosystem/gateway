package session

import (
	"net/url"
	"strings"
	
	"gateway/internal/core"
)

// Extractor extracts session ID from a request
type Extractor interface {
	Extract(req core.Request) string
}

// ExtractorFunc is a function that implements Extractor
type ExtractorFunc func(req core.Request) string

// Extract implements Extractor
func (f ExtractorFunc) Extract(req core.Request) string {
	return f(req)
}

// NewExtractor creates a session extractor based on configuration
func NewExtractor(config *core.SessionAffinityConfig) Extractor {
	if config == nil || !config.Enabled {
		return ExtractorFunc(func(req core.Request) string { return "" })
	}
	
	switch config.Source {
	case core.SessionSourceCookie:
		return NewCookieExtractor(config.CookieName)
	case core.SessionSourceHeader:
		return NewHeaderExtractor(config.HeaderName)
	case core.SessionSourceQuery:
		return NewQueryExtractor(config.QueryParam)
	default:
		// Default to cookie with GATEWAY_SESSION
		return NewCookieExtractor("GATEWAY_SESSION")
	}
}

// CookieExtractor extracts session ID from a cookie
type CookieExtractor struct {
	cookieName string
}

// NewCookieExtractor creates a new cookie extractor
func NewCookieExtractor(cookieName string) *CookieExtractor {
	if cookieName == "" {
		cookieName = "GATEWAY_SESSION"
	}
	return &CookieExtractor{cookieName: cookieName}
}

// Extract extracts session ID from cookie
func (e *CookieExtractor) Extract(req core.Request) string {
	cookieHeaders := req.Headers()["Cookie"]
	if len(cookieHeaders) == 0 {
		return ""
	}
	
	for _, cookieHeader := range cookieHeaders {
		cookies := parseCookies(cookieHeader)
		if value, ok := cookies[e.cookieName]; ok {
			return value
		}
	}
	
	return ""
}

// HeaderExtractor extracts session ID from a header
type HeaderExtractor struct {
	headerName string
}

// NewHeaderExtractor creates a new header extractor
func NewHeaderExtractor(headerName string) *HeaderExtractor {
	if headerName == "" {
		headerName = "X-Session-Id"
	}
	return &HeaderExtractor{headerName: headerName}
}

// Extract extracts session ID from header
func (e *HeaderExtractor) Extract(req core.Request) string {
	values := req.Headers()[e.headerName]
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// QueryExtractor extracts session ID from query parameter
type QueryExtractor struct {
	paramName string
}

// NewQueryExtractor creates a new query parameter extractor
func NewQueryExtractor(paramName string) *QueryExtractor {
	if paramName == "" {
		paramName = "session_id"
	}
	return &QueryExtractor{paramName: paramName}
}

// Extract extracts session ID from query parameter
func (e *QueryExtractor) Extract(req core.Request) string {
	// Parse URL to get query parameters
	u, err := url.Parse(req.URL())
	if err != nil {
		return ""
	}
	
	return u.Query().Get(e.paramName)
}

// parseCookies parses a cookie header string into a map
func parseCookies(cookieHeader string) map[string]string {
	cookies := make(map[string]string)
	
	// Split by semicolon to get individual cookies
	parts := strings.Split(cookieHeader, ";")
	for _, part := range parts {
		// Trim whitespace
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Split by = to get name and value
		idx := strings.Index(part, "=")
		if idx < 0 {
			continue
		}
		
		name := strings.TrimSpace(part[:idx])
		value := strings.TrimSpace(part[idx+1:])
		
		// Remove quotes if present
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		
		cookies[name] = value
	}
	
	return cookies
}