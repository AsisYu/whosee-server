/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-29 12:15:00
 * @Description: ITDog检查服务
 */
package services

import (
	"context"
	"dmainwhoseek/utils"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

// ITDogChecker 服务健康检查器
type ITDogChecker struct {
	lastCheckTime time.Time
	healthLogger  *utils.HealthLogger
}

// NewITDogChecker 创建新的ITDog检查器
func NewITDogChecker() *ITDogChecker {
	return &ITDogChecker{
		healthLogger: utils.GetHealthLogger(),
	}
}

// GetLastCheckTime 获取最后检查时间
func (ic *ITDogChecker) GetLastCheckTime() time.Time {
	return ic.lastCheckTime
}

// TestITDogHealth 测试ITDog服务健康状态
func (ic *ITDogChecker) TestITDogHealth(domain ...string) map[string]interface{} {
	ic.healthLogger.Println("开始检查ITDog服务健康状态...")
	ic.healthLogger.Println("[ITDog:详细-健康] 开始执行ITDog健康检查")

	result := map[string]interface{}{
		"available":      false,
		"testSuccessful": false,
		"responseTime":   0,
		"testResults":    make([]map[string]interface{}, 0),
	}

	// 确定测试URL
	testURL := "https://www.itdog.cn/ping"

	// 如果提供了域名，构建包含域名的URL
	if len(domain) > 0 && domain[0] != "" {
		targetDomain := domain[0]
		ic.healthLogger.Printf("[ITDog:详细-健康] 使用用户指定的域名: %s", targetDomain)
		// 构建ITDog URL，将目标域名作为ping参数
		testURL = fmt.Sprintf("https://www.itdog.cn/ping/%s", targetDomain)
	} else {
		ic.healthLogger.Printf("[ITDog:详细-健康] 未提供特定域名，将使用默认ITDog测试URL")
	}
	ic.healthLogger.Printf("[ITDog:详细-健康] 将测试目标URL: %s", testURL)

	testResult := map[string]interface{}{
		"url":          testURL,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"success":      false, // 默认失败，成功后更新
		"message":      "",
		"responseTime": 0, // 默认0毫秒
	}

	// 记录开始时间
	startTime := time.Now()
	ic.healthLogger.Printf("[ITDog:详细-健康] 开始启动Chrome浏览器实例")

	// 创建一个新的浏览器上下文
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ic.healthLogger.Printf("[ITDog:详细-健康] Chrome浏览器实例启动完成")

	// 设置超时 - 15秒
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	ic.healthLogger.Printf("[ITDog:详细-健康] 设置超时为15秒，开始访问测试URL: %s", testURL)

	// 使用chromedp检查页面可访问性
	var pageTitle string
	err := chromedp.Run(timeoutCtx,
		// 导航到ITDog网站
		chromedp.Navigate(testURL),

		// 等待页面加载（检查标题）
		chromedp.Title(&pageTitle),
	)

	// 计算响应时间
	responseTime := time.Since(startTime)
	testResult["responseTime"] = responseTime.Milliseconds()
	result["responseTime"] = responseTime.Milliseconds()
	ic.healthLogger.Printf("[ITDog:详细-健康] 浏览器访问完成，耗时: %v", responseTime)

	if err != nil {
		testResult["message"] = "请求失败：" + err.Error()
		testResult["success"] = false
		ic.healthLogger.Printf("ITDog健康检查失败: %v", err)
		ic.healthLogger.Printf("[ITDog:详细-健康] 访问失败或获取标题失败: %v", err)
	} else {
		testResult["success"] = true
		testResult["message"] = "ITDog网站可访问，页面标题: " + pageTitle
		ic.healthLogger.Printf("ITDog健康检查成功: 页面标题 '%s'", pageTitle)
		ic.healthLogger.Printf("[ITDog:详细-健康] 成功访问页面，获取标题: %s", pageTitle)
	}

	// 更新结果状态
	result["available"] = testResult["success"].(bool)
	result["testSuccessful"] = testResult["success"].(bool)
	ic.healthLogger.Printf("[ITDog:详细-健康] 健康状态: %s", map[bool]string{true: "可用", false: "不可用"}[result["available"].(bool)])

	// 添加测试结果
	ic.healthLogger.Printf("[ITDog:详细-健康] 添加测试结果到返回数据")
	testResults := result["testResults"].([]map[string]interface{})
	testResults = append(testResults, testResult)
	result["testResults"] = testResults

	// 更新最后检查时间
	ic.lastCheckTime = time.Now()

	ic.healthLogger.Printf("ITDog服务健康检查结果: %s", map[bool]string{true: "可用", false: "不可用"}[result["available"].(bool)])
	ic.healthLogger.Printf("[ITDog:详细-健康] 测试完成，当前状态: %s，耗时: %v",
		map[bool]string{true: "可用", false: "不可用"}[result["available"].(bool)], responseTime)

	return result
}

// GetITDogStatus 获取ITDog状态（up/down/unknown）
func (ic *ITDogChecker) GetITDogStatus() string {
	result := ic.TestITDogHealth()

	// 根据可用性返回状态
	if available, ok := result["available"].(bool); ok {
		if available {
			return "up"
		} else {
			return "down"
		}
	}

	return "unknown"
}

// CheckAllServers 检查所有ITDog服务器的状态
func (ic *ITDogChecker) CheckAllServers() []interface{} {
	testResult := ic.TestITDogHealth()

	// 创建服务器结果
	serverResult := map[string]interface{}{
		"name": "ITDogService",
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

	// 添加响应时间
	if responseTime, ok := testResult["responseTime"].(int64); ok {
		serverResult["responseTime"] = responseTime
	}

	// 添加测试结果
	if testResults, ok := testResult["testResults"].([]map[string]interface{}); ok {
		serverResult["testResults"] = testResults
	}

	log.Println("ITDog服务健康检查完成")
	return []interface{}{serverResult}
}
