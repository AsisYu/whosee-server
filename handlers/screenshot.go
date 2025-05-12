/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-03-31 04:10:00
 * @Description: 域名截图处理程序
 */
package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"dmainwhoseek/services"
)

// ScreenshotResponse 定义截图API的响应结构
type ScreenshotResponse struct {
	Success   bool   `json:"success"`
	ImageUrl  string `json:"imageUrl,omitempty"`
	FromCache bool   `json:"fromCache,omitempty"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ElementScreenshotRequest 定义元素截图请求结构
type ElementScreenshotRequest struct {
	URL      string `json:"url" binding:"required"`
	Selector string `json:"selector" binding:"required"`
	Wait     int    `json:"wait,omitempty"` // 等待时间（秒）
}

// 截图缓存时间（24小时）
const screenshotCacheDuration = 24 * time.Hour

// 截图存储目录
const screenshotDir = "./static/screenshots"

// ITDog测速截图存储目录
const itdogScreenshotDir = "./static/itdog"

// 截图 处理域名截图请求
func Screenshot(c *gin.Context, rdb *redis.Client) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "域名参数必填",
		})
		return
	}

	// 记录开始时间，用于计算耗时
	startTime := time.Now()

	// 检查域名格式
	if !strings.Contains(domain, ".") {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "无效的域名格式",
		})
		return
	}

	// 检查服务熔断器状态
	sb := services.GetServiceBreakers()
	if !sb.ScreenshotBreaker.AllowRequest() {
		log.Printf("截图服务熔断器开启，拒绝请求: %s", domain)
		c.JSON(http.StatusServiceUnavailable, ScreenshotResponse{
			Success: false,
			Error:   "截图服务暂不可用",
			Message: "服务过载，请稍后再试",
		})
		return
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("screenshot:%s", domain)

	// 检查缓存
	cachedData, err := rdb.Get(context.Background(), cacheKey).Result()
	if err == nil {
		// 缓存命中
		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("使用缓存的截图: %s", domain)
			response.FromCache = true
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// 确保截图目录存在
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		log.Printf("创建截图目录失败: %v", err)
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "服务器内部错误",
		})
		return
	}

	// 生成文件名
	fileName := fmt.Sprintf("%s_%d.png", domain, time.Now().Unix())
	filePath := filepath.Join(screenshotDir, fileName)
	fileURL := fmt.Sprintf("/static/screenshots/%s", fileName)

	// 使用chromedp获取截图
	log.Printf("开始获取域名截图: %s", domain)

	// 执行截图
	err = sb.ScreenshotBreaker.Execute(func() error {
		// 创建上下文
		ctx, cancel := chromedp.NewContext(
			context.Background(),
			chromedp.WithLogf(log.Printf),
		)
		defer cancel()

		// 设置超时 - 允许超时45秒，避免服务不可用
		ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
		defer cancel()

		// 截图数据
		var buf []byte

		// 执行截图
		err := chromedp.Run(ctx,
			chromedp.Navigate(fmt.Sprintf("https://%s", domain)),
			chromedp.Sleep(5*time.Second),
			chromedp.CaptureScreenshot(&buf),
		)

		if err != nil {
			log.Printf("截图失败: %v", err)
			return err
		}

		// 保存截图
		if err := os.WriteFile(filePath, buf, 0644); err != nil {
			log.Printf("保存截图失败: %v", err)
			return err
		}

		return nil
	})

	if err != nil {
		log.Printf("截图失败: %v", err)

		// 检查是否是服务熔断器开启的错误
		if err.Error() == "circuit open" {
			c.JSON(http.StatusServiceUnavailable, ScreenshotResponse{
				Success: false,
				Error:   "截图服务暂不可用",
				Message: "服务过载，请稍后再试",
			})
			return
		}

		// 检查是否是网站无法访问的错误
		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") ||
			strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") {
			// 返回特定的错误信息，指示网站无法访问
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "网站无法访问",
				Message: fmt.Sprintf("无法连接到网站: %s - %s", domain, err.Error()),
			})
			return
		}

		// 其他类型的错误
		c.JSON(http.StatusOK, ScreenshotResponse{
			Success: false,
			Error:   fmt.Sprintf("截图失败: %v", err),
		})
		return
	}

	// 构建响应
	response := ScreenshotResponse{
		Success:   true,
		ImageUrl:  fileURL,
		FromCache: false,
	}

	// 缓存响应
	if responseJSON, err := json.Marshal(response); err == nil {
		rdb.Set(context.Background(), cacheKey, responseJSON, screenshotCacheDuration)
	}

	// 计算耗时
	duration := time.Since(startTime).Milliseconds()
	log.Printf("截图完成: %s, 耗时: %dms", domain, duration)

	c.JSON(http.StatusOK, response)
}

// ScreenshotBase64 返回Base64编码的截图
func ScreenshotBase64(c *gin.Context, rdb *redis.Client) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "域名参数必填",
		})
		return
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("screenshot:base64:%s", domain)

	// 检查缓存
	cachedData, err := rdb.Get(context.Background(), cacheKey).Result()
	if err == nil {
		// 缓存命中
		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("使用缓存的Base64截图: %s", domain)
			response.FromCache = true
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// 完整URL
	url := fmt.Sprintf("https://%s", domain)

	// 创建上下文
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// 设置超时
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 截图数据
	var buf []byte

	// 执行截图
	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
		chromedp.CaptureScreenshot(&buf),
	)

	if err != nil {
		log.Printf("截图失败: %v", err)

		// 检查是否是网站无法访问的错误
		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") ||
			strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") {
			// 返回特定的错误信息，指示网站无法访问
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "网站无法访问",
				Message: fmt.Sprintf("无法连接到网站: %s - %s", domain, err.Error()),
			})
			return
		}

		// 其他类型的错误
		c.JSON(http.StatusOK, ScreenshotResponse{
			Success: false,
			Error:   fmt.Sprintf("截图失败: %v", err),
		})
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
		rdb.Set(context.Background(), cacheKey, responseJSON, screenshotCacheDuration)
	}

	c.JSON(http.StatusOK, response)
}

// ElementScreenshot 处理元素截图请求
func ElementScreenshot(c *gin.Context, rdb *redis.Client) {
	var req ElementScreenshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 记录开始时间，用于计算耗时
	startTime := time.Now()

	// 检查URL格式
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		req.URL = "https://" + req.URL
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("screenshot:element:%s:%s", req.URL, req.Selector)

	// 检查缓存
	cachedData, err := rdb.Get(context.Background(), cacheKey).Result()
	if err == nil {
		// 缓存命中
		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("使用缓存的元素截图: %s, 选择器: %s", req.URL, req.Selector)
			response.FromCache = true
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// 确保截图目录存在
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		log.Printf("创建截图目录失败: %v", err)
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "服务器内部错误",
		})
		return
	}

	// 生成文件名
	fileName := fmt.Sprintf("element_%d_%s.png", time.Now().Unix(), strings.ReplaceAll(req.Selector, " ", "_"))
	filePath := filepath.Join(screenshotDir, fileName)
	fileURL := fmt.Sprintf("/static/screenshots/%s", fileName)

	// 使用chromedp获取元素截图
	log.Printf("开始获取元素截图: %s, 选择器: %s", req.URL, req.Selector)

	// 创建上下文
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// 设置超时
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 截图数据
	var buf []byte

	// 设置等待时间
	waitTime := 2 * time.Second
	if req.Wait > 0 && req.Wait <= 10 {
		waitTime = time.Duration(req.Wait) * time.Second
	}

	// 执行截图
	err = chromedp.Run(ctx,
		chromedp.Navigate(req.URL),
		chromedp.Sleep(waitTime), // 等待页面加载完成
		chromedp.WaitVisible(req.Selector, chromedp.ByQuery),
		chromedp.Screenshot(req.Selector, &buf, chromedp.NodeVisible, chromedp.ByQuery),
	)

	if err != nil {
		log.Printf("元素截图失败: %v", err)

		// 检查是否是网站无法访问的错误
		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") ||
			strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") {
			// 返回特定的错误信息，指示网站无法访问
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "网站无法访问",
				Message: fmt.Sprintf("无法连接到网站: %s - %s", req.URL, err.Error()),
			})
			return
		}

		// 检查是否是元素未找到的错误
		if strings.Contains(err.Error(), "waiting for selector") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "not visible") {
			// 返回特定的错误信息，指示元素未找到
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "无法获取元素截图",
				Message: fmt.Sprintf("无法在网站上找到元素: %s, 选择器: %s", req.URL, req.Selector),
			})
			return
		}

		// 其他类型的错误
		c.JSON(http.StatusOK, ScreenshotResponse{
			Success: false,
			Error:   fmt.Sprintf("元素截图失败: %v", err),
		})
		return
	}

	// 保存截图
	if err := os.WriteFile(filePath, buf, 0644); err != nil {
		log.Printf("保存截图失败: %v", err)
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "保存截图失败",
		})
		return
	}

	// 构建响应
	response := ScreenshotResponse{
		Success:   true,
		ImageUrl:  fileURL,
		FromCache: false,
	}

	// 缓存响应
	if responseJSON, err := json.Marshal(response); err == nil {
		rdb.Set(context.Background(), cacheKey, responseJSON, screenshotCacheDuration)
	}

	// 计算耗时
	duration := time.Since(startTime).Milliseconds()
	log.Printf("元素截图完成: %s, 选择器: %s, 耗时: %dms", req.URL, req.Selector, duration)

	c.JSON(http.StatusOK, response)
}

// ElementScreenshotBase64 返回Base64编码的元素截图
func ElementScreenshotBase64(c *gin.Context, rdb *redis.Client) {
	var req ElementScreenshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 检查URL格式
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		req.URL = "https://" + req.URL
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("screenshot:element:base64:%s:%s", req.URL, req.Selector)

	// 检查缓存
	cachedData, err := rdb.Get(context.Background(), cacheKey).Result()
	if err == nil {
		// 缓存命中
		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("使用缓存的Base64元素截图: %s, 选择器: %s", req.URL, req.Selector)
			response.FromCache = true
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// 设置等待时间
	waitTime := 2 * time.Second
	if req.Wait > 0 && req.Wait <= 10 {
		waitTime = time.Duration(req.Wait) * time.Second
	}

	// 创建上下文
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// 设置超时
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 截图数据
	var buf []byte

	// 执行截图
	err = chromedp.Run(ctx,
		chromedp.Navigate(req.URL),
		chromedp.Sleep(waitTime), // 等待页面加载完成
		chromedp.WaitVisible(req.Selector, chromedp.ByQuery),
		chromedp.Screenshot(req.Selector, &buf, chromedp.NodeVisible, chromedp.ByQuery),
	)

	if err != nil {
		log.Printf("元素截图失败: %v", err)

		// 检查是否是网站无法访问的错误
		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") ||
			strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") {
			// 返回特定的错误信息，指示网站无法访问
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "网站无法访问",
				Message: fmt.Sprintf("无法连接到网站: %s - %s", req.URL, err.Error()),
			})
			return
		}

		// 检查是否是元素未找到的错误
		if strings.Contains(err.Error(), "waiting for selector") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "not visible") {
			// 返回特定的错误信息，指示元素未找到
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "无法获取元素截图",
				Message: fmt.Sprintf("无法在网站上找到元素: %s, 选择器: %s", req.URL, req.Selector),
			})
			return
		}

		// 其他类型的错误
		c.JSON(http.StatusOK, ScreenshotResponse{
			Success: false,
			Error:   fmt.Sprintf("元素截图失败: %v", err),
		})
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
		rdb.Set(context.Background(), cacheKey, responseJSON, screenshotCacheDuration)
	}

	c.JSON(http.StatusOK, response)
}

// ItdogScreenshot 处理itdog测速截图请求
func ItdogScreenshot(c *gin.Context, rdb *redis.Client) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "域名参数必填",
		})
		return
	}

	// 记录开始时间，用于计算耗时
	startTime := time.Now()

	// 检查域名格式
	if !strings.Contains(domain, ".") {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "无效的域名格式",
		})
		return
	}

	// 检查ITDog测速服务熔断器状态
	sb := services.GetServiceBreakers()
	if !sb.ItdogBreaker.AllowRequest() {
		log.Printf("ITDog测速服务熔断器开启，拒绝请求: %s", domain)
		c.JSON(http.StatusServiceUnavailable, ScreenshotResponse{
			Success: false,
			Error:   "ITDog测速服务暂不可用",
			Message: "服务过载，请稍后再试",
		})
		return
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("itdog_screenshot:%s", domain)

	// 检查缓存
	cachedData, err := rdb.Get(context.Background(), cacheKey).Result()
	if err == nil {
		// 缓存命中
		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("使用缓存的itdog测速截图: %s", domain)
			response.FromCache = true
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// 确保截图目录存在
	if err := os.MkdirAll(itdogScreenshotDir, 0755); err != nil {
		log.Printf("创建ITDog截图目录失败: %v", err)
		c.JSON(http.StatusInternalServerError, ScreenshotResponse{
			Success: false,
			Error:   "服务器内部错误",
		})
		return
	}

	// 生成文件名
	fileName := fmt.Sprintf("itdog_%s_%d.png", domain, time.Now().Unix())
	filePath := filepath.Join(itdogScreenshotDir, fileName)
	fileURL := fmt.Sprintf("/static/itdog/%s", fileName)

	// 使用chromedp获取截图
	log.Printf("开始获取itdog测速截图: %s", domain)

	// 执行截图
	err = sb.ItdogBreaker.Execute(func() error {
		// 创建上下文
		ctx, cancel := chromedp.NewContext(
			context.Background(),
			chromedp.WithLogf(log.Printf),
		)
		defer cancel()

		// 设置超时 - 允许超时90秒，避免服务不可用
		ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()

		// 截图数据
		var buf []byte

		// 执行截图
		err := chromedp.Run(ctx,
			// 导航到itdog测速页面
			chromedp.Navigate(fmt.Sprintf("https://www.itdog.cn/ping/%s", domain)),

			// 等待"单次测试"按钮出现
			chromedp.WaitVisible(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),

			// 点击"单次测试"按钮
			chromedp.Click(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),

			// 等待测试完成 - 通过检查进度条
			chromedp.Sleep(2*time.Second), // 先等待2秒让测试开始

			// 使用循环检查进度条，最多等待45秒
			func() chromedp.Action {
				return chromedp.ActionFunc(func(ctx context.Context) error {
					var isDone bool
					var attempts int
					for attempts < 45 { // 最多尝试45次，每次等待1秒
						// 执行JavaScript检查进度
						err := chromedp.Evaluate(`(() => {
							const progressBar = document.querySelector('.progress-bar');
							const nodeNum = document.querySelector('#check_node_num');
							if (!progressBar || !nodeNum) return false;
							
							// 获取当前进度值和总节点数
							const current = parseInt(progressBar.getAttribute('aria-valuenow') || '0');
							const total = parseInt(nodeNum.textContent || '0');
							
							// 确保进度值有效且达到总数
							return total > 0 && current === total;
						})()`, &isDone).Do(ctx)

						if err != nil {
							return err
						}

						if isDone {
							return nil // 测试完成，退出循环
						}

						// 等待1秒后再次检查
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(1 * time.Second):
							attempts++
						}
					}
					return nil // 达到最大尝试次数，继续执行
				})
			}(),

			// 额外等待一段时间确保地图更新
			chromedp.Sleep(3*time.Second),

			// 截取中国地图元素
			chromedp.Screenshot("#china_map", &buf, chromedp.NodeVisible, chromedp.ByQuery),
		)

		if err != nil {
			log.Printf("itdog测速截图失败: %v", err)
			return err
		}

		// 保存截图
		if err := os.WriteFile(filePath, buf, 0644); err != nil {
			log.Printf("保存itdog测速截图失败: %v", err)
			return err
		}

		return nil
	})

	if err != nil {
		log.Printf("itdog测速截图失败: %v", err)

		// 检查是否是服务熔断器开启的错误
		if err.Error() == "circuit open" {
			c.JSON(http.StatusServiceUnavailable, ScreenshotResponse{
				Success: false,
				Error:   "ITDog测速服务暂不可用",
				Message: "服务过载，请稍后再试",
			})
			return
		}

		// 检查是否是网站无法访问的错误
		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") ||
			strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") ||
			strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "TLS handshake timeout") {
			// 返回特定的错误信息，指示itdog网站无法访问
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "ITDog测速网站无法访问",
				Message: fmt.Sprintf("无法连接到ITDog测速网站: %s", domain),
			})
			return
		}

		// 检查是否是元素未找到的错误
		if strings.Contains(err.Error(), "waiting for selector") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "not visible") {
			// 返回特定的错误信息，指示元素未找到
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "无法获取测速地图",
				Message: fmt.Sprintf("无法在itdog网站上找到测速地图元素，域名: %s", domain),
			})
			return
		}

		// 其他类型的错误
		c.JSON(http.StatusOK, ScreenshotResponse{
			Success: false,
			Error:   fmt.Sprintf("itdog测速截图失败: %v", err),
		})
		return
	}

	// 构建响应
	response := ScreenshotResponse{
		Success:   true,
		ImageUrl:  fileURL,
		FromCache: false,
	}

	// 缓存响应
	if responseJSON, err := json.Marshal(response); err == nil {
		rdb.Set(context.Background(), cacheKey, responseJSON, screenshotCacheDuration)
	}

	// 计算耗时
	duration := time.Since(startTime).Milliseconds()
	log.Printf("itdog测速截图完成: %s, 耗时: %dms", domain, duration)

	c.JSON(http.StatusOK, response)
}

// ItdogScreenshotBase64 返回Base64编码的itdog测速截图
func ItdogScreenshotBase64(c *gin.Context, rdb *redis.Client) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "域名参数必填",
		})
		return
	}

	// 记录开始时间，用于计算耗时
	startTime := time.Now()

	// 检查域名格式
	if !strings.Contains(domain, ".") {
		c.JSON(http.StatusBadRequest, ScreenshotResponse{
			Success: false,
			Error:   "无效的域名格式",
		})
		return
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("itdog_screenshot_base64:%s", domain)

	// 检查缓存
	cachedData, err := rdb.Get(context.Background(), cacheKey).Result()
	if err == nil {
		// 缓存命中
		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("使用缓存的itdog测速截图(Base64): %s", domain)
			response.FromCache = true
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// 使用chromedp获取截图
	log.Printf("开始获取itdog测速截图(Base64): %s", domain)

	// 创建上下文
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// 设置超时
	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	// 截图数据
	var buf []byte

	// 执行截图
	err = chromedp.Run(ctx,
		// 导航到itdog测速页面
		chromedp.Navigate(fmt.Sprintf("https://www.itdog.cn/ping/%s", domain)),

		// 等待"单次测试"按钮出现
		chromedp.WaitVisible(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),

		// 点击"单次测试"按钮
		chromedp.Click(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),

		// 等待测试完成 - 通过检查进度条
		chromedp.Sleep(2*time.Second), // 先等待2秒让测试开始

		// 使用循环检查进度条，最多等待45秒
		func() chromedp.Action {
			return chromedp.ActionFunc(func(ctx context.Context) error {
				var isDone bool
				var attempts int
				for attempts < 45 { // 最多尝试45次，每次等待1秒
					// 执行JavaScript检查进度
					err := chromedp.Evaluate(`(() => {
						const progressBar = document.querySelector('.progress-bar');
						const nodeNum = document.querySelector('#check_node_num');
						if (!progressBar || !nodeNum) return false;
						
						// 获取当前进度值和总节点数
						const current = parseInt(progressBar.getAttribute('aria-valuenow') || '0');
						const total = parseInt(nodeNum.textContent || '0');
						
						// 确保进度值有效且达到总数
						return total > 0 && current === total;
					})()`, &isDone).Do(ctx)

					if err != nil {
						return err
					}

					if isDone {
						return nil // 测试完成，退出循环
					}

					// 等待1秒后再次检查
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(1 * time.Second):
						attempts++
					}
				}
				return nil // 达到最大尝试次数，继续执行
			})
		}(),

		// 额外等待一段时间确保地图更新
		chromedp.Sleep(3*time.Second),

		// 截取中国地图元素
		chromedp.Screenshot("#china_map", &buf, chromedp.NodeVisible, chromedp.ByQuery),
	)

	if err != nil {
		log.Printf("itdog测速截图(Base64)失败: %v", err)

		// 检查是否是网站无法访问的错误
		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") ||
			strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") ||
			strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") ||
			strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "TLS handshake timeout") {
			// 返回特定的错误信息，指示itdog网站无法访问
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "itdog测速网站无法访问",
				Message: fmt.Sprintf("无法连接到itdog测速网站: %s", domain),
			})
			return
		}

		// 检查是否是元素未找到的错误
		if strings.Contains(err.Error(), "waiting for selector") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "not visible") {
			// 返回特定的错误信息，指示元素未找到
			c.JSON(http.StatusOK, ScreenshotResponse{
				Success: false,
				Error:   "无法获取测速地图",
				Message: fmt.Sprintf("无法在itdog网站上找到测速地图元素，域名: %s", domain),
			})
			return
		}

		// 其他类型的错误
		c.JSON(http.StatusOK, ScreenshotResponse{
			Success: false,
			Error:   fmt.Sprintf("itdog测速截图失败: %v", err),
		})
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
		rdb.Set(context.Background(), cacheKey, responseJSON, screenshotCacheDuration)
	}

	// 计算耗时
	duration := time.Since(startTime).Milliseconds()
	log.Printf("itdog测速截图(Base64)完成: %s, 耗时: %dms", domain, duration)

	c.JSON(http.StatusOK, response)
}
