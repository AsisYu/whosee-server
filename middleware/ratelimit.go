/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-17 23:34:52
 * @LastEditors: AsisYu 2773943729@qq.com
 * @LastEditTime: 2025-01-17 23:37:08
 * @FilePath: \dmainwhoseek\server\middleware\ratelimit.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}
}

// 获取特定 IP 的限流器
func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

// Allow 检查是否允许请求
func (i *IPRateLimiter) Allow(ip string) bool {
	return i.getLimiter(ip).Allow()
}

func RateLimit(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rate:ip:%s", c.ClientIP())
		count, _ := rdb.Incr(c, key).Result()
		// 使用管道优化Redis操作
		pipe := rdb.Pipeline()
		pipe.Expire(c, key, time.Minute)
		// 添加IP黑名单检查
		if count > 100 { // 1分钟内超过100次
			pipe.Set(c, fmt.Sprintf("blacklist:%s", c.ClientIP()), true, time.Hour)
		}
		pipe.Exec(c)

		if count > 60 { // 每分钟60次请求限制
			c.AbortWithStatusJSON(429, gin.H{"error": "请求过于频繁"})
			return
		}
		c.Next()
	}
}
