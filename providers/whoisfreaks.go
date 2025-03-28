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
}

func NewWhoisFreaksProvider() *WhoisFreaksProvider {
	return &WhoisFreaksProvider{
		apiKey: os.Getenv("WHOISFREAKS_API_KEY"),
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
	
	apiURL := fmt.Sprintf("https://api.whoisfreaks.com/v1.0/whois?apiKey=%s&whois=live&domainName=%s",
		url.QueryEscape(apiKey),
		url.QueryEscape(domain))

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %v", err)
	}

	req.Header.Set("User-Agent", "DomainWhoseek/1.0")
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, fmt.Errorf("API request timeout: %v", err)
		}
		return nil, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %v", err)
	}

	var whoisResp WhoisFreaksResponse
	if err := json.Unmarshal(body, &whoisResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %v", err)
	}

	return &types.WhoisResponse{
		Available:   whoisResp.DomainRegistered != "yes",
		Domain:      whoisResp.DomainName,
		Registrar:   whoisResp.DomainRegistrar.RegistrarName,
		CreateDate:  whoisResp.CreateDate,
		ExpiryDate:  whoisResp.ExpiryDate,
		Status:      whoisResp.DomainStatus,
		NameServers: whoisResp.NameServers,
		UpdateDate:  whoisResp.UpdateDate,
	}, nil
}

func (p *WhoisFreaksProvider) Query(domain string) (*types.WhoisResponse, error, bool) {
	log.Printf("使用 WhoisFreaks API 查询域名: %s", domain)
	response, err := p.queryAPI(domain)
	if err != nil {
		log.Printf("WhoisFreaks API 查询失败: %v", err)
	}
	return response, err, false
}
