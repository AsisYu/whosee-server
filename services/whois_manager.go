package services

import (
	"context"
	"dmainwhoseek/types"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	CACHE_PREFIX = "whois:"
	CACHE_TTL    = 30 * 24 * time.Hour // 缓存一个月
	MAX_RETRIES  = 2                   // 每个提供者最大重试次数
)

type providerStatus struct {
	count       int       // 调用次数
	lastUsed    time.Time // 上次使用时间
	errorCount  int       // 连续错误次数
	isAvailable bool      // 是否可用
}

type WhoisManager struct {
	providers []WhoisProvider
	rdb       *redis.Client
	mu        sync.RWMutex
	status    map[string]*providerStatus
}

func NewWhoisManager(rdb *redis.Client) *WhoisManager {
	manager := &WhoisManager{
		providers: make([]WhoisProvider, 0),
		rdb:       rdb,
		status:    make(map[string]*providerStatus),
	}
	
	// 设置随机种子，确保每次启动程序时的随机性
	rand.Seed(time.Now().UnixNano())
	
	return manager
}

func (m *WhoisManager) AddProvider(provider WhoisProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 添加提供商
	m.providers = append(m.providers, provider)
	
	// 初始化时为每个提供商分配随机的起始状态
	// 这样可以确保不同的提供商在初始状态下有不同的优先级
	initialCountOffset := rand.Intn(2) // 随机初始使用次数 (0或1)
	
	// 随机的初始"上次使用时间"，分散在过去的10分钟内
	// 这样可以让不同的提供商在初始状态下有不同的时间偏移
	timeOffset := time.Duration(rand.Intn(600)) * time.Second // 0-600秒的随机偏移
	initialLastUsed := time.Now().Add(-timeOffset)
	
	m.status[provider.Name()] = &providerStatus{
		isAvailable: true,
		count:       initialCountOffset,
		lastUsed:    initialLastUsed,
	}
	
	log.Printf("添加WHOIS提供商: %s (初始使用次数=%d, 初始上次使用时间偏移=-%v)",
		provider.Name(), initialCountOffset, timeOffset)
}

func (m *WhoisManager) selectProvider() WhoisProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var selected WhoisProvider
	var minScore float64 = -1

	now := time.Now().UTC() // 使用UTC时间确保时区一致性
	log.Printf("开始选择WHOIS提供商. 当前可用提供商状态:")
	
	for _, p := range m.providers {
		status := m.status[p.Name()]
		log.Printf("  提供商: %s, 可用: %v, 使用次数: %d, 错误次数: %d, 上次使用: %v (距今%v)", 
			p.Name(), status.isAvailable, status.count, status.errorCount, 
			status.lastUsed.Format("2006-01-02 15:04:05"),
			now.Sub(status.lastUsed).Round(time.Second))
			
		if !status.isAvailable {
			if now.Sub(status.lastUsed) > 5*time.Minute {
				status.isAvailable = true
				status.errorCount = 0
				log.Printf("  重新启用提供商: %s", p.Name())
			} else {
				log.Printf("  跳过不可用提供商: %s", p.Name())
				continue
			}
		}

		usageWeight := float64(status.count) * 10.0
		errorWeight := float64(status.errorCount) * 20.0
		lastUsedMinutes := now.Sub(status.lastUsed).Minutes()
		timeWeight := -lastUsedMinutes * 5.0 // 负值，增加时间权重
		
		score := usageWeight + errorWeight + timeWeight
		
		log.Printf("  提供商: %s 得分计算: 使用(%d*10)=%v + 错误(%d*20)=%v + 时间(-%v*5)=%v = 总分%v", 
			p.Name(), status.count, usageWeight, status.errorCount, errorWeight, 
			lastUsedMinutes, timeWeight, score)

		if minScore == -1 || score < minScore {
			minScore = score
			selected = p
			log.Printf("  当前最优选择更新为: %s, 得分: %v", p.Name(), minScore)
		}
	}

	if selected != nil {
		log.Printf("最终选择提供商: %s, 得分: %v", selected.Name(), minScore)
	} else {
		log.Printf("无可用提供商")
	}
	
	return selected
}

