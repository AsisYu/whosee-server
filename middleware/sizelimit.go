/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-28 10:26:00
 * @Description: 请求大小限制中间件
 */
package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SizeLimitConfig 请求大小限制配置
type SizeLimitConfig struct {
	Limit           int64  // 请求体大小限制（字节）
	StatusCode      int    // 超过限制时返回的状态码
	Message         string // 超过限制时返回的错误消息
	SkipPathFunc    func(path string) bool // 跳过某些路径的检查函数
	ExcludedMethods []string // 跳过检查的HTTP方法
}

// DefaultSizeLimitConfig 返回默认的请求大小限制配置
func DefaultSizeLimitConfig() SizeLimitConfig {
	return SizeLimitConfig{
		Limit:           1024 * 1024, // 1MB
		StatusCode:      413,
		Message:         "请求体过大",
		SkipPathFunc:    func(path string) bool { return false },
		ExcludedMethods: []string{"GET", "HEAD", "OPTIONS"},
	}
}

// SizeLimit 大小限制中间件
func SizeLimit() gin.HandlerFunc {
	return SizeLimitWithConfig(DefaultSizeLimitConfig())
}

// SizeLimitWithConfig 带配置的大小限制中间件
func SizeLimitWithConfig(config SizeLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否跳过检查
		method := c.Request.Method
		path := c.Request.URL.Path

		// 根据方法跳过检查
		for _, m := range config.ExcludedMethods {
			if method == m {
				c.Next()
				return
			}
		}

		// 根据路径跳过检查
		if config.SkipPathFunc(path) {
			c.Next()
			return
		}

		// 读取Content-Length头
		contentLength := c.Request.ContentLength
		if contentLength > config.Limit {
			c.AbortWithStatusJSON(config.StatusCode, gin.H{
				"error":   "REQUEST_ENTITY_TOO_LARGE",
				"message": fmt.Sprintf("%s，最大允许大小为 %d 字节", config.Message, config.Limit),
			})
			return
		}

		// 处理未指定Content-Length的情况
		if contentLength == -1 {
			buffer := &bytes.Buffer{}
			limitReader := io.LimitReader(c.Request.Body, config.Limit+1)
			n, err := buffer.ReadFrom(limitReader)
			
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error":   "INVALID_REQUEST_BODY",
					"message": "无法读取请求体",
				})
				return
			}
			
			if n > config.Limit {
				c.AbortWithStatusJSON(config.StatusCode, gin.H{
					"error":   "REQUEST_ENTITY_TOO_LARGE",
					"message": fmt.Sprintf("%s，最大允许大小为 %d 字节", config.Message, config.Limit),
				})
				return
			}
			
			// 重置请求体，因为已经被读取
			c.Request.Body = io.NopCloser(buffer)
		}
		
		c.Next()
	}
}
