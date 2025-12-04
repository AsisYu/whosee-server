/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-18 00:57:29
 * @Description: Whois查询处理程序
 */
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"whosee/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// WhoisResponse 结构体用于解析 API 响应
type WhoisResponse struct {
	Status           bool   `json:"status"`
	DomainName       string `json:"domain_name"`
	QueryTime        string `json:"query_time"`
	WhoisServer      string `json:"whois_server"`
	DomainRegistered string `json:"domain_registered"`
	CreateDate       string `json:"create_date"`
	UpdateDate       string `json:"update_date"`
	ExpiryDate       string `json:"expiry_date"`
	DomainRegistrar  struct {
		IanaID        string `json:"iana_id"`
		RegistrarName string `json:"registrar_name"`
		WhoisServer   string `json:"whois_server"`
		WebsiteURL    string `json:"website_url"`
		EmailAddress  string `json:"email_address"`
		PhoneNumber   string `json:"phone_number"`
	} `json:"domain_registrar"`
	NameServers  []string `json:"name_servers"`
	DomainStatus []string `json:"domain_status"`
}

// 添加限流器
var rateLimiter = time.NewTicker(1 * time.Second) // 每秒最多一个请求

// ---------- 内部工具函数，降低圈复杂度 ----------
func getWhoisCache(ctx context.Context, rdb *redis.Client, key string) (gin.H, bool) {
	if rdb == nil {
		return nil, false
	}
	cachedData, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, false
	}
	var response gin.H
	if json.Unmarshal([]byte(cachedData), &response) == nil {
		response["isCached"] = true
		response["cacheTime"] = time.Now().Format("2006-01-02 15:04:05")
		return response, true
	}
	return nil, false
}

func setWhoisCache(ctx context.Context, rdb *redis.Client, key string, response gin.H) {
	if rdb == nil {
		return
	}
	data, err := json.Marshal(response)
	if err != nil {
		return
	}
	isRegistered := false
	if v, ok := response["available"].(bool); ok {
		isRegistered = !v
	}
	hasError := false
	_ = setCache(ctx, rdb, key, data, isRegistered, hasError)
}

func buildWhoisFreaksURL(apiKey, domain string) string {
	return fmt.Sprintf(
		"https://api.whoisfreaks.com/v1.0/whois?apiKey=%s&whois=live&domainName=%s",
		url.QueryEscape(apiKey), url.QueryEscape(domain),
	)
}

func maskKeyInURL(fullURL, apiKey string) string {
	if apiKey == "" {
		return fullURL
	}
	return strings.Replace(fullURL, apiKey, "HIDDEN", 1)
}

func doHTTPGet(url string) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "DomainWhoseek/1.0")
	return client.Do(req)
}

func parseWhoisFreaksResponse(body []byte) (gin.H, error) {
	var whoisResp WhoisResponse
	if err := json.Unmarshal(body, &whoisResp); err != nil {
		return nil, err
	}
	return gin.H{
		"available":    whoisResp.DomainRegistered != "yes",
		"domain":       whoisResp.DomainName,
		"registrar":    whoisResp.DomainRegistrar.RegistrarName,
		"creationDate": whoisResp.CreateDate,
		"expiryDate":   whoisResp.ExpiryDate,
		"status":       whoisResp.DomainStatus,
		"nameServers":  whoisResp.NameServers,
		"updatedDate":  whoisResp.UpdateDate,
		"isCached":     false,
		"cacheTime":    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func WhoisQuery(c *gin.Context, rdb *redis.Client) {
	// 从上下文中获取域名
	domain, exists := c.Get("domain")
	if !exists {
		log.Printf("WhoisQuery: 域名未在上下文中找到")
		c.JSON(400, gin.H{"error": "Domain not found"})
		return
	}
	domainStr := domain.(string)

	// 先查缓存
	cacheKey := utils.BuildCacheKey("cache", "whois", utils.SanitizeDomain(domainStr))
	if cached, ok := getWhoisCache(c.Request.Context(), rdb, cacheKey); ok {
		log.Printf("WhoisQuery: 返回缓存数据，域名: %s", domainStr)
		c.Header("X-Cache", "HIT")
		c.JSON(200, cached)
		return
	}

	// 速率限制
	<-rateLimiter.C

	// API key 与 URL
	apiKey := strings.TrimSpace(os.Getenv("WHOISFREAKS_API_KEY"))
	if len(apiKey) >= 8 {
		log.Printf("WhoisQuery: 使用的 API key 前缀: %s...", apiKey[:8])
	} else {
		log.Printf("WhoisQuery: 使用的 API key 长度: %d", len(apiKey))
	}
	apiURL := buildWhoisFreaksURL(apiKey, domainStr)
	log.Printf("WhoisQuery: 请求URL (隐藏key): %s", maskKeyInURL(apiURL, apiKey))
	log.Printf("WhoisQuery: 开始查询API，域名: %s", domainStr)

	// 请求
	resp, err := doHTTPGet(apiURL)
	if err != nil {
		log.Printf("WhoisQuery: API请求失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to query whois API"})
		return
	}
	defer resp.Body.Close()

	// 非200处理
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("WhoisQuery: API返回错误: 状态码=%d, 响应=%s", resp.StatusCode, string(body))
		if resp.StatusCode == 401 {
			c.JSON(500, gin.H{"error": "API authentication failed"})
		} else {
			c.JSON(resp.StatusCode, gin.H{"error": "API request failed"})
		}
		return
	}

	// 正常读取
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WhoisQuery: 读取API响应失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to read API response"})
		return
	}
	log.Printf("WhoisQuery: API响应: %s", string(body))

	// 解析
	response, err := parseWhoisFreaksResponse(body)
	if err != nil {
		log.Printf("WhoisQuery: 解析API响应失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to parse response"})
		return
	}

	// 缓存
	setWhoisCache(c.Request.Context(), rdb, cacheKey, response)

	c.Header("X-Cache", "MISS")
	response["isCached"] = false
	c.JSON(200, response)
}
