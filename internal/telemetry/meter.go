package telemetry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Metrics holds all gateway metrics
type Metrics struct {
	meter metric.Meter
	
	// HTTP metrics
	httpRequestsTotal    metric.Int64Counter
	httpRequestDuration  metric.Float64Histogram
	httpActiveRequests   metric.Int64UpDownCounter
	httpRequestSize      metric.Int64Histogram
	httpResponseSize     metric.Int64Histogram
	
	// WebSocket metrics
	wsConnectionsTotal   metric.Int64Counter
	wsActiveConnections  metric.Int64UpDownCounter
	wsMessagesSent       metric.Int64Counter
	wsMessagesReceived   metric.Int64Counter
	wsBytesSent          metric.Int64Counter
	wsBytesReceived      metric.Int64Counter
	
	// SSE metrics
	sseConnectionsTotal  metric.Int64Counter
	sseActiveConnections metric.Int64UpDownCounter
	sseEventsSent        metric.Int64Counter
	sseBytesSent         metric.Int64Counter
	
	// Backend metrics
	backendRequestsTotal   metric.Int64Counter
	backendRequestDuration metric.Float64Histogram
	backendActiveRequests  metric.Int64UpDownCounter
	backendErrors          metric.Int64Counter
	
	// Circuit breaker metrics
	circuitBreakerState    metric.Int64ObservableGauge
	circuitBreakerFailures metric.Int64Counter
	
	// Rate limit metrics
	rateLimitRequests      metric.Int64Counter
	rateLimitExceeded      metric.Int64Counter
	
	// Connection pool metrics
	poolActiveConnections  metric.Int64UpDownCounter
	poolIdleConnections    metric.Int64UpDownCounter
	poolWaitDuration       metric.Float64Histogram
	
	// Service discovery metrics
	serviceInstances       metric.Int64ObservableGauge
	serviceHealthy         metric.Int64ObservableGauge
	
	// Custom metric callbacks
	callbacks []metric.Registration
	mu        sync.RWMutex
}

