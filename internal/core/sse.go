package core

import (
	"context"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	ID      string // Event ID for reconnection
	Type    string // Event type
	Data    string // Event data (can be multiline)
	Retry   int    // Reconnection time in milliseconds
	Comment string // Comment line
}

// SSEWriter writes SSE events to a connection
type SSEWriter interface {
	// WriteEvent writes an SSE event
	WriteEvent(event *SSEEvent) error
	// WriteComment writes a comment (keepalive)
	WriteComment(comment string) error
	// Flush flushes any buffered data
	Flush() error
	// Close closes the writer
	Close() error
}

// SSEReader reads SSE events from a connection
type SSEReader interface {
	// ReadEvent reads the next SSE event
	ReadEvent() (*SSEEvent, error)
	// Close closes the reader
	Close() error
}

// SSEHandler handles SSE connections
type SSEHandler interface {
	// HandleSSE handles an SSE connection
	HandleSSE(ctx context.Context, writer SSEWriter, req Request) error
}

// SSEProxy proxies SSE connections
type SSEProxy interface {
	// ProxySSE proxies an SSE connection to a backend
	ProxySSE(ctx context.Context, clientWriter SSEWriter, backendURL string, headers map[string][]string) error
}
