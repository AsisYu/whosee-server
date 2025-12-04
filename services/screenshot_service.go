/*
 * @Author: AsisYu
 * @Date: 2025-01-20
 * @Description: 重构的截图服务 - 统一架构
 */
package services

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"whosee/utils"

	"github.com/chromedp/chromedp"
	"github.com/go-redis/redis/v8"
)

// ScreenshotType 截图类型枚举
type ScreenshotType string

const (
	TypeBasic        ScreenshotType = "basic"        // 基础截图
	TypeElement      ScreenshotType = "element"      // 元素截图
	TypeItdogMap     ScreenshotType = "itdog_map"    // ITDog地图
	TypeItdogTable   ScreenshotType = "itdog_table"  // ITDog表格
	TypeItdogIP      ScreenshotType = "itdog_ip"     // ITDog IP统计
	TypeItdogResolve ScreenshotType = "itdog_resolve" // ITDog综合测速
)

// OutputFormat 输出格式枚举
type OutputFormat string

const (
	FormatFile   OutputFormat = "file"   // 文件输出
	FormatBase64 OutputFormat = "base64" // Base64输出
)

// MaxUserCacheExpireHours 用户可设置的最大缓存TTL（防止DoS攻击）
// 限制用户通过cache_expire参数造成Redis内存耗尽
const MaxUserCacheExpireHours = 72 // 最多3天

// ScreenshotRequest 统一截图请求结构
type ScreenshotRequest struct {
	Type        ScreenshotType `json:"type"`                  // 截图类型
	Domain      string         `json:"domain"`                // 目标域名
	URL         string         `json:"url,omitempty"`         // 完整URL（优先级高于domain）
	Selector    string         `json:"selector,omitempty"`    // CSS/XPath选择器
	Format      OutputFormat   `json:"format"`                // 输出格式
	WaitTime    int            `json:"wait_time,omitempty"`   // 等待时间（秒）
	Timeout     int            `json:"timeout,omitempty"`     // 超时时间（秒）
	CacheExpire int            `json:"cache_expire,omitempty"` // 缓存过期时间（小时）
}

// ScreenshotResponse 统一截图响应结构
type ScreenshotResponse struct {
	Success     bool   `json:"success"`                // 是否成功
	ImageURL    string `json:"image_url,omitempty"`    // 图片URL/Base64
	ImageBase64 string `json:"image_base64,omitempty"` // Base64数据（仅Base64格式）
	FromCache   bool   `json:"from_cache,omitempty"`   // 是否来自缓存
	Error       string `json:"error,omitempty"`        // 错误码
	Message     string `json:"message,omitempty"`      // 错误描述
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // 元数据
}

// ScreenshotConfig 截图配置
type ScreenshotConfig struct {
	Type        ScreenshotType
	URL         string
	Selector    string
	WaitTime    time.Duration
	Timeout     time.Duration
	CacheKey    string
	FilePath    string
	FileURL     string
	Description string
}

// ScreenshotService 截图服务
type ScreenshotService struct {
	chromeManager *ChromeManager
	redisClient   *redis.Client
	config        *ScreenshotServiceConfig
}

// ScreenshotServiceConfig 服务配置
type ScreenshotServiceConfig struct {
	BaseDir          string        // 基础目录
	CacheExpiration  time.Duration // 默认缓存过期时间
	DefaultTimeout   time.Duration // 默认超时时间
	DefaultWaitTime  time.Duration // 默认等待时间
	MaxFileSize      int64         // 最大文件大小
	AllowedFormats   []string      // 允许的图片格式
}

// DefaultScreenshotServiceConfig 默认配置
var DefaultScreenshotServiceConfig = &ScreenshotServiceConfig{
	BaseDir:         "./static",
	CacheExpiration: 24 * time.Hour,
	DefaultTimeout:  60 * time.Second,
	DefaultWaitTime: 3 * time.Second,
	MaxFileSize:     10 * 1024 * 1024, // 10MB
	AllowedFormats:  []string{"png", "jpg", "jpeg", "webp"},
}

