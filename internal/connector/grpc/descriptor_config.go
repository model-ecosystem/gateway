package grpc

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gateway/internal/loader"
)

// DescriptorConfig holds configuration for descriptor loading
type DescriptorConfig struct {
	// DescriptorFiles is a list of .desc files to load (legacy)
	DescriptorFiles []string `yaml:"descriptorFiles"`
	// DescriptorDirs is a list of directories to scan for .desc files (legacy)
	DescriptorDirs []string `yaml:"descriptorDirs"`
	// DescriptorSources configures multi-source descriptor loading
	DescriptorSources []SourceConfig `yaml:"descriptorSources"`
	// AutoReload enables automatic reloading of descriptors
	AutoReload bool `yaml:"autoReload"`
	// ReloadInterval is the interval for checking descriptor changes
	ReloadInterval time.Duration `yaml:"reloadInterval"`
	// FailOnError determines if the gateway should fail on descriptor loading errors
	FailOnError bool `yaml:"failOnError"`
}

// SourceConfig represents a descriptor source configuration
type SourceConfig struct {
	Type       string                 `yaml:"type"` // file, http, k8s-configmap
	Paths      []string               `yaml:"paths"`
	URLs       []string               `yaml:"urls"`
	Headers    map[string]string      `yaml:"headers"`
	Timeout    int                    `yaml:"timeout"` // seconds
	Namespace  string                 `yaml:"namespace"`
	ConfigMaps []ConfigMapSource      `yaml:"configMaps"`
}

// ConfigMapSource represents a Kubernetes ConfigMap source
type ConfigMapSource struct {
	Name string   `yaml:"name"`
	Keys []string `yaml:"keys"`
}

// DefaultDescriptorConfig returns default descriptor configuration
func DefaultDescriptorConfig() DescriptorConfig {
	return DescriptorConfig{
		DescriptorFiles: []string{},
		DescriptorDirs:  []string{},
		AutoReload:      false,
		ReloadInterval:  30 * time.Second,
		FailOnError:     false,
	}
}

// DescriptorManager manages dynamic loading of descriptors
type DescriptorManager struct {
	config  DescriptorConfig
	loader  *DescriptorLoader
	logger  *slog.Logger
	mu      sync.RWMutex
	stopCh  chan struct{}
	stopped bool
}

// NewDescriptorManager creates a new descriptor manager
func NewDescriptorManager(config DescriptorConfig, registry *ProtoRegistry, logger *slog.Logger) *DescriptorManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &DescriptorManager{
		config: config,
		loader: NewDescriptorLoader(registry),
		logger: logger.With("component", "descriptor_manager"),
		stopCh: make(chan struct{}),
	}
}

// Start starts the descriptor manager
func (m *DescriptorManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return fmt.Errorf("descriptor manager already stopped")
	}

	// Initial load
	if err := m.loadAll(); err != nil {
		if m.config.FailOnError {
			return err
		}
		m.logger.Error("Failed to load descriptors", "error", err)
	}

	// Enable file watching if auto-reload is enabled
	if m.config.AutoReload {
		// Set up reload callback
		m.loader.SetReloadCallback(func(path string) {
			m.logger.Info("Descriptor file reloaded", "file", path)
		})

		// Enable file watching
		if err := m.loader.EnableFileWatching(); err != nil {
			m.logger.Error("Failed to enable file watching", "error", err)
			// Fall back to interval-based reloading
			if m.config.ReloadInterval > 0 {
				go m.autoReloadLoop()
			}
		} else {
			m.logger.Info("File watching enabled for descriptors")
		}
	}

	return nil
}

// Stop stops the descriptor manager
func (m *DescriptorManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.stopped {
		// Disable file watching
		if err := m.loader.DisableFileWatching(); err != nil {
			m.logger.Error("Failed to disable file watching", "error", err)
		}

		close(m.stopCh)
		m.stopped = true
	}
}

// loadAll loads all configured descriptors
func (m *DescriptorManager) loadAll() error {
	var errors []error

	// Load from multi-source configuration if available
	if len(m.config.DescriptorSources) > 0 {
		if err := m.loadFromSources(); err != nil {
			errors = append(errors, err)
		}
	}

	// Load individual files (legacy support)
	for _, file := range m.config.DescriptorFiles {
		if err := m.loader.LoadDescriptorFile(file); err != nil {
			m.logger.Error("Failed to load descriptor file", 
				"file", file,
				"error", err)
			errors = append(errors, fmt.Errorf("file %s: %w", file, err))
		} else {
			m.logger.Info("Loaded descriptor file", "file", file)
		}
	}

	// Load directories (legacy support)
	for _, dir := range m.config.DescriptorDirs {
		if err := m.loader.LoadDescriptorDirectory(dir); err != nil {
			m.logger.Error("Failed to load descriptor directory",
				"directory", dir,
				"error", err)
			errors = append(errors, fmt.Errorf("directory %s: %w", dir, err))
		} else {
			m.logger.Info("Loaded descriptor directory", "directory", dir)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to load %d descriptors", len(errors))
	}

	return nil
}

// autoReloadLoop periodically reloads descriptors
func (m *DescriptorManager) autoReloadLoop() {
	ticker := time.NewTicker(m.config.ReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.reload()
		case <-m.stopCh:
			return
		}
	}
}

