package telemetry

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/propagation"
)

// HeaderCarrier adapts http.Header to propagation.TextMapCarrier
type HeaderCarrier http.Header

// Get returns the value for a key
func (hc HeaderCarrier) Get(key string) string {
	return http.Header(hc).Get(key)
}

// Set sets the value for a key
func (hc HeaderCarrier) Set(key, value string) {
	http.Header(hc).Set(key, value)
}

// Keys returns all keys
func (hc HeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(hc))
	for k := range hc {
		keys = append(keys, k)
	}
	return keys
}

// MapCarrier adapts map[string]string to propagation.TextMapCarrier
type MapCarrier map[string]string

// Get returns the value for a key
func (mc MapCarrier) Get(key string) string {
	return mc[key]
}

// Set sets the value for a key
func (mc MapCarrier) Set(key, value string) {
	mc[key] = value
}

// Keys returns all keys
func (mc MapCarrier) Keys() []string {
	keys := make([]string, 0, len(mc))
	for k := range mc {
		keys = append(keys, k)
	}
	return keys
}

// InjectContext injects trace context into a carrier
func InjectContext(ctx context.Context, propagator propagation.TextMapPropagator, carrier propagation.TextMapCarrier) {
	propagator.Inject(ctx, carrier)
}

// ExtractContext extracts trace context from a carrier
func ExtractContext(ctx context.Context, propagator propagation.TextMapPropagator, carrier propagation.TextMapCarrier) context.Context {
	return propagator.Extract(ctx, carrier)
}

// PropagateHTTPHeaders propagates trace context through HTTP headers
func PropagateHTTPHeaders(ctx context.Context, req *http.Request, propagator propagation.TextMapPropagator) {
	propagator.Inject(ctx, HeaderCarrier(req.Header))
}

// ExtractHTTPHeaders extracts trace context from HTTP headers
func ExtractHTTPHeaders(ctx context.Context, req *http.Request, propagator propagation.TextMapPropagator) context.Context {
	return propagator.Extract(ctx, HeaderCarrier(req.Header))
}