func (m *WhoisManager) Query(domain string) (*types.WhoisResponse, error, bool) {
	log.Printf("开始检查缓存: %s", domain)

	cacheKey := CACHE_PREFIX + domain
	cachedResponse, found := m.checkCache(cacheKey)
	if found {
		log.Printf("命中缓存: %s", domain)
		return cachedResponse, nil, true
	}

	m.mu.RLock()
	availableProviders := []WhoisProvider{}
	for _, p := range m.providers {
		status := m.status[p.Name()]
		if status.isAvailable {
			availableProviders = append(availableProviders, p)
		}
	}
	m.mu.RUnlock()

	if len(availableProviders) == 0 {
		return nil, fmt.Errorf("没有可用的WHOIS提供商"), false
	}

	log.Printf("可用提供商列表: %v", getProviderNames(availableProviders))

	selectedProvider := m.selectProvider()
	if selectedProvider != nil {
		log.Printf("尝试使用提供商 %s 查询域名: %s", selectedProvider.Name(), domain)
		
		response, err, _ := selectedProvider.Query(domain)
		
		m.mu.Lock()
		status := m.status[selectedProvider.Name()]
		status.lastUsed = time.Now()
		status.count++
		
		if err != nil {
			status.errorCount++
			if status.errorCount >= MAX_RETRIES {
				status.isAvailable = false
				log.Printf("Provider %s 暂时禁用", selectedProvider.Name())
			}
			log.Printf("提供商 %s 查询失败: %v，尝试备用提供商", selectedProvider.Name(), err)
		} else {
			status.errorCount = 0
			m.mu.Unlock()
			
			if response != nil {
				m.cacheResponse(cacheKey, response)
				log.Printf("提供商 %s 查询成功，缓存结果", selectedProvider.Name())
				log.Printf("WHOIS查询完成 - 域名: %s, 提供商: %s, 结果: %+v", 
					domain, selectedProvider.Name(), *response)
				return response, nil, false
			}
		}
	}
	
	for _, provider := range availableProviders {
		if selectedProvider != nil && provider.Name() == selectedProvider.Name() {
			continue
		}
		
		log.Printf("尝试使用备用提供商 %s 查询域名: %s", provider.Name(), domain)
		
		response, err, _ := provider.Query(domain)
		
		m.mu.Lock()
		status := m.status[provider.Name()]
		status.lastUsed = time.Now()
		status.count++
		
		if err != nil {
			status.errorCount++
			if status.errorCount >= MAX_RETRIES {
				status.isAvailable = false
				log.Printf("Provider %s 暂时禁用", provider.Name())
			}
			log.Printf("提供商 %s 查询失败: %v，尝试下一个提供商", provider.Name(), err)
			continue
		} else {
			status.errorCount = 0
			m.mu.Unlock()
			
			if response != nil {
				m.cacheResponse(cacheKey, response)
				log.Printf("提供商 %s 查询成功，缓存结果", provider.Name())
				log.Printf("WHOIS查询完成 - 域名: %s, 提供商: %s, 结果: %+v", 
					domain, provider.Name(), *response)
				return response, nil, false
			}
		}
	}
	
	log.Printf("所有提供商查询失败 - 域名: %s", domain)
	return &types.WhoisResponse{
		Available: true,
		Domain:    domain,
	}, nil, false
}

func (m *WhoisManager) checkCache(key string) (*types.WhoisResponse, bool) {
	ctx := context.Background()
	data, err := m.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, false
	}

	var response types.WhoisResponse
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		log.Printf("解析缓存数据失败: %v", err)
		return nil, false
	}

	return &response, true
}

func (m *WhoisManager) cacheResponse(key string, response *types.WhoisResponse) {
	ctx := context.Background()
	if data, err := json.Marshal(response); err == nil {
		ttl := CACHE_TTL + time.Duration(rand.Int63n(int64(24*time.Hour)))
		if err := m.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
			log.Printf("缓存数据失败: %v", err)
		}
	}
}

