package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestWatcher(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	
	initialConfig := `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8080
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
      maxIdleConnsPerHost: 10
      idleConnTimeout: 90
      keepAlive: true
  registry:
    type: static
    static:
      services:
        - name: test-service
          instances:
            - id: test-1
              address: "127.0.0.1"
              port: 3000
              health: healthy
  router:
    rules:
      - id: test-route
        path: /test/*
        serviceName: test-service
        loadBalance: round_robin
`
	
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatal(err)
	}
	
	// Track config changes
	configChanges := 0
	var lastConfig *Config
	
	watcherConfig := &WatcherConfig{
		DebounceDuration: 100 * time.Millisecond,
		OnChange: func(cfg *Config) error {
			configChanges++
			lastConfig = cfg
			return nil
		},
		OnError: func(err error) {
			t.Errorf("Watcher error: %v", err)
		},
	}
	
	// Create and start watcher
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	watcher, err := NewWatcher(configPath, watcherConfig, logger)
	if err != nil {
		t.Fatal(err)
	}
	watcher.Start()
	defer watcher.Stop()
	
	// Give watcher time to start
	time.Sleep(200 * time.Millisecond)
	
	// Test 1: Modify config file
	t.Run("FileModification", func(t *testing.T) {
		updatedConfig := `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8081  # Changed port
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
      maxIdleConnsPerHost: 10
      idleConnTimeout: 90
      keepAlive: true
  registry:
    type: static
    static:
      services:
        - name: test-service
          instances:
            - id: test-1
              address: "127.0.0.1"
              port: 3000
              health: healthy
  router:
    rules:
      - id: test-route
        path: /test/*
        serviceName: test-service
        loadBalance: round_robin
`
		
		if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
			t.Fatal(err)
		}
		
		// Wait for reload
		time.Sleep(300 * time.Millisecond)
		
		if configChanges != 1 {
			t.Errorf("Expected 1 config change, got %d", configChanges)
		}
		
		if lastConfig == nil || lastConfig.Gateway.Frontend.HTTP.Port != 8081 {
			t.Error("Config not updated correctly")
		}
	})
	
	// Test 2: Multiple rapid changes (should debounce)
	t.Run("Debouncing", func(t *testing.T) {
		configChanges = 0
		
		// Make 3 rapid changes
		for i := 0; i < 3; i++ {
			config := `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: ` + strconv.Itoa(8082+i) + `
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
      maxIdleConnsPerHost: 10
      idleConnTimeout: 90
      keepAlive: true
  registry:
    type: static
    static:
      services:
        - name: test-service
          instances:
            - id: test-1
              address: "127.0.0.1"
              port: 3000
              health: healthy
  router:
    rules:
      - id: test-route
        path: /test/*
        serviceName: test-service
        loadBalance: round_robin
`
			if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
				t.Fatal(err)
			}
			time.Sleep(50 * time.Millisecond) // Less than debounce duration
		}
		
		// Wait for debounce
		time.Sleep(300 * time.Millisecond)
		
		// Should only trigger once due to debouncing
		if configChanges != 1 {
			t.Errorf("Expected 1 config change after debouncing, got %d", configChanges)
		}
	})
	
	// Test 3: File removal and recreation
	t.Run("FileRecreation", func(t *testing.T) {
		configChanges = 0
		
		// Remove file
		if err := os.Remove(configPath); err != nil {
			t.Fatal(err)
		}
		
		time.Sleep(200 * time.Millisecond)
		
		// Recreate file
		if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
			t.Fatal(err)
		}
		
		// Wait for reload
		time.Sleep(300 * time.Millisecond)
		
		if configChanges != 1 {
			t.Errorf("Expected 1 config change after recreation, got %d", configChanges)
		}
	})
}

func TestWatcherValidation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	
	// Test invalid config
	invalidConfig := `
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: -1  # Invalid port
      readTimeout: 30
      writeTimeout: 30
  backend:
    http:
      maxIdleConns: 100
  registry:
    type: static
    static:
      services: []
  router:
    rules: []
`
	
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatal(err)
	}
	
	errorCount := 0
	watcherConfig := &WatcherConfig{
		OnChange: func(cfg *Config) error {
			t.Error("Should not call OnChange for invalid config")
			return nil
		},
		OnError: func(err error) {
			errorCount++
		},
	}
	
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	watcher, err := NewWatcher(configPath, watcherConfig, logger)
	if err != nil {
		t.Fatal(err)
	}
	watcher.Start()
	defer watcher.Stop()
	
	// Update to trigger reload
	if err := os.WriteFile(configPath, []byte(invalidConfig+"# comment"), 0644); err != nil {
		t.Fatal(err)
	}
	
	time.Sleep(300 * time.Millisecond)
	
	if errorCount == 0 {
		t.Error("Expected validation error")
	}
}