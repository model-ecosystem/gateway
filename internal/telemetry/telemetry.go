package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds telemetry configuration
type Config struct {
	Enabled bool   `yaml:"enabled"`
	Service string `yaml:"service"`
	Version string `yaml:"version"`

	Tracing TracingConfig `yaml:"tracing"`
	Metrics MetricsConfig `yaml:"metrics"`
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	Enabled      bool              `yaml:"enabled"`
	Endpoint     string            `yaml:"endpoint"`
	Headers      map[string]string `yaml:"headers"`
	SampleRate   float64           `yaml:"sampleRate"`
	MaxBatchSize int               `yaml:"maxBatchSize"`
	BatchTimeout int               `yaml:"batchTimeout"` // seconds
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
	Port    int    `yaml:"port"`
}

// Telemetry manages OpenTelemetry providers
type Telemetry struct {
	config       Config
	tracer       trace.Tracer
	meter        metric.Meter
	shutdown     []func(context.Context) error
	resource     *resource.Resource
	propagator   propagation.TextMapPropagator
}

// New creates a new telemetry instance
func New(config Config) (*Telemetry, error) {
	t := &Telemetry{
		config:   config,
		shutdown: make([]func(context.Context) error, 0),
	}

	if !config.Enabled {
		// Return no-op telemetry
		t.tracer = otel.GetTracerProvider().Tracer("gateway")
		t.meter = otel.GetMeterProvider().Meter("gateway")
		t.propagator = propagation.NewCompositeTextMapPropagator()
		return t, nil
	}

	// Create resource
	if err := t.initResource(); err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize tracing
	if config.Tracing.Enabled {
		if err := t.initTracing(); err != nil {
			return nil, fmt.Errorf("failed to initialize tracing: %w", err)
		}
	} else {
		// Use no-op tracer when disabled
		t.tracer = otel.GetTracerProvider().Tracer("gateway")
	}

	// Initialize metrics
	if config.Metrics.Enabled {
		if err := t.initMetrics(); err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
	} else {
		// Use no-op meter when disabled
		t.meter = otel.GetMeterProvider().Meter("gateway")
	}

	// Set up propagator
	t.propagator = propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(t.propagator)

	return t, nil
}

// initResource creates the OpenTelemetry resource
func (t *Telemetry) initResource() error {
	// Define service attributes
	attrs := []attribute.KeyValue{
		semconv.ServiceName(t.config.Service),
		semconv.ServiceVersion(t.config.Version),
	}

	// Create resource
	resource, err := resource.New(
		context.Background(),
		resource.WithAttributes(attrs...),
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	t.resource = resource
	return nil
}

// initTracing initializes the tracing provider
func (t *Telemetry) initTracing() error {
	ctx := context.Background()

	// Create OTLP trace exporter
	opts := []otlptracehttp.Option{
		otlptracehttp.WithTimeout(time.Second * 30),
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  time.Minute,
		}),
	}

	if t.config.Tracing.Endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(t.config.Tracing.Endpoint))
	}

	if len(t.config.Tracing.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(t.config.Tracing.Headers))
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Configure batch processor
	batchOpts := []sdktrace.BatchSpanProcessorOption{}
	if t.config.Tracing.MaxBatchSize > 0 {
		batchOpts = append(batchOpts, sdktrace.WithMaxExportBatchSize(t.config.Tracing.MaxBatchSize))
	}
	if t.config.Tracing.BatchTimeout > 0 {
		batchOpts = append(batchOpts, sdktrace.WithBatchTimeout(time.Duration(t.config.Tracing.BatchTimeout)*time.Second))
	}

	// Configure sampler
	var sampler sdktrace.Sampler
	if t.config.Tracing.SampleRate > 0 && t.config.Tracing.SampleRate < 1 {
		sampler = sdktrace.TraceIDRatioBased(t.config.Tracing.SampleRate)
	} else {
		sampler = sdktrace.AlwaysSample()
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, batchOpts...),
		sdktrace.WithResource(t.resource),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	t.tracer = tp.Tracer("gateway")
	t.shutdown = append(t.shutdown, tp.Shutdown)

	return nil
}

// initMetrics initializes the metrics provider
func (t *Telemetry) initMetrics() error {
	// Create Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		return fmt.Errorf("failed to create metrics exporter: %w", err)
	}

	// Create meter provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(t.resource),
	)

	otel.SetMeterProvider(mp)
	t.meter = mp.Meter("gateway")
	t.shutdown = append(t.shutdown, mp.Shutdown)

	return nil
}

// Tracer returns the tracer
func (t *Telemetry) Tracer() trace.Tracer {
	return t.tracer
}

// Meter returns the meter
func (t *Telemetry) Meter() metric.Meter {
	return t.meter
}

// Propagator returns the propagator
func (t *Telemetry) Propagator() propagation.TextMapPropagator {
	return t.propagator
}

// Shutdown gracefully shuts down telemetry providers
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	for _, fn := range t.shutdown {
		if err := fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// StartSpan starts a new span with common attributes
func (t *Telemetry) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// RecordError records an error on the span from context
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
	}
}

// SetStatus sets the status on the span from context
func SetStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetStatus(code, description)
	}
}

// LogEvent logs an event to slog with trace context
func LogEvent(ctx context.Context, level slog.Level, msg string, args ...any) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		// Add trace context to log attributes
		spanCtx := span.SpanContext()
		args = append(args,
			"trace_id", spanCtx.TraceID().String(),
			"span_id", spanCtx.SpanID().String(),
		)
	}
	slog.LogAttrs(ctx, level, msg, slog.Group("telemetry", args...))
}