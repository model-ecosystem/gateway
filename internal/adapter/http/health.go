package http

import (
	"net/http"

	"gateway/internal/health"
)

// HealthConfig represents health check configuration
type HealthConfig struct {
	Enabled       bool
	HealthPath    string
	ReadyPath     string
	LivePath      string
	HealthHandler *health.Handler
}

// DefaultHealthConfig returns default health configuration
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Enabled:    true,
		HealthPath: "/health",
		ReadyPath:  "/ready",
		LivePath:   "/live",
	}
}

// SetupHealthRoutes sets up health check routes
func (a *Adapter) SetupHealthRoutes(mux *http.ServeMux, config HealthConfig) {
	if !config.Enabled || config.HealthHandler == nil {
		return
	}

	// Register health endpoints
	mux.HandleFunc(config.HealthPath, config.HealthHandler.Health)
	mux.HandleFunc(config.ReadyPath, config.HealthHandler.Ready)
	mux.HandleFunc(config.LivePath, config.HealthHandler.Live)

	a.logger.Info("Health check endpoints registered",
		"health", config.HealthPath,
		"ready", config.ReadyPath,
		"live", config.LivePath,
	)
}
