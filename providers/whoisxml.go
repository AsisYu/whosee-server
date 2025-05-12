/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-09 12:15:00
 * @Description: WhoisXML提供商
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
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     60 * time.Second,
				TLSHandshakeTimeout: 15 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 60 * time.Second,
				}).DialContext,
				DisableKeepAlives: false,
				ForceAttemptHTTP2: true,
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

func (p *WhoisXMLProvider) classifyError(err error, statusCode int) string {
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "timeout") {
			return "timeout"
		} else if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "no such host") {
			return "connection"
		} else if strings.Contains(err.Error(), "no route to host") ||
			strings.Contains(err.Error(), "network is unreachable") {
			return "network"
		}
		return "unknown"
	}

	switch {
	case statusCode == 429:
		return "rate_limit"
	case statusCode >= 500 && statusCode < 600:
		return "server"
	case statusCode >= 400 && statusCode < 500:
		return "client"
	default:
		return "success"
	}
}

func (p *WhoisXMLProvider) queryAPI(domain string) (*types.WhoisResponse, error) {
	apiKey := os.Getenv("WHOISXML_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("WHOISXML_API_KEY未配置")
	}

	keyLength := len(apiKey)
	if keyLength > 10 {
		log.Printf("使用WhoisXML API密钥(前10个字符): %s...", apiKey[:10])
	} else {
		log.Printf("使用WhoisXML API密钥(长度不足): %s...", apiKey)
	}

	apiURL := fmt.Sprintf("https://www.whoisxmlapi.com/whoisserver/WhoisService?apiKey=%s&domainName=%s&outputFormat=JSON",
		url.QueryEscape(apiKey),
		url.QueryEscape(domain))

	log.Printf("请求WhoisXML: %s", strings.Replace(apiURL, apiKey, "[HIDDEN]", 1))

	var (
		resp *http.Response
		body []byte
	)

	maxRetries := 3
	retryDelay := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("WhoisXML API 重试请求 #%d，域名: %s", attempt, domain)
			time.Sleep(retryDelay)
			retryDelay = time.Duration(float64(retryDelay) * 1.5)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("创建请求失败: %v", err)
		}

		req.Header.Set("User-Agent", "DomainWhoseek/1.0")
		req.Header.Set("Accept", "application/json")

		startTime := time.Now()
		resp, err = p.client.Do(req)
		requestTime := time.Since(startTime)
		cancel()

		log.Printf("WhoisXML API 请求耗时: %v (重试 %d/%d)", requestTime, attempt+1, maxRetries+1)

		if err != nil {
			errorType := p.classifyError(err, 0)
			log.Printf("WhoisXML API 请求失败 (%s) (重试 %d/%d): %v", errorType, attempt+1, maxRetries+1, err)

			if errorType == "connection" || errorType == "network" {
				retryDelay = time.Duration(float64(retryDelay) * 2)
			}
			continue
		}

		defer resp.Body.Close()

		log.Printf("WhoisXML API 响应状态码: %d (重试 %d/%d)", resp.StatusCode, attempt+1, maxRetries+1)

		errorType := p.classifyError(nil, resp.StatusCode)

		switch errorType {
		case "rate_limit":
			log.Printf("WhoisXML API 返回速率限制 (429)，将重试")
			retryDelay = time.Duration(float64(retryDelay) * 3)
			continue
		case "server":
			log.Printf("WhoisXML API 返回服务器错误 (5xx)，将重试")
			continue
		case "client":
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				log.Printf("WhoisXML API 返回客户端错误 (401/403)，将重试")
				continue
			}
			return nil, fmt.Errorf("WhoisXML API 返回客户端错误状态码: %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			body, _ := ioutil.ReadAll(resp.Body)
			log.Printf("WhoisXML API 返回非JSON格式: %s, 响应: %s (重试 %d/%d)",
				contentType, utils.TruncateString(string(body), 200), attempt+1, maxRetries+1)
			continue
		}

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("读取响应失败 (重试 %d/%d): %v", attempt+1, maxRetries+1, err)

			// 如果是上下文取消错误，但状态码正常，尝试重新读取不使用上下文
			if resp.StatusCode == http.StatusOK && strings.Contains(err.Error(), "context canceled") {
				log.Printf("发现上下文取消错误，尝试不使用上下文直接读取...")

				// 创建新的HTTP 请求，不使用上下文，增加超时时间
				directClient := &http.Client{Timeout: 30 * time.Second} // 增加到30秒
				directReq, directErr := http.NewRequest("GET", apiURL, nil)
				if directErr != nil {
					log.Printf("创建直接请求失败: %v", directErr)
					continue
				}

				// 添加头部
				directReq.Header.Set("User-Agent", "DomainWhoseek/1.0")
				directReq.Header.Set("Accept", "application/json")

				// 记录开始尝试直接请求
				log.Printf("开始直接请求，不使用原上下文，域名: %s", domain)
				directReqStart := time.Now()

				directResp, directErr := directClient.Do(directReq)
				directReqDuration := time.Since(directReqStart)

				if directErr != nil {
					log.Printf("直接请求失败: %v (耗时: %v)", directErr, directReqDuration)
					continue
				}
				defer directResp.Body.Close()

				// 检查响应状态码
				if directResp.StatusCode != http.StatusOK {
					log.Printf("直接请求返回非200状态码: %d (耗时: %v)", directResp.StatusCode, directReqDuration)
					continue
				}

				log.Printf("直接请求成功，状态码: %d，开始读取响应体 (耗时: %v)", directResp.StatusCode, directReqDuration)

				// 为读取操作单独设置超时
				readCtx, readCancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer readCancel()

				// 创建带超时的reader
				readErrChan := make(chan error, 1)
				bodyChan := make(chan []byte, 1)

				go func() {
					bodyBytes, readErr := ioutil.ReadAll(directResp.Body)
					if readErr != nil {
						readErrChan <- readErr
						return
					}
					bodyChan <- bodyBytes
				}()

				// 等待读取完成或超时
				select {
				case body = <-bodyChan:
					log.Printf("直接请求读取成功，获取响应内容长度: %d", len(body))
				case readErr := <-readErrChan:
					log.Printf("直接请求读取失败: %v", readErr)
					continue
				case <-readCtx.Done():
					log.Printf("直接请求读取超时")
					continue
				}
			} else {
				continue
			}
		}

		var whoisResp WhoisXMLResponse
		err = json.Unmarshal(body, &whoisResp)
		if err != nil {
			log.Printf("解析响应失败 (重试 %d/%d): %v, 响应: %s", attempt+1, maxRetries+1, err, utils.TruncateString(string(body), 200))
			continue
		}

		if whoisResp.WhoisRecord.DomainName == "" {
			log.Printf("响应验证失败：域名为空 (重试 %d/%d)", attempt+1, maxRetries+1)
			continue
		}

		statusStr := whoisResp.WhoisRecord.Status
		if statusStr == "" {
			statusStr = whoisResp.WhoisRecord.RegistryData.Status
		}
		statuses := strings.Fields(statusStr)

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

		var nameServers []string
		if len(whoisResp.WhoisRecord.NameServers.HostNames) > 0 {
			nameServers = whoisResp.WhoisRecord.NameServers.HostNames
		} else if len(whoisResp.WhoisRecord.RegistryData.NameServers.HostNames) > 0 {
			nameServers = whoisResp.WhoisRecord.RegistryData.NameServers.HostNames
		}

		result := &types.WhoisResponse{
			Available:   false,
			Domain:      whoisResp.WhoisRecord.DomainName,
			Registrar:   whoisResp.WhoisRecord.RegistrarName,
			CreateDate:  createdDate,
			ExpiryDate:  expiresDate,
			Status:      statuses,
			UpdateDate:  updatedDate,
			NameServers: nameServers,
		}

		log.Printf("WhoisXML 查询结果 - 域名: %s, 注册商: %s, 创建日期: %s, 到期日期: %s",
			result.Domain,
			result.Registrar,
			result.CreateDate,
			result.ExpiryDate)

		return result, nil
	}

	return nil, fmt.Errorf("WhoisXML API 请求失败，已重试 %d 次", maxRetries)
}
