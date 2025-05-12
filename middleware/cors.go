/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-28 10:18:00
 * @Description: 跨域请求的CORS配置内部实现
 */
package middleware

import (
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSConfig 跨域请求的配置信息
type CORSConfig struct {
	AllowOrigins     []string      // 允许的来源域名
	AllowMethods     []string      // 允许的HTTP方法
	AllowHeaders     []string      // 允许的请求头
	ExposeHeaders    []string      // 暴露给客户端的头信息
	AllowCredentials bool          // 是否允许发送Cookie
	MaxAge           time.Duration // 预检请求的缓存时间
	AllowAllOrigins  bool          // 是否允许所有来源域名
	UseWhitelist     bool          // 是否使用白名单
}

// DefaultCORSConfig 默认的CORS配置
func DefaultCORSConfig() CORSConfig {
	// 从环境变量中获取允许的来源域名
	allowOrigins := []string{"localhost", "127.0.0.1", "https://whosee.me"}
	if envOrigins := os.Getenv("CORS_ORIGINS"); envOrigins != "" {
		allowOrigins = strings.Split(envOrigins, ",")
		for i := range allowOrigins {
			allowOrigins[i] = strings.TrimSpace(allowOrigins[i])
		}
	}

	return CORSConfig{
		AllowOrigins: allowOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With",
			"X-API-KEY", "Access-Control-Request-Method", "Access-Control-Request-Headers",
		},
		ExposeHeaders:    []string{"Content-Length", "Content-Type", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowAllOrigins:  false,
		UseWhitelist:     true,
	}
}

// CORS 跨域请求的中间件
func CORS() gin.HandlerFunc {
	return CORSWithConfig(DefaultCORSConfig())
}

// CORSWithConfig 根据配置生成CORS中间件
func CORSWithConfig(config CORSConfig) gin.HandlerFunc {
	// 初始化允许的来源域名列表
	var allowOrigins []string
	for _, origin := range config.AllowOrigins {
		allowOrigins = append(allowOrigins, strings.ToLower(origin))
	}

	// 检查是否有通配符
	wildcard := false
	for _, origin := range allowOrigins {
		if origin == "*" {
			wildcard = true
			break
		}
	}

	return func(c *gin.Context) {
		// 获取请求来源
		origin := c.Request.Header.Get("Origin")

		// 如果不是CORS请求，直接跳过
		if origin == "" {
			c.Next()
			return
		}

		// 检查来源是否在允许列表中
		allowed := false

		// 如果允许所有来源或使用通配符，直接允许
		if config.AllowAllOrigins || wildcard {
			allowed = true
		} else {
			// 检查来源是否在白名单中
			origin = strings.ToLower(origin)
			for _, allowedOrigin := range allowOrigins {
				if allowedOrigin == origin {
					allowed = true
					break
				}

				// 支持通配符，如*.example.com
				if strings.HasPrefix(allowedOrigin, "*.") {
					domainSuffix := allowedOrigin[1:] // 去掉*
					if strings.HasSuffix(origin, domainSuffix) {
						allowed = true
						break
					}
				}
			}
		}

		// 如果不允许来源且使用白名单，直接跳过
		if !allowed && config.UseWhitelist {
			c.Next()
			return
		}

		// 设置CORS头信息
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
		c.Header("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
		c.Header("Access-Control-Max-Age", string(int(config.MaxAge.Seconds())))

		// 设置Credentials
		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// 处理OPTIONS预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
