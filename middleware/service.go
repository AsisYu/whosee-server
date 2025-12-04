/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: 服务注入中间件
 */
package middleware

import (
	"whosee/services"

	"github.com/gin-gonic/gin"
)

// ServiceMiddleware Gin路由器中间件，用于在请求上下文中添加各种服务
func ServiceMiddleware(container *services.ServiceContainer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 注入所有服务到上下文
		if container != nil {
			// 注入WHOIS管理器
			if container.WhoisManager != nil {
				c.Set("whoisManager", container.WhoisManager)
			}

			// 注入DNS检查器
			if container.DNSChecker != nil {
				c.Set("dnsChecker", container.DNSChecker)
			}

			// 注入截图检查器
			if container.ScreenshotChecker != nil {
				c.Set("screenshotChecker", container.ScreenshotChecker)
			}

			// 注入IT Dog检查器
			if container.ITDogChecker != nil {
				c.Set("itdogChecker", container.ITDogChecker)
			}

			// 注入健康检查器
			if container.HealthChecker != nil {
				c.Set("healthChecker", container.HealthChecker)
			}

			// 注入Redis客户端
			if container.RedisClient != nil {
				c.Set("redis", container.RedisClient)
			}

			// 注入工作池
			if container.WorkerPool != nil {
				c.Set("workerPool", container.WorkerPool)
			}
		}

		// 继续处理请求
		c.Next()
	}
}
