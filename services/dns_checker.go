/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-09 12:15:00
 * @Description: DNS检查服务
 */
package services

import (
	"context"
	"whosee/utils"
	"fmt"
	"net"
	"time"
)

// DNSChecker 实现DNS服务的健康检查
type DNSChecker struct {
	lastCheckTime time.Time
	servers       []string
	testDomains   []string
	healthLogger  *utils.HealthLogger
}

// NewDNSChecker 创建一个新的DNS检查器
func NewDNSChecker() *DNSChecker {
	return &DNSChecker{
		servers: []string{
			"8.8.8.8:53",         // Google DNS
			"1.1.1.1:53",         // Cloudflare DNS
			"114.114.114.114:53", // 国内DNS
		},
		testDomains: []string{
			"google.com",
			"baidu.com",
			"github.com",
		},
		healthLogger: utils.GetHealthLogger(),
	}
}

// GetLastCheckTime 返回上次检查时间
func (dc *DNSChecker) GetLastCheckTime() time.Time {
	return dc.lastCheckTime
}

// TestDNSHealth 测试DNS服务的健康状态
func (dc *DNSChecker) TestDNSHealth() map[string]interface{} {
	results := make(map[string]interface{})

	dc.healthLogger.Println("开始测试DNS服务健康状态...")

	totalServers := len(dc.servers)
	availableServers := 0

	for _, server := range dc.servers {
		serverResult := map[string]interface{}{
			"available":      false,
			"responseTime":   0,
			"testSuccessful": false,
			"testResults":    make([]map[string]interface{}, 0),
		}

		// 为每个服务器测试一个随机域名
		testDomain := dc.testDomains[time.Now().Nanosecond()%len(dc.testDomains)]

		// 创建自定义解析器
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Second * 5,
				}
				return d.DialContext(ctx, "udp", server)
			},
		}

		startTime := time.Now()
		testResult := map[string]interface{}{
			"domain":       testDomain,
			"timestamp":    startTime.UTC().Format(time.RFC3339),
			"success":      false,
			"message":      "",
			"responseTime": 0,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 尝试查询域名的IP
		ips, err := r.LookupHost(ctx, testDomain)

		responseTime := time.Since(startTime)
		testResult["responseTime"] = responseTime.Milliseconds()
		serverResult["responseTime"] = responseTime.Milliseconds()

		if err != nil {
			testResult["message"] = fmt.Sprintf("DNS查询失败: %v", err)
		} else if len(ips) == 0 {
			testResult["message"] = "DNS查询返回空结果"
		} else {
			testResult["success"] = true
			testResult["message"] = fmt.Sprintf("DNS查询成功，返回%d个IP地址", len(ips))
			testResult["ips"] = ips[:min(5, len(ips))] // 最多显示5个IP

			serverResult["available"] = true
			serverResult["testSuccessful"] = true
			availableServers++
		}

		// 添加测试结果
		serverResults := serverResult["testResults"].([]map[string]interface{})
		serverResults = append(serverResults, testResult)
		serverResult["testResults"] = serverResults

		// 服务器名称格式化
		serverName := server
		switch server {
		case "8.8.8.8:53":
			serverName = "GoogleDNS"
		case "1.1.1.1:53":
			serverName = "CloudflareDNS"
		case "114.114.114.114:53":
			serverName = "中国DNS"
		}

		results[serverName] = serverResult
	}

	// 更新最后检查时间
	dc.lastCheckTime = time.Now()

	dc.healthLogger.Printf("DNS服务检查完成: 共%d个服务器, %d个可用", totalServers, availableServers)

	return results
}

// GetDNSStatus 获取DNS服务的状态
func (dc *DNSChecker) GetDNSStatus() string {
	results := dc.TestDNSHealth()

	totalServers := len(results)
	availableServers := 0

	for _, result := range results {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if available, ok := resultMap["available"].(bool); ok && available {
				availableServers++
			}
		}
	}

	if totalServers == 0 {
		return "unknown"
	} else if availableServers == 0 {
		return "down"
	} else if availableServers < totalServers {
		return "degraded"
	}
	return "up"
}

// CheckAllServers 检查所有DNS服务器并返回结果数组
func (dc *DNSChecker) CheckAllServers() []interface{} {
	testResults := dc.TestDNSHealth()
	servers := make([]interface{}, 0, len(testResults))

	// 将map转换为数组
	for serverName, result := range testResults {
		if resultMap, ok := result.(map[string]interface{}); ok {
			// 设置服务器名称
			resultMap["name"] = serverName

			// 设置状态
			if available, ok := resultMap["available"].(bool); ok {
				if available {
					resultMap["status"] = "up"
				} else {
					resultMap["status"] = "down"
				}
			} else {
				resultMap["status"] = "unknown"
			}

			servers = append(servers, resultMap)
		}
	}

	dc.healthLogger.Printf("DNS检查完成，共检查了%d个服务器", len(servers))
	return servers
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
