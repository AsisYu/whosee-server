/*
 * @Author: AsisYu
 * @Date: 2025-01-20
 * @Description: 重构的截图处理器 - 统一接口实现
 */
package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"dmainwhoseek/services"
	"dmainwhoseek/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// NewScreenshotHandler 新的统一截图处理器
type NewScreenshotHandler struct {
	screenshotService *services.ScreenshotService
	chromeManager     *services.ChromeManager
}

// NewUnifiedScreenshotHandler 创建截图处理器
func NewUnifiedScreenshotHandler(screenshotService *services.ScreenshotService, chromeManager *services.ChromeManager) *NewScreenshotHandler {
	return &NewScreenshotHandler{
		screenshotService: screenshotService,
		chromeManager:     chromeManager,
	}
}

// TakeScreenshot 统一截图接口
func (h *NewScreenshotHandler) TakeScreenshot(c *gin.Context) {
	startTime := time.Now()

	// 解析请求
	req, err := h.parseRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, services.ScreenshotResponse{
			Success: false,
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	// 记录开始日志
	log.Printf("[SCREENSHOT] 开始%s截图: %s", req.Type, h.getDomainFromRequest(req))

	// 执行截图
	ctx := context.Background()
	response, err := h.screenshotService.TakeScreenshot(ctx, req)

	// 记录结果统计
	duration := time.Since(startTime)
	if err != nil {
		log.Printf("[SCREENSHOT] %s截图失败: %v, 耗时: %v", req.Type, err, duration)
		h.chromeManager.OnFailure(duration)
		c.JSON(http.StatusInternalServerError, services.ScreenshotResponse{
			Success: false,
			Error:   "INTERNAL_ERROR",
			Message: "截图服务内部错误",
		})
		return
	}

	// 记录成功统计
	if response.Success {
		h.chromeManager.OnSuccess(duration)
		log.Printf("[SCREENSHOT] %s截图成功, 耗时: %v, 缓存: %v",
			req.Type, duration, response.FromCache)
	} else {
		h.chromeManager.OnFailure(duration)
		log.Printf("[SCREENSHOT] %s截图失败: %s, 耗时: %v",
			req.Type, response.Error, duration)
	}

	// 返回结果
	c.JSON(http.StatusOK, response)
}

// parseRequest 解析请求参数
func (h *NewScreenshotHandler) parseRequest(c *gin.Context) (*services.ScreenshotRequest, error) {
	req := &services.ScreenshotRequest{}

	// 从URL路径解析类型和域名
	if screenshotType := c.Param("type"); screenshotType != "" {
		req.Type = services.ScreenshotType(screenshotType)
	}

	if domain := c.Param("domain"); domain != "" {
		req.Domain = domain
	}

	// 从查询参数解析其他参数
	if url := c.Query("url"); url != "" {
		req.URL = url
	}

	if selector := c.Query("selector"); selector != "" {
		req.Selector = selector
	}

	if format := c.Query("format"); format != "" {
		req.Format = services.OutputFormat(format)
	} else {
		req.Format = services.FormatFile // 默认文件格式
	}

	// 解析等待时间
	if waitStr := c.Query("wait"); waitStr != "" {
		if wait, err := strconv.Atoi(waitStr); err == nil && wait > 0 {
			req.WaitTime = wait
		}
	}

	// 解析超时时间
	if timeoutStr := c.Query("timeout"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			req.Timeout = timeout
		}
	}

	// 解析缓存时间
	if cacheStr := c.Query("cache"); cacheStr != "" {
		if cache, err := strconv.Atoi(cacheStr); err == nil && cache > 0 {
			req.CacheExpire = cache
		}
	}

	// 对于POST请求，尝试从请求体解析JSON
	if c.Request.Method == "POST" {
		var bodyReq services.ScreenshotRequest
		if err := c.ShouldBindJSON(&bodyReq); err == nil {
			// 合并请求体参数，请求体优先级更高
			if bodyReq.Type != "" {
				req.Type = bodyReq.Type
			}
			if bodyReq.Domain != "" {
				req.Domain = bodyReq.Domain
			}
			if bodyReq.URL != "" {
				req.URL = bodyReq.URL
			}
			if bodyReq.Selector != "" {
				req.Selector = bodyReq.Selector
			}
			if bodyReq.Format != "" {
				req.Format = bodyReq.Format
			}
			if bodyReq.WaitTime > 0 {
				req.WaitTime = bodyReq.WaitTime
			}
			if bodyReq.Timeout > 0 {
				req.Timeout = bodyReq.Timeout
			}
			if bodyReq.CacheExpire > 0 {
				req.CacheExpire = bodyReq.CacheExpire
			}
		}
	}

	// 设置默认值
	if req.Type == "" {
		req.Type = services.TypeBasic
	}

	return req, nil
}

