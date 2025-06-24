package grpc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gateway/internal/loader"
	"github.com/fsnotify/fsnotify"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// DescriptorLoader manages loading of .desc files
type DescriptorLoader struct {
	mu           sync.RWMutex
	registry     *ProtoRegistry
	loadedFiles  map[string]bool
	watcher      *fsnotify.Watcher
	logger       *slog.Logger
	watcherDone  chan struct{}
	onReload     func(path string) // Callback when a file is reloaded
	sourceRegistry *loader.SourceRegistry // Source registry for multi-source loading
}

// NewDescriptorLoader creates a new descriptor loader
func NewDescriptorLoader(registry *ProtoRegistry) *DescriptorLoader {
	return NewDescriptorLoaderWithLogger(registry, slog.Default())
}

// NewDescriptorLoaderWithLogger creates a new descriptor loader with logger
func NewDescriptorLoaderWithLogger(registry *ProtoRegistry, logger *slog.Logger) *DescriptorLoader {
	return &DescriptorLoader{
		registry:    registry,
		loadedFiles: make(map[string]bool),
		logger:      logger.With("component", "descriptor_loader"),
		sourceRegistry: loader.DefaultSourceRegistry(),
	}
}

// LoadDescriptorFile loads a .desc file
func (l *DescriptorLoader) LoadDescriptorFile(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if already loaded
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if l.loadedFiles[absPath] {
		return nil // Already loaded
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read descriptor file: %w", err)
	}

	// Load into registry
	if err := l.registry.LoadDescriptorSet(data); err != nil {
		return fmt.Errorf("failed to load descriptor set: %w", err)
	}

	l.loadedFiles[absPath] = true
	
	// Add to file watcher if enabled
	l.addFileToWatcher(absPath)
	
	return nil
}

// LoadDescriptorFromURI loads a descriptor from a URI
// URI format: scheme://path
// Examples:
//   - file:///path/to/descriptor.desc
//   - http://example.com/descriptor.desc
//   - k8s://configmap-name/descriptor-key
func (l *DescriptorLoader) LoadDescriptorFromURI(uri string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if already loaded
	if l.loadedFiles[uri] {
		return nil // Already loaded
	}

	// Load from source
	ctx := context.Background()
	reader, err := l.sourceRegistry.Load(ctx, uri)
	if err != nil {
		return fmt.Errorf("failed to load from URI %s: %w", uri, err)
	}
	defer reader.Close()

	// Read the data
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read descriptor data: %w", err)
	}

	// Load into registry
	if err := l.registry.LoadDescriptorSet(data); err != nil {
		return fmt.Errorf("failed to load descriptor set: %w", err)
	}

	l.loadedFiles[uri] = true
	
	// Note: File watching is only supported for local files
	if strings.HasPrefix(uri, "file://") {
		path := strings.TrimPrefix(uri, "file://")
		l.addFileToWatcher(path)
	}
	
	return nil
}

// LoadDescriptorDirectory loads all .desc files from a directory
func (l *DescriptorLoader) LoadDescriptorDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-.desc files
		if info.IsDir() || !strings.HasSuffix(path, ".desc") {
			return nil
		}

		return l.LoadDescriptorFile(path)
	})
}

// LoadDescriptorFromProto loads a FileDescriptorProto directly
func (l *DescriptorLoader) LoadDescriptorFromProto(fdp *descriptorpb.FileDescriptorProto) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp},
	}

	return l.registry.LoadDescriptorSetProto(fds)
}

// LoadDescriptorFromBytes loads a descriptor from raw bytes
func (l *DescriptorLoader) LoadDescriptorFromBytes(data []byte) error {
	// Try to unmarshal as FileDescriptorSet first
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fds); err == nil {
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.registry.LoadDescriptorSetProto(&fds)
	}

	// Try as single FileDescriptorProto
	var fdp descriptorpb.FileDescriptorProto
	if err := proto.Unmarshal(data, &fdp); err == nil {
		return l.LoadDescriptorFromProto(&fdp)
	}

	return fmt.Errorf("failed to unmarshal descriptor data")
}

// IsLoaded checks if a file has been loaded
func (l *DescriptorLoader) IsLoaded(path string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	return l.loadedFiles[absPath]
}

// GetLoadedFiles returns a list of loaded descriptor files
func (l *DescriptorLoader) GetLoadedFiles() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	files := make([]string, 0, len(l.loadedFiles))
	for file := range l.loadedFiles {
		files = append(files, file)
	}
	return files
}

