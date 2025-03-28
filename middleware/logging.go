package middleware

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func EnhancedLogging() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 生成请求ID
		requestID := fmt.Sprintf("%d-%s", time.Now().UnixNano(), c.ClientIP())
		c.Set("requestId", requestID)
		c.Header("X-Request-ID", requestID)

		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		log.Printf("[API] %v | %3d | %13v | %15s | %-7s %s%s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
			raw,
		)
	}
}
