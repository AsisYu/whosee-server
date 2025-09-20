# 路由目录 (Routes)

## 目录作用

路由目录管理应用程序的API端点定义、路由分组和中间件配置。它是组织所有HTTP路由及其相关处理程序的中心地点，使API结构清晰易维护。

## 文件列表与功能

### 核心路由文件
- `routes.go` - 主路由配置和注册
- `api.go` - API特定路由配置

### 专门化路由文件 🆕
- `screenshot_routes.go` - **重构后的截图服务路由**
  - 新版统一截图API路由配置
  - Chrome管理API路由
  - 向后兼容的旧版API路由
  - 优化的中间件配置

##  截图路由重构亮点

### 新版路由架构

```go
func RegisterScreenshotRoutes(r *gin.Engine, serviceContainer *services.ServiceContainer) {
    // 创建截图服务实例
    chromeManager := services.GetGlobalChromeManager()
    screenshotService := services.NewScreenshotService(chromeManager, serviceContainer.RedisClient, nil)
    screenshotHandler := handlers.NewUnifiedScreenshotHandler(screenshotService, chromeManager)

    apiv1 := r.Group("/api/v1")

    // 新的统一截图API (推荐使用)
    screenshotGroup := apiv1.Group("/screenshot")
    {
        // 统一截图接口 - 支持所有截图类型
        screenshotGroup.POST("/", screenshotHandler.TakeScreenshot)
        screenshotGroup.GET("/", screenshotHandler.TakeScreenshot)

        // Chrome管理接口
        screenshotGroup.GET("/chrome/status", handlers.NewChromeStatus)
        screenshotGroup.POST("/chrome/restart", handlers.NewChromeRestart)
    }

    // 兼容旧版API路由 (保持向后兼容)
    compatGroup := apiv1.Group("/")
    {
        // 基础截图兼容路由
        compatGroup.GET("screenshot/:domain", handlers.NewScreenshotRouteHandler)
        compatGroup.GET("screenshot", handlers.NewScreenshotRouteHandler)

        // 元素截图兼容路由
        compatGroup.POST("screenshot/element", handlers.NewElementScreenshotHandler)
        compatGroup.POST("screenshot/element/base64", handlers.NewElementScreenshotBase64Handler)

        // ITDog截图兼容路由
        compatGroup.GET("itdog/:domain", handlers.NewITDogHandler)
        compatGroup.GET("itdog/base64/:domain", handlers.NewITDogBase64Handler)
        // ... 更多ITDog路由
    }
}
```

### 路由优化特性

1. **统一接口设计** - 单一端点支持所有截图类型
2. **智能路由分组** - 新版和兼容版本分离
3. **Chrome管理API** - 专门的Chrome状态和管理端点
4. **优化中间件栈** - 针对截图服务优化的中间件配置

## 路由注册

`RegisterRoutes`函数在`routes.go`中是所有路由注册的入口点：

```go
func RegisterRoutes(r *gin.Engine, serviceContainer *services.ServiceContainer) {
    // 注册API路由
    RegisterAPIRoutes(r, serviceContainer)

    // 注册截图服务路由 🆕
    RegisterScreenshotRoutes(r, serviceContainer)

    // 注册健康检查路由
    RegisterHealthRoutes(r, serviceContainer)

    // 其他路由组可以在这里注册
}
```

## API端点组织

### 核心API组 (`/api/v1/`)
```go
// 域名查询服务
api.GET("/whois/:domain", handlers.WhoisQuery)
api.GET("/whois", handlers.WhoisQuery)
api.GET("/rdap/:domain", handlers.RDAPQuery)
api.GET("/dns/:domain", handlers.DNSQuery)

// 健康检查
api.GET("/health", handlers.HealthCheck)
```

### 截图服务组 (`/api/v1/screenshot/`) 🆕
```go
// 新版统一接口
screenshotGroup.POST("/", screenshotHandler.TakeScreenshot)
screenshotGroup.GET("/", screenshotHandler.TakeScreenshot)

// Chrome管理
screenshotGroup.GET("/chrome/status", handlers.NewChromeStatus)
screenshotGroup.POST("/chrome/restart", handlers.NewChromeRestart)
```

### 兼容API组 (向后兼容)
```go
// 基础截图
compatGroup.GET("screenshot/:domain", handlers.NewScreenshotRouteHandler)
compatGroup.GET("screenshot/base64/:domain", handlers.NewScreenshotBase64Handler)

// 元素截图
compatGroup.POST("screenshot/element", handlers.NewElementScreenshotHandler)
compatGroup.POST("screenshot/element/base64", handlers.NewElementScreenshotBase64Handler)

// ITDog系列
compatGroup.GET("itdog/:domain", handlers.NewITDogHandler)
compatGroup.GET("itdog/base64/:domain", handlers.NewITDogBase64Handler)
compatGroup.GET("itdog/table/:domain", handlers.NewITDogTableHandler)
compatGroup.GET("itdog/ip/:domain", handlers.NewITDogIPHandler)
compatGroup.GET("itdog/resolve/:domain", handlers.NewITDogResolveHandler)
```

## 中间件集成

路由包配置并附加中间件到路由组。常见的中间件包括：

