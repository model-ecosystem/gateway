package router

import (
	"math"
	"math/rand"
	"sync"
	"time"
	
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// ResponseTimeBalancer implements response time based load balancing
type ResponseTimeBalancer struct {
	mu       sync.RWMutex
	stats    map[string]*instanceStats
	window   time.Duration
	decayRate float64
}

type instanceStats struct {
	totalTime        time.Duration
	requestCount     int64
	lastUpdate       time.Time
	avgResponseTime  time.Duration
	ewmaResponseTime float64 // Exponentially weighted moving average
}

// NewResponseTimeBalancer creates a new response time based balancer
func NewResponseTimeBalancer() *ResponseTimeBalancer {
	return &ResponseTimeBalancer{
		stats:     make(map[string]*instanceStats),
		window:    5 * time.Minute,
		decayRate: 0.1, // EWMA decay rate
	}
}

// Select selects the instance with best response time
func (b *ResponseTimeBalancer) Select(instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	var selected *core.ServiceInstance
	var bestScore float64 = -1
	
	now := time.Now()
	
	// Find healthy instance with best response time score
	for i := range instances {
		inst := &instances[i]
		
		// Skip unhealthy instances
		if !inst.Healthy {
			continue
		}
		
		// Calculate score (lower response time = higher score)
		score := b.calculateScore(inst.ID, now)
		
		// Select instance with best score
		if bestScore == -1 || score > bestScore {
			selected = inst
			bestScore = score
		}
	}
	
	if selected == nil {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no healthy instances")
	}
	
	return selected, nil
}

// calculateScore calculates a score for an instance (higher is better)
func (b *ResponseTimeBalancer) calculateScore(instanceID string, now time.Time) float64 {
	stats, exists := b.stats[instanceID]
	if !exists || stats.requestCount == 0 {
		// No data yet, give neutral score
		return 1.0
	}
	
	// Use EWMA response time if available
	if stats.ewmaResponseTime > 0 {
		// Score inversely proportional to response time
		// Add 1ms to avoid division by zero
		return 1000.0 / (stats.ewmaResponseTime + 1)
	}
	
	// Fallback to average
	avgMs := float64(stats.avgResponseTime.Milliseconds())
	if avgMs <= 0 {
		return 1.0
	}
	
	return 1000.0 / avgMs
}

// RecordResponse records a response time for an instance
func (b *ResponseTimeBalancer) RecordResponse(instanceID string, duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	stats, exists := b.stats[instanceID]
	if !exists {
		stats = &instanceStats{
			lastUpdate: time.Now(),
		}
		b.stats[instanceID] = stats
	}
	
	// Update statistics
	stats.totalTime += duration
	stats.requestCount++
	stats.avgResponseTime = time.Duration(int64(stats.totalTime) / stats.requestCount)
	
	// Update EWMA
	durationMs := float64(duration.Milliseconds())
	if stats.ewmaResponseTime == 0 {
		stats.ewmaResponseTime = durationMs
	} else {
		stats.ewmaResponseTime = (1-b.decayRate)*stats.ewmaResponseTime + b.decayRate*durationMs
	}
	
	stats.lastUpdate = time.Now()
}

// Reset resets all statistics
func (b *ResponseTimeBalancer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.stats = make(map[string]*instanceStats)
}

// GetStats returns current statistics for monitoring
func (b *ResponseTimeBalancer) GetStats() map[string]ResponseTimeStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	result := make(map[string]ResponseTimeStats, len(b.stats))
	for id, stats := range b.stats {
		result[id] = ResponseTimeStats{
			AvgResponseTime:  stats.avgResponseTime,
			EWMAResponseTime: time.Duration(stats.ewmaResponseTime) * time.Millisecond,
			RequestCount:     stats.requestCount,
			LastUpdate:       stats.lastUpdate,
		}
	}
	
	return result
}

// ResponseTimeStats holds response time statistics
type ResponseTimeStats struct {
	AvgResponseTime  time.Duration
	EWMAResponseTime time.Duration
	RequestCount     int64
	LastUpdate       time.Time
}

