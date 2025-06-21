package factory

import (
	"net/http"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/metrics"
	metricsMiddleware "gateway/internal/middleware/metrics"
)

// CreateMetrics creates a new metrics instance
func CreateMetrics() *metrics.Metrics {
	return metrics.New()
}

// CreateMetricsHandler creates the Prometheus metrics HTTP handler
func CreateMetricsHandler() http.Handler {
	return metrics.Handler()
}

// CreateMetricsMiddleware creates metrics collection middleware
func CreateMetricsMiddleware(m *metrics.Metrics) core.Middleware {
	return metricsMiddleware.Middleware(m)
}

// ShouldEnableMetrics checks if metrics should be enabled based on config
func ShouldEnableMetrics(cfg *config.Metrics) bool {
	return cfg != nil && cfg.Enabled
}
