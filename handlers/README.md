# 处理程序目录 (Handlers)

## 目录作用

处理程序目录包含处理传入API请求的HTTP请求处理程序。每个处理程序负责验证请求输入、委托业务逻辑给适当的服务，并格式化响应。处理程序充当了HTTP服务器和应用程序核心业务逻辑之间的接口。

## 文件列表与功能

- `api.go` - 应用程序主要功能的核心API处理程序，包括WHOIS、RDAP、DNS、截图等
- `health.go` - 健康检查端点处理程序，提供系统各组件的详细健康状态
- `async_handlers.go` - 长时间运行任务的异步服务处理程序
- `dns.go` - 处理DNS记录查询相关的请求
- `whois.go` - 处理域名WHOIS信息查询的请求
- `whois_comparison.go` - WHOIS提供商比较功能，支持多个提供商同时查询对比
- `whoisxml.go` - 与外部WhoisXML API交互的处理程序
- `screenshot.go` - 管理网站截图功能，支持实时显示加载状态和错误处理
- `screenshot_api.go` - 截图API的专用处理程序，支持Base64格式返回
- `redis.go` - Redis数据存储和缓存相关的处理程序

## API端点说明

### WHOIS查询端点
- `GET /api/v1/whois?domain=example.com` - 通用WHOIS查询（自动选择最优提供商）
- `GET /api/v1/whois/:domain` - 通用WHOIS查询（路径参数）
- `GET /api/v1/whois/compare/:domain` - 多提供商WHOIS对比查询
- `GET /api/v1/whois/providers` - 获取可用WHOIS提供商信息

### RDAP查询端点 🆕
- `GET /api/v1/rdap?domain=example.com` - RDAP协议查询（专用IANA-RDAP提供商）
- `GET /api/v1/rdap/:domain` - RDAP协议查询（路径参数）

RDAP (Registration Data Access Protocol) 是WHOIS的现代化替代协议，提供：
- **标准化JSON格式响应**
- **更好的国际化支持**
- **增强的安全性和隐私保护**
- **RESTful API设计**
- **严格的数据结构标准**

### DNS查询端点
- `GET /api/v1/dns?domain=example.com` - DNS记录查询
- `GET /api/v1/dns/:domain` - DNS记录查询（路径参数）

### 截图端点
- `GET /api/v1/screenshot?domain=example.com` - 网站截图
- `GET /api/v1/screenshot/:domain` - 网站截图（路径参数）
- `GET /api/v1/screenshot/base64/:domain` - Base64格式截图

### ITDog端点
- `GET /api/v1/itdog/:domain` - ITDog网络检测
- `GET /api/v1/itdog/base64/:domain` - Base64格式ITDog截图
- `GET /api/v1/itdog/table/:domain` - ITDog表格视图截图
- `GET /api/v1/itdog/ip/:domain` - ITDog IP统计截图
- `GET /api/v1/itdog/resolve/:domain` - ITDog全国解析截图

## 请求处理流程

处理程序遵循一致的模式：

1. **从 URL、查询字符串、正文等提取请求参数**
2. **验证输入**的正确性和安全性
3. **从上下文中检索服务**（由中间件注入）
4. **委托给服务层**进行业务逻辑处理
5. **格式化响应**使用标准响应工具

## 异步处理模式

对于长时间运行的操作，我们使用异步模式：

1. 处理程序接收请求并将其传递给工作池
2. 处理程序立即返回任务ID或状态
3. 客户端可以使用任务ID查询操作状态

## 高并发优化

处理程序层实现了多项高并发优化：

1. **工作池模式** - 基于CPU核心数动态调整的请求处理池
2. **分布式限流器** - 使用Redis实现的请求限流机制
3. **熔断器模式** - 防止系统在故障条件下过载
4. **全异步API处理** - 非阻塞请求处理模式

## 健康检查系统

`health.go` 实现了增强的健康检查API，包括：

1. DNS服务健康检查
2. 截图服务健康检查
3. Redis服务健康状态
4. 其他关键服务的状态监控

健康检查结果会被缓存以减少资源消耗，统一通过 `/api/health` 端点返回。
