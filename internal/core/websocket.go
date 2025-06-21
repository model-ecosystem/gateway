package core

import (
	"context"
	"io"
	"time"
)

// WebSocketMessageType represents the type of WebSocket message
type WebSocketMessageType int

const (
	// WebSocketTextMessage denotes a text data message
	WebSocketTextMessage WebSocketMessageType = 1
	// WebSocketBinaryMessage denotes a binary data message
	WebSocketBinaryMessage WebSocketMessageType = 2
	// WebSocketCloseMessage denotes a close control message
	WebSocketCloseMessage WebSocketMessageType = 8
	// WebSocketPingMessage denotes a ping control message
	WebSocketPingMessage WebSocketMessageType = 9
	// WebSocketPongMessage denotes a pong control message
	WebSocketPongMessage WebSocketMessageType = 10
)

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type WebSocketMessageType
	Data []byte
}

// WebSocketConn represents a WebSocket connection
type WebSocketConn interface {
	// ReadMessage reads a message from the connection
	ReadMessage() (*WebSocketMessage, error)
	// WriteMessage writes a message to the connection
	WriteMessage(msg *WebSocketMessage) error
	// Close closes the connection
	Close() error
	// SetReadDeadline sets the read deadline
	SetReadDeadline(t time.Time) error
	// SetWriteDeadline sets the write deadline
	SetWriteDeadline(t time.Time) error
	// SetPingHandler sets the handler for ping messages
	SetPingHandler(h func(data string) error)
	// SetPongHandler sets the handler for pong messages
	SetPongHandler(h func(data string) error)
	// LocalAddr returns the local address
	LocalAddr() string
	// RemoteAddr returns the remote address
	RemoteAddr() string
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler interface {
	// HandleWebSocket handles a WebSocket connection
	HandleWebSocket(ctx context.Context, conn WebSocketConn) error
}

// WebSocketUpgrader upgrades HTTP connections to WebSocket
type WebSocketUpgrader interface {
	// Upgrade upgrades the HTTP connection to WebSocket
	Upgrade(ctx context.Context, r io.Reader, w io.Writer, headers map[string][]string) (WebSocketConn, error)
}

// WebSocketProxy proxies WebSocket connections
type WebSocketProxy interface {
	// ProxyWebSocket proxies a WebSocket connection to a backend
	ProxyWebSocket(ctx context.Context, clientConn WebSocketConn, backendURL string) error
}
