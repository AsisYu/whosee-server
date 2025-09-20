/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: 域名工具函数
 */
package utils

import (
	"crypto/md5"
	"fmt"
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

// GenerateSecureFilename 生成安全的文件名
func GenerateSecureFilename(input string) string {
	// 移除或替换危险字符
	dangerous := regexp.MustCompile(`[^\w\-.]`)
	safe := dangerous.ReplaceAllString(input, "_")

	// 限制长度并生成哈希后缀
	if len(safe) > 50 {
		hash := fmt.Sprintf("%x", md5.Sum([]byte(input)))
		safe = safe[:40] + "_" + hash[:8]
	}

	// 移除开头的点（避免隐藏文件）
	safe = strings.TrimPrefix(safe, ".")

	// 确保不为空
	if safe == "" {
		hash := fmt.Sprintf("%x", md5.Sum([]byte(input)))
		safe = "file_" + hash[:12]
	}

	return safe
}

// ValidateURL 验证URL是否安全
func ValidateURL(url string) bool {
	// 检查是否包含危险的协议
	dangerousProtocols := []string{"file://", "ftp://", "javascript:", "data:"}
	urlLower := strings.ToLower(url)

	for _, protocol := range dangerousProtocols {
		if strings.HasPrefix(urlLower, protocol) {
			return false
		}
	}

	// 检查是否包含本地地址
	localAddresses := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1", "10.", "192.168.", "172."}
	for _, addr := range localAddresses {
		if strings.Contains(urlLower, addr) {
			return false
		}
	}

	return true
}