// getDomainFromRequest 从请求中获取域名用于日志
func (h *NewScreenshotHandler) getDomainFromRequest(req *services.ScreenshotRequest) string {
	if req.Domain != "" {
		return req.Domain
	}
	if req.URL != "" {
		return req.URL
	}
	return "unknown"
}

// 为了兼容现有路由，保留旧的处理器函数

// NewScreenshot 基础截图处理器（新实现）
func NewScreenshot(c *gin.Context, rdb *redis.Client) {
	domain := c.Param("domain")
	if domain == "" {
		domain = c.Query("domain")
	}

	if domain == "" {
		c.JSON(http.StatusBadRequest, services.ScreenshotResponse{
			Success: false,
			Error:   "MISSING_PARAMETER",
			Message: "域名参数必填",
		})
		return
	}

	// 创建服务实例
	chromeManager := services.GetGlobalChromeManager()
	screenshotService := services.NewScreenshotService(chromeManager, rdb, nil)

	// 构造请求
	req := &services.ScreenshotRequest{
		Type:   services.TypeBasic,
		Domain: domain,
		Format: services.FormatFile,
	}

	// 执行截图
	ctx := context.Background()
	response, err := screenshotService.TakeScreenshot(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, services.ScreenshotResponse{
			Success: false,
			Error:   "INTERNAL_ERROR",
			Message: "截图服务内部错误",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// NewScreenshotBase64 Base64截图处理器（新实现）
func NewScreenshotBase64(c *gin.Context, rdb *redis.Client) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, services.ScreenshotResponse{
			Success: false,
			Error:   "MISSING_PARAMETER",
			Message: "域名参数必填",
		})
		return
	}

	// 创建服务实例
	chromeManager := services.GetGlobalChromeManager()
	screenshotService := services.NewScreenshotService(chromeManager, rdb, nil)

	// 构造请求
	req := &services.ScreenshotRequest{
		Type:   services.TypeBasic,
		Domain: domain,
		Format: services.FormatBase64,
	}

	// 执行截图
	ctx := context.Background()
	response, err := screenshotService.TakeScreenshot(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, services.ScreenshotResponse{
			Success: false,
			Error:   "INTERNAL_ERROR",
			Message: "截图服务内部错误",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// NewElementScreenshot 元素截图处理器（新实现）
func NewElementScreenshot(c *gin.Context, rdb *redis.Client) {
	var req struct {
		URL      string `json:"url" binding:"required"`
		Selector string `json:"selector" binding:"required"`
		Wait     int    `json:"wait,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, services.ScreenshotResponse{
			Success: false,
			Error:   "INVALID_REQUEST",
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 创建服务实例
	chromeManager := services.GetGlobalChromeManager()
	screenshotService := services.NewScreenshotService(chromeManager, rdb, nil)

	// 构造请求
	screenshotReq := &services.ScreenshotRequest{
		Type:     services.TypeElement,
		URL:      req.URL,
		Selector: req.Selector,
		Format:   services.FormatFile,
		WaitTime: req.Wait,
	}

	// 执行截图
	ctx := context.Background()
	response, err := screenshotService.TakeScreenshot(ctx, screenshotReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, services.ScreenshotResponse{
			Success: false,
			Error:   "INTERNAL_ERROR",
			Message: "截图服务内部错误",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// NewElementScreenshotBase64 元素截图Base64处理器（新实现）
func NewElementScreenshotBase64(c *gin.Context, rdb *redis.Client) {
	var req struct {
		URL      string `json:"url" binding:"required"`
		Selector string `json:"selector" binding:"required"`
		Wait     int    `json:"wait,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, services.ScreenshotResponse{
			Success: false,
			Error:   "INVALID_REQUEST",
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 创建服务实例
	chromeManager := services.GetGlobalChromeManager()
	screenshotService := services.NewScreenshotService(chromeManager, rdb, nil)

	// 构造请求
	screenshotReq := &services.ScreenshotRequest{
		Type:     services.TypeElement,
		URL:      req.URL,
		Selector: req.Selector,
		Format:   services.FormatBase64,
		WaitTime: req.Wait,
	}

	// 执行截图
	ctx := context.Background()
	response, err := screenshotService.TakeScreenshot(ctx, screenshotReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, services.ScreenshotResponse{
			Success: false,
			Error:   "INTERNAL_ERROR",
			Message: "截图服务内部错误",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ITDog截图处理器们（兼容旧接口）

// NewItdogScreenshot ITDog地图截图
func NewItdogScreenshot(c *gin.Context, rdb *redis.Client) {
	handleNewItdogScreenshot(c, rdb, services.TypeItdogMap, services.FormatFile)
}

// NewItdogScreenshotBase64 ITDog地图截图Base64
func NewItdogScreenshotBase64(c *gin.Context, rdb *redis.Client) {
	handleNewItdogScreenshot(c, rdb, services.TypeItdogMap, services.FormatBase64)
}

// NewItdogTableScreenshot ITDog表格截图
func NewItdogTableScreenshot(c *gin.Context, rdb *redis.Client) {
	handleNewItdogScreenshot(c, rdb, services.TypeItdogTable, services.FormatFile)
}

// NewItdogTableScreenshotBase64 ITDog表格截图Base64
func NewItdogTableScreenshotBase64(c *gin.Context, rdb *redis.Client) {
	handleNewItdogScreenshot(c, rdb, services.TypeItdogTable, services.FormatBase64)
}

// NewItdogIPScreenshot ITDog IP统计截图
func NewItdogIPScreenshot(c *gin.Context, rdb *redis.Client) {
	handleNewItdogScreenshot(c, rdb, services.TypeItdogIP, services.FormatFile)
}

// NewItdogIPBase64Screenshot ITDog IP统计截图Base64
func NewItdogIPBase64Screenshot(c *gin.Context, rdb *redis.Client) {
	handleNewItdogScreenshot(c, rdb, services.TypeItdogIP, services.FormatBase64)
}

// NewItdogResolveScreenshot ITDog综合测速截图
func NewItdogResolveScreenshot(c *gin.Context, rdb *redis.Client) {
	handleNewItdogScreenshot(c, rdb, services.TypeItdogResolve, services.FormatFile)
}

// NewItdogResolveScreenshotBase64 ITDog综合测速截图Base64
func NewItdogResolveScreenshotBase64(c *gin.Context, rdb *redis.Client) {
	handleNewItdogScreenshot(c, rdb, services.TypeItdogResolve, services.FormatBase64)
}

// handleNewItdogScreenshot ITDog截图通用处理函数
func handleNewItdogScreenshot(c *gin.Context, rdb *redis.Client, screenshotType services.ScreenshotType, format services.OutputFormat) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, services.ScreenshotResponse{
			Success: false,
			Error:   "MISSING_PARAMETER",
			Message: "域名参数必填",
		})
		return
	}

	// 验证域名格式
	if !utils.IsValidDomain(domain) {
		c.JSON(http.StatusBadRequest, services.ScreenshotResponse{
			Success: false,
			Error:   "INVALID_DOMAIN",
			Message: "无效的域名格式",
		})
		return
	}

	// 创建服务实例
	chromeManager := services.GetGlobalChromeManager()
	screenshotService := services.NewScreenshotService(chromeManager, rdb, nil)

	// 构造请求
	req := &services.ScreenshotRequest{
		Type:   screenshotType,
		Domain: domain,
		Format: format,
	}

	// 执行截图
	ctx := context.Background()
	response, err := screenshotService.TakeScreenshot(ctx, req)
	if err != nil {
		log.Printf("[SCREENSHOT] ITDog截图服务内部错误: %v", err)
		c.JSON(http.StatusInternalServerError, services.ScreenshotResponse{
			Success: false,
			Error:   "INTERNAL_ERROR",
			Message: "截图服务内部错误",
		})
		return
	}

	// 根据错误类型返回适当的HTTP状态码
	if !response.Success {
		switch response.Error {
		case "SERVICE_UNAVAILABLE":
			c.JSON(http.StatusServiceUnavailable, response)
			return
		case "TIMEOUT":
			c.JSON(http.StatusRequestTimeout, response)
			return
		case "NETWORK_ERROR":
			c.JSON(http.StatusBadGateway, response)
			return
		default:
			c.JSON(http.StatusOK, response) // 其他错误仍返回200，但在response中标明失败
			return
		}
	}

	c.JSON(http.StatusOK, response)
}

// NewChromeStatus 检查Chrome状态的API
func NewChromeStatus(c *gin.Context) {
	chromeManager := services.GetGlobalChromeManager()
	if chromeManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Chrome管理器未初始化",
		})
		return
	}

	stats := chromeManager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"chrome_status": stats,
		"message":       "Chrome状态检查完成",
	})
}

