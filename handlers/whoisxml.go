package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// WhoisXMLResponse represents the WhoisXML API response structure
type WhoisXMLResponse struct {
	WhoisRecord struct {
		DomainName   string   `json:"domainName"`
		CreatedDate  string   `json:"createdDate"`
		UpdatedDate  string   `json:"updatedDate"`
		ExpiresDate  string   `json:"expiresDate"`
		Registrar    string   `json:"registrar"`
		DomainStatus []string `json:"domainStatus"`
		NameServers  struct {
			HostNames []string `json:"hostNames"`
		} `json:"nameServers"`
	} `json:"WhoisRecord"`
}

func WhoisXMLQuery(c *gin.Context) {
	domain, exists := c.Get("domain")
	if !exists {
		log.Printf("WhoisXMLQuery: 域名未在上下文中找到")
		c.JSON(400, gin.H{"error": "Domain not found"})
		return
	}

	domainStr := domain.(string)
	cacheKey := fmt.Sprintf("whois:%s", domainStr)

	// 检查缓存
	if cachedData, err := rdb.Get(context.Background(), cacheKey).Result(); err == nil {
		var response gin.H
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			c.Header("X-Cache", "HIT")
			c.JSON(200, response)
			return
		}
	}

	apiKey := strings.TrimSpace(os.Getenv("WHOISXML_API_KEY"))

	// 构建请求URL
	apiURL := fmt.Sprintf("https://www.whoisxmlapi.com/whoisserver/WhoisService?apiKey=%s&domainName=%s&outputFormat=JSON",
		apiKey, domainStr)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("WhoisXMLQuery: 创建请求失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("User-Agent", "DomainWhoseek/1.0")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("WhoisXMLQuery: API请求失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to query WhoisXML API"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WhoisXMLQuery: 读取响应失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to read response"})
		return
	}

	var whoisResp WhoisXMLResponse
	if err := json.Unmarshal(body, &whoisResp); err != nil {
		log.Printf("WhoisXMLQuery: 解析响应失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to parse response"})
		return
	}

	// 检查域名是否已注册
	isAvailable := len(whoisResp.WhoisRecord.DomainStatus) == 0

	response := gin.H{
		"available":    isAvailable,
		"domain":       whoisResp.WhoisRecord.DomainName,
		"registrar":    whoisResp.WhoisRecord.Registrar,
		"creationDate": whoisResp.WhoisRecord.CreatedDate,
		"expiryDate":   whoisResp.WhoisRecord.ExpiresDate,
		"status":       whoisResp.WhoisRecord.DomainStatus,
		"nameServers":  whoisResp.WhoisRecord.NameServers.HostNames,
		"updatedDate":  whoisResp.WhoisRecord.UpdatedDate,
	}

	// 缓存结果
	if resultJSON, err := json.Marshal(response); err == nil {
		cacheDuration := REGISTERED_DOMAIN_CACHE_TIME
		if isAvailable {
			cacheDuration = UNREGISTERED_DOMAIN_CACHE_TIME
		}
		rdb.Set(context.Background(), cacheKey, resultJSON, cacheDuration)
	}

	c.Header("X-Cache", "MISS")
	c.JSON(200, response)
}
