package websocket

import (
	"github.com/prometheus/client_golang/prometheus"
)

// WebSocketMetrics holds WebSocket-specific metrics
type WebSocketMetrics struct {
	Connections      prometheus.Gauge
	ConnectionsTotal *prometheus.CounterVec
	MessagesSent     prometheus.Counter
	MessagesReceived prometheus.Counter
}

// NewWebSocketMetrics creates new WebSocket metrics
func NewWebSocketMetrics(connections prometheus.Gauge, connectionsTotal *prometheus.CounterVec, 
	messagesSent prometheus.Counter, messagesReceived prometheus.Counter) *WebSocketMetrics {
	return &WebSocketMetrics{
		Connections:      connections,
		ConnectionsTotal: connectionsTotal,
		MessagesSent:     messagesSent,
		MessagesReceived: messagesReceived,
	}
}