/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-09 12:15:00
 * @LastEditors: AsisYu 2773943729@qq.com
 * @LastEditTime: 2025-04-09 12:15:00
 * @Description: IP白名单和API Key验证中间件
 */
package middleware

import (
	"net"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// 可信IP白名单
var trustedIPs = []string{
	"127.0.0.1",    // 本地回环地址
	"::1",          // IPv6本地回环地址
	"10.0.0.0/8",   // 私有网络
	"172.16.0.0/12", // 私有网络
	"192.168.0.0/16", // 私有网络
	"104.21.88.5",
	"172.67.149.223",
}

// IsWhitelistedIP 检查IP是否在白名单中
func IsWhitelistedIP(ip string) bool {
	// 检查环境变量中是否有自定义白名单配置
	if envIPs := os.Getenv("TRUSTED_IPS"); envIPs != "" {
		customIPs := strings.Split(envIPs, ",")
		for _, customIP := range customIPs {
			customIP = strings.TrimSpace(customIP)
			if ip == customIP {
				return true
			}
			
			// 检查CIDR范围
			if strings.Contains(customIP, "/") {
				_, ipNet, err := net.ParseCIDR(customIP)
				if err == nil {
					parsedIP := net.ParseIP(ip)
					if parsedIP != nil && ipNet.Contains(parsedIP) {
						return true
					}
				}
			}
		}
	}

	// 检查默认白名单
	for _, trustedIP := range trustedIPs {
		if ip == trustedIP {
			return true
		}
		
		// 检查CIDR范围
		if strings.Contains(trustedIP, "/") {
			_, ipNet, err := net.ParseCIDR(trustedIP)
			if err == nil {
				parsedIP := net.ParseIP(ip)
				if parsedIP != nil && ipNet.Contains(parsedIP) {
					return true
				}
			}
		}
	}
	
	return false
}

// HasValidKey 检查请求是否包含有效的API Key
func HasValidKey(c *gin.Context) bool {
	apiKey := c.GetHeader("X-API-KEY")
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}
	
	if apiKey == "" {
		return false
	}
	
	// 从环境变量读取API Key
	validKey := os.Getenv("API_KEY")
	if validKey == "" {
		// 开发环境默认Key
		validKey = "development-key"
	}
	
	return apiKey == validKey
}
