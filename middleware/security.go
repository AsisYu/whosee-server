/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-17 23:47:06
 * @LastEditors: AsisYu 2773943729@qq.com
 * @LastEditTime: 2025-01-17 23:47:24
 * @FilePath: \dmainwhoseek\server\middleware\security.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package middleware

import "github.com/gin-gonic/gin"

func Security() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=()")
		c.Next()
	}
}
