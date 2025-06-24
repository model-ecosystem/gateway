package openapi

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gateway/internal/core"
)

// DescriptorManager manages dynamic loading of OpenAPI descriptors and route updates
type DescriptorManager struct {
	config       DescriptorConfig
	loader       *DescriptorLoader
	logger       *slog.Logger
	mu           sync.RWMutex
	stopCh       chan struct{}
	stopped      bool
	routeUpdater RouteUpdater
}

// RouteUpdater is an interface for updating routes dynamically
type RouteUpdater interface {
	// UpdateRoutes updates the routes based on OpenAPI specs
	UpdateRoutes(source string, routes []core.RouteRule) error
	// RemoveRoutes removes routes from a specific source
	RemoveRoutes(source string) error
}

// NewDescriptorManager creates a new OpenAPI descriptor manager
func NewDescriptorManager(config DescriptorConfig, logger *slog.Logger) *DescriptorManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &DescriptorManager{
		config: config,
		loader: NewDescriptorLoader(config, logger),
		logger: logger.With("component", "openapi_descriptor_manager"),
		stopCh: make(chan struct{}),
	}
}

// SetRouteUpdater sets the route updater for dynamic route updates
func (m *DescriptorManager) SetRouteUpdater(updater RouteUpdater) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routeUpdater = updater
}

// Start starts the descriptor manager
func (m *DescriptorManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return fmt.Errorf("descriptor manager already stopped")
	}

	// Set up reload callback to update routes
	m.loader.SetReloadCallback(func(source string, routes []core.RouteRule) {
		if m.routeUpdater != nil {
			if err := m.routeUpdater.UpdateRoutes(source, routes); err != nil {
				m.logger.Error("Failed to update routes", 
					"source", source, 
					"error", err)
			} else {
				m.logger.Info("Routes updated from OpenAPI spec",
					"source", source,
					"routes", len(routes))
			}
		}
	})

	// Initial load
	if err := m.loadAll(); err != nil {
		if m.config.FailOnError {
			return err
		}
		m.logger.Error("Failed to load OpenAPI specs", "error", err)
	}

	// Enable file watching if auto-reload is enabled
	if m.config.AutoReload {
		if err := m.loader.EnableFileWatching(); err != nil {
			m.logger.Error("Failed to enable file watching", "error", err)
			// Fall back to interval-based reloading
			if m.config.ReloadInterval > 0 {
				go m.autoReloadLoop()
			}
		} else {
			m.logger.Info("File watching enabled for OpenAPI specs")
		}

		// Also start interval-based reloading for URLs
		if len(m.config.SpecURLs) > 0 && m.config.ReloadInterval > 0 {
			go m.autoReloadURLs()
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

// loadAll loads all configured OpenAPI specs
func (m *DescriptorManager) loadAll() error {
	var errors []error

	// Load individual files
	for _, file := range m.config.SpecFiles {
		if err := m.loader.LoadSpecFile(file); err != nil {
			m.logger.Error("Failed to load OpenAPI spec file", 
				"file", file,
				"error", err)
			errors = append(errors, fmt.Errorf("file %s: %w", file, err))
		} else {
			m.logger.Info("Loaded OpenAPI spec file", "file", file)
		}
	}

	// Load directories
	for _, dir := range m.config.SpecDirs {
		if err := m.loader.LoadSpecDirectory(dir); err != nil {
			m.logger.Error("Failed to load OpenAPI spec directory",
				"directory", dir,
				"error", err)
			errors = append(errors, fmt.Errorf("directory %s: %w", dir, err))
		} else {
			m.logger.Info("Loaded OpenAPI spec directory", "directory", dir)
		}
	}

	// Load URLs
	for _, url := range m.config.SpecURLs {
		if err := m.loader.LoadSpecURL(url); err != nil {
			m.logger.Error("Failed to load OpenAPI spec from URL",
				"url", url,
				"error", err)
			errors = append(errors, fmt.Errorf("URL %s: %w", url, err))
		} else {
			m.logger.Info("Loaded OpenAPI spec from URL", "url", url)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to load %d OpenAPI specs", len(errors))
	}

	return nil
}

// autoReloadLoop periodically reloads all specs
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

// autoReloadURLs periodically reloads URL-based specs
func (m *DescriptorManager) autoReloadURLs() {
	ticker := time.NewTicker(m.config.ReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.reloadURLs()
		case <-m.stopCh:
			return
		}
	}
}

// reload reloads all specs
func (m *DescriptorManager) reload() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stopped {
		return
	}

	m.logger.Debug("Reloading OpenAPI specs")
	
	if err := m.loader.ReloadAll(); err != nil {
		m.logger.Error("Failed to reload OpenAPI specs", "error", err)
	}
}

// reloadURLs reloads only URL-based specs
func (m *DescriptorManager) reloadURLs() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stopped {
		return
	}

	m.logger.Debug("Reloading OpenAPI specs from URLs")
	
	for _, url := range m.config.SpecURLs {
		if err := m.loader.LoadSpecURL(url); err != nil {
			m.logger.Error("Failed to reload OpenAPI spec from URL",
				"url", url,
				"error", err)
		}
	}
}

// GetLoader returns the descriptor loader
func (m *DescriptorManager) GetLoader() *DescriptorLoader {
	return m.loader
}

// GetAllRoutes returns all routes from all loaded specs
func (m *DescriptorManager) GetAllRoutes() []core.RouteRule {
	return m.loader.GetAllRoutes()
}

// AddSpecFile adds a new spec file at runtime
func (m *DescriptorManager) AddSpecFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.loader.LoadSpecFile(path); err != nil {
		return err
	}

	// Add to config if not already present
	for _, existing := range m.config.SpecFiles {
		if existing == path {
			return nil
		}
	}
	m.config.SpecFiles = append(m.config.SpecFiles, path)

	m.logger.Info("Added OpenAPI spec file", "file", path)
	return nil
}

// RemoveSpecFile removes a spec file
func (m *DescriptorManager) RemoveSpecFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from config
	newFiles := make([]string, 0, len(m.config.SpecFiles))
	for _, file := range m.config.SpecFiles {
		if file != path {
			newFiles = append(newFiles, file)
		}
	}
	m.config.SpecFiles = newFiles

	// Remove routes if route updater is available
	if m.routeUpdater != nil {
		if err := m.routeUpdater.RemoveRoutes(path); err != nil {
			return fmt.Errorf("failed to remove routes: %w", err)
		}
	}

	m.logger.Info("Removed OpenAPI spec file", "file", path)
	return nil
}

// AddSpecURL adds a new spec URL at runtime
func (m *DescriptorManager) AddSpecURL(url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.loader.LoadSpecURL(url); err != nil {
		return err
	}

	// Add to config if not already present
	for _, existing := range m.config.SpecURLs {
		if existing == url {
			return nil
		}
	}
	m.config.SpecURLs = append(m.config.SpecURLs, url)

	m.logger.Info("Added OpenAPI spec URL", "url", url)
	return nil
}

// RemoveSpecURL removes a spec URL
func (m *DescriptorManager) RemoveSpecURL(url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from config
	newURLs := make([]string, 0, len(m.config.SpecURLs))
	for _, u := range m.config.SpecURLs {
		if u != url {
			newURLs = append(newURLs, u)
		}
	}
	m.config.SpecURLs = newURLs

	// Remove routes if route updater is available
	if m.routeUpdater != nil {
		if err := m.routeUpdater.RemoveRoutes(url); err != nil {
			return fmt.Errorf("failed to remove routes: %w", err)
		}
	}

	m.logger.Info("Removed OpenAPI spec URL", "url", url)
	return nil
}