package grpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"gateway/internal/config"
	"gateway/internal/connector"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Config defines gRPC connector configuration
type Config struct {
	// Connection settings
	MaxConcurrentStreams  int           `yaml:"maxConcurrentStreams"`
	InitialConnWindowSize int32         `yaml:"initialConnWindowSize"`
	InitialWindowSize     int32         `yaml:"initialWindowSize"`
	KeepAliveTime         time.Duration `yaml:"keepAliveTime"`
	KeepAliveTimeout      time.Duration `yaml:"keepAliveTimeout"`

	// TLS settings
	TLS       bool `yaml:"tls"`
	TLSConfig *tls.Config

	// Retry settings
	MaxRetryAttempts int           `yaml:"maxRetryAttempts"`
	RetryTimeout     time.Duration `yaml:"retryTimeout"`
}

// Connector implements gRPC backend connector
type Connector struct {
	config     *Config
	logger     *slog.Logger
	clients    map[string]*grpc.ClientConn
	clientsMu  sync.RWMutex
	transcoder *Transcoder
}

// New creates a new gRPC connector
func New(cfg *Config, logger *slog.Logger) *Connector {
	if cfg.KeepAliveTime == 0 {
		cfg.KeepAliveTime = 30 * time.Second
	}
	if cfg.KeepAliveTimeout == 0 {
		cfg.KeepAliveTimeout = 10 * time.Second
	}

	return &Connector{
		config:     cfg,
		logger:     logger,
		clients:    make(map[string]*grpc.ClientConn),
		transcoder: NewTranscoder(logger),
	}
}

// WithTranscoder sets a custom transcoder
func (c *Connector) WithTranscoder(transcoder *Transcoder) *Connector {
	c.transcoder = transcoder
	return c
}

// Type returns the connector type
func (c *Connector) Type() string {
	return "grpc"
}

// Forward handles gRPC requests
func (c *Connector) Forward(ctx context.Context, req core.Request, route *core.RouteResult) (core.Response, error) {
	if route == nil || route.Instance == nil {
		return nil, errors.NewError(
			errors.ErrorTypeUnavailable,
			"no backend instance available",
		)
	}

	// Check if route has gRPC configuration with transcoding enabled
	if route.Rule != nil && route.Rule.Metadata != nil {
		if grpcConfig, ok := route.Rule.Metadata["grpc"]; ok {
			if cfg, ok := grpcConfig.(*config.GRPCConfig); ok && cfg.EnableTranscoding {
				// Load proto descriptors if available
				if err := c.loadProtoDescriptors(cfg); err != nil {
					c.logger.Warn("failed to load proto descriptors",
						"error", err,
						"service", cfg.Service,
					)
				}
			}
		}
	}

	// Get or create connection
	target := fmt.Sprintf("%s:%d", route.Instance.Address, route.Instance.Port)
	conn, err := c.getConnection(target)
	if err != nil {
		return nil, errors.NewError(
			errors.ErrorTypeUnavailable,
			"failed to get gRPC connection",
		).WithCause(err).WithDetail("target", target)
	}

	// Extract gRPC method from request path
	method := req.Path()

	// Create metadata from headers
	md := metadata.New(make(map[string]string))
	for k, v := range req.Headers() {
		if len(v) > 0 {
			md.Set(k, v[0])
		}
	}

	// Create context with metadata
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Read request body
	body, err := io.ReadAll(req.Body())
	if err != nil {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"failed to read gRPC request body",
		).WithCause(err)
	}

	// Apply transcoding if enabled
	if route.Rule != nil && route.Rule.Metadata != nil {
		if grpcConfig, ok := route.Rule.Metadata["grpc"]; ok {
			if cfg, ok := grpcConfig.(*config.GRPCConfig); ok && cfg.EnableTranscoding {
				// Transcode JSON to protobuf
				body, err = c.transcoder.TranscodeRequestWithMethod(body, method)
				if err != nil {
					c.logger.Warn("transcoding failed, using pass-through",
						"error", err,
						"method", method,
					)
				}
			}
		}
	}

	// Create gRPC request
	var reply []byte
	err = conn.Invoke(ctx, method, body, &reply)
	if err != nil {
		return nil, c.handleGRPCError(err)
	}

	// Apply reverse transcoding if enabled
	if route.Rule != nil && route.Rule.Metadata != nil {
		if grpcConfig, ok := route.Rule.Metadata["grpc"]; ok {
			if cfg, ok := grpcConfig.(*config.GRPCConfig); ok && cfg.EnableTranscoding {
				// Transcode protobuf to JSON
				reply, err = c.transcoder.TranscodeResponseWithMethod(reply, method)
				if err != nil {
					c.logger.Warn("response transcoding failed, using pass-through",
						"error", err,
						"method", method,
					)
				}
			}
		}
	}

	// Create response
	return &grpcResponse{
		body:    reply,
		headers: make(map[string][]string),
	}, nil
}

