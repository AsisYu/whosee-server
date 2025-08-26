/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-29 12:15:00
 * @Description: 健康检查服务
 */
package services

import (
	"dmainwhoseek/utils"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"
)

// HealthChecker 是负责执行定期健康检查的服务
type HealthChecker struct {
	WhoisManager      *WhoisManager
	DNSChecker        *DNSChecker
	ScreenshotChecker *ScreenshotChecker
	ITDogChecker      *ITDogChecker
	CheckIntervalDays int
	stopChan          chan struct{}
	lastCheckTime     time.Time
	lastCheckResults  map[string]interface{}
	mutex             sync.RWMutex
	healthLogger      *utils.HealthLogger
}

// NewHealthChecker 创建一个新的健康检查器实例
func NewHealthChecker(whoisManager *WhoisManager, dnsChecker *DNSChecker, screenshotChecker *ScreenshotChecker, itdogChecker *ITDogChecker) *HealthChecker {
	// 从环境变量读取检查间隔，默认为1天
	intervalStr := os.Getenv("HEALTH_CHECK_INTERVAL_DAYS")
	interval := 1 // 默认值

	if intervalStr != "" {
		if i, err := strconv.Atoi(intervalStr); err == nil && i > 0 {
			interval = i
		}
	}

	return &HealthChecker{
		WhoisManager:      whoisManager,
		DNSChecker:        dnsChecker,
		ScreenshotChecker: screenshotChecker,
		ITDogChecker:      itdogChecker,
		CheckIntervalDays: interval,
		stopChan:          make(chan struct{}),
		lastCheckResults:  make(map[string]interface{}),
		healthLogger:      utils.GetHealthLogger(),
	}
}

// Start 开始定期健康检查
func (hc *HealthChecker) Start() {
	hc.healthLogger.Printf("启动定期健康检查服务，检查间隔: %d天", hc.CheckIntervalDays)

	// 立即执行一次检查
	hc.RunHealthCheck()

	// 启动定时检查
	go func() {
		ticker := time.NewTicker(time.Duration(hc.CheckIntervalDays) * 24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hc.RunHealthCheck()
			case <-hc.stopChan:
				hc.healthLogger.Println("健康检查服务已停止")
				return
			}
		}
	}()
}

// Stop 停止定期健康检查
func (hc *HealthChecker) Stop() {
	close(hc.stopChan)
}

// ForceRefresh 强制执行健康检查（仅供内部使用）
func (hc *HealthChecker) ForceRefresh() {
	hc.healthLogger.Println("执行强制健康检查刷新")
	hc.RunHealthCheck()
}

