package auth_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"gateway/internal/core"
	"gateway/internal/middleware/auth"
	"gateway/internal/middleware/auth/apikey"
)

// mockRequest implements core.Request for testing
type mockRequest struct {
	path    string
	headers map[string][]string
}

func (r *mockRequest) ID() string                     { return "test-id" }
func (r *mockRequest) Method() string                 { return "GET" }
func (r *mockRequest) Path() string                   { return r.path }
func (r *mockRequest) URL() string                    { return "http://test" + r.path }
func (r *mockRequest) RemoteAddr() string             { return "127.0.0.1:12345" }
func (r *mockRequest) Headers() map[string][]string   { return r.headers }
func (r *mockRequest) Body() io.ReadCloser            { return nil }
func (r *mockRequest) Context() context.Context       { return context.Background() }

// mockResponse implements core.Response for testing
type mockResponse struct {
	statusCode int
	headers    map[string][]string
}

func (r *mockResponse) StatusCode() int              { return r.statusCode }
func (r *mockResponse) Headers() map[string][]string { return r.headers }
func (r *mockResponse) Body() io.ReadCloser          { return nil }

func TestAuthMiddleware(t *testing.T) {
	logger := slog.Default()

	// Create API key provider with test keys
	apiKeyConfig := &apikey.Config{
		Keys: map[string]*apikey.KeyConfig{
			"test-key": {
				Key:     "secret-key",
				Subject: "test-service",
				Type:    "service",
				Scopes:  []string{"api:read", "api:write"},
			},
		},
		HashKeys: false,
	}

	apiKeyProvider, err := apikey.NewProvider(apiKeyConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create API key provider: %v", err)
	}

	// Create auth middleware
	authConfig := &auth.Config{
		Required:       true,
		Providers:      []string{"apikey"},
		SkipPaths:      []string{"/public/"},
		RequiredScopes: []string{"api:read"},
	}

	middleware := auth.NewMiddleware(authConfig, logger)
	middleware.AddProvider(apiKeyProvider)
	middleware.AddExtractor(apikey.NewExtractor())

	// Create test handler
	testHandler := func(ctx context.Context, req core.Request) (core.Response, error) {
		// Check if auth info is in context
		authInfo, ok := auth.GetAuthInfo(ctx)
		if ok {
			return &mockResponse{
				statusCode: 200,
				headers: map[string][]string{
					"X-Auth-Subject": {authInfo.Subject},
				},
			}, nil
		}
		return &mockResponse{statusCode: 200}, nil
	}

	// Apply middleware
	handler := middleware.Handler(testHandler)

	tests := []struct {
		name           string
		path           string
		headers        map[string][]string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "No authentication",
			path:           "/api/test",
			headers:        map[string][]string{},
			expectedStatus: 0,
			expectError:    true,
		},
		{
			name: "Valid API key",
			path: "/api/test",
			headers: map[string][]string{
				"X-API-Key": {"secret-key"},
			},
			expectedStatus: 200,
			expectError:    false,
		},
		{
			name: "Invalid API key",
			path: "/api/test",
			headers: map[string][]string{
				"X-API-Key": {"wrong-key"},
			},
			expectedStatus: 0,
			expectError:    true,
		},
		{
			name:           "Skip path - no auth required",
			path:           "/public/resource",
			headers:        map[string][]string{},
			expectedStatus: 200,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &mockRequest{
				path:    tt.path,
				headers: tt.headers,
			}

			resp, err := handler(context.Background(), req)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp.StatusCode() != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode())
				}
			}
		})
	}
}

func TestAuthMiddleware_Scopes(t *testing.T) {
	logger := slog.Default()

	// Create API key provider with different scoped keys
	apiKeyConfig := &apikey.Config{
		Keys: map[string]*apikey.KeyConfig{
			"admin-key": {
				Key:     "admin-secret",
				Subject: "admin-service",
				Type:    "service",
				Scopes:  []string{"api:read", "api:write", "api:admin"},
			},
			"read-only-key": {
				Key:     "readonly-secret",
				Subject: "readonly-service",
				Type:    "service",
				Scopes:  []string{"api:read"},
			},
		},
		HashKeys: false,
	}

	apiKeyProvider, err := apikey.NewProvider(apiKeyConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create API key provider: %v", err)
	}

	// Create auth middleware requiring write scope
	authConfig := &auth.Config{
		Required:       true,
		Providers:      []string{"apikey"},
		RequiredScopes: []string{"api:write"},
	}

	middleware := auth.NewMiddleware(authConfig, logger)
	middleware.AddProvider(apiKeyProvider)
	middleware.AddExtractor(apikey.NewExtractor())

	// Create test handler
	testHandler := func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{statusCode: 200}, nil
	}

	// Apply middleware
	handler := middleware.Handler(testHandler)

	tests := []struct {
		name        string
		apiKey      string
		expectError bool
	}{
		{
			name:        "Admin key with write scope",
			apiKey:      "admin-secret",
			expectError: false,
		},
		{
			name:        "Read-only key without write scope",
			apiKey:      "readonly-secret",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &mockRequest{
				path: "/api/test",
				headers: map[string][]string{
					"X-API-Key": {tt.apiKey},
				},
			}

			_, err := handler(context.Background(), req)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for insufficient scopes, got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}