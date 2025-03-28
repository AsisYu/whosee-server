package handlers

import (
	"context"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/exp/rand"
)

const (
	// 缓存配置
	MaxCacheSize    = 100 * 1024 * 1024 // 100MB 最大缓存大小
	MaxCacheEntries = 10000             // 最大缓存条目数

	// 缓存时间配置
	REGISTERED_DOMAIN_CACHE_TIME   = 30 * 24 * time.Hour // 已注册域名缓存30天
	UNREGISTERED_DOMAIN_CACHE_TIME = 7 * 24 * time.Hour  // 未注册域名缓存7天
	ERROR_DOMAIN_CACHE_TIME        = 24 * time.Hour      // 查询错误的域名缓存1天
	MIN_CACHE_JITTER               = -6 * time.Hour      // 最小抖动时间
	MAX_CACHE_JITTER               = 6 * time.Hour       // 最大抖动时间
)

var rdb *redis.Client

func InitRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	return rdb
}

// 获取带抖动的缓存时间
func getCacheTimeWithJitter(baseTime time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(MAX_CACHE_JITTER-MIN_CACHE_JITTER))) + MIN_CACHE_JITTER
	return baseTime + jitter
}

// 根据域名状态获取缓存时间
func getCacheTime(isRegistered bool, hasError bool) time.Duration {
	var baseTime time.Duration
	if hasError {
		baseTime = ERROR_DOMAIN_CACHE_TIME
	} else if isRegistered {
		baseTime = REGISTERED_DOMAIN_CACHE_TIME
	} else {
		baseTime = UNREGISTERED_DOMAIN_CACHE_TIME
	}
	return getCacheTimeWithJitter(baseTime)
}

// 检查缓存大小
func checkCacheSize(ctx context.Context, rdb *redis.Client) bool {
	_, err := rdb.Info(ctx, "memory").Result()
	if err != nil {
		return false
	}

	// 检查内存使用
	if rdb.DBSize(ctx).Val() > MaxCacheEntries {
		cleanupCache(ctx, rdb)
		return true
	}

	return false
}

// 清理缓存
func cleanupCache(ctx context.Context, rdb *redis.Client) {
	// 使用 SCAN 命令批量处理
	var cursor uint64
	count := int64(100)

	for {
		keys, newCursor, err := rdb.Scan(ctx, cursor, "whois:*", count).Result()
		if err != nil {
			break
		}

		cursor = newCursor

		if len(keys) > 0 {
			// 获取所有键的 TTL
			pipe := rdb.Pipeline()
			ttls := make(map[string]*redis.DurationCmd)
			for _, key := range keys {
				ttls[key] = pipe.TTL(ctx, key)
			}
			_, err := pipe.Exec(ctx)
			if err != nil {
				continue
			}

			// 删除过期时间最短的键
			var keysToDelete []string
			for key, ttlCmd := range ttls {
				ttl := ttlCmd.Val()
				if ttl < 24*time.Hour {
					keysToDelete = append(keysToDelete, key)
				}
			}

			if len(keysToDelete) > 0 {
				rdb.Del(ctx, keysToDelete...)
			}
		}

		if cursor == 0 {
			break
		}
	}
}

// 设置缓存
func setCache(ctx context.Context, rdb *redis.Client, key string, value interface{}, isRegistered bool, hasError bool) error {
	// 检查缓存大小
	if checkCacheSize(ctx, rdb) {
		return nil
	}

	// 获取适当的缓存时间
	cacheTime := getCacheTime(isRegistered, hasError)

	// 设置缓存
	return rdb.Set(ctx, key, value, cacheTime).Err()
}
