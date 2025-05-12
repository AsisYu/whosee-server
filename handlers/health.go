/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: 健康检查处理程序
 */
package handlers

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthCheckHandler 健康检查API处理程序
func HealthCheckHandler(healthChecker interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取参数
		detailed := c.DefaultQuery("detailed", "false") == "true"

		log.Printf("健康检查API调用: detailed=%v, URI=%s", detailed, c.Request.RequestURI)

		// 基本响应
		response := gin.H{
			"status":   "up",
			"version":  os.Getenv("APP_VERSION"),
			"time":     time.Now().UTC().Format(time.RFC3339),
			"services": gin.H{}, // 初始化services map
		}

		// 获取IP地址
		ip := c.ClientIP()
		log.Printf("健康检查API请求来自: %s", ip)

		// 尝试获取健康检查器
		if healthCheckerObj, hasHealthChecker := c.Get("healthChecker"); hasHealthChecker {
			// 使用已有的健康检查结果，减少重复查询
			if checker, ok := healthCheckerObj.(interface {
				GetHealthStatus() map[string]interface{}
			}); ok {
				serviceStatus := checker.GetHealthStatus()
				response["services"] = serviceStatus
				response["lastCheck"] = time.Now().Format(time.RFC3339)

				// 检查各服务状态，如果有服务异常，整体状态为degraded
				overallStatus := "up"
				for _, serviceInfo := range serviceStatus {
					if serviceMap, ok := serviceInfo.(map[string]interface{}); ok {
						if status, exists := serviceMap["status"]; exists && status != "up" {
							overallStatus = "degraded"
							break
						}
					}
				}
				response["status"] = overallStatus
			}
		}

		// 返回健康状态
		c.JSON(200, response)
	}
}
