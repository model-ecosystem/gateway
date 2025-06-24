package factory_test

import (
	"errors"
	"testing"

	"gateway/pkg/factory"
)

// Mock component for testing
type mockComponent struct {
	name       string
	initCalled bool
	initError  error
	config     *mockConfig
}

type mockConfig struct {
	Value    string
	Required bool
}

func (m *mockComponent) Init(parser factory.ConfigParser) error {
	m.initCalled = true
	if m.initError != nil {
		return m.initError
	}
	
	var cfg mockConfig
	if err := parser(&cfg); err != nil {
		return err
	}
	
	m.config = &cfg
	return nil
}

func (m *mockComponent) Name() string {
	return m.name
}

func (m *mockComponent) Validate() error {
	if m.config == nil {
		return errors.New("not initialized")
	}
	if m.config.Required && m.config.Value == "" {
		return errors.New("value is required")
	}
	return nil
}

func TestBuild(t *testing.T) {
	tests := []struct {
		name      string
		component *mockComponent
		config    mockConfig
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "successful build",
			component: &mockComponent{name: "test"},
			config:    mockConfig{Value: "test", Required: true},
			wantErr:   false,
		},
		{
			name:      "init error",
			component: &mockComponent{name: "test", initError: errors.New("init failed")},
			config:    mockConfig{},
			wantErr:   true,
			errMsg:    "init test: init failed",
		},
		{
			name:      "validation error",
			component: &mockComponent{name: "test"},
			config:    mockConfig{Required: true}, // Value is empty
			wantErr:   true,
			errMsg:    "validate test: value is required",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := factory.Build(tt.component, tt.config)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("Build() error = %v, want %v", err, tt.errMsg)
			}
			
			if !tt.wantErr && !result.initCalled {
				t.Error("Init() was not called")
			}
			
			if !tt.wantErr && result.config.Value != tt.config.Value {
				t.Errorf("Config not properly set: got %v, want %v", result.config.Value, tt.config.Value)
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	registry := factory.NewRegistry()
	
	// Test registration
	err := registry.Register("test", func() factory.Component {
		return &mockComponent{name: "test"}
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	
	// Test duplicate registration
	err = registry.Register("test", func() factory.Component {
		return &mockComponent{name: "test2"}
	})
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
	
	// Test creation
	component, err := registry.Create("test", mockConfig{Value: "test"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	
	if component.Name() != "test" {
		t.Errorf("Component name = %v, want %v", component.Name(), "test")
	}
	
	// Test non-existent component
	_, err = registry.Create("nonexistent", mockConfig{})
	if err == nil {
		t.Error("Expected error for non-existent component")
	}
	
	// Test list
	names := registry.List()
	if len(names) != 1 || names[0] != "test" {
		t.Errorf("List() = %v, want [test]", names)
	}
}