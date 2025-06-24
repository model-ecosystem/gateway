package loader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// K8sConfigMapSource loads content from Kubernetes ConfigMaps
type K8sConfigMapSource struct {
	client    *http.Client
	namespace string
	token     string
	apiServer string
}

// K8sConfigMapSourceConfig configures a Kubernetes ConfigMap source
type K8sConfigMapSourceConfig struct {
	Namespace string
	Token     string
	APIServer string
	Timeout   time.Duration
}

// NewK8sConfigMapSource creates a new Kubernetes ConfigMap source
func NewK8sConfigMapSource(config *K8sConfigMapSourceConfig) (*K8sConfigMapSource, error) {
	if config == nil {
		config = &K8sConfigMapSourceConfig{}
	}
	
	// Set defaults
	if config.Namespace == "" {
		// Try to read namespace from service account
		nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err == nil {
			config.Namespace = string(nsBytes)
		} else {
			config.Namespace = "default"
		}
	}
	
	if config.Token == "" {
		// Try to read token from service account
		tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err == nil {
			config.Token = string(tokenBytes)
		}
	}
	
	if config.APIServer == "" {
		// Use in-cluster configuration
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port := os.Getenv("KUBERNETES_SERVICE_PORT")
		if host != "" && port != "" {
			config.APIServer = fmt.Sprintf("https://%s:%s", host, port)
		} else {
			config.APIServer = "https://kubernetes.default.svc"
		}
	}
	
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	
	// Create HTTP client with CA cert
	client := &http.Client{
		Timeout: config.Timeout,
	}
	
	// Try to use service account CA cert
	if caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"); err == nil {
		// In production, you would configure TLS properly here
		// For now, we'll use the default client
		_ = caCert
	}
	
	return &K8sConfigMapSource{
		client:    client,
		namespace: config.Namespace,
		token:     config.Token,
		apiServer: config.APIServer,
	}, nil
}

