package telemetry

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// SpanKind represents the kind of span
type SpanKind string

const (
	SpanKindServer   SpanKind = "server"
	SpanKindClient   SpanKind = "client"
	SpanKindProducer SpanKind = "producer"
	SpanKindConsumer SpanKind = "consumer"
	SpanKindInternal SpanKind = "internal"
)

// StartHTTPServerSpan starts a new HTTP server span
func (t *Telemetry) StartHTTPServerSpan(r *http.Request) (context.Context, trace.Span) {
	ctx := r.Context()
	
	// Extract trace context from headers
	ctx = t.propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))
	
	// Start span with HTTP attributes
	ctx, span := t.tracer.Start(ctx, 
		fmt.Sprintf("%s %s", r.Method, r.URL.Path),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			semconv.HTTPMethod(r.Method),
			semconv.HTTPTarget(r.RequestURI),
			semconv.HTTPRoute(r.URL.Path),
			semconv.HTTPScheme(r.URL.Scheme),
			semconv.NetHostName(r.Host),
			attribute.String("net.peer.addr", r.RemoteAddr),
			semconv.HTTPUserAgent(r.UserAgent()),
		),
	)
	
	return ctx, span
}

// StartHTTPClientSpan starts a new HTTP client span
func (t *Telemetry) StartHTTPClientSpan(ctx context.Context, req *http.Request) (context.Context, trace.Span) {
	// Start span with HTTP attributes
	ctx, span := t.tracer.Start(ctx,
		fmt.Sprintf("HTTP %s %s", req.Method, req.URL.Host),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.HTTPMethod(req.Method),
			semconv.HTTPTarget(req.URL.RequestURI()),
			semconv.HTTPScheme(req.URL.Scheme),
			semconv.NetPeerName(req.URL.Host),
		),
	)
	
	// Inject trace context into headers
	t.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
	
	return ctx, span
}

// EndHTTPServerSpan ends an HTTP server span with status
func EndHTTPServerSpan(span trace.Span, statusCode int) {
	if !span.IsRecording() {
		return
	}
	
	// Set HTTP status code
	span.SetAttributes(semconv.HTTPStatusCode(statusCode))
	
	// Set span status based on HTTP status
	if statusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}
	
	span.End()
}

// EndHTTPClientSpan ends an HTTP client span with response
func EndHTTPClientSpan(span trace.Span, resp *http.Response, err error) {
	if !span.IsRecording() {
		return
	}
	
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else if resp != nil {
		span.SetAttributes(semconv.HTTPStatusCode(resp.StatusCode))
		if resp.StatusCode >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
	
	span.End()
}

// StartSpanWithKind starts a span with specific kind and attributes
func (t *Telemetry) StartSpanWithKind(ctx context.Context, name string, kind SpanKind, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	var spanKind trace.SpanKind
	switch kind {
	case SpanKindServer:
		spanKind = trace.SpanKindServer
	case SpanKindClient:
		spanKind = trace.SpanKindClient
	case SpanKindProducer:
		spanKind = trace.SpanKindProducer
	case SpanKindConsumer:
		spanKind = trace.SpanKindConsumer
	default:
		spanKind = trace.SpanKindInternal
	}
	
	return t.tracer.Start(ctx, name,
		trace.WithSpanKind(spanKind),
		trace.WithAttributes(attrs...),
	)
}

// StartWebSocketSpan starts a WebSocket span
func (t *Telemetry) StartWebSocketSpan(ctx context.Context, operation string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	defaultAttrs := []attribute.KeyValue{
		attribute.String("protocol", "websocket"),
		attribute.String("operation", operation),
	}
	attrs = append(defaultAttrs, attrs...)
	
	return t.tracer.Start(ctx, fmt.Sprintf("WebSocket %s", operation),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attrs...),
	)
}

// StartSSESpan starts a Server-Sent Events span
func (t *Telemetry) StartSSESpan(ctx context.Context, operation string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	defaultAttrs := []attribute.KeyValue{
		attribute.String("protocol", "sse"),
		attribute.String("operation", operation),
	}
	attrs = append(defaultAttrs, attrs...)
	
	return t.tracer.Start(ctx, fmt.Sprintf("SSE %s", operation),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attrs...),
	)
}

// StartGRPCSpan starts a gRPC span
func (t *Telemetry) StartGRPCSpan(ctx context.Context, method string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	defaultAttrs := []attribute.KeyValue{
		attribute.String("rpc.system", "grpc"),
		attribute.String("rpc.method", method),
	}
	attrs = append(defaultAttrs, attrs...)
	
	return t.tracer.Start(ctx, fmt.Sprintf("gRPC %s", method),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// SetAttributes sets attributes on the current span
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// SpanFromContext returns the span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan returns a new context with the span
func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}