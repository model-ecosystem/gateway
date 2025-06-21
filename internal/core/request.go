package core

import (
	"context"
	"io"
)

// request is a simple Request implementation
type request struct {
	id         string
	method     string
	path       string
	url        string
	remoteAddr string
	headers    map[string][]string
	body       io.ReadCloser
	ctx        context.Context
}

// NewRequest creates a new request
func NewRequest(id, method, path, url, remoteAddr string, headers map[string][]string, body io.ReadCloser, ctx context.Context) Request {
	return &request{
		id:         id,
		method:     method,
		path:       path,
		url:        url,
		remoteAddr: remoteAddr,
		headers:    headers,
		body:       body,
		ctx:        ctx,
	}
}

func (r *request) ID() string                   { return r.id }
func (r *request) Method() string               { return r.method }
func (r *request) Path() string                 { return r.path }
func (r *request) URL() string                  { return r.url }
func (r *request) RemoteAddr() string           { return r.remoteAddr }
func (r *request) Headers() map[string][]string { return r.headers }
func (r *request) Body() io.ReadCloser          { return r.body }
func (r *request) Context() context.Context     { return r.ctx }
