package management

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
)

// Mock implementations
type mockRegistry struct{}

func (m *mockRegistry) GetService(name string) ([]core.ServiceInstance, error) {
	return []core.ServiceInstance{
		{ID: "test-1", Name: name, Address: "127.0.0.1", Port: 3000, Healthy: true},
	}, nil
}

func (m *mockRegistry) Close() error { return nil }

type mockRouter struct{}

func (m *mockRouter) GetRoutes() []core.RouteRule {
	return []core.RouteRule{
		{ID: "test-route", Path: "/test/*", ServiceName: "test-service"},
	}
}

func TestManagementAPI_Health(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, logger)
	
	// Test health endpoint
	req := httptest.NewRequest(http.MethodGet, "/management/health", nil)
	w := httptest.NewRecorder()
	
	api.handleHealth(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	
	if resp.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", resp.Status)
	}
}

func TestManagementAPI_Routes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, logger)
	
	// Set mock router
	api.SetRouter(&mockRouter{})
	
	// Test routes endpoint
	req := httptest.NewRequest(http.MethodGet, "/management/routes", nil)
	w := httptest.NewRecorder()
	
	api.handleRoutes(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var resp RouteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	
	if len(resp.Routes) != 1 {
		t.Errorf("Expected 1 route, got %d", len(resp.Routes))
	}
}

func TestManagementAPI_Auth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	
	tests := []struct {
		name       string
		config     *config.Management
		authHeader string
		wantStatus int
	}{
		{
			name: "no auth",
			config: &config.Management{
				Enabled: true,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "token auth success",
			config: &config.Management{
				Enabled: true,
				Auth: &config.ManagementAuth{
					Type:  "token",
					Token: "secret123",
				},
			},
			authHeader: "Bearer secret123",
			wantStatus: http.StatusOK,
		},
		{
			name: "token auth failure",
			config: &config.Management{
				Enabled: true,
				Auth: &config.ManagementAuth{
					Type:  "token",
					Token: "secret123",
				},
			},
			authHeader: "Bearer wrong",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "basic auth success",
			config: &config.Management{
				Enabled: true,
				Auth: &config.ManagementAuth{
					Type: "basic",
					Users: map[string]string{
						"admin": "pass123",
					},
				},
			},
			authHeader: "Basic YWRtaW46cGFzczEyMw==", // admin:pass123
			wantStatus: http.StatusOK,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewAPI(tt.config, logger)
			
			// Create request with auth
			req := httptest.NewRequest(http.MethodGet, "/management/health", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			
			// Apply auth middleware
			handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			
			handler.ServeHTTP(w, req)
			
			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestManagementAPI_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &config.Management{
		Enabled: true,
		Host:    "127.0.0.1",
		Port:    19090, // Use high port to avoid conflicts
	}
	
	api := NewAPI(config, logger)
	
	// Start API
	ctx := context.Background()
	if err := api.Start(ctx); err != nil {
		t.Fatal(err)
	}
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Test that it's running
	resp, err := http.Get("http://127.0.0.1:19090/management/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	
	// Stop API
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := api.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	
	// Verify it's stopped
	_, err = http.Get("http://127.0.0.1:19090/management/health")
	if err == nil {
		t.Error("Expected error after stop, but request succeeded")
	}
}