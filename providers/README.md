# 提供商目录（Providers）

## 目录作用

提供商目录包含与第三方服务或API连接的封装组件，主要用于外部服务集成。在Whosee.me项目中，这些提供商特别用于WHOIS信息查询的各种数据源。

## 文件列表与功能

- `whoisfreaks_provider.go` - WhoisFreaks服务的集成封装
- `whoisxml_provider.go` - WhoisXML服务的集成封装

## 提供商接口

所有的提供商实现了共同的接口，这使得它们可以方便地交换或组合使用：

```go
type WHOISProvider interface {
    // 返回提供商的名称
    GetName() string
    
    // 查询域名的WHOIS信息
    Query(domain string) (*types.WhoisResponse, error)
    
    // 检查提供商是否可用
    IsAvailable() bool
}
```

## 提供商管理

提供商通过`WhoisManager`注册和管理：

```go
// 创建并注册提供商
whoisFreaksProvider := providers.NewWhoisFreaksProvider()
whoisXMLProvider := providers.NewWhoisXMLProvider()

// 将提供商添加到管理器
serviceContainer.WhoisManager.AddProvider(whoisFreaksProvider)
serviceContainer.WhoisManager.AddProvider(whoisXMLProvider)
```

## 弹性失败处理

提供商组件集成了熔断器弹性机制，当第三方服务不可用时自动故障转移：

1. 如果一个提供商失败，系统会自动尝试下一个提供商
2. 失败的提供商会进入熔断状态，一段时间后才会重试
3. 成功的查询会被缓存以减少对外部服务的依赖

## 错误处理

提供商实现了一致的错误处理模式：

```go
func (p *WhoisXMLProvider) Query(domain string) (*types.WhoisResponse, error) {
    // 尝试API调用
    resp, err := p.client.Get(fmt.Sprintf("%s?apiKey=%s&domainName=%s", p.apiURL, p.apiKey, domain))
    
    // 处理连接错误
    if err != nil {
        return nil, fmt.Errorf("WhoisXML provider connection error: %w", err)
    }
    
    // 处理API特定错误
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("WhoisXML provider returned status code %d", resp.StatusCode)
    }
    
    // 解析并返回响应
    // ...
}
```

## 配置

提供商通过环境变量配置，保持API密钥等敏感信息的安全：

```go
func NewWhoisFreaksProvider() *WhoisFreaksProvider {
    return &WhoisFreaksProvider{
        apiKey: os.Getenv("WHOISFREAKS_API_KEY"),
        apiURL: os.Getenv("WHOISFREAKS_API_URL"),
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}
```

## 健康检查与日志

### 健康检查机制

所有提供商都集成了健康检查功能，定期测试服务可用性：

- **自动检查**: 每24小时自动执行一次完整健康检查
- **实时测试**: 使用真实域名测试所有提供商的响应能力
- **状态监控**: 跟踪每个提供商的响应时间、成功率和调用次数

### 日志分离功能

为了更好地管理和监控提供商的健康状态，系统支持健康检查日志分离：

```bash
# 启用健康检查日志分离
HEALTH_LOG_SEPARATE=true

# 静默主日志中的健康检查信息
HEALTH_LOG_SILENT=true
```

**功能特性**：
- **独立日志文件**: 健康检查信息写入专门的 `logs/health_YYYY-MM-DD.log` 文件
- **主日志清洁**: 启用静默模式后，主服务日志不再包含健康检查信息
- **详细统计**: 提供每个提供商的详细状态统计和汇总信息
- **生产环境友好**: 便于日志分析和监控系统集成

**日志内容示例**：
```
=== WHOIS提供商健康检查 ===
IANA-RDAP: ✓ 正常 (总计: 1, 可用: 1)
IANA-WHOIS: ✓ 正常 (总计: 1, 可用: 1)
WhoisFreaks: ✗ 异常 (总计: 1, 可用: 0)
WhoisXML: ✗ 异常 (总计: 1, 可用: 0)

=== 全部服务健康检查 ===
整体状态: 部分可用
总服务数: 8
可用服务: 6
可用率: 75.00%
健康检查完成，耗时: 43.9070704s
```
