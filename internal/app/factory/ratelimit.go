package factory

import (
	"errors"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/internal/middleware/ratelimit"
	"gateway/internal/storage"
	"gateway/internal/storage/memory"
	"gateway/internal/storage/redis"
)

// CreateRateLimitMiddleware creates rate limiting middleware with configurable storage
func CreateRateLimitMiddleware(routerConfig *config.Router, gatewayConfig *config.Gateway, logger *slog.Logger) core.Middleware {
	// Create storage factory
	redisFactory := &RedisClientFactoryAdapter{}

	// Create stores based on configuration
	stores := make(map[string]storage.LimiterStore)

	if gatewayConfig != nil && gatewayConfig.RateLimitStorage != nil {
		// Create configured stores
		for name, storeCfg := range gatewayConfig.RateLimitStorage.Stores {
			store, err := createStore(name, storeCfg, redisFactory, logger)
			if err != nil {
				logger.Error("Failed to create rate limit store", "name", name, "error", err)
				// Fall back to memory store
				store = memory.NewStore(storage.DefaultConfig())
			}
			stores[name] = store
		}

		// Ensure default store exists
		if _, ok := stores["default"]; !ok {
			stores["default"] = memory.NewStore(storage.DefaultConfig())
		}
	} else {
		// No configuration, use default memory store
		stores["default"] = memory.NewStore(storage.DefaultConfig())
	}

	// Build per-route rate limit configuration
	rateLimitRules := make(map[string]*ratelimit.Config)

	for _, rule := range routerConfig.Rules {
		// Skip routes without rate limiting
		if rule.RateLimit <= 0 {
			continue
		}

		// Default burst to rate limit if not specified
		burst := rule.RateLimitBurst
		if burst <= 0 {
			burst = rule.RateLimit
		}

		// Determine which store to use
		storeName := rule.RateLimitStorage
		if storeName == "" {
			if gatewayConfig != nil && gatewayConfig.RateLimitStorage != nil {
				storeName = gatewayConfig.RateLimitStorage.Default
			}
			if storeName == "" {
				storeName = "default"
			}
		}

		store, exists := stores[storeName]
		if !exists {
			logger.Warn("Rate limit storage not found, using default",
				"route", rule.ID,
				"requestedStorage", storeName,
			)
			store = stores["default"]
		}

		config := &ratelimit.Config{
			Rate:    rule.RateLimit,
			Burst:   burst,
			KeyFunc: ratelimit.ByIP,
			Logger:  logger.With("middleware", "ratelimit", "route", rule.ID),
			Store:   store,
		}

		rateLimitRules[rule.Path] = config

		logger.Info("Rate limiting configured for route",
			"route", rule.ID,
			"path", rule.Path,
			"rate", rule.RateLimit,
			"burst", burst,
			"storage", storeName,
		)
	}

	// Return nil if no rate limiting configured
	if len(rateLimitRules) == 0 {
		return nil
	}

	// Create per-route rate limiter
	return ratelimit.PerRoute(rateLimitRules)
}

// CreateGlobalRateLimitMiddleware creates a global rate limiter with configurable storage
func CreateGlobalRateLimitMiddleware(rate, burst int, gatewayConfig *config.Gateway, logger *slog.Logger) core.Middleware {
	if rate <= 0 {
		return nil
	}

	if burst <= 0 {
		burst = rate
	}

	// Create storage
	var store storage.LimiterStore
	if gatewayConfig != nil && gatewayConfig.RateLimitStorage != nil && gatewayConfig.RateLimitStorage.Default != "" {
		// Create storage factory
		redisFactory := &RedisClientFactoryAdapter{}

		// Use the default storage from configuration
		if storeCfg, ok := gatewayConfig.RateLimitStorage.Stores[gatewayConfig.RateLimitStorage.Default]; ok {
			s, err := createStore("global", storeCfg, redisFactory, logger)
			if err != nil {
				logger.Error("Failed to create global rate limit store", "error", err)
				store = memory.NewStore(storage.DefaultConfig())
			} else {
				store = s
			}
		} else {
			store = memory.NewStore(storage.DefaultConfig())
		}
	} else {
		// No configuration, use default memory store
		store = memory.NewStore(storage.DefaultConfig())
	}

	config := &ratelimit.Config{
		Rate:    rate,
		Burst:   burst,
		KeyFunc: ratelimit.ByIP,
		Logger:  logger.With("middleware", "ratelimit", "scope", "global"),
		Store:   store,
	}

	logger.Info("Global rate limiting configured", "rate", rate, "burst", burst, "storage", "configurable")

	return ratelimit.Middleware(config)
}

// createStore creates a limiter store based on configuration
func createStore(name string, cfg *config.RateLimitStore, redisFactory *RedisClientFactoryAdapter, logger *slog.Logger) (storage.LimiterStore, error) {
	switch cfg.Type {
	case "memory", "":
		logger.Info("Creating memory limiter store", "name", name)
		return memory.NewStore(storage.DefaultConfig()), nil

	case "redis":
		if cfg.Redis == nil {
			return nil, errors.New("Redis configuration required for redis storage type")
		}

		// Create Redis client from storage-specific config
		client, err := redisFactory.CreateClient(cfg.Redis)
		if err != nil {
			logger.Warn("Failed to create Redis client, falling back to memory store",
				"name", name,
				"error", err,
			)
			return memory.NewStore(storage.DefaultConfig()), nil
		}

		logger.Info("Creating Redis limiter store",
			"name", name,
			"host", cfg.Redis.Host,
			"port", cfg.Redis.Port,
		)
		return redis.NewStore(client, storage.DefaultConfig()), nil

	default:
		return nil, errors.New("unknown storage type: " + cfg.Type)
	}
}

// CreateRateLimitKeyFunc creates a key function based on configuration
func CreateRateLimitKeyFunc(keyType string) ratelimit.KeyFunc {
	switch keyType {
	case "path":
		return ratelimit.ByPath
	case "ip_path":
		return ratelimit.ByIPAndPath
	default:
		return ratelimit.ByIP
	}
}
