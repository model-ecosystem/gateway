package openapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gateway/internal/core"
	"gateway/internal/loader"
)

// DescriptorConfig holds configuration for OpenAPI descriptor loading
type DescriptorConfig struct {
	// SpecFiles is a list of OpenAPI spec files to load
	SpecFiles []string `yaml:"specFiles"`
	// SpecDirs is a list of directories to scan for OpenAPI spec files
	SpecDirs []string `yaml:"specDirs"`
	// SpecURLs is a list of URLs to fetch OpenAPI specs from
	SpecURLs []string `yaml:"specUrls"`
	// AutoReload enables automatic reloading of specs
	AutoReload bool `yaml:"autoReload"`
	// ReloadInterval is the interval for checking spec changes
	ReloadInterval time.Duration `yaml:"reloadInterval"`
	// FailOnError determines if the gateway should fail on spec loading errors
	FailOnError bool `yaml:"failOnError"`
	// FileExtensions defines which file extensions to consider as OpenAPI specs
	FileExtensions []string `yaml:"fileExtensions"`
	// DefaultService is the default service name for routes without explicit service
	DefaultService string `yaml:"defaultService"`
}

// DefaultDescriptorConfig returns default OpenAPI descriptor configuration
func DefaultDescriptorConfig() DescriptorConfig {
	return DescriptorConfig{
		SpecFiles:      []string{},
		SpecDirs:       []string{},
		SpecURLs:       []string{},
		AutoReload:     false,
		ReloadInterval: 30 * time.Second,
		FailOnError:    false,
		FileExtensions: []string{".yaml", ".yml", ".json"},
		DefaultService: "api-service",
	}
}

// DescriptorLoader manages loading of OpenAPI specs for dynamic protocol support
type DescriptorLoader struct {
	mu           sync.RWMutex
	loader       *Loader
	loadedSpecs  map[string]*LoadedSpec // path/url -> spec
	watcher      *fsnotify.Watcher
	logger       *slog.Logger
	watcherDone  chan struct{}
	onReload     func(source string, routes []core.RouteRule) // Callback when spec is reloaded
	config       DescriptorConfig
	sourceRegistry *loader.SourceRegistry // Source registry for multi-source loading
}

// LoadedSpec represents a loaded OpenAPI specification
type LoadedSpec struct {
	Source       string            // File path or URL
	Spec         *Spec             // Parsed specification
	Routes       []core.RouteRule  // Generated routes
	LoadedAt     time.Time         // When the spec was loaded
	LastModified time.Time         // Last modification time (for files)
	Service      string            // Target service name
}

// NewDescriptorLoader creates a new OpenAPI descriptor loader
func NewDescriptorLoader(config DescriptorConfig, logger *slog.Logger) *DescriptorLoader {
	if logger == nil {
		logger = slog.Default()
	}

	openapiLoader := NewLoader(logger)
	return &DescriptorLoader{
		loader:      openapiLoader,
		loadedSpecs: make(map[string]*LoadedSpec),
		logger:      logger.With("component", "openapi_descriptor_loader"),
		config:      config,
		sourceRegistry: loader.DefaultSourceRegistry(),
	}
}

// LoadSpecFile loads an OpenAPI spec file
func (d *DescriptorLoader) LoadSpecFile(path string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Load the spec
	spec, err := d.loader.Load(absPath)
	if err != nil {
		return fmt.Errorf("failed to load spec file: %w", err)
	}

	// Get file info
	stat, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Generate routes from spec
	routes := d.loader.ToRouteRules(spec, d.config.DefaultService)

	// Store loaded spec
	d.loadedSpecs[absPath] = &LoadedSpec{
		Source:       absPath,
		Spec:         spec,
		Routes:       routes,
		LoadedAt:     time.Now(),
		LastModified: stat.ModTime(),
		Service:      d.config.DefaultService,
	}

	d.logger.Info("Loaded OpenAPI spec file",
		"file", absPath,
		"routes", len(routes),
		"title", spec.Info.Title,
		"version", spec.Info.Version,
	)

	// Call reload callback if set
	if d.onReload != nil {
		d.onReload(absPath, routes)
	}

	return nil
}

// LoadSpecURL loads an OpenAPI spec from URL
func (d *DescriptorLoader) LoadSpecURL(url string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Load the spec
	spec, err := d.loader.Load(url)
	if err != nil {
		return fmt.Errorf("failed to load spec from URL: %w", err)
	}

	// Generate routes from spec
	routes := d.loader.ToRouteRules(spec, d.config.DefaultService)

	// Store loaded spec
	d.loadedSpecs[url] = &LoadedSpec{
		Source:   url,
		Spec:     spec,
		Routes:   routes,
		LoadedAt: time.Now(),
		Service:  d.config.DefaultService,
	}

	d.logger.Info("Loaded OpenAPI spec from URL",
		"url", url,
		"routes", len(routes),
		"title", spec.Info.Title,
		"version", spec.Info.Version,
	)

	// Call reload callback if set
	if d.onReload != nil {
		d.onReload(url, routes)
	}

	return nil
}

// LoadSpecDirectory loads all OpenAPI specs from a directory
func (d *DescriptorLoader) LoadSpecDirectory(dir string) error {
	// Get absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if directory exists
	stat, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("failed to stat directory: %w", err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absDir)
	}

	// Walk directory
	var errors []error
	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file has OpenAPI extension
		if d.isOpenAPIFile(path) {
			if err := d.LoadSpecFile(path); err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", path, err))
				if d.config.FailOnError {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to load %d specs", len(errors))
	}

	return nil
}

