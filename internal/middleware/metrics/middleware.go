package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"
	
	"gateway/internal/core"
	"gateway/internal/metrics"
)

// responseWriter wraps http.ResponseWriter to capture status code and size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// Middleware creates metrics collection middleware
func Middleware(m *metrics.Metrics) core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Get the HTTP request from context (set by HTTP adapter)
			httpReq, ok := ctx.Value("http.request").(*http.Request)
			if !ok {
				// If no HTTP request in context, just pass through
				return next(ctx, req)
			}
			
			// Normalize path for metrics
			path := metrics.NormalizePath(req.Path())
			method := req.Method()
			
			// Track active requests
			m.ActiveRequests.WithLabelValues(method, path).Inc()
			defer m.ActiveRequests.WithLabelValues(method, path).Dec()
			
			// Track request size
			if httpReq.ContentLength > 0 {
				m.RequestSize.WithLabelValues(method, path).Observe(float64(httpReq.ContentLength))
			}
			
			// Time the request
			start := time.Now()
			
			// Call next handler
			resp, err := next(ctx, req)
			
			// Calculate duration
			duration := time.Since(start).Seconds()
			
			// Determine status code
			statusCode := 200
			if resp != nil {
				statusCode = resp.StatusCode()
			}
			if err != nil && statusCode == 200 {
				// If there was an error but no explicit status set, use 500
				statusCode = 500
			}
			statusStr := strconv.Itoa(statusCode)
			
			// Record metrics
			m.RequestsTotal.WithLabelValues(method, path, statusStr).Inc()
			m.RequestDuration.WithLabelValues(method, path, statusStr).Observe(duration)
			
			// Track response size if available
			if resp != nil && resp.Body() != nil {
				// We can't easily measure response size without buffering the entire response
				// This would need to be done at the HTTP adapter level
			}
			
			return resp, err
		}
	}
}