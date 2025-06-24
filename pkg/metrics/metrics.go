package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the gateway
type Metrics struct {
	// HTTP metrics
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestSize     *prometheus.HistogramVec
	ResponseSize    *prometheus.HistogramVec
	ActiveRequests  *prometheus.GaugeVec

	// Backend metrics
	BackendRequestsTotal   *prometheus.CounterVec
	BackendRequestDuration *prometheus.HistogramVec
	BackendErrors          *prometheus.CounterVec

	// WebSocket metrics
	WebSocketConnections      *prometheus.GaugeVec
	WebSocketConnectionsTotal *prometheus.CounterVec
	WebSocketMessagesSent     *prometheus.CounterVec
	WebSocketMessagesReceived *prometheus.CounterVec

	// SSE metrics
	SSEConnections      *prometheus.GaugeVec
	SSEConnectionsTotal *prometheus.CounterVec
	SSEEventsSent       *prometheus.CounterVec

	// Health check metrics
	HealthCheckDuration *prometheus.HistogramVec
	HealthCheckStatus   *prometheus.GaugeVec

	// Rate limiting metrics
	RateLimitHits     *prometheus.CounterVec
	RateLimitRejected *prometheus.CounterVec

	// Service discovery metrics
	ServiceInstances *prometheus.GaugeVec
}

// New creates a new Metrics instance with all metrics registered
func New() *Metrics {
	return NewWithRegistry(prometheus.DefaultRegisterer, prometheus.DefaultGatherer)
}

// NewWithRegistry creates a new Metrics instance with a custom registry
func NewWithRegistry(registerer prometheus.Registerer, gatherer prometheus.Gatherer) *Metrics {
	factory := promauto.With(registerer)

	return &Metrics{
		// HTTP metrics
		RequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		RequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_http_request_duration_seconds",
				Help:    "HTTP request latencies in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		RequestSize: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_http_request_size_bytes",
				Help:    "HTTP request sizes in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 7), // 100B to 100MB
			},
			[]string{"method", "path"},
		),
		ResponseSize: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_http_response_size_bytes",
				Help:    "HTTP response sizes in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 7), // 100B to 100MB
			},
			[]string{"method", "path"},
		),
		ActiveRequests: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_http_requests_active",
				Help: "Number of active HTTP requests",
			},
			[]string{"method", "path"},
		),

		// Backend metrics
		BackendRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_backend_requests_total",
				Help: "Total number of backend requests",
			},
			[]string{"service", "instance", "method", "status"},
		),
		BackendRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_backend_request_duration_seconds",
				Help:    "Backend request latencies in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "instance", "method"},
		),
		BackendErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_backend_errors_total",
				Help: "Total number of backend errors",
			},
			[]string{"service", "instance", "error_type"},
		),

		// WebSocket metrics
		WebSocketConnections: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_websocket_connections_active",
				Help: "Number of active WebSocket connections",
			},
			[]string{"service"},
		),
		WebSocketConnectionsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_websocket_connections_total",
				Help: "Total number of WebSocket connections",
			},
			[]string{"service", "status"},
		),
		WebSocketMessagesSent: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_websocket_messages_sent_total",
				Help: "Total number of WebSocket messages sent",
			},
			[]string{"service"},
		),
		WebSocketMessagesReceived: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_websocket_messages_received_total",
				Help: "Total number of WebSocket messages received",
			},
			[]string{"service"},
		),

		// SSE metrics
		SSEConnections: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_sse_connections_active",
				Help: "Number of active SSE connections",
			},
			[]string{"service"},
		),
		SSEConnectionsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_sse_connections_total",
				Help: "Total number of SSE connections",
			},
			[]string{"service", "status"},
		),
		SSEEventsSent: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_sse_events_sent_total",
				Help: "Total number of SSE events sent",
			},
			[]string{"service"},
		),

		// Health check metrics
		HealthCheckDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_health_check_duration_seconds",
				Help:    "Health check durations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"check_name"},
		),
		HealthCheckStatus: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_health_check_status",
				Help: "Health check status (1 = healthy, 0 = unhealthy)",
			},
			[]string{"check_name"},
		),

		// Rate limiting metrics
		RateLimitHits: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_rate_limit_hits_total",
				Help: "Total number of rate limit hits",
			},
			[]string{"route", "limit_type"},
		),
		RateLimitRejected: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_rate_limit_rejected_total",
				Help: "Total number of requests rejected due to rate limiting",
			},
			[]string{"route", "limit_type"},
		),

		// Service discovery metrics
		ServiceInstances: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_service_instances",
				Help: "Number of instances per service",
			},
			[]string{"service", "health"},
		),
	}
}

// NormalizePath normalizes the path for metrics labels to avoid high cardinality
func NormalizePath(path string) string {
	// Simple normalization - in production, you'd want more sophisticated logic
	// to handle path parameters, query strings, etc.
	const maxLength = 50
	if len(path) > maxLength {
		// Ensure we have room for "..."
		return path[:maxLength] + "..."
	}
	return path
}
