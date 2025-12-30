/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-12-30
 * @Description: HTTP访问日志中间件 - 使用结构化日志
 */

package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"whosee/pkg/logger"
)

// HTTPLogger 记录HTTP请求的结构化日志
func HTTPLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 计算延迟
		latency := time.Since(start)
		statusCode := c.Writer.Status()

		// 使用WithRequest获取带request_id的logger
		log := logger.WithRequest(c, "HTTP")

		// 构建日志字段
		fields := []interface{}{
			"method", c.Request.Method,
			"path", path,
			"status", statusCode,
			"latency_ms", latency.Milliseconds(),
			"user_agent", c.Request.UserAgent(),
		}

		if query != "" {
			fields = append(fields, "query", query)
		}

		// 如果有错误，记录错误信息
		if len(c.Errors) > 0 {
			fields = append(fields, "errors", c.Errors.String())
		}

		// 根据状态码选择日志级别
		message := "HTTP request completed"
		if statusCode >= 500 {
			log.With(fields...).Errorw(message)
		} else if statusCode >= 400 {
			log.With(fields...).Warnw(message)
		} else {
			log.With(fields...).Infow(message)
		}
	}
}