func (m *WhoisManager) TestProvidersHealth() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	results := make(map[string]interface{})
	testDomains := []string{"google.com", "microsoft.com", "github.com"} // 使用多个知名域名测试
	
	log.Printf("开始主动测试WHOIS服务商可用性")
	
	const queryTimeout = 10 * time.Second
	
	for _, provider := range m.providers {
		providerName := provider.Name()
		status := m.status[providerName]
		
		providerResult := map[string]interface{}{
			"available":      status.isAvailable,
			"errorCount":     status.errorCount,
			"lastUsed":       status.lastUsed.UTC().Format(time.RFC3339),
			"callCount":      status.count,
			"testResults":    make([]map[string]interface{}, 0),
			"responseTime":   0,
			"testSuccessful": false,  // 默认为false，测试成功后更新
		}
		
		testDomain := testDomains[rand.Intn(len(testDomains))]
		
		log.Printf("使用测试域名 %s 测试提供商 %s", testDomain, providerName)
		
		startTime := time.Now()
		
		testResult := map[string]interface{}{
			"domain":       testDomain,
			"timestamp":    startTime.UTC().Format(time.RFC3339),
			"success":      false,
			"message":      "",
			"responseTime": 0,
		}
		
		queryResp, queryErr, _ := m.queryWithTimeout(provider, testDomain, queryTimeout)
		
		responseTime := time.Since(startTime)
		testResult["responseTime"] = responseTime.Milliseconds()
		providerResult["responseTime"] = responseTime.Milliseconds()
		
		if queryErr != nil {
			testResult["message"] = queryErr.Error()
			
			status.errorCount++
			if status.errorCount >= MAX_RETRIES {
				status.isAvailable = false
				log.Printf("因测试失败，暂时禁用提供商 %s: %v", providerName, queryErr)
			}
		} else if queryResp == nil {
			testResult["message"] = "查询返回为空"
			
			status.errorCount++
			if status.errorCount >= MAX_RETRIES {
				status.isAvailable = false
				log.Printf("因返回空结果，暂时禁用提供商 %s", providerName)
			}
		} else {
			testResult["success"] = true
			testResult["message"] = "查询成功"
			
			testResult["resultSummary"] = map[string]interface{}{
				"registrar":    queryResp.Registrar,
				"creationDate": queryResp.CreateDate,
				"expiryDate":   queryResp.ExpiryDate,
			}
			
			status.errorCount = 0
			status.isAvailable = true
			providerResult["testSuccessful"] = true
		}
		
		providerTestResults := providerResult["testResults"].([]map[string]interface{})
		providerTestResults = append(providerTestResults, testResult)
		providerResult["testResults"] = providerTestResults
		
		status.lastUsed = time.Now()
		status.count++
		
		results[providerName] = providerResult
		
		log.Printf("提供商 %s 测试完成，响应时间: %v, 成功: %v", 
			providerName, responseTime, testResult["success"])
	}
	
	return results
}

func (m *WhoisManager) queryWithTimeout(provider WhoisProvider, domain string, timeout time.Duration) (*types.WhoisResponse, error, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	// 创建一个channel用于接收结果
	resultChan := make(chan struct {
		resp *types.WhoisResponse
		err  error
		fromCache bool
	}, 1)
	
	// 在goroutine中执行查询
	go func() {
		resp, err, fromCache := provider.Query(domain)
		resultChan <- struct {
			resp *types.WhoisResponse
			err  error
			fromCache bool
		}{resp, err, fromCache}
	}()
	
	// 等待结果或超时
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("查询超时 (>%v)", timeout), false
	case result := <-resultChan:
		return result.resp, result.err, result.fromCache
	}
}

func (m *WhoisManager) GetProvidersStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]interface{})
	
	for _, provider := range m.providers {
		name := provider.Name()
		status := m.status[name]
		
		result[name] = map[string]interface{}{
			"available":      status.isAvailable,
			"errorCount":     status.errorCount,
			"lastUsed":       status.lastUsed.UTC().Format(time.RFC3339),
			"callCount":      status.count,
			"lastTested":     status.lastUsed.UTC().Format(time.RFC3339), // 使用上次使用时间作为上次测试时间
			"testSuccessful": status.isAvailable, // 添加testSuccessful字段，与available状态一致
		}
	}
	
	return result
}

func (m *WhoisManager) GetOverallStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	availableCount := 0
	for _, provider := range m.providers {
		if m.status[provider.Name()].isAvailable {
			availableCount++
		}
	}
	
	switch {
	case availableCount == 0:
		return "down"
	case availableCount < len(m.providers):
		return "degraded"
	default:
		return "up"
	}
}

func getProviderNames(providers []WhoisProvider) []string {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}
	return names
}
