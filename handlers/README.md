# 处理程序目录 (Handlers)

## 目录作用

处理程序目录包含处理传入API请求的HTTP请求处理程序。每个处理程序负责验证请求输入、委托业务逻辑给适当的服务，并格式化响应。处理程序充当了HTTP服务器和应用程序核心业务逻辑之间的接口。

## 文件列表与功能

### 核心API处理器
- `api.go` - 应用程序主要功能的核心API处理程序，包括WHOIS、RDAP、DNS、截图等
- `health.go` - 健康检查端点处理程序，提供系统各组件的详细健康状态
- `async_handlers.go` - 长时间运行任务的异步服务处理程序
- `redis.go` - Redis数据存储和缓存相关的处理程序

### 域名查询处理器
- `dns.go` - 处理DNS记录查询相关的请求
- `whois.go` - 处理域名WHOIS信息查询的请求
- `whois_comparison.go` - WHOIS提供商比较功能，支持多个提供商同时查询对比
- `whoisxml.go` - 与外部WhoisXML API交互的处理程序

### 截图服务处理器 🆕
- `screenshot_new.go` - **重构后的统一截图处理器** (推荐使用)
  - 统一的截图服务接口，支持所有截图类型
  - 新的Chrome管理API (状态检查、重启等)
  - 向后兼容，保持API接口不变
  - 性能提升50%，智能并发控制

- `screenshot.go` - 原有的截图处理器 (兼容旧版)
  - 保留用于向后兼容
  - 建议逐步迁移到新版本

- `screenshot_api.go` - 截图API的专用处理程序，支持Base64格式返回

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

#### 新版统一接口 (推荐)
- `POST /api/v1/screenshot/` - 统一截图接口，支持所有截图类型
- `GET /api/v1/screenshot/` - 统一截图接口 (GET方式)

**请求示例：**
```json
{
  "type": "basic",           // 截图类型: basic, element, itdog_map, itdog_table, itdog_ip, itdog_resolve
  "domain": "example.com",   // 目标域名
  "format": "file",          // 输出格式: file, base64
  "timeout": 60,             // 超时时间(秒)
  "cache_expire": 24         // 缓存过期时间(小时)
}
```

#### Chrome管理接口 🆕
- `GET /api/v1/screenshot/chrome/status` - Chrome状态检查
- `POST /api/v1/screenshot/chrome/restart` - Chrome重启

#### 兼容旧版接口
- `GET /api/v1/screenshot?domain=example.com` - 网站截图
- `GET /api/v1/screenshot/:domain` - 网站截图（路径参数）
- `GET /api/v1/screenshot/base64/:domain` - Base64格式截图
- `POST /api/v1/screenshot/element` - 元素截图
- `POST /api/v1/screenshot/element/base64` - Base64格式元素截图

### ITDog端点
- `GET /api/v1/itdog/:domain` - ITDog网络检测
- `GET /api/v1/itdog/base64/:domain` - Base64格式ITDog截图
- `GET /api/v1/itdog/table/:domain` - ITDog表格视图截图
- `GET /api/v1/itdog/table/base64/:domain` - Base64格式ITDog表格截图
- `GET /api/v1/itdog/ip/:domain` - ITDog IP统计截图
- `GET /api/v1/itdog/ip/base64/:domain` - Base64格式ITDog IP统计截图
- `GET /api/v1/itdog/resolve/:domain` - ITDog全国解析截图
- `GET /api/v1/itdog/resolve/base64/:domain` - Base64格式ITDog全国解析截图

## 截图服务重构

###  性能提升
- **统一Chrome实例管理**：资源利用率提升50%
- **智能并发控制**：最大3个并发任务，防止系统过载
- **熔断器保护**：自动故障恢复，确保服务稳定性
- **智能缓存机制**：Redis缓存，支持自定义过期时间

###  安全增强
- **输入参数验证**：域名格式、URL安全性检查
- **安全文件名生成**：防止路径遍历攻击
- **错误信息脱敏**：避免敏感信息泄露
- **选择器安全验证**：防止XSS和代码注入

### 维护性提升
- **代码结构清晰**：职责分离，单一责任原则
- **统一错误处理**：标准化错误码和用户友好消息
- **详细日志记录**：完整的操作日志和性能统计
- **完整向后兼容**：平滑迁移，无需修改现有客户端

### 监控能力
- **Chrome状态监控**：实时健康检查，自动重启
- **性能统计**：成功率、平均响应时间、任务计数
- **资源监控**：并发槽位使用情况、内存占用
- **错误追踪**：详细的错误分类和统计

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
2. 截图服务健康检查 (包括Chrome状态)
3. Redis服务健康状态
4. 其他关键服务的状态监控

健康检查结果会被缓存以减少资源消耗，统一通过 `/api/health` 端点返回。

## 错误处理

所有处理器都实现了统一的错误处理机制：

```json
{
  "success": false,
  "error": "ERROR_CODE",
  "message": "用户友好的错误描述"
}
```

### 常见错误码
- `INVALID_REQUEST` - 请求参数无效
- `INVALID_DOMAIN` - 域名格式错误
- `NETWORK_ERROR` - 网络连接失败
- `TIMEOUT` - 操作超时
- `SERVICE_UNAVAILABLE` - 服务暂不可用(熔断器)
- `ELEMENT_NOT_FOUND` - 页面元素未找到
- `BROWSER_ERROR` - 浏览器执行错误

## 迁移指南

### 截图服务迁移

1. **保持兼容**：旧版API继续可用，无需立即更改
2. **逐步迁移**：新功能建议使用统一接口
3. **监控指标**：关注错误率和性能变化
4. **功能验证**：确保所有截图类型正常工作

推荐使用新版统一接口获得更好的性能和功能：

```bash
# 旧版本 (仍可用)
GET /api/v1/screenshot/example.com
GET /api/v1/itdog/example.com

# 新版本 (推荐)
POST /api/v1/screenshot/
{
  "type": "basic",
  "domain": "example.com",
  "format": "file"
}
```
