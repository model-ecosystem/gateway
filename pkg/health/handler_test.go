package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewChecker(t *testing.T) {
	checker := NewChecker()
	
	if checker == nil {
		t.Fatal("expected non-nil checker")
	}
	
	if checker.checks == nil {
		t.Error("expected checks map to be initialized")
	}
}

func TestChecker_RegisterCheck(t *testing.T) {
	checker := NewChecker()
	
	// Register a check
	check := func(ctx context.Context) error {
		return nil
	}
	
	checker.RegisterCheck("test-check", check)
	
	// Verify check was registered
	checker.mu.RLock()
	defer checker.mu.RUnlock()
	
	if _, exists := checker.checks["test-check"]; !exists {
		t.Error("check was not registered")
	}
}

func TestChecker_CheckHealth(t *testing.T) {
	tests := []struct {
		name     string
		checks   map[string]Check
		expected map[string]Status
	}{
		{
			name: "all healthy",
			checks: map[string]Check{
				"db": func(ctx context.Context) error {
					return nil
				},
				"cache": func(ctx context.Context) error {
					return nil
				},
			},
			expected: map[string]Status{
				"db":    StatusHealthy,
				"cache": StatusHealthy,
			},
		},
		{
			name: "mixed health",
			checks: map[string]Check{
				"db": func(ctx context.Context) error {
					return nil
				},
				"cache": func(ctx context.Context) error {
					return errors.New("cache connection failed")
				},
			},
			expected: map[string]Status{
				"db":    StatusHealthy,
				"cache": StatusUnhealthy,
			},
		},
		{
			name: "all unhealthy",
			checks: map[string]Check{
				"db": func(ctx context.Context) error {
					return errors.New("db connection failed")
				},
				"cache": func(ctx context.Context) error {
					return errors.New("cache connection failed")
				},
			},
			expected: map[string]Status{
				"db":    StatusUnhealthy,
				"cache": StatusUnhealthy,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker()
			
			// Register all checks
			for name, check := range tt.checks {
				checker.RegisterCheck(name, check)
			}
			
			// Run health checks
			results := checker.CheckHealth(context.Background())
			
			// Verify results
			if len(results) != len(tt.expected) {
				t.Errorf("expected %d results, got %d", len(tt.expected), len(results))
			}
			
			for name, expectedStatus := range tt.expected {
				result, exists := results[name]
				if !exists {
					t.Errorf("missing result for check %q", name)
					continue
				}
				
				if result.Status != expectedStatus {
					t.Errorf("check %q: expected status %q, got %q", name, expectedStatus, result.Status)
				}
				
				if expectedStatus == StatusUnhealthy && result.Error == "" {
					t.Errorf("check %q: expected error message for unhealthy status", name)
				}
				
				// Duration might be very small (less than a microsecond) for simple checks
				// so we won't check for non-zero duration
			}
		})
	}
}

func TestChecker_CheckHealth_Concurrent(t *testing.T) {
	checker := NewChecker()
	
	// Register multiple checks with delays
	for i := 0; i < 10; i++ {
		name := string(rune('a' + i))
		checker.RegisterCheck(name, func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}
	
	// Run health checks
	start := time.Now()
	results := checker.CheckHealth(context.Background())
	duration := time.Since(start)
	
	// All checks should run concurrently, so total time should be ~10ms, not 100ms
	if duration > 50*time.Millisecond {
		t.Errorf("checks took too long (%v), should run concurrently", duration)
	}
	
	// Verify all checks completed
	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}
}

func TestChecker_ConcurrentAccess(t *testing.T) {
	checker := NewChecker()
	
	// Concurrently register checks and run health checks
	var wg sync.WaitGroup
	
	// Register checks concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := string(rune('a' + i))
			checker.RegisterCheck(name, func(ctx context.Context) error {
				return nil
			})
		}(i)
	}
	
	// Run health checks concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = checker.CheckHealth(context.Background())
		}()
	}
	
	wg.Wait()
	
	// Final check should have all registered checks
	results := checker.CheckHealth(context.Background())
	if len(results) != 10 {
		t.Errorf("expected 10 registered checks, got %d", len(results))
	}
}

func TestNewHandler(t *testing.T) {
	checker := NewChecker()
	handler := NewHandler(checker, "1.0.0", "service-123")
	
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	
	if handler.version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", handler.version)
	}
	
	if handler.serviceID != "service-123" {
		t.Errorf("expected serviceID service-123, got %s", handler.serviceID)
	}
}