// NewScreenshotService 创建截图服务
func NewScreenshotService(chromeManager *ChromeManager, redisClient *redis.Client, config *ScreenshotServiceConfig) *ScreenshotService {
	if config == nil {
		config = DefaultScreenshotServiceConfig
	}

	service := &ScreenshotService{
		chromeManager: chromeManager,
		redisClient:   redisClient,
		config:        config,
	}

	// 创建必要的目录
	service.ensureDirectories()

	return service
}

// ensureDirectories 确保目录存在
func (s *ScreenshotService) ensureDirectories() {
	dirs := []string{
		filepath.Join(s.config.BaseDir, "screenshots"),
		filepath.Join(s.config.BaseDir, "itdog"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("[SCREENSHOT] 创建目录失败: %s, 错误: %v", dir, err)
		}
	}
}

// TakeScreenshot 统一截图接口
func (s *ScreenshotService) TakeScreenshot(ctx context.Context, req *ScreenshotRequest) (*ScreenshotResponse, error) {
	// 验证请求
	if err := s.validateRequest(req); err != nil {
		return &ScreenshotResponse{
			Success: false,
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
		}, nil
	}

	// 生成配置
	config, err := s.generateConfig(req)
	if err != nil {
		return &ScreenshotResponse{
			Success: false,
			Error:   "CONFIG_ERROR",
			Message: err.Error(),
		}, nil
	}

	// 检查缓存
	if cached := s.getFromCache(config.CacheKey); cached != nil {
		log.Printf("[SCREENSHOT] 使用缓存: %s", config.Description)
		cached.FromCache = true
		return cached, nil
	}

	// 检查熔断器
	if !s.chromeManager.AllowRequest() {
		return &ScreenshotResponse{
			Success: false,
			Error:   "SERVICE_UNAVAILABLE",
			Message: "截图服务暂时不可用，请稍后重试",
		}, nil
	}

	// 执行截图
	startTime := time.Now()
	response, err := s.executeScreenshot(ctx, config)
	duration := time.Since(startTime)

	// 记录日志
	if err != nil {
		log.Printf("[SCREENSHOT] %s失败: %v, 耗时: %v", config.Description, err, duration)
		return s.handleError(err, config)
	}

	// 缓存结果
	s.cacheResult(config.CacheKey, response, req.CacheExpire)

	log.Printf("[SCREENSHOT] %s成功, 耗时: %v", config.Description, duration)
	return response, nil
}

// validateRequest 验证请求
func (s *ScreenshotService) validateRequest(req *ScreenshotRequest) error {
	if req.Domain == "" && req.URL == "" {
		return fmt.Errorf("域名或URL必须提供")
	}

	if req.Domain != "" && !utils.IsValidDomain(req.Domain) {
		return fmt.Errorf("无效的域名格式: %s", req.Domain)
	}

	if req.URL != "" && !utils.ValidateURL(req.URL) {
		return fmt.Errorf("不安全的URL: %s", req.URL)
	}

	if req.Type == TypeElement && req.Selector == "" {
		return fmt.Errorf("元素截图必须提供选择器")
	}

	if req.Timeout > 0 && req.Timeout > 300 {
		return fmt.Errorf("超时时间不能超过300秒")
	}

	// 验证选择器安全性
	if req.Selector != "" {
		if strings.Contains(req.Selector, "javascript:") ||
			strings.Contains(req.Selector, "eval(") ||
			strings.Contains(req.Selector, "script") {
			return fmt.Errorf("选择器包含不安全的内容")
		}
	}

	return nil
}

