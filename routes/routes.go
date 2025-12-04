/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: API路由注册
 */
package routes

import (
	"context"
	"log"
	"os"
	"time"

	"whosee/handlers"
	"whosee/middleware"
	"whosee/services"
	"whosee/utils"

	"github.com/gin-gonic/gin"
)

// 域名验证中间件，提取到单独函数减少重复代码
func domainValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取域名参数
		var domain string
		// 优先从路径参数获取，其次从查询参数获取
		if d := c.Param("domain"); d != "" {
			domain = d
		} else {
			domain = c.Query("domain")
		}

		if domain == "" {
			utils.ErrorResponse(c, 400, "MISSING_PARAMETER", "Domain parameter is required")
			c.Abort()
			return
		}

		// 验证域名格式
		if !utils.IsValidDomain(domain) {
			log.Printf("查询失败: 无效的域名格式: %s", domain)
			utils.ErrorResponse(c, 400, "INVALID_DOMAIN", "Invalid domain format")
			c.Abort()
			return
		}

		// 将验证通过的域名存储在上下文中
		c.Set("domain", domain)
		c.Next()
	}
}

// 限流中间件
func rateLimitMiddleware(limiter *services.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		domain, _ := c.Get("domain")
		domainStr, _ := domain.(string)

		// 创建一个10秒超时的上下文
		reqCtx, cancel := c.Request.Context(), func() {}

		// 检查限流
		allowed, err := limiter.Allow(reqCtx, c.ClientIP())
		if err != nil {
			log.Printf("限流检查失败: %v, IP: %s, 域名: %s", err, c.ClientIP(), domainStr)
			// 继续执行，避免因限流器故障影响服务
		} else if !allowed {
			log.Printf("请求被限流, IP: %s, 域名: %s", c.ClientIP(), domainStr)
			utils.ErrorResponse(c, 429, "RATE_LIMITED", "Too many requests, please try again later")
			c.Abort()
			return
		}

		c.Next()
		cancel() // 释放上下文
	}
}

// 异步工作任务中间件
func asyncWorkerMiddleware(workerPool *services.WorkerPool, timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 已经在上游中间件中设置的域名
		domain, _ := c.Get("domain")
		domainStr, _ := domain.(string)

		// 创建一个带超时的上下文
		reqCtx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		c.Set("requestContext", reqCtx)
		c.Set("cancelFunc", cancel)

		// 创建结果和错误通道，用于后续请求处理
		resultChan := make(chan interface{}, 1)
		errorChan := make(chan error, 1)

		c.Set("resultChan", resultChan)
		c.Set("errorChan", errorChan)
		c.Set("workerPool", workerPool)

		// 标记通道是否已经使用，避免重复关闭
		c.Set("channelUsed", false)

		// 继续处理请求
		c.Next()

		// 检查通道是否在处理函数中使用
		channelUsed, exists := c.Get("channelUsed")

		// 如果通道未被使用或明确设置为未使用，则关闭并清理资源
		if !exists || channelUsed == false {
			cancel()
			close(resultChan)
			close(errorChan)
			log.Printf("[Worker] 清理异步资源，域名: %s", domainStr)
		} else {
			// 通道已被使用，由worker pool中的goroutine负责关闭
			log.Printf("[Worker] 异步任务已提交，域名: %s，资源将在任务完成后清理", domainStr)
		}
	}
}

