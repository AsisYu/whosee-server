/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-18 00:57:29
 * @LastEditors: AsisYu 2773943729@qq.com
 * @LastEditTime: 2025-01-18 01:33:11
 * @FilePath: \dmainwhoseek\server\handlers\whois.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
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

	"github.com/gin-gonic/gin"
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

func WhoisQuery(c *gin.Context) {
	// 从上下文中获取域名
	domain, exists := c.Get("domain")
	if !exists {
		log.Printf("WhoisQuery: 域名未在上下文中找到")
		c.JSON(400, gin.H{"error": "Domain not found"})
		return
	}

	domainStr := domain.(string)

	// 尝试从 Redis 获取缓存
	cacheKey := fmt.Sprintf("whois:%s", domainStr)
	if cachedData, err := rdb.Get(context.Background(), cacheKey).Result(); err == nil {
		var response gin.H
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("WhoisQuery: 返回缓存数据，域名: %s", domainStr)
			c.Header("X-Cache", "HIT")
			c.JSON(200, response)
			return
		}
	}

	// 获取并清理 API key
	apiKey := strings.TrimSpace(os.Getenv("WHOISFREAKS_API_KEY"))

	// 使用限流器
	<-rateLimiter.C

	// 打印 API key 的前几个字符（用于调试，不要打印完整的 key）
	log.Printf("WhoisQuery: 使用的 API key 前缀: %s...", apiKey[:8])

	// 构建并发送请求
	apiURL := fmt.Sprintf("https://api.whoisfreaks.com/v1.0/whois?apiKey=%s&whois=live&domainName=%s",
		url.QueryEscape(apiKey), // 确保 API key 被正确编码
		url.QueryEscape(domainStr))

	log.Printf("WhoisQuery: 请求URL (隐藏key): %s",
		strings.Replace(apiURL, apiKey, "HIDDEN", 1))

	log.Printf("WhoisQuery: 开始查询API，域名: %s", domainStr)

	// 构建 HTTP 客户端
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("WhoisQuery: 创建请求失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to create request"})
		return
	}

	// 添加用户代理
	req.Header.Set("User-Agent", "DomainWhoseek/1.0")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("WhoisQuery: API请求失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to query whois API"})
		return
	}
	defer resp.Body.Close()

	// 检查响应状态码
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WhoisQuery: 读取API响应失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to read API response"})
		return
	}

	log.Printf("WhoisQuery: API响应: %s", string(body))

	// 解析响应
	var whoisResp WhoisResponse
	if err := json.Unmarshal(body, &whoisResp); err != nil {
		log.Printf("WhoisQuery: 解析API响应失败: %v", err)
		c.JSON(500, gin.H{"error": "Failed to parse response"})
		return
	}

	// 构建响应
	response := gin.H{
		"available":    whoisResp.DomainRegistered != "yes",
		"domain":       whoisResp.DomainName,
		"registrar":    whoisResp.DomainRegistrar.RegistrarName,
		"creationDate": whoisResp.CreateDate,
		"expiryDate":   whoisResp.ExpiryDate,
		"status":       whoisResp.DomainStatus,
		"nameServers":  whoisResp.NameServers,
		"updatedDate":  whoisResp.UpdateDate,
	}

	// 缓存结果
	if resultJSON, err := json.Marshal(response); err == nil {
		isRegistered := !response["available"].(bool)
		hasError := false // 可以根据实际情况设置
		setCache(c.Request.Context(), rdb, cacheKey, resultJSON, isRegistered, hasError)
	}

	c.Header("X-Cache", "MISS")
	c.JSON(200, response)
}