// Load loads content from a ConfigMap
// The path format is: configmap-name/key
func (s *K8sConfigMapSource) Load(ctx context.Context, path string) (io.ReadCloser, error) {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid path format, expected: configmap-name/key")
	}
	
	configMapName := parts[0]
	key := parts[1]
	
	// Construct API URL
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps/%s",
		s.apiServer, s.namespace, configMapName)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Add authorization header
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ConfigMap: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get ConfigMap: %s", resp.Status)
	}
	
	// Parse ConfigMap
	var configMap struct {
		Data map[string]string `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&configMap); err != nil {
		return nil, fmt.Errorf("failed to decode ConfigMap: %w", err)
	}
	
	// Get the requested key
	content, ok := configMap.Data[key]
	if !ok {
		return nil, fmt.Errorf("key %s not found in ConfigMap %s", key, configMapName)
	}
	
	return io.NopCloser(strings.NewReader(content)), nil
}

// Type returns the source type
func (s *K8sConfigMapSource) Type() string {
	return "k8s-configmap"
}

// WatchableSource extends Source with watch capabilities
type WatchableSource interface {
	Source
	// Watch watches for changes to the source
	Watch(ctx context.Context, path string, onChange func()) error
}

// K8sConfigMapWatcher watches ConfigMap changes
type K8sConfigMapWatcher struct {
	*K8sConfigMapSource
}

// NewK8sConfigMapWatcher creates a new watcher
func NewK8sConfigMapWatcher(config *K8sConfigMapSourceConfig) (*K8sConfigMapWatcher, error) {
	source, err := NewK8sConfigMapSource(config)
	if err != nil {
		return nil, err
	}
	return &K8sConfigMapWatcher{source}, nil
}

// Watch watches for ConfigMap changes
func (w *K8sConfigMapWatcher) Watch(ctx context.Context, path string, onChange func()) error {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid path format, expected: configmap-name/key")
	}
	
	configMapName := parts[0]
	
	// In a real implementation, this would use the Kubernetes watch API
	// For now, we'll poll every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	var lastResourceVersion string
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Check ConfigMap resource version
			url := fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps/%s",
				w.apiServer, w.namespace, configMapName)
			
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				continue
			}
			
			if w.token != "" {
				req.Header.Set("Authorization", "Bearer "+w.token)
			}
			
			resp, err := w.client.Do(req)
			if err != nil {
				continue
			}
			
			if resp.StatusCode == http.StatusOK {
				var cm struct {
					Metadata struct {
						ResourceVersion string `json:"resourceVersion"`
					} `json:"metadata"`
				}
				
				if err := json.NewDecoder(resp.Body).Decode(&cm); err == nil {
					if lastResourceVersion != "" && lastResourceVersion != cm.Metadata.ResourceVersion {
						onChange()
					}
					lastResourceVersion = cm.Metadata.ResourceVersion
				}
			}
			resp.Body.Close()
		}
	}
}

// SourceRegistry manages different source types
type SourceRegistry struct {
	sources map[string]Source
}

// NewSourceRegistry creates a new source registry
func NewSourceRegistry() *SourceRegistry {
	return &SourceRegistry{
		sources: make(map[string]Source),
	}
}

// Register registers a source with a scheme
func (r *SourceRegistry) Register(scheme string, source Source) {
	r.sources[scheme] = source
}

// Get gets a source by scheme
func (r *SourceRegistry) Get(scheme string) (Source, bool) {
	source, ok := r.sources[scheme]
	return source, ok
}

// Load loads content from a URI
// URI format: scheme://path
// Examples:
//   - file:///path/to/file
//   - http://example.com/spec.yaml
//   - k8s://configmap-name/key
func (r *SourceRegistry) Load(ctx context.Context, uri string) (io.ReadCloser, error) {
	parts := strings.SplitN(uri, "://", 2)
	if len(parts) != 2 {
		// Default to file source for backward compatibility
		source, ok := r.sources["file"]
		if !ok {
			return nil, fmt.Errorf("file source not registered")
		}
		return source.Load(ctx, uri)
	}
	
	scheme := parts[0]
	path := parts[1]
	
	source, ok := r.sources[scheme]
	if !ok {
		return nil, fmt.Errorf("unknown source scheme: %s", scheme)
	}
	
	return source.Load(ctx, path)
}

// DefaultSourceRegistry creates a registry with default sources
func DefaultSourceRegistry() *SourceRegistry {
	registry := NewSourceRegistry()
	
	// Register file source
	registry.Register("file", NewFileSource())
	
	// Register HTTP/HTTPS sources
	httpSource := NewHTTPSource(&HTTPSourceConfig{
		Timeout: 30 * time.Second,
	})
	registry.Register("http", httpSource)
	registry.Register("https", httpSource)
	
	// Register K8s ConfigMap source (if in cluster)
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount"); err == nil {
		if k8sSource, err := NewK8sConfigMapSource(nil); err == nil {
			registry.Register("k8s", k8sSource)
			registry.Register("configmap", k8sSource)
		}
	}
	
	return registry
}

// SourceConfig represents configuration for a source
type SourceConfig struct {
	Type    string                 `yaml:"type"`
	Config  map[string]interface{} `yaml:"config"`
}

// MultiSourceLoader loads from multiple sources with fallback
type MultiSourceLoader struct {
	sources []Source
}

// NewMultiSourceLoader creates a new multi-source loader
func NewMultiSourceLoader(sources ...Source) *MultiSourceLoader {
	return &MultiSourceLoader{
		sources: sources,
	}
}

// Load tries to load from sources in order until one succeeds
func (l *MultiSourceLoader) Load(ctx context.Context, path string) (io.ReadCloser, error) {
	var lastErr error
	for _, source := range l.sources {
		reader, err := source.Load(ctx, path)
		if err == nil {
			return reader, nil
		}
		lastErr = err
	}
	
	if lastErr != nil {
		return nil, fmt.Errorf("all sources failed, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("no sources configured")
}

// Type returns the source type
func (l *MultiSourceLoader) Type() string {
	return "multi"
}

// CachedSource wraps a source with caching
type CachedSource struct {
	source    Source
	cache     map[string][]byte
	cacheTTL  time.Duration
	cacheTime map[string]time.Time
}

// NewCachedSource creates a cached source
func NewCachedSource(source Source, ttl time.Duration) *CachedSource {
	return &CachedSource{
		source:    source,
		cache:     make(map[string][]byte),
		cacheTime: make(map[string]time.Time),
		cacheTTL:  ttl,
	}
}

// Load loads with caching
func (s *CachedSource) Load(ctx context.Context, path string) (io.ReadCloser, error) {
	// Check cache
	if data, ok := s.cache[path]; ok {
		if time.Since(s.cacheTime[path]) < s.cacheTTL {
			return io.NopCloser(bytes.NewReader(data)), nil
		}
	}
	
	// Load from source
	reader, err := s.source.Load(ctx, path)
	if err != nil {
		return nil, err
	}
	
	// Read and cache
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		return nil, err
	}
	
	s.cache[path] = data
	s.cacheTime[path] = time.Now()
	
	return io.NopCloser(bytes.NewReader(data)), nil
}

// Type returns the source type
func (s *CachedSource) Type() string {
	return fmt.Sprintf("cached-%s", s.source.Type())
}