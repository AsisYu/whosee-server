/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-03-31 04:10:00
 * @Description: 日志中间件
 */
package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LogFormat 定义统一的日志格式
type LogFormat struct {
	TimeStamp    string      `json:"timestamp"`
	StatusCode   int         `json:"status_code"`
	Latency      string      `json:"latency"`
	ClientIP     string      `json:"client_ip"`
	Method       string      `json:"method"`
	Path         string      `json:"path"`
	Query        string      `json:"query,omitempty"`
	UserAgent    string      `json:"user_agent"`
	ErrorMessage string      `json:"error,omitempty"`
	RequestID    string      `json:"request_id"`
	RequestBody  interface{} `json:"request_body,omitempty"`
	RequestSize  int         `json:"request_size"`
	ResponseSize int         `json:"response_size"`
	Endpoint     string      `json:"endpoint,omitempty"`   // 新增：API端点标识
	Operation    string      `json:"operation,omitempty"`  // 新增：操作类型
	Parameters   interface{} `json:"parameters,omitempty"` // 新增：请求参数
}

// EnhancedLogging 提供增强的日志功能
func EnhancedLogging() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 生成请求ID
		requestID := fmt.Sprintf("%d-%s", time.Now().UnixNano(), c.ClientIP())
		c.Set("requestId", requestID)
		c.Header("X-Request-ID", requestID)

		// 记录请求开始时间
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 获取端点标识
		endpoint := getEndpointName(path)
		operation := getOperationType(path)

		// 记录请求开始
		log.Printf("[%s] %s 开始处理 | %15s | %s?%s | ID:%s",
			operation, endpoint, c.ClientIP(), path, raw, requestID)

		// 记录请求体（仅记录POST、PUT等非GET请求，且内容长度受限）
		var requestBody interface{}
		var requestBodySize int
		var params interface{}

		if c.Request.Method != "GET" && c.Request.ContentLength < 10240 { // 限制10KB
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			requestBodySize = len(bodyBytes)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// 尝试解析JSON
			if strings.Contains(c.ContentType(), "application/json") {
				json.Unmarshal(bodyBytes, &requestBody)
				params = requestBody
			}
		} else if c.Request.Method == "GET" {
			// 对于GET请求，记录查询参数
			if raw != "" {
				queryMap := make(map[string]string)
				for k, v := range c.Request.URL.Query() {
					if len(v) > 0 {
						queryMap[k] = v[0]
					}
				}
				params = queryMap
			}
		}

		// 替换原始响应写入器以捕获响应大小
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		// 处理请求
		c.Next()

		// 计算延迟
		latency := time.Since(start)

		// 获取错误信息（如果有）
		errorMessage := ""
		if len(c.Errors) > 0 {
			errorMessage = c.Errors.String()
		}

		// 构建完整日志
		logEntry := LogFormat{
			TimeStamp:    time.Now().Format("2006/01/02 - 15:04:05"),
			StatusCode:   c.Writer.Status(),
			Latency:      latency.String(),
			ClientIP:     c.ClientIP(),
			Method:       c.Request.Method,
			Path:         path,
			Query:        raw,
			UserAgent:    c.Request.UserAgent(),
			ErrorMessage: errorMessage,
			RequestID:    requestID,
			RequestBody:  requestBody,
			RequestSize:  requestBodySize,
			ResponseSize: blw.body.Len(),
			Endpoint:     endpoint,
			Operation:    operation,
			Parameters:   params,
		}

		// 构建请求完成日志
		statusText := "成功"
		logPrefix := "[完成]"
		if c.Writer.Status() >= 400 {
			statusText = "失败"
			logPrefix = "[错误]"
		}

		// 记录请求结束
		log.Printf("%s [%s] %s 处理%s | %3d | %13v | %15s | %s?%s",
			logPrefix, operation, endpoint, statusText,
			logEntry.StatusCode, logEntry.Latency,
			logEntry.ClientIP, path, raw)

		// 简化控制台日志输出 - 保留原来的格式以兼容现有日志分析工具
		log.Printf("[API] %v | %3d | %13v | %15s | %-7s %s%s%s",
			logEntry.TimeStamp,
			logEntry.StatusCode,
			logEntry.Latency,
			logEntry.ClientIP,
			logEntry.Method,
			logEntry.Path,
			func() string {
				if raw != "" {
					return "?" + raw
				}
				return ""
			}(),
			func() string {
				if errorMessage != "" {
					return " | ERROR: " + errorMessage
				}
				return ""
			}(),
		)

		// 对于慢请求或错误请求，记录更详细的信息
		if latency > 500*time.Millisecond || c.Writer.Status() >= 400 {
			jsonLog, _ := json.Marshal(logEntry)
			log.Printf("[详细] %s", string(jsonLog))
		}
	}
}

// getEndpointName 从路径获取端点名称
func getEndpointName(path string) string {
	// 移除API前缀并按路径段分割
	pathParts := strings.Split(strings.TrimPrefix(path, "/api/"), "/")

	// 提取主要端点名称
	if len(pathParts) > 0 && pathParts[0] != "" {
		return strings.ToUpper(pathParts[0])
	}

	return "ROOT"
}

// getOperationType 从路径获取操作类型
func getOperationType(path string) string {
	// 根据路径确定操作类型
	if strings.Contains(path, "/api/query") {
		return "WHOIS"
	} else if strings.Contains(path, "/api/dns") {
		return "DNS"
	} else if strings.Contains(path, "/api/screenshot") {
		return "SCREENSHOT"
	} else if strings.Contains(path, "/api/health") {
		return "HEALTH"
	} else if strings.Contains(path, "/api/auth") {
		return "AUTH"
	}

	return "API"
}

// bodyLogWriter 是一个响应体日志记录包装器
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write 写入响应并同时复制到缓冲区
func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// WriteString 写入字符串响应并同时复制到缓冲区
func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// WriteHeader 写入响应头
func (w *bodyLogWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
}

// Status 获取状态码
func (w *bodyLogWriter) Status() int {
	return w.ResponseWriter.Status()
}

// Size 获取大小
func (w *bodyLogWriter) Size() int {
	return w.ResponseWriter.Size()
}

// Written 检查是否已写入
func (w *bodyLogWriter) Written() bool {
	return w.ResponseWriter.Written()
}

// WriteHeaderNow 立即写入头部
func (w *bodyLogWriter) WriteHeaderNow() {
	w.ResponseWriter.WriteHeaderNow()
}

// Pusher 获取pusher
func (w *bodyLogWriter) Pusher() (pusher http.Pusher) {
	return w.ResponseWriter.Pusher()
}
