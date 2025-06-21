package websocket

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"gateway/pkg/request"
	"gateway/pkg/requestid"
)



// DefaultConfig returns default WebSocket configuration
func DefaultConfig() *Config {
	return &Config{
		Host:                 "0.0.0.0",
		Port:                 8081,
		ReadTimeout:          60,
		WriteTimeout:         60,
		HandshakeTimeout:     10,
		ReadBufferSize:       4096,
		WriteBufferSize:      4096,
		MaxMessageSize:       1024 * 1024, // 1MB
		CheckOrigin:          true,
		WriteDeadline:        30,
		PongWait:             60,
		PingPeriod:           30,
		CloseGracePeriod:     5,
	}
}

// TokenValidator interface for JWT token validation
type TokenValidator interface {
	ValidateConnection(ctx context.Context, connectionID string, token string, onExpired func()) error
	StopValidation(connectionID string)
}

// Adapter implements WebSocket protocol adapter
type Adapter struct {
	config         *Config
	handler        core.Handler
	upgrader       *websocket.Upgrader
	server         *http.Server
	logger         *slog.Logger
	mu             sync.RWMutex
	running        bool
	listener       net.Listener
	tokenValidator TokenValidator
}

// NewAdapter creates a new WebSocket adapter
func NewAdapter(config *Config, handler core.Handler, logger *slog.Logger) *Adapter {
	if config == nil {
		config = DefaultConfig()
	}

	upgrader := &websocket.Upgrader{
		HandshakeTimeout:  time.Duration(config.HandshakeTimeout) * time.Second,
		ReadBufferSize:    config.ReadBufferSize,
		WriteBufferSize:   config.WriteBufferSize,
		EnableCompression: config.EnableCompression,
		CheckOrigin:       makeCheckOrigin(config),
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
			logger.Error("WebSocket upgrade error",
				"status", status,
				"error", reason,
				"path", r.URL.Path,
				"remote", r.RemoteAddr,
			)
			http.Error(w, reason.Error(), status)
		},
	}

	return &Adapter{
		config:   config,
		handler:  handler,
		upgrader: upgrader,
		logger:   logger,
	}
}

// WithTokenValidator sets the token validator for the adapter
func (a *Adapter) WithTokenValidator(validator TokenValidator) *Adapter {
	a.tokenValidator = validator
	return a
}

// Start starts the WebSocket adapter
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return errors.NewError(errors.ErrorTypeInternal, "WebSocket adapter already running")
	}

	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
	
	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleWebSocket)
	
	a.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  time.Duration(a.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(a.config.WriteTimeout) * time.Second,
		TLSConfig:    a.config.TLSConfig,
	}

	// Setup listener - this will fail immediately if port is already in use
	var err error
	a.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return errors.NewError(errors.ErrorTypeInternal, fmt.Sprintf("failed to bind WebSocket listener to %s", addr)).
			WithCause(err)
	}
	
	// Wrap with TLS if enabled
	if a.config.TLS != nil && a.config.TLS.Enabled {
		// Use provided TLSConfig or create default one
		tlsConfig := a.config.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
			if a.config.TLS.CertFile != "" && a.config.TLS.KeyFile != "" {
				cert, err := tls.LoadX509KeyPair(a.config.TLS.CertFile, a.config.TLS.KeyFile)
				if err != nil {
					a.listener.Close()
					return errors.NewError(errors.ErrorTypeInternal, "failed to load TLS certificates").WithCause(err)
				}
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}
		a.listener = tls.NewListener(a.listener, tlsConfig)
		a.logger.Info("WebSocket adapter listening with TLS", "address", addr)
	} else {
		a.logger.Info("WebSocket adapter listening", "address", addr)
	}

	a.running = true

	// Start server in goroutine
	go func() {
		if err := a.server.Serve(a.listener); err != nil && err != http.ErrServerClosed {
			a.logger.Error("WebSocket server error", "error", err)
		}
	}()

	// Wait for context cancellation
	go func() {
		<-ctx.Done()
		if err := a.Stop(context.Background()); err != nil {
			a.logger.Error("Error stopping WebSocket adapter", "error", err)
		}
	}()

	return nil
}

