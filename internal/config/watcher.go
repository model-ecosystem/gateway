package config

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatcherConfig holds configuration for the config watcher
type WatcherConfig struct {
	// Debounce duration to avoid multiple rapid reloads
	DebounceDuration time.Duration
	// Callback function when config changes
	OnChange func(newConfig *Config) error
	// Callback function when reload fails
	OnError func(error)
}

// DefaultWatcherConfig returns default watcher configuration
func DefaultWatcherConfig() *WatcherConfig {
	return &WatcherConfig{
		DebounceDuration: 500 * time.Millisecond,
		OnChange:         nil,
		OnError:          nil,
	}
}

// Watcher monitors configuration file changes
type Watcher struct {
	configPath string
	config     *WatcherConfig
	watcher    *fsnotify.Watcher
	logger     *slog.Logger
	mu         sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
	debouncer  *time.Timer
}

// NewWatcher creates a new configuration watcher
func NewWatcher(configPath string, config *WatcherConfig, logger *slog.Logger) (*Watcher, error) {
	if config == nil {
		config = DefaultWatcherConfig()
	}

	// Get absolute path
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	w := &Watcher{
		configPath: absPath,
		config:     config,
		watcher:    watcher,
		logger:     logger.With("component", "config-watcher"),
		stopCh:     make(chan struct{}),
	}

	// Add config file to watcher
	if err := watcher.Add(absPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch config file: %w", err)
	}

	// Also watch the directory for atomic writes
	dir := filepath.Dir(absPath)
	if err := watcher.Add(dir); err != nil {
		// Non-fatal: some editors use atomic writes
		w.logger.Warn("Failed to watch config directory", "dir", dir, "error", err)
	}

	return w, nil
}

// Start begins watching for configuration changes
func (w *Watcher) Start() {
	w.wg.Add(1)
	go w.watchLoop()
	w.logger.Info("Configuration watcher started", "file", w.configPath)
}

// Stop stops the configuration watcher
func (w *Watcher) Stop() error {
	close(w.stopCh)
	w.wg.Wait()
	
	// Cancel any pending debounced reload
	w.mu.Lock()
	if w.debouncer != nil {
		w.debouncer.Stop()
	}
	w.mu.Unlock()
	
	return w.watcher.Close()
}

// watchLoop monitors file system events
func (w *Watcher) watchLoop() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("File watcher error", "error", err)
			if w.config.OnError != nil {
				w.config.OnError(fmt.Errorf("watcher error: %w", err))
			}

		case <-w.stopCh:
			return
		}
	}
}

// handleEvent processes file system events
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Check if event is for our config file
	if event.Name != w.configPath {
		// Could be a directory event or temp file
		return
	}

	// Handle different event types
	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		w.logger.Debug("Config file modified", "file", event.Name)
		w.scheduleReload()

	case event.Op&fsnotify.Create == fsnotify.Create:
		w.logger.Debug("Config file created", "file", event.Name)
		w.scheduleReload()

	case event.Op&fsnotify.Remove == fsnotify.Remove:
		w.logger.Warn("Config file removed", "file", event.Name)
		// Re-add watcher for when file is recreated
		w.watcher.Add(event.Name)

	case event.Op&fsnotify.Rename == fsnotify.Rename:
		w.logger.Debug("Config file renamed", "file", event.Name)
		// Re-add watcher for atomic writes
		w.watcher.Add(w.configPath)
		w.scheduleReload()
	}
}

// scheduleReload debounces reload requests
func (w *Watcher) scheduleReload() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Cancel previous timer if exists
	if w.debouncer != nil {
		w.debouncer.Stop()
	}

	// Schedule new reload
	w.debouncer = time.AfterFunc(w.config.DebounceDuration, func() {
		if err := w.reload(); err != nil {
			w.logger.Error("Config reload failed", "error", err)
			if w.config.OnError != nil {
				w.config.OnError(err)
			}
		}
	})
}

// reload loads and applies new configuration
func (w *Watcher) reload() error {
	w.logger.Info("Reloading configuration", "file", w.configPath)

	// Load new configuration
	newConfig, err := Load(w.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate new configuration
	if err := w.validateConfig(newConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Apply new configuration
	if w.config.OnChange != nil {
		if err := w.config.OnChange(newConfig); err != nil {
			return fmt.Errorf("failed to apply config: %w", err)
		}
	}

	w.logger.Info("Configuration reloaded successfully")
	return nil
}

// validateConfig performs basic validation on new configuration
func (w *Watcher) validateConfig(cfg *Config) error {
	// Basic validation
	if cfg.Gateway.Frontend.HTTP.Port <= 0 || cfg.Gateway.Frontend.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", cfg.Gateway.Frontend.HTTP.Port)
	}

	// Validate at least one service is configured
	if cfg.Gateway.Registry.Type == "static" && len(cfg.Gateway.Registry.Static.Services) == 0 {
		return fmt.Errorf("no services configured in static registry")
	}

	// Validate routes reference existing services
	serviceMap := make(map[string]bool)
	for _, svc := range cfg.Gateway.Registry.Static.Services {
		serviceMap[svc.Name] = true
	}

	for _, rule := range cfg.Gateway.Router.Rules {
		if rule.ServiceName != "" && !serviceMap[rule.ServiceName] {
			return fmt.Errorf("route %s references unknown service: %s", rule.ID, rule.ServiceName)
		}
	}

	return nil
}

// GetCurrentConfig returns the current configuration path
func (w *Watcher) GetCurrentConfig() string {
	return w.configPath
}