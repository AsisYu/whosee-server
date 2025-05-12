# 路由目录 (Routes)

## 目录作用

路由目录管理应用程序的API端点定义、路由分组和中间件配置。它是组织所有HTTP路由及其相关处理程序的中心地点，使API结构清晰易维护。

## 文件列表与功能

- `routes.go` - 主路由配置和注册
- `api.go` - API特定路由配置

## 路由注册

`RegisterRoutes`函数在`routes.go`中是所有路由注册的入口点：

```go
func RegisterRoutes(r *gin.Engine, serviceContainer *services.ServiceContainer) {
    // 注册API路由
    RegisterAPIRoutes(r, serviceContainer)
    
    // 注册健康检查路由
    RegisterHealthRoutes(r, serviceContainer)
    
    // 其他路由组可以在这里注册
}
```

## 中间件集成

路由包配置并附加中间件到路由组。常见的中间件包括：

1. **服务中间件** - 将服务组件注入到请求上下文中
2. **域名验证** - 在处理前验证域名参数
3. **限流** - 基于IP和/或域名限制请求速率
4. **CORS配置** - 处理跨源资源共享

```go
// 中间件配置示例
api := r.Group("/api/v1")
api.Use(middleware.ServiceMiddleware(serviceContainer))
api.Use(domainValidationMiddleware())
api.Use(rateLimitMiddleware(serviceContainer.Limiter))
```

## API版本化

路由包通过URL路径前缀实现API版本化：

- 旧版API：`/api/...`
- 版本1 API：`/api/v1/...`

这允许在引入新的端点版本的同时保持向后兼容性。

## URL参数和查询字符串

路由定义指定了如何提取参数：

```go
// URL参数示例
api.GET("/whois/:domain", handlers.WhoisQuery)

// 查询字符串示例
api.GET("/dns", handlers.DNSQuery) // 从查询字符串提取域名
```

## 请求流程

路由包编排的典型请求流程是：

1. 请求到达定义的端点
2. 路由特定的中间件处理请求
3. 请求被委托给适当的处理程序
4. 处理程序处理请求并返回响应

## 设计原则

1. **集中路由** - 所有路由在一个位置定义
2. **逻辑分组** - 路由按逻辑分组组织
3. **一致命名** - 路由命名遵循一致的模式
4. **最小重复** - 在组级别应用通用中间件

## 高并发支持

路由层已经优化以支持高并发场景，包括：

1. **分布式限流** - 基于Redis的跨多个服务实例的限流
2. **请求过滤** - 在无效请求消耗资源前提前拒绝
3. **异步处理** - 非阻塞API处理模式
4. **增强型健康检查** - 所有服务的统一健康检查端点