// NewMetrics creates all metrics
func (t *Telemetry) NewMetrics() (*Metrics, error) {
	m := &Metrics{
		meter:     t.meter,
		callbacks: make([]metric.Registration, 0),
	}
	
	var err error
	
	// HTTP metrics
	m.httpRequestsTotal, err = t.meter.Int64Counter(
		"gateway_http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_requests_total: %w", err)
	}
	
	m.httpRequestDuration, err = t.meter.Float64Histogram(
		"gateway_http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_request_duration: %w", err)
	}
	
	m.httpActiveRequests, err = t.meter.Int64UpDownCounter(
		"gateway_http_active_requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_active_requests: %w", err)
	}
	
	m.httpRequestSize, err = t.meter.Int64Histogram(
		"gateway_http_request_size_bytes",
		metric.WithDescription("HTTP request size in bytes"),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(100, 1000, 10000, 100000, 1000000),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_request_size: %w", err)
	}
	
	m.httpResponseSize, err = t.meter.Int64Histogram(
		"gateway_http_response_size_bytes",
		metric.WithDescription("HTTP response size in bytes"),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(100, 1000, 10000, 100000, 1000000),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_response_size: %w", err)
	}
	
	// WebSocket metrics
	m.wsConnectionsTotal, err = t.meter.Int64Counter(
		"gateway_websocket_connections_total",
		metric.WithDescription("Total number of WebSocket connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ws_connections_total: %w", err)
	}
	
	m.wsActiveConnections, err = t.meter.Int64UpDownCounter(
		"gateway_websocket_active_connections",
		metric.WithDescription("Number of active WebSocket connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ws_active_connections: %w", err)
	}
	
	m.wsMessagesSent, err = t.meter.Int64Counter(
		"gateway_websocket_messages_sent_total",
		metric.WithDescription("Total WebSocket messages sent"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ws_messages_sent: %w", err)
	}
	
	m.wsMessagesReceived, err = t.meter.Int64Counter(
		"gateway_websocket_messages_received_total",
		metric.WithDescription("Total WebSocket messages received"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ws_messages_received: %w", err)
	}
	
	m.wsBytesSent, err = t.meter.Int64Counter(
		"gateway_websocket_bytes_sent_total",
		metric.WithDescription("Total WebSocket bytes sent"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ws_bytes_sent: %w", err)
	}
	
	m.wsBytesReceived, err = t.meter.Int64Counter(
		"gateway_websocket_bytes_received_total",
		metric.WithDescription("Total WebSocket bytes received"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ws_bytes_received: %w", err)
	}
	
	// SSE metrics
	m.sseConnectionsTotal, err = t.meter.Int64Counter(
		"gateway_sse_connections_total",
		metric.WithDescription("Total number of SSE connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sse_connections_total: %w", err)
	}
	
	m.sseActiveConnections, err = t.meter.Int64UpDownCounter(
		"gateway_sse_active_connections",
		metric.WithDescription("Number of active SSE connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sse_active_connections: %w", err)
	}
	
	m.sseEventsSent, err = t.meter.Int64Counter(
		"gateway_sse_events_sent_total",
		metric.WithDescription("Total SSE events sent"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sse_events_sent: %w", err)
	}
	
	m.sseBytesSent, err = t.meter.Int64Counter(
		"gateway_sse_bytes_sent_total",
		metric.WithDescription("Total SSE bytes sent"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sse_bytes_sent: %w", err)
	}
	
	// Backend metrics
	m.backendRequestsTotal, err = t.meter.Int64Counter(
		"gateway_backend_requests_total",
		metric.WithDescription("Total backend requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create backend_requests_total: %w", err)
	}
	
	m.backendRequestDuration, err = t.meter.Float64Histogram(
		"gateway_backend_request_duration_seconds",
		metric.WithDescription("Backend request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create backend_request_duration: %w", err)
	}
	
	// Circuit breaker metrics
	m.circuitBreakerState, err = t.meter.Int64ObservableGauge(
		"gateway_circuit_breaker_state",
		metric.WithDescription("Circuit breaker state (0=closed, 1=open, 2=half-open)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create circuit_breaker_state: %w", err)
	}
	
	// Rate limit metrics
	m.rateLimitRequests, err = t.meter.Int64Counter(
		"gateway_rate_limit_requests_total",
		metric.WithDescription("Total rate limit checks"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate_limit_requests: %w", err)
	}
	
	m.rateLimitExceeded, err = t.meter.Int64Counter(
		"gateway_rate_limit_exceeded_total",
		metric.WithDescription("Total rate limit exceeded"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate_limit_exceeded: %w", err)
	}
	
	// Service discovery metrics
	m.serviceInstances, err = t.meter.Int64ObservableGauge(
		"gateway_service_instances",
		metric.WithDescription("Number of service instances"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create service_instances: %w", err)
	}
	
	m.serviceHealthy, err = t.meter.Int64ObservableGauge(
		"gateway_service_healthy_instances",
		metric.WithDescription("Number of healthy service instances"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create service_healthy: %w", err)
	}
	
	return m, nil
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(ctx context.Context, method, route string, statusCode int, duration time.Duration) {
	attrs := []attribute.KeyValue{
		semconv.HTTPMethod(method),
		semconv.HTTPRoute(route),
		semconv.HTTPStatusCode(statusCode),
	}
	
	m.httpRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.httpRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordHTTPActiveRequest increments active HTTP requests
func (m *Metrics) RecordHTTPActiveRequest(ctx context.Context, delta int64) {
	m.httpActiveRequests.Add(ctx, delta)
}

// RecordHTTPRequestSize records HTTP request size
func (m *Metrics) RecordHTTPRequestSize(ctx context.Context, size int64) {
	m.httpRequestSize.Record(ctx, size)
}

// RecordHTTPResponseSize records HTTP response size
func (m *Metrics) RecordHTTPResponseSize(ctx context.Context, size int64) {
	m.httpResponseSize.Record(ctx, size)
}

// RecordWebSocketConnection records WebSocket connection
func (m *Metrics) RecordWebSocketConnection(ctx context.Context, route string) {
	attrs := []attribute.KeyValue{
		attribute.String("route", route),
	}
	m.wsConnectionsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordWebSocketActiveConnection updates active WebSocket connections
func (m *Metrics) RecordWebSocketActiveConnection(ctx context.Context, delta int64) {
	m.wsActiveConnections.Add(ctx, delta)
}

// RecordWebSocketMessage records WebSocket message
func (m *Metrics) RecordWebSocketMessage(ctx context.Context, direction string, size int64) {
	attrs := []attribute.KeyValue{
		attribute.String("direction", direction),
	}
	
	if direction == "sent" {
		m.wsMessagesSent.Add(ctx, 1, metric.WithAttributes(attrs...))
		m.wsBytesSent.Add(ctx, size, metric.WithAttributes(attrs...))
	} else {
		m.wsMessagesReceived.Add(ctx, 1, metric.WithAttributes(attrs...))
		m.wsBytesReceived.Add(ctx, size, metric.WithAttributes(attrs...))
	}
}

// RecordSSEConnection records SSE connection
func (m *Metrics) RecordSSEConnection(ctx context.Context, route string) {
	attrs := []attribute.KeyValue{
		attribute.String("route", route),
	}
	m.sseConnectionsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordSSEActiveConnection updates active SSE connections
func (m *Metrics) RecordSSEActiveConnection(ctx context.Context, delta int64) {
	m.sseActiveConnections.Add(ctx, delta)
}

// RecordSSEEvent records SSE event sent
func (m *Metrics) RecordSSEEvent(ctx context.Context, size int64) {
	m.sseEventsSent.Add(ctx, 1)
	m.sseBytesSent.Add(ctx, size)
}

// RecordBackendRequest records backend request
func (m *Metrics) RecordBackendRequest(ctx context.Context, service, instance string, statusCode int, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("service", service),
		attribute.String("instance", instance),
		attribute.Int("status_code", statusCode),
	}
	
	m.backendRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.backendRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	
	if statusCode >= 500 {
		m.backendErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordBackendActiveRequest updates active backend requests
func (m *Metrics) RecordBackendActiveRequest(ctx context.Context, delta int64) {
	m.backendActiveRequests.Add(ctx, delta)
}

// RecordCircuitBreakerState records circuit breaker state
func (m *Metrics) RecordCircuitBreakerState(ctx context.Context, service string, state int64) {
	attrs := []attribute.KeyValue{
		attribute.String("service", service),
	}
	
	// Use callback to set gauge value
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Store state for callback
	key := fmt.Sprintf("circuit_%s", service)
	m.setGaugeValue(key, state, attrs)
}

// RecordRateLimit records rate limit check
func (m *Metrics) RecordRateLimit(ctx context.Context, key string, allowed bool) {
	attrs := []attribute.KeyValue{
		attribute.String("key", key),
		attribute.Bool("allowed", allowed),
	}
	
	m.rateLimitRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
	if !allowed {
		m.rateLimitExceeded.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordServiceInstances records service instance count
func (m *Metrics) RecordServiceInstances(ctx context.Context, service string, total, healthy int64) {
	attrs := []attribute.KeyValue{
		attribute.String("service", service),
	}
	
	// Use callbacks to set gauge values
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.setGaugeValue(fmt.Sprintf("instances_%s", service), total, attrs)
	m.setGaugeValue(fmt.Sprintf("healthy_%s", service), healthy, attrs)
}

// gaugeValues stores gauge values for callbacks
var gaugeValues = struct {
	sync.RWMutex
	values map[string]int64
}{
	values: make(map[string]int64),
}

// setGaugeValue sets a gauge value for callback
func (m *Metrics) setGaugeValue(key string, value int64, attrs []attribute.KeyValue) {
	gaugeValues.Lock()
	gaugeValues.values[key] = value
	gaugeValues.Unlock()
}

// RegisterCallbacks registers metric callbacks
func (m *Metrics) RegisterCallbacks(meter metric.Meter) error {
	// Circuit breaker state callback
	reg, err := meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			gaugeValues.RLock()
			defer gaugeValues.RUnlock()
			
			for key, value := range gaugeValues.values {
				if key[:7] == "circuit" {
					service := key[8:]
					o.ObserveInt64(m.circuitBreakerState,
						value,
						metric.WithAttributes(attribute.String("service", service)),
					)
				}
			}
			return nil
		},
		m.circuitBreakerState,
	)
	if err != nil {
		return fmt.Errorf("failed to register circuit breaker callback: %w", err)
	}
	m.callbacks = append(m.callbacks, reg)
	
	// Service instances callback
	reg, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			gaugeValues.RLock()
			defer gaugeValues.RUnlock()
			
			for key, value := range gaugeValues.values {
				if len(key) > 10 && key[:10] == "instances_" {
					service := key[10:]
					o.ObserveInt64(m.serviceInstances,
						value,
						metric.WithAttributes(attribute.String("service", service)),
					)
				} else if len(key) > 8 && key[:8] == "healthy_" {
					service := key[8:]
					o.ObserveInt64(m.serviceHealthy,
						value,
						metric.WithAttributes(attribute.String("service", service)),
					)
				}
			}
			return nil
		},
		m.serviceInstances,
		m.serviceHealthy,
	)
	if err != nil {
		return fmt.Errorf("failed to register service instances callback: %w", err)
	}
	m.callbacks = append(m.callbacks, reg)
	
	return nil
}

// Unregister unregisters all callbacks
func (m *Metrics) Unregister() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, reg := range m.callbacks {
		if err := reg.Unregister(); err != nil {
			return err
		}
	}
	m.callbacks = nil
	return nil
}