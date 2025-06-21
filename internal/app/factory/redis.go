package factory

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"gateway/internal/config"
	"gateway/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// CreateRedisClient creates a Redis client from configuration
func CreateRedisClient(cfg *config.Redis, logger *slog.Logger) (*redis.Client, error) {
	if cfg == nil {
		return nil, errors.NewError(errors.ErrorTypeInternal, "Redis configuration is nil")
	}

	// Default values
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 6379
	}
	if cfg.MaxActive == 0 {
		cfg.MaxActive = 100
	}
	if cfg.MaxIdle == 0 {
		cfg.MaxIdle = 10
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 10
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 5
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 5
	}

	// Create options
	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,

		// Pool settings
		PoolSize:     cfg.MaxActive,
		MinIdleConns: cfg.MaxIdle,

		// Timeouts
		DialTimeout:  time.Duration(cfg.ConnectTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	}

	// Configure idle timeout
	if cfg.IdleTimeout > 0 {
		opts.ConnMaxIdleTime = time.Duration(cfg.IdleTimeout) * time.Second
	}

	// Configure TLS if enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		}

		// Load client certificates if provided
		if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
			if err != nil {
				return nil, errors.NewError(errors.ErrorTypeInternal, "failed to load Redis client certificate").WithCause(err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		opts.TLSConfig = tlsConfig
	}

	// Create client
	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ConnectTimeout)*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "failed to connect to Redis").WithCause(err)
	}

	logger.Info("Connected to Redis",
		"addr", opts.Addr,
		"db", cfg.DB,
		"poolSize", cfg.MaxActive,
	)

	return client, nil
}

// CreateRedisClusterClient creates a Redis cluster client from configuration
func CreateRedisClusterClient(cfg *config.Redis, logger *slog.Logger) (*redis.ClusterClient, error) {
	if cfg == nil || !cfg.Cluster {
		return nil, errors.NewError(errors.ErrorTypeInternal, "Redis cluster configuration is invalid")
	}

	if len(cfg.ClusterNodes) == 0 {
		return nil, errors.NewError(errors.ErrorTypeInternal, "No cluster nodes specified")
	}

	// Create cluster options
	opts := &redis.ClusterOptions{
		Addrs:    cfg.ClusterNodes,
		Password: cfg.Password,

		// Pool settings
		PoolSize:     cfg.MaxActive,
		MinIdleConns: cfg.MaxIdle,

		// Timeouts
		DialTimeout:  time.Duration(cfg.ConnectTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	}

	// Configure idle timeout
	if cfg.IdleTimeout > 0 {
		opts.ConnMaxIdleTime = time.Duration(cfg.IdleTimeout) * time.Second
	}

	// Configure TLS if enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		}

		// Load client certificates if provided
		if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
			if err != nil {
				return nil, errors.NewError(errors.ErrorTypeInternal, "failed to load Redis client certificate").WithCause(err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		opts.TLSConfig = tlsConfig
	}

	// Create cluster client
	client := redis.NewClusterClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ConnectTimeout)*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "failed to connect to Redis cluster").WithCause(err)
	}

	logger.Info("Connected to Redis cluster",
		"nodes", cfg.ClusterNodes,
		"poolSize", cfg.MaxActive,
	)

	return client, nil
}

// CreateRedisSentinelClient creates a Redis sentinel client from configuration
func CreateRedisSentinelClient(cfg *config.Redis, logger *slog.Logger) (*redis.Client, error) {
	if cfg == nil || !cfg.Sentinel {
		return nil, errors.NewError(errors.ErrorTypeInternal, "Redis sentinel configuration is invalid")
	}

	if len(cfg.SentinelNodes) == 0 {
		return nil, errors.NewError(errors.ErrorTypeInternal, "No sentinel nodes specified")
	}

	if cfg.MasterName == "" {
		return nil, errors.NewError(errors.ErrorTypeInternal, "Sentinel master name not specified")
	}

	// Create sentinel options
	opts := &redis.FailoverOptions{
		MasterName:    cfg.MasterName,
		SentinelAddrs: cfg.SentinelNodes,
		Password:      cfg.Password,
		DB:            cfg.DB,

		// Pool settings
		PoolSize:     cfg.MaxActive,
		MinIdleConns: cfg.MaxIdle,

		// Timeouts
		DialTimeout:  time.Duration(cfg.ConnectTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	}

	// Configure idle timeout
	if cfg.IdleTimeout > 0 {
		opts.ConnMaxIdleTime = time.Duration(cfg.IdleTimeout) * time.Second
	}

	// Configure TLS if enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		}

		// Load client certificates if provided
		if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
			if err != nil {
				return nil, errors.NewError(errors.ErrorTypeInternal, "failed to load Redis client certificate").WithCause(err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		opts.TLSConfig = tlsConfig
	}

	// Create sentinel client
	client := redis.NewFailoverClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ConnectTimeout)*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "failed to connect to Redis sentinel").WithCause(err)
	}

	logger.Info("Connected to Redis sentinel",
		"master", cfg.MasterName,
		"sentinels", cfg.SentinelNodes,
		"poolSize", cfg.MaxActive,
	)

	return client, nil
}

// CreateRedisLimiterFromConfig creates appropriate Redis client and limiter
func CreateRedisLimiterFromConfig(cfg *config.Redis, limit int, burst int, logger *slog.Logger) (interface{}, error) {
	if cfg == nil {
		return nil, nil // No Redis configured, will use in-memory
	}

	var client interface{}
	var err error

	// Create appropriate client based on configuration
	if cfg.Cluster {
		client, err = CreateRedisClusterClient(cfg, logger)
	} else if cfg.Sentinel {
		client, err = CreateRedisSentinelClient(cfg, logger)
	} else {
		client, err = CreateRedisClient(cfg, logger)
	}

	if err != nil {
		logger.Warn("Failed to create Redis client, will use in-memory rate limiting",
			"error", err,
		)
		return nil, nil
	}

	return client, nil
}
