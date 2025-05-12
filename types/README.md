# 类型目录 (Types)

## 目录作用

类型目录包含在整个应用程序中使用的数据结构定义和类型声明。这些结构为数据表示建立了一个共同的基础，确保系统的不同组件之间的一致性。

## 文件列表与功能

- `whois.go` - WHOIS相关的数据结构定义和接口

## 数据模型

该目录中定义的主要数据模型包括：

```go
// WhoisResponse 统一的WHOIS响应结构
type WhoisResponse struct {
    Available    bool     `json:"available"`
    Domain       string   `json:"domain"`
    Registrar    string   `json:"registrar"`
    CreateDate   string   `json:"creationDate"`
    ExpiryDate   string   `json:"expiryDate"`
    Status       []string `json:"status"`
    NameServers  []string `json:"nameServers"`
    UpdateDate   string   `json:"updatedDate"`
    Registrant   *Contact `json:"registrant,omitempty"`
    Admin        *Contact `json:"admin,omitempty"`
    Tech         *Contact `json:"tech,omitempty"`
    WhoisServer  string   `json:"whoisServer,omitempty"`
    DomainAge    int      `json:"domainAge,omitempty"`
    ContactEmail string   `json:"contactEmail,omitempty"`
    SourceProvider string  `json:"sourceProvider,omitempty"` // 数据来源提供商
    StatusCode    int     `json:"statusCode"`              // 查询状态码
    StatusMessage string   `json:"statusMessage,omitempty"` // 状态描述信息
    CachedAt     string   `json:"cachedAt,omitempty"`      // 数据缓存时间
}

// Contact 联系人信息结构
type Contact struct {
    Name         string `json:"name,omitempty"`
    Organization string `json:"organization,omitempty"`
    Email        string `json:"email,omitempty"`
    Phone        string `json:"phone,omitempty"`
    Country      string `json:"country,omitempty"`
    Province     string `json:"province,omitempty"`
    City         string `json:"city,omitempty"`
}
```

## WHOIS提供者接口

系统定义了WHOIS服务提供者接口，用于不同WHOIS查询服务的实现：

```go
// WhoisProvider WHOIS服务提供者接口
type WhoisProvider interface {
    Query(domain string) (*WhoisResponse, error, bool)
    Name() string
}
```

## 类型集中的优势

1. **一致性** - 确保数据结构在整个应用程序中一致使用
2. **文档化** - 类型作为数据模型的自文档
3. **类型安全** - 利用Go的类型系统防止错误
4. **可维护性** - 对数据结构的更改只需要在一个地方进行
