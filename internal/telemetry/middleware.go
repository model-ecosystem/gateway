package telemetry

import (
	"context"
	"net/http"
	"time"

	"gateway/internal/core"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Middleware wraps handlers with telemetry
type Middleware struct {
	telemetry *Telemetry
	metrics   *Metrics
}

// NewMiddleware creates a new telemetry middleware
func NewMiddleware(telemetry *Telemetry, metrics *Metrics) *Middleware {
	return &Middleware{
		telemetry: telemetry,
		metrics:   metrics,
	}
}

// WrapHTTP wraps an HTTP handler with telemetry
func (m *Middleware) WrapHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start server span
		ctx, span := m.telemetry.StartHTTPServerSpan(r)
		defer span.End()
		
		// Update request context
		r = r.WithContext(ctx)
		
		// Record active request
		m.metrics.RecordHTTPActiveRequest(ctx, 1)
		defer m.metrics.RecordHTTPActiveRequest(ctx, -1)
		
		// Record request size
		if r.ContentLength > 0 {
			m.metrics.RecordHTTPRequestSize(ctx, r.ContentLength)
		}
		
		// Wrap response writer to capture status and size
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		
		// Time the request
		start := time.Now()
		
		// Handle the request
		next.ServeHTTP(rw, r)
		
		// Record metrics
		duration := time.Since(start)
		m.metrics.RecordHTTPRequest(ctx, r.Method, r.URL.Path, rw.statusCode, duration)
		
		if rw.written > 0 {
			m.metrics.RecordHTTPResponseSize(ctx, rw.written)
		}
		
		// Update span status
		EndHTTPServerSpan(span, rw.statusCode)
	})
}

// WrapHandler wraps a core.Handler with telemetry
func (m *Middleware) WrapHandler(name string, handler core.Handler) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Start span
		ctx, span := m.telemetry.StartSpan(ctx, name,
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("handler.name", name),
				attribute.String("request.method", req.Method()),
				attribute.String("request.path", req.Path()),
			),
		)
		defer span.End()
		
		// Time the handler
		start := time.Now()
		
		// Call the handler
		resp, err := handler(ctx, req)
		
		// Record duration
		duration := time.Since(start)
		span.SetAttributes(attribute.Float64("handler.duration_ms", float64(duration.Milliseconds())))
		
		// Handle error
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return resp, err
		}
		
		// Set response attributes
		if resp != nil {
			statusCode := resp.StatusCode()
			span.SetAttributes(
				attribute.Int("response.status", statusCode),
			)
			
			if statusCode >= 400 {
				span.SetStatus(codes.Error, http.StatusText(statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		}
		
		return resp, err
	}
}

// WrapWebSocketHandler wraps a WebSocket handler with telemetry
// WebSocket handlers in this architecture are just core.Handler functions
func (m *Middleware) WrapWebSocketHandler(name string, handler core.Handler) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Start WebSocket span
		ctx, span := m.telemetry.StartWebSocketSpan(ctx, name,
			attribute.String("request.path", req.Path()),
			attribute.String("request.method", req.Method()),
		)
		defer span.End()
		
		// Record connection
		m.metrics.RecordWebSocketConnection(ctx, req.Path())
		m.metrics.RecordWebSocketActiveConnection(ctx, 1)
		defer m.metrics.RecordWebSocketActiveConnection(ctx, -1)
		
		// Call handler
		resp, err := handler(ctx, req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		
		return resp, err
	}
}

// WrapSSEHandler wraps SSE handler with telemetry
// SSE handlers in this architecture are just core.Handler functions
func (m *Middleware) WrapSSEHandler(name string, handler core.Handler) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Start SSE span
		ctx, span := m.telemetry.StartSSESpan(ctx, name,
			attribute.String("request.path", req.Path()),
			attribute.String("request.method", req.Method()),
		)
		defer span.End()
		
		// Record connection
		m.metrics.RecordSSEConnection(ctx, req.Path())
		m.metrics.RecordSSEActiveConnection(ctx, 1)
		defer m.metrics.RecordSSEActiveConnection(ctx, -1)
		
		// Call handler
		resp, err := handler(ctx, req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		
		return resp, err
	}
}

// responseWriter wraps http.ResponseWriter to capture status and size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// telemetryWebSocketConn wraps WebSocket connection for metrics
type telemetryWebSocketConn struct {
	core.WebSocketConn
	ctx     context.Context
	metrics *Metrics
}

func (c *telemetryWebSocketConn) WriteMessage(msg *core.WebSocketMessage) error {
	err := c.WebSocketConn.WriteMessage(msg)
	if err == nil && msg != nil {
		c.metrics.RecordWebSocketMessage(c.ctx, "sent", int64(len(msg.Data)))
	}
	return err
}

func (c *telemetryWebSocketConn) ReadMessage() (*core.WebSocketMessage, error) {
	msg, err := c.WebSocketConn.ReadMessage()
	if err == nil && msg != nil {
		c.metrics.RecordWebSocketMessage(c.ctx, "received", int64(len(msg.Data)))
	}
	return msg, err
}

// telemetrySSEWriter wraps SSE writer for metrics
type telemetrySSEWriter struct {
	core.SSEWriter
	ctx     context.Context
	metrics *Metrics
}

func (w *telemetrySSEWriter) WriteEvent(event *core.SSEEvent) error {
	// Calculate event size
	size := int64(len(event.Type) + len(event.Data) + len(event.ID))
	
	err := w.SSEWriter.WriteEvent(event)
	if err == nil {
		w.metrics.RecordSSEEvent(w.ctx, size)
	}
	return err
}

// ExtractTraceID extracts trace ID from context
func ExtractTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// ExtractSpanID extracts span ID from context
func ExtractSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// InjectHTTPHeaders injects trace context into HTTP headers
func (m *Middleware) InjectHTTPHeaders(ctx context.Context, headers http.Header) {
	m.telemetry.propagator.Inject(ctx, propagation.HeaderCarrier(headers))
}

// ExtractHTTPHeaders extracts trace context from HTTP headers
func (m *Middleware) ExtractHTTPHeaders(ctx context.Context, headers http.Header) context.Context {
	return m.telemetry.propagator.Extract(ctx, propagation.HeaderCarrier(headers))
}