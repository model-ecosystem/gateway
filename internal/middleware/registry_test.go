package middleware

import (
	"context"
	"log/slog"
	"testing"
	"time"
	
	"gateway/internal/config"
	"gateway/internal/storage"
)

// mockStore implements storage.LimiterStore for testing
type mockStore struct{}

// Ensure mockStore implements storage.LimiterStore
var _ storage.LimiterStore = (*mockStore)(nil)

func (m *mockStore) Allow(ctx context.Context, key string, limit, burst int, window time.Duration) (bool, int, time.Time, error) {
	return true, limit, time.Now().Add(window), nil
}

func (m *mockStore) AllowN(ctx context.Context, key string, n, limit, burst int, window time.Duration) (bool, int, time.Time, error) {
	return true, limit, time.Now().Add(window), nil
}

func (m *mockStore) Reset(ctx context.Context, key string) error {
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

func TestRegistry_RegisterAll(t *testing.T) {
	logger := slog.Default()
	registry := NewRegistry(logger)
	store := &mockStore{}
	
	// Register all middleware
	if err := registry.RegisterAll(store); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	
	// Check that all middleware are registered
	components := registry.List()
	expectedComponents := []string{"auth", "circuitbreaker", "cors", "ratelimit", "retry"}
	
	if len(components) != len(expectedComponents) {
		t.Errorf("Expected %d components, got %d", len(expectedComponents), len(components))
	}
	
	// Verify each component is registered
	componentMap := make(map[string]bool)
	for _, c := range components {
		componentMap[c] = true
	}
	
	for _, expected := range expectedComponents {
		if !componentMap[expected] {
			t.Errorf("Expected component %s not found", expected)
		}
	}
}

func TestRegistry_Create(t *testing.T) {
	logger := slog.Default()
	registry := NewRegistry(logger)
	store := &mockStore{}
	
	// Register all middleware
	if err := registry.RegisterAll(store); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	
	// Test creating CORS middleware
	corsConfig := config.CORS{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
	}
	
	component, err := registry.Create("cors", corsConfig)
	if err != nil {
		t.Fatalf("Create(cors) error = %v", err)
	}
	
	if component == nil {
		t.Error("Create(cors) returned nil component")
	}
	
	// Test creating non-existent middleware
	_, err = registry.Create("non-existent", nil)
	if err == nil {
		t.Error("Expected error for non-existent component")
	}
}