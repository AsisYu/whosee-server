/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-19 10:00:00
 * @Description: IANA RDAP 提供商 - 基于RDAP协议的现代化WHOIS查询
 */
package providers

import (
	"context"
	"dmainwhoseek/types"
	"dmainwhoseek/utils"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RDAPResponse RDAP 协议的标准响应结构
type RDAPResponse struct {
	ObjectClassName string       `json:"objectClassName"`
	Handle          string       `json:"handle"`
	LDHName         string       `json:"ldhName"`
	UnicodeName     string       `json:"unicodeName,omitempty"`
	Entities        []Entity     `json:"entities,omitempty"`
	Status          []string     `json:"status,omitempty"`
	Events          []Event      `json:"events,omitempty"`
	NameServers     []NameServer `json:"nameservers,omitempty"`
	SecureDNS       *SecureDNS   `json:"secureDNS,omitempty"`
	Links           []Link       `json:"links,omitempty"`
	Port43          string       `json:"port43,omitempty"`
	Notices         []Notice     `json:"notices,omitempty"`
	Remarks         []Notice     `json:"remarks,omitempty"`
}

type Entity struct {
	ObjectClassName string        `json:"objectClassName"`
	Handle          string        `json:"handle"`
	Roles           []string      `json:"roles"`
	VCardArray      []interface{} `json:"vcardArray,omitempty"`
	PublicIDs       []PublicID    `json:"publicIds,omitempty"`
	Entities        []Entity      `json:"entities,omitempty"`
	Remarks         []Notice      `json:"remarks,omitempty"`
	Links           []Link        `json:"links,omitempty"`
	Events          []Event       `json:"events,omitempty"`
	Status          []string      `json:"status,omitempty"`
}

type Event struct {
	EventAction string `json:"eventAction"`
	EventDate   string `json:"eventDate"`
	EventActor  string `json:"eventActor,omitempty"`
}

type NameServer struct {
	ObjectClassName string       `json:"objectClassName"`
	LDHName         string       `json:"ldhName"`
	UnicodeName     string       `json:"unicodeName,omitempty"`
	IPAddresses     *IPAddresses `json:"ipAddresses,omitempty"`
	Status          []string     `json:"status,omitempty"`
	Links           []Link       `json:"links,omitempty"`
}

type IPAddresses struct {
	V4 []string `json:"v4,omitempty"`
	V6 []string `json:"v6,omitempty"`
}

type SecureDNS struct {
	ZoneSigned       bool      `json:"zoneSigned"`
	DelegationSigned bool      `json:"delegationSigned"`
	MaxSigLife       int       `json:"maxSigLife,omitempty"`
	DSData           []DSData  `json:"dsData,omitempty"`
	KeyData          []KeyData `json:"keyData,omitempty"`
}

type DSData struct {
	KeyTag     int    `json:"keyTag"`
	Algorithm  int    `json:"algorithm"`
	Digest     string `json:"digest"`
	DigestType int    `json:"digestType"`
}

type KeyData struct {
	Flags     int    `json:"flags"`
	Protocol  int    `json:"protocol"`
	Algorithm int    `json:"algorithm"`
	PublicKey string `json:"publicKey"`
}

type PublicID struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}

type Link struct {
	Value    string   `json:"value,omitempty"`
	Rel      string   `json:"rel"`
	Href     string   `json:"href"`
	HrefLang []string `json:"hreflang,omitempty"`
	Title    string   `json:"title,omitempty"`
	Media    string   `json:"media,omitempty"`
	Type     string   `json:"type,omitempty"`
}

type Notice struct {
	Title       string   `json:"title,omitempty"`
	Type        string   `json:"type,omitempty"`
	Description []string `json:"description,omitempty"`
	Links       []Link   `json:"links,omitempty"`
}

type IANARDAPProvider struct {
	client *http.Client
}

func NewIANARDAPProvider() *IANARDAPProvider {
	return &IANARDAPProvider{
		client: &http.Client{
			Timeout: 15 * time.Second,
			// 禁用自动重定向，我们手动处理重定向以获得更好的控制和日志记录
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				MaxIdleConns:          10,
				MaxIdleConnsPerHost:   2,
				MaxConnsPerHost:       5,
				IdleConnTimeout:       30 * time.Second,
			},
		},
	}
}

func (p *IANARDAPProvider) Name() string {
	return "IANA-RDAP"
}

