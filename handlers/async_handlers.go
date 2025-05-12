/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: 对应服务容器组件的异步处理函数
 */
package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"dmainwhoseek/services"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// 从上下文运行环境中获取Redis客户端
func getRedisFromContext(ctx context.Context) (*redis.Client, error) {
	// 分离通用函数以方便合并不同的异步处理程序
	value := ctx.Value("redis")
	if value == nil {
		return nil, fmt.Errorf("redis client not found in context")
	}
	
	redisClient, ok := value.(*redis.Client)
	if !ok || redisClient == nil {
		return nil, fmt.Errorf("invalid redis client in context")
	}
	
	return redisClient, nil
}

// 从上下文获取域名
func getDomainFromContext(ctx context.Context) (string, error) {
	value := ctx.Value("domain")
	if value == nil {
		return "", fmt.Errorf("domain not found in context")
	}
	
	domain, ok := value.(string)
	if !ok || domain == "" {
		return "", fmt.Errorf("invalid domain in context")
	}
	
	return domain, nil
}

// AsyncDNSQuery DNS查询异步处理
func AsyncDNSQuery(ctx context.Context, dnsChecker *services.DNSChecker) (gin.H, error) {
	// 从上下文获取必要的参数
	domain, err := getDomainFromContext(ctx)
	if err != nil {
		log.Printf("DNS查询失败: %v", err)
		return nil, err
	}

	// 获取Redis客户端以进行缓存操作
	rdb, err := getRedisFromContext(ctx)
	if err != nil {
		log.Printf("DNS查询无法获取Redis客户端: %v", err)
		// 如果无法获取Redis客户端，则直接进行查询而不使用缓存
		log.Printf("开始不缓存的DNS查询: %s", domain)
		result := dnsChecker.TestDNSHealth()
		result["isCached"] = false
		result["cacheTime"] = time.Now().Format(time.RFC3339)
		return result, nil
	}

	// 检查缓存
	cacheKey := fmt.Sprintf("dns:%s", domain)
	if cachedData, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Printf("从缓存获取DNS记录: %s", domain)
		
		var result gin.H
		if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
			// 添加缓存元数据
			result["isCached"] = true
			result["cacheTime"] = time.Now().Format(time.RFC3339)
			return result, nil
		}
	}

	// 执行DNS查询
	log.Printf("开始新的DNS查询: %s", domain)
	result := dnsChecker.TestDNSHealth()
	result["isCached"] = false
	result["cacheTime"] = time.Now().Format(time.RFC3339)
	
	// 保存到缓存
	data, _ := json.Marshal(result)
	rdb.Set(ctx, cacheKey, data, 30*time.Minute) // DNS记录缓存30分钟
	
	return result, nil
}

