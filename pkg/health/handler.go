package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Check represents a health check function
type Check func(ctx context.Context) error

// Checker manages health checks
type Checker struct {
	checks map[string]Check
	mu     sync.RWMutex
}

// NewChecker creates a new health checker
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]Check),
	}
}

// RegisterCheck registers a health check
func (c *Checker) RegisterCheck(name string, check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// CheckHealth runs all health checks
func (c *Checker) CheckHealth(ctx context.Context) map[string]CheckResult {
	c.mu.RLock()
	checks := make(map[string]Check, len(c.checks))
	for name, check := range c.checks {
		checks[name] = check
	}
	c.mu.RUnlock()

	results := make(map[string]CheckResult)
	var wg sync.WaitGroup
	var resultsMu sync.Mutex

	for name, check := range checks {
		wg.Add(1)
		go func(name string, check Check) {
			defer wg.Done()

			start := time.Now()
			err := check(ctx)
			duration := time.Since(start)

			result := CheckResult{
				Status:   StatusHealthy,
				Duration: duration,
			}

			if err != nil {
				result.Status = StatusUnhealthy
				result.Error = err.Error()
			}

			resultsMu.Lock()
			results[name] = result
			resultsMu.Unlock()
		}(name, check)
	}

	wg.Wait()
	return results
}

// CheckResult represents the result of a health check
type CheckResult struct {
	Status   Status        `json:"status"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// HealthResponse represents the overall health response
type HealthResponse struct {
	Status    Status                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
	Version   string                 `json:"version,omitempty"`
	ServiceID string                 `json:"service_id,omitempty"`
}

// Handler creates HTTP handlers for health endpoints
type Handler struct {
	checker   *Checker
	version   string
	serviceID string
}

// NewHandler creates a new health handler
func NewHandler(checker *Checker, version, serviceID string) *Handler {
	return &Handler{
		checker:   checker,
		version:   version,
		serviceID: serviceID,
	}
}

// Health handles the /health endpoint
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	results := h.checker.CheckHealth(ctx)

	// Determine overall status
	status := StatusHealthy
	for _, result := range results {
		if result.Status == StatusUnhealthy {
			status = StatusUnhealthy
			break
		} else if result.Status == StatusDegraded {
			status = StatusDegraded
		}
	}

	response := HealthResponse{
		Status:    status,
		Timestamp: time.Now(),
		Checks:    results,
		Version:   h.version,
		ServiceID: h.serviceID,
	}

	// Set status code based on health
	statusCode := http.StatusOK
	if status == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// Ready handles the /ready endpoint
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	// For readiness, we only check critical dependencies
	// This is a simplified version - you might want to check specific services
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	results := h.checker.CheckHealth(ctx)

	// Check if all critical checks pass
	ready := true
	for _, result := range results {
		// You might want to filter only critical checks here
		if result.Status == StatusUnhealthy {
			ready = false
			break
		}
	}

	response := map[string]interface{}{
		"ready":     ready,
		"timestamp": time.Now(),
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// Live handles the /live endpoint (Kubernetes liveness probe)
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	// Liveness check - just return OK if the service is running
	// This should be very lightweight
	response := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