func (p *IANARDAPProvider) Query(domain string) (*types.WhoisResponse, error, bool) {
	log.Printf("使用 IANA RDAP 查询域名: %s", domain)

	// 验证域名格式
	if !utils.IsValidDomain(domain) {
		return &types.WhoisResponse{
			Domain:         domain,
			Available:      false,
			StatusCode:     422,
			StatusMessage:  "无效的域名格式",
			SourceProvider: p.Name(),
		}, fmt.Errorf("无效的域名格式: %s", domain), false
	}

	// 尝试多个查询策略
	strategies := []struct {
		name string
		url  string
	}{
		{
			name: "RDAP.org引导服务器",
			url:  "https://rdap.org/domain/" + url.QueryEscape(domain),
		},
		{
			name: "通用RDAP引导服务器",
			url:  "https://bootstrap.rdap.org/domain/" + url.QueryEscape(domain),
		},
	}

	var lastErr error
	var response *types.WhoisResponse

	for i, strategy := range strategies {
		log.Printf("RDAP查询策略 %d (%s): %s", i+1, strategy.name, strategy.url)

		resp, err := p.queryRDAP(strategy.url, domain)
		if err == nil && resp != nil {
			response = resp
			break
		}

		lastErr = err
		log.Printf("RDAP查询策略 %d (%s) 失败: %v", i+1, strategy.name, err)

		// 如果不是最后一个策略，稍作延迟
		if i < len(strategies)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	if response == nil {
		return &types.WhoisResponse{
			Domain:         domain,
			Available:      false,
			StatusCode:     500,
			StatusMessage:  "RDAP查询失败",
			SourceProvider: p.Name(),
		}, fmt.Errorf("所有RDAP查询策略都失败: %v", lastErr), false
	}

	return response, nil, false
}

func (p *IANARDAPProvider) queryRDAP(rdapURL, domain string) (*types.WhoisResponse, error) {
	log.Printf("请求RDAP: %s", rdapURL)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", rdapURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建RDAP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "application/rdap+json, application/json")
	req.Header.Set("User-Agent", "WhoseeWhois/1.0 (+https://whosee.me)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RDAP请求失败: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("RDAP API 响应状态码: %d", resp.StatusCode)

	// 处理重定向 - 引导服务器通常返回302重定向
	if resp.StatusCode == 302 || resp.StatusCode == 301 {
		location := resp.Header.Get("Location")
		if location == "" {
			return nil, fmt.Errorf("RDAP重定向缺少Location头")
		}
		log.Printf("RDAP重定向到: %s", location)

		// 递归查询重定向的URL，但限制重定向次数以防止无限循环
		return p.queryRDAPWithRedirectLimit(location, domain, 3)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取RDAP响应失败: %v", err)
	}

	// 处理HTTP错误状态
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("RDAP查询失败 (状态码: %d): Not Found - 可能该域名不存在或RDAP服务器不支持该域名", resp.StatusCode)
	} else if resp.StatusCode != 200 {
		// 尝试解析错误信息
		var errorResp map[string]interface{}
		if json.Unmarshal(body, &errorResp) == nil {
			if title, ok := errorResp["title"].(string); ok {
				return nil, fmt.Errorf("RDAP查询失败 (状态码: %d): %s", resp.StatusCode, title)
			}
		}
		return nil, fmt.Errorf("RDAP查询失败，状态码: %d", resp.StatusCode)
	}

	// 解析RDAP响应
	var rdapResp RDAPResponse
	if err := json.Unmarshal(body, &rdapResp); err != nil {
		return nil, fmt.Errorf("解析RDAP响应失败: %v", err)
	}

	// 转换为统一格式
	whoisResp := p.convertRDAPToWhois(&rdapResp, domain)
	log.Printf("RDAP 查询成功: 域名=%s, 注册商=%s, 创建日期=%s, 到期日期=%s",
		domain, whoisResp.Registrar, whoisResp.CreateDate, whoisResp.ExpiryDate)

	return whoisResp, nil
}

// queryRDAPWithRedirectLimit 处理重定向，带重定向次数限制
func (p *IANARDAPProvider) queryRDAPWithRedirectLimit(rdapURL, domain string, maxRedirects int) (*types.WhoisResponse, error) {
	if maxRedirects <= 0 {
		return nil, fmt.Errorf("RDAP重定向次数超过限制")
	}

	log.Printf("跟随RDAP重定向: %s (剩余重定向次数: %d)", rdapURL, maxRedirects)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", rdapURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建RDAP重定向请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "application/rdap+json, application/json")
	req.Header.Set("User-Agent", "WhoseeWhois/1.0 (+https://whosee.me)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RDAP重定向请求失败: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("RDAP重定向响应状态码: %d", resp.StatusCode)

	// 再次检查重定向
	if resp.StatusCode == 302 || resp.StatusCode == 301 {
		location := resp.Header.Get("Location")
		if location == "" {
			return nil, fmt.Errorf("RDAP重定向缺少Location头")
		}
		return p.queryRDAPWithRedirectLimit(location, domain, maxRedirects-1)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取RDAP重定向响应失败: %v", err)
	}

	// 处理HTTP错误状态
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("RDAP查询失败 (状态码: %d): Not Found - 可能该域名不存在或RDAP服务器不支持该域名", resp.StatusCode)
	} else if resp.StatusCode != 200 {
		// 尝试解析错误信息
		var errorResp map[string]interface{}
		if json.Unmarshal(body, &errorResp) == nil {
			if title, ok := errorResp["title"].(string); ok {
				return nil, fmt.Errorf("RDAP查询失败 (状态码: %d): %s", resp.StatusCode, title)
			}
		}
		return nil, fmt.Errorf("RDAP查询失败，状态码: %d", resp.StatusCode)
	}

	// 解析RDAP响应
	var rdapResp RDAPResponse
	if err := json.Unmarshal(body, &rdapResp); err != nil {
		return nil, fmt.Errorf("解析RDAP响应失败: %v", err)
	}

	// 转换为统一格式
	whoisResp := p.convertRDAPToWhois(&rdapResp, domain)
	log.Printf("RDAP 重定向查询成功: 域名=%s, 注册商=%s, 创建日期=%s, 到期日期=%s",
		domain, whoisResp.Registrar, whoisResp.CreateDate, whoisResp.ExpiryDate)

	return whoisResp, nil
}

func (p *IANARDAPProvider) convertRDAPToWhois(rdap *RDAPResponse, domain string) *types.WhoisResponse {
	whois := &types.WhoisResponse{
		Domain:         domain,
		Available:      false, // RDAP返回数据说明域名已注册
		StatusCode:     200,
		StatusMessage:  "查询成功",
		SourceProvider: p.Name(),
		WhoisServer:    rdap.Port43,
	}

	// 处理域名状态
	whois.Status = rdap.Status

	// 处理事件信息（创建、更新、到期日期）
	for _, event := range rdap.Events {
		switch strings.ToLower(event.EventAction) {
		case "registration":
			whois.CreateDate = event.EventDate
		case "last update of rdap database", "last changed":
			whois.UpdateDate = event.EventDate
		case "expiration":
			whois.ExpiryDate = event.EventDate
		}
	}

	// 处理名称服务器
	var nameServers []string
	for _, ns := range rdap.NameServers {
		if ns.LDHName != "" {
			nameServers = append(nameServers, ns.LDHName)
		}
	}
	whois.NameServers = nameServers

	// 处理实体信息（注册商、联系人）
	for _, entity := range rdap.Entities {
		if p.entityHasRole(entity.Roles, "registrar") {
			whois.Registrar = p.extractEntityName(entity)
		}

		if p.entityHasRole(entity.Roles, "registrant") {
			whois.Registrant = p.extractContact(entity)
		}

		if p.entityHasRole(entity.Roles, "administrative") {
			whois.Admin = p.extractContact(entity)
		}

		if p.entityHasRole(entity.Roles, "technical") {
			whois.Tech = p.extractContact(entity)
		}
	}

	// 计算域名年龄
	if whois.CreateDate != "" {
		if createTime, err := time.Parse("2006-01-02T15:04:05Z", whois.CreateDate); err == nil {
			whois.DomainAge = int(time.Since(createTime).Hours() / 24)
		}
	}

	// 格式化日期（将ISO 8601转换为更友好的格式）
	whois.CreateDate = p.formatDate(whois.CreateDate)
	whois.UpdateDate = p.formatDate(whois.UpdateDate)
	whois.ExpiryDate = p.formatDate(whois.ExpiryDate)

	return whois
}

func (p *IANARDAPProvider) extractEntityName(entity Entity) string {
	// 尝试从vCard中提取名称
	if len(entity.VCardArray) > 1 {
		if properties, ok := entity.VCardArray[1].([]interface{}); ok {
			for _, prop := range properties {
				if propArray, ok := prop.([]interface{}); ok && len(propArray) >= 4 {
					if propName, ok := propArray[0].(string); ok && propName == "fn" {
						if value, ok := propArray[3].(string); ok {
							return value
						}
					}
				}
			}
		}
	}

	// 回退到handle
	return entity.Handle
}

func (p *IANARDAPProvider) extractContact(entity Entity) *types.Contact {
	contact := &types.Contact{}

	// 解析vCard数据
	if len(entity.VCardArray) > 1 {
		if properties, ok := entity.VCardArray[1].([]interface{}); ok {
			for _, prop := range properties {
				if propArray, ok := prop.([]interface{}); ok && len(propArray) >= 4 {
					propName, _ := propArray[0].(string)
					value, _ := propArray[3].(string)

					switch propName {
					case "fn":
						contact.Name = value
					case "org":
						contact.Organization = value
					case "email":
						contact.Email = value
					case "tel":
						contact.Phone = value
					case "adr":
						// 地址信息通常更复杂，这里简化处理
						if addrArray, ok := propArray[3].([]interface{}); ok && len(addrArray) >= 3 {
							if city, ok := addrArray[3].(string); ok {
								contact.City = city
							}
							if province, ok := addrArray[4].(string); ok {
								contact.Province = province
							}
							if country, ok := addrArray[6].(string); ok {
								contact.Country = country
							}
						}
					}
				}
			}
		}
	}

	return contact
}

func (p *IANARDAPProvider) formatDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	// 尝试解析ISO 8601格式
	if t, err := time.Parse("2006-01-02T15:04:05Z", dateStr); err == nil {
		return t.Format("2006-01-02")
	}

	// 尝试其他格式
	formats := []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.Format("2006-01-02")
		}
	}

	return dateStr // 如果无法解析，返回原始格式
}

// 辅助函数：检查实体是否具有特定角色
func (p *IANARDAPProvider) entityHasRole(roles []string, targetRole string) bool {
	for _, role := range roles {
		if strings.EqualFold(role, targetRole) {
			return true
		}
	}
	return false
}
