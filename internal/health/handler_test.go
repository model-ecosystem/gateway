package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChecker_RegisterAndCheck(t *testing.T) {
	checker := NewChecker()

	// Register a successful check
	checker.RegisterCheck("success", func(ctx context.Context) error {
		return nil
	})

	// Register a failing check
	checker.RegisterCheck("failure", func(ctx context.Context) error {
		return errors.New("check failed")
	})

	// Register a slow check
	checker.RegisterCheck("slow", func(ctx context.Context) error {
		select {
		case <-time.After(100 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// Run checks
	ctx := context.Background()
	results := checker.CheckHealth(ctx)

	// Verify results
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Check success
	if result, ok := results["success"]; ok {
		if result.Status != StatusHealthy {
			t.Errorf("Expected success check to be healthy, got %s", result.Status)
		}
		if result.Error != "" {
			t.Errorf("Expected no error for success check, got %s", result.Error)
		}
	} else {
		t.Error("Success check result not found")
	}

	// Check failure
	if result, ok := results["failure"]; ok {
		if result.Status != StatusUnhealthy {
			t.Errorf("Expected failure check to be unhealthy, got %s", result.Status)
		}
		if result.Error == "" {
			t.Error("Expected error for failure check")
		}
	} else {
		t.Error("Failure check result not found")
	}

	// Check slow
	if result, ok := results["slow"]; ok {
		if result.Status != StatusHealthy {
			t.Errorf("Expected slow check to be healthy, got %s", result.Status)
		}
		if result.Duration < 100*time.Millisecond {
			t.Errorf("Expected slow check to take at least 100ms, got %v", result.Duration)
		}
	} else {
		t.Error("Slow check result not found")
	}
}

func TestChecker_ContextCancellation(t *testing.T) {
	checker := NewChecker()

	// Register a check that respects context
	checker.RegisterCheck("context-aware", func(ctx context.Context) error {
		select {
		case <-time.After(1 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// Run with cancelled context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	results := checker.CheckHealth(ctx)
	duration := time.Since(start)

	// Should complete quickly due to context cancellation
	if duration > 200*time.Millisecond {
		t.Errorf("Check took too long: %v", duration)
	}

	// The check should still return a result (even if cancelled)
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestHandler_Health(t *testing.T) {
	checker := NewChecker()
	
	// Register checks
	checker.RegisterCheck("healthy", func(ctx context.Context) error {
		return nil
	})
	checker.RegisterCheck("unhealthy", func(ctx context.Context) error {
		return errors.New("service unavailable")
	})

	handler := NewHandler(checker, "1.0.0", "test-service")

	// Test health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.StatusCode)
	}

	// Check response body
	var healthResp HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if healthResp.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", healthResp.Status)
	}

	if healthResp.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", healthResp.Version)
	}

	if healthResp.ServiceID != "test-service" {
		t.Errorf("Expected service ID test-service, got %s", healthResp.ServiceID)
	}

	if len(healthResp.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(healthResp.Checks))
	}
}

func TestHandler_Ready(t *testing.T) {
	checker := NewChecker()
	
	// Register a healthy check
	checker.RegisterCheck("database", func(ctx context.Context) error {
		return nil
	})

	handler := NewHandler(checker, "1.0.0", "test-service")

	// Test ready endpoint
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	handler.Ready(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check response body
	var readyResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&readyResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if ready, ok := readyResp["ready"].(bool); !ok || !ready {
		t.Error("Expected ready to be true")
	}
}

func TestHandler_Live(t *testing.T) {
	checker := NewChecker()
	handler := NewHandler(checker, "1.0.0", "test-service")

	// Test live endpoint
	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()

	handler.Live(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check response body
	var liveResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&liveResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status, ok := liveResp["status"].(string); !ok || status != "ok" {
		t.Error("Expected status to be 'ok'")
	}
}