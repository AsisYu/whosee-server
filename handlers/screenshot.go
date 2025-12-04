/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-03-31 04:10:00
 * @Description: 域名截图处理程序
 */
package handlers

import (
	"context"
	"whosee/services"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"whosee/utils"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// ScreenshotResponse 定义截图API的响应结构
type ScreenshotResponse struct {
	Success     bool   `json:"success"`
	ImageUrl    string `json:"imageUrl,omitempty"`
	ImageBase64 string `json:"imageBase64,omitempty"`
	FromCache   bool   `json:"fromCache,omitempty"`
	Error       string `json:"error,omitempty"`
	Message     string `json:"message,omitempty"`
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

// ITDogScreenshotConfig ITDog截图配置
type ITDogScreenshotConfig struct {
	Domain      string
	CacheKey    string
	FileName    string
	FileURL     string
	FilePath    string
	Selector    string // 截图元素选择器
	Description string // 截图描述（用于日志）
}

// 通用小工具：缓存与文件写入，降低重复
func getJSONCache(ctx context.Context, rdb *redis.Client, key string, out interface{}) bool {
	if rdb == nil {
		return false
	}
	data, err := rdb.Get(ctx, key).Result()
	if err != nil || data == "" {
		return false
	}
	return json.Unmarshal([]byte(data), out) == nil
}

func setJSONCache(ctx context.Context, rdb *redis.Client, key string, value interface{}, ttl time.Duration) {
	if rdb == nil {
		return
	}
	b, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = rdb.Set(ctx, key, b, ttl).Err()
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// performITDogScreenshot 执行ITDog截图的通用函数
func performITDogScreenshot(config ITDogScreenshotConfig, rdb *redis.Client) (*ScreenshotResponse, error) {
	// 检查ITDog测速服务熔断器状态
	sb := services.GetServiceBreakers()
	if !sb.ItdogBreaker.AllowRequest() {
		log.Printf("ITDog测速服务熔断器开启，拒绝请求: %s", config.Domain)
		return &ScreenshotResponse{
			Success: false,
			Error:   "ITDog测速服务暂不可用",
			Message: "服务过载，请稍后再试",
		}, nil
	}

	// 缓存
	var cached ScreenshotResponse
	if getJSONCache(context.Background(), rdb, config.CacheKey, &cached) {
		log.Printf("使用缓存的%s: %s", config.Description, config.Domain)
		cached.FromCache = true
		return &cached, nil
	}

	// 目录
	if err := ensureDir(itdogScreenshotDir); err != nil {
		log.Printf("创建ITDog截图目录失败: %v", err)
		return &ScreenshotResponse{
			Success: false,
			Error:   "服务器内部错误",
		}, nil
	}

	// 使用统一的Chrome管理器获取截图
	log.Printf("开始获取%s (统一浏览器): %s", config.Description, config.Domain)

	// 执行截图
	err := sb.ItdogBreaker.Execute(func() error {
		// 获取全局Chrome工具
		chromeUtil := utils.GetGlobalChromeUtil()
		if chromeUtil == nil {
			return fmt.Errorf("Chrome工具未初始化")
		}

		// 增加重试机制
		maxRetries := 2
		for retry := 0; retry <= maxRetries; retry++ {
			if retry > 0 {
				log.Printf("[CHROME-UTIL] %s重试第 %d 次", config.Description, retry)
				time.Sleep(time.Duration(retry) * 2 * time.Second) // 递增等待时间
			}

			// 从Chrome工具获取上下文，设置120秒超时（增加超时时间）
			ctx, cancel, chromeErr := chromeUtil.GetContext(120 * time.Second)
			if chromeErr != nil {
				log.Printf("[CHROME-UTIL] 获取Chrome上下文失败 (重试 %d/%d): %v", retry, maxRetries, chromeErr)
				if retry == maxRetries {
					return fmt.Errorf("获取Chrome上下文失败: %v", chromeErr)
				}
				continue
			}

			// 检查上下文初始状态
			select {
			case <-ctx.Done():
				cancel()
				log.Printf("[CHROME-UTIL] 上下文在使用前已被取消 (重试 %d/%d)", retry, maxRetries)
				if retry == maxRetries {
					return fmt.Errorf("上下文在使用前已被取消")
				}
				continue
			default:
			}

			log.Printf("[CHROME-UTIL] 开始执行%s操作，域名: %s (重试 %d/%d)", config.Description, config.Domain, retry, maxRetries)

			// 截图数据
			var buf []byte

			// 执行截图操作
			err := chromedp.Run(ctx,
				// 导航到itdog测速页面
				chromedp.ActionFunc(func(ctx context.Context) error {
					// 检查上下文状态
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤1: 导航到ITDog页面: %s", fmt.Sprintf("https://www.itdog.cn/ping/%s", config.Domain))
					return chromedp.Navigate(fmt.Sprintf("https://www.itdog.cn/ping/%s", config.Domain)).Do(ctx)
				}),

				// 等待页面加载
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤2: 等待页面加载完成")
					return chromedp.Sleep(2 * time.Second).Do(ctx)
				}),

				// 等待"单次测试"按钮出现，增加超时时间
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤3: 等待单次测试按钮出现")
					return chromedp.WaitVisible(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery).Do(ctx)
				}),

				// 点击"单次测试"按钮
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤4: 点击单次测试按钮")
					return chromedp.Click(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery).Do(ctx)
				}),

				// 等待测试开始
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤5: 等待测试开始")
					return chromedp.Sleep(3 * time.Second).Do(ctx)
				}),

				// 使用循环检查进度条，最多等待60秒（增加等待时间）
				func() chromedp.Action {
					return chromedp.ActionFunc(func(ctx context.Context) error {
						var isDone bool
						var attempts int
						maxAttempts := 60 // 增加最大尝试次数

						log.Printf("[CHROME-UTIL] 步骤6: 开始检查%s测试进度", config.Description)

						for attempts < maxAttempts {
							// 检查上下文是否已取消
							select {
							case <-ctx.Done():
								log.Printf("[CHROME-UTIL] %s上下文已取消，停止等待", config.Description)
								return ctx.Err()
							default:
							}

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
								log.Printf("[CHROME-UTIL] %s进度检查出错: %v", config.Description, err)
								return err
							}

							if isDone {
								log.Printf("[CHROME-UTIL] %s测试完成，进度: %d/%d", config.Description, attempts, maxAttempts)
								return nil // 测试完成，退出循环
							}

							// 每5次尝试打印一次进度
							if attempts%5 == 0 && attempts > 0 {
								log.Printf("[CHROME-UTIL] %s等待测试完成，已等待 %d 秒", config.Description, attempts)
							}

							// 等待1秒后再次检查
							select {
							case <-ctx.Done():
								return ctx.Err()
							case <-time.After(1 * time.Second):
								attempts++
							}
						}

						log.Printf("[CHROME-UTIL] %s等待超时，已尝试 %d 次", config.Description, attempts)
						return nil // 达到最大尝试次数，继续执行
					})
				}(),

				// 额外等待一段时间确保页面元素更新
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤7: 等待页面元素更新")
					return chromedp.Sleep(5 * time.Second).Do(ctx)
				}),

				// 截取指定元素 - 检查是否是XPath选择器
				func() chromedp.Action {
					return chromedp.ActionFunc(func(ctx context.Context) error {
						select {
						case <-ctx.Done():
							return ctx.Err()
						default:
						}
						log.Printf("[CHROME-UTIL] 步骤8: 开始截图")
						if strings.HasPrefix(config.Selector, "//") {
							// 使用XPath选择器
							log.Printf("[CHROME-UTIL] 使用XPath选择器截图: %s", config.Selector)
							return chromedp.Screenshot(config.Selector, &buf, chromedp.NodeVisible, chromedp.BySearch).Do(ctx)
						} else {
							// 使用CSS选择器
							log.Printf("[CHROME-UTIL] 使用CSS选择器截图: %s", config.Selector)
							return chromedp.Screenshot(config.Selector, &buf, chromedp.NodeVisible, chromedp.ByQuery).Do(ctx)
						}
					})
				}(),
			)

			// 清理资源
			cancel()

			if err != nil {
				log.Printf("[CHROME-UTIL] %s失败 (重试 %d/%d): %v", config.Description, retry, maxRetries, err)
				if strings.Contains(err.Error(), "context canceled") && retry < maxRetries {
					continue // 重试
				}
				if retry == maxRetries {
					return err
				}
				continue
			}

			log.Printf("[CHROME-UTIL] %s截图成功，大小: %d bytes", config.Description, len(buf))

			// 保存截图
			if err := writeFile(config.FilePath, buf); err != nil {
				log.Printf("保存%s失败: %v", config.Description, err)
				return err
			}

			return nil // 成功完成
		}

		return fmt.Errorf("重试次数耗尽")
	})

	if err != nil {
		log.Printf("%s失败: %v", config.Description, err)

		// 检查是否是服务熔断器开启的错误
		if err.Error() == "circuit open" {
			return &ScreenshotResponse{
				Success: false,
				Error:   "ITDog测速服务暂不可用",
				Message: "服务过载，请稍后再试",
			}, nil
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
			return &ScreenshotResponse{
				Success: false,
				Error:   "ITDog测速网站无法访问",
				Message: fmt.Sprintf("无法连接到ITDog测速网站: %s", config.Domain),
			}, nil
		}

		// 检查是否是元素未找到的错误
		if strings.Contains(err.Error(), "waiting for selector") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "not visible") {
			// 返回特定的错误信息，指示元素未找到
			return &ScreenshotResponse{
				Success: false,
				Error:   "无法获取测速内容",
				Message: fmt.Sprintf("无法在itdog网站上找到%s元素，域名: %s", config.Description, config.Domain),
			}, nil
		}

		// 其他类型的错误
		return &ScreenshotResponse{
			Success: false,
			Error:   fmt.Sprintf("%s操作失败: %v", config.Description, err),
		}, nil
	}

	// 写入缓存（12小时）
	response := &ScreenshotResponse{Success: true, ImageUrl: config.FileURL}
	setJSONCache(context.Background(), rdb, config.CacheKey, response, 12*time.Hour)

	log.Printf("%s完成: %s", config.Description, config.Domain)
	return response, nil
}

