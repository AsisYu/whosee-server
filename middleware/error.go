/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-03-31 04:10:00
 * @Description: 错误处理中间件
 */
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
