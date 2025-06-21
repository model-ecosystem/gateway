package request

import (
	"context"
	"io"
	"net/http"
)

// BaseRequest provides common request implementation
type BaseRequest struct {
	id         string
	httpReq    *http.Request
	method     string
	protocol   string
	remoteAddr string
}

// NewBase creates a new base request from an HTTP request
func NewBase(id string, httpReq *http.Request, method, protocol string) *BaseRequest {
	return &BaseRequest{
		id:         id,
		httpReq:    httpReq,
		method:     method,
		protocol:   protocol,
		remoteAddr: httpReq.RemoteAddr,
	}
}

// ID returns the unique request identifier
func (r *BaseRequest) ID() string {
	return r.id
}

// Method returns the HTTP method
func (r *BaseRequest) Method() string {
	return r.method
}

// Protocol returns the protocol name
func (r *BaseRequest) Protocol() string {
	return r.protocol
}

// Path returns the request path
func (r *BaseRequest) Path() string {
	return r.httpReq.URL.Path
}

// URL returns the full request URL
func (r *BaseRequest) URL() string {
	return r.httpReq.URL.String()
}

// RemoteAddr returns the client's remote address
func (r *BaseRequest) RemoteAddr() string {
	return r.remoteAddr
}

// Headers returns a copy of the request headers
func (r *BaseRequest) Headers() map[string][]string {
	headers := make(map[string][]string, len(r.httpReq.Header))
	for k, v := range r.httpReq.Header {
		headers[k] = v
	}
	return headers
}

// Body returns the request body reader
func (r *BaseRequest) Body() io.ReadCloser {
	if r.httpReq.Body != nil {
		return r.httpReq.Body
	}
	return io.NopCloser(nil)
}

// Context returns the request context
func (r *BaseRequest) Context() context.Context {
	return r.httpReq.Context()
}
