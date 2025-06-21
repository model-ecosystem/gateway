package websocket

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		HandshakeTimeout:  10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		MaxMessageSize:    1024 * 1024, // 1MB
		MaxConnections:    10,
		ConnectionTimeout: 10 * time.Second,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
		CloseTimeout:      5 * time.Second,
	}
}

// Connector handles WebSocket connections to backend services
type Connector struct {
	config *Config
	dialer *websocket.Dialer
	logger *slog.Logger
}

// NewConnector creates a new WebSocket connector
func NewConnector(config *Config, logger *slog.Logger) *Connector {
	if config == nil {
		config = DefaultConfig()
	}

	dialer := &websocket.Dialer{
		HandshakeTimeout: config.HandshakeTimeout,
		ReadBufferSize:   config.ReadBufferSize,
		WriteBufferSize:  config.WriteBufferSize,
		NetDial: (&net.Dialer{
			Timeout: config.ConnectionTimeout,
		}).Dial,
	}

	return &Connector{
		config: config,
		dialer: dialer,
		logger: logger,
	}
}

// Connect establishes a WebSocket connection to a backend service
func (c *Connector) Connect(ctx context.Context, instance *core.ServiceInstance, path string, headers http.Header) (*Connection, error) {
	// Build WebSocket URL
	scheme := "ws"
	if instance.Scheme == "https" || instance.Scheme == "wss" {
		scheme = "wss"
	}

	u := url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s:%d", instance.Address, instance.Port),
		Path:   path,
	}

	c.logger.Debug("Connecting to WebSocket backend",
		"url", u.String(),
		"instance", instance.ID,
	)

	// Create connection with context
	conn, resp, err := c.dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			return nil, errors.NewError(
				errors.ErrorTypeUnavailable,
				fmt.Sprintf("WebSocket handshake failed: %d", resp.StatusCode),
			).WithCause(err)
		}
		return nil, errors.NewError(
			errors.ErrorTypeUnavailable,
			"Failed to connect to WebSocket backend",
		).WithCause(err)
	}

	// Set max message size
	conn.SetReadLimit(c.config.MaxMessageSize)

	return &Connection{
		conn:     conn,
		instance: instance,
		logger:   c.logger,
	}, nil
}

// Connection represents a WebSocket connection to a backend service
type Connection struct {
	conn     *websocket.Conn
	instance *core.ServiceInstance
	logger   *slog.Logger
	mu       sync.Mutex
}

// ReadMessage reads a message from the backend
func (c *Connection) ReadMessage() (*core.WebSocketMessage, error) {
	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			return nil, errors.NewError(errors.ErrorTypeInternal, "WebSocket connection closed unexpectedly").WithCause(err)
		}
		return nil, errors.NewError(errors.ErrorTypeInternal, "failed to read WebSocket message").WithCause(err)
	}

	return &core.WebSocketMessage{
		Type: mapMessageType(msgType),
		Data: data,
	}, nil
}

// WriteMessage writes a message to the backend
func (c *Connection) WriteMessage(msg *core.WebSocketMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if err := c.conn.WriteMessage(mapMessageTypeReverse(msg.Type), msg.Data); err != nil {
		return errors.NewError(errors.ErrorTypeInternal, "failed to write WebSocket message").WithCause(err)
	}
	return nil
}

// Close closes the connection
func (c *Connection) Close() error {
	if err := c.conn.Close(); err != nil {
		return errors.NewError(errors.ErrorTypeInternal, "failed to close WebSocket connection").WithCause(err)
	}
	return nil
}

// SetReadDeadline sets the read deadline
func (c *Connection) SetReadDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return errors.NewError(errors.ErrorTypeInternal, "failed to set WebSocket read deadline").WithCause(err)
	}
	return nil
}

// SetWriteDeadline sets the write deadline
func (c *Connection) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// SetPingHandler sets the handler for ping messages
func (c *Connection) SetPingHandler(h func(data string) error) {
	c.conn.SetPingHandler(h)
}

// SetPongHandler sets the handler for pong messages
func (c *Connection) SetPongHandler(h func(data string) error) {
	c.conn.SetPongHandler(h)
}

// LocalAddr returns the local address
func (c *Connection) LocalAddr() string {
	if addr := c.conn.LocalAddr(); addr != nil {
		return addr.String()
	}
	return ""
}

// RemoteAddr returns the remote address
func (c *Connection) RemoteAddr() string {
	if addr := c.conn.RemoteAddr(); addr != nil {
		return addr.String()
	}
	return ""
}

// Proxy bidirectionally proxies messages between client and backend
func (c *Connection) Proxy(ctx context.Context, clientConn core.WebSocketConn) error {
	// Error channel to coordinate goroutines
	errChan := make(chan error, 2)
	
	// Client to backend
	go func() {
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
				msg, err := clientConn.ReadMessage()
				if err != nil {
					c.logger.Debug("Error reading from client", "error", err)
					errChan <- err
					return
				}

				if err := c.WriteMessage(msg); err != nil {
					c.logger.Debug("Error writing to backend", "error", err)
					errChan <- err
					return
				}
			}
		}
	}()

	// Backend to client
	go func() {
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
				msg, err := c.ReadMessage()
				if err != nil {
					c.logger.Debug("Error reading from backend", "error", err)
					errChan <- err
					return
				}

				if err := clientConn.WriteMessage(msg); err != nil {
					c.logger.Debug("Error writing to client", "error", err)
					errChan <- err
					return
				}
			}
		}
	}()

	// Wait for first error
	err := <-errChan
	
	// Close both connections
	c.Close()
	clientConn.Close()
	
	// Check if it was a normal close
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return nil
	}
	
	return err
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

// Ensure Connection implements core.WebSocketConn
var _ core.WebSocketConn = (*Connection)(nil)