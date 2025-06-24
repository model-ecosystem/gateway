package cors

import (
	"testing"
	
	"gateway/pkg/factory"
)

func TestComponent_Implementation(t *testing.T) {
	// Test that Component implements factory.Component
	var _ factory.Component = (*Component)(nil)
}

func TestComponent_Init(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name: "default config",
			config: map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "custom config",
			config: map[string]interface{}{
				"allowedOrigins": []string{"https://example.com"},
				"allowedMethods": []string{"GET", "POST"},
				"allowCredentials": true,
				"maxAge": 3600,
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			component := NewComponent()
			
			// Use factory.Build to initialize and validate
			result, err := factory.Build(component, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// Skip further validation if error was expected
			if tt.wantErr {
				return
			}
			
			// Verify the component was properly initialized
			if comp, ok := result.(*Component); ok {
				if comp.cors == nil {
					t.Error("CORS instance was not initialized")
				}
			} else {
				t.Error("Result is not a *Component")
			}
		})
	}
}

func TestComponent_Build(t *testing.T) {
	component := NewComponent()
	
	// Initialize with default config
	parser := func(v interface{}) error {
		return nil
	}
	
	if err := component.Init(parser); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	
	// Build middleware
	middleware := component.(*Component).Build()
	if middleware == nil {
		t.Error("Build() returned nil middleware")
	}
}

func TestComponent_BuildPanicsWhenNotInitialized(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Build() should panic when component is not initialized")
		}
	}()
	
	component := &Component{}
	component.Build()
}