/*
 * @Author: AsisYu
 * @Date: 2025-04-25
 * @Description: 截图相关API处理程序
 */
package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"whosee/utils"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// ScreenshotBase64Handler Base64编码的网站截图处理程序
func ScreenshotBase64Handler(c *gin.Context) {
	// 从路径参数获取域名
	domainStr := c.Param("domain")
	if domainStr == "" {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "域名参数必填",
		})
		return
	}

	// 检查域名格式
	if !strings.Contains(domainStr, ".") {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "无效的域名格式",
		})
		return
	}

	// 从上下文获取Redis客户端
	redisClient, exists := c.Get("redis")
	if !exists {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "无法获取Redis客户端",
		})
		return
	}

	// 类型断言
	redisClientPtr, ok := redisClient.(*redis.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "Redis客户端类型错误",
		})
		return
	}

	// 记录开始时间
	startTime := time.Now()

	// 为域名构建缓存键
	cacheKey := utils.BuildCacheKey("cache", "screenshot", "base64", utils.SanitizeDomain(domainStr))

	// 检查缓存
	cachedData, err := redisClientPtr.Get(context.Background(), cacheKey).Result()
	if err == nil {
		// 缓存命中
		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			// 计算处理时间
			processingTime := time.Since(startTime).Milliseconds()
			log.Printf("[SCREENSHOT-BASE64] 域名: %s | 缓存命中 | 耗时: %dms", domainStr, processingTime)

			response.FromCache = true
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// 完整URL
	url := fmt.Sprintf("https://%s", domainStr)

	// 创建上下文
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// 设置超时
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 截图数据
	var buf []byte

	// 执行截图
	log.Printf("[SCREENSHOT-BASE64] 域名: %s | 开始截图", domainStr)
	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
		chromedp.CaptureScreenshot(&buf),
	)

	if err != nil {
		// 计算处理时间
		processingTime := time.Since(startTime).Milliseconds()

		// 检查是否是网站无法访问的错误
		var errorResponse ScreenshotResponse
		var errorType string

		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") ||
			strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") {
			// 返回特定的错误信息，指示网站无法访问
			errorResponse = ScreenshotResponse{
				Success: false,
				Error:   "网站无法访问",
				Message: fmt.Sprintf("无法连接到网站: %s - %s", domainStr, err.Error()),
			}
			errorType = "网站无法访问"
		} else {
			// 其他类型的错误
			errorResponse = ScreenshotResponse{
				Success: false,
				Error:   fmt.Sprintf("截图失败: %v", err),
			}
			errorType = "截图失败"
		}

		log.Printf("[SCREENSHOT-BASE64] 域名: %s | 错误: %s | 耗时: %dms | 详情: %v",
			domainStr, errorType, processingTime, err)

		c.JSON(http.StatusOK, errorResponse)
		return
	}

	// 转换为Base64
	base64Data := base64.StdEncoding.EncodeToString(buf)
	dataURI := fmt.Sprintf("data:image/png;base64,%s", base64Data)

	// 构建响应
	response := ScreenshotResponse{
		Success:   true,
		ImageUrl:  dataURI,
		FromCache: false,
	}

	// 缓存响应
	if responseJSON, err := json.Marshal(response); err == nil {
		redisClientPtr.Set(context.Background(), cacheKey, responseJSON, 12*time.Hour)
	}

	// 计算处理时间
	processingTime := time.Since(startTime).Milliseconds()
	log.Printf("[SCREENSHOT-BASE64] 域名: %s | 成功 | 耗时: %dms", domainStr, processingTime)

	c.JSON(http.StatusOK, response)
}

