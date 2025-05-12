/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-03-31 04:10:00
 * @Description: 限流中间件
 */
package middleware

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/time/rate"
)

// IPRateLimiter 内存中的IP限流器
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

// RateLimitConfig 限流器配置
type RateLimitConfig struct {
	RedisClient     *redis.Client // Redis客户端，用于分布式限流
	Key             string        // 限流器键
	Rate            int           // 允许的请求速率
	Period          time.Duration // 限流周期
	Burst           int           // 突发请求允许的数量
	IPLookup        []string      // IP查找方法
	ExcludeIPs      []string      // 排除的IP列表
	CacheExpiration time.Duration // 缓存过期时间
	StatusCode      int           // 超限状态码
	Message         string        // 超限消息
	UseMemory       bool          // 是否使用内存限流
}

// DefaultRateLimitConfig 默认限流器配置
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Key:             "limit:global",
		Rate:            60,
		Period:          time.Minute,
		Burst:           10,
		IPLookup:        []string{"X-Forwarded-For", "X-Real-IP", "RemoteAddr"},
		ExcludeIPs:      []string{"127.0.0.1", "::1"},
		CacheExpiration: 3 * time.Minute,
		StatusCode:      429,
		Message:         "请求过于频繁，请稍后再试",
		UseMemory:       false,
	}
}

// NewIPRateLimiter 创建一个新的IP限流器
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}
}

// getLimiter 获取特定IP的限流器
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

// getClientIP 获取客户端IP
func getClientIP(c *gin.Context, methods []string) string {
	for _, method := range methods {
		switch method {
		case "X-Forwarded-For":
			forwardedIPs := c.GetHeader("X-Forwarded-For")
			if forwardedIPs != "" {
				// 获取第一个IP
				ip := strings.Split(forwardedIPs, ",")[0]
				return strings.TrimSpace(ip)
			}
		case "X-Real-IP":
			ip := c.GetHeader("X-Real-IP")
			if ip != "" {
				return ip
			}
		case "RemoteAddr":
			ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
			if err == nil {
				return ip
			}
		}
	}

	// 默认使用gin的ClientIP
	return c.ClientIP()
}

// isExcludedIP 检查IP是否在排除列表中
func isExcludedIP(ip string, excludeIPs []string) bool {
	for _, excludeIP := range excludeIPs {
		if ip == excludeIP {
			return true
		}

		// 检查CIDR
		if strings.Contains(excludeIP, "/") {
			_, ipNet, err := net.ParseCIDR(excludeIP)
			if err == nil {
				parsedIP := net.ParseIP(ip)
				if parsedIP != nil && ipNet.Contains(parsedIP) {
					return true
				}
			}
		}
	}
	return false
}

// RateLimit 限流中间件
func RateLimit() gin.HandlerFunc {
	return RateLimitWithConfig(DefaultRateLimitConfig())
}

// RateLimitWithConfig 限流中间件（可配置）
func RateLimitWithConfig(config RateLimitConfig) gin.HandlerFunc {
	// 如果使用内存限流，创建一个内存限流器
	var ipLimiter *IPRateLimiter
	if config.UseMemory {
		ipLimiter = NewIPRateLimiter(rate.Limit(float64(config.Rate)/config.Period.Seconds()), config.Burst)
	}

	return func(c *gin.Context) {
		// 获取客户端IP
		ip := getClientIP(c, config.IPLookup)

		// 检查是否排除该IP
		if isExcludedIP(ip, config.ExcludeIPs) {
			c.Next()
			return
		}

		// 构造限流器标识符
		identifier := fmt.Sprintf("%s:%s:%s", config.Key, ip, c.Request.URL.Path)

		// 检查是否允许请求
		allowed := false

		if config.UseMemory {
			// 使用内存限流器
			allowed = ipLimiter.Allow(identifier)
		} else if config.RedisClient != nil {
			// 使用Redis限流
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			// 使用滑动窗口算法
			periodSeconds := int(config.Period.Seconds())
			currentTime := time.Now().Unix()
			windowKey := fmt.Sprintf("%s:%d", identifier, currentTime/int64(periodSeconds))

			// 获取当前窗口的请求次数
			exists, err := config.RedisClient.Exists(ctx, windowKey).Result()
			if err != nil {
				// Redis错误，允许请求
				log.Printf("[限流] Redis错误，允许请求: %s", err)
				allowed = true
			} else {
				if exists == 0 {
					// 新窗口，设置请求次数为1
					_, err = config.RedisClient.SetEX(ctx, windowKey, 1, config.CacheExpiration).Result()
					allowed = true
				} else {
					// 获取当前窗口的请求次数
					count, err := config.RedisClient.Incr(ctx, windowKey).Result()
					if err != nil {
						log.Printf("[限流] Redis错误，允许请求: %s", err)
						allowed = true
					} else {
						// 检查是否允许请求
						if count <= int64(config.Rate)+int64(config.Burst) {
							allowed = true
						}
					}
				}
			}
		} else {
			// 没有配置限流器，允许所有请求
			allowed = true
		}

		// 处理限流结果
		if !allowed {
			// 设置限流相关头部
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", config.Rate))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(config.Period).Unix()))
			c.Header("Retry-After", fmt.Sprintf("%d", int(config.Period.Seconds())))

			// 返回429状态码
			c.JSON(config.StatusCode, gin.H{
				"error":   "TOO_MANY_REQUESTS",
				"message": config.Message,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// HealthCheckRateLimit 健康检查限流中间件
func HealthCheckRateLimit() gin.HandlerFunc {
	// 配置限流器
	config := DefaultRateLimitConfig()
	config.Key = "limit:health"
	config.Rate = 300 // 每分钟300次
	config.Message = "健康检查请求过于频繁"
	config.UseMemory = true // 使用内存限流

	return RateLimitWithConfig(config)
}
