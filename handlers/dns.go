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

	"whosee/utils"

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

// 内部工具：读取缓存
func getDNSCache(ctx context.Context, rdb *redis.Client, key string) (*DNSResponse, bool) {
	if rdb == nil {
		return nil, false
	}
	cachedData, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, false
	}
	var resp DNSResponse
	if json.Unmarshal([]byte(cachedData), &resp) == nil && resp.Domain != "" && len(resp.Records) > 0 {
		resp.IsCached = true
		resp.CacheTime = time.Now().Format("2006-01-02 15:04:05")
		return &resp, true
	}
	return nil, false
}

// 内部工具：写入缓存
func setDNSCache(ctx context.Context, rdb *redis.Client, key string, resp *DNSResponse, ttl time.Duration) {
	if rdb == nil || resp == nil {
		return
	}
	if data, err := json.Marshal(resp); err == nil {
		_ = rdb.Set(ctx, key, data, ttl).Err()
	}
}

// 各类记录查询分解，降低圈复杂度
func queryAAndAAAA(domain string) []DNSRecord {
	var records []DNSRecord
	if ips, err := net.LookupIP(domain); err == nil {
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				records = append(records, DNSRecord{Type: "A", Value: ipv4.String()})
			} else {
				records = append(records, DNSRecord{Type: "AAAA", Value: ip.String()})
			}
		}
	}
	return records
}

func queryMX(domain string) []DNSRecord {
	var records []DNSRecord
	if mxs, err := net.LookupMX(domain); err == nil {
		for _, mx := range mxs {
			records = append(records, DNSRecord{Type: "MX", Value: fmt.Sprintf("%s (优先级: %d)", mx.Host, mx.Pref)})
		}
	}
	return records
}

func queryNS(domain string) []DNSRecord {
	var records []DNSRecord
	if nss, err := net.LookupNS(domain); err == nil {
		for _, ns := range nss {
			records = append(records, DNSRecord{Type: "NS", Value: ns.Host})
		}
	}
	return records
}

func queryTXT(domain string) []DNSRecord {
	var records []DNSRecord
	if txts, err := net.LookupTXT(domain); err == nil {
		for _, txt := range txts {
			records = append(records, DNSRecord{Type: "TXT", Value: txt})
		}
	}
	return records
}

func queryCNAME(domain string) []DNSRecord {
	var records []DNSRecord
	if cname, err := net.LookupCNAME(domain); err == nil && cname != domain+"." {
		records = append(records, DNSRecord{Type: "CNAME", Value: strings.TrimSuffix(cname, ".")})
	}
	return records
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
	cacheKey := utils.BuildCacheKey("cache", "dns", utils.SanitizeDomain(domainStr))
	log.Printf("DNSQuery: 尝试从Redis获取缓存，键: %s", cacheKey)
	if cached, ok := getDNSCache(context.Background(), rdb, cacheKey); ok {
		c.Header("X-Cache", "HIT")
		c.JSON(200, cached)
		return
	}

	// 查询各种DNS记录（分解后的调用）
	records := []DNSRecord{}
	records = append(records, queryAAndAAAA(domainStr)...)
	records = append(records, queryMX(domainStr)...)
	records = append(records, queryNS(domainStr)...)
	records = append(records, queryTXT(domainStr)...)
	records = append(records, queryCNAME(domainStr)...)

	// 构建响应
	response := &DNSResponse{
		Domain:    domainStr,
		Records:   records,
		QueryTime: time.Now().Format("2006-01-02 15:04:05"),
		IsCached:  false,
		CacheTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 缓存结果 (1小时)
	setDNSCache(context.Background(), rdb, cacheKey, response, 1*time.Hour)

	elapsedTime := time.Since(startTime)
	log.Printf("DNSQuery: 完成查询，域名: %s, 耗时: %v, 找到记录数: %d", domainStr, elapsedTime, len(records))

	c.Header("X-Cache", "MISS")
	// 确保响应中包含缓存状态字段
	response.IsCached = false
	c.JSON(200, response)
}
