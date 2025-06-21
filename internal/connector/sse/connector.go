package sse

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"gateway/internal/core"
	"gateway/pkg/errors"
)

// Config represents SSE backend configuration
type Config struct {
	DialTimeout      time.Duration
	ResponseTimeout  time.Duration
	KeepaliveTimeout time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		DialTimeout:      10 * time.Second,
		ResponseTimeout:  30 * time.Second, 
		KeepaliveTimeout: 30 * time.Second,
	}
}

// Connector handles SSE connections to backend services
type Connector struct {
	config *Config
	client *http.Client
	logger *slog.Logger
}

// NewConnector creates a new SSE connector
func NewConnector(config *Config, client *http.Client, logger *slog.Logger) *Connector {
	if config == nil {
		config = DefaultConfig()
	}

	if client == nil {
		client = &http.Client{
			Timeout: config.ResponseTimeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: config.DialTimeout,
				}).DialContext,
			},
		}
	}

	return &Connector{
		config: config,
		client: client,
		logger: logger,
	}
}

// Connect establishes an SSE connection to a backend service
func (c *Connector) Connect(ctx context.Context, instance *core.ServiceInstance, path string, headers http.Header) (*Connection, error) {
	// Build URL
	scheme := instance.Scheme
	if scheme == "" {
		scheme = "http"
	}

	url := fmt.Sprintf("%s://%s:%d%s", scheme, instance.Address, instance.Port, path)

	c.logger.Debug("Connecting to SSE backend",
		"url", url,
		"instance", instance.ID,
	)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.NewError(
			errors.ErrorTypeInternal,
			"Failed to create SSE request",
		).WithCause(err)
	}

	// Set headers
	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Make request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.NewError(
			errors.ErrorTypeUnavailable,
			"Failed to connect to SSE backend",
		).WithCause(err)
	}

	// Check response
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, errors.NewError(
			errors.ErrorTypeUnavailable,
			fmt.Sprintf("SSE backend returned status %d", resp.StatusCode),
		)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		resp.Body.Close()
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			fmt.Sprintf("Invalid content type: %s", contentType),
		)
	}

	return &Connection{
		resp:     resp,
		reader:   newReader(resp.Body),
		instance: instance,
		logger:   c.logger,
	}, nil
}

// Connection represents an SSE connection to a backend service
type Connection struct {
	resp     *http.Response
	reader   core.SSEReader
	instance *core.ServiceInstance
	logger   *slog.Logger
	closed   bool
}

// ReadEvent reads the next event from the backend
func (c *Connection) ReadEvent() (*core.SSEEvent, error) {
	if c.closed {
		return nil, errors.NewError(errors.ErrorTypeInternal, "SSE connection closed")
	}
	return c.reader.ReadEvent()
}

// Close closes the connection
func (c *Connection) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	c.reader.Close()
	return c.resp.Body.Close()
}

// Proxy proxies events from backend to client
func (c *Connection) Proxy(ctx context.Context, clientWriter core.SSEWriter) error {
	// Start proxying events
	eventCount := 0
	
	for {
		select {
		case <-ctx.Done():
			c.logger.Debug("SSE proxy context cancelled",
				"instance", c.instance.ID,
				"events", eventCount,
			)
			return errors.NewError(errors.ErrorTypeTimeout, "SSE proxy context cancelled").WithCause(ctx.Err())
			
		default:
			event, err := c.ReadEvent()
			if err != nil {
				if err == io.EOF {
					c.logger.Debug("SSE backend closed connection",
						"instance", c.instance.ID,
						"events", eventCount,
					)
					return nil
				}
				// Check if it's already a structured error
				if _, ok := err.(*errors.Error); ok {
					c.logger.Error("Error reading SSE event",
						"error", err,
						"instance", c.instance.ID,
					)
					return err
				}
				c.logger.Error("Error reading SSE event",
					"error", err,
					"instance", c.instance.ID,
				)
				return errors.NewError(errors.ErrorTypeInternal, "failed to read SSE event").WithCause(err)
			}

			// Forward event to client
			if err := clientWriter.WriteEvent(event); err != nil {
				c.logger.Debug("Error writing to client",
					"error", err,
					"instance", c.instance.ID,
				)
				return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE event to client").WithCause(err)
			}

			eventCount++
			
			// Log periodic stats
			if eventCount%100 == 0 {
				c.logger.Debug("SSE proxy progress",
					"instance", c.instance.ID,
					"events", eventCount,
				)
			}
		}
	}
}