// getConnection gets or creates a gRPC connection
func (c *Connector) getConnection(target string) (*grpc.ClientConn, error) {
	c.clientsMu.RLock()
	conn, exists := c.clients[target]
	c.clientsMu.RUnlock()

	if exists {
		return conn, nil
	}

	c.clientsMu.Lock()
	defer c.clientsMu.Unlock()

	// Double-check after acquiring write lock
	if conn, exists := c.clients[target]; exists {
		return conn, nil
	}

	// Create dial options
	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                c.config.KeepAliveTime,
			Timeout:             c.config.KeepAliveTimeout,
			PermitWithoutStream: true,
		}),
	}

	// Configure TLS
	if c.config.TLS {
		if c.config.TLSConfig != nil {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(c.config.TLSConfig)))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Configure window sizes
	if c.config.InitialWindowSize > 0 {
		opts = append(opts, grpc.WithInitialWindowSize(c.config.InitialWindowSize))
	}
	if c.config.InitialConnWindowSize > 0 {
		opts = append(opts, grpc.WithInitialConnWindowSize(c.config.InitialConnWindowSize))
	}

	// Create connection
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "failed to dial gRPC target").WithCause(err).WithDetail("target", target)
	}

	c.clients[target] = conn
	c.logger.Info("created gRPC connection", "target", target)

	return conn, nil
}

// handleGRPCError converts gRPC errors to gateway errors
func (c *Connector) handleGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return errors.NewError(
			errors.ErrorTypeInternal,
			"unknown gRPC error",
		).WithCause(err)
	}

	var errType errors.ErrorType
	switch st.Code() {
	case codes.NotFound:
		errType = errors.ErrorTypeNotFound
	case codes.InvalidArgument:
		errType = errors.ErrorTypeBadRequest
	case codes.DeadlineExceeded:
		errType = errors.ErrorTypeTimeout
	case codes.Unavailable:
		errType = errors.ErrorTypeUnavailable
	default:
		errType = errors.ErrorTypeInternal
	}

	return errors.NewError(
		errType,
		st.Message(),
	).WithDetail("code", st.Code().String())
}

// Close closes all gRPC connections
func (c *Connector) Close() error {
	c.clientsMu.Lock()
	defer c.clientsMu.Unlock()

	for target, conn := range c.clients {
		if err := conn.Close(); err != nil {
			c.logger.Error("failed to close gRPC connection",
				"target", target,
				"error", err,
			)
		}
	}

	c.clients = make(map[string]*grpc.ClientConn)
	return nil
}

// grpcResponse implements core.Response for gRPC responses
type grpcResponse struct {
	body    []byte
	headers map[string][]string
}

func (r *grpcResponse) StatusCode() int {
	return 200 // gRPC always returns 200 for successful responses
}

func (r *grpcResponse) Headers() map[string][]string {
	return r.headers
}

func (r *grpcResponse) Body() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(r.body))
}

// loadProtoDescriptors loads proto descriptors from configuration
func (c *Connector) loadProtoDescriptors(cfg *config.GRPCConfig) error {
	// Try to load from base64 first
	if cfg.ProtoDescriptorBase64 != "" {
		return c.transcoder.LoadProtoDescriptorBase64(cfg.ProtoDescriptorBase64)
	}

	// Try to load from file
	if cfg.ProtoDescriptor != "" {
		data, err := os.ReadFile(cfg.ProtoDescriptor)
		if err != nil {
			return fmt.Errorf("failed to read proto descriptor file: %w", err)
		}
		return c.transcoder.LoadProtoDescriptor(data)
	}

	return nil
}

// Ensure Connector implements connector.Connector
var _ connector.Connector = (*Connector)(nil)