// AsyncScreenshot 异步处理网站截图
func AsyncScreenshot(ctx context.Context, screenshotChecker *services.ScreenshotChecker) (gin.H, error) {
	// 从上下文获取必要的参数
	log.Printf("[Screenshot:详细] 开始从上下文提取域名信息")
	domain, err := getDomainFromContext(ctx)
	if err != nil {
		log.Printf("截图失败: %v", err)
		return nil, err
	}
	log.Printf("[Screenshot:详细] 成功获取域名: %s", domain)

	// 获取Redis客户端以进行缓存操作
	log.Printf("[Screenshot:详细] 尝试从上下文获取Redis客户端")
	rdb, err := getRedisFromContext(ctx)
	if err != nil {
		log.Printf("截图无法获取Redis客户端: %v", err)
		// 如果无法获取Redis客户端，则直接进行截图而不使用缓存
		log.Printf("开始不缓存的截图: %s", domain)
		log.Printf("[Screenshot:详细] 将直接执行截图操作（无缓存模式）")
		
		// 模拟gin的ResponseWriter以获取截图结果
		mockResponseWriter := &mockResponseWriter{headers: make(http.Header)}
		mockGinContext := &gin.Context{
			Writer: mockResponseWriter,
			Params: []gin.Param{{
				Key:   "domain",
				Value: domain,
			}},
		}
		
		// 直接调用截图函数（无缓存模式）
		Screenshot(mockGinContext, nil)
		
		// 获取截图结果
		response := mockResponseWriter.GetResponseData()
		if response != nil {
			// 尝试获取文件路径
			filePath := ""
			if imageURL, ok := response["imageUrl"].(string); ok {
				// 从imageUrl提取文件名
				fileName := filepath.Base(imageURL)
				filePath = filepath.Join("./static/screenshots", fileName)
			}
			
			return gin.H{
				"success":    response["success"],
				"imageUrl":   response["imageUrl"],
				"error":      response["error"],
				"message":    response["message"],
				"fromCache":  false,
				"filepath":   filePath,
				"statusCode": mockResponseWriter.statusCode,
			}, nil
		} else {
			return gin.H{
				"success":    false,
				"error":      "截图失败",
				"message":    "无法获取截图结果",
				"fromCache":  false,
				"statusCode": http.StatusInternalServerError,
			}, fmt.Errorf("无法获取截图结果")
		}
	}
	log.Printf("[Screenshot:详细] 成功获取Redis客户端")

	// 检查缓存
	cacheKey := fmt.Sprintf("screenshot:%s", domain)
	log.Printf("[Screenshot:详细] 开始检查截图缓存，键名: %s", cacheKey)
	if cachedData, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Printf("从缓存获取截图: %s", domain)
		log.Printf("[Screenshot:详细] 找到缓存数据，大小: %d bytes", len(cachedData))

		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("[Screenshot:详细] 成功解析缓存的JSON数据")
			
			// 提取文件路径
			filePath := ""
			if response.ImageUrl != "" {
				// 从imageUrl提取文件名
				fileName := filepath.Base(response.ImageUrl)
				filePath = filepath.Join("./static/screenshots", fileName)
			}
			
			return gin.H{
				"success":    response.Success,
				"imageUrl":   response.ImageUrl,
				"error":      response.Error,
				"message":    response.Message,
				"fromCache":  true,
				"filepath":   filePath,
				"statusCode": http.StatusOK,
			}, nil
		} else {
			log.Printf("[Screenshot:详细] 无法解析缓存的JSON数据: %v", err)
		}
	} else {
		log.Printf("[Screenshot:详细] 缓存中没有找到数据: %v", err)
	}

	// 模拟gin的ResponseWriter以获取截图结果
	mockResponseWriter := &mockResponseWriter{headers: make(http.Header)}
	mockGinContext := &gin.Context{
		Writer: mockResponseWriter,
		Params: []gin.Param{{
			Key:   "domain",
			Value: domain,
		}},
	}

	// 执行截图
	log.Printf("开始新的截图: %s", domain)
	log.Printf("[Screenshot:详细] 将执行新的截图操作")
	Screenshot(mockGinContext, rdb)  // 传递Redis客户端以启用缓存

	// 获取截图结果
	response := mockResponseWriter.GetResponseData()
	if response != nil {
		// 提取文件路径
		filePath := ""
		if imageURL, ok := response["imageUrl"].(string); ok {
			// 从imageUrl提取文件名
			fileName := filepath.Base(imageURL)
			filePath = filepath.Join("./static/screenshots", fileName)
		}
		
		return gin.H{
			"success":    response["success"],
			"imageUrl":   response["imageUrl"],
			"error":      response["error"],
			"message":    response["message"],
			"fromCache":  false,
			"filepath":   filePath,
			"statusCode": mockResponseWriter.statusCode,
		}, nil
	} else {
		return gin.H{
			"success":    false,
			"error":      "截图失败",
			"message":    "无法获取截图结果",
			"fromCache":  false,
			"statusCode": http.StatusInternalServerError,
		}, fmt.Errorf("无法获取截图结果")
	}
}

