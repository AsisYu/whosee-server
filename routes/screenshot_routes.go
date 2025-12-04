/*
 * @Author: AsisYu
 * @Date: 2025-01-20
 * @Description: 重构的截图服务路由配置
 */
package routes

import (
	"time"

	"whosee/handlers"
	"whosee/services"

	"github.com/gin-gonic/gin"
)

// RegisterScreenshotRoutes 注册截图服务路由
func RegisterScreenshotRoutes(r *gin.Engine, serviceContainer *services.ServiceContainer) {
	// 创建截图服务实例
	chromeManager := services.GetGlobalChromeManager()
	screenshotService := services.NewScreenshotService(chromeManager, serviceContainer.RedisClient, nil)
	screenshotHandler := handlers.NewUnifiedScreenshotHandler(screenshotService, chromeManager)

	apiv1 := r.Group("/api/v1")

	// 新的统一截图API (推荐使用)
	screenshotGroup := apiv1.Group("/screenshot")
	{
		// 统一截图接口 - 支持所有截图类型
		screenshotGroup.POST("/", screenshotHandler.TakeScreenshot)
		screenshotGroup.GET("/", screenshotHandler.TakeScreenshot)

		// Chrome管理接口
		screenshotGroup.GET("/chrome/status", handlers.NewChromeStatus)
		screenshotGroup.POST("/chrome/restart", handlers.NewChromeRestart)
	}

	// 兼容旧版API路由 (保持向后兼容)
	compatGroup := apiv1.Group("/")
	{
		// 基础截图兼容路由
		compatGroup.GET("screenshot/:domain", addRedisMiddleware(serviceContainer), handlers.ScreenshotHandler)
		compatGroup.GET("screenshot", addRedisMiddleware(serviceContainer), handlers.ScreenshotHandler)

		// Base64截图兼容路由
		compatGroup.GET("screenshot/base64/:domain", addRedisMiddleware(serviceContainer), handlers.ScreenshotBase64Handler)

		// 元素截图兼容路由
		compatGroup.POST("screenshot/element", addRedisMiddleware(serviceContainer), handlers.NewElementScreenshotHandler)
		compatGroup.POST("screenshot/element/base64", addRedisMiddleware(serviceContainer), handlers.NewElementScreenshotBase64Handler)

		// ITDog截图兼容路由
		compatGroup.GET("itdog/:domain", addRedisMiddleware(serviceContainer), handlers.ITDogHandler)
		compatGroup.GET("itdog/base64/:domain", addRedisMiddleware(serviceContainer), handlers.ITDogBase64Handler)
		compatGroup.GET("itdog/table/:domain", addRedisMiddleware(serviceContainer), handlers.ITDogTableHandler)
		compatGroup.GET("itdog/table/base64/:domain", addRedisMiddleware(serviceContainer), handlers.ITDogTableBase64Handler)
		compatGroup.GET("itdog/ip/:domain", addRedisMiddleware(serviceContainer), handlers.ITDogIPHandler)
		compatGroup.GET("itdog/ip/base64/:domain", addRedisMiddleware(serviceContainer), handlers.ITDogIPBase64Handler)
		compatGroup.GET("itdog/resolve/:domain", addRedisMiddleware(serviceContainer), handlers.ITDogResolveHandler)
		compatGroup.GET("itdog/resolve/base64/:domain", addRedisMiddleware(serviceContainer), handlers.ITDogResolveBase64Handler)
	}

	// 添加中间件到所有截图路由
	screenshotGroup.Use(domainValidationMiddleware())
	screenshotGroup.Use(rateLimitMiddleware(serviceContainer.Limiter))
	screenshotGroup.Use(asyncWorkerMiddleware(serviceContainer.WorkerPool, 120*time.Second))

	// 为兼容路由也添加必要的中间件
	compatGroup.Use(domainValidationMiddleware())
	compatGroup.Use(rateLimitMiddleware(serviceContainer.Limiter))
	compatGroup.Use(asyncWorkerMiddleware(serviceContainer.WorkerPool, 120*time.Second))
}

// addRedisMiddleware 添加Redis中间件
func addRedisMiddleware(serviceContainer *services.ServiceContainer) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("redis", serviceContainer.RedisClient)
		c.Next()
	}
}

