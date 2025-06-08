# 工具包目录 (Utils)

## 目录作用

工具包目录包含各种辅助功能和通用工具函数，这些函数被整个应用程序使用。这些功能通常是与业务逻辑无关的通用功能，如字符串处理、响应格式化、域名处理、Chrome浏览器管理等。

## 文件列表与功能

- `api.go` - API响应格式化工具和统一响应结构
- `domain.go` - 域名验证和清理工具
- `string_utils.go` - 字符串处理工具函数
- `chrome.go` - Chrome浏览器工具和统一实例管理

## 标准响应格式

工具包提供了统一的API响应格式，确保所有API端点返回一致的结构：

```go
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
```

## 域名验证

工具包提供了域名验证功能，确保输入的域名符合有效格式：

```go
// IsValidDomain 验证域名是否有效
func IsValidDomain(domain string) bool {
	// 忽略协议前缀
	domain = strings.TrimPrefix(strings.TrimPrefix(domain, "http://"), "https://")
	
	// 移除端口和路径
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	
	// 使用正则表达式验证域名格式
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+ [a-zA-Z]{2,}$`)
	return domainRegex.MatchString(domain)
}

// SanitizeDomain 清理和标准化域名
func SanitizeDomain(domain string) string {
	// 去除协议前缀
	domain = strings.TrimPrefix(strings.TrimPrefix(domain, "http://"), "https://")
	
	// 移除端口和路径
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	
	// 转换为小写
	return strings.ToLower(domain)
}
```

## 字符串工具

工具包提供了字符串处理功能：

```go
// TruncateString 截断长字符串，超过最大长度时添加省略号
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

## Chrome浏览器工具

Chrome工具提供了统一的浏览器实例管理，用于截图和页面操作。采用单例模式，避免重复创建Chrome实例，大幅降低资源消耗。

```go
// 获取全局Chrome工具实例
chromeUtil := utils.GetGlobalChromeUtil()

// 获取Chrome上下文用于操作
ctx, cancel, err := chromeUtil.GetContext(60 * time.Second)
if err != nil {
    return err
}
defer cancel()

// 使用chromedp执行操作
err = chromedp.Run(ctx,
    chromedp.Navigate("https://example.com"),
    chromedp.Screenshot("#selector", &buf, chromedp.NodeVisible, chromedp.ByQuery),
)

// 检查Chrome健康状态
if chromeUtil.IsHealthy() {
    log.Println("Chrome实例运行正常")
}

// 获取统计信息
stats := chromeUtil.GetStats()
fmt.Printf("Chrome运行状态: %+v\n", stats)
```

### Chrome工具特性

- **单例模式** - 全局唯一Chrome实例，减少资源消耗
- **自动重启** - 检测到异常时自动重启Chrome实例
- **上下文管理** - 为每个操作提供独立的子上下文
- **健康检查** - 定期检查Chrome实例健康状态
- **内存优化** - 针对截图场景优化的启动参数
- **统计信息** - 提供运行状态和性能统计

## 设计原则

1. **通用性** - 工具函数应该是通用的，而不是特定于业务
2. **无状态** - 工具应该是无状态的，不依赖于上下文
3. **测试覆盖** - 所有工具功能应该有高测试覆盖率
4. **简单性** - 每个工具应该做一件事并做好
