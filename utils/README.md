# 工具包目录 (Utils)

## 目录作用

工具包目录包含各种辅助功能和通用工具函数，这些函数被整个应用程序使用。这些功能通常是与业务逻辑无关的通用功能，如字符串处理、响应格式化、域名处理、Chrome浏览器管理等。

## 文件列表与功能

### 核心工具文件
- `api.go` - API响应格式化工具和统一响应结构
- `string_utils.go` - 字符串处理工具函数
- `cache_keys.go` - 缓存键生成和管理工具
- `health_logger.go` - 健康检查日志记录工具

### 域名处理工具 
- `domain.go` - **增强的域名验证和安全工具**
  - 域名格式验证和清理
  - URL安全性检查
  - 安全文件名生成
  - 防止路径遍历攻击

### Chrome浏览器工具 
- `chrome.go` - Chrome浏览器工具和智能实例管理（支持冷启动、热启动、智能混合模式）
- `chrome_downloader.go` - Chrome浏览器下载器，支持智能平台检测和自动下载

##  安全增强功能

### 域名验证和安全工具

```go
// IsValidDomain 验证域名是否有效
func IsValidDomain(domain string) bool {
    if domain == "" {
        return false
    }

    // 清理域名
    cleaned := SanitizeDomain(domain)
    if cleaned == "" {
        return false
    }

    // 长度检查
    if len(cleaned) > 253 {
        return false
    }

    // 基本格式验证
    domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
    if !domainRegex.MatchString(cleaned) {
        return false
    }

    // 检查是否包含有效的TLD
    parts := strings.Split(cleaned, ".")
    if len(parts) < 2 {
        return false
    }

    return true
}

// ValidateURL 验证URL安全性
func ValidateURL(url string) bool {
    if url == "" {
        return false
    }

    // 检查协议
    if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
        return false
    }

    // 解析URL
    parsedURL, err := urlPkg.Parse(url)
    if err != nil {
        return false
    }

    // 检查主机名
    if parsedURL.Host == "" {
        return false
    }

    // 防止内网访问
    host := parsedURL.Hostname()
    if isPrivateIP(host) {
        return false
    }

    return true
}

// GenerateSecureFilename 生成安全的文件名
func GenerateSecureFilename(input string) string {
    if input == "" {
        return "unknown"
    }

    // 替换危险字符
    safe := strings.ReplaceAll(input, "..", "_")
    safe = strings.ReplaceAll(safe, "/", "_")
    safe = strings.ReplaceAll(safe, "\\", "_")
    safe = strings.ReplaceAll(safe, ":", "_")
    safe = strings.ReplaceAll(safe, "*", "_")
    safe = strings.ReplaceAll(safe, "?", "_")
    safe = strings.ReplaceAll(safe, "<", "_")
    safe = strings.ReplaceAll(safe, ">", "_")
    safe = strings.ReplaceAll(safe, "|", "_")

    // 限制长度
    if len(safe) > 100 {
        safe = safe[:100]
    }

    // 确保不为空
    if safe == "" {
        return "unknown"
    }

    return safe
}
```

### 内网IP检测
```go
// isPrivateIP 检查是否为内网IP
func isPrivateIP(host string) bool {
    ip := net.ParseIP(host)
    if ip == nil {
        return false
    }

    // 私有网络范围
    privateRanges := []string{
        "10.0.0.0/8",
        "172.16.0.0/12",
        "192.168.0.0/16",
        "127.0.0.0/8",
        "169.254.0.0/16",
        "::1/128",
        "fc00::/7",
        "fe80::/10",
    }

    for _, cidr := range privateRanges {
        _, network, _ := net.ParseCIDR(cidr)
        if network != nil && network.Contains(ip) {
            return true
        }
    }

    return false
}
```

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

Chrome工具提供了智能的浏览器实例管理，用于截图和页面操作。采用智能混合模式，特别适合WHOIS服务（主要功能）+ 偶尔截图的使用场景。

###  核心特性

- **智能混合模式** - 默认采用智能混合模式，按需启动+智能复用
- **三种运行模式** - 冷启动、热启动、智能混合，可根据使用场景选择
- **智能平台检测** - 自动检测Windows、Linux、macOS等平台，支持WSL和容器环境
- **自动Chrome下载** - 智能下载和管理Chrome浏览器，支持中国镜像源
- **资源优化** - 根据使用频率动态调整空闲超时时间
- **并发控制** - 限制最大并发数，避免资源耗尽
- **错误恢复** - 自动重启异常的Chrome实例