### 基础中间件
1. **服务中间件** - 将服务组件注入到请求上下文中
2. **域名验证** - 在处理前验证域名参数
3. **限流** - 基于IP和/或域名限制请求速率
4. **CORS配置** - 处理跨源资源共享

### 截图服务专用中间件 🆕
```go
// 截图服务中间件栈
screenshotGroup.Use(domainValidationMiddleware())
screenshotGroup.Use(rateLimitMiddleware(serviceContainer.Limiter))
screenshotGroup.Use(asyncWorkerMiddleware(serviceContainer.WorkerPool, 120*time.Second))

// Redis中间件 (兼容路由需要)
func addRedisMiddleware(serviceContainer *services.ServiceContainer) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Set("redis", serviceContainer.RedisClient)
        c.Next()
    }
}
```

## API版本化

路由包通过URL路径前缀实现API版本化：

- **版本1 API**: `/api/v1/...` (当前主版本)
- **新版截图API**: `/api/v1/screenshot/...` 🆕
- **兼容API**: `/api/v1/screenshot/:domain`, `/api/v1/itdog/:domain` 等

这允许在引入新的端点版本的同时保持向后兼容性。

## 请求流程

### 新版统一截图API流程 
```
1. POST /api/v1/screenshot/
   ↓
2. domainValidationMiddleware() - 验证请求参数
   ↓
3. rateLimitMiddleware() - 检查请求限流
   ↓
4. asyncWorkerMiddleware() - 异步任务处理
   ↓
5. screenshotHandler.TakeScreenshot() - 统一处理器
   ↓
6. screenshotService.TakeScreenshot() - 业务逻辑
   ↓
7. chromeManager.GetContext() - Chrome资源管理
   ↓
8. 返回统一响应格式
```

### 传统API流程
```
1. 请求到达定义的端点
   ↓
2. 路由特定的中间件处理请求
   ↓
3. 请求被委托给适当的处理程序
   ↓
4. 处理程序处理请求并返回响应
```

## URL参数和查询字符串

### 新版统一API
```go
// POST请求体
{
  "type": "basic|element|itdog_map|itdog_table|itdog_ip|itdog_resolve",
  "domain": "example.com",
  "url": "https://example.com",        // 可选，优先级高于domain
  "selector": ".main-content",         // 元素截图必需
  "format": "file|base64",
  "timeout": 60,                       // 秒
  "wait_time": 3,                      // 秒
  "cache_expire": 24                   // 小时
}
```

### 兼容API
```go
// URL参数示例
api.GET("/screenshot/:domain", handlers.NewScreenshotRouteHandler)
api.GET("/itdog/:domain", handlers.NewITDogHandler)

// 查询字符串示例
api.GET("/screenshot", handlers.NewScreenshotRouteHandler) // ?domain=example.com
```

## 错误处理和响应格式

### 统一响应格式
```json
{
  "success": true,
  "image_url": "/static/screenshots/basic_example_com_1642723200.png",
  "from_cache": false,
  "metadata": {
    "size": 45234,
    "type": "basic",
    "description": "基础截图"
  }
}
```

### 错误响应
```json
{
  "success": false,
  "error": "INVALID_DOMAIN",
  "message": "域名格式错误"
}
```

## 设计原则

1. **集中路由** - 所有路由在统一位置定义
2. **逻辑分组** - 路由按功能模块分组组织
3. **一致命名** - 路由命名遵循RESTful模式
4. **最小重复** - 在组级别应用通用中间件
5. **向后兼容** - 新版本不破坏现有API 🆕
6. **性能优化** - 路由级别的性能优化 🆕

## 高并发支持

路由层已经优化以支持高并发场景，包括：

1. **分布式限流** - 基于Redis的跨多个服务实例的限流
2. **请求过滤** - 在无效请求消耗资源前提前拒绝
3. **异步处理** - 非阻塞API处理模式
4. **增强型健康检查** - 所有服务的统一健康检查端点
5. **Chrome资源池** - 统一Chrome实例管理，防止资源过载 🆕

## 监控和调试

### Chrome管理API 🆕
```bash
# 检查Chrome状态
GET /api/v1/screenshot/chrome/status

# 重启Chrome实例
POST /api/v1/screenshot/chrome/restart
```

### 响应示例
```json
{
  "success": true,
  "chrome_status": {
    "is_running": true,
    "is_healthy": true,
    "current_tasks": 1,
    "max_concurrent": 3,
    "available_slots": 2,
    "total_tasks": 156,
    "success_rate": 98.7,
    "avg_duration_ms": 2341,
    "uptime_seconds": 3600
  }
}
```

## 迁移指南

### 从旧版API迁移到新版

```bash
# 旧版本
GET /api/v1/screenshot/example.com
GET /api/v1/itdog/example.com

# 新版本 (推荐)
POST /api/v1/screenshot/
{
  "type": "basic",
  "domain": "example.com",
  "format": "file"
}

POST /api/v1/screenshot/
{
  "type": "itdog_map",
  "domain": "example.com",
  "format": "file"
}
```

### 渐进式迁移策略

1. **阶段1**: 部署新版本，保持所有旧API工作
2. **阶段2**: 新功能使用新API，监控性能指标
3. **阶段3**: 逐步迁移现有客户端到新API
4. **阶段4**: 根据使用情况决定是否废弃旧API
