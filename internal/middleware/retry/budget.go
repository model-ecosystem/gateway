package retry

import (
	"sync"
	"sync/atomic"
	"time"
)

// Budget tracks retry budget to prevent retry storms
type Budget struct {
	// Configuration
	ratio        float64       // Ratio of requests that can be retried (0-1)
	minRequests  int64         // Minimum requests before budget is enforced
	windowSize   time.Duration // Time window for tracking

	// State
	requests     atomic.Int64  // Total requests in window
	retries      atomic.Int64  // Total retries in window
	windowStart  atomic.Int64  // Start of current window (unix nano)
	mu           sync.Mutex    // Protects window rotation
}

// NewBudget creates a new retry budget
func NewBudget(ratio float64, minRequests int64, windowSize time.Duration) *Budget {
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	
	if minRequests < 1 {
		minRequests = 10
	}
	
	if windowSize <= 0 {
		windowSize = time.Minute
	}
	
	b := &Budget{
		ratio:       ratio,
		minRequests: minRequests,
		windowSize:  windowSize,
	}
	
	b.windowStart.Store(time.Now().UnixNano())
	return b
}

// CanRetry checks if a retry is allowed within budget
func (b *Budget) CanRetry() bool {
	b.maybeRotateWindow()
	
	// Always allow retries until we have enough requests
	requests := b.requests.Load()
	if requests < b.minRequests {
		return true
	}
	
	// Check if we're within budget
	retries := b.retries.Load()
	maxRetries := int64(float64(requests) * b.ratio)
	
	return retries < maxRetries
}

// RecordRequest records a request attempt
func (b *Budget) RecordRequest() {
	b.maybeRotateWindow()
	b.requests.Add(1)
}

// RecordRetry records a retry attempt
func (b *Budget) RecordRetry() {
	b.maybeRotateWindow()
	b.retries.Add(1)
}

// Stats returns current budget statistics
func (b *Budget) Stats() BudgetStats {
	b.maybeRotateWindow()
	
	requests := b.requests.Load()
	retries := b.retries.Load()
	
	var retryRate float64
	if requests > 0 {
		retryRate = float64(retries) / float64(requests)
	}
	
	return BudgetStats{
		Requests:       requests,
		Retries:        retries,
		RetryRate:      retryRate,
		BudgetRatio:    b.ratio,
		WindowStart:    time.Unix(0, b.windowStart.Load()),
		WindowDuration: b.windowSize,
	}
}

// maybeRotateWindow checks if we need to start a new time window
func (b *Budget) maybeRotateWindow() {
	now := time.Now().UnixNano()
	windowStart := b.windowStart.Load()
	
	// Check if window has expired
	if now-windowStart < int64(b.windowSize) {
		return
	}
	
	// Need to rotate window
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Double-check under lock
	windowStart = b.windowStart.Load()
	if now-windowStart < int64(b.windowSize) {
		return
	}
	
	// Reset counters for new window
	b.requests.Store(0)
	b.retries.Store(0)
	b.windowStart.Store(now)
}

// BudgetStats holds retry budget statistics
type BudgetStats struct {
	Requests       int64
	Retries        int64
	RetryRate      float64
	BudgetRatio    float64
	WindowStart    time.Time
	WindowDuration time.Duration
}

// GlobalBudget provides a global retry budget across all routes
type GlobalBudget struct {
	budget *Budget
	mu     sync.RWMutex
}

// NewGlobalBudget creates a new global retry budget
func NewGlobalBudget(ratio float64, minRequests int64, windowSize time.Duration) *GlobalBudget {
	return &GlobalBudget{
		budget: NewBudget(ratio, minRequests, windowSize),
	}
}

// CanRetry checks if a retry is allowed
func (g *GlobalBudget) CanRetry() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.budget.CanRetry()
}

// RecordRequest records a request
func (g *GlobalBudget) RecordRequest() {
	g.mu.RLock()
	defer g.mu.RUnlock()
	g.budget.RecordRequest()
}

// RecordRetry records a retry
func (g *GlobalBudget) RecordRetry() {
	g.mu.RLock()
	defer g.mu.RUnlock()
	g.budget.RecordRetry()
}

// Stats returns budget statistics
func (g *GlobalBudget) Stats() BudgetStats {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.budget.Stats()
}

// PerRouteBudget provides per-route retry budgets
type PerRouteBudget struct {
	defaultRatio    float64
	minRequests     int64
	windowSize      time.Duration
	routeBudgets    sync.Map // map[string]*Budget
	routeConfigs    map[string]float64 // route-specific ratios
}

// NewPerRouteBudget creates per-route retry budgets
func NewPerRouteBudget(defaultRatio float64, minRequests int64, windowSize time.Duration, routeConfigs map[string]float64) *PerRouteBudget {
	return &PerRouteBudget{
		defaultRatio: defaultRatio,
		minRequests:  minRequests,
		windowSize:   windowSize,
		routeConfigs: routeConfigs,
	}
}

// GetBudget gets or creates a budget for a route
func (p *PerRouteBudget) GetBudget(route string) *Budget {
	// Try to get existing budget
	if budget, ok := p.routeBudgets.Load(route); ok {
		return budget.(*Budget)
	}
	
	// Get ratio for this route
	ratio := p.defaultRatio
	if routeRatio, ok := p.routeConfigs[route]; ok {
		ratio = routeRatio
	}
	
	// Create new budget
	budget := NewBudget(ratio, p.minRequests, p.windowSize)
	
	// Store and return
	actual, _ := p.routeBudgets.LoadOrStore(route, budget)
	return actual.(*Budget)
}