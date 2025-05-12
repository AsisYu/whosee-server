/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-29 12:15:00
 * @Description: WHOIS查询服务
 */
package services

import (
	"context"
	"dmainwhoseek/types"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// 查询状态码常量
const (
	// 成功状态
	StatusSuccess          = 200 // 成功
	StatusSuccessFromCache = 201 // 从缓存成功获取

	// 错误状态
	StatusBadRequest    = 400 // 无效请求
	StatusNotFound      = 404 // 未找到域名
	StatusTimeout       = 408 // 查询超时
	StatusRateLimited   = 429 // 超出API请求限制
	StatusServerError   = 500 // 服务器内部错误
	StatusProviderError = 503 // 所有提供商都失败
	StatusInvalidDomain = 422 // 无效域名格式
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
	// 创建一个空的WhoisResponse用于错误情况下返回
	emptyResponse := &types.WhoisResponse{
		Domain:        domain,
		StatusMessage: "查询失败",
	}

	// 从缓存中检查
	log.Printf("开始检查缓存: %s", domain)
	cacheKey := CACHE_PREFIX + domain
	cachedResponse, found := m.checkCache(cacheKey)
	if found {
		log.Printf("命中缓存: %s", domain)
		return cachedResponse, nil, true
	}

	// 获取可用提供商
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
		emptyResponse.StatusCode = StatusProviderError
		emptyResponse.StatusMessage = "没有可用的WHOIS提供商"
		return emptyResponse, fmt.Errorf("没有可用的WHOIS提供商"), false
	}

	log.Printf("可用提供商列表: %v", getProviderNames(availableProviders))

	// 创建上下文，用于控制所有查询的总超时时间
	// 检查是否是已知的慢域名，如果是，增加总超时时间
	var totalTimeout time.Duration = 15 * time.Second
	slowDomains := []string{"byd.com", "outlook.com", "microsoft.com", "alibaba.com", "tencent.com"}
	for _, slowDomain := range slowDomains {
		if strings.Contains(domain, slowDomain) {
			totalTimeout = 30 * time.Second
			log.Printf("检测到已知的慢域名: %s，增加总超时时间至 %v", domain, totalTimeout)
			break
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	// 创建通道，用于接收提供商查询结果
	type queryResult struct {
		response *types.WhoisResponse
		provider WhoisProvider
		err      error
		cached   bool
	}

	resultChan := make(chan queryResult, len(availableProviders))

	// 选择最优提供商
	selectedProvider := m.selectProvider()

	// 检测到慢域名时同时启动所有查询
	isSlowDomain := totalTimeout > 15*time.Second

	// 并行查询所有可用提供商
	for _, provider := range availableProviders {
		// 为每个提供商启动一个goroutine
		go func(p WhoisProvider) {
			// 主提供商超时设置长一些
			providerTimeout := 10 * time.Second
			if p.Name() == selectedProvider.Name() {
				log.Printf("优先使用提供商 %s 查询域名: %s", p.Name(), domain)
				if isSlowDomain {
					providerTimeout = 20 * time.Second
					log.Printf("已知慢域名 %s 使用更长超时时间: %v", domain, providerTimeout)
				}
			} else {
				log.Printf("同时使用备用提供商 %s 查询域名: %s", p.Name(), domain)
			}

			// 为每个提供商设置单独的超时
			response, err, fromCache := m.queryWithTimeout(p, domain, providerTimeout)

			// 报告结果，除非上下文已取消
			select {
			case <-ctx.Done():
				// 上下文已取消，不发送结果
			case resultChan <- queryResult{response, p, err, fromCache}:
				// 结果已发送
			}
		}(provider)
	}

	// 收集结果
	var lastError error
	firstResult := make(chan struct{}) // 用于通知已经收到至少一个结果
	go func() {
		for i := 0; i < len(availableProviders); i++ {
			select {
			case <-ctx.Done():
				return
			case <-firstResult:
				return
			case <-time.After(func() time.Duration {
				if isSlowDomain {
					return 5 * time.Second
				}
				return 2 * time.Second
			}()):
				log.Printf("等待WHOIS查询结果中...已等待 %d 秒", func() int {
					if isSlowDomain {
						return i * 5
					}
					return i * 2
				}())
			}
		}
	}()

	// 等待结果
	timeoutTimer := time.NewTimer(func() time.Duration {
		if isSlowDomain {
			return 25 * time.Second // 慢域名使用更长的总超时
		}
		return 12 * time.Second // 正常总超时
	}()) // 总体超时比context略短，便于记录日志
	defer timeoutTimer.Stop()

	// 跟踪已完成的查询
	completedQueries := 0
	doneProviders := make(map[string]bool)

	// 收集结果
	for {
		select {
		case <-timeoutTimer.C:
			log.Printf("查询WHOIS超时，已完成 %d/%d 个提供商查询", completedQueries, len(availableProviders))

			// 设置超时状态码
			emptyResponse.StatusCode = StatusTimeout
			emptyResponse.StatusMessage = "查询超时，所有提供商均未返回结果"

			return emptyResponse, fmt.Errorf("查询超时: 所有提供商均未返回结果"), false

		case result := <-resultChan:
			completedQueries++

			// 如果我们已经在处理此提供商的结果，则跳过
			if doneProviders[result.provider.Name()] {
				continue
			}
			doneProviders[result.provider.Name()] = true

			// 通知已经收到至少一个结果
			select {
			case firstResult <- struct{}{}:
			default:
			}

			m.mu.Lock()
			status := m.status[result.provider.Name()]
			status.lastUsed = time.Now()
			status.count++

			if result.err != nil {
				// 提供商查询失败
				status.errorCount++
				if status.errorCount >= MAX_RETRIES {
					status.isAvailable = false
					log.Printf("提供商 %s 暂时禁用", result.provider.Name())
				}
				m.mu.Unlock()

				// 记录错误并继续尝试其他提供商
				lastError = result.err
				log.Printf("提供商 %s 查询失败: %v", result.provider.Name(), result.err)

				// 如果所有提供商都完成了，返回最后一个错误
				if completedQueries >= len(availableProviders) {
					// 根据错误类型设置状态码
					emptyResponse.StatusCode = StatusProviderError

					if lastError != nil {
						if strings.Contains(lastError.Error(), "查询超时") {
							emptyResponse.StatusCode = StatusTimeout
							emptyResponse.StatusMessage = "查询超时: " + lastError.Error()
						} else if strings.Contains(lastError.Error(), "速率限制") || strings.Contains(lastError.Error(), "rate limit") {
							emptyResponse.StatusCode = StatusRateLimited
							emptyResponse.StatusMessage = "API请求频率超限"
						} else if strings.Contains(lastError.Error(), "无效域名") {
							emptyResponse.StatusCode = StatusInvalidDomain
							emptyResponse.StatusMessage = "无效域名格式"
						} else {
							emptyResponse.StatusMessage = "查询失败: " + lastError.Error()
						}
					}

					log.Printf("所有提供商均查询失败，返回最后一个错误")
					return emptyResponse, lastError, false
				}

				// 继续等待其他提供商
				continue
			}

			// 提供商查询成功
			status.errorCount = 0
			m.mu.Unlock()

			// 确保结果包含状态码和提供商信息
			if result.response != nil {
				if result.response.StatusCode == 0 {
					result.response.StatusCode = StatusSuccess
				}
				if result.response.StatusMessage == "" {
					result.response.StatusMessage = "查询成功"
				}
				if result.response.SourceProvider == "" {
					result.response.SourceProvider = result.provider.Name()
				}
			}

			// 缓存并返回结果
			m.cacheResponse(cacheKey, result.response)
			log.Printf("提供商 %s 查询成功，已缓存结果", result.provider.Name())
			return result.response, nil, false
		}
	}
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

	// 设置缓存状态码和来源
	response.StatusCode = StatusSuccessFromCache
	response.StatusMessage = "从缓存获取成功"
	if response.SourceProvider == "" {
		response.SourceProvider = "Cache"
	}

	return &response, true
}

func getProviderNames(providers []WhoisProvider) []string {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}
	return names
}

func (m *WhoisManager) cacheResponse(key string, response *types.WhoisResponse) {
	ctx := context.Background()

	// 添加缓存时间
	response.CachedAt = time.Now().Format("2006-01-02 15:04:05")

	if data, err := json.Marshal(response); err == nil {
		ttl := CACHE_TTL + time.Duration(rand.Int63n(int64(24*time.Hour)))
		if err := m.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
			log.Printf("缓存结果失败: %v", err)
		} else {
			log.Printf("成功缓存WHOIS数据，键: %s，缓存时间: %s, TTL: %v", key, response.CachedAt, ttl)
		}
	}
}

