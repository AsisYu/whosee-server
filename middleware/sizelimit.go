package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SizeLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 限制请求体大小为 1MB
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)

		if c.Request.ContentLength > 1024*1024 {
			c.JSON(413, gin.H{"error": "请求体过大"})
			c.Abort()
			return
		}

		c.Next()
	}
}