// ITDogBase64Handler ITDogBase64编码的网站截图处理程序
func ITDogBase64Handler(c *gin.Context) {
	// 从路径参数获取域名
	domainStr := c.Param("domain")
	if domainStr == "" {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "域名参数必填",
		})
		return
	}

	// 检查域名格式
	if !strings.Contains(domainStr, ".") {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "无效的域名格式",
		})
		return
	}

	// 从上下文获取Redis客户端
	redisClient, exists := c.Get("redis")
	if !exists {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "无法获取Redis客户端",
		})
		return
	}

	// 类型断言
	redisClientPtr, ok := redisClient.(*redis.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "Redis客户端类型错误",
		})
		return
	}

	// 记录开始时间
	startTime := time.Now()

	// 为域名构建缓存键
	cacheKey := utils.BuildCacheKey("cache", "itdog", "map", "base64", utils.SanitizeDomain(domainStr))

	// 检查缓存
	cachedData, err := redisClientPtr.Get(context.Background(), cacheKey).Result()
	if err == nil {
		// 缓存命中
		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			// 计算处理时间
			processingTime := time.Since(startTime).Milliseconds()
			log.Printf("[ITDOG-BASE64] 域名: %s | 缓存命中 | 耗时: %dms", domainStr, processingTime)

			response.FromCache = true
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// 完整URL
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// 设置超时
	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	// 截图数据
	var buf []byte

	// 执行截图
	log.Printf("[ITDOG-BASE64] 域名: %s | 开始截图", domainStr)
	err = chromedp.Run(ctx,
		// 跳转到ITDog网站
		chromedp.Navigate(fmt.Sprintf("https://www.itdog.cn/ping/%s", domainStr)),

		// 等待"单次ping"按钮可见
		chromedp.WaitVisible(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),

		// 点击"单次ping"按钮
		chromedp.Click(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),

		// 等待ping完成 - 等待进度条完成
		chromedp.Sleep(2*time.Second), // 等待2秒钟确保ping开始

		// 使用JavaScript等待进度条完成
		func() chromedp.Action {
			return chromedp.ActionFunc(func(ctx context.Context) error {
				var isDone bool
				var attempts int
				for attempts < 45 { // 最多等待45秒
					// 执行JavaScript等待进度条完成
					err := chromedp.Evaluate(`(() => {
						const progressBar = document.querySelector('.progress-bar');
						const nodeNum = document.querySelector('#check_node_num');
						if (!progressBar || !nodeNum) return false;
						
						// 获取当前进度和总进度
						const current = parseInt(progressBar.getAttribute('aria-valuenow') || '0');
						const total = parseInt(nodeNum.textContent || '0');
						
						// 检查进度是否完成
						return total > 0 && current === total;
					})()`, &isDone).Do(ctx)

					if err != nil {
						return err
					}

					if isDone {
						return nil // 进度完成
					}

					// 等待1秒钟后重试
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(1 * time.Second):
						attempts++
					}
				}
				return nil // 最多等待45秒后超时
			})
		}(),

		// 等待3秒钟确保地图加载完成
		chromedp.Sleep(3*time.Second),

		// 截取地图
		chromedp.Screenshot("#china_map", &buf, chromedp.NodeVisible, chromedp.ByQuery),
	)

	if err != nil {
		// 计算处理时间
		processingTime := time.Since(startTime).Milliseconds()

		// 检查错误类型
		var errorResponse ScreenshotResponse
		var errorType string

		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") ||
			strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") ||
			strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "TLS handshake timeout") {
			// 网站无法访问
			errorResponse = ScreenshotResponse{
				Success: false,
				Error:   "网站无法访问",
				Message: fmt.Sprintf("无法连接到网站: %s - %s", domainStr, err.Error()),
			}
			errorType = "网站无法访问"
		} else if strings.Contains(err.Error(), "waiting for selector") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "not visible") {
			// 元素未找到
			errorResponse = ScreenshotResponse{
				Success: false,
				Error:   "元素未找到",
				Message: fmt.Sprintf("无法找到元素: %s", err.Error()),
			}
			errorType = "元素未找到"
		} else {
			// 其他错误
			errorResponse = ScreenshotResponse{
				Success: false,
				Error:   fmt.Sprintf("截图失败: %v", err),
			}
			errorType = "截图失败"
		}

		log.Printf("[ITDOG-BASE64] 域名: %s | 错误: %s | 耗时: %dms | 详情: %v",
			domainStr, errorType, processingTime, err)

		c.JSON(http.StatusOK, errorResponse)
		return
	}

	// 转换为Base64
	base64Data := base64.StdEncoding.EncodeToString(buf)
	dataURI := fmt.Sprintf("data:image/png;base64,%s", base64Data)

	// 构建响应
	response := ScreenshotResponse{
		Success:   true,
		ImageUrl:  dataURI,
		FromCache: false,
	}

	// 缓存响应
	if responseJSON, err := json.Marshal(response); err == nil {
		redisClientPtr.Set(context.Background(), cacheKey, responseJSON, 12*time.Hour)
	}

	// 计算处理时间
	processingTime := time.Since(startTime).Milliseconds()
	log.Printf("[ITDOG-BASE64] 域名: %s | 成功 | 耗时: %dms", domainStr, processingTime)

	c.JSON(http.StatusOK, response)
}

