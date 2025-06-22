package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"gateway/internal/core"
	"gateway/internal/metrics"
)


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

			// Response size tracking would need to be done at the HTTP adapter level
			// to avoid buffering the entire response

			return resp, err
		}
	}
}
