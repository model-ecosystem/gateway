package loader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewK8sConfigMapSource(t *testing.T) {
	tests := []struct {
		name   string
		config *K8sConfigMapSourceConfig
		envs   map[string]string
		files  map[string]string
	}{
		{
			name:   "nil config with defaults",
			config: nil,
			envs: map[string]string{
				"KUBERNETES_SERVICE_HOST": "10.0.0.1",
				"KUBERNETES_SERVICE_PORT": "443",
			},
		},
		{
			name: "custom config",
			config: &K8sConfigMapSourceConfig{
				Namespace: "custom-ns",
				Token:     "custom-token",
				APIServer: "https://custom.k8s.local",
				Timeout:   10 * time.Second,
			},
		},
		{
			name:   "config from service account",
			config: nil,
			files: map[string]string{
				"namespace": "kube-system",
				"token":     "sa-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.envs {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Create temp service account directory if needed
			var tmpDir string
			if len(tt.files) > 0 {
				tmpDir = t.TempDir()
				saDir := filepath.Join(tmpDir, "var", "run", "secrets", "kubernetes.io", "serviceaccount")
				os.MkdirAll(saDir, 0755)

				// Create symlink from expected path to temp path
				os.Symlink(saDir, "/var/run/secrets/kubernetes.io/serviceaccount")
				defer os.Remove("/var/run/secrets/kubernetes.io/serviceaccount")

				for file, content := range tt.files {
					os.WriteFile(filepath.Join(saDir, file), []byte(content), 0644)
				}
			}

			source, err := NewK8sConfigMapSource(tt.config)
			if err != nil {
				t.Fatalf("Failed to create source: %v", err)
			}

			// Verify configuration
			if tt.config != nil {
				if tt.config.Namespace != "" && source.namespace != tt.config.Namespace {
					t.Errorf("Expected namespace %s, got %s", tt.config.Namespace, source.namespace)
				}
				if tt.config.Token != "" && source.token != tt.config.Token {
					t.Errorf("Expected token %s, got %s", tt.config.Token, source.token)
				}
				if tt.config.APIServer != "" && source.apiServer != tt.config.APIServer {
					t.Errorf("Expected API server %s, got %s", tt.config.APIServer, source.apiServer)
				}
			}
		})
	}
}

func TestK8sConfigMapSource_Load(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/api/v1/namespaces/default/configmaps/test-cm":
			configMap := map[string]interface{}{
				"data": map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			}
			json.NewEncoder(w).Encode(configMap)
		case "/api/v1/namespaces/default/configmaps/empty-cm":
			configMap := map[string]interface{}{
				"data": map[string]string{},
			}
			json.NewEncoder(w).Encode(configMap)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	source := &K8sConfigMapSource{
		client:    &http.Client{Timeout: 5 * time.Second},
		namespace: "default",
		token:     "test-token",
		apiServer: server.URL,
	}

	tests := []struct {
		name        string
		path        string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{
			name:     "valid configmap and key",
			path:     "test-cm/key1",
			expected: "value1",
		},
		{
			name:     "valid configmap different key",
			path:     "test-cm/key2",
			expected: "value2",
		},
		{
			name:        "invalid path format",
			path:        "invalid-format",
			expectError: true,
			errorMsg:    "invalid path format",
		},
		{
			name:        "non-existent configmap",
			path:        "missing-cm/key1",
			expectError: true,
			errorMsg:    "404",
		},
		{
			name:        "non-existent key",
			path:        "empty-cm/missing-key",
			expectError: true,
			errorMsg:    "key missing-key not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := source.Load(context.Background(), tt.path)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			defer reader.Close()

			data, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("Failed to read data: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Expected content %s, got %s", tt.expected, string(data))
			}
		})
	}
}

func TestK8sConfigMapSource_Type(t *testing.T) {
	source := &K8sConfigMapSource{}
	if typ := source.Type(); typ != "k8s-configmap" {
		t.Errorf("Expected type 'k8s-configmap', got %s", typ)
	}
}

func TestSourceRegistry(t *testing.T) {
	registry := NewSourceRegistry()

	// Register sources
	fileSource := NewFileSource()
	httpSource := NewHTTPSource(nil)
	registry.Register("file", fileSource)
	registry.Register("http", httpSource)

	// Test Get
	source, ok := registry.Get("file")
	if !ok {
		t.Error("Expected to find file source")
	}
	if source.Type() != "file" {
		t.Errorf("Expected file source, got %s", source.Type())
	}

	// Test unknown source
	_, ok = registry.Get("unknown")
	if ok {
		t.Error("Expected not to find unknown source")
	}
}

func TestSourceRegistry_Load(t *testing.T) {
	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	testContent := "test content"
	os.WriteFile(testFile, []byte(testContent), 0644)

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("http content"))
	}))
	defer server.Close()

	registry := NewSourceRegistry()
	registry.Register("file", NewFileSource())
	
	// Create HTTP source wrapper that adds back the scheme
	httpSource := NewHTTPSource(nil)
	httpWrapper := &mockSource{
		loadFunc: func(ctx context.Context, path string) (io.ReadCloser, error) {
			// Add back the http:// scheme
			return httpSource.Load(ctx, "http://"+path)
		},
		typ: "http",
	}
	registry.Register("http", httpWrapper)

	tests := []struct {
		name        string
		uri         string
		expected    string
		expectError bool
	}{
		{
			name:     "file URI",
			uri:      fmt.Sprintf("file://%s", testFile),
			expected: testContent,
		},
		{
			name:     "http URI",
			uri:      "http://" + strings.TrimPrefix(server.URL, "http://"),
			expected: "http content",
		},
		{
			name:     "backward compatibility - no scheme",
			uri:      testFile,
			expected: testContent,
		},
		{
			name:        "unknown scheme",
			uri:         "unknown://path",
			expectError: true,
		},
		{
			name:        "no file source for backward compatibility",
			uri:         testFile,
			expectError: true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the last test case, remove file source
			if i == len(tests)-1 {
				registry = NewSourceRegistry()
			}

			reader, err := registry.Load(context.Background(), tt.uri)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			defer reader.Close()

			data, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("Failed to read data: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Expected content %s, got %s", tt.expected, string(data))
			}
		})
	}
}

