package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNew(t *testing.T) {
	m := New()

	// Test that all metrics are created
	if m.RequestsTotal == nil {
		t.Error("RequestsTotal is nil")
	}
	if m.RequestDuration == nil {
		t.Error("RequestDuration is nil")
	}
	if m.RequestSize == nil {
		t.Error("RequestSize is nil")
	}
	if m.ResponseSize == nil {
		t.Error("ResponseSize is nil")
	}
	if m.ActiveRequests == nil {
		t.Error("ActiveRequests is nil")
	}
	if m.BackendRequestsTotal == nil {
		t.Error("BackendRequestsTotal is nil")
	}
	if m.BackendRequestDuration == nil {
		t.Error("BackendRequestDuration is nil")
	}
	if m.BackendErrors == nil {
		t.Error("BackendErrors is nil")
	}
	if m.WebSocketConnections == nil {
		t.Error("WebSocketConnections is nil")
	}
	if m.WebSocketConnectionsTotal == nil {
		t.Error("WebSocketConnectionsTotal is nil")
	}
	if m.WebSocketMessagesSent == nil {
		t.Error("WebSocketMessagesSent is nil")
	}
	if m.WebSocketMessagesReceived == nil {
		t.Error("WebSocketMessagesReceived is nil")
	}
	if m.SSEConnections == nil {
		t.Error("SSEConnections is nil")
	}
	if m.SSEConnectionsTotal == nil {
		t.Error("SSEConnectionsTotal is nil")
	}
	if m.SSEEventsSent == nil {
		t.Error("SSEEventsSent is nil")
	}
	if m.HealthCheckDuration == nil {
		t.Error("HealthCheckDuration is nil")
	}
	if m.HealthCheckStatus == nil {
		t.Error("HealthCheckStatus is nil")
	}
	if m.RateLimitHits == nil {
		t.Error("RateLimitHits is nil")
	}
	if m.RateLimitRejected == nil {
		t.Error("RateLimitRejected is nil")
	}
	if m.ServiceInstances == nil {
		t.Error("ServiceInstances is nil")
	}
}

func TestMetricsCollection(t *testing.T) {
	// Create a new registry for testing to avoid conflicts
	registry := prometheus.NewRegistry()
	m := NewWithRegistry(registry, registry)

	// Test request counter
	m.RequestsTotal.WithLabelValues("GET", "/api/test", "200").Inc()
	m.RequestsTotal.WithLabelValues("POST", "/api/test", "201").Inc()
	m.RequestsTotal.WithLabelValues("GET", "/api/test", "404").Inc()

	// Verify counts
	count := testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "/api/test", "200"))
	if count != 1 {
		t.Errorf("Expected 1 GET request with 200, got %f", count)
	}

	count = testutil.ToFloat64(m.RequestsTotal.WithLabelValues("POST", "/api/test", "201"))
	if count != 1 {
		t.Errorf("Expected 1 POST request with 201, got %f", count)
	}

	count = testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "/api/test", "404"))
	if count != 1 {
		t.Errorf("Expected 1 GET request with 404, got %f", count)
	}

	// Test active requests gauge
	m.ActiveRequests.WithLabelValues("GET", "/api/test").Inc()
	active := testutil.ToFloat64(m.ActiveRequests.WithLabelValues("GET", "/api/test"))
	if active != 1 {
		t.Errorf("Expected 1 active request, got %f", active)
	}

	m.ActiveRequests.WithLabelValues("GET", "/api/test").Dec()
	active = testutil.ToFloat64(m.ActiveRequests.WithLabelValues("GET", "/api/test"))
	if active != 0 {
		t.Errorf("Expected 0 active requests, got %f", active)
	}

	// Test histogram
	m.RequestDuration.WithLabelValues("GET", "/api/test", "200").Observe(0.1)
	m.RequestDuration.WithLabelValues("GET", "/api/test", "200").Observe(0.2)
	m.RequestDuration.WithLabelValues("GET", "/api/test", "200").Observe(0.3)

	// Test backend metrics
	m.BackendRequestsTotal.WithLabelValues("backend-service", "instance-1", "GET", "200").Inc()
	backendCount := testutil.ToFloat64(m.BackendRequestsTotal.WithLabelValues("backend-service", "instance-1", "GET", "200"))
	if backendCount != 1 {
		t.Errorf("Expected 1 backend request, got %f", backendCount)
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "short path",
			path:     "/api/v1/users",
			expected: "/api/v1/users",
		},
		{
			name:     "long path",
			path:     "/api/v1/users/12345678901234567890123456789012345678901234567890/profile/settings",
			expected: "/api/v1/users/123456789012345678901234567890123456...",
		},
		{
			name:     "exactly 50 chars",
			path:     "/api/v1/users/12345678901234567890123456789012345",
			expected: "/api/v1/users/12345678901234567890123456789012345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.path)
			if result != tt.expected {
				t.Errorf("NormalizePath(%s) = %s, want %s", tt.path, result, tt.expected)
			}
		})
	}
}
