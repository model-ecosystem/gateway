package router

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"gateway/internal/core"
)

// Mock request for testing consistent hash
type mockHashRequest struct {
	id         string
	path       string
	remoteAddr string
	headers    map[string][]string
}

func (m *mockHashRequest) ID() string { return m.id }
func (m *mockHashRequest) Method() string { return "GET" }
func (m *mockHashRequest) Path() string   { return m.path }
func (m *mockHashRequest) URL() string { return m.path }
func (m *mockHashRequest) RemoteAddr() string { return m.remoteAddr }
func (m *mockHashRequest) Headers() map[string][]string { 
	if m.headers != nil {
		return m.headers
	}
	return make(map[string][]string)
}
func (m *mockHashRequest) Body() io.ReadCloser { return io.NopCloser(strings.NewReader("")) }
func (m *mockHashRequest) Context() context.Context { return context.Background() }

func TestConsistentHashBalancer_Basic(t *testing.T) {
	balancer := NewConsistentHashBalancer(3) // 3 replicas for testing
	
	instances := []core.ServiceInstance{
		{ID: "server-1", Address: "10.0.0.1", Port: 8080, Healthy: true},
		{ID: "server-2", Address: "10.0.0.2", Port: 8080, Healthy: true},
		{ID: "server-3", Address: "10.0.0.3", Port: 8080, Healthy: true},
	}
	
	// Test with session-based routing
	req := &mockHashRequest{
		path:    "/api/test",
		headers: map[string][]string{"X-Session-Id": {"session-123"}},
	}
	
	// Should always return the same instance for the same session
	var firstInstance *core.ServiceInstance
	for i := 0; i < 10; i++ {
		instance, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatal(err)
		}
		
		if firstInstance == nil {
			firstInstance = instance
		} else if instance.ID != firstInstance.ID {
			t.Errorf("Expected consistent instance selection, got different instances")
		}
	}
}

func TestConsistentHashBalancer_Distribution(t *testing.T) {
	balancer := NewConsistentHashBalancer(150) // Default replicas
	
	instances := []core.ServiceInstance{
		{ID: "server-1", Address: "10.0.0.1", Port: 8080, Healthy: true},
		{ID: "server-2", Address: "10.0.0.2", Port: 8080, Healthy: true},
		{ID: "server-3", Address: "10.0.0.3", Port: 8080, Healthy: true},
	}
	
	// Test distribution with different client IPs
	distribution := make(map[string]int)
	for i := 0; i < 1000; i++ {
		req := &mockHashRequest{
			path:       "/api/test",
			remoteAddr: fmt.Sprintf("192.168.1.%d:8080", i%256),
		}
		
		instance, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatal(err)
		}
		
		distribution[instance.ID]++
	}
	
	// Check that all servers got some requests
	for _, inst := range instances {
		if distribution[inst.ID] == 0 {
			t.Errorf("Instance %s got no requests", inst.ID)
		}
	}
	
	// Check reasonable distribution (each should get roughly 33%)
	for id, count := range distribution {
		percentage := float64(count) / 10.0
		t.Logf("Instance %s: %d requests (%.1f%%)", id, count, percentage)
		if percentage < 20 || percentage > 50 {
			t.Errorf("Instance %s has skewed distribution: %.1f%%", id, percentage)
		}
	}
}

func TestConsistentHashBalancer_Failover(t *testing.T) {
	balancer := NewConsistentHashBalancer(10)
	
	instances := []core.ServiceInstance{
		{ID: "server-1", Address: "10.0.0.1", Port: 8080, Healthy: true},
		{ID: "server-2", Address: "10.0.0.2", Port: 8080, Healthy: true},
		{ID: "server-3", Address: "10.0.0.3", Port: 8080, Healthy: true},
	}
	
	req := &mockHashRequest{
		path:    "/api/test",
		headers: map[string][]string{"X-Session-Id": {"session-456"}},
	}
	
	// Get initial instance
	instance1, err := balancer.SelectForRequest(req, instances)
	if err != nil {
		t.Fatal(err)
	}
	
	// Mark the selected instance as unhealthy
	unhealthyInstances := make([]core.ServiceInstance, len(instances))
	copy(unhealthyInstances, instances)
	for i := range unhealthyInstances {
		if unhealthyInstances[i].ID == instance1.ID {
			unhealthyInstances[i].Healthy = false
			break
		}
	}
	
	// Should failover to next instance in ring
	instance2, err := balancer.SelectForRequest(req, unhealthyInstances)
	if err != nil {
		t.Fatal(err)
	}
	
	if instance2.ID == instance1.ID {
		t.Error("Expected failover to different instance")
	}
	
	// Restore health and verify it returns to original
	instance3, err := balancer.SelectForRequest(req, instances)
	if err != nil {
		t.Fatal(err)
	}
	
	if instance3.ID != instance1.ID {
		t.Error("Expected to return to original instance after recovery")
	}
}

