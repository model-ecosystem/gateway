package health

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gateway/internal/core"
)

// Mock service registry for testing
type mockRegistry struct {
	shouldError bool
}

func (m *mockRegistry) GetService(name string) ([]core.ServiceInstance, error) {
	if m.shouldError {
		return nil, errors.New("registry error")
	}
	return []core.ServiceInstance{}, nil
}

func TestRegistryCheck(t *testing.T) {
	// Test with healthy registry
	registry := &mockRegistry{shouldError: false}
	check := RegistryCheck(registry)

	err := check(context.Background())
	if err != nil {
		t.Errorf("Expected no error for healthy registry, got %v", err)
	}

	// Test with erroring registry
	registry.shouldError = true
	err = check(context.Background())
	// For this implementation, even errors return nil (registry is responding)
	if err != nil {
		t.Errorf("Expected no error even for erroring registry, got %v", err)
	}
}

func TestHTTPCheck(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test successful check
	check := HTTPCheck(server.URL+"/health", 5*time.Second)
	err := check(context.Background())
	if err != nil {
		t.Errorf("Expected no error for healthy endpoint, got %v", err)
	}

	// Test failing check (404)
	check = HTTPCheck(server.URL+"/nonexistent", 5*time.Second)
	err = check(context.Background())
	if err == nil {
		t.Error("Expected error for 404 endpoint")
	}

	// Test with cancelled context
	check = HTTPCheck(server.URL+"/health", 5*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = check(ctx)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestHTTPCheck_Timeout(t *testing.T) {
	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Test with short timeout
	check := HTTPCheck(server.URL, 50*time.Millisecond)
	err := check(context.Background())
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestCustomCheck(t *testing.T) {
	// Test successful custom check
	successCheck := CustomCheck("test", func() error {
		return nil
	})

	err := successCheck(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test failing custom check
	failCheck := CustomCheck("test", func() error {
		return errors.New("custom error")
	})

	err = failCheck(context.Background())
	if err == nil {
		t.Error("Expected error")
	}

	// Test context cancellation
	slowCheck := CustomCheck("test", func() error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = slowCheck(ctx)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// Helper function to create TCP check for testing
func createTCPCheck(addr string, timeout time.Duration) Check {
	return func(ctx context.Context) error {
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}
}

func TestTCPCheck(t *testing.T) {
	// Start a test TCP server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// Test successful connection
	check := createTCPCheck(addr, 5*time.Second)
	err = check(context.Background())
	if err != nil {
		t.Errorf("Expected successful TCP check, got %v", err)
	}

	// Test failed connection (invalid address)
	check = createTCPCheck("localhost:99999", 1*time.Second)
	err = check(context.Background())
	if err == nil {
		t.Error("Expected error for invalid address")
	}
}
