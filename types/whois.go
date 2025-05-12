/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-18 22:34:01
 * @Description: WHOIS查询类型定义
 */
package types

// WhoisResponse 统一的WHOIS响应结构
type WhoisResponse struct {
	Available      bool     `json:"available"`
	Domain         string   `json:"domain"`
	Registrar      string   `json:"registrar"`
	CreateDate     string   `json:"creationDate"`
	ExpiryDate     string   `json:"expiryDate"`
	Status         []string `json:"status"`
	NameServers    []string `json:"nameServers"`
	UpdateDate     string   `json:"updatedDate"`
	Registrant     *Contact `json:"registrant,omitempty"`
	Admin          *Contact `json:"admin,omitempty"`
	Tech           *Contact `json:"tech,omitempty"`
	WhoisServer    string   `json:"whoisServer,omitempty"`
	DomainAge      int      `json:"domainAge,omitempty"`
	ContactEmail   string   `json:"contactEmail,omitempty"`
	SourceProvider string   `json:"sourceProvider,omitempty"` // 数据来源提供商
	StatusCode     int      `json:"statusCode"`               // 查询状态码
	StatusMessage  string   `json:"statusMessage,omitempty"`  // 状态描述信息
	CachedAt       string   `json:"cachedAt,omitempty"`       // 数据缓存时间
}

type Contact struct {
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Country      string `json:"country,omitempty"`
	Province     string `json:"province,omitempty"`
	City         string `json:"city,omitempty"`
}

// WhoisProvider WHOIS服务提供者接口
type WhoisProvider interface {
	Query(domain string) (*WhoisResponse, error, bool)
	Name() string
}
