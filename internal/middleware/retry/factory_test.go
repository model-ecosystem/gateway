package retry

import (
	"log/slog"
	"testing"
	
	"gateway/internal/config"
	"gateway/pkg/factory"
)

func TestComponent_Implementation(t *testing.T) {
	// Test that Component implements factory.Component
	var _ factory.Component = (*Component)(nil)
}

func TestComponent_Init(t *testing.T) {
	logger := slog.Default()
	
	tests := []struct {
		name    string
		config  config.Retry
		wantErr bool
		errMsg  string
	}{
		{
			name: "disabled",
			config: config.Retry{
				Enabled: false,
			},
			wantErr: true,
			errMsg:  "retry middleware is not enabled",
		},
		{
			name: "enabled with defaults",
			config: config.Retry{
				Enabled: true,
				Default: config.RetryConfig{
					MaxAttempts: 3,
				},
			},
			wantErr: false,
		},
		{
			name: "enabled with custom config",
			config: config.Retry{
				Enabled: true,
				Default: config.RetryConfig{
					MaxAttempts:  5,
					InitialDelay: 200,
					MaxDelay:     10000,
					Multiplier:   1.5,
					Jitter:       true,
					BudgetRatio:  0.2,
				},
			},
			wantErr: false,
		},
		{
			name: "with route specific config",
			config: config.Retry{
				Enabled: true,
				Default: config.RetryConfig{
					MaxAttempts: 3,
				},
				Routes: map[string]config.RetryConfig{
					"/api/v1/*": {
						MaxAttempts: 5,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "with service specific config",
			config: config.Retry{
				Enabled: true,
				Default: config.RetryConfig{
					MaxAttempts: 3,
				},
				Services: map[string]config.RetryConfig{
					"backend-service": {
						MaxAttempts: 2,
					},
				},
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			component := NewComponent(logger)
			
			// Use factory.Build to initialize and validate
			_, err := factory.Build(component, tt.config)
			
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestComponent_Build(t *testing.T) {
	logger := slog.Default()
	component := NewComponent(logger)
	
	// Initialize with valid config
	config := config.Retry{
		Enabled: true,
		Default: config.RetryConfig{
			MaxAttempts: 3,
		},
	}
	
	result, err := factory.Build(component, config)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	
	// Build middleware
	retryComponent := result.(*Component)
	middleware := retryComponent.Build()
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

func TestComponent_Validate(t *testing.T) {
	logger := slog.Default()
	
	tests := []struct {
		name    string
		config  config.Retry
		wantErr bool
	}{
		{
			name: "valid config",
			config: config.Retry{
				Enabled: true,
				Default: config.RetryConfig{
					MaxAttempts: 3,
				},
			},
			wantErr: false,
		},
		{
			name: "zero max attempts uses default",
			config: config.Retry{
				Enabled: true,
				Default: config.RetryConfig{
					MaxAttempts: 0, // Will be set to default 3
				},
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			component := NewComponent(logger)
			_, err := factory.Build(component, tt.config)
			
			if tt.wantErr && err == nil {
				t.Error("Expected validation error but got none")
			} else if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}