package core

import (
	"bytes"
	"io"
)

// response is a simple Response implementation
type response struct {
	statusCode int
	headers    map[string][]string
	body       *bytes.Buffer
}

// NewResponse creates a new response for error cases or simple responses
func NewResponse(statusCode int, body []byte) *response {
	buf := new(bytes.Buffer)
	if body != nil {
		buf.Write(body)
	}
	return &response{
		statusCode: statusCode,
		headers:    make(map[string][]string),
		body:       buf,
	}
}

func (r *response) StatusCode() int              { return r.statusCode }
func (r *response) Headers() map[string][]string { return r.headers }
func (r *response) Body() io.ReadCloser          { return io.NopCloser(bytes.NewReader(r.body.Bytes())) }
