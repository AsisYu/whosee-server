/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-03-31 02:25:00
 * @Description: DNS查询处理器
 */
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// DNSRecord 表示DNS记录
type DNSRecord struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   uint32 `json:"ttl,omitempty"`
}

// DNSResponse 表示DNS查询响应
type DNSResponse struct {
	Domain    string      `json:"domain"`
	Records   []DNSRecord `json:"records"`
	QueryTime string      `json:"query_time"`
	IsCached  bool        `json:"is_cached"`
	CacheTime string      `json:"cache_time"`
}

// DNSQuery 处理DNS查询请求
func DNSQuery(c *gin.Context, rdb *redis.Client) {
	startTime := time.Now()
	// 从上下文中获取域名
	domain, exists := c.Get("domain")
	if !exists {
		log.Printf("DNSQuery: 域名未在上下文中找到")
		c.JSON(400, gin.H{"error": "Domain not found"})
		return
	}

	domainStr := domain.(string)
	log.Printf("DNSQuery: 开始查询域名: %s", domainStr)

	// 尝试从Redis获取缓存
	cacheKey := fmt.Sprintf("dns:%s", domainStr)
	log.Printf("DNSQuery: 尝试从Redis获取缓存，键: %s", cacheKey)
	if cachedData, err := rdb.Get(context.Background(), cacheKey).Result(); err == nil {
		var response DNSResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			// 验证缓存数据格式是否正确
			if len(response.Records) == 0 || response.Domain == "" {
				log.Printf("DNSQuery: 缓存数据格式不正确，将重新查询: %+v", response)
			} else {
				log.Printf("DNSQuery: 返回缓存数据，域名: %s, 记录数: %d", domainStr, len(response.Records))
				c.Header("X-Cache", "HIT")
				// 更新缓存状态
				response.IsCached = true
				response.CacheTime = time.Now().Format("2006-01-02 15:04:05")
				c.JSON(200, response)
				return
			}
		} else {
			log.Printf("DNSQuery: 缓存数据解析失败: %v, 原始数据: %s", err, cachedData)
		}
	} else {
		log.Printf("DNSQuery: 缓存未命中，将进行DNS查询: %v", err)
	}

	// 查询各种DNS记录
	records := []DNSRecord{}
	log.Printf("DNSQuery: 开始查询各种DNS记录，域名: %s", domainStr)

	// 查询A记录
	log.Printf("DNSQuery: 查询A/AAAA记录，域名: %s", domainStr)
	if ips, err := net.LookupIP(domainStr); err == nil {
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				records = append(records, DNSRecord{
					Type:  "A",
					Value: ipv4.String(),
				})
				log.Printf("DNSQuery: 找到A记录: %s", ipv4.String())
			} else {
				records = append(records, DNSRecord{
					Type:  "AAAA",
					Value: ip.String(),
				})
				log.Printf("DNSQuery: 找到AAAA记录: %s", ip.String())
			}
		}
	} else {
		log.Printf("DNSQuery: A/AAAA记录查询失败: %v", err)
	}

	// 查询MX记录
	log.Printf("DNSQuery: 查询MX记录，域名: %s", domainStr)
	if mxs, err := net.LookupMX(domainStr); err == nil {
		for _, mx := range mxs {
			records = append(records, DNSRecord{
				Type:  "MX",
				Value: fmt.Sprintf("%s (优先级: %d)", mx.Host, mx.Pref),
			})
			log.Printf("DNSQuery: 找到MX记录: %s (优先级: %d)", mx.Host, mx.Pref)
		}
	} else {
		log.Printf("DNSQuery: MX记录查询失败: %v", err)
	}

	// 查询NS记录
	log.Printf("DNSQuery: 查询NS记录，域名: %s", domainStr)
	if nss, err := net.LookupNS(domainStr); err == nil {
		for _, ns := range nss {
			records = append(records, DNSRecord{
				Type:  "NS",
				Value: ns.Host,
			})
			log.Printf("DNSQuery: 找到NS记录: %s", ns.Host)
		}
	} else {
		log.Printf("DNSQuery: NS记录查询失败: %v", err)
	}

	// 查询TXT记录
	log.Printf("DNSQuery: 查询TXT记录，域名: %s", domainStr)
	if txts, err := net.LookupTXT(domainStr); err == nil {
		for _, txt := range txts {
			records = append(records, DNSRecord{
				Type:  "TXT",
				Value: txt,
			})
			log.Printf("DNSQuery: 找到TXT记录: %s", txt)
		}
	} else {
		log.Printf("DNSQuery: TXT记录查询失败: %v", err)
	}

	// 查询CNAME记录
	log.Printf("DNSQuery: 查询CNAME记录，域名: %s", domainStr)
	if cname, err := net.LookupCNAME(domainStr); err == nil && cname != domainStr+"." {
		records = append(records, DNSRecord{
			Type:  "CNAME",
			Value: strings.TrimSuffix(cname, "."),
		})
		log.Printf("DNSQuery: 找到CNAME记录: %s", strings.TrimSuffix(cname, "."))
	} else if err != nil {
		log.Printf("DNSQuery: CNAME记录查询失败: %v", err)
	}

	// 构建响应
	response := DNSResponse{
		Domain:    domainStr,
		Records:   records,
		QueryTime: time.Now().Format("2006-01-02 15:04:05"),
		IsCached:  false,
		CacheTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 缓存结果 (1小时)
	if resultJSON, err := json.Marshal(response); err == nil {
		rdb.Set(context.Background(), cacheKey, resultJSON, 1*time.Hour)
		log.Printf("DNSQuery: 缓存DNS查询结果，域名: %s, 有效期: 1小时", domainStr)
	} else {
		log.Printf("DNSQuery: 缓存DNS查询结果失败: %v", err)
	}

	elapsedTime := time.Since(startTime)
	log.Printf("DNSQuery: 完成查询，域名: %s, 耗时: %v, 找到记录数: %d", domainStr, elapsedTime, len(records))

	c.Header("X-Cache", "MISS")
	// 确保响应中包含缓存状态字段
	response.IsCached = false
	c.JSON(200, response)
}
