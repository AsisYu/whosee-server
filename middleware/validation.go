package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func RequestValidation() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查请求大小
		if c.Request.ContentLength > 1024*1024 { // 1MB限制
			c.AbortWithStatusJSON(413, gin.H{
				"error": "请求体过大",
				"code":  "REQUEST_TOO_LARGE",
			})
			return
		}

		// 检查User-Agent
		userAgent := c.GetHeader("User-Agent")
		if userAgent == "" {
			c.AbortWithStatusJSON(400, gin.H{
				"error": "缺少User-Agent",
				"code":  "INVALID_REQUEST",
			})
			return
		}

		// 验证Content-Type
		if c.Request.Method != "GET" &&
			c.GetHeader("Content-Type") != "application/json" {
			c.AbortWithStatusJSON(415, gin.H{
				"error": "仅支持application/json",
				"code":  "UNSUPPORTED_MEDIA_TYPE",
			})
			return
		}

		// 检查请求路径
		path := c.Request.URL.Path
		if len(path) > 255 || strings.Contains(path, "..") {
			c.AbortWithStatusJSON(400, gin.H{
				"error": "无效的请求路径",
				"code":  "INVALID_PATH",
			})
			return
		}

		c.Next()
	}
}