// EnableFileWatching enables automatic reloading when spec files change
func (d *DescriptorLoader) EnableFileWatching() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.watcher != nil {
		return fmt.Errorf("file watching already enabled")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	d.watcher = watcher
	d.watcherDone = make(chan struct{})

	// Add all loaded spec files to watcher
	for source := range d.loadedSpecs {
		// Only watch files, not URLs
		if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
			if err := watcher.Add(source); err != nil {
				d.logger.Error("Failed to watch file", "file", source, "error", err)
			}
		}
	}

	// Start watcher goroutine
	go d.watchFiles()

	d.logger.Info("File watching enabled for OpenAPI specs")
	return nil
}

// DisableFileWatching disables automatic reloading
func (d *DescriptorLoader) DisableFileWatching() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.watcher == nil {
		return nil
	}

	close(d.watcherDone)
	err := d.watcher.Close()
	d.watcher = nil

	return err
}

// SetReloadCallback sets the callback function called when a spec is reloaded
func (d *DescriptorLoader) SetReloadCallback(callback func(source string, routes []core.RouteRule)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onReload = callback
}

// GetLoadedSpecs returns all loaded specs
func (d *DescriptorLoader) GetLoadedSpecs() map[string]*LoadedSpec {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a copy
	result := make(map[string]*LoadedSpec, len(d.loadedSpecs))
	for k, v := range d.loadedSpecs {
		result[k] = v
	}
	return result
}

// GetAllRoutes returns all routes from all loaded specs
func (d *DescriptorLoader) GetAllRoutes() []core.RouteRule {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var routes []core.RouteRule
	for _, spec := range d.loadedSpecs {
		routes = append(routes, spec.Routes...)
	}
	return routes
}

// watchFiles watches for file changes
func (d *DescriptorLoader) watchFiles() {
	for {
		select {
		case event, ok := <-d.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				d.handleFileChange(event.Name)
			}
		case err, ok := <-d.watcher.Errors:
			if !ok {
				return
			}
			d.logger.Error("File watcher error", "error", err)
		case <-d.watcherDone:
			return
		}
	}
}

// handleFileChange handles a file change event
func (d *DescriptorLoader) handleFileChange(path string) {
	d.logger.Info("OpenAPI spec file changed, reloading", "file", path)

	// Reload the spec
	if err := d.LoadSpecFile(path); err != nil {
		d.logger.Error("Failed to reload OpenAPI spec", "file", path, "error", err)
	}
}

// isOpenAPIFile checks if a file is an OpenAPI spec based on extension
func (d *DescriptorLoader) isOpenAPIFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range d.config.FileExtensions {
		if ext == allowed {
			return true
		}
	}
	return false
}

// ReloadAll reloads all specs
func (d *DescriptorLoader) ReloadAll() error {
	d.mu.RLock()
	sources := make([]string, 0, len(d.loadedSpecs))
	for source := range d.loadedSpecs {
		sources = append(sources, source)
	}
	d.mu.RUnlock()

	var errors []error
	for _, source := range sources {
		if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
			if err := d.LoadSpecURL(source); err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", source, err))
			}
		} else {
			if err := d.LoadSpecFile(source); err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", source, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to reload %d specs", len(errors))
	}

	return nil
}

// LoadSpecFromURI loads an OpenAPI spec from a URI
// URI format: scheme://path
// Examples:
//   - file:///path/to/spec.yaml
//   - http://example.com/spec.yaml
//   - k8s://configmap-name/spec-key
func (d *DescriptorLoader) LoadSpecFromURI(uri string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if already loaded
	if _, exists := d.loadedSpecs[uri]; exists {
		// Reload the spec
		delete(d.loadedSpecs, uri)
	}

	// Load from source
	ctx := context.Background()
	reader, err := d.sourceRegistry.Load(ctx, uri)
	if err != nil {
		return fmt.Errorf("failed to load from URI %s: %w", uri, err)
	}
	defer reader.Close()

	// Read the data
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read spec data: %w", err)
	}

	// Parse the spec
	spec, err := d.loader.ParseBytes(data)
	if err != nil {
		return fmt.Errorf("failed to parse spec: %w", err)
	}

	// Generate routes from spec
	routes := d.loader.ToRouteRules(spec, d.config.DefaultService)

	// Store loaded spec
	d.loadedSpecs[uri] = &LoadedSpec{
		Source:   uri,
		Spec:     spec,
		Routes:   routes,
		LoadedAt: time.Now(),
		Service:  d.config.DefaultService,
	}

	d.logger.Info("Loaded OpenAPI spec from URI",
		"uri", uri,
		"routes", len(routes),
		"title", spec.Info.Title,
		"version", spec.Info.Version,
	)

	// Call reload callback if set
	if d.onReload != nil {
		d.onReload(uri, routes)
	}

	// Add file watching for local files
	if strings.HasPrefix(uri, "file://") {
		path := strings.TrimPrefix(uri, "file://")
		if d.watcher != nil {
			if err := d.watcher.Add(path); err != nil {
				d.logger.Error("Failed to watch file", "file", path, "error", err)
			}
		}
	}

	return nil
}

// SetSourceRegistry sets a custom source registry
func (d *DescriptorLoader) SetSourceRegistry(registry *loader.SourceRegistry) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sourceRegistry = registry
}

// GetSourceRegistry returns the current source registry
func (d *DescriptorLoader) GetSourceRegistry() *loader.SourceRegistry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sourceRegistry
}