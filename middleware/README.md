# 中间件目录 (Middleware)

## 目录作用

中间件目录包含在HTTP请求处理过程中执行的中间件组件。这些组件负责跨切关注点，如认证、验证、限流、性能监控等，它们在请求达到实际的业务处理程序之前执行。

## 文件列表与功能

### 核心中间件
- `service.go` - 服务注入中间件，将服务组件注入到Gin上下文
- `error.go` - 错误处理中间件，统一错误处理机制
- `logging.go` - 日志中间件，记录请求和响应信息

### 安全中间件 
- `auth.go` - JWT认证中间件，处理用户令牌和权限验证
- `ip_whitelist.go` - IP白名单中间件，支持严格模式和宽松模式
- `security.go` - 安全头中间件，设置各种安全HTTP头
- `validation.go` - 请求验证中间件，验证请求参数格式
- `validator.go` - 数据验证器，结构化数据验证

### 性能和限制中间件 
- `ratelimit.go` - 分布式限流中间件，基于Redis的请求限流
- `sizelimit.go` - 请求大小限制中间件，防止大文件攻击
- `monitoring.go` - 性能监控中间件，收集请求统计信息

### 网络中间件 
- `cors.go` - CORS中间件，处理跨域资源共享

## 中间件工作原理

在Gin中，中间件工作如下：

1. 请求达到服务器
2. 按注册顺序执行中间件
3. 在中间件内可以选择调用`c.Next()`跳到下一个中间件/处理程序或`c.Abort()`中断请求
4. 当所有处理完成后，中间件上下文堆栈会从后向前执行(c.Next()后的代码)

```go
func ExampleMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 中间件前置代码
        log.Println("请求到达")

        // 调用下一个中间件/处理程序
        c.Next()

        // 中间件后置代码 - 处理器完成后执行
        log.Println("请求完成")
    }
}
```

## 主要中间件详解

###  服务中间件

`middleware.ServiceMiddleware`是核心中间件，它将服务容器中的所有服务注入到Gin请求上下文中，使得处理程序可以访问到这些服务：

```go
// 注册服务中间件
r.Use(middleware.ServiceMiddleware(serviceContainer))

// 在处理器中使用
func MyHandler(c *gin.Context) {
    whoisManager := c.MustGet("whoisManager").(*services.WhoisManager)
    // 使用服务
}
```

###  分布式限流

限流中间件用于防止API过度调用，使用Redis实现分布式的限流机制：

```go
// 基本限流：每分钟60个请求
apiGroup.Use(middleware.RateLimitMiddleware(serviceContainer.Limiter, 60, time.Minute))

// 针对不同端点的限流配置
screenshotGroup.Use(middleware.RateLimitMiddleware(limiter, 10, time.Minute))  // 截图较少频率
whoisGroup.Use(middleware.RateLimitMiddleware(limiter, 100, time.Minute))      // WHOIS较高频率
```

**限流特性：**
- **分布式支持** - 基于Redis，支持多实例部署
- **灵活配置** - 支持不同端点不同限流策略
- **IP级别限流** - 基于客户端IP地址限流
- **优雅降级** - 超限时返回429状态码和重试时间

###  安全中间件套件

#### JWT认证中间件
```go
// JWT认证配置
authConfig := middleware.AuthConfig{
    SecretKey:      os.Getenv("JWT_SECRET"),
    TokenLookup:    "header:Authorization",
    TokenHeadName:  "Bearer",
    TimeFunc:       time.Now,
    Timeout:        time.Hour * 24,
    MaxRefresh:     time.Hour * 24,
}

authGroup.Use(middleware.JWTAuthMiddleware(authConfig))
```

#### IP白名单中间件
```go
// 严格模式：只允许白名单IP
strictConfig := middleware.IPWhitelistConfig{
    AllowedIPs:   []string{"127.0.0.1", "192.168.1.0/24"},
    StrictMode:   true,
    HeaderName:   "X-Real-IP",
}

// 宽松模式：记录但不阻止
looseConfig := middleware.IPWhitelistConfig{
    AllowedIPs:   []string{"10.0.0.0/8"},
    StrictMode:   false,
    LogBlocked:   true,
}

api.Use(middleware.IPWhitelistMiddleware(strictConfig))
```

