package openapi

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gateway/internal/core"
)

func TestDescriptorLoader(t *testing.T) {
	// Create test OpenAPI spec
	specContent := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      summary: List users
      operationId: listUsers
      responses:
        '200':
          description: Success
  /users/{userId}:
    get:
      summary: Get user
      operationId: getUser
      parameters:
        - name: userId
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Success
`

	// Create temp directory
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "test-api.yaml")
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Create loader
	config := DefaultDescriptorConfig()
	config.DefaultService = "test-service"
	loader := NewDescriptorLoader(config, nil)

	// Test loading spec file
	t.Run("LoadSpecFile", func(t *testing.T) {
		err := loader.LoadSpecFile(specFile)
		if err != nil {
			t.Fatalf("Failed to load spec file: %v", err)
		}

		// Check loaded specs
		specs := loader.GetLoadedSpecs()
		if len(specs) != 1 {
			t.Errorf("Expected 1 loaded spec, got %d", len(specs))
		}

		// Check routes
		routes := loader.GetAllRoutes()
		if len(routes) != 2 {
			t.Errorf("Expected 2 routes, got %d", len(routes))
		}

		// Verify route details
		foundList := false
		foundGet := false
		for _, route := range routes {
			if route.Path == "/users" {
				foundList = true
				if route.ServiceName != "test-service" {
					t.Errorf("Expected service name test-service, got %s", route.ServiceName)
				}
			}
			if route.Path == "/users/*" { // Converted from {userId}
				foundGet = true
			}
		}

		if !foundList {
			t.Error("Did not find /users route")
		}
		if !foundGet {
			t.Error("Did not find /users/* route")
		}
	})

	// Test loading directory
	t.Run("LoadSpecDirectory", func(t *testing.T) {
		// Create another spec file
		spec2File := filepath.Join(tmpDir, "test-api-v2.yaml")
		spec2Content := `openapi: 3.0.0
info:
  title: Test API V2
  version: 2.0.0
paths:
  /v2/users:
    get:
      summary: List users V2
      responses:
        '200':
          description: Success
`
		if err := os.WriteFile(spec2File, []byte(spec2Content), 0644); err != nil {
			t.Fatalf("Failed to write spec2 file: %v", err)
		}

		// Load directory
		loader2 := NewDescriptorLoader(config, nil)
		err := loader2.LoadSpecDirectory(tmpDir)
		if err != nil {
			t.Fatalf("Failed to load spec directory: %v", err)
		}

		// Check loaded specs
		specs := loader2.GetLoadedSpecs()
		if len(specs) != 2 {
			t.Errorf("Expected 2 loaded specs, got %d", len(specs))
		}

		// Check routes
		routes := loader2.GetAllRoutes()
		if len(routes) != 3 { // 2 from first spec + 1 from second
			t.Errorf("Expected 3 routes, got %d", len(routes))
		}
	})

	// Test reload callback
	t.Run("ReloadCallback", func(t *testing.T) {
		loader3 := NewDescriptorLoader(config, nil)
		
		reloadCalled := false
		var reloadedSource string
		var reloadedRoutes []core.RouteRule
		
		loader3.SetReloadCallback(func(source string, routes []core.RouteRule) {
			reloadCalled = true
			reloadedSource = source
			reloadedRoutes = routes
		})

		err := loader3.LoadSpecFile(specFile)
		if err != nil {
			t.Fatalf("Failed to load spec file: %v", err)
		}

		if !reloadCalled {
			t.Error("Reload callback was not called")
		}
		if reloadedSource == "" {
			t.Error("Reload callback did not receive source")
		}
		if len(reloadedRoutes) != 2 {
			t.Errorf("Reload callback received %d routes, expected 2", len(reloadedRoutes))
		}
	})

	// Test file extensions
	t.Run("FileExtensions", func(t *testing.T) {
		loader4 := NewDescriptorLoader(config, nil)
		
		// Test valid extensions
		validExts := []string{".yaml", ".yml", ".json"}
		for _, ext := range validExts {
			if !loader4.isOpenAPIFile("test" + ext) {
				t.Errorf("Expected %s to be recognized as OpenAPI file", ext)
			}
		}

		// Test invalid extensions
		invalidExts := []string{".txt", ".xml", ".proto"}
		for _, ext := range invalidExts {
			if loader4.isOpenAPIFile("test" + ext) {
				t.Errorf("Expected %s to NOT be recognized as OpenAPI file", ext)
			}
		}
	})
}

func TestDescriptorManager(t *testing.T) {
	// Create test spec
	specContent := `openapi: 3.0.0
info:
  title: Manager Test API
  version: 1.0.0
paths:
  /test:
    get:
      summary: Test endpoint
      responses:
        '200':
          description: Success
`

	// Create temp directory
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "manager-test.yaml")
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Create manager
	config := DefaultDescriptorConfig()
	config.SpecFiles = []string{specFile}
	config.DefaultService = "test-service"
	
	manager := NewDescriptorManager(config, nil)

	// Test route updater
	t.Run("RouteUpdater", func(t *testing.T) {
		updateCalled := false
		var updatedSource string
		var updatedRoutes []core.RouteRule

		updater := &mockRouteUpdater{
			updateFunc: func(source string, routes []core.RouteRule) error {
				updateCalled = true
				updatedSource = source
				updatedRoutes = routes
				return nil
			},
		}

		manager.SetRouteUpdater(updater)

		// Start manager
		err := manager.Start()
		if err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		defer manager.Stop()

		// Give it time to load
		time.Sleep(100 * time.Millisecond)

		if !updateCalled {
			t.Error("Route updater was not called")
		}
		if updatedSource == "" {
			t.Error("Route updater did not receive source")
		}
		if len(updatedRoutes) != 1 {
			t.Errorf("Route updater received %d routes, expected 1", len(updatedRoutes))
		}
	})

	// Test adding spec at runtime
	t.Run("AddSpecFile", func(t *testing.T) {
		manager2 := NewDescriptorManager(DefaultDescriptorConfig(), nil)
		
		updateCount := 0
		updater := &mockRouteUpdater{
			updateFunc: func(source string, routes []core.RouteRule) error {
				updateCount++
				return nil
			},
		}
		manager2.SetRouteUpdater(updater)

		err := manager2.Start()
		if err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		defer manager2.Stop()

		// Add spec file
		err = manager2.AddSpecFile(specFile)
		if err != nil {
			t.Fatalf("Failed to add spec file: %v", err)
		}

		// Check that routes were updated
		if updateCount != 1 {
			t.Errorf("Expected 1 route update, got %d", updateCount)
		}

		// Check routes
		routes := manager2.GetAllRoutes()
		if len(routes) != 1 {
			t.Errorf("Expected 1 route, got %d", len(routes))
		}
	})

	// Test removing spec
	t.Run("RemoveSpecFile", func(t *testing.T) {
		manager3 := NewDescriptorManager(DefaultDescriptorConfig(), nil)
		
		removeCalled := false
		var removedSource string
		
		updater := &mockRouteUpdater{
			updateFunc: func(source string, routes []core.RouteRule) error {
				return nil
			},
			removeFunc: func(source string) error {
				removeCalled = true
				removedSource = source
				return nil
			},
		}
		manager3.SetRouteUpdater(updater)

		err := manager3.Start()
		if err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		defer manager3.Stop()

		// Add then remove spec
		err = manager3.AddSpecFile(specFile)
		if err != nil {
			t.Fatalf("Failed to add spec file: %v", err)
		}

		err = manager3.RemoveSpecFile(specFile)
		if err != nil {
			t.Fatalf("Failed to remove spec file: %v", err)
		}

		if !removeCalled {
			t.Error("Remove routes was not called")
		}
		if removedSource != specFile {
			t.Errorf("Expected removed source %s, got %s", specFile, removedSource)
		}
	})
}

// mockRouteUpdater implements RouteUpdater for testing
type mockRouteUpdater struct {
	updateFunc func(source string, routes []core.RouteRule) error
	removeFunc func(source string) error
}

func (m *mockRouteUpdater) UpdateRoutes(source string, routes []core.RouteRule) error {
	if m.updateFunc != nil {
		return m.updateFunc(source, routes)
	}
	return nil
}

func (m *mockRouteUpdater) RemoveRoutes(source string) error {
	if m.removeFunc != nil {
		return m.removeFunc(source)
	}
	return nil
}