// ITDogTableHandler ITDog表格测速截图处理程序
func ITDogTableHandler(c *gin.Context) {
	// 从上下文获取Redis客户端
	redisClient, exists := c.Get("redis")
	if !exists {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "无法获取Redis客户端",
		})
		return
	}

	// 类型断言
	redisClientPtr, ok := redisClient.(*redis.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "Redis客户端类型错误",
		})
		return
	}

	// 调用核心实现函数
	ItdogTableScreenshot(c, redisClientPtr)
}

// ITDogTableBase64Handler ITDog表格测速Base64编码截图处理程序
func ITDogTableBase64Handler(c *gin.Context) {
	// 从上下文获取Redis客户端
	redisClient, exists := c.Get("redis")
	if !exists {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "无法获取Redis客户端",
		})
		return
	}

	// 类型断言
	redisClientPtr, ok := redisClient.(*redis.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "Redis客户端类型错误",
		})
		return
	}

	// 调用核心实现函数
	ItdogTableScreenshotBase64(c, redisClientPtr)
}

// ITDogIPHandler ITDog IP统计测速截图处理程序
func ITDogIPHandler(c *gin.Context) {
	// 从上下文获取Redis客户端
	redisClient, exists := c.Get("redis")
	if !exists {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "无法获取Redis客户端",
		})
		return
	}

	// 类型断言
	redisClientPtr, ok := redisClient.(*redis.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "Redis客户端类型错误",
		})
		return
	}

	// 调用核心实现函数
	ItdogIPScreenshot(c, redisClientPtr)
}

// ITDogIPBase64Handler ITDog IP统计测速Base64编码截图处理程序
func ITDogIPBase64Handler(c *gin.Context) {
	// 从上下文获取Redis客户端
	redisClient, exists := c.Get("redis")
	if !exists {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "无法获取Redis客户端",
		})
		return
	}

	// 类型断言
	redisClientPtr, ok := redisClient.(*redis.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "Redis客户端类型错误",
		})
		return
	}

	// 调用核心实现函数
	ItdogIPBase64Screenshot(c, redisClientPtr)
}

// ITDogResolveHandler ITDog综合测速截图处理程序
func ITDogResolveHandler(c *gin.Context) {
	// 从上下文获取Redis客户端
	redisClient, exists := c.Get("redis")
	if !exists {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "无法获取Redis客户端",
		})
		return
	}

	// 类型断言
	redisClientPtr, ok := redisClient.(*redis.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "Redis客户端类型错误",
		})
		return
	}

	// 调用核心实现函数
	ItdogResolveScreenshot(c, redisClientPtr)
}

// ITDogResolveBase64Handler ITDog综合测速Base64编码截图处理程序
func ITDogResolveBase64Handler(c *gin.Context) {
	// 从上下文获取Redis客户端
	redisClient, exists := c.Get("redis")
	if !exists {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "无法获取Redis客户端",
		})
		return
	}

	// 类型断言
	redisClientPtr, ok := redisClient.(*redis.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "内部服务器错误",
			Message: "Redis客户端类型错误",
		})
		return
	}

	// 调用核心实现函数
	ItdogResolveScreenshotBase64(c, redisClientPtr)
}
