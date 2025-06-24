package versioning

import (
	"context"
	"io"
	"testing"
	"time"

	"gateway/internal/core"
)

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		request        core.Request
		expectedVersion string
	}{
		{
			name: "path strategy - version in path",
			config: &Config{
				Enabled:        true,
				Strategy:       StrategyPath,
				DefaultVersion: "1.0",
			},
			request: &mockRequest{
				path: "/v2/users/123",
			},
			expectedVersion: "2",
		},
		{
			name: "path strategy - version with minor",
			config: &Config{
				Enabled:        true,
				Strategy:       StrategyPath,
				DefaultVersion: "1.0",
			},
			request: &mockRequest{
				path: "/v2.1/users/123",
			},
			expectedVersion: "2.1",
		},
		{
			name: "path strategy - no version in path",
			config: &Config{
				Enabled:        true,
				Strategy:       StrategyPath,
				DefaultVersion: "1.0",
			},
			request: &mockRequest{
				path: "/users/123",
			},
			expectedVersion: "1.0",
		},
		{
			name: "header strategy - version in header",
			config: &Config{
				Enabled:        true,
				Strategy:       StrategyHeader,
				DefaultVersion: "1.0",
				VersionHeader:  "X-API-Version",
			},
			request: &mockRequest{
				headers: map[string][]string{
					"X-API-Version": {"2.0"},
				},
			},
			expectedVersion: "2.0",
		},
		{
			name: "query strategy - version in query",
			config: &Config{
				Enabled:        true,
				Strategy:       StrategyQuery,
				DefaultVersion: "1.0",
				VersionQuery:   "version",
			},
			request: &mockRequest{
				url: "/users?version=3&format=json",
			},
			expectedVersion: "3",
		},
		{
			name: "accept strategy - version in accept header",
			config: &Config{
				Enabled:        true,
				Strategy:       StrategyAccept,
				DefaultVersion: "1.0",
			},
			request: &mockRequest{
				headers: map[string][]string{
					"Accept": {"application/vnd.api+json;version=2.5"},
				},
			},
			expectedVersion: "2.5",
		},
		{
			name: "disabled versioning",
			config: &Config{
				Enabled:        false,
				DefaultVersion: "1.0",
			},
			request: &mockRequest{
				path: "/v2/users",
			},
			expectedVersion: "1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.config, nil)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}

			version := manager.ExtractVersion(tt.request)
			if version != tt.expectedVersion {
				t.Errorf("Expected version %s, got %s", tt.expectedVersion, version)
			}
		})
	}
}

func TestTransformPath(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		path         string
		version      string
		expectedPath string
	}{
		{
			name: "remove version from path",
			config: &Config{
				Enabled:  true,
				Strategy: StrategyPath,
			},
			path:         "/v2/users/123",
			version:      "2",
			expectedPath: "/users/123",
		},
		{
			name: "add path prefix for version",
			config: &Config{
				Enabled: true,
				VersionMappings: map[string]VersionMapping{
					"2": {
						PathPrefix: "/api/v2",
					},
				},
			},
			path:         "/users/123",
			version:      "2",
			expectedPath: "/api/v2/users/123",
		},
		{
			name: "no transformation",
			config: &Config{
				Enabled: false,
			},
			path:         "/v2/users/123",
			version:      "2",
			expectedPath: "/v2/users/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.config, nil)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}

			result := manager.TransformPath(tt.path, tt.version)
			if result != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, result)
			}
		})
	}
}

