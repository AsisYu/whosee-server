/*
 * @Author: AsisYu
 * @Date: 2025-04-25
 * @Description: JSON请求验证器
 */
package middleware

import (
	"dmainwhoseek/utils"
	"log"

	"github.com/gin-gonic/gin"
)

// JSONBodyValidator JSON请求验证器
func JSONBodyValidator(model interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果请求不是POST或PUT，则直接跳过验证
		if c.Request.Method != "POST" && c.Request.Method != "PUT" && c.Request.Method != "PATCH" {
			c.Next()
			return
		}

		// 复制模型
		newModel := model // 注意：这里是正确的复制模型，若需要使用指针或其他方法

		// 绑定JSON到模型中
		if err := c.ShouldBindJSON(newModel); err != nil {
			log.Printf("[验证器] JSON验证失败: %v", err)
			utils.ErrorResponse(c, 400, "INVALID_REQUEST", "Invalid request format: "+err.Error())
			c.Abort()
			return
		}

		// 将验证后的模型放到上下文中
		if c.Request.Method == "POST" {
			c.Set("elementScreenshotRequest", newModel)
		}

		c.Next()
	}
}