// RunHealthCheck 执行一次完整的健康检查
func (hc *HealthChecker) RunHealthCheck() {
	startTime := time.Now()
	hc.healthLogger.LogHealthCheckStart("定期健康检查")

	// 创建存储所有服务结果的映射
	servicesMap := make(map[string]interface{})

	// 检查WHOIS服务
	if hc.WhoisManager != nil {
		// 执行详细的健康检查
		results := hc.WhoisManager.TestProvidersHealth()

		// 统计结果
		totalProviders := len(results)
		availableProviders := 0
		successfulTests := 0

		for _, result := range results {
			resultMap, ok := result.(map[string]interface{})
			if !ok {
				continue
			}

			// 检查provider是否可用，并同时设置testSuccessful字段
			isAvailable := false
			if available, ok := resultMap["available"].(bool); ok && available {
				isAvailable = true
				availableProviders++
			}

			// 统一设置testSuccessful字段
			resultMap["testSuccessful"] = isAvailable

			// 统计成功测试数量
			if isAvailable {
				successfulTests++
			}
		}

		// 构建WHOIS服务状态
		var whoisStatus string
		switch {
		case successfulTests == 0:
			whoisStatus = "down"
		case availableProviders == 0:
			whoisStatus = "down"
		case availableProviders < totalProviders:
			whoisStatus = "degraded"
		default:
			whoisStatus = "up"
		}

		// 记录健康检查结果
		hc.healthLogger.LogServiceStatus("WHOIS", totalProviders, availableProviders, whoisStatus)
		hc.healthLogger.Printf("WHOIS健康检查完成: 提供商总数=%d, 可用提供商=%d, 测试成功=%d",
			totalProviders, availableProviders, successfulTests)

		// 添加WHOIS服务到服务映射
		servicesMap["whois"] = map[string]interface{}{
			"status":         whoisStatus,
			"total":          totalProviders,
			"available":      availableProviders,
			"testSuccessful": successfulTests,
			"providers":      results,
		}

		// 添加whois_providers状态映射 (保持向后兼容)
		whoisProviders := make(map[string]bool)
		for providerName, result := range results {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if available, exists := resultMap["available"].(bool); exists {
					whoisProviders[providerName] = available
				}
			}
		}

		// 构建适当的数据结构包装结果
		wrappedResults := make(map[string]interface{})
		wrappedResults["providers"] = results
		wrappedResults["whois_providers"] = whoisProviders
		wrappedResults["timestamp"] = time.Now().UTC().Format(time.RFC3339)

		// 保存WHOIS结果
		hc.mutex.Lock()
		// 将WHOIS结果合并到总结果中
		for k, v := range wrappedResults {
			hc.lastCheckResults[k] = v
		}
		hc.mutex.Unlock()
	}

	// 检查DNS服务
	if hc.DNSChecker != nil {
		dnsResults := hc.DNSChecker.CheckAllServers()

		// 统计DNS服务器
		totalServers := len(dnsResults)
		availableServers := 0

		for _, result := range dnsResults {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if status, ok := resultMap["status"].(string); ok && status == "up" {
					availableServers++
				}
			}
		}

		// 计算DNS状态
		var dnsStatus string
		if totalServers == 0 {
			dnsStatus = "unknown"
		} else if availableServers == 0 {
			dnsStatus = "down"
		} else if availableServers < totalServers {
			dnsStatus = "degraded"
		} else {
			dnsStatus = "up"
		}

		// 添加DNS服务状态
		servicesMap["dns"] = map[string]interface{}{
			"status":    dnsStatus,
			"total":     totalServers,
			"available": availableServers,
			"servers":   dnsResults,
		}

		hc.healthLogger.LogServiceStatus("DNS", totalServers, availableServers, dnsStatus)
		hc.healthLogger.Printf("DNS健康检查完成: 总服务器=%d, 可用服务器=%d, 状态=%s",
			totalServers, availableServers, dnsStatus)
	}

	// 检查截图服务
	if hc.ScreenshotChecker != nil {
		screenshotServers := hc.ScreenshotChecker.CheckAllServers()

		// 统计截图服务状态
		totalServices := len(screenshotServers)
		availableServices := 0

		for _, server := range screenshotServers {
			if serverMap, ok := server.(map[string]interface{}); ok {
				if status, ok := serverMap["status"].(string); ok && status == "up" {
					availableServices++
				}
			}
		}

		// 计算截图服务总体状态
		var screenshotStatus string
		if totalServices == 0 {
			screenshotStatus = "unknown"
		} else if availableServices == 0 {
			screenshotStatus = "down"
		} else if availableServices < totalServices {
			screenshotStatus = "degraded"
		} else {
			screenshotStatus = "up"
		}

		// 添加截图服务状态
		servicesMap["screenshot"] = map[string]interface{}{
			"status":    screenshotStatus,
			"total":     totalServices,
			"available": availableServices,
			"servers":   screenshotServers,
		}

		hc.healthLogger.LogServiceStatus("截图", totalServices, availableServices, screenshotStatus)
		hc.healthLogger.Printf("截图服务健康检查完成: 服务数=%d, 可用服务=%d, 状态=%s",
			totalServices, availableServices, screenshotStatus)
	}

	// 检查ITDog服务
	if hc.ITDogChecker != nil {
		itdogServers := hc.ITDogChecker.CheckAllServers()

		// 统计ITDog服务状态
		totalServices := len(itdogServers)
		availableServices := 0

		for _, server := range itdogServers {
			if serverMap, ok := server.(map[string]interface{}); ok {
				if status, ok := serverMap["status"].(string); ok && status == "up" {
					availableServices++
				}
			}
		}

		// 计算ITDog服务总体状态
		var itdogStatus string
		if totalServices == 0 {
			itdogStatus = "unknown"
		} else if availableServices == 0 {
			itdogStatus = "down"
		} else if availableServices < totalServices {
			itdogStatus = "degraded"
		} else {
			itdogStatus = "up"
		}

		// 添加ITDog服务状态
		servicesMap["itdog"] = map[string]interface{}{
			"status":    itdogStatus,
			"total":     totalServices,
			"available": availableServices,
			"servers":   itdogServers,
		}

		hc.healthLogger.LogServiceStatus("ITDog", totalServices, availableServices, itdogStatus)
		hc.healthLogger.Printf("ITDog服务健康检查完成: 服务数=%d, 可用服务=%d, 状态=%s",
			totalServices, availableServices, itdogStatus)
	}

	// 更新最后检查时间和结果
	hc.mutex.Lock()

	// 生成健康检查总结
	hc.generateHealthCheckSummary(servicesMap)

	// 将服务映射添加到结果中
	hc.lastCheckResults["services"] = servicesMap

	// 记录服务映射内容
	servicesJson, _ := json.Marshal(servicesMap)
	hc.healthLogger.Printf("健康检查缓存服务映射内容: %s", string(servicesJson))

	hc.lastCheckTime = time.Now()
	hc.mutex.Unlock()

	hc.healthLogger.LogHealthCheckEnd("全部服务健康检查", time.Since(startTime))
}

