package dockercompose

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
)

func TestRegistry(t *testing.T) {
	// Skip if not in Docker environment
	if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
		t.Skip("Docker not available")
	}

	config := &Config{
		ProjectName:     "test-project",
		LabelPrefix:     "gateway",
		RefreshInterval: 1 * time.Second,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry, err := NewRegistry(config, logger)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Just verify we can create the registry without error
	if registry == nil {
		t.Error("Registry should not be nil")
	}
}

func TestLabelParsing(t *testing.T) {
	r := &Registry{
		config: &Config{
			LabelPrefix: "gateway",
		},
	}

	tests := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name: "explicit enable",
			labels: map[string]string{
				"gateway.enable": "true",
			},
			expected: true,
		},
		{
			name: "explicit disable",
			labels: map[string]string{
				"gateway.enable": "false",
			},
			expected: false,
		},
		{
			name: "has port",
			labels: map[string]string{
				"gateway.port": "8080",
			},
			expected: true,
		},
		{
			name:     "no gateway labels",
			labels:   map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.shouldExposeService(tt.labels)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMetadataExtraction(t *testing.T) {
	r := &Registry{
		config: &Config{
			LabelPrefix: "gateway",
		},
	}

	labels := map[string]string{
		"gateway.path":                    "/api/*",
		"gateway.loadbalance":             "round_robin",
		"gateway.instance.zone":           "us-east-1",
		"com.docker.compose.project":      "myapp",
		"com.docker.compose.service":      "api",
		"other.label":                     "ignored",
	}

	metadata := r.extractMetadata(labels)

	// Check service metadata
	if metadata["path"] != "/api/*" {
		t.Errorf("Expected path /api/*, got %v", metadata["path"])
	}
	if metadata["loadbalance"] != "round_robin" {
		t.Errorf("Expected loadbalance round_robin, got %v", metadata["loadbalance"])
	}
	if metadata["compose.project"] != "myapp" {
		t.Errorf("Expected compose.project myapp, got %v", metadata["compose.project"])
	}
}

func TestPortExtraction(t *testing.T) {
	r := &Registry{
		config: &Config{
			LabelPrefix: "gateway",
		},
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	tests := []struct {
		name        string
		container   types.Container
		expectedPort int
		expectError bool
	}{
		{
			name: "from label",
			container: types.Container{
				Labels: map[string]string{
					"gateway.port": "8080",
				},
			},
			expectedPort: 8080,
			expectError: false,
		},
		{
			name: "invalid port label",
			container: types.Container{
				Labels: map[string]string{
					"gateway.port": "invalid",
				},
			},
			expectError: true,
		},
		{
			name: "from exposed port",
			container: types.Container{
				Ports: []types.Port{
					{PrivatePort: 3000},
				},
			},
			expectedPort: 3000,
			expectError: false,
		},
		{
			name:        "no port",
			container:   types.Container{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := r.getServicePort(tt.container)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if port != tt.expectedPort {
					t.Errorf("Expected port %d, got %d", tt.expectedPort, port)
				}
			}
		})
	}
}