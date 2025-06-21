package http

import (
	"gateway/internal/core"
	"gateway/pkg/request"
	"net/http"
)

// newRequest creates a new HTTP request wrapper
func newRequest(id string, r *http.Request) core.Request {
	return request.NewBase(id, r, r.Method, "http")
}