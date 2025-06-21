package factory

import (
	"gateway/internal/config"
	"gateway/internal/storage/redis"
	goredis "github.com/redis/go-redis/v9"
)

// RedisClientFactoryAdapter adapts our Redis factory to the storage interface
type RedisClientFactoryAdapter struct{}

// CreateClient creates a Redis client and wraps it with our interface
func (f *RedisClientFactoryAdapter) CreateClient(cfg *config.Redis) (redis.Client, error) {
	// Create the actual Redis client
	var universalClient goredis.UniversalClient
	var err error

	if cfg.Cluster {
		clusterClient, clusterErr := CreateRedisClusterClient(cfg, nil)
		if clusterErr != nil {
			return nil, clusterErr
		}
		universalClient = clusterClient
		err = clusterErr
	} else if cfg.Sentinel {
		sentinelClient, sentinelErr := CreateRedisSentinelClient(cfg, nil)
		if sentinelErr != nil {
			return nil, sentinelErr
		}
		universalClient = sentinelClient
		err = sentinelErr
	} else {
		standardClient, standardErr := CreateRedisClient(cfg, nil)
		if standardErr != nil {
			return nil, standardErr
		}
		universalClient = standardClient
		err = standardErr
	}

	if err != nil {
		return nil, err
	}

	// Wrap with our adapter
	return redis.NewClientAdapter(universalClient), nil
}
