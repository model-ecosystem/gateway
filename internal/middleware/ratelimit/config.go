package ratelimit

import (
	"log/slog"

	"gateway/internal/storage"
)

// Config defines rate limit configuration with storage backend
type Config struct {
	// Rate is requests per second
	Rate int
	// Burst is the maximum burst size
	Burst int
	// KeyFunc extracts the rate limit key from request
	KeyFunc KeyFunc
	// Logger for logging
	Logger *slog.Logger
	// Store is the storage backend
	Store storage.LimiterStore
}