// reload reloads all descriptors
func (m *DescriptorManager) reload() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stopped {
		return
	}

	m.logger.Debug("Reloading descriptors")
	
	if err := m.loadAll(); err != nil {
		m.logger.Error("Failed to reload descriptors", "error", err)
	}
}

// GetLoader returns the descriptor loader
func (m *DescriptorManager) GetLoader() *DescriptorLoader {
	return m.loader
}

// AddDescriptorFile adds a new descriptor file at runtime
func (m *DescriptorManager) AddDescriptorFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.loader.LoadDescriptorFile(path); err != nil {
		return err
	}

	// Add to config if not already present
	for _, existing := range m.config.DescriptorFiles {
		if existing == path {
			return nil
		}
	}
	m.config.DescriptorFiles = append(m.config.DescriptorFiles, path)

	m.logger.Info("Added descriptor file", "file", path)
	return nil
}

// RemoveDescriptorFile removes a descriptor file (requires restart to take effect)
func (m *DescriptorManager) RemoveDescriptorFile(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from config
	newFiles := make([]string, 0, len(m.config.DescriptorFiles))
	for _, file := range m.config.DescriptorFiles {
		if file != path {
			newFiles = append(newFiles, file)
		}
	}
	m.config.DescriptorFiles = newFiles

	m.logger.Info("Removed descriptor file from config (restart required)", "file", path)
}

// loadFromSources loads descriptors from multi-source configuration
func (m *DescriptorManager) loadFromSources() error {
	var errors []error

	// Get or create source registry
	registry := m.loader.GetSourceRegistry()
	if registry == nil {
		registry = loader.DefaultSourceRegistry()
		m.loader.SetSourceRegistry(registry)
	}

	// Configure custom sources based on config
	for _, sourceConfig := range m.config.DescriptorSources {
		switch sourceConfig.Type {
		case "file":
			// Load from file paths
			for _, path := range sourceConfig.Paths {
				uri := "file://" + path
				if err := m.loader.LoadDescriptorFromURI(uri); err != nil {
					m.logger.Error("Failed to load descriptor from file",
						"path", path,
						"error", err)
					errors = append(errors, fmt.Errorf("file %s: %w", path, err))
				} else {
					m.logger.Info("Loaded descriptor from file", "path", path)
				}
			}

		case "http", "https":
			// Configure HTTP source with headers and timeout
			httpConfig := &loader.HTTPSourceConfig{
				Headers: sourceConfig.Headers,
			}
			if sourceConfig.Timeout > 0 {
				httpConfig.Timeout = time.Duration(sourceConfig.Timeout) * time.Second
			} else {
				httpConfig.Timeout = 30 * time.Second
			}
			
			httpSource := loader.NewHTTPSource(httpConfig)
			registry.Register("http", httpSource)
			registry.Register("https", httpSource)

			// Load from URLs
			for _, url := range sourceConfig.URLs {
				if err := m.loader.LoadDescriptorFromURI(url); err != nil {
					m.logger.Error("Failed to load descriptor from URL",
						"url", url,
						"error", err)
					errors = append(errors, fmt.Errorf("url %s: %w", url, err))
				} else {
					m.logger.Info("Loaded descriptor from URL", "url", url)
				}
			}

		case "k8s", "k8s-configmap", "configmap":
			// Configure K8s source
			k8sConfig := &loader.K8sConfigMapSourceConfig{
				Namespace: sourceConfig.Namespace,
			}
			if sourceConfig.Timeout > 0 {
				k8sConfig.Timeout = time.Duration(sourceConfig.Timeout) * time.Second
			}

			k8sSource, err := loader.NewK8sConfigMapSource(k8sConfig)
			if err != nil {
				m.logger.Error("Failed to create K8s source", "error", err)
				errors = append(errors, fmt.Errorf("k8s source: %w", err))
				continue
			}

			registry.Register("k8s", k8sSource)
			registry.Register("configmap", k8sSource)

			// Load from ConfigMaps
			for _, cm := range sourceConfig.ConfigMaps {
				for _, key := range cm.Keys {
					uri := fmt.Sprintf("k8s://%s/%s", cm.Name, key)
					if err := m.loader.LoadDescriptorFromURI(uri); err != nil {
						m.logger.Error("Failed to load descriptor from ConfigMap",
							"configmap", cm.Name,
							"key", key,
							"error", err)
						errors = append(errors, fmt.Errorf("configmap %s/%s: %w", cm.Name, key, err))
					} else {
						m.logger.Info("Loaded descriptor from ConfigMap",
							"configmap", cm.Name,
							"key", key)
					}
				}
			}

		default:
			m.logger.Warn("Unknown source type", "type", sourceConfig.Type)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to load from %d sources", len(errors))
	}

	return nil
}