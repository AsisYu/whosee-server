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
	"log"
	"strings"
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

// 健康检查API的专用限流中间件
func HealthCheckRateLimit(rdb *redis.Client) gin.HandlerFunc {
	// 创建内存中的限流器，针对详细健康检查
	detailedHealthLimiter := NewIPRateLimiter(rate.Limit(1.0/60.0), 1) // 每IP每分钟1次
	
	// 允许的前端域名列表，与CORS中间件保持一致
	allowedOrigins := map[string]bool{
		"http://localhost:8080":           true, // Vue开发环境
		"http://localhost:3000":           true, // 开发环境
		"http://localhost:5173":           true, // SvelteKit开发环境
		"https://domain-whois.vercel.app": true, // 生产环境
		"https://whosee.me":               true, // 域名
	}
	
	// 最后一次详细健康检查的缓存（全局共享）
	var (
		lastDetailedCheck      map[string]interface{}
		lastDetailedCheckTime  time.Time
		lastDetailedCheckMutex sync.RWMutex
	)
	
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/health") {
			c.Next()
			return
		}
		
		// 检查是否为详细健康检查请求
		isDetailedCheck := c.Query("detailed") == "true"
		clientIP := c.ClientIP()
		
		// 基本健康检查的限流（不那么严格）
		if !isDetailedCheck {
			key := fmt.Sprintf("rate:health:%s", clientIP)
			count, _ := rdb.Incr(c, key).Result()
			rdb.Expire(c, key, time.Minute)
			
			if count > 10 { // 每分钟限制10次基本健康检查
				c.AbortWithStatusJSON(429, gin.H{"error": "健康检查请求过于频繁"})
				return
			}
			c.Next()
			return
		}
		
		// 详细健康检查的安全措施
		
		// 1. 验证请求来源（仅允许前端应用访问）
		origin := c.Request.Header.Get("Origin")
		
		// 检查是否来自允许的前端域名
		isAllowedOrigin := allowedOrigins[origin]
		
		// 特殊情况：来自localhost的直接API调用或内部请求
		isLocalRequest := clientIP == "127.0.0.1" || clientIP == "::1"
		
		// 开发环境：直接允许本地请求访问详细健康检查
		if isLocalRequest {
			log.Printf("本地开发环境请求 (IP: %s) - 允许访问详细健康检查", clientIP)
			c.Next()
			return
		}
		
		if !isAllowedOrigin && !isLocalRequest {
			log.Printf("来自未授权来源的详细健康检查尝试 (Origin: %s, IP: %s)", origin, clientIP)
			c.AbortWithStatusJSON(403, gin.H{"error": "只有授权的前端应用可以执行详细健康检查"})
			return
		}
		
		// 2. 验证API密钥（仅针对非前端请求，例如直接API调用）
		if !isAllowedOrigin && !isLocalRequest {
			apiKey := c.GetHeader("X-API-Key")
			expectedAPIKey := "your-health-check-api-key" // 从环境变量或配置获取
			
			if expectedAPIKey != "" && apiKey != expectedAPIKey {
				log.Printf("来自IP %s 的未授权详细健康检查尝试（缺少有效的API密钥）", clientIP)
				c.AbortWithStatusJSON(401, gin.H{"error": "没有权限执行详细健康检查"})
				return
			}
		}
		
		// 3. 强力限流：使用内存限流器，每IP每分钟一次
		if !detailedHealthLimiter.Allow(clientIP) {
			log.Printf("来自IP %s 的详细健康检查请求因频率限制被拒绝", clientIP)
			c.AbortWithStatusJSON(429, gin.H{"error": "详细健康检查请求过于频繁，请至少等待1分钟"})
			return
		}
		
		// 4. 结果缓存：检查是否有足够新的缓存可用
		lastDetailedCheckMutex.RLock()
		cacheAge := time.Since(lastDetailedCheckTime)
		hasValidCache := !lastDetailedCheckTime.IsZero() && cacheAge < 5*time.Minute && lastDetailedCheck != nil
		cachedResult := lastDetailedCheck
		lastDetailedCheckMutex.RUnlock()
		
		if hasValidCache {
			// 返回缓存的结果，避免重复执行详细检查
			log.Printf("为请求 (Origin: %s, IP: %s) 返回缓存的详细健康检查结果（缓存时间: %v）", origin, clientIP, cacheAge)
			c.Set("useHealthCheckCache", true)
			c.Set("cachedHealthCheckResult", cachedResult)
		} else {
			// 标记此请求需要执行新的详细健康检查
			c.Set("needNewHealthCheck", true)
			
			// 创建一个钩子来保存新的健康检查结果到缓存
			c.Set("saveHealthCheckCache", func(result map[string]interface{}) {
				lastDetailedCheckMutex.Lock()
				defer lastDetailedCheckMutex.Unlock()
				lastDetailedCheck = result
				lastDetailedCheckTime = time.Now()
				log.Printf("已更新详细健康检查缓存，由请求 (Origin: %s, IP: %s) 触发", origin, clientIP)
			})
		}
		
		c.Next()
	}
}
