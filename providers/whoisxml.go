package providers

import (
	"dmainwhoseek/types"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type WhoisXMLResponse struct {
	WhoisRecord struct {
		DomainName    string `json:"domainName"`
		RegistrarName string `json:"registrarName"`
		CreatedDate   string `json:"createdDate"`
		ExpiresDate   string `json:"expiresDate"`
		UpdatedDate   string `json:"updatedDate"`
		Status        string `json:"status"`
		WhoisServer   string `json:"whoisServer"`
		ContactEmail  string `json:"contactEmail"`
		NameServers   struct {
			HostNames []string `json:"hostNames"`
		} `json:"nameServers"`
		Registrant struct {
			Name         string `json:"name"`
			Organization string `json:"organization"`
			Email        string `json:"email"`
			Phone        string `json:"telephone"`
			Country      string `json:"country"`
			State        string `json:"state"`
			City         string `json:"city"`
		} `json:"registrant"`
		RegistryData struct {
			Status      string `json:"status"`
			CreatedDate string `json:"createdDate"`
			ExpiresDate string `json:"expiresDate"`
			UpdatedDate string `json:"updatedDate"`
			WhoisServer string `json:"whoisServer"`
			NameServers struct {
				HostNames []string `json:"hostNames"`
			} `json:"nameServers"`
			Registrant struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"registrant"`
		} `json:"registryData"`
		EstimatedDomainAge int `json:"estimatedDomainAge"`
	} `json:"WhoisRecord"`
}

type WhoisXMLProvider struct {
	apiKey string
}

func NewWhoisXMLProvider() *WhoisXMLProvider {
	return &WhoisXMLProvider{
		apiKey: os.Getenv("WHOISXML_API_KEY"),
	}
}

func (p *WhoisXMLProvider) Name() string {
	return "WhoisXML"
}

func (p *WhoisXMLProvider) Query(domain string) (*types.WhoisResponse, error, bool) {
	log.Printf("使用 WhoisXML API 查询域名: %s", domain)
	response, err := p.queryAPI(domain)
	if err != nil {
		log.Printf("WhoisXML API 查询失败: %v", err)
	}
	return response, err, false
}

func (p *WhoisXMLProvider) queryAPI(domain string) (*types.WhoisResponse, error) {
	apiKey := os.Getenv("WHOISXML_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("WHOISXML_API_KEY未配置")
	}
	
	// 记录API密钥前几个字符，用于调试
	log.Printf("使用WhoisXML API密钥(前10个字符): %s...", apiKey[:10])
	
	apiURL := fmt.Sprintf("https://www.whoisxmlapi.com/whoisserver/WhoisService?apiKey=%s&domainName=%s&outputFormat=JSON",
		url.QueryEscape(apiKey),
		url.QueryEscape(domain))
	
	// 添加请求日志，隐藏API密钥
	log.Printf("请求WhoisXML: %s", strings.Replace(apiURL, apiKey, "[HIDDEN]", 1))

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %v", err)
	}

	req.Header.Set("User-Agent", "DomainWhoseek/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("WhoisXML API 返回非200状态码: %d, 响应: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("WhoisXML API 返回非JSON格式: %s, 响应: %s", contentType, string(body))
		return nil, fmt.Errorf("unexpected content type: %s", contentType)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %v", err)
	}

	log.Printf("WhoisXML API 原始响应: %s", string(body))

	var whoisResp WhoisXMLResponse
	if err := json.Unmarshal(body, &whoisResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %v", err)
	}

	// 优先使用主状态，如果为空则使用注册数据中的状态
	statusStr := whoisResp.WhoisRecord.Status
	if statusStr == "" {
		statusStr = whoisResp.WhoisRecord.RegistryData.Status
	}
	statuses := strings.Fields(statusStr)

	// 优先使用主记录的日期，如果为空则使用注册数据中的日期
	createdDate := whoisResp.WhoisRecord.CreatedDate
	if createdDate == "" {
		createdDate = whoisResp.WhoisRecord.RegistryData.CreatedDate
	}

	expiresDate := whoisResp.WhoisRecord.ExpiresDate
	if expiresDate == "" {
		expiresDate = whoisResp.WhoisRecord.RegistryData.ExpiresDate
	}

	updatedDate := whoisResp.WhoisRecord.UpdatedDate
	if updatedDate == "" {
		updatedDate = whoisResp.WhoisRecord.RegistryData.UpdatedDate
	}

	// 优先使用主记录的名称服务器，如果为空则使用注册数据中的
	nameServers := whoisResp.WhoisRecord.NameServers.HostNames
	if len(nameServers) == 0 {
		nameServers = whoisResp.WhoisRecord.RegistryData.NameServers.HostNames
	}

	// 检查是否有错误响应
	if whoisResp.WhoisRecord.DomainName == "" {
		return nil, fmt.Errorf("domain not found or API error")
	}

	// 构建联系人信息
	registrant := &types.Contact{
		Name:         whoisResp.WhoisRecord.Registrant.Name,
		Organization: whoisResp.WhoisRecord.Registrant.Organization,
		Email:        whoisResp.WhoisRecord.Registrant.Email,
		Phone:        whoisResp.WhoisRecord.Registrant.Phone,
		Country:      whoisResp.WhoisRecord.Registrant.Country,
		Province:     whoisResp.WhoisRecord.Registrant.State,
		City:         whoisResp.WhoisRecord.Registrant.City,
	}

	// 如果主记录中没有注册人信息，使用注册数据中的
	if registrant.Name == "" && registrant.Email == "" {
		registrant = &types.Contact{
			Name:  whoisResp.WhoisRecord.RegistryData.Registrant.Name,
			Email: whoisResp.WhoisRecord.RegistryData.Registrant.Email,
		}
	}

	whoisServer := whoisResp.WhoisRecord.WhoisServer
	if whoisServer == "" {
		whoisServer = whoisResp.WhoisRecord.RegistryData.WhoisServer
	}

	return &types.WhoisResponse{
		Available:    len(statuses) == 0,
		Domain:       whoisResp.WhoisRecord.DomainName,
		Registrar:    whoisResp.WhoisRecord.RegistrarName,
		CreateDate:   createdDate,
		ExpiryDate:   expiresDate,
		Status:       statuses,
		NameServers:  nameServers,
		UpdateDate:   updatedDate,
		Registrant:   registrant,
		WhoisServer:  whoisServer,
		DomainAge:    whoisResp.WhoisRecord.EstimatedDomainAge,
		ContactEmail: whoisResp.WhoisRecord.ContactEmail,
	}, nil
}
