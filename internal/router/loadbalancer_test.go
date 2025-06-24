package router

import (
	"testing"
	"time"

	"gateway/internal/core"
)

func TestWeightedRoundRobinBalancer(t *testing.T) {
	balancer := NewWeightedRoundRobinBalancer()

	instances := []core.ServiceInstance{
		{
			ID:      "heavy",
			Healthy: true,
			Metadata: map[string]any{
				"weight": 3,
			},
		},
		{
			ID:      "light",
			Healthy: true,
			Metadata: map[string]any{
				"weight": 1,
			},
		},
		{
			ID:      "medium",
			Healthy: true,
			Metadata: map[string]any{
				"weight": 2,
			},
		},
	}

	// Track selection counts
	counts := make(map[string]int)
	totalRounds := 600

	for i := 0; i < totalRounds; i++ {
		selected, err := balancer.Select(instances)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[selected.ID]++
	}

	// Check distribution roughly matches weights
	// With smooth weighted round-robin, distribution should be exact
	expectedHeavy := 300  // 3/6 * 600
	expectedMedium := 200 // 2/6 * 600
	expectedLight := 100  // 1/6 * 600

	t.Logf("Distribution: heavy=%d, medium=%d, light=%d", 
		counts["heavy"], counts["medium"], counts["light"])

	// Allow small variance for rounding
	tolerance := 10
	if abs(counts["heavy"]-expectedHeavy) > tolerance {
		t.Errorf("heavy count %d not close to expected %d", counts["heavy"], expectedHeavy)
	}
	if abs(counts["medium"]-expectedMedium) > tolerance {
		t.Errorf("medium count %d not close to expected %d", counts["medium"], expectedMedium)
	}
	if abs(counts["light"]-expectedLight) > tolerance {
		t.Errorf("light count %d not close to expected %d", counts["light"], expectedLight)
	}
}

func TestWeightedRandomBalancer(t *testing.T) {
	balancer := NewWeightedRandomBalancer()

	instances := []core.ServiceInstance{
		{
			ID:      "heavy",
			Healthy: true,
			Metadata: map[string]any{
				"weight": 4.0, // Test float weight
			},
		},
		{
			ID:      "light",
			Healthy: true,
			Metadata: map[string]any{
				"weight": 1,
			},
		},
	}

	// Track selection counts
	counts := make(map[string]int)
	totalRounds := 1000

	for i := 0; i < totalRounds; i++ {
		selected, err := balancer.Select(instances)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[selected.ID]++
	}

	// Check distribution roughly matches weights (80/20 split)
	expectedHeavy := 800
	expectedLight := 200

	t.Logf("Distribution: heavy=%d, light=%d", counts["heavy"], counts["light"])

	// Allow 10% variance for randomness
	tolerance := 100
	if abs(counts["heavy"]-expectedHeavy) > tolerance {
		t.Errorf("heavy count %d not close to expected %d", counts["heavy"], expectedHeavy)
	}
	if abs(counts["light"]-expectedLight) > tolerance {
		t.Errorf("light count %d not close to expected %d", counts["light"], expectedLight)
	}
}

func TestLeastConnectionsBalancer(t *testing.T) {
	balancer := NewLeastConnectionsBalancer()

	instances := []core.ServiceInstance{
		{ID: "inst1", Healthy: true},
		{ID: "inst2", Healthy: true},
		{ID: "inst3", Healthy: true},
	}

	// First selection should distribute evenly
	selected1, _ := balancer.Select(instances)
	selected2, _ := balancer.Select(instances)
	selected3, _ := balancer.Select(instances)

	// Should have selected different instances
	if selected1.ID == selected2.ID && selected2.ID == selected3.ID {
		t.Error("expected different instances to be selected initially")
	}

	// Simulate connections finishing
	balancer.DecrementConnections(selected1.ID)
	balancer.DecrementConnections(selected2.ID)

	// Next selection should prefer instances with fewer connections
	selected4, _ := balancer.Select(instances)
	if selected4.ID != selected1.ID && selected4.ID != selected2.ID {
		t.Error("expected instance with fewer connections to be selected")
	}

	// Test connection counts
	counts := balancer.GetConnectionCounts()
	t.Logf("Connection counts: %v", counts)
}

func TestResponseTimeBalancer(t *testing.T) {
	balancer := NewResponseTimeBalancer()

	instances := []core.ServiceInstance{
		{ID: "fast", Healthy: true},
		{ID: "slow", Healthy: true},
	}

	// Record some response times
	balancer.RecordResponse("fast", 10*time.Millisecond)
	balancer.RecordResponse("fast", 15*time.Millisecond)
	balancer.RecordResponse("fast", 12*time.Millisecond)
	
	balancer.RecordResponse("slow", 100*time.Millisecond)
	balancer.RecordResponse("slow", 120*time.Millisecond)
	balancer.RecordResponse("slow", 110*time.Millisecond)

	// Should prefer the faster instance
	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		selected, err := balancer.Select(instances)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[selected.ID]++
	}

	t.Logf("Selection counts: fast=%d, slow=%d", counts["fast"], counts["slow"])

	// Fast instance should be selected much more often
	if counts["fast"] < counts["slow"] {
		t.Error("expected fast instance to be selected more often")
	}

	// Check stats
	stats := balancer.GetStats()
	t.Logf("Response time stats: %+v", stats)
}

func TestAdaptiveBalancer(t *testing.T) {
	balancer := NewAdaptiveBalancer()

	instances := []core.ServiceInstance{
		{ID: "inst1", Healthy: true},
		{ID: "inst2", Healthy: true},
		{ID: "inst3", Healthy: true},
	}

	// Select instances multiple times
	for i := 0; i < 30; i++ {
		selected, err := balancer.Select(instances)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		
		// Simulate varying performance
		var success bool
		var latency time.Duration
		
		if selected.ID == "inst1" {
			// inst1 is fast and reliable
			success = true
			latency = 10 * time.Millisecond
		} else if selected.ID == "inst2" {
			// inst2 is slow but reliable
			success = true
			latency = 100 * time.Millisecond
		} else {
			// inst3 has failures
			success = i%3 != 0 // Fail 1/3 of requests
			latency = 50 * time.Millisecond
		}
		
		// Record result (would need to track which strategy was used)
		// For now, just ensure it doesn't panic
		balancer.RecordResult("round_robin", success, latency)
	}

	// Should adapt weights based on performance
	// Just ensure it continues to work without errors
	for i := 0; i < 10; i++ {
		_, err := balancer.Select(instances)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestLoadBalancerNoHealthyInstances(t *testing.T) {
	tests := []struct {
		name     string
		balancer core.LoadBalancer
	}{
		{"WeightedRoundRobin", NewWeightedRoundRobinBalancer()},
		{"WeightedRandom", NewWeightedRandomBalancer()},
		{"LeastConnections", NewLeastConnectionsBalancer()},
		{"ResponseTime", NewResponseTimeBalancer()},
		{"Adaptive", NewAdaptiveBalancer()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instances := []core.ServiceInstance{
				{ID: "inst1", Healthy: false},
				{ID: "inst2", Healthy: false},
			}

			_, err := tt.balancer.Select(instances)
			if err == nil {
				t.Error("expected error when no healthy instances")
			}
		})
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}