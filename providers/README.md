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