func (m *WhoisManager) queryWithTimeout(provider WhoisProvider, domain string, timeout time.Duration) (*types.WhoisResponse, error, bool) {
	// 设置超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 创建通道，用于接收提供商查询结果
	resultChan := make(chan struct {
		resp   *types.WhoisResponse
		err    error
		cached bool
	}, 1)

	// 在单独的goroutine中执行提供商查询
	go func() {
		resp, err, cached := provider.Query(domain)

		// 确保结果包含状态码和提供商信息
		if resp != nil && err == nil {
			if resp.StatusCode == 0 {
				resp.StatusCode = StatusSuccess
			}
			if resp.StatusMessage == "" {
				resp.StatusMessage = "查询成功"
			}
			if resp.SourceProvider == "" {
				resp.SourceProvider = provider.Name()
			}
		}

		resultChan <- struct {
			resp   *types.WhoisResponse
			err    error
			cached bool
		}{resp, err, cached}
	}()

	// 等待结果或超时
	select {
	case <-ctx.Done():
		// 超时，返回超时错误
		return &types.WhoisResponse{
			Domain:         domain,
			StatusCode:     StatusTimeout,
			StatusMessage:  "查询超时",
			SourceProvider: provider.Name(),
		}, fmt.Errorf("查询超时: %s 超时 %v", provider.Name(), timeout), false
	case result := <-resultChan:
		return result.resp, result.err, result.cached
	}
}

