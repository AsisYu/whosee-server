/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-17 21:22:04
 * @LastEditors: AsisYu 2773943729@qq.com
 * @LastEditTime: 2025-01-18 01:02:03
 * @FilePath: \dmainwhoseek\server\middleware\cors.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package middleware

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录初始状态
		fmt.Printf("收到请求，Origin: %s, Method: %s\n", c.Request.Header.Get("Origin"), c.Request.Method)

		// 清除所有可能存在的 CORS 相关头
		c.Writer.Header().Del("Access-Control-Allow-Origin")
		c.Writer.Header().Del("Access-Control-Allow-Credentials")
		c.Writer.Header().Del("Access-Control-Allow-Methods")
		c.Writer.Header().Del("Access-Control-Allow-Headers")
		c.Writer.Header().Del("Access-Control-Max-Age")

		// 添加对任意域名OPTIONS请求的支持
		if c.Request.Method == "OPTIONS" {
			origin := c.Request.Header.Get("Origin")
			// 对任意域名的OPTIONS请求都返回允许的头部
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "*")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
			c.AbortWithStatus(204)
			return
		}

		origin := c.Request.Header.Get("Origin")
		
		// 如果没有Origin头，可能是内部请求或健康检查，直接通过
		if origin == "" {
			// 这是内部请求或健康检查，不需要CORS头
			if c.Request.URL.Path == "/api/health" {
				// 健康检查路径，不记录日志以减少噪音
			} else {
				// 其他内部请求，记录但不显示为错误
				log.Printf("内部请求或无Origin请求: %s %s", c.Request.Method, c.Request.URL.Path)
			}
			c.Next()
			return
		}
		
		// 允许的域名列表
		allowedOrigins := map[string]bool{
			"http://localhost:8080":           true, // Vue开发环境
			"http://localhost:3000":           true, // 开发环境
			"http://localhost:5173":           true, // SvelteKit开发环境
			"https://domain-whois.vercel.app": true, // 生产环境
			"https://whosee.me":               true, //域名
		}

		// 如果是允许的域名，设置对应的 CORS 头
		if allowedOrigins[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers",
				"Content-Type, Authorization, X-Requested-With, Accept")
			// 添加允许的响应头
			c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Cache")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")

			// 记录设置的 CORS 头
			fmt.Printf("设置 CORS 头部 - Origin: %s\n", origin)
			fmt.Printf("当前所有 Access-Control-Allow-Origin 头: %v\n",
				c.Writer.Header()["Access-Control-Allow-Origin"])
		} else {
			fmt.Printf("请求来源不在允许列表中: %s\n", origin)
		}

		c.Next()

		// 记录最终响应头
		fmt.Printf("响应完成，最终的 CORS 头: %v\n",
			c.Writer.Header()["Access-Control-Allow-Origin"])
	}
}