// Screenshot 处理域名截图请求
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
	cacheKey := utils.BuildCacheKey("cache", "screenshot", utils.SanitizeDomain(domain))

	// 检查缓存
	var cachedResp ScreenshotResponse
	if getJSONCache(context.Background(), rdb, cacheKey, &cachedResp) {
		log.Printf("使用缓存的截图: %s", domain)
		cachedResp.FromCache = true
		c.JSON(http.StatusOK, cachedResp)
		return
	}

	// 确保截图目录存在
	if err := ensureDir(screenshotDir); err != nil {
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

	var err error
	err = sb.ScreenshotBreaker.Execute(func() error {
		// 临时绕过Chrome工具，直接使用chromedp进行测试
		log.Printf("[DEBUG] 绕过Chrome工具，直接使用chromedp进行截图")

		// 创建上下文
		tempCtx, tempCancel := chromedp.NewContext(context.Background())
		defer tempCancel()

		// 设置超时
		tempCtx, timeoutCancel := context.WithTimeout(tempCtx, 90*time.Second)
		defer timeoutCancel()

		// 截图数据
		var buf []byte

		// 执行截图
		err := chromedp.Run(tempCtx,
			chromedp.Navigate(fmt.Sprintf("https://%s", domain)),
			chromedp.Sleep(8*time.Second),
			chromedp.CaptureScreenshot(&buf),
		)

		if err != nil {
			log.Printf("[DEBUG] 直接使用chromedp截图失败: %v", err)
			return err
		}

		// 保存截图
		if err := writeFile(filePath, buf); err != nil {
			log.Printf("保存截图失败: %v", err)
			return err
		}

		log.Printf("[DEBUG] 直接使用chromedp截图成功，大小: %d bytes", len(buf))
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
	setJSONCache(context.Background(), rdb, cacheKey, response, screenshotCacheDuration)

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
	cacheKey := utils.BuildCacheKey("cache", "screenshot", "base64", utils.SanitizeDomain(domain))

	// 检查缓存
	var cached ScreenshotResponse
	if getJSONCache(context.Background(), rdb, cacheKey, &cached) {
		log.Printf("使用缓存的Base64截图: %s", domain)
		cached.FromCache = true
		c.JSON(http.StatusOK, cached)
		return
	}

	// 完整URL
	url := fmt.Sprintf("https://%s", domain)

	// 优先使用全局Chrome工具
	chromeUtil := utils.GetGlobalChromeUtil()
	var ctx context.Context
	var cancel context.CancelFunc
	var timeoutCancel context.CancelFunc

	if chromeUtil != nil {
		// 使用全局Chrome工具（已包含超时）
		globalCtx, globalCancel, err := chromeUtil.GetContext(30 * time.Second)
		if err != nil {
			log.Printf("获取全局Chrome上下文失败: %v，回退到chromedp.NewContext", err)
			// 回退到chromedp.NewContext
			ctx, cancel = chromedp.NewContext(context.Background())
			// 为回退方案设置超时
			ctx, timeoutCancel = context.WithTimeout(ctx, 30*time.Second)
			defer timeoutCancel()
		} else {
			ctx = globalCtx
			cancel = globalCancel
		}
	} else {
		// 回退到chromedp.NewContext
		ctx, cancel = chromedp.NewContext(context.Background())
		// 为回退方案设置超时
		ctx, timeoutCancel = context.WithTimeout(ctx, 30*time.Second)
		defer timeoutCancel()
	}

	if cancel != nil {
		defer cancel()
	}

	// 截图数据
	var buf []byte

	// 执行截图
	var err error
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
		Success:     true,
		ImageUrl:    dataURI,
		ImageBase64: base64Data,
		FromCache:   false,
	}

	// 缓存响应
	setJSONCache(context.Background(), rdb, cacheKey, response, screenshotCacheDuration)

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
	cacheKey := utils.BuildCacheKey("cache", "screenshot", "element", req.URL, req.Selector)

	// 检查缓存
	var cachedResp ScreenshotResponse
	if getJSONCache(context.Background(), rdb, cacheKey, &cachedResp) {
		log.Printf("使用缓存的元素截图: %s, 选择器: %s", req.URL, req.Selector)
		cachedResp.FromCache = true
		c.JSON(http.StatusOK, cachedResp)
		return
	}

	// 确保截图目录存在
	if err := ensureDir(screenshotDir); err != nil {
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

	// 优先使用全局Chrome工具
	chromeUtil := utils.GetGlobalChromeUtil()
	var ctx context.Context
	var cancel context.CancelFunc
	var timeoutCancel context.CancelFunc

	if chromeUtil != nil {
		// 使用全局Chrome工具（已包含超时）
		globalCtx, globalCancel, err := chromeUtil.GetContext(30 * time.Second)
		if err != nil {
			log.Printf("获取全局Chrome上下文失败: %v，回退到chromedp.NewContext", err)
			// 回退到chromedp.NewContext
			ctx, cancel = chromedp.NewContext(
				context.Background(),
				chromedp.WithLogf(log.Printf),
			)
			// 为回退方案设置超时
			ctx, timeoutCancel = context.WithTimeout(ctx, 30*time.Second)
			defer timeoutCancel()
		} else {
			ctx = globalCtx
			cancel = globalCancel
		}
	} else {
		// 回退到chromedp.NewContext
		ctx, cancel = chromedp.NewContext(
			context.Background(),
			chromedp.WithLogf(log.Printf),
		)
		// 为回退方案设置超时
		ctx, timeoutCancel = context.WithTimeout(ctx, 30*time.Second)
		defer timeoutCancel()
	}

	if cancel != nil {
		defer cancel()
	}

	// 截图数据
	var buf []byte

	// 设置等待时间
	waitTime := 2 * time.Second
	if req.Wait > 0 && req.Wait <= 10 {
		waitTime = time.Duration(req.Wait) * time.Second
	}

	// 执行截图
	var err error
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
	if err := writeFile(filePath, buf); err != nil {
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
	setJSONCache(context.Background(), rdb, cacheKey, response, screenshotCacheDuration)

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
	cacheKey := utils.BuildCacheKey("cache", "screenshot", "element", "base64", req.URL, req.Selector)

	// 检查缓存
	var cachedResp ScreenshotResponse
	if getJSONCache(context.Background(), rdb, cacheKey, &cachedResp) {
		log.Printf("使用缓存的Base64元素截图: %s, 选择器: %s", req.URL, req.Selector)
		cachedResp.FromCache = true
		c.JSON(http.StatusOK, cachedResp)
		return
	}

	// 设置等待时间
	waitTime := 2 * time.Second
	if req.Wait > 0 && req.Wait <= 10 {
		waitTime = time.Duration(req.Wait) * time.Second
	}

	// 优先使用全局Chrome工具
	chromeUtil := utils.GetGlobalChromeUtil()
	var ctx context.Context
	var cancel context.CancelFunc
	var timeoutCancel context.CancelFunc

	if chromeUtil != nil {
		// 使用全局Chrome工具（已包含超时）
		globalCtx, globalCancel, err := chromeUtil.GetContext(30 * time.Second)
		if err != nil {
			log.Printf("获取全局Chrome上下文失败: %v，回退到chromedp.NewContext", err)
			// 回退到chromedp.NewContext
			ctx, cancel = chromedp.NewContext(context.Background())
			// 为回退方案设置超时
			ctx, timeoutCancel = context.WithTimeout(ctx, 30*time.Second)
			defer timeoutCancel()
		} else {
			ctx = globalCtx
			cancel = globalCancel
		}
	} else {
		// 回退到chromedp.NewContext
		ctx, cancel = chromedp.NewContext(context.Background())
		// 为回退方案设置超时
		ctx, timeoutCancel = context.WithTimeout(ctx, 30*time.Second)
		defer timeoutCancel()
	}

	if cancel != nil {
		defer cancel()
	}

	// 截图数据
	var buf []byte

	// 执行截图
	var err error
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
		Success:     true,
		ImageUrl:    dataURI,
		ImageBase64: base64Data,
		FromCache:   false,
	}

	// 缓存响应
	setJSONCache(context.Background(), rdb, cacheKey, response, screenshotCacheDuration)

	c.JSON(http.StatusOK, response)
}

// ItdogScreenshot ITDog测速地图截图（使用统一Chrome管理器）
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

	// 配置截图参数
	config := ITDogScreenshotConfig{
		Domain:      domain,
		CacheKey:    utils.BuildCacheKey("cache", "itdog", "map", utils.SanitizeDomain(domain)),
		FileName:    fmt.Sprintf("itdog_%s_%d.png", domain, time.Now().Unix()),
		FileURL:     fmt.Sprintf("/static/itdog/itdog_%s_%d.png", domain, time.Now().Unix()),
		FilePath:    filepath.Join(itdogScreenshotDir, fmt.Sprintf("itdog_%s_%d.png", domain, time.Now().Unix())),
		Selector:    "#china_map",
		Description: "itdog测速地图截图",
	}

	// 执行截图
	response, err := performITDogScreenshot(config, rdb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if response.Success == false {
		if strings.Contains(response.Error, "暂不可用") {
			c.JSON(http.StatusServiceUnavailable, response)
		} else {
			c.JSON(http.StatusOK, response)
		}
		return
	}

	// 计算总耗时
	elapsedTime := time.Since(startTime)
	log.Printf("itdog测速地图截图完成: %s, 耗时: %v", domain, elapsedTime)

	c.JSON(http.StatusOK, response)
}

// ItdogTableScreenshot ITDog测速表格截图（使用统一Chrome管理器）
func ItdogTableScreenshot(c *gin.Context, rdb *redis.Client) {
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

	// 配置截图参数
	config := ITDogScreenshotConfig{
		Domain:      domain,
		CacheKey:    utils.BuildCacheKey("cache", "itdog", "table", utils.SanitizeDomain(domain)),
		FileName:    fmt.Sprintf("itdog_table_%s_%d.png", domain, time.Now().Unix()),
		FileURL:     fmt.Sprintf("/static/itdog/itdog_table_%s_%d.png", domain, time.Now().Unix()),
		FilePath:    filepath.Join(itdogScreenshotDir, fmt.Sprintf("itdog_table_%s_%d.png", domain, time.Now().Unix())),
		Selector:    ".card.mb-0[style*='height:550px']",
		Description: "itdog测速表格截图",
	}

	// 执行截图
	response, err := performITDogScreenshot(config, rdb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if response.Success == false {
		if strings.Contains(response.Error, "暂不可用") {
			c.JSON(http.StatusServiceUnavailable, response)
		} else {
			c.JSON(http.StatusOK, response)
		}
		return
	}

	// 计算总耗时
	elapsedTime := time.Since(startTime)
	log.Printf("itdog测速表格截图完成: %s, 耗时: %v", domain, elapsedTime)

	c.JSON(http.StatusOK, response)
}

// ItdogIPScreenshot ITDog IP统计截图（使用统一Chrome管理器）
func ItdogIPScreenshot(c *gin.Context, rdb *redis.Client) {
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

	// 配置截图参数
	config := ITDogScreenshotConfig{
		Domain:      domain,
		CacheKey:    utils.BuildCacheKey("cache", "itdog", "ip", utils.SanitizeDomain(domain)),
		FileName:    fmt.Sprintf("itdog_ip_%s_%d.png", domain, time.Now().Unix()),
		FileURL:     fmt.Sprintf("/static/itdog/itdog_ip_%s_%d.png", domain, time.Now().Unix()),
		FilePath:    filepath.Join(itdogScreenshotDir, fmt.Sprintf("itdog_ip_%s_%d.png", domain, time.Now().Unix())),
		Selector:    `//div[contains(@class, "card") and contains(@class, "mb-0")][.//h5[contains(text(), "域名解析统计")]]`,
		Description: "itdog IP统计截图",
	}

	// 执行截图
	response, err := performITDogScreenshot(config, rdb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if response.Success == false {
		if strings.Contains(response.Error, "暂不可用") {
			c.JSON(http.StatusServiceUnavailable, response)
		} else {
			c.JSON(http.StatusOK, response)
		}
		return
	}

	// 计算总耗时
	elapsedTime := time.Since(startTime)
	log.Printf("itdog IP统计截图完成: %s, 耗时: %v", domain, elapsedTime)

	c.JSON(http.StatusOK, response)
}

// ItdogResolveScreenshot ITDog综合测速截图（使用统一Chrome管理器）
func ItdogResolveScreenshot(c *gin.Context, rdb *redis.Client) {
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

	// 配置截图参数
	config := ITDogScreenshotConfig{
		Domain:      domain,
		CacheKey:    utils.BuildCacheKey("cache", "itdog", "resolve", utils.SanitizeDomain(domain)),
		FileName:    fmt.Sprintf("itdog_resolve_%s_%d.png", domain, time.Now().Unix()),
		FileURL:     fmt.Sprintf("/static/itdog/itdog_resolve_%s_%d.png", domain, time.Now().Unix()),
		FilePath:    filepath.Join(itdogScreenshotDir, fmt.Sprintf("itdog_resolve_%s_%d.png", domain, time.Now().Unix())),
		Selector:    ".dt-responsive.table-responsive",
		Description: "itdog综合测速截图",
	}

	// 执行截图
	response, err := performITDogScreenshot(config, rdb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if response.Success == false {
		if strings.Contains(response.Error, "暂不可用") {
			c.JSON(http.StatusServiceUnavailable, response)
		} else {
			c.JSON(http.StatusOK, response)
		}
		return
	}

	// 计算总耗时
	elapsedTime := time.Since(startTime)
	log.Printf("itdog综合测速截图完成: %s, 耗时: %v", domain, elapsedTime)

	c.JSON(http.StatusOK, response)
}

// ItdogScreenshotBase64 返回Base64编码的ITDog测速地图截图
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

	// 配置截图参数
	config := ITDogScreenshotConfig{
		Domain:      domain,
		CacheKey:    utils.BuildCacheKey("cache", "itdog", "map", "base64", utils.SanitizeDomain(domain)),
		FileName:    fmt.Sprintf("itdog_%s_%d.png", domain, time.Now().Unix()),
		FileURL:     fmt.Sprintf("/static/itdog/itdog_%s_%d.png", domain, time.Now().Unix()),
		FilePath:    filepath.Join(itdogScreenshotDir, fmt.Sprintf("itdog_%s_%d.png", domain, time.Now().Unix())),
		Selector:    "#china_map",
		Description: "itdog测速地图截图(Base64)",
	}

	// 执行截图
	response, err := performITDogScreenshotBase64(config, rdb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if response.Success == false {
		if strings.Contains(response.Error, "暂不可用") {
			c.JSON(http.StatusServiceUnavailable, response)
		} else {
			c.JSON(http.StatusOK, response)
		}
		return
	}

	// 计算总耗时
	elapsedTime := time.Since(startTime)
	log.Printf("itdog测速地图截图(Base64)完成: %s, 耗时: %v", domain, elapsedTime)

	c.JSON(http.StatusOK, response)
}

// ItdogTableScreenshotBase64 返回Base64编码的ITDog测速表格截图
func ItdogTableScreenshotBase64(c *gin.Context, rdb *redis.Client) {
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

	// 配置截图参数
	config := ITDogScreenshotConfig{
		Domain:      domain,
		CacheKey:    utils.BuildCacheKey("cache", "itdog", "table", "base64", utils.SanitizeDomain(domain)),
		FileName:    fmt.Sprintf("itdog_table_%s_%d.png", domain, time.Now().Unix()),
		FileURL:     fmt.Sprintf("/static/itdog/itdog_table_%s_%d.png", domain, time.Now().Unix()),
		FilePath:    filepath.Join(itdogScreenshotDir, fmt.Sprintf("itdog_table_%s_%d.png", domain, time.Now().Unix())),
		Selector:    ".card.mb-0[style*='height:550px']",
		Description: "itdog测速表格截图(Base64)",
	}

	// 执行截图
	response, err := performITDogScreenshotBase64(config, rdb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if response.Success == false {
		if strings.Contains(response.Error, "暂不可用") {
			c.JSON(http.StatusServiceUnavailable, response)
		} else {
			c.JSON(http.StatusOK, response)
		}
		return
	}

	// 计算总耗时
	elapsedTime := time.Since(startTime)
	log.Printf("itdog测速表格截图(Base64)完成: %s, 耗时: %v", domain, elapsedTime)

	c.JSON(http.StatusOK, response)
}

// ItdogIPBase64Screenshot 返回Base64编码的ITDog IP统计截图
func ItdogIPBase64Screenshot(c *gin.Context, rdb *redis.Client) {
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

	// 配置截图参数
	config := ITDogScreenshotConfig{
		Domain:      domain,
		CacheKey:    utils.BuildCacheKey("cache", "itdog", "ip", "base64", utils.SanitizeDomain(domain)),
		FileName:    fmt.Sprintf("itdog_ip_%s_%d.png", domain, time.Now().Unix()),
		FileURL:     fmt.Sprintf("/static/itdog/itdog_ip_%s_%d.png", domain, time.Now().Unix()),
		FilePath:    filepath.Join(itdogScreenshotDir, fmt.Sprintf("itdog_ip_%s_%d.png", domain, time.Now().Unix())),
		Selector:    `//div[contains(@class, "card") and contains(@class, "mb-0")][.//h5[contains(text(), "域名解析统计")]]`,
		Description: "itdog IP统计截图(Base64)",
	}

	// 执行截图
	response, err := performITDogScreenshotBase64(config, rdb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if response.Success == false {
		if strings.Contains(response.Error, "暂不可用") {
			c.JSON(http.StatusServiceUnavailable, response)
		} else {
			c.JSON(http.StatusOK, response)
		}
		return
	}

	// 计算总耗时
	elapsedTime := time.Since(startTime)
	log.Printf("itdog IP统计截图(Base64)完成: %s, 耗时: %v", domain, elapsedTime)

	c.JSON(http.StatusOK, response)
}

// ItdogResolveScreenshotBase64 返回Base64编码的ITDog综合测速截图
func ItdogResolveScreenshotBase64(c *gin.Context, rdb *redis.Client) {
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

	// 配置截图参数
	config := ITDogScreenshotConfig{
		Domain:      domain,
		CacheKey:    utils.BuildCacheKey("cache", "itdog", "resolve", "base64", utils.SanitizeDomain(domain)),
		FileName:    fmt.Sprintf("itdog_resolve_%s_%d.png", domain, time.Now().Unix()),
		FileURL:     fmt.Sprintf("/static/itdog/itdog_resolve_%s_%d.png", domain, time.Now().Unix()),
		FilePath:    filepath.Join(itdogScreenshotDir, fmt.Sprintf("itdog_resolve_%s_%d.png", domain, time.Now().Unix())),
		Selector:    ".dt-responsive.table-responsive",
		Description: "itdog综合测速截图(Base64)",
	}

	// 执行截图
	response, err := performITDogScreenshotBase64(config, rdb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if response.Success == false {
		if strings.Contains(response.Error, "暂不可用") {
			c.JSON(http.StatusServiceUnavailable, response)
		} else {
			c.JSON(http.StatusOK, response)
		}
		return
	}

	// 计算总耗时
	elapsedTime := time.Since(startTime)
	log.Printf("itdog综合测速截图(Base64)完成: %s, 耗时: %v", domain, elapsedTime)

	c.JSON(http.StatusOK, response)
}

// performITDogScreenshotBase64 执行ITDog截图的Base64版本
func performITDogScreenshotBase64(config ITDogScreenshotConfig, rdb *redis.Client) (*ScreenshotResponse, error) {
	// 检查ITDog测速服务熔断器状态
	sb := services.GetServiceBreakers()
	if !sb.ItdogBreaker.AllowRequest() {
		log.Printf("ITDog测速服务熔断器开启，拒绝请求: %s", config.Domain)
		return &ScreenshotResponse{
			Success: false,
			Error:   "ITDog测速服务暂不可用",
			Message: "服务过载，请稍后再试",
		}, nil
	}

	// 缓存
	var cached ScreenshotResponse
	if getJSONCache(context.Background(), rdb, config.CacheKey, &cached) {
		log.Printf("使用缓存的%s: %s", config.Description, config.Domain)
		cached.FromCache = true
		return &cached, nil
	}

	// 目录
	if err := ensureDir(itdogScreenshotDir); err != nil {
		log.Printf("创建ITDog截图目录失败: %v", err)
		return &ScreenshotResponse{
			Success: false,
			Error:   "服务器内部错误",
		}, nil
	}

	// 使用统一的Chrome管理器获取截图
	log.Printf("开始获取%s (统一浏览器): %s", config.Description, config.Domain)

	var buf []byte

	// 执行截图
	err := sb.ItdogBreaker.Execute(func() error {
		// 获取全局Chrome工具
		chromeUtil := utils.GetGlobalChromeUtil()
		if chromeUtil == nil {
			return fmt.Errorf("Chrome工具未初始化")
		}

		// 增加重试机制
		maxRetries := 2
		for retry := 0; retry <= maxRetries; retry++ {
			if retry > 0 {
				log.Printf("[CHROME-UTIL] %s重试第 %d 次", config.Description, retry)
				time.Sleep(time.Duration(retry) * 2 * time.Second) // 递增等待时间
			}

			// 从Chrome工具获取上下文，设置120秒超时（增加超时时间）
			ctx, cancel, chromeErr := chromeUtil.GetContext(120 * time.Second)
			if chromeErr != nil {
				log.Printf("[CHROME-UTIL] 获取Chrome上下文失败 (重试 %d/%d): %v", retry, maxRetries, chromeErr)
				if retry == maxRetries {
					return fmt.Errorf("获取Chrome上下文失败: %v", chromeErr)
				}
				continue
			}

			// 检查上下文初始状态
			select {
			case <-ctx.Done():
				cancel()
				log.Printf("[CHROME-UTIL] 上下文在使用前已被取消 (重试 %d/%d)", retry, maxRetries)
				if retry == maxRetries {
					return fmt.Errorf("上下文在使用前已被取消")
				}
				continue
			default:
			}

			log.Printf("[CHROME-UTIL] 开始执行%s操作，域名: %s (重试 %d/%d)", config.Description, config.Domain, retry, maxRetries)

			// 截图数据
			var buf []byte

			// 执行截图操作
			err := chromedp.Run(ctx,
				// 导航到itdog测速页面
				chromedp.ActionFunc(func(ctx context.Context) error {
					// 检查上下文状态
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤1: 导航到ITDog页面: %s", fmt.Sprintf("https://www.itdog.cn/ping/%s", config.Domain))
					return chromedp.Navigate(fmt.Sprintf("https://www.itdog.cn/ping/%s", config.Domain)).Do(ctx)
				}),

				// 等待页面加载
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤2: 等待页面加载完成")
					return chromedp.Sleep(2 * time.Second).Do(ctx)
				}),

				// 等待"单次测试"按钮出现，增加超时时间
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤3: 等待单次测试按钮出现")
					return chromedp.WaitVisible(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery).Do(ctx)
				}),

				// 点击"单次测试"按钮
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤4: 点击单次测试按钮")
					return chromedp.Click(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery).Do(ctx)
				}),

				// 等待测试开始
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤5: 等待测试开始")
					return chromedp.Sleep(3 * time.Second).Do(ctx)
				}),

				// 使用循环检查进度条，最多等待60秒（增加等待时间）
				func() chromedp.Action {
					return chromedp.ActionFunc(func(ctx context.Context) error {
						var isDone bool
						var attempts int
						maxAttempts := 60 // 增加最大尝试次数

						log.Printf("[CHROME-UTIL] 步骤6: 开始检查%s测试进度", config.Description)

						for attempts < maxAttempts {
							// 检查上下文是否已取消
							select {
							case <-ctx.Done():
								log.Printf("[CHROME-UTIL] %s上下文已取消，停止等待", config.Description)
								return ctx.Err()
							default:
							}

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
								log.Printf("[CHROME-UTIL] %s进度检查出错: %v", config.Description, err)
								return err
							}

							if isDone {
								log.Printf("[CHROME-UTIL] %s测试完成，进度: %d/%d", config.Description, attempts, maxAttempts)
								return nil // 测试完成，退出循环
							}

							// 每5次尝试打印一次进度
							if attempts%5 == 0 && attempts > 0 {
								log.Printf("[CHROME-UTIL] %s等待测试完成，已等待 %d 秒", config.Description, attempts)
							}

							// 等待1秒后再次检查
							select {
							case <-ctx.Done():
								return ctx.Err()
							case <-time.After(1 * time.Second):
								attempts++
							}
						}

						log.Printf("[CHROME-UTIL] %s等待超时，已尝试 %d 次", config.Description, attempts)
						return nil // 达到最大尝试次数，继续执行
					})
				}(),

				// 额外等待一段时间确保页面元素更新
				chromedp.ActionFunc(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					log.Printf("[CHROME-UTIL] 步骤7: 等待页面元素更新")
					return chromedp.Sleep(5 * time.Second).Do(ctx)
				}),

				// 截取指定元素 - 检查是否是XPath选择器
				func() chromedp.Action {
					return chromedp.ActionFunc(func(ctx context.Context) error {
						select {
						case <-ctx.Done():
							return ctx.Err()
						default:
						}
						log.Printf("[CHROME-UTIL] 步骤8: 开始截图")
						if strings.HasPrefix(config.Selector, "//") {
							// 使用XPath选择器
							log.Printf("[CHROME-UTIL] 使用XPath选择器截图: %s", config.Selector)
							return chromedp.Screenshot(config.Selector, &buf, chromedp.NodeVisible, chromedp.BySearch).Do(ctx)
						} else {
							// 使用CSS选择器
							log.Printf("[CHROME-UTIL] 使用CSS选择器截图: %s", config.Selector)
							return chromedp.Screenshot(config.Selector, &buf, chromedp.NodeVisible, chromedp.ByQuery).Do(ctx)
						}
					})
				}(),
			)

			// 清理资源
			cancel()

			if err != nil {
				log.Printf("[CHROME-UTIL] %s失败 (重试 %d/%d): %v", config.Description, retry, maxRetries, err)
				if strings.Contains(err.Error(), "context canceled") && retry < maxRetries {
					continue // 重试
				}
				if retry == maxRetries {
					return err
				}
				continue
			}

			log.Printf("[CHROME-UTIL] %s截图成功，大小: %d bytes", config.Description, len(buf))
			if err := writeFile(config.FilePath, buf); err != nil {
				log.Printf("保存%s失败: %v", config.Description, err)
				return err
			}
			return nil
		}
		return fmt.Errorf("重试次数耗尽")
	})

	if err != nil {
		log.Printf("%s失败: %v", config.Description, err)
		if err.Error() == "circuit open" {
			return &ScreenshotResponse{
				Success: false,
				Error:   "ITDog测速服务暂不可用",
				Message: "服务过载，请稍后再试",
			}, nil
		}
		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") || strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") || strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") || strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") || strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") || strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "TLS handshake timeout") {
			return &ScreenshotResponse{
				Success: false,
				Error:   "ITDog测速网站无法访问",
				Message: fmt.Sprintf("无法连接到ITDog测速网站: %s", config.Domain),
			}, nil
		}
		return &ScreenshotResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 转换为Base64
	base64Data := base64.StdEncoding.EncodeToString(buf)
	dataURI := fmt.Sprintf("data:image/png;base64,%s", base64Data)

	// 构建响应
	response := &ScreenshotResponse{
		Success:     true,
		ImageUrl:    dataURI,
		ImageBase64: base64Data,
		FromCache:   false,
	}

	// 缓存结果 (12小时)
	setJSONCache(context.Background(), rdb, config.CacheKey, response, 12*time.Hour)

	log.Printf("%s完成: %s", config.Description, config.Domain)
	return response, nil
}

