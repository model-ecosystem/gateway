// Package factory provides a generic framework for component creation and initialization
package factory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
)

// Component is the base interface that all components must implement
type Component interface {
	// Init initializes the component with configuration
	Init(parser ConfigParser) error
	
	// Name returns the component name for logging and identification
	Name() string
	
	// Validate validates the component state after initialization
	Validate() error
}

// ConfigParser is a function that parses configuration into the provided structure
// Components call this to extract their specific configuration
type ConfigParser func(v any) error

// Build creates and initializes a component with the provided configuration
func Build[T Component](component T, config any) (T, error) {
	var zero T
	
	// Create parser function that will unmarshal config into the target
	parser := func(v any) error {
		return parseConfig(config, v)
	}
	
	// Initialize the component
	if err := component.Init(parser); err != nil {
		return zero, fmt.Errorf("init %s: %w", component.Name(), err)
	}
	
	// Validate the initialized component
	if err := component.Validate(); err != nil {
		return zero, fmt.Errorf("validate %s: %w", component.Name(), err)
	}
	
	return component, nil
}

// BuildWithLogger creates and initializes a component with configuration and logger
func BuildWithLogger[T Component](component T, config any, logger *slog.Logger) (T, error) {
	logger.Debug("Building component", "name", component.Name())
	
	result, err := Build(component, config)
	if err != nil {
		logger.Error("Failed to build component", "name", component.Name(), "error", err)
		return result, err
	}
	
	logger.Info("Component built successfully", "name", component.Name())
	return result, nil
}

// parseConfig fills the target structure with configuration data
func parseConfig(source any, target any) error {
	// If source is already the correct type, use direct assignment
	if reflect.TypeOf(source) == reflect.TypeOf(target).Elem() {
		reflect.ValueOf(target).Elem().Set(reflect.ValueOf(source))
		return nil
	}
	
	// Otherwise, use JSON marshaling as intermediate format
	// This handles most common cases and is flexible
	data, err := json.Marshal(source)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	
	return nil
}

// Optional interfaces for advanced use cases

// Lifecycle provides lifecycle hooks for components
type Lifecycle interface {
	Component
	// Start starts the component
	Start() error
	// Stop stops the component gracefully
	Stop() error
}

// HealthChecker provides health check capability
type HealthChecker interface {
	Component
	// Health checks if the component is healthy
	Health() error
}

// Configurable provides direct configuration access
type Configurable interface {
	Component
	// SetDefaults sets default configuration values
	SetDefaults()
	// GetConfig returns the current configuration
	GetConfig() any
}