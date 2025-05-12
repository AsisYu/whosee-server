/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-09 12:15:00
 * @Description: IP白名单和API Key验证中间件 - 增强版
 */
package middleware

import (
	"context"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// IPWhitelistConfig 白名单中间件配置
type IPWhitelistConfig struct {
	APIKey         string        // API密钥
	APIDevMode     bool          // 开发模式标志
	TrustedIPs     string        // 信任的IP列表（逗号分隔）
	TrustedIPsList []string      // 预定义的信任IP列表
	RedisClient    *redis.Client // Redis客户端用于缓存
	StrictMode     bool          // 严格模式 - 如果为true，则要求同时满足IP白名单和API密钥
	CacheExpiration time.Duration // 缓存过期时间
}

// 默认可信IP白名单
var defaultTrustedIPs = []string{
	"127.0.0.1",     // 本地回环地址
	"::1",           // IPv6本地回环地址
	"10.0.0.0/8",    // 私有网络
	"172.16.0.0/12", // 私有网络
	"192.168.0.0/16", // 私有网络
}

// IsWhitelistedIP 检查IP是否在白名单中，支持配置选项
func IsWhitelistedIP(ip string, config IPWhitelistConfig) bool {
	// 开发模式下跳过白名单检查
	if config.APIDevMode {
		return true
	}

	// 检查环境变量中是否有自定义白名单配置
	if config.TrustedIPs != "" {
		for _, trustedIP := range strings.Split(config.TrustedIPs, ",") {
			if strings.TrimSpace(trustedIP) == ip {
				return true
			}
		}
	}

	// 检查预定义的信任IP列表
	for _, trustedIP := range config.TrustedIPsList {
		if trustedIP == ip {
			return true
		}
	}

	// 检查默认白名单
	for _, trustedIP := range defaultTrustedIPs {
		if ip == trustedIP {
			return true
		}
		
		// 检查CIDR范围
		if strings.Contains(trustedIP, "/") {
			_, ipNet, err := net.ParseCIDR(trustedIP)
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

// HasValidKey 检查请求是否包含有效的API Key
func HasValidKey(c *gin.Context, apiKey string) bool {
	providedKey := c.GetHeader("X-API-KEY")
	if providedKey == "" {
		providedKey = c.Query("apikey")
	}
	
	return providedKey != "" && providedKey == apiKey
}

// IPWhitelistMiddleware 创建基本的IP白名单中间件
func IPWhitelistMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		
		// 从环境变量读取API Key
		apiKey := os.Getenv("API_KEY")
		
		// 检查IP白名单或API密钥
		if !IsWhitelistedIP(ip, IPWhitelistConfig{TrustedIPs: os.Getenv("TRUSTED_IPS")}) && 
		   !HasValidKey(c, apiKey) {
			c.JSON(403, gin.H{
				"error":   "ACCESS_DENIED",
				"message": "您没有访问此API的权限",
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// IPWhitelistWithConfig 创建带配置的高级IP白名单中间件
func IPWhitelistWithConfig(config IPWhitelistConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		
		// 如果有Redis客户端，尝试从缓存中获取结果
		if config.RedisClient != nil {
			cacheKey := "ip:whitelist:" + ip
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			
			// 尝试从缓存获取结果
			cachedResult, err := config.RedisClient.Get(ctx, cacheKey).Result()
			if err == nil {
				// 缓存命中，直接使用缓存结果
				if cachedResult == "allowed" {
					c.Next()
					return
				} else if cachedResult == "denied" {
					if HasValidKey(c, config.APIKey) {
						if !config.StrictMode {
							c.Next()
							return
						}
					}
					c.JSON(403, gin.H{
						"error":   "ACCESS_DENIED",
						"message": "您没有访问此API的权限",
					})
					c.Abort()
					return
				}
			}
		}
		
		// 检查IP白名单
		ipAllowed := IsWhitelistedIP(ip, config)
		
		// 检查API密钥
		keyValid := HasValidKey(c, config.APIKey)
		
		// 根据严格模式决定是否允许访问
		if config.StrictMode {
			// 严格模式：必须同时通过IP和API密钥验证
			allowed := ipAllowed && keyValid
			if !allowed {
				log.Printf("[安全] 访问被拒绝，IP: %s，严格模式下IP白名单和API密钥验证失败", ip)
				c.JSON(403, gin.H{
					"error":   "ACCESS_DENIED",
					"message": "您没有访问此API的权限",
				})
				c.Abort()
				
				// 缓存结果
				if config.RedisClient != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
					defer cancel()
					config.RedisClient.Set(ctx, "ip:whitelist:"+ip, "denied", config.CacheExpiration)
				}
				return
			}
		} else {
			// 非严格模式：只要通过IP白名单或API密钥验证之一即可
			if !ipAllowed && !keyValid {
				log.Printf("[安全] 访问被拒绝，IP: %s，IP白名单和API密钥验证均失败", ip)
				c.JSON(403, gin.H{
					"error":   "ACCESS_DENIED",
					"message": "您没有访问此API的权限",
				})
				c.Abort()
				
				// 缓存结果
				if config.RedisClient != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
					defer cancel()
					config.RedisClient.Set(ctx, "ip:whitelist:"+ip, "denied", config.CacheExpiration)
				}
				return
			}
		}
		
		// 请求被允许
		if config.RedisClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			config.RedisClient.Set(ctx, "ip:whitelist:"+ip, "allowed", config.CacheExpiration)
		}
		
		c.Next()
	}
}
