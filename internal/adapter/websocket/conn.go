package websocket

import (
	"context"
	"sync"
	"time"

	"gateway/internal/core"
	"gateway/pkg/errors"
	"github.com/gorilla/websocket"
)

// conn wraps gorilla/websocket.Conn to implement core.WebSocketConn
type conn struct {
	ws           *websocket.Conn
	remote       string
	ctx          context.Context
	disconnected bool
	mu           sync.RWMutex
}

// newConn creates a new WebSocket connection wrapper
func newConn(ws *websocket.Conn, remoteAddr string) *conn {
	return &conn{
		ws:     ws,
		remote: remoteAddr,
		ctx:    context.Background(),
	}
}

// newConnWithContext creates a new WebSocket connection wrapper with context
func newConnWithContext(ws *websocket.Conn, remoteAddr string, ctx context.Context) *conn {
	return &conn{
		ws:     ws,
		remote: remoteAddr,
		ctx:    ctx,
	}
}

// ReadMessage reads a message from the connection
func (c *conn) ReadMessage() (*core.WebSocketMessage, error) {
	// Check if context is done (client disconnected)
	select {
	case <-c.ctx.Done():
		c.markDisconnected()
		return nil, errors.NewError(errors.ErrorTypeInternal, "client disconnected")
	default:
	}

	c.mu.RLock()
	if c.disconnected {
		c.mu.RUnlock()
		return nil, errors.NewError(errors.ErrorTypeInternal, "connection is disconnected")
	}
	c.mu.RUnlock()

	msgType, data, err := c.ws.ReadMessage()
	if err != nil {
		c.handleError(err)
		return nil, err
	}

	return &core.WebSocketMessage{
		Type: mapMessageType(msgType),
		Data: data,
	}, nil
}

// WriteMessage writes a message to the connection
func (c *conn) WriteMessage(msg *core.WebSocketMessage) error {
	// Check if context is done (client disconnected)
	select {
	case <-c.ctx.Done():
		c.markDisconnected()
		return errors.NewError(errors.ErrorTypeInternal, "client disconnected")
	default:
	}

	c.mu.RLock()
	if c.disconnected {
		c.mu.RUnlock()
		return errors.NewError(errors.ErrorTypeInternal, "connection is disconnected")
	}
	c.mu.RUnlock()

	err := c.ws.WriteMessage(mapMessageTypeReverse(msg.Type), msg.Data)
	if err != nil {
		c.handleError(err)
		return err
	}
	return nil
}

// Close closes the connection
func (c *conn) Close() error {
	c.markDisconnected()
	return c.ws.Close()
}

// SetReadDeadline sets the read deadline
func (c *conn) SetReadDeadline(t time.Time) error {
	return c.ws.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline
func (c *conn) SetWriteDeadline(t time.Time) error {
	return c.ws.SetWriteDeadline(t)
}

// SetPingHandler sets the handler for ping messages
func (c *conn) SetPingHandler(h func(data string) error) {
	c.ws.SetPingHandler(h)
}

// SetPongHandler sets the handler for pong messages
func (c *conn) SetPongHandler(h func(data string) error) {
	c.ws.SetPongHandler(h)
}

// LocalAddr returns the local address
func (c *conn) LocalAddr() string {
	if addr := c.ws.LocalAddr(); addr != nil {
		return addr.String()
	}
	return ""
}

// RemoteAddr returns the remote address
func (c *conn) RemoteAddr() string {
	return c.remote
}

// mapMessageType maps gorilla websocket message types to core types
func mapMessageType(t int) core.WebSocketMessageType {
	switch t {
	case websocket.TextMessage:
		return core.WebSocketTextMessage
	case websocket.BinaryMessage:
		return core.WebSocketBinaryMessage
	case websocket.CloseMessage:
		return core.WebSocketCloseMessage
	case websocket.PingMessage:
		return core.WebSocketPingMessage
	case websocket.PongMessage:
		return core.WebSocketPongMessage
	default:
		return core.WebSocketTextMessage
	}
}

// mapMessageTypeReverse maps core message types to gorilla websocket types
func mapMessageTypeReverse(t core.WebSocketMessageType) int {
	switch t {
	case core.WebSocketTextMessage:
		return websocket.TextMessage
	case core.WebSocketBinaryMessage:
		return websocket.BinaryMessage
	case core.WebSocketCloseMessage:
		return websocket.CloseMessage
	case core.WebSocketPingMessage:
		return websocket.PingMessage
	case core.WebSocketPongMessage:
		return websocket.PongMessage
	default:
		return websocket.TextMessage
	}
}

// IsDisconnected returns true if the connection is disconnected
func (c *conn) IsDisconnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.disconnected
}

// markDisconnected marks the connection as disconnected
func (c *conn) markDisconnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disconnected = true
}

// handleError handles connection errors
func (c *conn) handleError(err error) {
	if err != nil {
		// Check for disconnect errors
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
			c.markDisconnected()
		}
	}
}

// Ensure conn implements core.WebSocketConn
var _ core.WebSocketConn = (*conn)(nil)