// generateConfig 生成截图配置
func (s *ScreenshotService) generateConfig(req *ScreenshotRequest) (*ScreenshotConfig, error) {
	config := &ScreenshotConfig{
		Type:        req.Type,
		Selector:    req.Selector,
		Description: s.getDescription(req.Type),
	}

	// 设置URL
	if req.URL != "" {
		config.URL = req.URL
	} else {
		config.URL = s.buildURL(req.Type, req.Domain)
	}

	// 设置时间参数
	config.WaitTime = s.config.DefaultWaitTime
	if req.WaitTime > 0 {
		config.WaitTime = time.Duration(req.WaitTime) * time.Second
	}

	config.Timeout = s.config.DefaultTimeout
	if req.Timeout > 0 {
		config.Timeout = time.Duration(req.Timeout) * time.Second
	}

	// 设置缓存键
	config.CacheKey = s.buildCacheKey(req)

	// 设置文件路径（仅文件格式需要）
	if req.Format == FormatFile {
		config.FilePath, config.FileURL = s.buildFilePaths(req)
	}

	return config, nil
}

// buildURL 构建目标URL
func (s *ScreenshotService) buildURL(screenshotType ScreenshotType, domain string) string {
	switch screenshotType {
	case TypeItdogMap, TypeItdogTable, TypeItdogIP, TypeItdogResolve:
		return fmt.Sprintf("https://www.itdog.cn/ping/%s", domain)
	default:
		return fmt.Sprintf("https://%s", domain)
	}
}

// buildCacheKey 构建缓存键
func (s *ScreenshotService) buildCacheKey(req *ScreenshotRequest) string {
	data := fmt.Sprintf("%s_%s_%s_%s_%d_%d",
		req.Type, req.Domain, req.URL, req.Selector, req.WaitTime, req.Timeout)
	hash := fmt.Sprintf("%x", md5.Sum([]byte(data)))
	return fmt.Sprintf("screenshot:%s:%s", req.Type, hash[:16])
}

// buildFilePaths 构建文件路径
func (s *ScreenshotService) buildFilePaths(req *ScreenshotRequest) (string, string) {
	// 安全的文件名生成
	safeDomain := utils.GenerateSecureFilename(req.Domain)
	timestamp := time.Now().Unix()

	var subDir string
	switch req.Type {
	case TypeItdogMap, TypeItdogTable, TypeItdogIP, TypeItdogResolve:
		subDir = "itdog"
	default:
		subDir = "screenshots"
	}

	fileName := fmt.Sprintf("%s_%s_%d.png", req.Type, safeDomain, timestamp)
	filePath := filepath.Join(s.config.BaseDir, subDir, fileName)
	fileURL := fmt.Sprintf("/static/%s/%s", subDir, fileName)

	return filePath, fileURL
}

// getDescription 获取描述
func (s *ScreenshotService) getDescription(screenshotType ScreenshotType) string {
	descriptions := map[ScreenshotType]string{
		TypeBasic:        "基础截图",
		TypeElement:      "元素截图",
		TypeItdogMap:     "ITDog测速地图",
		TypeItdogTable:   "ITDog测速表格",
		TypeItdogIP:      "ITDog IP统计",
		TypeItdogResolve: "ITDog综合测速",
	}
	return descriptions[screenshotType]
}

// executeScreenshot 执行截图
func (s *ScreenshotService) executeScreenshot(ctx context.Context, config *ScreenshotConfig) (*ScreenshotResponse, error) {
	// 获取Chrome上下文
	chromeCtx, cancel, err := s.chromeManager.GetContext(config.Timeout)
	if err != nil {
		return nil, fmt.Errorf("获取Chrome上下文失败: %v", err)
	}
	defer cancel()

	// 创建带超时的上下文
	taskCtx, taskCancel := context.WithTimeout(chromeCtx, config.Timeout)
	defer taskCancel()

	// 执行截图操作
	var buf []byte
	switch config.Type {
	case TypeBasic:
		err = s.takeBasicScreenshot(taskCtx, config, &buf)
	case TypeElement:
		err = s.takeElementScreenshot(taskCtx, config, &buf)
	case TypeItdogMap, TypeItdogTable, TypeItdogIP, TypeItdogResolve:
		err = s.takeItdogScreenshot(taskCtx, config, &buf)
	default:
		return nil, fmt.Errorf("不支持的截图类型: %s", config.Type)
	}

	if err != nil {
		return nil, err
	}

	// 处理结果
	return s.processResult(buf, config)
}

