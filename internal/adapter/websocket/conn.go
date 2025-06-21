package websocket

import (
	"time"

	"github.com/gorilla/websocket"
	"gateway/internal/core"
)

// conn wraps gorilla/websocket.Conn to implement core.WebSocketConn
type conn struct {
	ws     *websocket.Conn
	remote string
}

// newConn creates a new WebSocket connection wrapper
func newConn(ws *websocket.Conn, remoteAddr string) *conn {
	return &conn{
		ws:     ws,
		remote: remoteAddr,
	}
}

// ReadMessage reads a message from the connection
func (c *conn) ReadMessage() (*core.WebSocketMessage, error) {
	msgType, data, err := c.ws.ReadMessage()
	if err != nil {
		return nil, err
	}

	return &core.WebSocketMessage{
		Type: mapMessageType(msgType),
		Data: data,
	}, nil
}

// WriteMessage writes a message to the connection
func (c *conn) WriteMessage(msg *core.WebSocketMessage) error {
	return c.ws.WriteMessage(mapMessageTypeReverse(msg.Type), msg.Data)
}

// Close closes the connection
func (c *conn) Close() error {
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

// Ensure conn implements core.WebSocketConn
var _ core.WebSocketConn = (*conn)(nil)