package router

import (
	"context"
	"fmt"
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

func (r *stickyTestRequest) ID() string                   { return r.id }
func (r *stickyTestRequest) Method() string               { return r.method }
func (r *stickyTestRequest) Path() string                 { return r.path }
func (r *stickyTestRequest) URL() string                  { return r.url }
func (r *stickyTestRequest) RemoteAddr() string           { return r.remote }
func (r *stickyTestRequest) Headers() map[string][]string { return r.headers }
func (r *stickyTestRequest) Body() io.ReadCloser          { return r.body }
func (r *stickyTestRequest) Context() context.Context     { return r.ctx }

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

func TestStickySessionLRUEviction(t *testing.T) {
	instances := []core.ServiceInstance{
		{ID: "instance-1", Healthy: true},
		{ID: "instance-2", Healthy: true},
	}

	config := &core.SessionAffinityConfig{
		Enabled:    true,
		TTL:        time.Hour,
		Source:     core.SessionSourceCookie,
		CookieName: "SESSION",
	}

	fallback := NewRoundRobinBalancer()
	balancer := NewStickySessionBalancer(fallback, config)

	// Get the internal store to set max entries to a small value for testing
	// balancer is already of type *StickySessionBalancer
	memStore := balancer.store.(*memorySessionStore)
	memStore.maxEntries = 5 // Small limit for testing

	// Create more sessions than the limit
	sessionToInstance := make(map[string]string)

	// Add 5 sessions (up to the limit)
	for i := 0; i < 5; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		req := &stickyTestRequest{
			headers: map[string][]string{
				"Cookie": {fmt.Sprintf("SESSION=%s", sessionID)},
			},
		}

		instance, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed for session %s: %v", sessionID, err)
		}

		sessionToInstance[sessionID] = instance.ID

		// Small delay to ensure different access times
		time.Sleep(10 * time.Millisecond)
	}

	// Now add a 6th session - should evict the oldest (session-0)
	req := &stickyTestRequest{
		headers: map[string][]string{
			"Cookie": {"SESSION=session-5"},
		},
	}

	instance, err := balancer.SelectForRequest(req, instances)
	if err != nil {
		t.Fatalf("SelectForRequest failed for session-5: %v", err)
	}
	sessionToInstance["session-5"] = instance.ID

	// Check that session-0 has been evicted
	req0 := &stickyTestRequest{
		headers: map[string][]string{
			"Cookie": {"SESSION=session-0"},
		},
	}

	_, err = balancer.SelectForRequest(req0, instances)
	if err != nil {
		t.Fatalf("SelectForRequest failed for evicted session: %v", err)
	}

	// The instance for session-0 might be different now (not sticky anymore)
	// But sessions 1-5 should still be sticky
	for i := 1; i <= 5; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		req := &stickyTestRequest{
			headers: map[string][]string{
				"Cookie": {fmt.Sprintf("SESSION=%s", sessionID)},
			},
		}

		instance, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatalf("SelectForRequest failed for session %s: %v", sessionID, err)
		}

		if instance.ID != sessionToInstance[sessionID] {
			t.Errorf("Session %s lost stickiness after LRU eviction", sessionID)
		}
	}

	// Test that accessing a session updates its position in LRU
	// Access session-1 multiple times
	for i := 0; i < 3; i++ {
		req1 := &stickyTestRequest{
			headers: map[string][]string{
				"Cookie": {"SESSION=session-1"},
			},
		}
		_, _ = balancer.SelectForRequest(req1, instances)
		time.Sleep(10 * time.Millisecond)
	}

	// Add more sessions to trigger eviction
	for i := 6; i < 10; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		req := &stickyTestRequest{
			headers: map[string][]string{
				"Cookie": {fmt.Sprintf("SESSION=%s", sessionID)},
			},
		}
		_, _ = balancer.SelectForRequest(req, instances)
		time.Sleep(10 * time.Millisecond)
	}

	// Session-1 should still be sticky (it was accessed recently)
	req1 := &stickyTestRequest{
		headers: map[string][]string{
			"Cookie": {"SESSION=session-1"},
		},
	}

	instance1, err := balancer.SelectForRequest(req1, instances)
	if err != nil {
		t.Fatalf("SelectForRequest failed for session-1: %v", err)
	}

	if instance1.ID != sessionToInstance["session-1"] {
		t.Error("Recently accessed session-1 was evicted")
	}
}

func TestMemorySessionStoreClose(t *testing.T) {
	// Create a store with small cleanup interval for testing
	store := newMemorySessionStore(100)

	// Add some sessions
	for i := 0; i < 10; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		store.SetInstance(sessionID, fmt.Sprintf("instance-%d", i), time.Hour)
	}

	// Close the store
	err := store.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Give some time for goroutine to exit
	time.Sleep(100 * time.Millisecond)

	// The store should still be functional (just no cleanup)
	instanceID, found := store.GetInstance("session-5")
	if !found {
		t.Error("Expected to find session-5 after close")
	}
	if instanceID != "instance-5" {
		t.Errorf("Expected instance-5, got %s", instanceID)
	}
}
