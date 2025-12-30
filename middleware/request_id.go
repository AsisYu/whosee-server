/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-12-30
 * @Description: Request ID中间件 - 用于请求追踪
 */

package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"whosee/pkg/logger"
)

// RequestID 生成或传播请求ID，用于分布式追踪
// 优先使用客户端提供的X-Request-ID头，否则生成新UUID
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从请求头获取request ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// 生成新的UUID作为request ID
			requestID = uuid.New().String()
		}

		// 存储到gin context（用于中间件和handler）
		c.Set("request_id", requestID)

		// 存储到标准context（用于service层）
		ctx := context.WithValue(c.Request.Context(), logger.RequestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		// 在响应头中返回request ID，方便客户端追踪
		c.Writer.Header().Set("X-Request-ID", requestID)

		c.Next()
	}
}
