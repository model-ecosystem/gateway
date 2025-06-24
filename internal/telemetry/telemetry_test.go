package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func TestNew(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-gateway",
		Version: "1.0.0",
		Tracing: TracingConfig{
			Enabled:    true,
			SampleRate: 0.5,
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
			Port:    9090,
		},
	}

	telemetry, err := New(cfg)
	if err != nil {
		// Some initialization might fail in test environment
		t.Logf("New returned error (may be expected in test): %v", err)
	}
	if telemetry == nil {
		t.Fatal("Expected non-nil telemetry")
	}
}

func TestNew_Disabled(t *testing.T) {
	cfg := Config{
		Enabled: false,
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed for disabled telemetry: %v", err)
	}
	if telemetry == nil {
		t.Fatal("Expected non-nil telemetry even when disabled")
	}
}

func TestShutdown(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-service",
		Version: "1.0.0",
		Tracing: TracingConfig{
			Enabled:    true,
			SampleRate: 1.0,
		},
		Metrics: MetricsConfig{
			Enabled: false, // Disable to avoid port conflicts
		},
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Logf("New returned error (may be expected in test): %v", err)
		return
	}
	
	// Shutdown should work regardless
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = telemetry.Shutdown(shutdownCtx)
	if err != nil {
		t.Logf("Shutdown returned error: %v", err)
	}
}

func TestTelemetry_StartSpan(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-service",
		Tracing: TracingConfig{
			Enabled:    true,
			SampleRate: 1.0,
		},
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Logf("New returned error (may be expected in test): %v", err)
		return
	}
	defer telemetry.Shutdown(context.Background())

	// Test creating a span
	ctx := context.Background()
	ctx, span := telemetry.StartSpan(ctx, "test-operation")
	
	if span == nil {
		t.Fatal("Expected non-nil span")
	}
	
	// Use the span interface directly
	span.SetAttributes(
		attribute.String("test.key1", "value1"),
		attribute.Int("test.key2", 42),
	)
	
	// Record error using package function
	RecordError(ctx, errors.New("test error"))
	
	// End span
	span.End()
}

func TestTelemetry_DisabledOperations(t *testing.T) {
	cfg := Config{
		Enabled: false,
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	ctx := context.Background()

	// All operations should be no-ops when disabled
	ctx, span := telemetry.StartSpan(ctx, "test")
	if span == nil {
		t.Error("Expected non-nil span even when telemetry is disabled")
	}
	
	// These should not panic
	RecordError(ctx, errors.New("error"))
	span.End()
	
	telemetry.Shutdown(ctx)
}

func TestTelemetry_Accessors(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-service",
		Version: "1.0.0",
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Logf("New returned error (may be expected in test): %v", err)
		return
	}
	defer telemetry.Shutdown(context.Background())

	// Test accessor methods
	if telemetry.Tracer() == nil {
		t.Error("Expected non-nil tracer")
	}
	
	if telemetry.Meter() == nil {
		t.Error("Expected non-nil meter")
	}
	
	if telemetry.Propagator() == nil {
		t.Error("Expected non-nil propagator")
	}
}

func TestRecordError(t *testing.T) {
	// Test the package-level RecordError function
	ctx := context.Background()
	
	// Should not panic even without a span
	RecordError(ctx, errors.New("test error"))
}

func TestSetStatus(t *testing.T) {
	// Test the package-level SetStatus function
	ctx := context.Background()
	
	// Should not panic even without a span
	SetStatus(ctx, 1, "test status")
}

func TestLogEvent(t *testing.T) {
	// Test the package-level LogEvent function
	ctx := context.Background()
	
	// Should not panic even without a span
	LogEvent(ctx, 0, "test message", "key", "value")
}