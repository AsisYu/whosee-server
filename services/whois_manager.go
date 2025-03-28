package services

import (
	"context"
	"dmainwhoseek/types"
	"encoding/json"
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
	providers []types.WhoisProvider
	rdb       *redis.Client
	mu        sync.RWMutex
	status    map[string]*providerStatus
}

func NewWhoisManager(rdb *redis.Client) *WhoisManager {
	return &WhoisManager{
		providers: make([]types.WhoisProvider, 0),
		rdb:       rdb,
		status:    make(map[string]*providerStatus),
	}
}

func (m *WhoisManager) AddProvider(provider types.WhoisProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = append(m.providers, provider)
	m.status[provider.Name()] = &providerStatus{
		isAvailable: true,
		lastUsed:    time.Now(),
	}
}

func (m *WhoisManager) selectProvider() types.WhoisProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var selected types.WhoisProvider
	var minScore float64 = -1

	now := time.Now()
	for _, p := range m.providers {
		status := m.status[p.Name()]
		if !status.isAvailable {
			// 如果提供者不可用，检查是否可以重新启用
			if now.Sub(status.lastUsed) > 5*time.Minute {
				status.isAvailable = true
				status.errorCount = 0
			} else {
				continue
			}
		}

		// 计算提供者得分（越低越好）
		score := float64(status.count)
		score += float64(status.errorCount) * 10              // 错误次数权重更大
		score += now.Sub(status.lastUsed).Minutes()           // 考虑时间间隔
		score -= float64(len(m.providers)-status.count) * 0.5 // 平衡使用次数

		if minScore == -1 || score < minScore {
			minScore = score
			selected = p
		}
	}

	return selected
}

func (m *WhoisManager) Query(domain string) (*types.WhoisResponse, error, bool) {
	// 先检查缓存
	cacheKey := CACHE_PREFIX + domain
	response, isCached := m.checkCache(cacheKey)
	if isCached {
		log.Printf("命中缓存: %s", domain)
		return response, nil, true
	}

	// 选择提供者并查询
	provider := m.selectProvider()
	if provider == nil {
		// 返回空响应而不是错误
		log.Printf("没有可用的提供商，返回空响应")
		return &types.WhoisResponse{
			Available: true,
			Domain:    domain,
		}, nil, false
	}

	status := m.status[provider.Name()]
	response, err, _ := provider.Query(domain)

	m.mu.Lock()
	defer m.mu.Unlock()

	status.lastUsed = time.Now()
	status.count++

	if err != nil {
		status.errorCount++
		if status.errorCount >= MAX_RETRIES {
			status.isAvailable = false
			log.Printf("Provider %s 暂时禁用", provider.Name())
		}
		// 尝试其他提供者
		var allFailed = true
		for _, p := range m.providers {
			if p.Name() != provider.Name() && m.status[p.Name()].isAvailable {
				response, err, _ = p.Query(domain)
				if err == nil {
					m.status[p.Name()].count++
					m.status[p.Name()].lastUsed = time.Now()
					allFailed = false
					break
				}
			}
		}
		
		// 如果所有提供商都失败，返回空响应而不是错误
		if allFailed {
			log.Printf("所有提供商查询失败，返回空响应，域名: %s", domain)
			return &types.WhoisResponse{
				Available: true,
				Domain:    domain,
			}, nil, false
		}
	} else {
		status.errorCount = 0
	}

	if err == nil && response != nil {
		m.cacheResponse(cacheKey, response)
	}

	return response, nil, false
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
		// 添加随机时间防止缓存雪崩
		ttl := CACHE_TTL + time.Duration(rand.Int63n(int64(24*time.Hour)))
		if err := m.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
			log.Printf("缓存数据失败: %v", err)
		}
	}
}
