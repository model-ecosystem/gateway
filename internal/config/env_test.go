package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLoadEnv(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, env := range originalEnv {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	// Set test environment variables
	testEnvVars := map[string]string{
		"GATEWAY_GATEWAY_FRONTEND_HTTP_HOST":    "127.0.0.1",
		"GATEWAY_GATEWAY_FRONTEND_HTTP_PORT":    "9090",
		"GATEWAY_GATEWAY_REGISTRY_TYPE":         "docker",
		"GATEWAY_GATEWAY_METRICS_ENABLED":       "true",
		"GATEWAY_GATEWAY_METRICS_PATH":          "/custom-metrics",
		"GATEWAY_GATEWAY_CORS_ENABLED":          "true",
		"GATEWAY_GATEWAY_CORS_ALLOWEDORIGINS":   "https://example.com,https://app.example.com",
		"GATEWAY_GATEWAY_CORS_ALLOWCREDENTIALS": "true",
		"GATEWAY_GATEWAY_CORS_MAXAGE":           "7200",
	}

	for k, v := range testEnvVars {
		os.Setenv(k, v)
	}

	// Create a config with some default values
	cfg := &Config{
		Gateway: Gateway{
			Frontend: Frontend{
				HTTP: HTTP{
					Host: "0.0.0.0",
					Port: 8080,
				},
			},
			Registry: Registry{
				Type: "static",
			},
		},
	}

	// Load environment variables
	err := LoadEnv(cfg)
	if err != nil {
		t.Fatalf("LoadEnv failed: %v", err)
	}

	// Verify values were overridden
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"HTTP Host", cfg.Gateway.Frontend.HTTP.Host, "127.0.0.1"},
		{"HTTP Port", cfg.Gateway.Frontend.HTTP.Port, 9090},
		{"Registry Type", cfg.Gateway.Registry.Type, "docker"},
		{"Metrics Enabled", cfg.Gateway.Metrics != nil && cfg.Gateway.Metrics.Enabled, true},
		{"Metrics Path", cfg.Gateway.Metrics != nil && cfg.Gateway.Metrics.Path == "/custom-metrics", true},
		{"CORS Enabled", cfg.Gateway.CORS != nil && cfg.Gateway.CORS.Enabled, true},
		{"CORS Credentials", cfg.Gateway.CORS != nil && cfg.Gateway.CORS.AllowCredentials, true},
		{"CORS MaxAge", cfg.Gateway.CORS != nil && cfg.Gateway.CORS.MaxAge == 7200, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.expected) {
				t.Errorf("got %v, expected %v", tt.got, tt.expected)
			}
		})
	}

	// Check CORS allowed origins
	if cfg.Gateway.CORS != nil && len(cfg.Gateway.CORS.AllowedOrigins) == 2 {
		if cfg.Gateway.CORS.AllowedOrigins[0] != "https://example.com" ||
			cfg.Gateway.CORS.AllowedOrigins[1] != "https://app.example.com" {
			t.Errorf("CORS AllowedOrigins not parsed correctly: %v", cfg.Gateway.CORS.AllowedOrigins)
		}
	} else {
		t.Error("CORS AllowedOrigins not loaded from env")
	}
}

func TestLoadEnv_InvalidValues(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, env := range originalEnv {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	tests := []struct {
		name    string
		envVar  string
		value   string
		wantErr bool
	}{
		{
			name:    "Invalid int",
			envVar:  "GATEWAY_GATEWAY_FRONTEND_HTTP_PORT",
			value:   "not-a-number",
			wantErr: true,
		},
		{
			name:    "Invalid bool",
			envVar:  "GATEWAY_GATEWAY_METRICS_ENABLED",
			value:   "maybe",
			wantErr: true,
		},
		{
			name:    "Invalid float",
			envVar:  "GATEWAY_GATEWAY_RETRY_DEFAULT_MULTIPLIER",
			value:   "not-a-float",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv(tt.envVar, tt.value)

			cfg := &Config{}
			err := LoadEnv(cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("LoadEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnvExample(t *testing.T) {
	cfg := &Config{}
	examples := EnvExample(cfg)

	// Check that we get some examples
	if len(examples) == 0 {
		t.Error("Expected some environment variable examples")
	}

	// Check for some expected examples
	expectedPrefixes := []string{
		"GATEWAY_GATEWAY_FRONTEND_HTTP_PORT=",
		"GATEWAY_GATEWAY_FRONTEND_HTTP_HOST=",
		"GATEWAY_GATEWAY_REGISTRY_TYPE=",
	}

	for _, prefix := range expectedPrefixes {
		found := false
		for _, example := range examples {
			if strings.HasPrefix(example, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find example starting with %s", prefix)
		}
	}
}

func TestHasEnvVarsWithPrefix(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, env := range originalEnv {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	os.Clearenv()
	os.Setenv("GATEWAY_TEST_VAR", "value")
	os.Setenv("OTHER_VAR", "value")

	tests := []struct {
		prefix string
		want   bool
	}{
		{"GATEWAY_TEST", true},
		{"GATEWAY_MISSING", false},
		{"OTHER", true},
		{"NOTFOUND", false},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			got := hasEnvVarsWithPrefix(tt.prefix)
			if got != tt.want {
				t.Errorf("hasEnvVarsWithPrefix(%s) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}
