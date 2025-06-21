package ratelimit

import (
	"context"
	"strings"
	
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// Middleware creates rate limiting middleware with storage backend
func Middleware(cfg *Config) core.Middleware {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = ByIP
	}

	// Create limiter with storage backend
	limiter := NewStoreLimiter(cfg.Store, cfg.Rate, cfg.Burst)

	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			key := cfg.KeyFunc(req)

			// Use the limiter with storage backend
			if err := limiter.Allow(ctx, key); err != nil {
				if cfg.Logger != nil {
					cfg.Logger.Warn("rate limit check",
						"key", key,
						"path", req.Path(),
						"method", req.Method(),
						"error", err,
					)
				}

				// If it's already a rate limit error, return it
				var rateLimitErr *errors.Error
				if errors.As(err, &rateLimitErr) && rateLimitErr.Type == errors.ErrorTypeRateLimit {
					return nil, err
				}

				// Otherwise, wrap it
				return nil, errors.NewError(
					errors.ErrorTypeRateLimit,
					"rate limit exceeded",
				).WithDetail("key", key).WithCause(err)
			}

			return next(ctx, req)
		}
	}
}

// PerRoute creates a rate limiter with per-route configuration
func PerRoute(rules map[string]*Config) core.Middleware {
	// Create limiters for each route
	limiters := make(map[string]*StoreLimiter)

	for path, cfg := range rules {
		limiters[path] = NewStoreLimiter(cfg.Store, cfg.Rate, cfg.Burst)
	}

	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Find matching rule
			var limiter *StoreLimiter
			var cfg *Config

			for path, rl := range rules {
				if matchPath(req.Path(), path) {
					limiter = limiters[path]
					cfg = rl
					break
				}
			}

			// No rate limit for this path
			if limiter == nil || cfg == nil {
				return next(ctx, req)
			}

			// Use default key function if not specified
			keyFunc := cfg.KeyFunc
			if keyFunc == nil {
				keyFunc = ByIP
			}

			key := keyFunc(req)
			if err := limiter.Allow(ctx, key); err != nil {
				if cfg.Logger != nil {
					cfg.Logger.Warn("rate limit check",
						"key", key,
						"path", req.Path(),
						"method", req.Method(),
						"error", err,
					)
				}

				// If it's already a rate limit error, return it
				var rateLimitErr *errors.Error
				if errors.As(err, &rateLimitErr) && rateLimitErr.Type == errors.ErrorTypeRateLimit {
					return nil, err
				}

				// Otherwise, wrap it
				return nil, errors.NewError(
					errors.ErrorTypeRateLimit,
					"rate limit exceeded",
				).WithDetail("key", key).WithDetail("path", req.Path()).WithCause(err)
			}

			return next(ctx, req)
		}
	}
}

// matchPath checks if a request path matches a pattern
func matchPath(requestPath, pattern string) bool {
	// Exact match
	if requestPath == pattern {
		return true
	}
	
	// Wildcard match
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(requestPath, prefix)
	}
	
	return false
}