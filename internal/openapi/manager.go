package openapi

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"gateway/internal/core"
	"github.com/fsnotify/fsnotify"
)

// Config represents OpenAPI manager configuration
type Config struct {
	Enabled         bool              `yaml:"enabled"`
	SpecsDirectory  string            `yaml:"specsDirectory"`  // Directory containing OpenAPI specs
	SpecURLs        []string          `yaml:"specUrls"`        // URLs to OpenAPI specs
	DefaultService  string            `yaml:"defaultService"`  // Default service name
	ReloadInterval  time.Duration     `yaml:"reloadInterval"`  // Reload interval for URLs
	WatchFiles      bool              `yaml:"watchFiles"`      // Watch local files for changes
	ServiceMappings map[string]string `yaml:"serviceMappings"` // Tag to service mappings
}

// Manager manages dynamic routes from OpenAPI specifications
type Manager struct {
	config       *Config
	loader       *Loader
	router       core.Router
	logger       *slog.Logger
	specs        map[string]*Spec // source -> spec
	currentRules []core.RouteRule
	mu           sync.RWMutex
	watcher      *fsnotify.Watcher
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewManager creates a new OpenAPI manager
func NewManager(config *Config, router core.Router, logger *slog.Logger) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config: config,
		loader: NewLoader(logger),
		router: router,
		logger: logger.With("component", "openapi_manager"),
		specs:  make(map[string]*Spec),
		ctx:    ctx,
		cancel: cancel,
	}

	// Create file watcher if needed
	if config.WatchFiles && config.SpecsDirectory != "" {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, fmt.Errorf("failed to create file watcher: %w", err)
		}
		m.watcher = watcher
	}

	return m, nil
}

// Start starts the OpenAPI manager
func (m *Manager) Start() error {
	// Initial load
	if err := m.reload(); err != nil {
		return fmt.Errorf("initial load failed: %w", err)
	}

	// Start file watcher
	if m.watcher != nil {
		if err := m.watcher.Add(m.config.SpecsDirectory); err != nil {
			return fmt.Errorf("failed to watch directory: %w", err)
		}
		go m.watchFiles()
	}

	// Start URL reloader
	if len(m.config.SpecURLs) > 0 && m.config.ReloadInterval > 0 {
		go m.reloadURLs()
	}

	m.logger.Info("OpenAPI manager started",
		"directory", m.config.SpecsDirectory,
		"urls", len(m.config.SpecURLs),
		"watchFiles", m.config.WatchFiles,
	)

	return nil
}

// Stop stops the OpenAPI manager
func (m *Manager) Stop() error {
	m.cancel()

	if m.watcher != nil {
		m.watcher.Close()
	}

	m.logger.Info("OpenAPI manager stopped")
	return nil
}

// GetSpecs returns all loaded specs
func (m *Manager) GetSpecs() map[string]*Spec {
	m.mu.RLock()
	defer m.mu.RUnlock()

	specs := make(map[string]*Spec)
	for k, v := range m.specs {
		specs[k] = v
	}
	return specs
}

// GetRoutes returns all current routes
func (m *Manager) GetRoutes() []core.RouteRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]core.RouteRule, len(m.currentRules))
	copy(rules, m.currentRules)
	return rules
}

// reload loads all specs and updates routes
func (m *Manager) reload() error {
	newSpecs := make(map[string]*Spec)
	var allRules []core.RouteRule

	// Load from directory
	if m.config.SpecsDirectory != "" {
		specs, err := m.loader.LoadDirectory(m.config.SpecsDirectory)
		if err != nil {
			m.logger.Error("Failed to load specs from directory",
				"directory", m.config.SpecsDirectory,
				"error", err,
			)
		} else {
			for _, spec := range specs {
				source := filepath.Join(m.config.SpecsDirectory, spec.Info.Title+".yaml")
				newSpecs[source] = spec
				rules := m.loader.ToRouteRules(spec, m.config.DefaultService)
				allRules = append(allRules, rules...)
			}
		}
	}

	// Load from URLs
	for _, url := range m.config.SpecURLs {
		spec, err := m.loader.Load(url)
		if err != nil {
			m.logger.Error("Failed to load spec from URL",
				"url", url,
				"error", err,
			)
			continue
		}
		newSpecs[url] = spec
		rules := m.loader.ToRouteRules(spec, m.config.DefaultService)
		allRules = append(allRules, rules...)
	}

	// Update router with new rules
	if err := m.updateRoutes(allRules); err != nil {
		return fmt.Errorf("failed to update routes: %w", err)
	}

	// Update specs
	m.mu.Lock()
	m.specs = newSpecs
	m.currentRules = allRules
	m.mu.Unlock()

	m.logger.Info("OpenAPI specs reloaded",
		"specs", len(newSpecs),
		"routes", len(allRules),
	)

	return nil
}

// updateRoutes updates the router with new rules
func (m *Manager) updateRoutes(rules []core.RouteRule) error {
	// This is a simplified implementation
	// In a real system, you'd want to:
	// 1. Diff old and new rules
	// 2. Remove deleted routes
	// 3. Update changed routes
	// 4. Add new routes

	for _, rule := range rules {
		// Router interface would need AddRule method
		// For now, we'll just log
		m.logger.Debug("Would add route",
			"id", rule.ID,
			"path", rule.Path,
			"service", rule.ServiceName,
		)
	}

	return nil
}

// watchFiles watches for file changes
func (m *Manager) watchFiles() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			// Check if it's an OpenAPI file
			ext := filepath.Ext(event.Name)
			if ext != ".yaml" && ext != ".yml" && ext != ".json" {
				continue
			}

			switch event.Op {
			case fsnotify.Create, fsnotify.Write:
				m.logger.Info("OpenAPI file changed", "file", event.Name)
				if err := m.reload(); err != nil {
					m.logger.Error("Failed to reload after file change",
						"file", event.Name,
						"error", err,
					)
				}
			case fsnotify.Remove:
				m.logger.Info("OpenAPI file removed", "file", event.Name)
				if err := m.reload(); err != nil {
					m.logger.Error("Failed to reload after file removal",
						"file", event.Name,
						"error", err,
					)
				}
			}
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.logger.Error("File watcher error", "error", err)
		}
	}
}

// reloadURLs periodically reloads specs from URLs
func (m *Manager) reloadURLs() {
	ticker := time.NewTicker(m.config.ReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.reload(); err != nil {
				m.logger.Error("Failed to reload URLs", "error", err)
			}
		}
	}
}
