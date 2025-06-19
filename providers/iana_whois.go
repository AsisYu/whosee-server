/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-19 10:15:00
 * @Description: IANA WHOIS 提供商 - 基于TCP端口43的传统WHOIS查询
 */
package providers

import (
	"dmainwhoseek/types"
	"dmainwhoseek/utils"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

type IANAWhoisProvider struct {
	timeout time.Duration
}

func NewIANAWhoisProvider() *IANAWhoisProvider {
	return &IANAWhoisProvider{
		timeout: 10 * time.Second,
	}
}

func (p *IANAWhoisProvider) Name() string {
	return "IANA-WHOIS"
}

func (p *IANAWhoisProvider) Query(domain string) (*types.WhoisResponse, error, bool) {
	log.Printf("使用 IANA WHOIS (端口43) 查询域名: %s", domain)

	// 验证域名格式
	if !utils.IsValidDomain(domain) {
		return &types.WhoisResponse{
			Domain:         domain,
			Available:      false,
			StatusCode:     422,
			StatusMessage:  "无效的域名格式",
			SourceProvider: p.Name(),
		}, fmt.Errorf("无效的域名格式: %s", domain), false
	}

	// 提取顶级域名
	tld := p.extractTLD(domain)
	if tld == "" {
		return &types.WhoisResponse{
			Domain:         domain,
			Available:      false,
			StatusCode:     422,
			StatusMessage:  "无法提取顶级域名",
			SourceProvider: p.Name(),
		}, fmt.Errorf("无法提取顶级域名: %s", domain), false
	}

	// 首先查询IANA获取权威WHOIS服务器
	whoisServer, err := p.queryIANAForTLD(tld)
	if err != nil {
		log.Printf("查询IANA失败: %v", err)
		return &types.WhoisResponse{
			Domain:         domain,
			Available:      false,
			StatusCode:     500,
			StatusMessage:  "查询IANA失败",
			SourceProvider: p.Name(),
		}, err, false
	}

	if whoisServer == "" {
		return &types.WhoisResponse{
			Domain:         domain,
			Available:      false,
			StatusCode:     404,
			StatusMessage:  "未找到权威WHOIS服务器",
			SourceProvider: p.Name(),
		}, fmt.Errorf("未找到 %s 的权威WHOIS服务器", tld), false
	}

	log.Printf("找到权威WHOIS服务器: %s", whoisServer)

	// 查询权威WHOIS服务器
	whoisData, err := p.queryWhoisServer(whoisServer, domain)
	if err != nil {
		log.Printf("查询权威WHOIS服务器失败: %v", err)
		return &types.WhoisResponse{
			Domain:         domain,
			Available:      false,
			StatusCode:     500,
			StatusMessage:  "查询权威WHOIS服务器失败",
			SourceProvider: p.Name(),
		}, err, false
	}

	// 解析WHOIS数据
	response := p.parseWhoisData(whoisData, domain)
	response.WhoisServer = whoisServer
	response.SourceProvider = p.Name()

	log.Printf("IANA WHOIS 查询成功: 域名=%s, 注册商=%s, 创建日期=%s, 到期日期=%s",
		domain, response.Registrar, response.CreateDate, response.ExpiryDate)

	return response, nil, false
}

func (p *IANAWhoisProvider) extractTLD(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

func (p *IANAWhoisProvider) queryIANAForTLD(tld string) (string, error) {
	log.Printf("查询IANA获取 %s 的WHOIS服务器", tld)

	conn, err := net.DialTimeout("tcp", "whois.iana.org:43", p.timeout)
	if err != nil {
		return "", fmt.Errorf("连接IANA失败: %v", err)
	}
	defer conn.Close()

	// 设置读写超时
	conn.SetDeadline(time.Now().Add(p.timeout))

	// 发送查询
	_, err = conn.Write([]byte(tld + "\r\n"))
	if err != nil {
		return "", fmt.Errorf("发送查询失败: %v", err)
	}

	// 读取响应
	buffer := make([]byte, 4096)
	var response strings.Builder

	for {
		n, err := conn.Read(buffer)
		if n > 0 {
			response.Write(buffer[:n])
		}
		if err != nil {
			break
		}
	}

	responseText := response.String()
	log.Printf("IANA响应长度: %d 字节", len(responseText))

	// 从响应中提取WHOIS服务器
	whoisServer := p.extractWhoisServer(responseText)
	return whoisServer, nil
}

func (p *IANAWhoisProvider) extractWhoisServer(ianaResponse string) string {
	// 查找whois服务器信息
	lines := strings.Split(ianaResponse, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "whois:") {
			whoisServer := strings.TrimSpace(line[6:])
			log.Printf("从IANA响应中提取到WHOIS服务器: %s", whoisServer)
			return whoisServer
		}
	}
	return ""
}

func (p *IANAWhoisProvider) queryWhoisServer(server, domain string) (string, error) {
	log.Printf("查询WHOIS服务器: %s，域名: %s", server, domain)

	conn, err := net.DialTimeout("tcp", server+":43", p.timeout)
	if err != nil {
		return "", fmt.Errorf("连接WHOIS服务器失败: %v", err)
	}
	defer conn.Close()

	// 设置读写超时
	conn.SetDeadline(time.Now().Add(p.timeout))

	// 发送查询
	_, err = conn.Write([]byte(domain + "\r\n"))
	if err != nil {
		return "", fmt.Errorf("发送查询失败: %v", err)
	}

	// 读取响应
	buffer := make([]byte, 8192)
	var response strings.Builder

	for {
		n, err := conn.Read(buffer)
		if n > 0 {
			response.Write(buffer[:n])
		}
		if err != nil {
			break
		}
	}

	responseText := response.String()
	log.Printf("WHOIS服务器响应长度: %d 字节", len(responseText))

	return responseText, nil
}

func (p *IANAWhoisProvider) parseWhoisData(whoisData, domain string) *types.WhoisResponse {
	response := &types.WhoisResponse{
		Domain:        domain,
		Available:     false,
		StatusCode:    200,
		StatusMessage: "查询成功",
	}

	lines := strings.Split(whoisData, "\n")

	// 检查域名是否可用
	lowerData := strings.ToLower(whoisData)
	if strings.Contains(lowerData, "no match") ||
		strings.Contains(lowerData, "not found") ||
		strings.Contains(lowerData, "no data found") {
		response.Available = true
		response.StatusMessage = "域名可用"
		return response
	}

	// 解析各个字段
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, ">>>") {
			continue
		}

		// 分割键值对
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "registrar", "sponsoring registrar":
			response.Registrar = value
		case "creation date", "created", "domain name commencement date", "created on":
			response.CreateDate = p.parseDate(value)
		case "registry expiry date", "expiry date", "expires", "expires on", "expiration date":
			response.ExpiryDate = p.parseDate(value)
		case "updated date", "last modified", "last updated", "modified":
			response.UpdateDate = p.parseDate(value)
		case "domain status", "status":
			if response.Status == nil {
				response.Status = []string{}
			}
			response.Status = append(response.Status, value)
		case "name server", "nserver":
			if response.NameServers == nil {
				response.NameServers = []string{}
			}
			response.NameServers = append(response.NameServers, value)
		case "registrant email", "admin email", "contact email":
			response.ContactEmail = value
		}
	}

	// 计算域名年龄
	if response.CreateDate != "" {
		response.DomainAge = p.calculateDomainAge(response.CreateDate)
	}

	return response
}

func (p *IANAWhoisProvider) parseDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	// 清理日期字符串
	dateStr = strings.TrimSpace(dateStr)

	// 移除可能的后缀信息（如时区、UTC等）
	if idx := strings.Index(dateStr, "("); idx != -1 {
		dateStr = strings.TrimSpace(dateStr[:idx])
	}

	// 尝试多种日期格式
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02-Jan-2006",
		"2006.01.02",
		"02/01/2006",
		"01/02/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.Format("2006-01-02")
		}
	}

	// 如果无法解析，返回原始字符串
	return dateStr
}

func (p *IANAWhoisProvider) calculateDomainAge(createDateStr string) int {
	if createDateStr == "" {
		return 0
	}

	createDate, err := time.Parse("2006-01-02", createDateStr)
	if err != nil {
		return 0
	}

	return int(time.Since(createDate).Hours() / 24)
}