###  基本使用

```go
// 获取全局Chrome工具实例（智能混合模式）
chromeUtil := utils.GetGlobalChromeUtil()

// 获取Chrome上下文用于操作（自动启动Chrome）
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

// 获取Chrome运行统计
stats := utils.GetChromeStats()
fmt.Printf("Chrome运行状态: %+v\n", stats)
```

###  模式配置

Chrome工具支持三种运行模式，可根据使用场景灵活选择：

```go
// 方式1: 使用便捷函数设置全局模式
utils.SetGlobalChromeMode("cold")    // 冷启动模式
utils.SetGlobalChromeMode("warm")    // 热启动模式
utils.SetGlobalChromeMode("auto")    // 智能混合模式（推荐）

// 方式2: 使用自定义配置
config := utils.ConfigureChromeMode("auto")
chromeUtil := utils.GetGlobalChromeUtilWithConfig(config)

// 方式3: 完全自定义配置
customConfig := utils.ChromeConfig{
    Mode:                utils.ChromeModeAuto,
    IdleTimeout:         5 * time.Minute,
    EnableHealthCheck:   false,
    PrewarmOnStart:      false,
}
chromeUtil := utils.NewChromeUtilWithConfig(customConfig)
```

### 三种模式对比

| 模式 | 启动方式 | 资源占用 | 响应速度 | 适用场景 | 空闲管理 |
|------|----------|----------|----------|----------|----------|
| **冷启动** | 每次重新启动 | 最低 | 慢(2-3秒) | 极少使用截图 | 用完即关 |
| **热启动** | 预热保持运行 | 较高 | 最快(<100ms) | 频繁使用截图 | 10分钟自动关闭 |
| **智能混合**  | 按需+智能复用 | 中等 | 适中 | **WHOIS主业务+偶尔截图** | 智能调整(1.5-6分钟) |

###  智能混合模式详解

智能混合模式是为您的使用场景特别设计的：

**智能启动策略：**
- 首次使用：快速启动（偶尔使用策略）
- 频繁使用（>5次）：自动切换为热启动策略
- 空闲检测：根据使用频率智能调整空闲超时时间

**智能空闲管理：**
- 偶尔使用：1.5分钟空闲后自动关闭
- 频繁使用：6分钟空闲后自动关闭
- 实例复用：健康的Chrome实例直接复用

**智能行为示例：**
```
# 首次使用
[CHROME-UTIL] 智能模式：偶尔使用，采用快速启动策略

# 使用频繁后
[CHROME-UTIL] 智能模式：频繁使用，采用热启动策略
[CHROME-UTIL] 智能模式：频繁使用，延长空闲时间至 6m0s

# 复用现有实例
[CHROME-UTIL] 智能模式：复用现有实例
```

###  Chrome下载器

自动管理Chrome浏览器的下载和安装：

```go
// 创建Chrome下载器
downloader := utils.NewChromeDownloader()

// 检查Chrome是否存在
if downloader.IsChromeBinaryExists() {
    log.Println("Chrome已存在")
}

// 确保Chrome可用（自动下载）
execPath, err := downloader.EnsureChrome()
if err != nil {
    log.Printf("Chrome准备失败: %v", err)
} else {
    log.Printf("Chrome就绪: %s", execPath)
}

// 获取Chrome信息
info := downloader.GetChromeInfo()
fmt.Printf("Chrome信息: %+v\n", info)
```

**Chrome下载器特性：**
- **智能平台检测** - 自动识别Windows、Linux、macOS等平台
- **特殊环境支持** - 检测WSL、Docker容器等特殊环境
- **中国镜像优化** - 自动使用华为云、淘宝等国内镜像源
- **多重下载策略** - 官方源 + 多个镜像源，确保下载成功
- **文件完整性验证** - 下载后验证文件大小和可执行性
- **智能路径搜索** - 支持多种Chrome归档结构

###  高级功能

```go
// 强制重置Chrome实例
err := chromeUtil.ForceReset()

// 执行详细诊断（仅在出问题时）
chromeUtil.performDetailedDiagnosis()

// 获取详细统计信息
stats := chromeUtil.GetDetailedStats()

// 检查Chrome健康状态（快速检查）
if chromeUtil.IsHealthy() {
    log.Println("Chrome实例健康")
}

// 手动停止Chrome
chromeUtil.Stop()

// 重启Chrome
err := chromeUtil.Restart()
```