// AsyncItdogScreenshot 异步处理ITDog测速截图
func AsyncItdogScreenshot(ctx context.Context, itdogChecker *services.ITDogChecker) (gin.H, error) {
	// 从上下文获取必要的参数
	log.Printf("[ITDog:详细] 开始从上下文提取域名信息")
	domain, err := getDomainFromContext(ctx)
	if err != nil {
		log.Printf("ITDog截图失败: %v", err)
		return nil, err
	}
	log.Printf("[ITDog:详细] 成功获取域名: %s", domain)

	// 获取Redis客户端以进行缓存操作
	log.Printf("[ITDog:详细] 尝试从上下文获取Redis客户端")
	rdb, err := getRedisFromContext(ctx)
	if err != nil {
		log.Printf("ITDog截图无法获取Redis客户端: %v", err)
		// 如果无法获取Redis客户端，则直接进行截图而不使用缓存
		log.Printf("开始不缓存的ITDog截图: %s", domain)
		log.Printf("[ITDog:详细] 将直接执行测速截图操作（无缓存模式）")
		
		// 模拟gin的ResponseWriter以获取截图结果
		mockResponseWriter := &mockResponseWriter{headers: make(http.Header)}
		mockGinContext := &gin.Context{
			Writer: mockResponseWriter,
			Params: []gin.Param{{
				Key:   "domain",
				Value: domain,
			}},
		}
		
		// 直接调用截图函数（无缓存模式）
		ItdogScreenshot(mockGinContext, nil)
		
		// 获取截图结果
		response := mockResponseWriter.GetResponseData()
		if response != nil {
			// 尝试获取文件路径
			filePath := ""
			if imageURL, ok := response["imageUrl"].(string); ok {
				// 从imageUrl提取文件名
				fileName := filepath.Base(imageURL)
				filePath = filepath.Join("./static/itdog", fileName)
			}
			
			return gin.H{
				"success":    response["success"],
				"imageUrl":   response["imageUrl"],
				"error":      response["error"],
				"message":    response["message"],
				"fromCache":  false,
				"filepath":   filePath,
				"statusCode": mockResponseWriter.statusCode,
			}, nil
		} else {
			return gin.H{
				"success":    false,
				"error":      "ITDog截图失败",
				"message":    "无法获取截图结果",
				"fromCache":  false,
				"statusCode": http.StatusInternalServerError,
			}, fmt.Errorf("无法获取截图结果")
		}
	}
	log.Printf("[ITDog:详细] 成功获取Redis客户端")

	// 检查缓存
	cacheKey := fmt.Sprintf("itdog_screenshot:%s", domain)
	log.Printf("[ITDog:详细] 开始检查测速截图缓存，键名: %s", cacheKey)
	if cachedData, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Printf("从缓存获取ITDog截图: %s", domain)
		log.Printf("[ITDog:详细] 找到缓存数据，大小: %d bytes", len(cachedData))

		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("[ITDog:详细] 成功解析缓存的JSON数据")
			
			// 提取文件路径
			filePath := ""
			if response.ImageUrl != "" {
				// 从imageUrl提取文件名
				fileName := filepath.Base(response.ImageUrl)
				filePath = filepath.Join("./static/itdog", fileName)
			}
			
			return gin.H{
				"success":    response.Success,
				"imageUrl":   response.ImageUrl,
				"error":      response.Error,
				"message":    response.Message,
				"fromCache":  true,
				"filepath":   filePath,
				"statusCode": http.StatusOK,
			}, nil
		} else {
			log.Printf("[ITDog:详细] 无法解析缓存的JSON数据: %v", err)
		}
	} else {
		log.Printf("[ITDog:详细] 缓存中没有找到数据: %v", err)
	}

	// 模拟gin的ResponseWriter以获取截图结果
	mockResponseWriter := &mockResponseWriter{headers: make(http.Header)}
	mockGinContext := &gin.Context{
		Writer: mockResponseWriter,
		Params: []gin.Param{{
			Key:   "domain",
			Value: domain,
		}},
	}

	// 执行截图
	log.Printf("开始新的ITDog截图: %s", domain)
	log.Printf("[ITDog:详细] 将执行新的测速截图操作")
	ItdogScreenshot(mockGinContext, rdb)  // 传递Redis客户端以启用缓存

	// 获取截图结果
	response := mockResponseWriter.GetResponseData()
	if response != nil {
		// 提取文件路径
		filePath := ""
		if imageURL, ok := response["imageUrl"].(string); ok {
			// 从imageUrl提取文件名
			fileName := filepath.Base(imageURL)
			filePath = filepath.Join("./static/itdog", fileName)
		}
		
		return gin.H{
			"success":    response["success"],
			"imageUrl":   response["imageUrl"],
			"error":      response["error"],
			"message":    response["message"],
			"fromCache":  false,
			"filepath":   filePath,
			"statusCode": mockResponseWriter.statusCode,
		}, nil
	} else {
		return gin.H{
			"success":    false,
			"error":      "ITDog截图失败",
			"message":    "无法获取截图结果",
			"fromCache":  false,
			"statusCode": http.StatusInternalServerError,
		}, fmt.Errorf("无法获取截图结果")
	}
}

// mockResponseWriter 模拟gin的ResponseWriter以获取截图结果
type mockResponseWriter struct {
	statusCode int
	data       []byte
	headers    http.Header
}

func (w *mockResponseWriter) Header() http.Header {
	return w.headers
}

func (w *mockResponseWriter) Write(data []byte) (int, error) {
	w.data = data
	return len(data), nil
}

func (w *mockResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *mockResponseWriter) WriteHeaderNow() {}

func (w *mockResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *mockResponseWriter) Status() int {
	return w.statusCode
}

func (w *mockResponseWriter) Size() int {
	return len(w.data)
}

func (w *mockResponseWriter) Written() bool {
	return w.data != nil
}

func (w *mockResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

func (w *mockResponseWriter) CloseNotify() <-chan bool {
	return nil
}

func (w *mockResponseWriter) Flush() {}

func (w *mockResponseWriter) Pusher() http.Pusher {
	return nil
}

// GetResponseData 解析ResponseWriter中的数据为JSON
func (w *mockResponseWriter) GetResponseData() map[string]interface{} {
	if len(w.data) == 0 {
		return nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal(w.data, &data); err != nil {
		return nil
	}
	return data
}

// 估算JSON数据大小的辅助函数
func estimateJsonSize(data interface{}) int {
	if data == nil {
		return 0
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	return len(jsonData)
}