func (m *WhoisManager) TestProvidersHealth() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make(map[string]interface{})
	testDomains := []string{"google.com", "microsoft.com", "github.com"} // 使用测试域名

	log.Printf("开始测试WHOIS提供商可用性")

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
			"testSuccessful": false,             // 默认为false，测试成功后更新
			"statusCode":     StatusServerError, // 默认为服务器内部错误，测试成功后更新
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
			"statusCode":   StatusServerError,
		}

		queryResp, queryErr, _ := m.queryWithTimeout(provider, testDomain, queryTimeout)

		responseTime := time.Since(startTime)
		testResult["responseTime"] = responseTime.Milliseconds()
		providerResult["responseTime"] = responseTime.Milliseconds()

		if queryErr != nil {
			testResult["message"] = queryErr.Error()
			testResult["statusCode"] = StatusServerError

			if strings.Contains(queryErr.Error(), "超时") || strings.Contains(queryErr.Error(), "timeout") {
				testResult["statusCode"] = StatusTimeout
			}

			status.errorCount++
			if status.errorCount >= MAX_RETRIES {
				status.isAvailable = false
				log.Printf("由于测试失败，暂时禁用提供商 %s: %v", providerName, queryErr)
			}

		} else if queryResp == nil {
			testResult["message"] = "空响应"
			testResult["statusCode"] = StatusServerError

			status.errorCount++
			if status.errorCount >= MAX_RETRIES {
				status.isAvailable = false
				log.Printf("由于测试失败，暂时禁用提供商 %s", providerName)
			}
		} else {
			testResult["success"] = true
			testResult["message"] = "测试成功"
			testResult["statusCode"] = queryResp.StatusCode

			testResult["resultSummary"] = map[string]interface{}{
				"registrar":      queryResp.Registrar,
				"creationDate":   queryResp.CreateDate,
				"expiryDate":     queryResp.ExpiryDate,
				"sourceProvider": queryResp.SourceProvider,
			}

			status.errorCount = 0
			status.isAvailable = true
			providerResult["testSuccessful"] = true
			providerResult["statusCode"] = queryResp.StatusCode
		}

		providerTestResults := providerResult["testResults"].([]map[string]interface{})
		providerTestResults = append(providerTestResults, testResult)
		providerResult["testResults"] = providerTestResults

		status.lastUsed = time.Now()
		status.count++

		results[providerName] = providerResult

		log.Printf("提供商 %s 测试结果: 响应时间 %v 毫秒，测试 %v，状态码 %v",
			providerName, responseTime, testResult["success"], testResult["statusCode"])
	}

	return results
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
			"lastTested":     status.lastUsed.UTC().Format(time.RFC3339), // 使用上次使用时间作为最后测试时间
			"testSuccessful": status.isAvailable,                         // 使用可用状态作为测试成功状态
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