func TestConsistentHashBalancer_AddRemoveInstances(t *testing.T) {
	// Use more replicas for better distribution
	balancer := NewConsistentHashBalancer(150)
	
	instances := []core.ServiceInstance{
		{ID: "server-1", Address: "10.0.0.1", Port: 8080, Healthy: true},
		{ID: "server-2", Address: "10.0.0.2", Port: 8080, Healthy: true},
	}
	
	// Track which sessions go to which servers
	sessionMapping := make(map[string]string)
	for i := 0; i < 100; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		req := &mockHashRequest{
			path:    "/api/test",
			headers: map[string][]string{"X-Session-Id": {sessionID}},
		}
		
		instance, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatal(err)
		}
		sessionMapping[sessionID] = instance.ID
	}
	
	// Add a new instance
	instances = append(instances, core.ServiceInstance{
		ID: "server-3", Address: "10.0.0.3", Port: 8080, Healthy: true,
	})
	
	// Check how many sessions moved
	moved := 0
	for sessionID, oldServer := range sessionMapping {
		req := &mockHashRequest{
			path:    "/api/test",
			headers: map[string][]string{"X-Session-Id": {sessionID}},
		}
		
		instance, err := balancer.SelectForRequest(req, instances)
		if err != nil {
			t.Fatal(err)
		}
		
		if instance.ID != oldServer {
			moved++
		}
	}
	
	// With consistent hashing and proper replica count, approximately 1/3 of sessions should move
	// Allow some variance due to hash distribution
	movePercentage := float64(moved) / float64(len(sessionMapping)) * 100
	t.Logf("Sessions moved after adding instance: %d (%.1f%%)", moved, movePercentage)
	
	// Allow 25-45% movement (33% ± 12%)
	if movePercentage < 25 || movePercentage > 45 {
		t.Errorf("Unexpected session movement: %.1f%% (expected 25-45%%)", movePercentage)
	}
}

func TestConsistentHashBalancer_HashKeyPriority(t *testing.T) {
	balancer := NewConsistentHashBalancer(10)
	
	instances := []core.ServiceInstance{
		{ID: "server-1", Address: "10.0.0.1", Port: 8080, Healthy: true},
		{ID: "server-2", Address: "10.0.0.2", Port: 8080, Healthy: true},
	}
	
	tests := []struct {
		name     string
		req      *mockHashRequest
		sameAs   int // Index of request that should hash to same instance
	}{
		{
			name: "session header priority",
			req: &mockHashRequest{
				path:       "/api/test",
				remoteAddr: "192.168.1.1:8080",
				headers:    map[string][]string{
					"X-Session-Id": {"session-1"},
					"Cookie": {"session=cookie-1"},
				},
			},
		},
		{
			name: "same session header",
			req: &mockHashRequest{
				path:       "/api/different",
				remoteAddr: "192.168.1.2:8080",
				headers:    map[string][]string{
					"X-Session-Id": {"session-1"},
					"Cookie": {"session=cookie-2"},
				},
			},
			sameAs: 0,
		},
		{
			name: "cookie when no session header",
			req: &mockHashRequest{
				path:       "/api/test",
				remoteAddr: "192.168.1.1:8080",
				headers:    map[string][]string{"Cookie": {"session=cookie-3"}},
			},
		},
		{
			name: "client IP when no session",
			req: &mockHashRequest{
				path:       "/api/test",
				remoteAddr: "192.168.1.100:8080",
			},
		},
		{
			name: "same client IP",
			req: &mockHashRequest{
				path:       "/api/different",
				remoteAddr: "192.168.1.100:8080",
			},
			sameAs: 3,
		},
	}
	
	results := make([]*core.ServiceInstance, len(tests))
	
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance, err := balancer.SelectForRequest(tt.req, instances)
			if err != nil {
				t.Fatal(err)
			}
			results[i] = instance
			
			if tt.sameAs > 0 && results[tt.sameAs].ID != instance.ID {
				t.Errorf("Expected same instance as test %d, got different", tt.sameAs)
			}
		})
	}
}

func TestConsistentHashBalancer_NoHealthyInstances(t *testing.T) {
	balancer := NewConsistentHashBalancer(10)
	
	instances := []core.ServiceInstance{
		{ID: "server-1", Address: "10.0.0.1", Port: 8080, Healthy: false},
		{ID: "server-2", Address: "10.0.0.2", Port: 8080, Healthy: false},
	}
	
	req := &mockHashRequest{path: "/api/test"}
	
	_, err := balancer.SelectForRequest(req, instances)
	if err == nil {
		t.Error("Expected error for no healthy instances")
	}
}

func TestConsistentHashBalancer_Stats(t *testing.T) {
	balancer := NewConsistentHashBalancer(10)
	
	instances := []core.ServiceInstance{
		{ID: "server-1", Address: "10.0.0.1", Port: 8080, Healthy: true},
		{ID: "server-2", Address: "10.0.0.2", Port: 8080, Healthy: true},
	}
	
	// Force ring update
	req := &mockHashRequest{path: "/api/test"}
	_, _ = balancer.SelectForRequest(req, instances)
	
	stats := balancer.GetStats()
	
	if stats["instances"].(int) != 2 {
		t.Errorf("Expected 2 instances, got %d", stats["instances"])
	}
	
	if stats["virtual_nodes"].(int) != 20 { // 2 instances * 10 replicas
		t.Errorf("Expected 20 virtual nodes, got %d", stats["virtual_nodes"])
	}
	
	distribution := stats["distribution"].(map[string]int)
	if distribution["server-1"]+distribution["server-2"] != 20 {
		t.Error("Distribution count mismatch")
	}
}