// UpdateScreenshotRoutesInMain 更新主路由文件中的截图路由
// 这个函数用于更新现有的路由注册
func UpdateScreenshotRoutesInMain(r *gin.Engine, serviceContainer *services.ServiceContainer) {
	// 移除原有的截图路由配置，使用新的路由配置
	// 这需要在原有的 RegisterAPIRoutes 函数中调用

	apiv1 := r.Group("/api/v1")

	// 更新截图路由配置
	screenshotGroup := apiv1.Group("/screenshot")
	screenshotGroup.Use(domainValidationMiddleware())
	screenshotGroup.Use(rateLimitMiddleware(serviceContainer.Limiter))
	screenshotGroup.Use(asyncWorkerMiddleware(serviceContainer.WorkerPool, 120*time.Second))

	// 基础截图路由 - 使用新的处理器
	screenshotGroup.GET("", func(c *gin.Context) {
		handlers.ScreenshotHandler(c)
	})
	screenshotGroup.GET("/:domain", func(c *gin.Context) {
		handlers.ScreenshotHandler(c)
	})

	// Base64截图路由
	screenshotBase64Group := apiv1.Group("/screenshot/base64")
	screenshotBase64Group.Use(domainValidationMiddleware())
	screenshotBase64Group.Use(rateLimitMiddleware(serviceContainer.Limiter))
	screenshotBase64Group.GET("/:domain", func(c *gin.Context) {
		handlers.ScreenshotBase64Handler(c)
	})

	// 元素截图路由
	elementGroup := apiv1.Group("/screenshot/element")
	elementGroup.POST("", func(c *gin.Context) {
		handlers.NewElementScreenshotHandler(c)
	})
	elementGroup.POST("/base64", func(c *gin.Context) {
		handlers.NewElementScreenshotBase64Handler(c)
	})

	// ITDog截图路由组
	itdogGroup := apiv1.Group("/itdog")
	itdogGroup.Use(domainValidationMiddleware())
	itdogGroup.Use(rateLimitMiddleware(serviceContainer.Limiter))
	itdogGroup.Use(asyncWorkerMiddleware(serviceContainer.WorkerPool, 30*time.Second))

	// ITDog地图截图
	itdogGroup.GET("/:domain", func(c *gin.Context) {
		handlers.ITDogHandler(c)
	})

	// ITDog Base64截图路由
	itdogBase64Group := apiv1.Group("/itdog/base64")
	itdogBase64Group.Use(domainValidationMiddleware())
	itdogBase64Group.Use(rateLimitMiddleware(serviceContainer.Limiter))
	itdogBase64Group.GET("/:domain", func(c *gin.Context) {
		handlers.ITDogBase64Handler(c)
	})

	// ITDog表格截图路由
	itdogTableGroup := apiv1.Group("/itdog/table")
	itdogTableGroup.Use(domainValidationMiddleware())
	itdogTableGroup.Use(rateLimitMiddleware(serviceContainer.Limiter))
	itdogTableGroup.GET("/:domain", func(c *gin.Context) {
		handlers.ITDogTableHandler(c)
	})

	// ITDog IP统计截图路由
	itdogIPGroup := apiv1.Group("/itdog/ip")
	itdogIPGroup.Use(domainValidationMiddleware())
	itdogIPGroup.Use(rateLimitMiddleware(serviceContainer.Limiter))
	itdogIPGroup.GET("/:domain", func(c *gin.Context) {
		handlers.ITDogIPHandler(c)
	})

	// ITDog IP统计截图Base64路由
	itdogIPBase64Group := apiv1.Group("/itdog/ip/base64")
	itdogIPBase64Group.Use(domainValidationMiddleware())
	itdogIPBase64Group.GET("/:domain", func(c *gin.Context) {
		handlers.ITDogIPBase64Handler(c)
	})

	// ITDog表格截图Base64路由
	itdogTableBase64Group := apiv1.Group("/itdog/table/base64")
	itdogTableBase64Group.Use(domainValidationMiddleware())
	itdogTableBase64Group.GET("/:domain", func(c *gin.Context) {
		handlers.ITDogTableBase64Handler(c)
	})

	// ITDog全国解析截图路由
	itdogResolveGroup := apiv1.Group("/itdog/resolve")
	itdogResolveGroup.Use(domainValidationMiddleware())
	itdogResolveGroup.GET("/:domain", func(c *gin.Context) {
		handlers.ITDogResolveHandler(c)
	})

	// ITDog全国解析截图Base64路由
	itdogResolveBase64Group := apiv1.Group("/itdog/resolve/base64")
	itdogResolveBase64Group.Use(domainValidationMiddleware())
	itdogResolveBase64Group.GET("/:domain", func(c *gin.Context) {
		handlers.ITDogResolveBase64Handler(c)
	})

	// Chrome管理API
	chromeGroup := apiv1.Group("/chrome")
	chromeGroup.GET("/status", handlers.NewChromeStatus)
	chromeGroup.POST("/restart", handlers.NewChromeRestart)
}