// RegisterAPIRoutes 注册所有API路由
func RegisterAPIRoutes(r *gin.Engine, serviceContainer *services.ServiceContainer) {
	// 确保限流器已初始化
	if serviceContainer.Limiter == nil {
		serviceContainer.InitializeLimiter("limit:api", 60, time.Minute)
	}

	apiLimiter := serviceContainer.Limiter
	apiv1 := r.Group("/api/v1")

	// 健康检查路由
	r.GET("/api/health", handlers.HealthCheckHandler(serviceContainer.HealthChecker))

	// 认证令牌路由 - 用于客户端获取JWT令牌
	r.POST("/api/auth/token", middleware.GenerateToken(serviceContainer.RedisClient))

	// 应用安全中间件
	if os.Getenv("DISABLE_API_SECURITY") != "true" {
		// 配置IP白名单中间件
		// StrictMode: 如果为true，需要同时满足IP白名单和API密钥验证
		// 如果为false，只需要满足其中一个条件即可（推荐生产环境使用false）
		strictMode := os.Getenv("IP_WHITELIST_STRICT_MODE") == "true"
		config := middleware.IPWhitelistConfig{
			APIKey:          os.Getenv("API_KEY"),
			APIDevMode:      os.Getenv("API_DEV_MODE") == "true",
			TrustedIPs:      os.Getenv("TRUSTED_IPS"),
			RedisClient:     serviceContainer.RedisClient,
			StrictMode:      strictMode,
			TrustedIPsList:  []string{"127.0.0.1", "::1"},
			CacheExpiration: 5 * time.Minute,
		}

		// 应用IP白名单中间件
		apiv1.Use(middleware.IPWhitelistWithConfig(config))

		// 应用JWT认证中间件 - 确保所有API调用都需要有效的JWT令牌
		apiv1.Use(middleware.AuthRequired(serviceContainer.RedisClient))

		// 应用CORS中间件
		corsConfig := middleware.DefaultCORSConfig()
		r.Use(middleware.CORSWithConfig(corsConfig))

		// 应用安全头部中间件
		securityConfig := middleware.DefaultSecurityConfig()
		r.Use(middleware.SecurityWithConfig(securityConfig))

		// 应用限流中间件
		rateLimitConfig := middleware.DefaultRateLimitConfig()
		rateLimitConfig.RedisClient = serviceContainer.RedisClient
		r.Use(middleware.RateLimitWithConfig(rateLimitConfig))
	} else {
		log.Printf("[警告] API安全限制已禁用! 任何人都可以访问API，这在生产环境中不安全")
	}

	// 添加请求大小限制，对不同类型请求启用不同的限制
	// 使用可配置的SizeLimitWithConfig中间件，允许10MB的大请求体
	apiv1.Use(middleware.SizeLimitWithConfig(middleware.SizeLimitConfig{
		Limit:      10 * 1024 * 1024, // 限制请求体大小为10MB
		Message:    "请求体超过最大限制",
		StatusCode: 413,
		// 添加跳过路径函数，排除某些路径的大小限制检查
		SkipPathFunc: func(path string) bool {
			return false // 默认不跳过任何路径的大小限制检查
		},
		ExcludedMethods: []string{"GET", "DELETE", "OPTIONS", "HEAD"}, // 排除某些方法的大小限制检查
	}))

	// WHOIS查询路由
	whoisGroup := apiv1.Group("/whois")
	whoisGroup.Use(domainValidationMiddleware())
	whoisGroup.Use(rateLimitMiddleware(apiLimiter))
	whoisGroup.Use(asyncWorkerMiddleware(serviceContainer.WorkerPool, 15*time.Second))
	whoisGroup.GET("", handlers.WhoisHandler)
	whoisGroup.GET("/:domain", handlers.WhoisHandler)

	// WHOIS提供商比较路由
	whoisCompareGroup := apiv1.Group("/whois/compare")
	whoisCompareGroup.Use(domainValidationMiddleware())
	whoisCompareGroup.Use(rateLimitMiddleware(apiLimiter))
	whoisCompareGroup.GET("/:domain", handlers.WhoisComparisonHandler)

	// WHOIS提供商信息路由
	apiv1.GET("/whois/providers", handlers.WhoisProvidersInfoHandler)

	// RDAP查询路由
	rdapGroup := apiv1.Group("/rdap")
	rdapGroup.Use(domainValidationMiddleware())
	rdapGroup.Use(rateLimitMiddleware(apiLimiter))
	rdapGroup.Use(asyncWorkerMiddleware(serviceContainer.WorkerPool, 15*time.Second))
	rdapGroup.GET("", handlers.RDAPHandler)
	rdapGroup.GET("/:domain", handlers.RDAPHandler)

	// DNS查询路由
	dnsGroup := apiv1.Group("/dns")
	dnsGroup.Use(domainValidationMiddleware())
	dnsGroup.Use(rateLimitMiddleware(apiLimiter))
	dnsGroup.Use(asyncWorkerMiddleware(serviceContainer.WorkerPool, 10*time.Second))
	dnsGroup.GET("", handlers.DNSHandler)
	dnsGroup.GET("/:domain", handlers.DNSHandler)

	// 🔧 P2-3修复：启用统一截图架构
	// 注册重构后的截图服务路由，包含新的统一API和向后兼容的legacy路由
	// 这将替换下面所有手动定义的截图路由，启用Chrome管理器、熔断器和并发控制
	RegisterScreenshotRoutes(apiv1, serviceContainer)
}
