/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-29 12:15:00
 * @Description: WHOIS查询服务
 */
package services

import (
	"context"
	"whosee/types"
	"whosee/utils"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
)

// WhoisProvider 定义了 WHOIS 查询提供者的接口
type WhoisProvider interface {
	Query(domain string) (*types.WhoisResponse, error, bool)
	Name() string
}

type WhoisService struct {
	redis    *redis.Client
	provider WhoisProvider
}

func (s *WhoisService) GetWhoisInfo(domain string) (*types.WhoisResponse, error) {
	// 先从 Redis 缓存获取
	cacheKey := utils.BuildCacheKey("cache", "whois", utils.SanitizeDomain(domain))
	if cachedData, err := s.redis.Get(context.Background(), cacheKey).Result(); err == nil {
		var whoisInfo types.WhoisResponse
		if err := json.Unmarshal([]byte(cachedData), &whoisInfo); err == nil {
			return &whoisInfo, nil
		}
	}

	// 从 API 获取数据
	whoisInfo, err, _ := s.provider.Query(domain)
	if err != nil {
		return nil, err
	}

	// 计算缓存时间
	cacheDuration := s.calculateCacheDuration(whoisInfo)

	// 存入 Redis
	if jsonData, err := json.Marshal(whoisInfo); err == nil {
		s.redis.Set(context.Background(), cacheKey, jsonData, cacheDuration)
	}

	return whoisInfo, nil
}

func CacheWhoisResult(rdb *redis.Client, domain string, response interface{}) error {
	ctx := context.Background()
	cacheKey := utils.BuildCacheKey("cache", "whois", utils.SanitizeDomain(domain))
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, cacheKey, data, 24*time.Hour).Err()
}

// 根据域名到期时间计算缓存时间
func (s *WhoisService) calculateCacheDuration(info *types.WhoisResponse) time.Duration {
	if info == nil || info.ExpiryDate == "" {
		return 24 * time.Hour // 默认缓存24小时
	}

	// 解析过期时间
	expiryDate, err := time.Parse(time.RFC3339, info.ExpiryDate)
	if err != nil {
		return 24 * time.Hour // 如果解析失败，使用默认缓存时间
	}

	// 计算距离到期的时间
	daysUntilExpiry := time.Until(expiryDate).Hours() / 24

	switch {
	case daysUntilExpiry <= 15: // 15天内到期
		return 1 * time.Hour // 缓存1小时
	case daysUntilExpiry <= 30: // 30天内到期
		return 6 * time.Hour // 缓存6小时
	case daysUntilExpiry <= 90: // 90天内到期
		return 12 * time.Hour // 缓存12小时
	default:
		return 24 * time.Hour // 默认缓存24小时
	}
}
