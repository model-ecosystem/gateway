package middleware

import (
	"context"
	"gateway/internal/core"
	"log/slog"
	"time"
)

// Chain combines multiple middleware
func Chain(middlewares ...core.Middleware) core.Middleware {
	return func(next core.Handler) core.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Logging adds request logging
func Logging(logger *slog.Logger) core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			start := time.Now()
			
			logger.Info("request",
				"id", req.ID(),
				"method", req.Method(),
				"path", req.Path(),
			)
			
			resp, err := next(ctx, req)
			
			logger.Info("response",
				"id", req.ID(),
				"duration", time.Since(start),
				"error", err,
			)
			
			return resp, err
		}
	}
}

// Recovery recovers from panics
func Recovery() core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (resp core.Response, err error) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic recovered", "panic", r, "request", req.ID())
					resp = core.NewResponse(500, []byte("Internal Server Error"))
				}
			}()
			
			return next(ctx, req)
		}
	}
}