/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-19 11:00:00
 * @Description: WHOIS提供商比较处理程序 - 展示不同WHOIS方法的查询结果
 */
package handlers

import (
	"context"
	"whosee/providers"
	"whosee/types"
	"whosee/utils"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// WhoisComparisonResponse 比较查询响应结构
type WhoisComparisonResponse struct {
	Domain    string                          `json:"domain"`
	Results   map[string]*WhoisProviderResult `json:"results"`
	Summary   *ComparisonSummary              `json:"summary"`
	Timestamp string                          `json:"timestamp"`
}

// WhoisProviderResult 单个提供商的查询结果
type WhoisProviderResult struct {
	Provider     string               `json:"provider"`
	Success      bool                 `json:"success"`
	Data         *types.WhoisResponse `json:"data,omitempty"`
	Error        string               `json:"error,omitempty"`
	ResponseTime int64                `json:"responseTimeMs"`
	Cached       bool                 `json:"cached,omitempty"`
}

// ComparisonSummary 比较结果摘要
type ComparisonSummary struct {
	TotalProviders    int    `json:"totalProviders"`
	SuccessfulQueries int    `json:"successfulQueries"`
	FailedQueries     int    `json:"failedQueries"`
	FastestProvider   string `json:"fastestProvider,omitempty"`
	FastestTime       int64  `json:"fastestTimeMs,omitempty"`
	SlowestProvider   string `json:"slowestProvider,omitempty"`
	SlowestTime       int64  `json:"slowestTimeMs,omitempty"`
	RecommendedMethod string `json:"recommendedMethod,omitempty"`
	MethodDescription string `json:"methodDescription,omitempty"`
}

// WhoisComparisonHandler 处理WHOIS提供商比较请求
func WhoisComparisonHandler(c *gin.Context) {
	startTime := time.Now()

	// 从上下文获取域名
	domain, exists := c.Get("domain")
	if !exists {
		log.Printf("WhoisComparison: 域名未在上下文中找到")
		utils.ErrorResponse(c, 400, "MISSING_DOMAIN", "Domain not found in context")
		return
	}

	domainStr := domain.(string)
	log.Printf("开始WHOIS提供商比较查询: %s", domainStr)

	// 创建所有提供商实例
	providerInstances := map[string]types.WhoisProvider{
		"IANA-RDAP":   providers.NewIANARDAPProvider(),
		"IANA-WHOIS":  providers.NewIANAWhoisProvider(),
		"WhoisFreaks": providers.NewWhoisFreaksProvider(),
		"WhoisXML":    providers.NewWhoisXMLProvider(),
	}

	// 并发查询所有提供商
	results := make(map[string]*WhoisProviderResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 使用带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for providerName, provider := range providerInstances {
		wg.Add(1)
		go func(name string, p types.WhoisProvider) {
			defer wg.Done()
			queryProvider(ctx, name, p, domainStr, &mu, results)
		}(providerName, provider)
	}

	// 等待所有查询完成
	wg.Wait()

	// 生成比较摘要
	summary := generateComparisonSummary(results)

	// 构建响应
	response := &WhoisComparisonResponse{
		Domain:    domainStr,
		Results:   results,
		Summary:   summary,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	processingTime := time.Since(startTime).Milliseconds()
	log.Printf("WHOIS提供商比较完成: %s, 处理时间: %dms, 成功: %d/%d",
		domainStr, processingTime, summary.SuccessfulQueries, summary.TotalProviders)

	utils.SuccessResponse(c, response, &utils.MetaInfo{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Processing: processingTime,
		Version:    "1.0",
	})
}

// queryProvider 查询单个提供商
func queryProvider(ctx context.Context, name string, provider types.WhoisProvider, domain string, mu *sync.Mutex, results map[string]*WhoisProviderResult) {
	startTime := time.Now()
	result := &WhoisProviderResult{
		Provider: name,
	}

	// 在超时上下文中执行查询
	done := make(chan struct{})
	var whoisResp *types.WhoisResponse
	var err error
	var cached bool

	go func() {
		defer close(done)
		whoisResp, err, cached = provider.Query(domain)
	}()

	select {
	case <-done:
		// 查询完成
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			log.Printf("提供商 %s 查询失败: %v", name, err)
		} else {
			result.Success = true
			result.Data = whoisResp
			result.Cached = cached
			log.Printf("提供商 %s 查询成功", name)
		}
	case <-ctx.Done():
		// 超时
		result.Success = false
		result.Error = "查询超时"
		log.Printf("提供商 %s 查询超时", name)
	}

	result.ResponseTime = time.Since(startTime).Milliseconds()

	// 线程安全地存储结果
	mu.Lock()
	results[name] = result
	mu.Unlock()
}

// generateComparisonSummary 生成比较摘要
func generateComparisonSummary(results map[string]*WhoisProviderResult) *ComparisonSummary {
	summary := &ComparisonSummary{
		TotalProviders: len(results),
	}

	var fastestTime int64 = -1
	var slowestTime int64 = -1

	for providerName, result := range results {
		if result.Success {
			summary.SuccessfulQueries++

			// 记录最快和最慢的提供商
			if fastestTime == -1 || result.ResponseTime < fastestTime {
				fastestTime = result.ResponseTime
				summary.FastestProvider = providerName
				summary.FastestTime = result.ResponseTime
			}

			if slowestTime == -1 || result.ResponseTime > slowestTime {
				slowestTime = result.ResponseTime
				summary.SlowestProvider = providerName
				summary.SlowestTime = result.ResponseTime
			}
		} else {
			summary.FailedQueries++
		}
	}

	// 推荐方法逻辑
	summary.RecommendedMethod, summary.MethodDescription = recommendBestMethod(results)

	return summary
}

// recommendBestMethod 根据查询结果推荐最佳方法
func recommendBestMethod(results map[string]*WhoisProviderResult) (string, string) {
	// 优先级：成功率 > 响应时间 > 协议现代性

	// 1. 检查IANA RDAP（最现代的协议）
	if rdapResult, exists := results["IANA-RDAP"]; exists && rdapResult.Success {
		return "IANA-RDAP", "推荐使用IANA RDAP协议，这是最现代的WHOIS查询方法，提供结构化JSON数据，更适合程序处理"
	}

	// 2. 检查IANA WHOIS（官方权威）
	if ianaResult, exists := results["IANA-WHOIS"]; exists && ianaResult.Success {
		return "IANA-WHOIS", "推荐使用IANA官方WHOIS协议，基于TCP端口43的传统查询，权威性高"
	}

	// 3. 找到最快的成功提供商
	var fastestProvider string
	var fastestTime int64 = -1

	for name, result := range results {
		if result.Success && (fastestTime == -1 || result.ResponseTime < fastestTime) {
			fastestTime = result.ResponseTime
			fastestProvider = name
		}
	}

	if fastestProvider != "" {
		descriptions := map[string]string{
			"WhoisFreaks": "推荐使用WhoisFreaks API，响应速度快，提供商业级服务质量",
			"WhoisXML":    "推荐使用WhoisXML API，提供详细的WHOIS信息和良好的数据结构",
		}

		if desc, exists := descriptions[fastestProvider]; exists {
			return fastestProvider, desc
		}
		return fastestProvider, fmt.Sprintf("推荐使用%s，在当前测试中表现最佳", fastestProvider)
	}

	return "无", "所有提供商查询都失败，建议检查网络连接或域名有效性"
}

// WhoisProvidersInfoHandler 提供商信息处理程序
func WhoisProvidersInfoHandler(c *gin.Context) {
	info := map[string]interface{}{
		"providers": map[string]interface{}{
			"IANA-RDAP": map[string]interface{}{
				"name":        "IANA RDAP",
				"description": "基于RDAP协议的现代化WHOIS查询，提供结构化JSON数据",
				"type":        "官方协议",
				"endpoint":    "https://rdap.iana.org/rdap/",
				"features":    []string{"结构化数据", "JSON格式", "现代协议", "标准化响应"},
				"pros":        []string{"数据结构化", "解析简单", "ICANN推荐", "未来标准"},
				"cons":        []string{"部分TLD支持有限", "相对较新"},
			},
			"IANA-WHOIS": map[string]interface{}{
				"name":        "IANA WHOIS",
				"description": "基于TCP端口43的传统WHOIS查询，权威性高",
				"type":        "官方协议",
				"endpoint":    "whois.iana.org:43",
				"features":    []string{"官方权威", "层级查询", "全球支持", "历史悠久"},
				"pros":        []string{"权威性高", "覆盖全面", "免费使用", "标准协议"},
				"cons":        []string{"数据格式不统一", "解析复杂", "需要多级查询"},
			},
			"WhoisFreaks": map[string]interface{}{
				"name":        "WhoisFreaks API",
				"description": "商业WHOIS API服务，提供高质量数据",
				"type":        "商业API",
				"endpoint":    "https://api.whoisfreaks.com/",
				"features":    []string{"高可用性", "快速响应", "统一格式", "商业支持"},
				"pros":        []string{"响应快速", "数据质量高", "格式统一", "技术支持"},
				"cons":        []string{"需要API密钥", "有请求限制", "商业服务"},
			},
			"WhoisXML": map[string]interface{}{
				"name":        "WhoisXML API",
				"description": "企业级WHOIS API服务，提供详细信息",
				"type":        "商业API",
				"endpoint":    "https://www.whoisxmlapi.com/",
				"features":    []string{"企业级", "详细数据", "历史记录", "全球覆盖"},
				"pros":        []string{"数据详细", "历史记录", "企业级", "全球支持"},
				"cons":        []string{"需要API密钥", "商业服务", "成本较高"},
			},
		},
		"comparison": map[string]interface{}{
			"free_options":        []string{"IANA-RDAP", "IANA-WHOIS"},
			"commercial_options":  []string{"WhoisFreaks", "WhoisXML"},
			"fastest_typical":     []string{"WhoisFreaks", "WhoisXML"},
			"most_authoritative":  []string{"IANA-RDAP", "IANA-WHOIS"},
			"best_for_dev":        []string{"IANA-RDAP", "WhoisFreaks"},
			"best_for_enterprise": []string{"WhoisXML", "WhoisFreaks"},
		},
		"recommendations": map[string]interface{}{
			"development": "对于开发环境，推荐使用IANA RDAP协议，免费且数据结构化",
			"production":  "对于生产环境，推荐使用商业API（WhoisFreaks或WhoisXML）以获得更好的可靠性",
			"learning":    "对于学习WHOIS协议，推荐从IANA WHOIS开始，了解传统查询方式",
			"future":      "RDAP是未来趋势，建议逐步迁移到RDAP协议",
		},
	}

	utils.SuccessResponse(c, info, &utils.MetaInfo{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   "1.0",
	})
}
