package ratelimit

import (
	"context"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"log/slog"
)

// Config defines rate limit configuration
type Config struct {
	// Rate is requests per second
	Rate int
	// Burst is the maximum burst size
	Burst int
	// KeyFunc extracts the rate limit key from request
	KeyFunc KeyFunc
	// Logger for logging
	Logger *slog.Logger
}

// Middleware creates a rate limiting middleware
func Middleware(cfg *Config) core.Middleware {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = ByIP
	}

	limiter := NewTokenBucket(cfg.Rate, cfg.Burst)

	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			key := cfg.KeyFunc(req)

			if !limiter.Allow(key) {
				if cfg.Logger != nil {
					cfg.Logger.Warn("rate limit exceeded",
						"key", key,
						"path", req.Path(),
						"method", req.Method(),
					)
				}

				return nil, errors.NewError(
					errors.ErrorTypeRateLimit,
					"rate limit exceeded",
				).WithDetail("key", key)
			}

			return next(ctx, req)
		}
	}
}

// PerRoute creates a rate limiter with per-route configuration
func PerRoute(rules map[string]*Config) core.Middleware {
	limiters := make(map[string]*TokenBucket)

	for path, cfg := range rules {
		limiters[path] = NewTokenBucket(cfg.Rate, cfg.Burst)
	}

	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Find matching rule
			var limiter *TokenBucket
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
			if !limiter.Allow(key) {
				if cfg.Logger != nil {
					cfg.Logger.Warn("rate limit exceeded",
						"key", key,
						"path", req.Path(),
						"method", req.Method(),
					)
				}

				return nil, errors.NewError(
					errors.ErrorTypeRateLimit,
					"rate limit exceeded",
				).WithDetail("key", key).WithDetail("path", req.Path())
			}

			return next(ctx, req)
		}
	}
}

// matchPath checks if a request path matches a pattern
func matchPath(reqPath, pattern string) bool {
	// Simple prefix matching for now
	// TODO: Support wildcard patterns
	return len(reqPath) >= len(pattern) && reqPath[:len(pattern)] == pattern
}
