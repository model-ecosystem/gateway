package compose

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
	
	"gateway/internal/core"
)

func TestLoadComposeFile(t *testing.T) {
	// Create test compose file
	composeContent := `
version: "3.8"
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    labels:
      gateway.enabled: "true"
      gateway.service: "web-service"
      gateway.port: "80"
    environment:
      ENV_VAR: ${TEST_VAR}
    networks:
      - frontend

  api:
    image: api:latest
    ports:
      - "3000:3000"
    labels:
      gateway.enabled: "true"
      gateway.service: "api-service"
    deploy:
      replicas: 3
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  database:
    image: postgres:13
    ports:
      - "5432:5432"
    labels:
      gateway.enabled: "false"

networks:
  frontend:
    driver: bridge
`

	// Create test directory
	tmpDir := t.TempDir()
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write compose file: %v", err)
	}

	// Create .env file
	envContent := `TEST_VAR=test_value
DB_PASSWORD=secret`
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to write env file: %v", err)
	}

	// Create registry without Docker connection for testing
	registry := &Registry{
		config: &Config{
			ComposeFile:     composeFile,
			EnvironmentFile: envFile,
			LabelPrefix:     "gateway",
			ProjectName:     "test",
		},
		composeData:  make(map[string]*ComposeService),
		services:     make(map[string]*core.Service),
		lastModified: make(map[string]time.Time),
		logger:       slog.Default(),
	}

	// Test loading compose file without Docker integration
	envVars := map[string]string{"TEST_VAR": "test_value"}
	compose, err := registry.loadComposeFile(composeFile, envVars)
	if err != nil {
		t.Fatalf("Failed to load compose file: %v", err)
	}
	
	// Manually populate composeData for testing
	for name, service := range compose.Services {
		service.Name = name
		registry.composeData[name] = service
	}

	// Check loaded services
	if len(registry.composeData) != 3 {
		t.Errorf("Expected 3 services, got %d", len(registry.composeData))
	}

	// Check web service
	if web, ok := registry.composeData["web"]; ok {
		if web.Image != "nginx:latest" {
			t.Errorf("Expected web image to be nginx:latest, got %s", web.Image)
		}
		if len(web.Ports) != 1 || web.Ports[0] != "8080:80" {
			t.Errorf("Expected web ports [8080:80], got %v", web.Ports)
		}
		if web.Labels["gateway.enabled"] != "true" {
			t.Error("Expected gateway.enabled to be true")
		}
		if web.Labels["gateway.service"] != "web-service" {
			t.Errorf("Expected gateway.service to be web-service, got %s", web.Labels["gateway.service"])
		}
	} else {
		t.Error("Web service not found")
	}

	// Check API service
	if api, ok := registry.composeData["api"]; ok {
		if api.Deploy == nil || api.Deploy.Replicas != 3 {
			t.Error("Expected api to have 3 replicas")
		}
		if api.Healthcheck == nil {
			t.Error("Expected api to have healthcheck")
		}
	} else {
		t.Error("API service not found")
	}
}

func TestIsGatewayEnabled(t *testing.T) {
	registry := &Registry{
		config: &Config{
			LabelPrefix: "gateway",
		},
	}

	tests := []struct {
		name     string
		service  *ComposeService
		expected bool
	}{
		{
			name: "enabled via label",
			service: &ComposeService{
				Labels: map[string]string{
					"gateway.enabled": "true",
				},
			},
			expected: true,
		},
		{
			name: "has service label",
			service: &ComposeService{
				Labels: map[string]string{
					"gateway.service": "my-service",
				},
			},
			expected: true,
		},
		{
			name: "has ports",
			service: &ComposeService{
				Ports:  []string{"8080:80"},
				Labels: map[string]string{},
			},
			expected: true,
		},
		{
			name: "disabled via label",
			service: &ComposeService{
				Labels: map[string]string{
					"gateway.enabled": "false",
				},
				Ports: []string{"8080:80"},
			},
			expected: false,
		},
		{
			name: "no gateway config",
			service: &ComposeService{
				Labels: map[string]string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.isGatewayEnabled(tt.service)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetServicePort(t *testing.T) {
	registry := &Registry{
		config: &Config{
			LabelPrefix: "gateway",
		},
	}

	tests := []struct {
		name     string
		service  *ComposeService
		expected int
	}{
		{
			name: "port from label",
			service: &ComposeService{
				Labels: map[string]string{
					"gateway.port": "3000",
				},
			},
			expected: 3000,
		},
		{
			name: "port from mapping",
			service: &ComposeService{
				Ports:  []string{"8080:80"},
				Labels: map[string]string{},
			},
			expected: 80,
		},
		{
			name: "single port",
			service: &ComposeService{
				Ports:  []string{"80"},
				Labels: map[string]string{},
			},
			expected: 80,
		},
		{
			name: "no port",
			service: &ComposeService{
				Labels: map[string]string{},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.getServicePort(tt.service)
			if result != tt.expected {
				t.Errorf("Expected port %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestSubstituteEnvVars(t *testing.T) {
	registry := &Registry{}

	envVars := map[string]string{
		"PORT":     "8080",
		"HOSTNAME": "example.com",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Port is ${PORT}",
			expected: "Port is 8080",
		},
		{
			input:    "Host: $HOSTNAME",
			expected: "Host: example.com",
		},
		{
			input:    "URL: http://${HOSTNAME}:${PORT}",
			expected: "URL: http://example.com:8080",
		},
		{
			input:    "Unknown ${UNKNOWN_VAR}",
			expected: "Unknown ${UNKNOWN_VAR}",
		},
	}

	for _, tt := range tests {
		result := registry.substituteEnvVars(tt.input, envVars)
		if result != tt.expected {
			t.Errorf("substituteEnvVars(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}