// NewChromeRestart 重启Chrome的API
func NewChromeRestart(c *gin.Context) {
	chromeManager := services.GetGlobalChromeManager()
	if chromeManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Chrome管理器未初始化",
		})
		return
	}

	err := chromeManager.Restart()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Chrome重启失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Chrome重启成功",
	})
}

// 新实现处理器映射
var NewScreenshotHandlerMapping = map[string]func(*gin.Context, *redis.Client){
	"NewScreenshot":                    NewScreenshot,
	"NewScreenshotBase64":              NewScreenshotBase64,
	"NewElementScreenshot":             NewElementScreenshot,
	"NewElementScreenshotBase64":       NewElementScreenshotBase64,
	"NewItdogScreenshot":               NewItdogScreenshot,
	"NewItdogScreenshotBase64":         NewItdogScreenshotBase64,
	"NewItdogTableScreenshot":          NewItdogTableScreenshot,
	"NewItdogTableScreenshotBase64":    NewItdogTableScreenshotBase64,
	"NewItdogIPScreenshot":             NewItdogIPScreenshot,
	"NewItdogIPBase64Screenshot":       NewItdogIPBase64Screenshot,
	"NewItdogResolveScreenshot":        NewItdogResolveScreenshot,
	"NewItdogResolveScreenshotBase64":  NewItdogResolveScreenshotBase64,
}

// 新版便捷函数用于在routes中调用
func NewScreenshotRouteHandler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewScreenshot(c, rdb)
}

func NewScreenshotBase64Handler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewScreenshotBase64(c, rdb)
}

func NewElementScreenshotHandler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewElementScreenshot(c, rdb)
}

func NewElementScreenshotBase64Handler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewElementScreenshotBase64(c, rdb)
}

func NewITDogHandler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewItdogScreenshot(c, rdb)
}

func NewITDogBase64Handler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewItdogScreenshotBase64(c, rdb)
}

func NewITDogTableHandler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewItdogTableScreenshot(c, rdb)
}

func NewITDogTableBase64Handler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewItdogTableScreenshotBase64(c, rdb)
}

func NewITDogIPHandler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewItdogIPScreenshot(c, rdb)
}

func NewITDogIPBase64Handler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewItdogIPBase64Screenshot(c, rdb)
}

func NewITDogResolveHandler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewItdogResolveScreenshot(c, rdb)
}

func NewITDogResolveBase64Handler(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	NewItdogResolveScreenshotBase64(c, rdb)
}