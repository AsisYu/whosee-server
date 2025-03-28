package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			log.Printf("Error: %v", err)

			c.JSON(500, gin.H{
				"error":     "服务器内部错误",
				"requestId": c.GetString("requestId"),
				"timestamp": time.Now().Unix(),
				"path":      c.Request.URL.Path,
				"code":      "INTERNAL_SERVER_ERROR",
			})
		}
	}
}
