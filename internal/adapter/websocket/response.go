package websocket

import (
	"io"

	"gateway/internal/core"
)

// response implements core.Response for WebSocket
type response struct {
	conn       core.WebSocketConn
	statusCode int
	headers    map[string][]string
}

// newResponse creates a new WebSocket response
func newResponse(conn core.WebSocketConn, statusCode int) *response {
	return &response{
		conn:       conn,
		statusCode: statusCode,
		headers:    make(map[string][]string),
	}
}

// StatusCode returns the status code
func (r *response) StatusCode() int {
	return r.statusCode
}

// Headers returns the response headers
func (r *response) Headers() map[string][]string {
	return r.headers
}

// Body returns the response body (not used for WebSocket)
func (r *response) Body() io.ReadCloser {
	return io.NopCloser(nil)
}

// WebSocketConn returns the underlying WebSocket connection
func (r *response) WebSocketConn() core.WebSocketConn {
	return r.conn
}