// generateHealthCheckSummary 生成健康检查总结
func (hc *HealthChecker) generateHealthCheckSummary(servicesMap map[string]interface{}) {
	hc.healthLogger.Printf("")
	hc.healthLogger.Printf("=== 健康检查总结 ===")
	
	totalServices := 0
	availableServices := 0
	allServicesUp := true
	
	// 统计各服务状态
	for serviceName, serviceData := range servicesMap {
		if serviceMap, ok := serviceData.(map[string]interface{}); ok {
			if total, ok := serviceMap["total"].(int); ok {
				totalServices += total
			}
			if available, ok := serviceMap["available"].(int); ok {
				availableServices += available
			}
			if status, ok := serviceMap["status"].(string); ok {
				if status != "up" {
					allServicesUp = false
				}
				hc.healthLogger.Printf("  %s服务: %s (可用: %v/%v)", 
					serviceName, status, serviceMap["available"], serviceMap["total"])
			}
		}
	}
	
	// 计算总体健康状态
	overallStatus := "up"
	if !allServicesUp {
		if availableServices == 0 {
			overallStatus = "down"
		} else {
			overallStatus = "degraded"
		}
	}
	
	// 计算可用率
	availabilityRate := float64(0)
	if totalServices > 0 {
		availabilityRate = float64(availableServices) / float64(totalServices) * 100
	}
	
	hc.healthLogger.Printf("")
	hc.healthLogger.Printf("总体状态: %s", overallStatus)
	hc.healthLogger.Printf("服务总数: %d", totalServices)
	hc.healthLogger.Printf("可用服务: %d", availableServices)
	hc.healthLogger.Printf("可用率: %.1f%%", availabilityRate)
	hc.healthLogger.Printf("检查时间: %s", time.Now().Format("2006-01-02 15:04:05"))
	hc.healthLogger.Printf("=== 健康检查总结结束 ===")
	hc.healthLogger.Printf("")
}

// GetLastCheckTime 获取最后一次检查的时间
func (hc *HealthChecker) GetLastCheckTime() time.Time {
	return hc.lastCheckTime
}

// GetLastCheckResults 获取最近一次的检查结果
func (hc *HealthChecker) GetLastCheckResults() map[string]interface{} {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	// 不再自动执行检查，只返回当前缓存的结果
	return hc.lastCheckResults
}

// ShouldRunCheck 检查是否应该运行检查
func (hc *HealthChecker) ShouldRunCheck() bool {
	// 如果从未检查过，则应该检查
	if hc.lastCheckTime.IsZero() {
		return true
	}

	// 如果已经超过检查间隔，则应该检查
	nextCheckTime := hc.lastCheckTime.Add(time.Duration(hc.CheckIntervalDays) * 24 * time.Hour)
	return time.Now().After(nextCheckTime)
}

// UpdateCheckResults 更新健康检查结果
func (hc *HealthChecker) UpdateCheckResults(results map[string]interface{}) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	// 更新结果
	hc.lastCheckResults = results
	// 更新检查时间
	hc.lastCheckTime = time.Now()

	hc.healthLogger.Printf("健康检查结果已更新，时间: %s", hc.lastCheckTime.Format(time.RFC3339))
}

// GetHealthStatus 获取健康状态 - 用于健康检查API处理程序
func (hc *HealthChecker) GetHealthStatus() map[string]interface{} {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	// 如果缓存的结果中有services字段，直接返回
	if services, exists := hc.lastCheckResults["services"]; exists {
		if servicesMap, ok := services.(map[string]interface{}); ok {
			hc.healthLogger.Printf("返回缓存的健康检查服务状态，包含 %d 个服务", len(servicesMap))
			return servicesMap
		}
	}

	// 如果没有services字段，返回整个lastCheckResults作为向后兼容
	hc.healthLogger.Printf("返回完整的健康检查结果作为服务状态")
	return hc.lastCheckResults
}
