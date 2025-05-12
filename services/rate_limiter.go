/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-10 16:12:00
 * @Description: 使用Redis的分布式限流器
 */
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RateLimiter 使用Redis的分布式限流器
type RateLimiter struct {
	rdb       *redis.Client
	keyPrefix string
	limit     int           // 时间窗口内允许的最大请求数
	window    time.Duration // 时间窗口
}

// NewRateLimiter 创建限流器
func NewRateLimiter(rdb *redis.Client, keyPrefix string, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		limit:     limit,
		window:    window,
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	redisKey := fmt.Sprintf("%s:%s", rl.keyPrefix, key)
	
	// 使用Redis的有序集合实现限流
	now := time.Now().UnixNano()
	windowStart := now - int64(rl.window)
	
	// 批量执行Redis命令
	pipe := rl.rdb.Pipeline()
	
	// 删除时间窗口外的请求记录
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	
	// 添加当前请求记录
	pipe.ZAdd(ctx, redisKey, &redis.Z{Score: float64(now), Member: now})
	
	// 获取当前时间窗口内的请求数
	countCmd := pipe.ZCard(ctx, redisKey)
	
	// 设置key的过期时间为时间窗口的两倍
	pipe.Expire(ctx, redisKey, rl.window*2)
	
	// 执行批量命令
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	
	// 检查是否超过限流阈值
	count, err := countCmd.Result()
	if err != nil {
		return false, err
	}
	
	return count <= int64(rl.limit), nil
}

// GetCurrentCount 获取当前请求数
func (rl *RateLimiter) GetCurrentCount(ctx context.Context, key string) (int64, error) {
	redisKey := fmt.Sprintf("%s:%s", rl.keyPrefix, key)
	
	// 使用Redis的有序集合实现限流
	now := time.Now().UnixNano()
	windowStart := now - int64(rl.window)
	
	// 批量执行Redis命令
	pipe := rl.rdb.Pipeline()
	
	// 删除时间窗口外的请求记录
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	
	// 获取当前时间窗口内的请求数
	countCmd := pipe.ZCard(ctx, redisKey)
	
	// 执行批量命令
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	
	return countCmd.Result()
}
