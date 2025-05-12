/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-18 00:57:29
 * @Description: WhoisFreaks提供商
 */
package providers

import (
	"context"
	"dmainwhoseek/types"
	"dmainwhoseek/utils"
	"encoding/json"
	"errors"
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

type WhoisFreaksResponse struct {
	Status           interface{} `json:"status"` // 使用interface{}类型以适应不同的返回值类型
	DomainName       string      `json:"domain_name"`
	DomainRegistered string      `json:"domain_registered"`
	CreateDate       string      `json:"create_date"`
	UpdateDate       string      `json:"update_date"`
	ExpiryDate       string      `json:"expiry_date"`
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
			Timeout: 30 * time.Second, // 增加到30秒
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     60 * time.Second,
				TLSHandshakeTimeout: 15 * time.Second, // 增加到15秒
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second, // 增加到10秒
					KeepAlive: 60 * time.Second,
				}).DialContext,
				DisableKeepAlives: false,
				ForceAttemptHTTP2: true, // 启用HTTP/2
			},
		},
	}
}

func (p *WhoisFreaksProvider) Name() string {
	return "WhoisFreaks"
}

// 添加响应验证
func (p *WhoisFreaksProvider) parseResponse(body []byte) error {
	var raw struct {
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

func (p *WhoisFreaksProvider) classifyError(err error, statusCode int) string {
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

	// 基于状态码分类
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

	// 改进重试逻辑
	var (
		resp *http.Response
		body []byte
	)

	maxRetries := 3               // 增加到3次重试
	retryDelay := 1 * time.Second // 初始重试延迟降低

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("WhoisFreaks API 重试请求 #%d，域名: %s，延迟: %v", attempt, domain, retryDelay)
			time.Sleep(retryDelay)
			retryDelay = time.Duration(float64(retryDelay) * 1.5) // 使用1.5倍退避而不是2倍
		}

		// 创建带有5秒超时的上下文
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
		cancel() // 请求完成后立即取消上下文

		// 记录请求时间
		log.Printf("WhoisFreaks API 请求耗时: %v (尝试 %d/%d)", requestTime, attempt+1, maxRetries+1)

		if err != nil {
			errorType := p.classifyError(err, 0)
			log.Printf("WhoisFreaks API 请求失败 (%s) (尝试 %d/%d): %v", errorType, attempt+1, maxRetries+1, err)

			// 网络连接错误的退避时间更长
			if errorType == "connection" || errorType == "network" {
				retryDelay = time.Duration(float64(retryDelay) * 2)
			}
			continue // 重试
		}

		defer resp.Body.Close()

		// 检查状态码
		log.Printf("WhoisFreaks API 响应状态码: %d (尝试 %d/%d)", resp.StatusCode, attempt+1, maxRetries+1)

		// 基于状态码分类错误并决定是否重试
		errorType := p.classifyError(nil, resp.StatusCode)

		// 处理不同类型的错误
		switch errorType {
		case "rate_limit":
			// 速率限制，增加更长的退避
			log.Printf("WhoisFreaks API 返回速率限制（429），将使用更长时间重试")
			retryDelay = time.Duration(float64(retryDelay) * 3) // 更长的退避
			continue
		case "server":
			// 服务器错误，正常重试
			log.Printf("WhoisFreaks API 返回服务器错误（5xx），将重试")
			continue
		case "client":
			// 客户端错误，通常不重试，除非是401或403可能是临时的
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				log.Printf("WhoisFreaks API 返回授权错误（%d），尝试重试", resp.StatusCode)
				continue
			}
			// 其他客户端错误不重试
			return nil, fmt.Errorf("WhoisFreaks API 返回客户端错误状态码: %d", resp.StatusCode)
		}

		// 响应状态码正常，读取响应体
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("读取响应失败 (尝试 %d/%d): %v", attempt+1, maxRetries+1, err)
			continue // 重试
		}

		// 尝试解析响应
		var whoisResp WhoisFreaksResponse
		err = json.Unmarshal(body, &whoisResp)
		if err != nil {
			log.Printf("解析响应失败 (尝试 %d/%d): %v, 响应体: %s", attempt+1, maxRetries+1, err, utils.TruncateString(string(body), 200))
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
		log.Printf("WhoisFreaks 查询结果 - 域名: %s, 注册商: %s, 创建日期: %s, 到期日期: %s",
			result.Domain,
			result.Registrar,
			result.CreateDate,
			result.ExpiryDate)

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
