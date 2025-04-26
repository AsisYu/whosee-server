package providers

import (
	"dmainwhoseek/types"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type WhoisFreaksResponse struct {
	Status           interface{} `json:"status"`   // 使用interface{}类型以适应不同的返回值类型
	DomainName       string `json:"domain_name"`
	DomainRegistered string `json:"domain_registered"`
	CreateDate       string `json:"create_date"`
	UpdateDate       string `json:"update_date"`
	ExpiryDate       string `json:"expiry_date"`
	DomainRegistrar  struct {
		RegistrarName string `json:"registrar_name"`
	} `json:"domain_registrar"`
	NameServers  []string `json:"name_servers"`
	DomainStatus []string `json:"domain_status"`
}

type WhoisFreaksProvider struct {
	apiKey string
	client *http.Client
}

func NewWhoisFreaksProvider() *WhoisFreaksProvider {
	return &WhoisFreaksProvider{
		apiKey: os.Getenv("WHOISFREAKS_API_KEY"),
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

func (p *WhoisFreaksProvider) Name() string {
	return "WhoisFreaks"
}

// 添加响应验证
func (p *WhoisFreaksProvider) parseResponse(body []byte) error {
	var raw struct{
		Status interface{} `json:"status"`
	}
	
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}

	switch v := raw.Status.(type) {
	case float64:
		if v != 1 {
			return fmt.Errorf("API返回失败状态: %v", v)
		}
	case bool:
		if !v {
			return errors.New("API返回false状态")
		}
	default:
		return fmt.Errorf("未知status类型: %T", v)
	}
	
	return nil
}

func (p *WhoisFreaksProvider) queryAPI(domain string) (*types.WhoisResponse, error) {
	apiKey := os.Getenv("WHOISFREAKS_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("WHOISFREAKS_API_KEY未配置")
	}
	
	// 记录API密钥前几个字符，用于调试
	keyLength := len(apiKey)
	if keyLength > 10 {
		log.Printf("使用WhoisFreaks API密钥(前10个字符): %s...", apiKey[:10])
	} else {
		log.Printf("使用WhoisFreaks API密钥(长度不足): %s...", apiKey)
	}
	
	apiURL := fmt.Sprintf("https://api.whoisfreaks.com/v1.0/whois?apiKey=%s&whois=live&domainName=%s",
		url.QueryEscape(apiKey),
		url.QueryEscape(domain))

	// 添加请求日志，隐藏API密钥
	log.Printf("请求WhoisFreaks: %s", strings.Replace(apiURL, apiKey, "[HIDDEN]", 1))

	// 实现重试逻辑
	var (
		resp *http.Response
		body []byte
	)
	
	maxRetries := 2
	retryDelay := 2 * time.Second
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("WhoisFreaks API 重试请求 #%d，域名: %s", attempt, domain)
			time.Sleep(retryDelay)
			retryDelay *= 2 // 指数退避
		}
		
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("创建请求失败: %v", err)
		}
		
		req.Header.Set("User-Agent", "DomainWhoseek/1.0")
		resp, err = p.client.Do(req)
		
		if err != nil {
			log.Printf("WhoisFreaks API 请求失败 (尝试 %d/%d): %v", attempt+1, maxRetries+1, err)
			continue // 重试
		}
		
		defer resp.Body.Close()
		body, err = ioutil.ReadAll(resp.Body)
		
		if err != nil {
			log.Printf("读取响应失败 (尝试 %d/%d): %v", attempt+1, maxRetries+1, err)
			continue // 重试
		}
		
		// 检查状态码
		log.Printf("WhoisFreaks API 响应状态码: %d (尝试 %d/%d)", resp.StatusCode, attempt+1, maxRetries+1)
		
		// 如果状态码表示限流或服务器错误，则重试
		if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
			log.Printf("WhoisFreaks API 返回状态码 %d，将重试", resp.StatusCode)
			continue
		}
		
		// 如果状态码不是成功，则返回错误
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("WhoisFreaks API 返回错误状态码: %d", resp.StatusCode)
		}
		
		// 尝试解析响应
		var whoisResp WhoisFreaksResponse
		err = json.Unmarshal(body, &whoisResp)
		if err != nil {
			log.Printf("解析响应失败 (尝试 %d/%d): %v", attempt+1, maxRetries+1, err)
			continue // 重试
		}
		
		// 验证响应
		err = p.parseResponse(body)
		if err != nil {
			log.Printf("响应验证失败 (尝试 %d/%d): %v", attempt+1, maxRetries+1, err)
			continue // 重试
		}
		
		// 成功处理响应
		result := &types.WhoisResponse{
			Available:   whoisResp.DomainRegistered != "yes",
			Domain:      whoisResp.DomainName,
			Registrar:   whoisResp.DomainRegistrar.RegistrarName,
			CreateDate:  whoisResp.CreateDate,
			ExpiryDate:  whoisResp.ExpiryDate,
			Status:      whoisResp.DomainStatus,
			NameServers: whoisResp.NameServers,
			UpdateDate:  whoisResp.UpdateDate,
		}
		
		// 记录查询结果
		log.Printf("WhoisFreaks 查询结果 - 域名: %s, 注册商: %s, 创建日期: %s, 到期日期: %s, 状态: %v, 名称服务器: %v",
			result.Domain, 
			result.Registrar, 
			result.CreateDate, 
			result.ExpiryDate, 
			result.Status, 
			result.NameServers)
		
		return result, nil
	}
	
	// 所有重试都失败
	return nil, fmt.Errorf("WhoisFreaks API 请求失败，已重试 %d 次", maxRetries)
}

func (p *WhoisFreaksProvider) Query(domain string) (*types.WhoisResponse, error, bool) {
	log.Printf("使用 WhoisFreaks API 查询域名: %s", domain)
	response, err := p.queryAPI(domain)
	if err != nil {
		log.Printf("WhoisFreaks API 查询失败: %v", err)
	} else {
		log.Printf("WhoisFreaks API 查询成功: %s", domain)
	}
	return response, err, false
}