// Stop stops the WebSocket adapter
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	a.logger.Info("Stopping WebSocket adapter")

	if a.server != nil {
		if err := a.server.Shutdown(ctx); err != nil {
			return errors.NewError(errors.ErrorTypeInternal, "failed to shutdown WebSocket server").WithCause(err)
		}
	}

	a.running = false
	return nil
}

// Type returns the adapter type
func (a *Adapter) Type() string {
	return "websocket"
}

// handleWebSocket handles WebSocket upgrade and connection
func (a *Adapter) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Generate request ID if not present
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = requestid.GenerateRequestID()
	}

	// Validate JWT token before upgrade if validator is configured
	if a.tokenValidator != nil {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token := authHeader[7:]
			connectionID := reqID
			
			// Do a preliminary validation check
			err := a.tokenValidator.ValidateConnection(r.Context(), connectionID, token, func() {})
			if err != nil {
				// Initial validation failed - reject before upgrade
				a.logger.Error("JWT validation failed for WebSocket connection",
					"error", err,
					"remote", r.RemoteAddr,
				)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			// Stop this preliminary validation
			a.tokenValidator.StopValidation(connectionID)
		}
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Error already logged by upgrader.Error
		return
	}
	// Note: conn.Close() is NOT deferred here - it will be managed by the handler/proxy

	// Set max message size
	conn.SetReadLimit(a.config.MaxMessageSize)

	// Setup disconnect handler
	conn.SetCloseHandler(func(code int, text string) error {
		a.logger.Info("WebSocket client initiated close",
			"code", code,
			"text", text,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
		)
		message := websocket.FormatCloseMessage(code, text)
		conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
		return nil
	})

	// Create WebSocket connection wrapper with context
	wsConn := newConnWithContext(conn, r.RemoteAddr, r.Context())

	// Create request from HTTP upgrade request
	req := &wsRequest{
		BaseRequest: request.NewBase(reqID, r, "WEBSOCKET", "websocket"),
		conn:        wsConn,
	}

	// Handle the WebSocket connection through the handler chain
	ctx := r.Context()
	
	// Start JWT validation if configured
	if a.tokenValidator != nil {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token := authHeader[7:]
			connectionID := reqID
			
			// Start token validation
			err := a.tokenValidator.ValidateConnection(ctx, connectionID, token, func() {
				// Token expired, close the connection
				a.logger.Info("JWT token expired, closing WebSocket connection",
					"connectionID", connectionID,
					"remote", r.RemoteAddr,
				)
				
				// Send close message
				closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "authentication expired")
				conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
				conn.Close()
			})
			
			if err != nil {
				// Initial validation failed
				a.logger.Error("JWT validation failed for WebSocket connection",
					"error", err,
					"remote", r.RemoteAddr,
				)
				// Send close message
				closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "unauthorized")
				conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
				conn.Close()
				return
			}
			
			// Stop validation when connection closes
			defer a.tokenValidator.StopValidation(connectionID)
		}
	}
	resp, err := a.handler(ctx, req)
	if err != nil {
		a.logger.Error("WebSocket handler error",
			"error", err,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
		)
		// Send close message and close connection on error
		closeMsg := websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error())
		conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
		conn.Close()
		return
	}
	
	// Check if handler successfully processed the WebSocket
	if resp != nil && resp.StatusCode() == http.StatusSwitchingProtocols {
		a.logger.Debug("WebSocket connection established", 
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
		)
		// Connection will be closed by the proxy when done
	} else {
		// If not handled properly, close the connection
		a.logger.Warn("WebSocket handler did not properly handle connection",
			"path", r.URL.Path,
			"status", resp.StatusCode(),
		)
		conn.Close()
	}
}

// makeCheckOrigin creates origin checker function
func makeCheckOrigin(config *Config) func(r *http.Request) bool {
	if !config.CheckOrigin {
		return func(r *http.Request) bool { return true }
	}

	allowedOrigins := make(map[string]bool)
	for _, origin := range config.AllowedOrigins {
		allowedOrigins[origin] = true
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		
		// Allow same origin
		if origin == "http://"+r.Host || origin == "https://"+r.Host {
			return true
		}
		
		// Check allowed origins
		if len(allowedOrigins) == 0 {
			return false
		}
		
		return allowedOrigins[origin] || allowedOrigins["*"]
	}
}


// wsRequest implements core.Request for WebSocket
type wsRequest struct {
	*request.BaseRequest
	conn *conn
}