// ChromeStatus 检查Chrome工具状态的API
func ChromeStatus(c *gin.Context) {
	chromeUtil := utils.GetGlobalChromeUtil()
	if chromeUtil == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Chrome工具未初始化",
		})
		return
	}

	// 获取详细状态信息
	detailedStats := chromeUtil.GetDetailedStats()

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"chrome_status": detailedStats,
		"message":       "Chrome工具详细状态检查完成",
	})
}

// ChromeDiagnose 执行Chrome诊断的API
func ChromeDiagnose(c *gin.Context) {
	chromeUtil := utils.GetGlobalChromeUtil()
	if chromeUtil == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Chrome工具未初始化",
		})
		return
	}

	// 执行诊断
	diagnosis := chromeUtil.Diagnose()

	// 根据诊断结果返回适当的HTTP状态码
	severity, _ := diagnosis["severity"].(string)
	var httpStatus int
	switch severity {
	case "critical":
		httpStatus = http.StatusInternalServerError
	case "high":
		httpStatus = http.StatusServiceUnavailable
	case "medium":
		httpStatus = http.StatusAccepted
	default:
		httpStatus = http.StatusOK
	}

	c.JSON(httpStatus, gin.H{
		"success":   severity == "none" || severity == "medium",
		"diagnosis": diagnosis,
		"message":   "Chrome诊断完成",
	})
}

// ChromeForceReset 强制重置Chrome的API
func ChromeForceReset(c *gin.Context) {
	chromeUtil := utils.GetGlobalChromeUtil()
	if chromeUtil == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Chrome工具未初始化",
		})
		return
	}

	// 执行强制重置
	err := chromeUtil.ForceReset()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Chrome强制重置失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Chrome强制重置成功",
	})
}