###  性能优化

- **并发控制** - 最大3个并发Chrome操作，避免资源竞争
- **内存优化** - 针对截图场景优化的启动参数
- **智能重启** - 异常检测和自动重启机制
- **资源释放** - 自动空闲超时和资源清理
- **上下文管理** - 为每个操作提供独立的子上下文

###  诊断和监控

- **简化日志** - 去除定期健康检查，只在必要时输出诊断信息
- **智能诊断** - 仅在出现问题时执行详细诊断
- **统计信息** - 使用次数、运行时间、成功率等统计
- **错误恢复** - 连续失败时自动执行强制重置

## 缓存键管理

```go
// 缓存键生成工具
func GenerateCacheKey(prefix string, params ...string) string {
    key := prefix
    for _, param := range params {
        key += ":" + param
    }
    return key
}

// 通用缓存键前缀
const (
    CacheKeyWhois      = "whois"
    CacheKeyDNS        = "dns"
    CacheKeyScreenshot = "screenshot"
    CacheKeyHealth     = "health"
)
```

## 健康检查日志

```go
// 健康检查日志记录器
type HealthLogger struct {
    logger *log.Logger
    file   *os.File
}

// 记录健康检查日志
func (hl *HealthLogger) LogHealth(component string, status bool, details map[string]interface{}) {
    logEntry := map[string]interface{}{
        "timestamp": time.Now().UTC().Format(time.RFC3339),
        "component": component,
        "status":    status,
        "details":   details,
    }

    jsonData, _ := json.Marshal(logEntry)
    hl.logger.Println(string(jsonData))
}
```

## 设计原则

1. **通用性** - 工具函数应该是通用的，而不是特定于业务
2. **无状态** - 工具应该是无状态的，不依赖于上下文
3. **智能化** - 根据使用模式自动优化性能和资源使用
4. **简单性** - 每个工具应该做一件事并做好
5. **可靠性** - 具备错误恢复和自愈能力
6. **安全性** - 所有输入都经过验证和清理 

## 安全最佳实践 

### 输入验证
```go
// 总是验证域名
if !utils.IsValidDomain(domain) {
    return errors.New("无效的域名格式")
}

// 验证URL安全性
if !utils.ValidateURL(url) {
    return errors.New("不安全的URL")
}

// 生成安全文件名
safeFileName := utils.GenerateSecureFilename(userInput)
```

### 防护措施
- **路径遍历防护** - 防止 `../../../etc/passwd` 等攻击
- **内网访问防护** - 阻止访问内网IP地址
- **文件名清理** - 移除危险字符和控制字符
- **输入长度限制** - 防止过长输入导致的问题

## 最佳实践

1. **Chrome使用** - 默认使用智能混合模式，适合大多数场景
2. **错误处理** - 合理处理Chrome启动失败和超时错误
3. **资源清理** - 及时调用cancel函数释放Chrome上下文
4. **性能监控** - 定期查看Chrome统计信息，了解使用情况
5. **模式选择** - 根据截图使用频率选择合适的运行模式
6. **安全验证** - 始终验证用户输入的域名和URL 
7. **文件安全** - 使用安全文件名生成，防止路径攻击 

## 工具函数索引

### 域名和URL
- `IsValidDomain(domain string) bool` - 验证域名格式
- `SanitizeDomain(domain string) string` - 清理域名
- `ValidateURL(url string) bool` - 验证URL安全性

### 文件安全
- `GenerateSecureFilename(input string) string` - 生成安全文件名

### 字符串处理
- `TruncateString(s string, maxLen int) string` - 截断字符串

### API响应
- `SuccessResponse(c *gin.Context, data interface{}, meta *MetaInfo)` - 成功响应
- `ErrorResponse(c *gin.Context, statusCode int, errorCode string, message string)` - 错误响应

### Chrome管理
- `GetGlobalChromeUtil() *ChromeUtil` - 获取Chrome工具实例
- `SetGlobalChromeMode(mode string)` - 设置Chrome模式

### 缓存管理
- `GenerateCacheKey(prefix string, params ...string) string` - 生成缓存键
