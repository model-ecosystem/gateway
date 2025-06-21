package http

import (
	"gateway/internal/core"
	"gateway/pkg/request"
	"net/http"
)

// newRequest creates a new HTTP request wrapper
func newRequest(id string, r *http.Request) core.Request {
	// Set X-Forwarded-Proto header based on TLS
	if r.TLS != nil {
		r.Header.Set("X-Forwarded-Proto", "https")
	} else if r.Header.Get("X-Forwarded-Proto") == "" {
		// Only set if not already present
		r.Header.Set("X-Forwarded-Proto", "http")
	}

	return request.NewBase(id, r, r.Method, "http")
}