// ReloadFile reloads a descriptor file
func (l *DescriptorLoader) ReloadFile(path string) error {
	l.mu.Lock()
	absPath, err := filepath.Abs(path)
	if err != nil {
		l.mu.Unlock()
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	// Remove from loaded files tracker
	delete(l.loadedFiles, absPath)
	
	// Note: In a real implementation, we would need to clear the specific
	// descriptors from the registry before reloading. For now, we'll just
	// reload and let the registry handle duplicates.
	l.mu.Unlock()

	return l.LoadDescriptorFile(path)
}

// ClearAll clears all loaded descriptors
func (l *DescriptorLoader) ClearAll() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create new registry
	l.registry = NewProtoRegistry()
	l.loadedFiles = make(map[string]bool)
}

// EnableFileWatching enables automatic reloading when descriptor files change
func (l *DescriptorLoader) EnableFileWatching() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.watcher != nil {
		return fmt.Errorf("file watching already enabled")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	l.watcher = watcher
	l.watcherDone = make(chan struct{})

	// Start the watcher goroutine
	go l.watchFiles()

	// Add all loaded files to the watcher
	for file := range l.loadedFiles {
		if err := l.watcher.Add(file); err != nil {
			l.logger.Error("Failed to watch file", "file", file, "error", err)
		} else {
			l.logger.Debug("Watching file", "file", file)
		}
	}

	l.logger.Info("File watching enabled")
	return nil
}

// DisableFileWatching disables automatic reloading
func (l *DescriptorLoader) DisableFileWatching() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.watcher == nil {
		return nil
	}

	// Close the watcher
	if err := l.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close file watcher: %w", err)
	}

	// Wait for the watcher goroutine to finish
	close(l.watcherDone)
	l.watcher = nil
	l.watcherDone = nil

	l.logger.Info("File watching disabled")
	return nil
}

// SetReloadCallback sets a callback function to be called when a file is reloaded
func (l *DescriptorLoader) SetReloadCallback(callback func(path string)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onReload = callback
}

// SetSourceRegistry sets a custom source registry
func (l *DescriptorLoader) SetSourceRegistry(registry *loader.SourceRegistry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sourceRegistry = registry
}

// GetSourceRegistry returns the current source registry
func (l *DescriptorLoader) GetSourceRegistry() *loader.SourceRegistry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.sourceRegistry
}

// watchFiles handles file system events
func (l *DescriptorLoader) watchFiles() {
	for {
		select {
		case event, ok := <-l.watcher.Events:
			if !ok {
				return
			}

			// Handle file changes
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				l.handleFileChange(event.Name)
			} else if event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
				l.handleFileRemoval(event.Name)
			}

		case err, ok := <-l.watcher.Errors:
			if !ok {
				return
			}
			l.logger.Error("File watcher error", "error", err)

		case <-l.watcherDone:
			return
		}
	}
}

// handleFileChange handles when a watched file is modified or created
func (l *DescriptorLoader) handleFileChange(path string) {
	l.logger.Info("Descriptor file changed, reloading", "file", path)

	// Reload the file
	if err := l.ReloadFile(path); err != nil {
		l.logger.Error("Failed to reload descriptor file", "file", path, "error", err)
		return
	}

	// Call the reload callback if set
	l.mu.RLock()
	callback := l.onReload
	l.mu.RUnlock()

	if callback != nil {
		callback(path)
	}
}

// handleFileRemoval handles when a watched file is removed
func (l *DescriptorLoader) handleFileRemoval(path string) {
	l.logger.Warn("Descriptor file removed", "file", path)

	l.mu.Lock()
	defer l.mu.Unlock()

	// Remove from loaded files
	delete(l.loadedFiles, path)

	// Stop watching the file
	if l.watcher != nil {
		_ = l.watcher.Remove(path)
	}

	// Note: We don't remove the descriptors from the registry here
	// because they might still be in use. The registry would need
	// a more sophisticated mechanism to safely remove descriptors.
}

// addFileToWatcher adds a file to the watcher if watching is enabled
func (l *DescriptorLoader) addFileToWatcher(path string) {
	if l.watcher != nil {
		if err := l.watcher.Add(path); err != nil {
			l.logger.Error("Failed to watch file", "file", path, "error", err)
		} else {
			l.logger.Debug("Added file to watcher", "file", path)
		}
	}
}