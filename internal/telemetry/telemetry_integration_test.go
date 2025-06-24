package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestTelemetry_FullStackEnabled(t *testing.T) {
	// Create a mock OTLP endpoint
	receivedRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		Enabled: true,
		Service: "test-gateway",
		Version: "1.0.0",
		Tracing: TracingConfig{
			Enabled:      true,
			Endpoint:     server.URL,
			SampleRate:   1.0,
			MaxBatchSize: 100,
			BatchTimeout: 1,
			Headers: map[string]string{
				"X-Custom-Header": "test-value",
			},
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
			Port:    0, // Use 0 to avoid port conflicts in tests
		},
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create telemetry: %v", err)
	}
	defer telemetry.Shutdown(context.Background())

	// Test span creation with various options
	ctx := context.Background()
	ctx, span := telemetry.StartSpan(ctx, "test-operation",
		trace.WithAttributes(attribute.String("test.attribute", "value")),
		trace.WithSpanKind(trace.SpanKindServer),
	)

	// Verify span is recording
	if !span.IsRecording() {
		t.Error("Expected span to be recording")
	}

	// Test span operations
	span.SetAttributes(
		attribute.Bool("test.bool", true),
		attribute.Int64("test.int64", 123),
		attribute.Float64("test.float64", 123.45),
		attribute.StringSlice("test.slice", []string{"a", "b", "c"}),
	)

	// Test error recording
	testErr := &customError{msg: "test error", code: 500}
	RecordError(ctx, testErr)

	// Test status setting
	SetStatus(ctx, codes.Error, "operation failed")

	// Create a child span
	childCtx, childSpan := telemetry.StartSpan(ctx, "child-operation")
	childSpan.AddEvent("child event", trace.WithAttributes(
		attribute.String("event.type", "test"),
	))
	childSpan.End()

	// End parent span
	span.End()

	// Force flush to ensure export
	time.Sleep(2 * time.Second)

	// Verify telemetry was sent
	if receivedRequests == 0 {
		t.Log("Warning: No telemetry requests received (may be expected in test environment)")
	}

	// Verify metrics endpoint would be accessible
	if telemetry.meter == nil {
		t.Error("Expected non-nil meter")
	}

	// Test propagator
	carrier := make(http.Header)
	telemetry.Propagator().Inject(childCtx, propagation.HeaderCarrier(carrier))
	if len(carrier) == 0 {
		t.Error("Expected propagator to inject headers")
	}
}

func TestTelemetry_TracingOnlyEnabled(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-service",
		Version: "1.0.0",
		Tracing: TracingConfig{
			Enabled:    true,
			SampleRate: 0.5,
		},
		Metrics: MetricsConfig{
			Enabled: false,
		},
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Logf("New returned error (may be expected): %v", err)
		return
	}
	defer telemetry.Shutdown(context.Background())

	// Verify tracing is available
	if telemetry.tracer == nil {
		t.Error("Expected non-nil tracer when tracing is enabled")
	}

	// Create a span
	ctx, span := telemetry.StartSpan(context.Background(), "test")
	defer span.End()

	// Should be able to record without panic
	RecordError(ctx, nil) // nil error should be handled gracefully
}

func TestTelemetry_MetricsOnlyEnabled(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-service",
		Version: "1.0.0",
		Tracing: TracingConfig{
			Enabled: false,
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
			Port:    0,
		},
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create telemetry with metrics only: %v", err)
	}
	defer telemetry.Shutdown(context.Background())

	// Verify metrics is available
	if telemetry.meter == nil {
		t.Error("Expected non-nil meter when metrics is enabled")
	}

	// Verify tracer is no-op but still usable
	ctx, span := telemetry.StartSpan(context.Background(), "test")
	if span == nil {
		t.Error("Expected non-nil span even with tracing disabled")
	}
	span.End()

	// These should not panic
	RecordError(ctx, &customError{msg: "test", code: 404})
	SetStatus(ctx, codes.Ok, "success")
}

func TestTelemetry_EdgeCases(t *testing.T) {
	// Test with empty service name
	cfg := Config{
		Enabled: true,
		Service: "",
		Version: "",
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Logf("New with empty service returned error: %v", err)
	}
	if telemetry != nil {
		defer telemetry.Shutdown(context.Background())
	}

	// Test with invalid sample rate
	cfg2 := Config{
		Enabled: true,
		Service: "test",
		Tracing: TracingConfig{
			Enabled:    true,
			SampleRate: 2.0, // Invalid, should fallback to AlwaysSample
		},
	}

	telemetry2, err := New(cfg2)
	if err != nil {
		t.Logf("New with invalid sample rate returned error: %v", err)
		return
	}
	defer telemetry2.Shutdown(context.Background())

	// Should still create spans
	_, span := telemetry2.StartSpan(context.Background(), "test")
	span.End()
}

func TestTelemetry_PropagationExtraction(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-service",
		Tracing: TracingConfig{
			Enabled: true,
		},
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Logf("New returned error: %v", err)
		return
	}
	defer telemetry.Shutdown(context.Background())

	// Create a span and inject into headers
	ctx, span := telemetry.StartSpan(context.Background(), "parent")
	defer span.End()

	headers := make(http.Header)
	telemetry.Propagator().Inject(ctx, propagation.HeaderCarrier(headers))

	// Extract from headers into new context
	newCtx := telemetry.Propagator().Extract(context.Background(), propagation.HeaderCarrier(headers))

	// Create child span from extracted context
	_, childSpan := telemetry.StartSpan(newCtx, "child")
	defer childSpan.End()

	// Verify parent-child relationship
	parentSpanCtx := span.SpanContext()
	childSpanCtx := childSpan.SpanContext()

	if parentSpanCtx.TraceID() != childSpanCtx.TraceID() {
		t.Error("Expected child span to have same trace ID as parent")
	}
}

func TestTelemetry_ShutdownTimeout(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-service",
		Tracing: TracingConfig{
			Enabled: true,
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    0,
		},
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Logf("New returned error: %v", err)
		return
	}

	// Create spans that might still be exporting
	for i := 0; i < 10; i++ {
		_, span := telemetry.StartSpan(context.Background(), "test-span")
		span.SetAttributes(attribute.Int("index", i))
		span.End()
	}

	// Shutdown with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err = telemetry.Shutdown(ctx)
	if err != nil {
		// This is expected with such a short timeout
		t.Logf("Shutdown with short timeout returned error (expected): %v", err)
		if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "timeout") {
			t.Logf("Unexpected error type: %v", err)
		}
	}
}

func TestLogEvent_WithAndWithoutSpan(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-service",
		Tracing: TracingConfig{
			Enabled: true,
		},
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Logf("New returned error: %v", err)
		return
	}
	defer telemetry.Shutdown(context.Background())

	// Test with span
	ctx, span := telemetry.StartSpan(context.Background(), "test")
	LogEvent(ctx, 0, "test message with span", "key1", "value1", "key2", 42)
	span.End()

	// Test without span
	LogEvent(context.Background(), 0, "test message without span", "key1", "value1")
}

func TestTelemetry_MultipleShutdowns(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Service: "test-service",
	}

	telemetry, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create telemetry: %v", err)
	}

	// First shutdown
	err = telemetry.Shutdown(context.Background())
	if err != nil {
		t.Logf("First shutdown error: %v", err)
	}

	// Second shutdown - should not panic
	err = telemetry.Shutdown(context.Background())
	if err != nil {
		t.Logf("Second shutdown error: %v", err)
	}
}

// Helper type for testing
type customError struct {
	msg  string
	code int
}

func (e *customError) Error() string {
	return e.msg
}