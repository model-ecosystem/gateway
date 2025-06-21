package router

import (
	"context"
	"io"
	"testing"
	"time"
	
	"gateway/internal/core"
)

// stickyTestRequest implements core.Request for testing sticky sessions
type stickyTestRequest struct {
	id      string
	method  string
	path    string
	url     string
	remote  string
	headers map[string][]string
	body    io.ReadCloser
	ctx     context.Context
}

func (r *stickyTestRequest) ID() string                      { return r.id }
func (r *stickyTestRequest) Method() string                  { return r.method }
func (r *stickyTestRequest) Path() string                    { return r.path }
func (r *stickyTestRequest) URL() string                     { return r.url }
func (r *stickyTestRequest) RemoteAddr() string              { return r.remote }
func (r *stickyTestRequest) Headers() map[string][]string    { return r.headers }
func (r *stickyTestRequest) Body() io.ReadCloser             { return r.body }
func (r *stickyTestRequest) Context() context.Context        { return r.ctx }

func TestStickySessionBalancer(t *testing.T) {
	instances := []core.ServiceInstance{
		{ID: "instance-1", Healthy: true},
		{ID: "instance-2", Healthy: true},
		{ID: "instance-3", Healthy: true},
	}
	
	t.Run("cookie-based session", func(t *testing.T) {
		config := &core.SessionAffinityConfig{
			Enabled:    true,
			TTL:        time.Hour,
			Source:     core.SessionSourceCookie,
			CookieName: "SESSION",
		}
		
		fallback := NewRoundRobinBalancer()
		balancer := NewStickySessionBalancer(fallback, config)
		
		// First request with session cookie
		req1 := &stickyTestRequest{
			headers: map[string][]string{
				"Cookie": {"SESSION=abc123"},
			},
		}
		
		instance1, err := balancer.SelectForRequest(req1, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		// Second request with same session should get same instance
		req2 := &stickyTestRequest{
			headers: map[string][]string{
				"Cookie": {"SESSION=abc123"},
			},
		}
		
		instance2, err := balancer.SelectForRequest(req2, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		if instance1.ID != instance2.ID {
			t.Errorf("Expected same instance, got %s and %s", instance1.ID, instance2.ID)
		}
		
		// Request with different session should get different instance (eventually)
		req3 := &stickyTestRequest{
			headers: map[string][]string{
				"Cookie": {"SESSION=xyz789"},
			},
		}
		
		instance3, err := balancer.SelectForRequest(req3, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		// Should be assigned an instance
		if instance3.ID == "" {
			t.Errorf("Expected instance assignment for new session")
		}
	})
	
	t.Run("header-based session", func(t *testing.T) {
		config := &core.SessionAffinityConfig{
			Enabled:    true,
			TTL:        time.Hour,
			Source:     core.SessionSourceHeader,
			HeaderName: "X-Session-Id",
		}
		
		fallback := NewRoundRobinBalancer()
		balancer := NewStickySessionBalancer(fallback, config)
		
		// Request with session header
		req := &stickyTestRequest{
			headers: map[string][]string{
				"X-Session-Id": {"session-123"},
			},
		}
		
		instance1, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		// Same session should get same instance
		instance2, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		if instance1.ID != instance2.ID {
			t.Errorf("Expected same instance, got %s and %s", instance1.ID, instance2.ID)
		}
	})
	
	t.Run("query-based session", func(t *testing.T) {
		config := &core.SessionAffinityConfig{
			Enabled:    true,
			TTL:        time.Hour,
			Source:     core.SessionSourceQuery,
			QueryParam: "sid",
		}
		
		fallback := NewRoundRobinBalancer()
		balancer := NewStickySessionBalancer(fallback, config)
		
		// Request with session in query
		req := &stickyTestRequest{
			url: "/api/test?sid=query-session-456",
		}
		
		instance1, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		// Same session should get same instance
		instance2, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		if instance1.ID != instance2.ID {
			t.Errorf("Expected same instance, got %s and %s", instance1.ID, instance2.ID)
		}
	})
	
	t.Run("no session fallback", func(t *testing.T) {
		config := &core.SessionAffinityConfig{
			Enabled:    true,
			TTL:        time.Hour,
			Source:     core.SessionSourceCookie,
			CookieName: "SESSION",
		}
		
		fallback := NewRoundRobinBalancer()
		balancer := NewStickySessionBalancer(fallback, config)
		
		// Request without session
		req := &stickyTestRequest{
			headers: map[string][]string{},
		}
		
		// Should use fallback balancer
		instance, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		if instance.ID == "" {
			t.Errorf("Expected instance from fallback balancer")
		}
	})
	
	t.Run("instance health check", func(t *testing.T) {
		config := &core.SessionAffinityConfig{
			Enabled:    true,
			TTL:        time.Hour,
			Source:     core.SessionSourceCookie,
			CookieName: "SESSION",
		}
		
		fallback := NewRoundRobinBalancer()
		balancer := NewStickySessionBalancer(fallback, config)
		
		// First request
		req := &stickyTestRequest{
			headers: map[string][]string{
				"Cookie": {"SESSION=health-test"},
			},
		}
		
		instance1, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		// Mark the instance as unhealthy
		unhealthyInstances := make([]core.ServiceInstance, len(instances))
		copy(unhealthyInstances, instances)
		for i := range unhealthyInstances {
			if unhealthyInstances[i].ID == instance1.ID {
				unhealthyInstances[i].Healthy = false
				break
			}
		}
		
		// Request should get a different healthy instance
		instance2, err := balancer.SelectForRequest(req, unhealthyInstances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		if instance2.ID == instance1.ID {
			t.Errorf("Expected different instance when original is unhealthy")
		}
		
		// Third request with all healthy should stick to new instance
		instance3, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed: %v", err)
		}
		
		if instance3.ID != instance2.ID {
			t.Errorf("Expected to stick to new instance, got %s and %s", instance2.ID, instance3.ID)
		}
	})
	
	t.Run("implements RequestAwareLoadBalancer", func(t *testing.T) {
		config := &core.SessionAffinityConfig{
			Enabled:    true,
			TTL:        time.Hour,
			Source:     core.SessionSourceCookie,
			CookieName: "SESSION",
		}
		
		fallback := NewRoundRobinBalancer()
		balancer := NewStickySessionBalancer(fallback, config)
		
		// Verify it implements the interface
		var _ core.RequestAwareLoadBalancer = balancer
	})
}