func TestGetServiceForVersion(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		serviceName     string
		version         string
		expectedService string
	}{
		{
			name: "custom service mapping",
			config: &Config{
				Enabled:        true,
				DefaultVersion: "1.0",
				VersionMappings: map[string]VersionMapping{
					"2.0": {
						Service: "api-service-v2",
					},
				},
			},
			serviceName:     "api-service",
			version:         "2.0",
			expectedService: "api-service-v2",
		},
		{
			name: "default pattern",
			config: &Config{
				Enabled:        true,
				DefaultVersion: "1.0",
			},
			serviceName:     "api-service",
			version:         "2.0",
			expectedService: "api-service-2.0",
		},
		{
			name: "default version no suffix",
			config: &Config{
				Enabled:        true,
				DefaultVersion: "1.0",
			},
			serviceName:     "api-service",
			version:         "1.0",
			expectedService: "api-service",
		},
		{
			name: "disabled versioning",
			config: &Config{
				Enabled: false,
			},
			serviceName:     "api-service",
			version:         "2.0",
			expectedService: "api-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.config, nil)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}

			result := manager.GetServiceForVersion(tt.serviceName, tt.version)
			if result != tt.expectedService {
				t.Errorf("Expected service %s, got %s", tt.expectedService, result)
			}
		})
	}
}

func TestCheckDeprecation(t *testing.T) {
	sunset := time.Now().Add(30 * 24 * time.Hour)
	removal := time.Now().Add(90 * 24 * time.Hour)

	config := &Config{
		Enabled: true,
		DeprecatedVersions: map[string]DeprecationInfo{
			"1.0": {
				Message:     "Version 1.0 is deprecated",
				SunsetDate:  sunset,
				RemovalDate: removal,
			},
		},
		VersionMappings: map[string]VersionMapping{
			"1.5": {
				Deprecated: true,
			},
		},
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test deprecated version
	info := manager.CheckDeprecation("1.0")
	if info == nil {
		t.Error("Expected deprecation info for version 1.0")
	} else {
		if info.Message != "Version 1.0 is deprecated" {
			t.Errorf("Expected deprecation message, got %s", info.Message)
		}
	}

	// Test version mapping deprecation
	info = manager.CheckDeprecation("1.5")
	if info == nil {
		t.Error("Expected deprecation info for version 1.5")
	}

	// Test non-deprecated version
	info = manager.CheckDeprecation("2.0")
	if info != nil {
		t.Error("Expected no deprecation info for version 2.0")
	}
}

func TestIsVersionAllowed(t *testing.T) {
	pastRemoval := time.Now().Add(-1 * time.Hour)
	futureRemoval := time.Now().Add(1 * time.Hour)

	config := &Config{
		Enabled: true,
		DeprecatedVersions: map[string]DeprecationInfo{
			"1.0": {
				RemovalDate: pastRemoval,
			},
			"1.5": {
				RemovalDate: futureRemoval,
			},
		},
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test removed version
	if manager.IsVersionAllowed("1.0") {
		t.Error("Expected version 1.0 to not be allowed")
	}

	// Test future removal
	if !manager.IsVersionAllowed("1.5") {
		t.Error("Expected version 1.5 to be allowed")
	}

	// Test allowed version
	if !manager.IsVersionAllowed("2.0") {
		t.Error("Expected version 2.0 to be allowed")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1", "2", -1},
		{"2", "1", 1},
		{"1", "1", 0},
		{"1.0", "1.1", -1},
		{"2.0", "1.9", 1},
		{"1.0.0", "1.0.0", 0},
		{"v1.0", "1.0", 0}, // Handle v prefix
		{"1.2.3", "1.2", 1},
		{"1.2", "1.2.3", -1},
	}

	for _, tt := range tests {
		result := CompareVersions(tt.v1, tt.v2)
		if result != tt.expected {
			t.Errorf("CompareVersions(%s, %s) = %d, expected %d", 
				tt.v1, tt.v2, result, tt.expected)
		}
	}
}

// Mock request for testing
type mockRequest struct {
	id         string
	method     string
	path       string
	url        string
	remoteAddr string
	headers    map[string][]string
	body       string
}

func (r *mockRequest) ID() string                    { return r.id }
func (r *mockRequest) Method() string                { return r.method }
func (r *mockRequest) Path() string                  { return r.path }
func (r *mockRequest) URL() string                   { return r.url }
func (r *mockRequest) RemoteAddr() string            { return r.remoteAddr }
func (r *mockRequest) Headers() map[string][]string  { return r.headers }
func (r *mockRequest) Body() io.ReadCloser           { return nil }
func (r *mockRequest) Context() context.Context      { return context.Background() }