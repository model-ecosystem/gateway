package cors

import (
	"context"
	"io"
	"net/http"

	"gateway/internal/core"
)

// Middleware creates CORS middleware for the gateway
func Middleware(config Config) core.Middleware {
	cors := New(config)

	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Get HTTP request from context
			httpReq, ok := ctx.Value("http.request").(*http.Request)
			if !ok {
				// If no HTTP request in context, just pass through
				return next(ctx, req)
			}

			// Get HTTP response writer from context
			httpWriter, ok := ctx.Value("http.writer").(http.ResponseWriter)
			if !ok {
				// If no HTTP writer in context, just pass through
				return next(ctx, req)
			}

			origin := httpReq.Header.Get("Origin")

			// Check if this is a preflight request
			if httpReq.Method == http.MethodOptions && httpReq.Header.Get("Access-Control-Request-Method") != "" {
				cors.handlePreflight(httpWriter, httpReq, origin)

				if !config.OptionsPassthrough {
					// Return empty response for preflight
					return &corsResponse{
						statusCode: config.OptionsSuccessStatus,
						headers:    make(map[string][]string),
					}, nil
				}
			} else {
				cors.handleActualRequest(httpWriter, httpReq, origin)
			}

			// Continue to next handler
			return next(ctx, req)
		}
	}
}

// corsResponse implements core.Response for CORS preflight responses
type corsResponse struct {
	statusCode int
	headers    map[string][]string
}

func (r *corsResponse) StatusCode() int {
	return r.statusCode
}

func (r *corsResponse) Body() io.ReadCloser {
	return nil
}

func (r *corsResponse) Headers() map[string][]string {
	return r.headers
}
