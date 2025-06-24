package versioning

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"gateway/internal/config"
)

func TestVersioningMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.VersioningConfig
		request        func() *http.Request
		expectedVersion string
		expectedHeaders map[string]string
	}{
		{
			name: "path versioning",
			config: &config.VersioningConfig{
				Enabled:        true,
				Strategy:       "path",
				DefaultVersion: "1.0",
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/v2/users", nil)
			},
			expectedVersion: "2",
			expectedHeaders: map[string]string{
				"X-API-Version": "2",
			},
		},
		{
			name: "header versioning",
			config: &config.VersioningConfig{
				Enabled:        true,
				Strategy:       "header",
				DefaultVersion: "1.0",
				VersionHeader:  "X-API-Version",
			},
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/users", nil)
				req.Header.Set("X-API-Version", "2.0")
				return req
			},
			expectedVersion: "2.0",
			expectedHeaders: map[string]string{
				"X-API-Version": "2.0",
			},
		},
		{
			name: "query versioning",
			config: &config.VersioningConfig{
				Enabled:        true,
				Strategy:       "query",
				DefaultVersion: "1.0",
				VersionQuery:   "version",
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/users?version=3.0", nil)
			},
			expectedVersion: "3.0",
			expectedHeaders: map[string]string{
				"X-API-Version": "3.0",
			},
		},
		{
			name: "default version",
			config: &config.VersioningConfig{
				Enabled:        true,
				Strategy:       "path",
				DefaultVersion: "1.0",
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/users", nil)
			},
			expectedVersion: "1.0",
			expectedHeaders: map[string]string{
				"X-API-Version": "1.0",
			},
		},
		{
			name: "deprecated version",
			config: &config.VersioningConfig{
				Enabled:        true,
				Strategy:       "path",
				DefaultVersion: "1.0",
				DeprecatedVersions: map[string]*config.DeprecationInfo{
					"1.0": {
						Message:    "Version 1.0 is deprecated",
						SunsetDate: "2025-06-01T00:00:00Z",
					},
				},
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/v1.0/users", nil)
			},
			expectedVersion: "1.0",
			expectedHeaders: map[string]string{
				"X-API-Version":               "1.0",
				"X-API-Deprecated":            "true",
				"X-API-Deprecation-Message":   "Version 1.0 is deprecated",
				"Sunset":                      "Sun, 01 Jun 2025 00:00:00 UTC",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewVersioningMiddleware(tt.config, slog.Default())

			// Create a test handler that captures the context
			var capturedCtx context.Context
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedCtx = r.Context()
				w.WriteHeader(http.StatusOK)
			})

			// Apply middleware
			wrapped := middleware.Middleware(handler)

			// Create response recorder
			rec := httptest.NewRecorder()

			// Execute request
			wrapped.ServeHTTP(rec, tt.request())

			// Check version in context
			version := GetVersionFromContext(capturedCtx)
			if version != tt.expectedVersion {
				t.Errorf("Expected version %s, got %s", tt.expectedVersion, version)
			}

			// Check response headers
			for header, expected := range tt.expectedHeaders {
				actual := rec.Header().Get(header)
				if actual != expected {
					t.Errorf("Expected header %s=%s, got %s", header, expected, actual)
				}
			}
		})
	}
}

func TestVersionMapping(t *testing.T) {
	config := &config.VersioningConfig{
		Enabled:        true,
		Strategy:       "path",
		DefaultVersion: "1.0",
		VersionMappings: map[string]*config.VersionMapping{
			"2.0": {
				Service:    "api-v2",
				PathPrefix: "/api",
			},
		},
	}

	middleware := NewVersioningMiddleware(config, slog.Default())

	var capturedCtx context.Context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.Middleware(handler)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v2.0/users", nil)

	wrapped.ServeHTTP(rec, req)

	// Check service override
	serviceOverride := GetServiceOverrideFromContext(capturedCtx)
	if serviceOverride != "api-v2" {
		t.Errorf("Expected service override api-v2, got %s", serviceOverride)
	}

	// Check path modification
	expectedPath := "/api/v2.0/users"
	if req.URL.Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, req.URL.Path)
	}
}

func TestDisabledVersioning(t *testing.T) {
	config := &config.VersioningConfig{
		Enabled: false,
	}

	middleware := NewVersioningMiddleware(config, slog.Default())

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.Middleware(handler)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v2/users", nil)

	wrapped.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler was not called")
	}

	// Should not add version header when disabled
	if rec.Header().Get("X-API-Version") != "" {
		t.Error("Version header should not be added when versioning is disabled")
	}
}