package loader

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileSource_Load(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	
	content := `gateway:
  host: localhost
  port: 8080`
	
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test successful load
	source := NewFileSource()
	reader, err := source.Load(context.Background(), testFile)
	if err != nil {
		t.Fatalf("Failed to load file: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content %s, got %s", content, data)
	}

	// Test non-existent file
	_, err = source.Load(context.Background(), "/non/existent/file.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestFileSource_Type(t *testing.T) {
	source := NewFileSource()
	if typ := source.Type(); typ != "file" {
		t.Errorf("Expected type 'file', got %s", typ)
	}
}

func TestHTTPSource_Load(t *testing.T) {
	// Create test server
	content := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0`
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spec.yaml":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(content))
		case "/not-found":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		case "/timeout":
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Test successful load
	config := &HTTPSourceConfig{
		Timeout: 1 * time.Second,
		Headers: map[string]string{
			"User-Agent": "gateway-test",
		},
	}
	source := NewHTTPSource(config)

	reader, err := source.Load(context.Background(), server.URL+"/spec.yaml")
	if err != nil {
		t.Fatalf("Failed to load HTTP resource: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content %s, got %s", content, data)
	}

	// Test 404 response
	_, err = source.Load(context.Background(), server.URL+"/not-found")
	if err == nil {
		t.Error("Expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected error to contain '404', got: %v", err)
	}

	// Test timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	_, err = source.Load(ctx, server.URL+"/timeout")
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestHTTPSource_Type(t *testing.T) {
	source := NewHTTPSource(nil)
	if typ := source.Type(); typ != "http" {
		t.Errorf("Expected type 'http', got %s", typ)
	}
}

func TestHTTPSource_DefaultConfig(t *testing.T) {
	// Test with nil config - should use defaults
	source := NewHTTPSource(nil)
	if source.client.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", source.client.Timeout)
	}
}

func TestHTTPSource_Headers(t *testing.T) {
	// Test that custom headers are sent
	receivedHeaders := make(map[string]string)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders["User-Agent"] = r.Header.Get("User-Agent")
		receivedHeaders["X-Custom"] = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := &HTTPSourceConfig{
		Headers: map[string]string{
			"User-Agent": "test-agent",
			"X-Custom":   "custom-value",
		},
	}
	source := NewHTTPSource(config)

	reader, err := source.Load(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}
	reader.Close()

	if receivedHeaders["User-Agent"] != "test-agent" {
		t.Errorf("Expected User-Agent 'test-agent', got %s", receivedHeaders["User-Agent"])
	}
	if receivedHeaders["X-Custom"] != "custom-value" {
		t.Errorf("Expected X-Custom 'custom-value', got %s", receivedHeaders["X-Custom"])
	}
}

func TestHTTPSource_ContextCancellation(t *testing.T) {
	// Test that context cancellation works
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	source := NewHTTPSource(nil)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel immediately
	cancel()
	
	_, err := source.Load(ctx, server.URL)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}