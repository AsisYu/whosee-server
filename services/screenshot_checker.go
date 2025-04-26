package services

import (
	"log"
	"net/http"
	"time"
)

// ScreenshotChecker 实现网站截图服务的健康检查
type ScreenshotChecker struct {
	lastCheckTime time.Time
	testURLs      []string
}

// NewScreenshotChecker 创建新的截图服务检查器
func NewScreenshotChecker() *ScreenshotChecker {
	return &ScreenshotChecker{
		testURLs: []string{
			"https://www.baidu.com",  // 使用中国服务器更容易访问的网站作为主要测试
			"https://www.bing.com",   // 备用测试网站
			"https://www.github.com", // 备用测试网站
		},
	}
}

// GetLastCheckTime 返回上次检查时间
func (sc *ScreenshotChecker) GetLastCheckTime() time.Time {
	return sc.lastCheckTime
}

// TestScreenshotHealth 测试截图服务的健康状态
func (sc *ScreenshotChecker) TestScreenshotHealth() map[string]interface{} {
	log.Println("开始测试截图服务健康状态...")
	
	result := map[string]interface{}{
		"available":      false,
		"testSuccessful": false,
		"responseTime":   0,
		"testResults":    make([]map[string]interface{}, 0),
	}
	
	// 记录成功的测试数
	successfulTests := 0
	// 测试URL总数，目前未使用，但保留供未来使用
	// totalTests := len(sc.testURLs)
	
	// 测试每个URL
	for _, url := range sc.testURLs {
		testResult := map[string]interface{}{
			"url":          url,
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
			"success":      false,
			"message":      "",
			"responseTime": 0,
		}
		
		// 检查网站是否可访问（我们不实际生成截图，只检查URL是否可达）
		startTime := time.Now()
		
		client := &http.Client{
			Timeout: 15 * time.Second, // 增加到15秒超时
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // 不跟随重定向
			},
		}
		
		req, err := http.NewRequest("HEAD", url, nil)
		if err != nil {
			testResult["message"] = "创建请求失败: " + err.Error()
		} else {
			req.Header.Set("User-Agent", "WhoseeMeHealthCheck/1.0")
			
			resp, err := client.Do(req)
			responseTime := time.Since(startTime)
			testResult["responseTime"] = responseTime.Milliseconds()
			
			if err != nil {
				testResult["message"] = "请求失败: " + err.Error()
			} else {
				// 2xx或3xx状态码都认为是成功
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					testResult["success"] = true
					testResult["message"] = "网站可访问"
					testResult["statusCode"] = resp.StatusCode
					successfulTests++
				} else {
					testResult["message"] = "网站返回错误状态码: " + resp.Status
					testResult["statusCode"] = resp.StatusCode
				}
				resp.Body.Close()
			}
		}
		
		// 添加测试结果
		testResults := result["testResults"].([]map[string]interface{})
		testResults = append(testResults, testResult)
		result["testResults"] = testResults
	}
	
	// 只要有一个测试成功，就认为服务可用
	serviceAvailable := successfulTests > 0
	
	// 更新结果
	result["available"] = serviceAvailable
	result["testSuccessful"] = serviceAvailable
	
	// 更新最后检查时间
	sc.lastCheckTime = time.Now()
	
	log.Printf("截图服务检查完成: 服务%s", map[bool]string{true: "可用", false: "不可用"}[serviceAvailable])
	
	return result
}

// GetScreenshotStatus 获取截图服务的状态
func (sc *ScreenshotChecker) GetScreenshotStatus() string {
	result := sc.TestScreenshotHealth()
	
	// 判断服务可用性
	if available, ok := result["available"].(bool); ok {
		if available {
			return "up"
		} else {
			return "down"
		}
	}
	
	return "unknown"
}

// CheckAllServers 检查截图服务并返回结果
func (sc *ScreenshotChecker) CheckAllServers() []interface{} {
	testResult := sc.TestScreenshotHealth()
	
	// 创建一个单一服务条目
	serverResult := map[string]interface{}{
		"name": "ScreenshotService",
	}
	
	// 设置状态
	if available, ok := testResult["available"].(bool); ok {
		if available {
			serverResult["status"] = "up"
		} else {
			serverResult["status"] = "down"
		}
	} else {
		serverResult["status"] = "unknown"
	}
	
	// 复制响应时间
	if responseTime, ok := testResult["responseTime"].(int64); ok {
		serverResult["responseTime"] = responseTime
	}
	
	// 复制测试结果
	if testResults, ok := testResult["testResults"].([]map[string]interface{}); ok {
		serverResult["testResults"] = testResults
	}
	
	log.Println("截图服务检查完成")
	return []interface{}{serverResult}
}
