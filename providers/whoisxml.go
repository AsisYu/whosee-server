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
	client *http.Client
}

func NewWhoisXMLProvider() *WhoisXMLProvider {
	return &WhoisXMLProvider{
		apiKey: os.Getenv("WHOISXML_API_KEY"),
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     60 * time.Second,
			},
		},
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
	} else {
		log.Printf("WhoisXML API 查询成功: %s", domain)
	}
	return response, err, false
}

func (p *WhoisXMLProvider) queryAPI(domain string) (*types.WhoisResponse, error) {
	apiKey := os.Getenv("WHOISXML_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("WHOISXML_API_KEY未配置")
	}
	
	// 记录API密钥前几个字符，用于调试
	keyLength := len(apiKey)
	if keyLength > 10 {
		log.Printf("使用WhoisXML API密钥(前10个字符): %s...", apiKey[:10])
	} else {
		log.Printf("使用WhoisXML API密钥(长度不足): %s...", apiKey)
	}
	
	apiURL := fmt.Sprintf("https://www.whoisxmlapi.com/whoisserver/WhoisService?apiKey=%s&domainName=%s&outputFormat=JSON",
		url.QueryEscape(apiKey),
		url.QueryEscape(domain))
	
	// 添加请求日志，隐藏API密钥
	log.Printf("请求WhoisXML: %s", strings.Replace(apiURL, apiKey, "[HIDDEN]", 1))

	// 实现重试逻辑
	var (
		resp *http.Response
		body []byte
	)
	
	maxRetries := 2
	retryDelay := 2 * time.Second
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("WhoisXML API 重试请求 #%d，域名: %s", attempt, domain)
			time.Sleep(retryDelay)
			retryDelay *= 2 // 指数退避
		}
		
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("创建请求失败: %v", err)
		}
		
		req.Header.Set("User-Agent", "DomainWhoseek/1.0")
		req.Header.Set("Accept", "application/json")
		
		resp, err = p.client.Do(req)
		
		if err != nil {
			log.Printf("WhoisXML API 请求失败 (尝试 %d/%d): %v", attempt+1, maxRetries+1, err)
			continue // 重试
		}
		
		defer resp.Body.Close()
		
		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(resp.Body)
			log.Printf("WhoisXML API 返回非200状态码: %d, 响应: %s (尝试 %d/%d)", 
				resp.StatusCode, string(body), attempt+1, maxRetries+1)
			
			// 如果状态码表示限流或服务器错误，则重试
			if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
				log.Printf("WhoisXML API 返回状态码 %d，将重试", resp.StatusCode)
				continue
			}
			
			return nil, fmt.Errorf("API返回状态码 %d", resp.StatusCode)
		}
		
		// 检查Content-Type
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			body, _ := ioutil.ReadAll(resp.Body)
			log.Printf("WhoisXML API 返回非JSON格式: %s, 响应: %s (尝试 %d/%d)", 
				contentType, string(body), attempt+1, maxRetries+1)
			continue // 重试
		}
		
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("读取响应失败 (尝试 %d/%d): %v", attempt+1, maxRetries+1, err)
			continue // 重试
		}
		
		// 解析响应
		var whoisResp WhoisXMLResponse
		err = json.Unmarshal(body, &whoisResp)
		if err != nil {
			log.Printf("解析响应失败 (尝试 %d/%d): %v", attempt+1, maxRetries+1, err)
			continue // 重试
		}
		
		// 验证响应
		if whoisResp.WhoisRecord.DomainName == "" {
			log.Printf("响应验证失败：域名为空 (尝试 %d/%d)", attempt+1, maxRetries+1)
			continue // 重试
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
		
		// 优先使用主记录的名称服务器，如果为空则使用注册数据中的名称服务器
		nameServers := whoisResp.WhoisRecord.NameServers.HostNames
		if len(nameServers) == 0 {
			nameServers = whoisResp.WhoisRecord.RegistryData.NameServers.HostNames
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
		
		// 如果主记录中没有注册人信息，尝试使用注册数据中的信息
		if registrant.Name == "" && registrant.Email == "" {
			registrant.Name = whoisResp.WhoisRecord.RegistryData.Registrant.Name
			registrant.Email = whoisResp.WhoisRecord.RegistryData.Registrant.Email
		}
		
		// 成功处理响应
		result := &types.WhoisResponse{
			Available:    false, // WhoisXML API不直接提供域名可用性信息
			Domain:       whoisResp.WhoisRecord.DomainName,
			Registrar:    whoisResp.WhoisRecord.RegistrarName,
			CreateDate:   createdDate,
			ExpiryDate:   expiresDate,
			Status:       statuses,
			NameServers:  nameServers,
			UpdateDate:   updatedDate,
			Registrant:   registrant,
			WhoisServer:  whoisResp.WhoisRecord.WhoisServer,
			DomainAge:    whoisResp.WhoisRecord.EstimatedDomainAge,
			ContactEmail: whoisResp.WhoisRecord.ContactEmail,
		}
		
		// 记录查询结果
		log.Printf("WhoisXML 查询结果 - 域名: %s, 注册商: %s, 创建日期: %s, 到期日期: %s, 状态: %v, 名称服务器: %v",
			result.Domain, 
			result.Registrar, 
			result.CreateDate, 
			result.ExpiryDate, 
			result.Status, 
			result.NameServers)
		
		return result, nil
	}
	
	// 所有重试都失败
	return nil, fmt.Errorf("WhoisXML API 请求失败，已重试 %d 次", maxRetries)
}
