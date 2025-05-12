/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: API响应工具
 */
package utils

import (
	"time"

	"github.com/gin-gonic/gin"
)

// 统一响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *MetaInfo   `json:"meta,omitempty"`
}

// 错误信息结构
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// 元信息结构
type MetaInfo struct {
	Timestamp  string `json:"timestamp"`
	RequestID  string `json:"requestId,omitempty"`
	Cached     bool   `json:"cached,omitempty"`
	CachedAt   string `json:"cachedAt,omitempty"`
	Version    string `json:"version,omitempty"`
	Processing int64  `json:"processingTimeMs,omitempty"`
}

// SuccessResponse 统一成功响应
func SuccessResponse(c *gin.Context, data interface{}, meta *MetaInfo) {
	if meta == nil {
		meta = &MetaInfo{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
	}

	c.JSON(200, APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

// ErrorResponse 统一错误响应
func ErrorResponse(c *gin.Context, statusCode int, errorCode string, message string) {
	c.JSON(statusCode, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    errorCode,
			Message: message,
		},
		Meta: &MetaInfo{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}
