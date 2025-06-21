package config

import (
	"fmt"
	"os"

	"gateway/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Loader loads configuration from file
type Loader struct {
	path       string
	envEnabled bool
}

// NewLoader creates a config loader
func NewLoader(path string) *Loader {
	return &Loader{
		path:       path,
		envEnabled: true, // Enable env vars by default
	}
}

// WithEnvVars enables or disables environment variable loading
func (l *Loader) WithEnvVars(enabled bool) *Loader {
	l.envEnabled = enabled
	return l
}

// Load loads the configuration
func (l *Loader) Load() (*Config, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, errors.NewError(errors.ErrorTypeInternal, "failed to read config file").WithCause(err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, errors.NewError(errors.ErrorTypeInternal, "failed to parse config").WithCause(err)
	}

	// Override with environment variables if enabled
	if l.envEnabled {
		if err := LoadEnv(&cfg); err != nil {
			return nil, errors.NewError(errors.ErrorTypeInternal, "failed to load env vars").WithCause(err)
		}
	}

	// Validate configuration
	if err := l.validate(&cfg); err != nil {
		return nil, errors.NewError(errors.ErrorTypeBadRequest, "invalid configuration").WithCause(err)
	}

	return &cfg, nil
}

// validate validates the configuration
func (l *Loader) validate(cfg *Config) error {
	// Validate required fields
	if cfg.Gateway.Frontend.HTTP.Port == 0 {
		return errors.NewError(errors.ErrorTypeBadRequest, "frontend HTTP port is required")
	}

	if cfg.Gateway.Registry.Type == "" {
		return errors.NewError(errors.ErrorTypeBadRequest, "registry type is required")
	}

	// Validate registry
	switch cfg.Gateway.Registry.Type {
	case "static":
		if cfg.Gateway.Registry.Static == nil {
			return fmt.Errorf("static registry configuration is required")
		}
	case "docker":
		if cfg.Gateway.Registry.Docker == nil {
			return fmt.Errorf("docker registry configuration is required")
		}
	default:
		return fmt.Errorf("unknown registry type: %s", cfg.Gateway.Registry.Type)
	}

	// Validate routes
	if len(cfg.Gateway.Router.Rules) == 0 {
		return fmt.Errorf("at least one route rule is required")
	}

	for i, rule := range cfg.Gateway.Router.Rules {
		if rule.ID == "" {
			return fmt.Errorf("route rule %d: ID is required", i)
		}
		if rule.Path == "" {
			return fmt.Errorf("route rule %d: path is required", i)
		}
		if rule.ServiceName == "" {
			return fmt.Errorf("route rule %d: service name is required", i)
		}
	}

	return nil
}