// takeBasicScreenshot 基础截图
func (s *ScreenshotService) takeBasicScreenshot(ctx context.Context, config *ScreenshotConfig, buf *[]byte) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(config.URL),
		chromedp.Sleep(config.WaitTime),
		chromedp.CaptureScreenshot(buf),
	)
}

// takeElementScreenshot 元素截图
func (s *ScreenshotService) takeElementScreenshot(ctx context.Context, config *ScreenshotConfig, buf *[]byte) error {
	// 检查选择器类型
	var selectorType chromedp.QueryOption
	if strings.HasPrefix(config.Selector, "//") {
		selectorType = chromedp.BySearch // XPath
	} else {
		selectorType = chromedp.ByQuery // CSS
	}

	return chromedp.Run(ctx,
		chromedp.Navigate(config.URL),
		chromedp.Sleep(config.WaitTime),
		chromedp.WaitVisible(config.Selector, selectorType),
		chromedp.Screenshot(config.Selector, buf, chromedp.NodeVisible, selectorType),
	)
}

// takeItdogScreenshot ITDog截图
func (s *ScreenshotService) takeItdogScreenshot(ctx context.Context, config *ScreenshotConfig, buf *[]byte) error {
	// 设置选择器
	selector := s.getItdogSelector(config.Type)

	return chromedp.Run(ctx,
		// 导航到页面
		chromedp.Navigate(config.URL),
		chromedp.Sleep(2*time.Second),

		// 等待并点击测试按钮
		chromedp.WaitVisible(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),
		chromedp.Click(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),

		// 等待测试完成
		s.waitForItdogCompletion(),

		// 等待页面更新
		chromedp.Sleep(5*time.Second),

		// 截图
		s.screenshotItdogElement(selector, buf),
	)
}

// getItdogSelector 获取ITDog选择器
func (s *ScreenshotService) getItdogSelector(screenshotType ScreenshotType) string {
	selectors := map[ScreenshotType]string{
		TypeItdogMap:     "#china_map",
		TypeItdogTable:   ".card.mb-0[style*='height:550px']",
		TypeItdogIP:      `//div[contains(@class, "card") and contains(@class, "mb-0")][.//h5[contains(text(), "域名解析统计")]]`,
		TypeItdogResolve: ".dt-responsive.table-responsive",
	}
	return selectors[screenshotType]
}

// waitForItdogCompletion 等待ITDog测试完成
func (s *ScreenshotService) waitForItdogCompletion() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		maxAttempts := 60
		for attempts := 0; attempts < maxAttempts; attempts++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			var isDone bool
			err := chromedp.Evaluate(`(() => {
				const progressBar = document.querySelector('.progress-bar');
				const nodeNum = document.querySelector('#check_node_num');
				if (!progressBar || !nodeNum) return false;

				const current = parseInt(progressBar.getAttribute('aria-valuenow') || '0');
				const total = parseInt(nodeNum.textContent || '0');

				return total > 0 && current === total;
			})()`, &isDone).Do(ctx)

			if err != nil {
				return err
			}

			if isDone {
				return nil
			}

			time.Sleep(1 * time.Second)
		}
		return nil // 超时不报错，继续执行
	})
}

// screenshotItdogElement 截图ITDog元素
func (s *ScreenshotService) screenshotItdogElement(selector string, buf *[]byte) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if strings.HasPrefix(selector, "//") {
			return chromedp.Screenshot(selector, buf, chromedp.NodeVisible, chromedp.BySearch).Do(ctx)
		}
		return chromedp.Screenshot(selector, buf, chromedp.NodeVisible, chromedp.ByQuery).Do(ctx)
	})
}

