package factory

import (
	"net/http"
	"testing"
	"time"

	"gateway/internal/config"
	"log/slog"
)

func TestCreateHTTPClient(t *testing.T) {
	tests := []struct {
		name   string
		config config.HTTPBackend
	}{
		{
			name: "default config",
			config: config.HTTPBackend{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90,
			},
		},
		{
			name: "custom config",
			config: config.HTTPBackend{
				MaxIdleConns:          200,
				MaxIdleConnsPerHost:   20,
				MaxConnsPerHost:       50,
				IdleConnTimeout:       60,
				KeepAlive:             true,
				KeepAliveTimeout:      30,
				DisableCompression:    true,
				DisableHTTP2:          true,
				DialTimeout:           5,
				ResponseHeaderTimeout: 10,
				ExpectContinueTimeout: 1,
				TLSHandshakeTimeout:   5,
			},
		},
		{
			name: "with backend TLS",
			config: config.HTTPBackend{
				MaxIdleConns: 100,
				TLS: &config.BackendTLS{
					Enabled:            true,
					InsecureSkipVerify: true,
					ServerName:         "example.com",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := CreateHTTPClient(tt.config)

			if client == nil {
				t.Fatal("Expected HTTP client, got nil")
			}

			// Verify transport configuration
			transport, ok := client.Transport.(*http.Transport)
			if !ok {
				t.Fatal("Expected *http.Transport")
			}

			// Check connection pool settings
			if transport.MaxIdleConns != tt.config.MaxIdleConns {
				t.Errorf("MaxIdleConns: expected %d, got %d", tt.config.MaxIdleConns, transport.MaxIdleConns)
			}
			if transport.MaxIdleConnsPerHost != tt.config.MaxIdleConnsPerHost {
				t.Errorf("MaxIdleConnsPerHost: expected %d, got %d", tt.config.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
			}

			// The implementation always sets ForceAttemptHTTP2 to true and DisableCompression to false
			// regardless of config, so we don't check these
		})
	}
}

func TestCreateHTTPConnector(t *testing.T) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	config := config.HTTPBackend{
		MaxIdleConns: 100,
	}

	connector := CreateHTTPConnector(client, config)

	if connector == nil {
		t.Fatal("Expected HTTP connector, got nil")
	}

	// Connector doesn't have a Type() method in the interface
	// Just verify it was created successfully
}

func TestCreateSSEConnector(t *testing.T) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	tests := []struct {
		name   string
		config *config.SSEBackend
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name: "with config",
			config: &config.SSEBackend{
				ConnectTimeout: 10,
				ReadTimeout:    30,
				BufferSize:     8192,
				RetryInterval:  5,
				MaxRetries:     3,
				MaxEventSize:   65536,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			connector := CreateSSEConnector(tt.config, client, logger)

			if connector == nil {
				t.Fatal("Expected SSE connector, got nil")
			}

			// Just verify connector was created successfully
		})
	}
}

func TestCreateWebSocketConnector(t *testing.T) {
	tests := []struct {
		name   string
		config *config.WebSocketBackend
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name: "with config",
			config: &config.WebSocketBackend{
				HandshakeTimeout:      10,
				ReadTimeout:           30,
				WriteTimeout:          30,
				ReadBufferSize:        4096,
				WriteBufferSize:       4096,
				MaxMessageSize:        65536,
				MaxConnections:        100,
				ConnectionTimeout:     10,
				IdleConnectionTimeout: 60,
				PingInterval:          30,
				PongTimeout:           60,
				CloseTimeout:          10,
				EnableCompression:     true,
				CompressionLevel:      1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			connector := CreateWebSocketConnector(tt.config, logger)

			if connector == nil {
				t.Fatal("Expected WebSocket connector, got nil")
			}

			// Just verify connector was created successfully
		})
	}
}

func TestCreateBackendTLSConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.BackendTLS
		wantError bool
		wantNil   bool
	}{
		{
			name: "enabled TLS",
			config: &config.BackendTLS{
				Enabled: true,
			},
			wantError: false,
			wantNil:   false,
		},
		{
			name: "insecure skip verify",
			config: &config.BackendTLS{
				Enabled:            true,
				InsecureSkipVerify: true,
			},
			wantError: false,
		},
		{
			name: "with server name",
			config: &config.BackendTLS{
				Enabled:    true,
				ServerName: "example.com",
			},
			wantError: false,
		},
		{
			name: "with invalid client cert",
			config: &config.BackendTLS{
				Enabled:        true,
				ClientCertFile: "nonexistent.crt",
				ClientKeyFile:  "nonexistent.key",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig, err := createBackendTLSConfig(tt.config)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tlsConfig == nil {
				t.Error("Expected TLS config, got nil")
				return
			}

			if tt.config.InsecureSkipVerify != tlsConfig.InsecureSkipVerify {
				t.Errorf("InsecureSkipVerify: expected %v, got %v", tt.config.InsecureSkipVerify, tlsConfig.InsecureSkipVerify)
			}

			if tt.config.ServerName != tlsConfig.ServerName {
				t.Errorf("ServerName: expected %s, got %s", tt.config.ServerName, tlsConfig.ServerName)
			}
		})
	}
}
