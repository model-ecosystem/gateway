package auth

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
		config  config.Auth
		wantErr bool
		errMsg  string
	}{
		{
			name: "no providers",
			config: config.Auth{
				Required: true,
			},
			wantErr: true,
			errMsg:  "no auth providers configured",
		},
		{
			name: "jwt provider enabled",
			config: config.Auth{
				Required:  true,
				Providers: []string{"jwt"},
				JWT: &config.JWTConfig{
					Enabled:       true,
					SigningMethod: "HS256",
					Secret:        "test-secret",
				},
			},
			wantErr: false,
		},
		{
			name: "api key provider enabled",
			config: config.Auth{
				Required:  true,
				Providers: []string{"apikey"},
				APIKey: &config.APIKeyConfig{
					Enabled: true,
					Keys: map[string]*config.APIKeyDetails{
						"test-key": {
							Subject: "test-user",
							Scopes:  []string{"read"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "both providers enabled",
			config: config.Auth{
				Required:  true,
				Providers: []string{"jwt", "apikey"},
				JWT: &config.JWTConfig{
					Enabled:       true,
					SigningMethod: "HS256",
					Secret:        "test-secret",
				},
				APIKey: &config.APIKeyConfig{
					Enabled: true,
					Keys: map[string]*config.APIKeyDetails{
						"test-key": {
							Subject: "test-user",
							Scopes:  []string{"read"},
						},
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
	config := config.Auth{
		Required:  true,
		Providers: []string{"jwt"},
		JWT: &config.JWTConfig{
			Enabled:       true,
			SigningMethod: "HS256",
			Secret:        "test-secret",
		},
	}
	
	result, err := factory.Build(component, config)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	
	// Build middleware
	authComponent := result.(*Component)
	middleware := authComponent.Build()
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