// processResult 处理截图结果
func (s *ScreenshotService) processResult(buf []byte, config *ScreenshotConfig) (*ScreenshotResponse, error) {
	if len(buf) == 0 {
		return nil, fmt.Errorf("截图数据为空")
	}

	response := &ScreenshotResponse{
		Success: true,
		Metadata: map[string]interface{}{
			"size":        len(buf),
			"type":        config.Type,
			"description": config.Description,
		},
	}

	// 根据输出格式处理
	if config.FilePath != "" {
		// 文件格式
		if err := os.WriteFile(config.FilePath, buf, 0644); err != nil {
			return nil, fmt.Errorf("保存文件失败: %v", err)
		}
		response.ImageURL = config.FileURL
	} else {
		// Base64格式
		base64Data := base64.StdEncoding.EncodeToString(buf)
		response.ImageURL = fmt.Sprintf("data:image/png;base64,%s", base64Data)
		response.ImageBase64 = base64Data
	}

	return response, nil
}

// handleError 处理错误
func (s *ScreenshotService) handleError(err error, config *ScreenshotConfig) (*ScreenshotResponse, error) {
	errStr := err.Error()

	// 网络连接错误
	if strings.Contains(errStr, "net::ERR_NAME_NOT_RESOLVED") ||
		strings.Contains(errStr, "net::ERR_CONNECTION_REFUSED") ||
		strings.Contains(errStr, "net::ERR_CONNECTION_TIMED_OUT") {
		return &ScreenshotResponse{
			Success: false,
			Error:   "NETWORK_ERROR",
			Message: "无法连接到目标网站",
		}, nil
	}

	// 元素未找到
	if strings.Contains(errStr, "waiting for selector") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "not visible") {
		return &ScreenshotResponse{
			Success: false,
			Error:   "ELEMENT_NOT_FOUND",
			Message: "页面元素未找到或不可见",
		}, nil
	}

	// 超时错误
	if strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout") {
		return &ScreenshotResponse{
			Success: false,
			Error:   "TIMEOUT",
			Message: "操作超时",
		}, nil
	}

	// Chrome错误
	if strings.Contains(errStr, "chrome") || strings.Contains(errStr, "chromedp") {
		return &ScreenshotResponse{
			Success: false,
			Error:   "BROWSER_ERROR",
			Message: "浏览器执行错误",
		}, nil
	}

	// 默认错误
	return &ScreenshotResponse{
		Success: false,
		Error:   "UNKNOWN_ERROR",
		Message: "截图操作失败",
	}, nil
}

// 缓存相关方法
func (s *ScreenshotService) getFromCache(key string) *ScreenshotResponse {
	if s.redisClient == nil {
		return nil
	}

	data, err := s.redisClient.Get(context.Background(), key).Result()
	if err != nil {
		return nil
	}

	var response ScreenshotResponse
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		return nil
	}

	return &response
}

func (s *ScreenshotService) cacheResult(key string, response *ScreenshotResponse, expireHours int) {
	if s.redisClient == nil {
		return
	}

	// 🔐 P2-2修复：防止用户设置过大的缓存TTL导致Redis内存耗尽
	// 默认使用配置的过期时间
	expiration := s.config.CacheExpiration
	if expiration <= 0 {
		// 如果配置值无效，使用默认值24小时
		expiration = 24 * time.Hour
	}

	// 如果用户指定了缓存时间
	if expireHours > 0 {
		// 强制上限：最多72小时（3天）
		if expireHours > MaxUserCacheExpireHours {
			log.Printf("[Security] User requested cache TTL %dh exceeds max %dh, clamping to max",
				expireHours, MaxUserCacheExpireHours)
			expireHours = MaxUserCacheExpireHours
		}

		// 强制下限：至少1分钟，防止过于频繁的缓存失效
		userExpiration := time.Duration(expireHours) * time.Hour
		if userExpiration < time.Minute {
			log.Printf("[Security] User requested cache TTL %v is too small, setting to 1 minute", userExpiration)
			userExpiration = time.Minute
		}

		expiration = userExpiration
	}

	data, err := json.Marshal(response)
	if err != nil {
		return
	}

	s.redisClient.Set(context.Background(), key, data, expiration)
}