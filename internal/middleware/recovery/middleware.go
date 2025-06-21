package recovery

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"gateway/internal/core"
	gwerrors "gateway/pkg/errors"
)

// Config holds recovery middleware configuration
type Config struct {
	// StackTrace enables stack trace logging
	StackTrace bool
	// PanicHandler is called when a panic occurs (optional)
	PanicHandler func(ctx context.Context, recovered interface{}, stack []byte)
}

// Middleware creates panic recovery middleware
func Middleware(config Config, logger *slog.Logger) core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (resp core.Response, err error) {
			defer func() {
				if r := recover(); r != nil {
					// Get stack trace
					stack := debug.Stack()

					// Log the panic
					logger.Error("panic recovered",
						"panic", r,
						"path", req.Path(),
						"method", req.Method(),
					)

					if config.StackTrace {
						logger.Error("stack trace",
							"stack", string(stack),
						)
					}

					// Call panic handler if configured
					if config.PanicHandler != nil {
						config.PanicHandler(ctx, r, stack)
					}

					// Convert panic to error
					err = &gwerrors.Error{
						Type:    gwerrors.ErrorTypeInternal,
						Message: "Internal server error",
						Details: map[string]interface{}{
							"panic": fmt.Sprintf("%v", r),
						},
					}
				}
			}()

			// Call next handler
			return next(ctx, req)
		}
	}
}

// Default creates recovery middleware with default configuration
func Default(logger *slog.Logger) core.Middleware {
	return Middleware(Config{
		StackTrace: true,
	}, logger)
}
