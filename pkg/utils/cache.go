package utils

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

// ErrCacheMiss is returned when a key does not exist in cache.
var ErrCacheMiss = errors.New("cache miss")

// CacheGet decodes the JSON value at key into dest. Returns ErrCacheMiss if not found.
// If Redis is unavailable, returns ErrCacheMiss without an error log.
func CacheGet(ctx context.Context, key string, dest interface{}) error {
	if config.RedisClient == nil {
		return ErrCacheMiss
	}
	raw, err := config.RedisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		return err
	}
	return json.Unmarshal([]byte(raw), dest)
}

// CacheSet stores the value as JSON at key with the given TTL. Silently no-ops without Redis.
func CacheSet(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if config.RedisClient == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return config.RedisClient.Set(ctx, key, data, ttl).Err()
}

// CacheDel removes one or more keys.
func CacheDel(ctx context.Context, keys ...string) {
	if config.RedisClient == nil || len(keys) == 0 {
		return
	}
	config.RedisClient.Del(ctx, keys...)
}

// CacheDelByPrefix scans and deletes all keys matching prefix*.
// Useful for invalidating list caches keyed by query parameters.
func CacheDelByPrefix(ctx context.Context, prefix string) {
	if config.RedisClient == nil || prefix == "" {
		return
	}
	iter := config.RedisClient.Scan(ctx, 0, prefix+"*", 200).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 200 {
			config.RedisClient.Del(ctx, keys...)
			keys = keys[:0]
		}
	}
	if len(keys) > 0 {
		config.RedisClient.Del(ctx, keys...)
	}
}
