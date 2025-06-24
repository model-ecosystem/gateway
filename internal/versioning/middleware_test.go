package versioning

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"gateway/internal/core"
)

func TestVersioningMiddleware(t *testing.T) {
	config := &Config{
		Enabled:        true,
		Strategy:       StrategyPath,
		DefaultVersion: "1.0",
		DeprecatedVersions: map[string]DeprecationInfo{
			"1.0": {
				Message:    "Version 1.0 is deprecated",
				SunsetDate: time.Now().Add(30 * 24 * time.Hour),
			},
		},
		VersionMappings: map[string]VersionMapping{
			"2.0": {
				PathPrefix: "/api/v2",
			},
		},
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	middleware := NewMiddleware(manager, nil)

	tests := []struct {
		name               string
		request            core.Request
		expectError        bool
		expectedPath       string
		expectedHeaders    map[string]string
		checkDeprecation   bool
		disallowedVersion  string
	}{
		{
			name: "extract version from path",
			request: &mockRequest{
				path: "/v2/users/123",
			},
			expectedPath: "/users/123",
			expectedHeaders: map[string]string{
				"X-API-Version": "2",
			},
		},
		{
			name: "deprecated version",
			request: &mockRequest{
				path: "/v1.0/users/123",
			},
			expectedPath: "/users/123",
			expectedHeaders: map[string]string{
				"X-API-Version":              "1.0",
				"X-API-Deprecated":           "true",
				"X-API-Deprecation-Message":  "Version 1.0 is deprecated",
			},
			checkDeprecation: true,
		},
		{
			name: "default version",
			request: &mockRequest{
				path: "/users/123",
			},
			expectedPath: "/users/123",
			expectedHeaders: map[string]string{
				"X-API-Version": "1.0",
			},
		},
		{
			name: "disallowed version",
			request: &mockRequest{
				path: "/v0.9/users/123",
			},
			expectError:       true,
			disallowedVersion: "0.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up disallowed version if needed
			if tt.disallowedVersion != "" {
				manager.config.DeprecatedVersions[tt.disallowedVersion] = DeprecationInfo{
					RemovalDate: time.Now().Add(-1 * time.Hour), // Past date
				}
			}

			// Create handler
			called := false
			handler := middleware.Middleware()(func(ctx context.Context, req core.Request) (core.Response, error) {
				called = true
				
				// Check transformed path
				if req.Path() != tt.expectedPath {
					t.Errorf("Expected path %s, got %s", tt.expectedPath, req.Path())
				}

				// Check version in context
				if version, ok := GetVersion(ctx); ok {
					if version == "" {
						t.Error("Expected version in context")
					}
				}

				return &mockResponse{
					statusCode: 200,
					headers:    make(map[string][]string),
				}, nil
			})

			// Execute middleware
			resp, err := handler(context.Background(), tt.request)

			// Check error
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if !called {
					// This is expected for disallowed versions
					return
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && resp != nil {
				// Check response headers
				respHeaders := resp.Headers()
				for key, expectedValue := range tt.expectedHeaders {
					if values, ok := respHeaders[key]; ok {
						if len(values) == 0 || values[0] != expectedValue {
							t.Errorf("Expected header %s=%s, got %v", key, expectedValue, values)
						}
					} else {
						t.Errorf("Missing expected header %s", key)
					}
				}

				// Check deprecation headers
				if tt.checkDeprecation {
					if sunset, ok := respHeaders["Sunset"]; ok {
						if len(sunset) == 0 {
							t.Error("Expected Sunset header to have value")
						}
					} else {
						t.Error("Missing Sunset header for deprecated version")
					}
				}
			}

			// Clean up
			if tt.disallowedVersion != "" {
				delete(manager.config.DeprecatedVersions, tt.disallowedVersion)
			}
		})
	}
}

func TestRouteAwareMiddleware(t *testing.T) {
	config := &Config{
		Enabled:        true,
		Strategy:       StrategyHeader,
		DefaultVersion: "1.0",
		VersionHeader:  "X-API-Version",
		VersionMappings: map[string]VersionMapping{
			"2.0": {
				Service: "api-v2",
				Transformations: map[string]interface{}{
					"rename_field": "user_name",
				},
			},
		},
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	middleware := RouteAwareMiddleware(manager, nil)

	// Test with route in context
	route := &core.RouteResult{
		Rule: &core.RouteRule{
			ServiceName: "api-service",
			Metadata:    make(map[string]interface{}),
		},
		Instance: &core.ServiceInstance{
			ID:      "inst-1",
			Address: "localhost",
			Port:    8080,
		},
	}

	ctx := context.WithValue(context.Background(), routeContextKey{}, route)

	request := &mockRequest{
		headers: map[string][]string{
			"X-API-Version": {"2.0"},
		},
		path: "/users/123",
	}

	// Create handler
	handler := middleware(func(ctx context.Context, req core.Request) (core.Response, error) {
		// Check if route was modified
		if modifiedRoute := getRouteFromContext(ctx); modifiedRoute != nil {
			if modifiedRoute.Rule.ServiceName != "api-v2" {
				t.Errorf("Expected service name api-v2, got %s", modifiedRoute.Rule.ServiceName)
			}

			// Check metadata
			if modifiedRoute.Rule.Metadata["apiVersion"] != "2.0" {
				t.Error("Expected apiVersion in metadata")
			}
			if modifiedRoute.Rule.Metadata["originalService"] != "api-service" {
				t.Error("Expected originalService in metadata")
			}
			if modifiedRoute.Rule.Metadata["transformations"] == nil {
				t.Error("Expected transformations in metadata")
			}
		} else {
			t.Error("Expected modified route in context")
		}

		return &mockResponse{
			statusCode: 200,
			headers:    make(map[string][]string),
		}, nil
	})

	// Execute middleware
	resp, err := handler(ctx, request)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check version headers
	if resp != nil {
		headers := resp.Headers()
		if headers["X-API-Version"][0] != "2.0" {
			t.Error("Expected X-API-Version header")
		}
	}
}

func TestVersionedRequest(t *testing.T) {
	original := &mockRequest{
		id:         "req-123",
		method:     "GET",
		path:       "/v2/users/123",
		url:        "/v2/users/123?format=json",
		remoteAddr: "192.168.1.100",
		headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	}

	versioned := &versionedRequest{
		original: original,
		version:  "2.0",
		path:     "/users/123", // Transformed path
	}

	// Test delegated methods
	if versioned.ID() != original.ID() {
		t.Error("ID should be delegated")
	}
	if versioned.Method() != original.Method() {
		t.Error("Method should be delegated")
	}
	if versioned.Path() != "/users/123" {
		t.Error("Path should be transformed")
	}
	if versioned.URL() != "/users/123?format=json" {
		t.Error("URL should have transformed path")
	}
	if versioned.RemoteAddr() != original.RemoteAddr() {
		t.Error("RemoteAddr should be delegated")
	}
	if len(versioned.Headers()) != len(original.Headers()) {
		t.Error("Headers should be delegated")
	}
}

func TestVersionedResponse(t *testing.T) {
	original := &mockResponse{
		statusCode: 200,
		headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	}

	versioned := &versionedResponse{
		original: original,
		versionHeaders: map[string]string{
			"X-API-Version":    "2.0",
			"X-API-Deprecated": "true",
		},
	}

	// Test status code delegation
	if versioned.StatusCode() != original.StatusCode() {
		t.Error("StatusCode should be delegated")
	}

	// Test headers merge
	headers := versioned.Headers()
	if headers["Content-Type"][0] != "application/json" {
		t.Error("Original headers should be preserved")
	}
	if headers["X-API-Version"][0] != "2.0" {
		t.Error("Version headers should be added")
	}
	if headers["X-API-Deprecated"][0] != "true" {
		t.Error("Deprecation headers should be added")
	}
}

// Mock response for testing
type mockResponse struct {
	statusCode int
	headers    map[string][]string
	body       string
}

func (r *mockResponse) StatusCode() int               { return r.statusCode }
func (r *mockResponse) Headers() map[string][]string  { return r.headers }
func (r *mockResponse) Body() io.ReadCloser {
	return io.NopCloser(strings.NewReader(r.body))
}