func TestMultiSourceLoader(t *testing.T) {
	// Create sources
	failingSource := &mockSource{
		loadFunc: func(ctx context.Context, path string) (io.ReadCloser, error) {
			return nil, fmt.Errorf("source 1 failed")
		},
		typ: "failing",
	}

	successSource := &mockSource{
		loadFunc: func(ctx context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("success")), nil
		},
		typ: "success",
	}

	loader := NewMultiSourceLoader(failingSource, successSource)

	// Test successful load (falls back to second source)
	reader, err := loader.Load(context.Background(), "test")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if string(data) != "success" {
		t.Errorf("Expected 'success', got %s", string(data))
	}

	// Test all sources fail
	loader2 := NewMultiSourceLoader(failingSource)
	_, err = loader2.Load(context.Background(), "test")
	if err == nil {
		t.Error("Expected error when all sources fail")
	}

	// Test no sources
	loader3 := NewMultiSourceLoader()
	_, err = loader3.Load(context.Background(), "test")
	if err == nil {
		t.Error("Expected error when no sources configured")
	}

	// Test Type
	if typ := loader.Type(); typ != "multi" {
		t.Errorf("Expected type 'multi', got %s", typ)
	}
}

func TestCachedSource(t *testing.T) {
	callCount := 0
	baseSource := &mockSource{
		loadFunc: func(ctx context.Context, path string) (io.ReadCloser, error) {
			callCount++
			content := fmt.Sprintf("content-%d", callCount)
			return io.NopCloser(strings.NewReader(content)), nil
		},
		typ: "base",
	}

	cached := NewCachedSource(baseSource, 100*time.Millisecond)

	// First load - should call base source
	reader1, err := cached.Load(context.Background(), "test")
	if err != nil {
		t.Fatalf("First load failed: %v", err)
	}
	data1, _ := io.ReadAll(reader1)
	reader1.Close()

	if string(data1) != "content-1" {
		t.Errorf("Expected 'content-1', got %s", string(data1))
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// Second load - should use cache
	reader2, err := cached.Load(context.Background(), "test")
	if err != nil {
		t.Fatalf("Second load failed: %v", err)
	}
	data2, _ := io.ReadAll(reader2)
	reader2.Close()

	if string(data2) != "content-1" {
		t.Errorf("Expected cached 'content-1', got %s", string(data2))
	}
	if callCount != 1 {
		t.Errorf("Expected still 1 call (cached), got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third load - should call base source again
	reader3, err := cached.Load(context.Background(), "test")
	if err != nil {
		t.Fatalf("Third load failed: %v", err)
	}
	data3, _ := io.ReadAll(reader3)
	reader3.Close()

	if string(data3) != "content-2" {
		t.Errorf("Expected new 'content-2', got %s", string(data3))
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls after cache expiry, got %d", callCount)
	}

	// Test Type
	if typ := cached.Type(); typ != "cached-base" {
		t.Errorf("Expected type 'cached-base', got %s", typ)
	}
}

func TestDefaultSourceRegistry(t *testing.T) {
	// Test without Kubernetes service account
	registry := DefaultSourceRegistry()

	// Should have file source
	if _, ok := registry.Get("file"); !ok {
		t.Error("Expected file source to be registered")
	}

	// Should have HTTP/HTTPS sources
	if _, ok := registry.Get("http"); !ok {
		t.Error("Expected http source to be registered")
	}
	if _, ok := registry.Get("https"); !ok {
		t.Error("Expected https source to be registered")
	}

	// K8s source registration depends on service account presence
	// We can't easily test this without mocking the filesystem
}

func TestK8sConfigMapWatcher_Watch(t *testing.T) {
	resourceVersion := "1000"
	changeCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Simulate version change after first request
		if changeCount > 0 {
			resourceVersion = "1001"
		}
		changeCount++

		response := map[string]interface{}{
			"metadata": map[string]interface{}{
				"resourceVersion": resourceVersion,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	watcher := &K8sConfigMapWatcher{
		&K8sConfigMapSource{
			client:    &http.Client{Timeout: 5 * time.Second},
			namespace: "default",
			token:     "test-token",
			apiServer: server.URL,
		},
	}

	// Test invalid path
	err := watcher.Watch(context.Background(), "invalid", func() {})
	if err == nil || !strings.Contains(err.Error(), "invalid path format") {
		t.Errorf("Expected invalid path format error, got: %v", err)
	}

	// Test watch with cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = watcher.Watch(ctx, "test-cm/key", func() {
		// Change callback - in real scenario this would be called
		// when resource version changes
	})

	if err != context.DeadlineExceeded {
		t.Errorf("Expected deadline exceeded error, got: %v", err)
	}
}

// mockSource is a test helper
type mockSource struct {
	loadFunc func(ctx context.Context, path string) (io.ReadCloser, error)
	typ      string
}

func (m *mockSource) Load(ctx context.Context, path string) (io.ReadCloser, error) {
	return m.loadFunc(ctx, path)
}

func (m *mockSource) Type() string {
	return m.typ
}