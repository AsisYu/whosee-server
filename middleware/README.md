# 中间件目录 (Middleware)

## 目录作用

中间件目录包含在HTTP请求处理过程中执行的中间件组件。这些组件负责跨切关注点，如认证、验证、限流、性能监控等，它们在请求达到实际的业务处理程序之前执行。

## 文件列表与功能

- `service.go` - 服务注入中间件，将服务组件注入到Gin上下文
- `auth.go` - 认证中间件，处理用户令牌和权限
- `ratelimit.go` - 限流中间件，防止请求泛滥
- `logging.go` - 日志中间件，记录请求和响应信息
- `error.go` - 错误处理中间件，统一错误处理机制
- `cors.go` - CORS中间件，处理跨域请求
- `ip_whitelist.go` - IP白名单中间件，限制IP访问
- `monitoring.go` - 监控中间件，监控应用性能
- `security.go` - 安全中间件，增强应用安全
- `sizelimit.go` - 请求大小限制中间件，限制请求大小
- `validation.go` - 请求验证中间件，验证请求参数

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

## 主要中间件

### 服务中间件

`middleware.ServiceMiddleware`是核心中间件，它将服务容器中的所有服务注入到Gin请求上下文中，使得处理程序可以访问到这些服务：

```go
// 注册服务中间件
r.Use(middleware.ServiceMiddleware(serviceContainer))
```

### 分布式限流

限流中间件用于防止API过度调用，使用Redis实现分布式的限流机制：

```go
// 限流中间件示例
apiGroup.Use(middleware.RateLimitMiddleware(serviceContainer.Limiter, 60, time.Minute))
```

### 统一错误处理

错误处理中间件负责捕获和标准化应用程序中的错误，确保客户端始终收到一致的错误响应格式：

```go
// 错误处理中间件
r.Use(middleware.ErrorMiddleware())
```

### CORS中间件

CORS中间件用于处理跨域请求：

```go
// CORS中间件示例
r.Use(middleware.CorsMiddleware(corsConfig))
```

### IP白名单中间件

IP白名单中间件用于限制IP访问：

```go
// IP白名单中间件示例
apiGroup.Use(middleware.IPWhitelistMiddleware([]string{"127.0.0.1", "192.168.1.0/24"}))
```

### 监控中间件

监控中间件用于监控应用性能：

```go
// 监控中间件示例
r.Use(middleware.MonitoringMiddleware(serviceContainer.StatsCollector))
```

### 安全中间件

安全中间件用于增强应用安全：

```go
// 安全中间件示例
r.Use(middleware.SecurityHeadersMiddleware())
```

### 请求大小限制中间件

请求大小限制中间件用于限制请求大小：

```go
// 请求大小限制中间件示例
r.Use(middleware.SizeLimitMiddleware(10 * 1024 * 1024))
```

### 请求验证中间件

请求验证中间件用于验证请求参数：

```go
// 请求验证中间件示例
apiGroup.Use(middleware.ValidationMiddleware())
```

## 使用指南

中间件可以在不同级别应用：

```go
// 全局中间件
r.Use(middleware.Logger())

// 路由组中间件
apiGroup := r.Group("/api")
apiGroup.Use(middleware.Auth())

// 单一路由中间件
r.GET("/special", middleware.SpecialMiddleware(), handlers.SpecialHandler)
```

## 最佳实践

以下是中间件使用的最佳实践：

```go
// 1. 首先注册必须的中间件
r.Use(middleware.ErrorMiddleware())         // 错误处理中间件
r.Use(middleware.LoggingMiddleware())       // 日志中间件
r.Use(middleware.RecoveryMiddleware())      // 恢复中间件
r.Use(middleware.SecurityHeadersMiddleware()) // 安全中间件

// 2. 然后注册请求处理相关的中间件
r.Use(middleware.SizeLimitMiddleware(10 * 1024 * 1024)) // 请求大小限制中间件
r.Use(middleware.TimeoutMiddleware(30 * time.Second))    // 请求超时中间件
r.Use(middleware.CorsMiddleware())                       // CORS中间件

// 3. API路由组中注册相关的中间件
apiGroup := r.Group("/api")
apiGroup.Use(middleware.IPWhitelistMiddleware())          // IP白名单中间件
apiGroup.Use(middleware.RateLimitMiddleware())           // 限流中间件
apiGroup.Use(middleware.AuthMiddleware())                // 认证中间件
apiGroup.Use(middleware.ValidationMiddleware())          // 请求验证中间件
apiGroup.Use(middleware.MonitoringMiddleware())          // 监控中间件
