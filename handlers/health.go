/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: 健康检查处理程序
 */
package handlers

import (
	"dmainwhoseek/utils"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// deriveBackendURL 根据请求头与TLS状态推断后端URL（含协议与Host）
func deriveBackendURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if v := c.GetHeader("X-Forwarded-Proto"); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			scheme = strings.TrimSpace(parts[0])
		}
	}

	host := c.Request.Host
	if v := c.GetHeader("X-Forwarded-Host"); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			host = strings.TrimSpace(parts[0])
		}
	}
	// 如果未通过 X-Forwarded-Host 指定端口，尝试使用 X-Forwarded-Port 补充端口
	if !strings.Contains(host, ":") {
		if p := c.GetHeader("X-Forwarded-Port"); p != "" {
			port := strings.TrimSpace(strings.Split(p, ",")[0])
			if port != "" {
				host = host + ":" + port
			}
		}
	}

	return scheme + "://" + host
}

// HealthCheckHandler 健康检查API处理程序
func HealthCheckHandler(healthChecker interface{}) gin.HandlerFunc {
	healthLogger := utils.GetHealthLogger()
	return func(c *gin.Context) {
		// 获取参数
		detailed := c.DefaultQuery("detailed", "false") == "true"

		healthLogger.Printf("健康检查API调用: detailed=%v, URI=%s", detailed, c.Request.RequestURI)

		// 基本响应
		response := gin.H{
			"status":   "up",
			"version":  os.Getenv("APP_VERSION"),
			"time":     time.Now().UTC().Format(time.RFC3339),
			"services": gin.H{}, // 初始化services map
		}

		// 获取IP地址
		ip := c.ClientIP()
		healthLogger.Printf("健康检查API请求来自: %s", ip)

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

		// 添加当前后端URL提示
		backendURL := deriveBackendURL(c)
		backendPort := func() string {
			u, err := url.Parse(backendURL)
			if err == nil {
				if p := u.Port(); p != "" {
					return p
				}
				if u.Scheme == "https" {
					return "443"
				}
				return "80"
			}
			return ""
		}()
		listenPort := os.Getenv("PORT")
		if listenPort == "" {
			listenPort = "8080"
		}
		response["backendUrl"] = backendURL
		response["backendPort"] = backendPort
		response["listenPort"] = listenPort
		healthLogger.Printf("健康检查返回后端URL: %s, 客户端端口: %s, 监听端口: %s", backendURL, backendPort, listenPort)

		// 返回健康状态
		c.JSON(200, response)
	}
}
