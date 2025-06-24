package factory

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/metrics"
	"gateway/internal/telemetry"
)

// TelemetryFactory creates telemetry and metrics instances
type TelemetryFactory struct {
	BaseComponentFactory
}

// NewTelemetryFactory creates a new telemetry factory
func NewTelemetryFactory(logger *slog.Logger) *TelemetryFactory {
	return &TelemetryFactory{
		BaseComponentFactory: NewBaseComponentFactory(logger),
	}
}

// CreateTelemetry creates telemetry instance from configuration
func (f *TelemetryFactory) CreateTelemetry(cfg *config.Telemetry) (*telemetry.Telemetry, *telemetry.Metrics, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil, nil
	}

	// Convert config to telemetry config
	telemetryConfig := telemetry.Config{
		Enabled: cfg.Enabled,
		Service: cfg.Service,
		Version: cfg.Version,
		Tracing: telemetry.TracingConfig{
			Enabled:      cfg.Tracing.Enabled,
			Endpoint:     cfg.Tracing.Endpoint,
			Headers:      cfg.Tracing.Headers,
			SampleRate:   cfg.Tracing.SampleRate,
			MaxBatchSize: cfg.Tracing.MaxBatchSize,
			BatchTimeout: cfg.Tracing.BatchTimeout,
		},
		Metrics: telemetry.MetricsConfig{
			Enabled: cfg.Metrics.Enabled,
		},
	}
	
	gatewayTelemetry, err := telemetry.New(telemetryConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("creating telemetry: %w", err)
	}
	
	// Create telemetry metrics if enabled
	var telemetryMetrics *telemetry.Metrics
	if cfg.Metrics.Enabled {
		telemetryMetrics, err = gatewayTelemetry.NewMetrics()
		if err != nil {
			return nil, nil, fmt.Errorf("creating telemetry metrics: %w", err)
		}
	}
	
	f.logger.Info("Telemetry enabled", "service", cfg.Service, "version", cfg.Version)
	return gatewayTelemetry, telemetryMetrics, nil
}

// CreateMetrics creates metrics instance from configuration
func (f *TelemetryFactory) CreateMetrics(cfg *config.Metrics) *metrics.Metrics {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	
	return metrics.New()
}

// ShouldEnableMetrics checks if metrics should be enabled
func (f *TelemetryFactory) ShouldEnableMetrics(cfg *config.Metrics) bool {
	return cfg != nil && cfg.Enabled
}