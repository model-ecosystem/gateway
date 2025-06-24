package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPCheck(t *testing.T) {
	tests := []struct {
		name         string
		setupServer  func() *httptest.Server
		timeout      time.Duration
		expectError  bool
		errorContains string
	}{
		{
			name: "healthy endpoint",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			timeout:     5 * time.Second,
			expectError: false,
		},
		{
			name: "unhealthy endpoint (400)",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
				}))
			},
			timeout:      5 * time.Second,
			expectError:  true,
			errorContains: "unhealthy status: 400",
		},
		{
			name: "unhealthy endpoint (500)",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			timeout:      5 * time.Second,
			expectError:  true,
			errorContains: "unhealthy status: 500",
		},
		{
			name: "timeout",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(100 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				}))
			},
			timeout:      10 * time.Millisecond,
			expectError:  true,
			errorContains: "request failed:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			check := HTTPCheck(server.URL, tt.timeout)
			err := check(context.Background())

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestHTTPCheck_InvalidURL(t *testing.T) {
	check := HTTPCheck("://invalid-url", time.Second)
	err := check(context.Background())
	
	if err == nil {
		t.Error("expected error for invalid URL")
	}
	if !contains(err.Error(), "creating request:") {
		t.Errorf("expected 'creating request:' error, got: %v", err)
	}
}

func TestHTTPCheck_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	check := HTTPCheck(server.URL, 5*time.Second)
	err := check(ctx)

	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestDatabaseCheck(t *testing.T) {
	tests := []struct {
		name        string
		pingFunc    func(context.Context) error
		expectError bool
	}{
		{
			name: "healthy database",
			pingFunc: func(ctx context.Context) error {
				return nil
			},
			expectError: false,
		},
		{
			name: "unhealthy database",
			pingFunc: func(ctx context.Context) error {
				return errors.New("connection refused")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := DatabaseCheck(tt.pingFunc)
			err := check(context.Background())

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDiskSpaceCheck(t *testing.T) {
	// This is a placeholder test since the actual implementation is also a placeholder
	check := DiskSpaceCheck("/tmp", 1<<30) // 1GB
	err := check(context.Background())
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMemoryCheck(t *testing.T) {
	// This is a placeholder test since the actual implementation is also a placeholder
	check := MemoryCheck(80.0) // 80% max usage
	err := check(context.Background())
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCustomCheck(t *testing.T) {
	tests := []struct {
		name        string
		checkFunc   func() error
		expectError bool
		errorContains string
	}{
		{
			name: "successful check",
			checkFunc: func() error {
				return nil
			},
			expectError: false,
		},
		{
			name: "failing check",
			checkFunc: func() error {
				return errors.New("custom check failed")
			},
			expectError: true,
			errorContains: "custom check failed",
		},
		{
			name: "slow check with context timeout",
			checkFunc: func() error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			expectError: true,
			errorContains: "check timeout:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := CustomCheck("test-check", tt.checkFunc)
			
			ctx := context.Background()
			if tt.name == "slow check with context timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
				defer cancel()
			}
			
			err := check(ctx)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCustomCheck_ImmediateContextCancellation(t *testing.T) {
	called := false
	checkFunc := func() error {
		called = true
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	check := CustomCheck("test-check", checkFunc)
	err := check(ctx)

	if err == nil {
		t.Error("expected error for cancelled context")
	}
	
	// Give some time for goroutine to potentially execute
	time.Sleep(10 * time.Millisecond)
	
	// The check function should not have been called due to immediate cancellation
	if called {
		t.Error("check function should not have been called with cancelled context")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || (len(s) > len(substr) && contains(s[1:], substr)))
}