#### 安全头中间件
```go
// 自动设置安全HTTP头
r.Use(middleware.SecurityHeadersMiddleware())

// 设置的安全头包括：
// - X-Content-Type-Options: nosniff
// - X-Frame-Options: DENY
// - X-XSS-Protection: 1; mode=block
// - Strict-Transport-Security: max-age=31536000
// - Content-Security-Policy: default-src 'self'
```

### 监控和日志中间件

#### 性能监控中间件
```go
// 收集请求统计信息
r.Use(middleware.MonitoringMiddleware(serviceContainer.StatsCollector))

// 监控指标包括：
// - 请求数量和频率
// - 响应时间分布
// - 错误率统计
// - 状态码分布
// - 路径访问统计
```

#### 日志中间件
```go
// 结构化日志记录
logConfig := middleware.LoggingConfig{
    TimeFormat:      time.RFC3339,
    UTC:            true,
    SkipPaths:      []string{"/health", "/metrics"},
    EnableColors:   false,
    LogLatency:     true,
    LogUserAgent:   true,
    LogReferer:     true,
}

r.Use(middleware.LoggingMiddleware(logConfig))
```

###  请求验证和限制

#### 请求大小限制
```go
// 限制请求体大小（防止大文件攻击）
r.Use(middleware.SizeLimitMiddleware(10 * 1024 * 1024)) // 10MB限制

// 针对不同端点的不同限制
uploadGroup.Use(middleware.SizeLimitMiddleware(100 * 1024 * 1024)) // 100MB
apiGroup.Use(middleware.SizeLimitMiddleware(1 * 1024 * 1024))       // 1MB
```

#### 参数验证中间件
```go
// 自动验证请求参数
type DomainRequest struct {
    Domain string `json:"domain" validate:"required,domain"`
    Type   string `json:"type" validate:"required,oneof=basic element"`
}

// 在路由中使用
apiGroup.POST("/screenshot",
    middleware.ValidationMiddleware(&DomainRequest{}),
    handlers.ScreenshotHandler,
)
```

### CORS中间件

```go
// CORS配置
corsConfig := middleware.CorsConfig{
    AllowOrigins:     []string{"https://example.com", "https://app.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
    ExposeHeaders:    []string{"Content-Length"},
    AllowCredentials: true,
    MaxAge:           12 * time.Hour,
}

r.Use(middleware.CorsMiddleware(corsConfig))
```

##  截图服务专用中间件配置

针对重构后的截图服务，推荐使用以下中间件配置：

```go
// 截图服务中间件栈
screenshotGroup := apiv1.Group("/screenshot")
{
    // 1. 域名验证中间件
    screenshotGroup.Use(domainValidationMiddleware())

    // 2. 限流中间件（较低频率，因为截图操作耗时）
    screenshotGroup.Use(rateLimitMiddleware(serviceContainer.Limiter, 10, time.Minute))

    // 3. 异步工作中间件（支持长时间运行的截图任务）
    screenshotGroup.Use(asyncWorkerMiddleware(serviceContainer.WorkerPool, 120*time.Second))

    // 4. 请求大小限制（防止过大的截图请求）
    screenshotGroup.Use(middleware.SizeLimitMiddleware(1 * 1024 * 1024))

    // 5. 监控中间件
    screenshotGroup.Use(middleware.MonitoringMiddleware(serviceContainer.StatsCollector))
}

// 兼容API的Redis中间件
func addRedisMiddleware(serviceContainer *services.ServiceContainer) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Set("redis", serviceContainer.RedisClient)
        c.Next()
    }
}
```

## 错误处理中间件

统一错误处理中间件负责捕获和标准化应用程序中的错误：

```go
// 错误处理中间件
r.Use(middleware.ErrorMiddleware())

// 支持的错误类型：
// - HTTP错误（400, 401, 403, 404, 500等）
// - 业务逻辑错误
// - 验证错误
// - 超时错误
// - 限流错误

// 错误响应格式：
{
  "success": false,
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "请求参数验证失败"
  },
  "meta": {
    "timestamp": "2024-01-20T10:30:00Z",
    "requestId": "req-123456"
  }
}
```

## 中间件注册顺序 ⚠️

中间件的注册顺序非常重要，推荐顺序：

