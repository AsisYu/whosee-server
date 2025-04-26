/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-10 16:12:00
 * @Description: u57fau4e8eRedisu7684u5206u5e03u5f0fu9650u6d41u5668
 */
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RateLimiter u5b9eu73b0u5206u5e03u5f0fu9650u6d41u5668
type RateLimiter struct {
	rdb       *redis.Client
	keyPrefix string
	limit     int           // u65f6u95f4u7a97u53e3u5185u5141u8bb8u7684u6700u5927u8bf7u6c42u6570
	window    time.Duration // u65f6u95f4u7a97u53e3
}

// NewRateLimiter u521bu5efau65b0u7684u9650u6d41u5668
func NewRateLimiter(rdb *redis.Client, keyPrefix string, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		limit:     limit,
		window:    window,
	}
}

// Allow u68c0u67e5u662fu5426u5141u8bb8u8bf7u6c42u901au8fc7
func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	redisKey := fmt.Sprintf("%s:%s", rl.keyPrefix, key)
	
	// u4f7fu7528Redisu6ed1u52a8u7a97u53e3u5b9eu73b0u9650u6d41
	now := time.Now().UnixNano()
	windowStart := now - int64(rl.window)
	
	// u4e8bu52a1u6267u884c
	pipe := rl.rdb.Pipeline()
	
	// u79fbu9664u65f6u95f4u7a97u53e3u4e4bu5916u7684u8bf7u6c42u8bb0u5f55
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	
	// u6dfbu52a0u5f53u524du8bf7u6c42u8bb0u5f55
	pipe.ZAdd(ctx, redisKey, &redis.Z{Score: float64(now), Member: now})
	
	// u83b7u53d6u5f53u524du7a97u53e3u5185u7684u8bf7u6c42u6570
	countCmd := pipe.ZCard(ctx, redisKey)
	
	// u8bbeu7f6ekeyu8fc7u671fu65f6u95f4u4e3au7a97u53e3u65f6u95f4u7684u4e24u500duff0cu907fu514du957fu671fu5360u7528u5185u5b58
	pipe.Expire(ctx, redisKey, rl.window*2)
	
	// u6267u884cu4e8bu52a1
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	
	// u68c0u67e5u662fu5426u8d85u51fau9650u5236
	count, err := countCmd.Result()
	if err != nil {
		return false, err
	}
	
	return count <= int64(rl.limit), nil
}

// GetCurrentCount u83b7u53d6u5f53u524du8ba1u6570
func (rl *RateLimiter) GetCurrentCount(ctx context.Context, key string) (int64, error) {
	redisKey := fmt.Sprintf("%s:%s", rl.keyPrefix, key)
	
	// u4f7fu7528Redisu6ed1u52a8u7a97u53e3u5b9eu73b0u9650u6d41
	now := time.Now().UnixNano()
	windowStart := now - int64(rl.window)
	
	// u4e8bu52a1u6267u884c
	pipe := rl.rdb.Pipeline()
	
	// u79fbu9664u65f6u95f4u7a97u53e3u4e4bu5916u7684u8bf7u6c42u8bb0u5f55
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	
	// u83b7u53d6u5f53u524du7a97u53e3u5185u7684u8bf7u6c42u6570
	countCmd := pipe.ZCard(ctx, redisKey)
	
	// u6267u884cu4e8bu52a1
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	
	return countCmd.Result()
}
