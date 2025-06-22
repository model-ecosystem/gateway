package sse

import (
	"github.com/prometheus/client_golang/prometheus"
)

// SSEMetrics holds SSE-specific metrics
type SSEMetrics struct {
	Connections      prometheus.Gauge
	ConnectionsTotal *prometheus.CounterVec
	EventsSent       prometheus.Counter
}

// NewSSEMetrics creates new SSE metrics
func NewSSEMetrics(connections prometheus.Gauge, connectionsTotal *prometheus.CounterVec, 
	eventsSent prometheus.Counter) *SSEMetrics {
	return &SSEMetrics{
		Connections:      connections,
		ConnectionsTotal: connectionsTotal,
		EventsSent:       eventsSent,
	}
}