/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: 域名工具函数
 */
package utils

import (
	"regexp"
	"strings"
)

// IsValidDomain 验证域名是否有效
func IsValidDomain(domain string) bool {
	// 忽略协议前缀
	domain = strings.TrimPrefix(strings.TrimPrefix(domain, "http://"), "https://")
	
	// 移除端口和路径
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	
	// 使用正则表达式验证域名格式
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	return domainRegex.MatchString(domain)
}

// SanitizeDomain 清理和标准化域名
func SanitizeDomain(domain string) string {
	// 去除协议前缀
	domain = strings.TrimPrefix(strings.TrimPrefix(domain, "http://"), "https://")
	
	// 移除端口和路径
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	
	// 转换为小写
	return strings.ToLower(domain)
}
