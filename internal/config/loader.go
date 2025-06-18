package config

import (
	"fmt"
	"os"
	
	"gopkg.in/yaml.v3"
)

// Loader loads configuration from file
type Loader struct {
	path string
}

// NewLoader creates a config loader
func NewLoader(path string) *Loader {
	return &Loader{path: path}
}

// Load loads the configuration
func (l *Loader) Load() (*Config, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}