// AdaptiveBalancer combines multiple strategies adaptively
type AdaptiveBalancer struct {
	mu               sync.RWMutex
	roundRobin       *RoundRobinBalancer
	leastConnections *LeastConnectionsBalancer
	responseTime     *ResponseTimeBalancer
	
	// Adaptive weights for each strategy
	weights struct {
		roundRobin   float64
		leastConn    float64
		responseTime float64
	}
	
	// Performance tracking
	strategyStats map[string]*strategyPerformance
}

type strategyPerformance struct {
	successCount int64
	errorCount   int64
	totalLatency time.Duration
}

// NewAdaptiveBalancer creates a new adaptive load balancer
func NewAdaptiveBalancer() *AdaptiveBalancer {
	return &AdaptiveBalancer{
		roundRobin:       NewRoundRobinBalancer(),
		leastConnections: NewLeastConnectionsBalancer(),
		responseTime:     NewResponseTimeBalancer(),
		strategyStats:    make(map[string]*strategyPerformance),
		weights: struct {
			roundRobin   float64
			leastConn    float64
			responseTime float64
		}{
			roundRobin:   0.33,
			leastConn:    0.33,
			responseTime: 0.34,
		},
	}
}

// Select adaptively selects an instance using weighted strategy selection
func (b *AdaptiveBalancer) Select(instances []core.ServiceInstance) (*core.ServiceInstance, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// Select strategy based on adaptive weights
	r := rand.Float64()
	
	var selected *core.ServiceInstance
	var err error
	var strategy string
	
	if r < b.weights.roundRobin {
		selected, err = b.roundRobin.Select(instances)
		strategy = "round_robin"
	} else if r < b.weights.roundRobin + b.weights.leastConn {
		selected, err = b.leastConnections.Select(instances)
		strategy = "least_connections"
	} else {
		selected, err = b.responseTime.Select(instances)
		strategy = "response_time"
	}
	
	// Track strategy selection for adaptive learning
	if selected != nil {
		if _, ok := b.strategyStats[strategy]; !ok {
			b.strategyStats[strategy] = &strategyPerformance{}
		}
	}
	
	return selected, err
}

// RecordResult records the result of a request for adaptive learning
func (b *AdaptiveBalancer) RecordResult(strategy string, success bool, latency time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	stats, ok := b.strategyStats[strategy]
	if !ok {
		return
	}
	
	if success {
		stats.successCount++
		stats.totalLatency += latency
	} else {
		stats.errorCount++
	}
	
	// Periodically adjust weights based on performance
	totalRequests := stats.successCount + stats.errorCount
	if totalRequests > 0 && totalRequests%100 == 0 {
		b.adjustWeights()
	}
}

// adjustWeights adjusts strategy weights based on performance
func (b *AdaptiveBalancer) adjustWeights() {
	// Calculate performance scores for each strategy
	scores := make(map[string]float64)
	totalScore := 0.0
	
	for strategy, stats := range b.strategyStats {
		if stats.successCount == 0 {
			continue
		}
		
		// Score based on success rate and average latency
		successRate := float64(stats.successCount) / float64(stats.successCount + stats.errorCount)
		avgLatency := float64(stats.totalLatency) / float64(stats.successCount)
		
		// Normalize latency (lower is better)
		latencyScore := 1.0 / (1.0 + avgLatency/float64(time.Second))
		
		// Combined score (70% success rate, 30% latency)
		score := 0.7*successRate + 0.3*latencyScore
		scores[strategy] = score
		totalScore += score
	}
	
	// Update weights based on scores
	if totalScore > 0 {
		if score, ok := scores["round_robin"]; ok {
			b.weights.roundRobin = score / totalScore
		}
		if score, ok := scores["least_connections"]; ok {
			b.weights.leastConn = score / totalScore
		}
		if score, ok := scores["response_time"]; ok {
			b.weights.responseTime = score / totalScore
		}
		
		// Ensure weights sum to 1
		sum := b.weights.roundRobin + b.weights.leastConn + b.weights.responseTime
		if sum > 0 && math.Abs(sum-1.0) > 0.01 {
			b.weights.roundRobin /= sum
			b.weights.leastConn /= sum
			b.weights.responseTime /= sum
		}
	}
}