func TestHandler_Health(t *testing.T) {
	tests := []struct {
		name           string
		checks         map[string]Check
		expectedStatus int
		expectedJSON   map[string]interface{}
	}{
		{
			name: "all healthy",
			checks: map[string]Check{
				"db": func(ctx context.Context) error {
					return nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedJSON: map[string]interface{}{
				"status": "healthy",
			},
		},
		{
			name: "unhealthy",
			checks: map[string]Check{
				"db": func(ctx context.Context) error {
					return errors.New("connection failed")
				},
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedJSON: map[string]interface{}{
				"status": "unhealthy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker()
			for name, check := range tt.checks {
				checker.RegisterCheck(name, check)
			}
			
			handler := NewHandler(checker, "1.0.0", "test-service")
			
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			
			handler.Health(w, req)
			
			resp := w.Result()
			defer resp.Body.Close()
			
			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
			
			// Check content type
			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", contentType)
			}
			
			// Decode and check response
			var response HealthResponse
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			
			if string(response.Status) != tt.expectedJSON["status"] {
				t.Errorf("expected status %s, got %s", tt.expectedJSON["status"], response.Status)
			}
			
			if response.Version != "1.0.0" {
				t.Errorf("expected version 1.0.0, got %s", response.Version)
			}
			
			if response.ServiceID != "test-service" {
				t.Errorf("expected serviceID test-service, got %s", response.ServiceID)
			}
			
			if response.Timestamp.IsZero() {
				t.Error("expected non-zero timestamp")
			}
			
			if len(response.Checks) != len(tt.checks) {
				t.Errorf("expected %d checks, got %d", len(tt.checks), len(response.Checks))
			}
		})
	}
}

func TestHandler_Ready(t *testing.T) {
	tests := []struct {
		name           string
		checks         map[string]Check
		expectedStatus int
		expectedReady  bool
	}{
		{
			name: "ready",
			checks: map[string]Check{
				"db": func(ctx context.Context) error {
					return nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedReady:  true,
		},
		{
			name: "not ready",
			checks: map[string]Check{
				"db": func(ctx context.Context) error {
					return errors.New("not connected")
				},
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedReady:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker()
			for name, check := range tt.checks {
				checker.RegisterCheck(name, check)
			}
			
			handler := NewHandler(checker, "1.0.0", "test-service")
			
			req := httptest.NewRequest("GET", "/ready", nil)
			w := httptest.NewRecorder()
			
			handler.Ready(w, req)
			
			resp := w.Result()
			defer resp.Body.Close()
			
			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
			
			// Check content type
			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", contentType)
			}
			
			// Decode and check response
			var response map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			
			ready, ok := response["ready"].(bool)
			if !ok {
				t.Fatal("ready field not found or not a boolean")
			}
			
			if ready != tt.expectedReady {
				t.Errorf("expected ready=%v, got %v", tt.expectedReady, ready)
			}
			
			if _, ok := response["timestamp"]; !ok {
				t.Error("timestamp field not found")
			}
		})
	}
}

func TestHandler_Live(t *testing.T) {
	checker := NewChecker()
	handler := NewHandler(checker, "1.0.0", "test-service")
	
	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()
	
	handler.Live(w, req)
	
	resp := w.Result()
	defer resp.Body.Close()
	
	// Check status code - should always be 200 for liveness
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	
	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
	
	// Decode and check response
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	status, ok := response["status"].(string)
	if !ok {
		t.Fatal("status field not found or not a string")
	}
	
	if status != "ok" {
		t.Errorf("expected status=ok, got %s", status)
	}
	
	if _, ok := response["timestamp"]; !ok {
		t.Error("timestamp field not found")
	}
}

func TestHandler_Timeout(t *testing.T) {
	checker := NewChecker()
	
	// Register a slow check
	checker.RegisterCheck("slow", func(ctx context.Context) error {
		select {
		case <-time.After(10 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	
	handler := NewHandler(checker, "1.0.0", "test-service")
	
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	start := time.Now()
	handler.Health(w, req)
	duration := time.Since(start)
	
	// Should timeout after 5 seconds
	if duration > 6*time.Second {
		t.Errorf("handler took too long: %v", duration)
	}
	
	resp := w.Result()
	defer resp.Body.Close()
	
	// Should still return a response
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}
}

func TestCheckResult_JSON(t *testing.T) {
	result := CheckResult{
		Status:   StatusHealthy,
		Error:    "",
		Duration: 123 * time.Millisecond,
	}
	
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal CheckResult: %v", err)
	}
	
	// Check that duration is included in JSON
	if !strings.Contains(string(data), "duration") {
		t.Error("duration field not found in JSON")
	}
	
	// Test with error
	result.Status = StatusUnhealthy
	result.Error = "test error"
	
	data, err = json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal CheckResult with error: %v", err)
	}
	
	if !strings.Contains(string(data), "test error") {
		t.Error("error message not found in JSON")
	}
}

func TestHealthResponse_JSON(t *testing.T) {
	response := HealthResponse{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
		Checks: map[string]CheckResult{
			"db": {
				Status:   StatusHealthy,
				Duration: 10 * time.Millisecond,
			},
		},
		Version:   "1.0.0",
		ServiceID: "test-123",
	}
	
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal HealthResponse: %v", err)
	}
	
	// Verify all fields are present
	requiredFields := []string{"status", "timestamp", "checks", "version", "service_id"}
	for _, field := range requiredFields {
		if !strings.Contains(string(data), field) {
			t.Errorf("field %q not found in JSON", field)
		}
	}
}