package config

import (
	_ "embed"
	"gopkg.in/yaml.v3"
)

//go:embed default.yaml
var defaultConfigYAML string

// LoadDefault loads the default embedded configuration
func LoadDefault() (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(defaultConfigYAML), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}