```go
// 1. 错误恢复和日志（最先执行）
r.Use(gin.Recovery())
r.Use(middleware.LoggingMiddleware())

// 2. 安全头和CORS
r.Use(middleware.SecurityHeadersMiddleware())
r.Use(middleware.CorsMiddleware())

// 3. 请求限制
r.Use(middleware.SizeLimitMiddleware(10 * 1024 * 1024))

// 4. 服务注入（必须在业务逻辑之前）
r.Use(middleware.ServiceMiddleware(serviceContainer))

// 5. API组级别的中间件
apiGroup := r.Group("/api/v1")
{
    // 认证和授权
    apiGroup.Use(middleware.IPWhitelistMiddleware(ipConfig))
    apiGroup.Use(middleware.JWTAuthMiddleware(authConfig))

    // 限流和验证
    apiGroup.Use(middleware.RateLimitMiddleware(limiter, 60, time.Minute))
    apiGroup.Use(middleware.ValidationMiddleware())

    // 监控（最后执行，获得完整的请求信息）
    apiGroup.Use(middleware.MonitoringMiddleware(statsCollector))
}

// 6. 错误处理（最后注册，捕获所有错误）
r.Use(middleware.ErrorMiddleware())
```

## 自定义中间件开发

创建自定义中间件的模板：

```go
// 自定义中间件示例
func CustomMiddleware(config CustomConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 前置处理
        startTime := time.Now()

        // 验证逻辑
        if !validateRequest(c, config) {
            c.JSON(400, gin.H{"error": "验证失败"})
            c.Abort()
            return
        }

        // 设置上下文数据
        c.Set("startTime", startTime)

        // 执行下一个中间件/处理器
        c.Next()

        // 后置处理
        duration := time.Since(startTime)
        log.Printf("请求耗时: %v", duration)

        // 清理资源
        cleanup(c)
    }
}
```

## 性能优化建议

### 中间件性能优化
1. **减少中间件数量** - 只使用必要的中间件
2. **优化执行顺序** - 将快速失败的中间件放在前面
3. **缓存计算结果** - 避免重复计算
4. **使用连接池** - Redis、数据库连接复用
5. **异步处理** - 非关键路径的处理异步化

### 内存优化
```go
// 避免在中间件中创建大对象
func BadMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        largeMap := make(map[string]string, 10000) // 每次请求都创建
        // ...
    }
}

// 推荐做法：使用对象池或全局变量
var mapPool = sync.Pool{
    New: func() interface{} {
        return make(map[string]string, 100)
    },
}

func GoodMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        m := mapPool.Get().(map[string]string)
        defer func() {
            // 清空并归还对象池
            for k := range m {
                delete(m, k)
            }
            mapPool.Put(m)
        }()
        // ...
    }
}
```

## 最佳实践总结

1. **安全第一** - 始终将安全中间件放在前面
2. **错误处理** - 使用统一的错误处理中间件
3. **性能监控** - 在生产环境启用监控中间件
4. **合理限流** - 根据业务特点设置合理的限流策略
5. **日志记录** - 记录关键信息但避免敏感数据
6. **配置化** - 中间件参数支持配置文件配置
7. **测试覆盖** - 为自定义中间件编写单元测试

## 中间件配置示例

完整的生产环境中间件配置：

```go
func SetupMiddleware(r *gin.Engine, serviceContainer *services.ServiceContainer) {
    // 基础中间件
    r.Use(gin.Recovery())
    r.Use(middleware.LoggingMiddleware(logConfig))
    r.Use(middleware.SecurityHeadersMiddleware())
    r.Use(middleware.CorsMiddleware(corsConfig))
    r.Use(middleware.SizeLimitMiddleware(10 * 1024 * 1024))

    // 服务注入
    r.Use(middleware.ServiceMiddleware(serviceContainer))

    // API路由组
    apiv1 := r.Group("/api/v1")
    apiv1.Use(middleware.IPWhitelistMiddleware(ipConfig))
    apiv1.Use(middleware.RateLimitMiddleware(serviceContainer.Limiter, 60, time.Minute))
    apiv1.Use(middleware.MonitoringMiddleware(serviceContainer.StatsCollector))

    // 截图服务专用配置
    screenshotGroup := apiv1.Group("/screenshot")
    screenshotGroup.Use(middleware.RateLimitMiddleware(serviceContainer.Limiter, 10, time.Minute))
    screenshotGroup.Use(middleware.AsyncWorkerMiddleware(serviceContainer.WorkerPool, 120*time.Second))

    // 错误处理（最后）
    r.Use(middleware.ErrorMiddleware())
}
