package tracking

import (
	"context"
	"time"
	
	"gateway/internal/core"
	"gateway/internal/router"
)

// Middleware tracks connections and response times for load balancers
type Middleware struct {
	connectionTracker map[string]*router.ConnectionTracker
	responseTime      map[string]*router.ResponseTimeBalancer
}

// NewMiddleware creates a new tracking middleware
func NewMiddleware() *Middleware {
	return &Middleware{
		connectionTracker: make(map[string]*router.ConnectionTracker),
		responseTime:      make(map[string]*router.ResponseTimeBalancer),
	}
}

// WrapHandler wraps a handler to track connections and response times
func (m *Middleware) WrapHandler(name string, handler core.Handler) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Get route result from context
		routeResult, ok := ctx.Value(routeContextKey{}).(*core.RouteResult)
		if !ok || routeResult == nil || routeResult.Instance == nil {
			// No routing info, just pass through
			return handler(ctx, req)
		}
		
		instanceID := routeResult.Instance.ID
		routeID := routeResult.Rule.ID
		
		// Track connection start for least connections balancer
		if tracker := m.getConnectionTracker(routeID, routeResult.Rule.Balancer); tracker != nil {
			tracker.StartRequest(instanceID)
			defer tracker.EndRequest(instanceID)
		}
		
		// Track response time
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)
		
		// Record response time for response time balancer
		if rtBalancer := m.getResponseTimeBalancer(routeID, routeResult.Rule.Balancer); rtBalancer != nil {
			rtBalancer.RecordResponse(instanceID, duration)
		}
		
		// Record for adaptive balancer
		if adaptiveBalancer, ok := routeResult.Rule.Balancer.(*router.AdaptiveBalancer); ok {
			success := err == nil && (resp == nil || resp.StatusCode() < 500)
			adaptiveBalancer.RecordResult(m.getBalancerType(routeResult.Rule.LoadBalance), success, duration)
		}
		
		return resp, err
	}
}

// getConnectionTracker gets or creates a connection tracker for the route
func (m *Middleware) getConnectionTracker(routeID string, balancer core.LoadBalancer) *router.ConnectionTracker {
	if _, ok := balancer.(*router.LeastConnectionsBalancer); ok {
		if tracker, exists := m.connectionTracker[routeID]; exists {
			return tracker
		}
		tracker := router.NewConnectionTracker(balancer)
		m.connectionTracker[routeID] = tracker
		return tracker
	}
	return nil
}

// getResponseTimeBalancer gets the response time balancer if applicable
func (m *Middleware) getResponseTimeBalancer(routeID string, balancer core.LoadBalancer) *router.ResponseTimeBalancer {
	if rtBalancer, ok := balancer.(*router.ResponseTimeBalancer); ok {
		return rtBalancer
	}
	// Check if it's part of adaptive balancer
	if _, ok := balancer.(*router.AdaptiveBalancer); ok {
		// Return the response time component (would need to expose it)
		// For now, we'll track it separately
		if rtBalancer, exists := m.responseTime[routeID]; exists {
			return rtBalancer
		}
		rtBalancer := router.NewResponseTimeBalancer()
		m.responseTime[routeID] = rtBalancer
		return rtBalancer
	}
	return nil
}

// getBalancerType returns the balancer type string for adaptive tracking
func (m *Middleware) getBalancerType(strategy core.LoadBalanceStrategy) string {
	switch strategy {
	case core.LoadBalanceRoundRobin:
		return "round_robin"
	case core.LoadBalanceLeastConnections:
		return "least_connections"
	case core.LoadBalanceResponseTime:
		return "response_time"
	default:
		return string(strategy)
	}
}

// routeContextKey is the key for storing route info in context (matching handler.go)
type routeContextKey struct{}