package factory

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/registry/docker"
	"gateway/internal/registry/static"
)

// CreateRegistry creates a service registry based on configuration
func CreateRegistry(cfg *config.Registry, logger *slog.Logger) (core.ServiceRegistry, error) {
	switch cfg.Type {
	case "docker":
		return createDockerRegistry(cfg.Docker, logger)
	case "static", "":
		return createStaticRegistry(cfg.Static, logger)
	default:
		return nil, fmt.Errorf("unknown registry type: %s", cfg.Type)
	}
}

// createDockerRegistry creates a Docker-based service registry
func createDockerRegistry(cfg *config.DockerRegistry, logger *slog.Logger) (*docker.Registry, error) {
	if cfg == nil {
		cfg = &config.DockerRegistry{}
	}

	dockerConfig := &docker.Config{
		Host:            cfg.Host,
		Version:         cfg.Version,
		CertPath:        cfg.CertPath,
		LabelPrefix:     cfg.LabelPrefix,
		Network:         cfg.Network,
		RefreshInterval: cfg.RefreshInterval,
	}

	// Set defaults
	if dockerConfig.LabelPrefix == "" {
		dockerConfig.LabelPrefix = "gateway"
	}
	if dockerConfig.RefreshInterval == 0 {
		dockerConfig.RefreshInterval = 10
	}

	return docker.NewRegistry(dockerConfig, logger)
}

// createStaticRegistry creates a static service registry
func createStaticRegistry(cfg *config.StaticRegistry, logger *slog.Logger) (*static.Registry, error) {
	return static.NewRegistry(cfg)
}
