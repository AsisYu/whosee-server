/*
 * @Author: AsisYu
 * @Date: 2025-06-04
 * @Description: Chrome浏览器工具
 */
package utils

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/chromedp"
)

// Chrome浏览器工具全局实例
var (
	globalChromeUtil *ChromeUtil
	chromeUtilOnce   sync.Once
)

// ChromeUtil Chrome浏览器工具
type ChromeUtil struct {
	ctx            context.Context
	cancel         context.CancelFunc
	allocCtx       context.Context
	allocCancel    context.CancelFunc
	mu             sync.RWMutex
	isRunning      bool
	restarts       int
	lastRestart    time.Time
	concurrencySem chan struct{} // 并发控制信号量
	maxConcurrent  int           // 最大并发数
}

// GetGlobalChromeUtil 获取全局Chrome工具实例
func GetGlobalChromeUtil() *ChromeUtil {
	chromeUtilOnce.Do(func() {
		globalChromeUtil = NewChromeUtil()
		if err := globalChromeUtil.Start(); err != nil {
			log.Printf("[CHROME-UTIL] 初始化Chrome工具失败: %v", err)
		}
	})
	return globalChromeUtil
}

// NewChromeUtil 创建新的Chrome工具
func NewChromeUtil() *ChromeUtil {
	maxConcurrent := 3 // 限制最大并发数为3
	return &ChromeUtil{
		isRunning:      false,
		restarts:       0,
		maxConcurrent:  maxConcurrent,
		concurrencySem: make(chan struct{}, maxConcurrent),
	}
}

// Start 启动Chrome实例
func (cu *ChromeUtil) Start() error {
	cu.mu.Lock()
	defer cu.mu.Unlock()

	if cu.isRunning {
		log.Printf("[CHROME-UTIL] Chrome实例已在运行")
		return nil
	}

	log.Printf("[CHROME-UTIL] 正在启动Chrome实例...")

	// 尝试多种启动策略
	strategies := []func() error{
		cu.startWithMinimalOptions,
		cu.startWithStandardOptions,
		cu.startWithFallbackOptions,
	}

	var lastErr error
	for i, strategy := range strategies {
		log.Printf("[CHROME-UTIL] 尝试启动策略 %d/%d", i+1, len(strategies))

		lastErr = strategy()
		if lastErr == nil {
			cu.isRunning = true
			cu.lastRestart = time.Now()
			log.Printf("[CHROME-UTIL] Chrome实例启动成功 (策略%d) | 重启次数: %d | 最大并发: %d", i+1, cu.restarts, cu.maxConcurrent)
			return nil
		}

		log.Printf("[CHROME-UTIL] 策略 %d 失败: %v", i+1, lastErr)
		cu.cleanup()

		// 在尝试下一个策略前等待
		if i < len(strategies)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	return fmt.Errorf("所有Chrome启动策略都失败了，最后错误: %v", lastErr)
}

// startWithMinimalOptions 使用最小配置启动Chrome
func (cu *ChromeUtil) startWithMinimalOptions() error {
	log.Printf("[CHROME-UTIL] 使用最小配置启动Chrome...")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("single-process", true),
		chromedp.Flag("window-size", "1920,1080"),
	)

	return cu.tryStartWithOptions(opts)
}

// startWithStandardOptions 使用标准配置启动Chrome
func (cu *ChromeUtil) startWithStandardOptions() error {
	log.Printf("[CHROME-UTIL] 使用标准配置启动Chrome...")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("mute-audio", true),
	)

	return cu.tryStartWithOptions(opts)
}

// startWithFallbackOptions 使用备用配置启动Chrome
func (cu *ChromeUtil) startWithFallbackOptions() error {
	log.Printf("[CHROME-UTIL] 使用备用配置启动Chrome...")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("single-process", false),   // 尝试多进程模式
		chromedp.Flag("window-size", "1280,720"), // 更小的窗口
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-plugins", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("mute-audio", true),
	)

	return cu.tryStartWithOptions(opts)
}

// tryStartWithOptions 尝试使用给定选项启动Chrome
func (cu *ChromeUtil) tryStartWithOptions(opts []chromedp.ExecAllocatorOption) error {
	// 创建分配器上下文
	cu.allocCtx, cu.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)

	// 创建Chrome上下文
	cu.ctx, cu.cancel = chromedp.NewContext(cu.allocCtx)

	// 设置启动超时
	timeoutCtx, timeoutCancel := context.WithTimeout(cu.ctx, 60*time.Second)
	defer timeoutCancel()

	// 简单的启动测试 - 只尝试创建上下文
	log.Printf("[CHROME-UTIL] 测试Chrome上下文创建...")

	// 创建一个简单的测试任务
	err := chromedp.Run(timeoutCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("[CHROME-UTIL] Chrome上下文创建成功")
			return nil
		}),
	)

	if err != nil {
		log.Printf("[CHROME-UTIL] Chrome上下文测试失败: %v", err)
		return err
	}

	log.Printf("[CHROME-UTIL] Chrome启动测试成功")
	return nil
}

// Stop 停止Chrome实例
func (cu *ChromeUtil) Stop() {
	cu.mu.Lock()
	defer cu.mu.Unlock()

	if !cu.isRunning {
		return
	}

	log.Printf("[CHROME-UTIL] 正在停止Chrome实例...")
	cu.cleanup()
	cu.isRunning = false
	log.Printf("[CHROME-UTIL] Chrome实例已停止")
}

// cleanup 清理资源
func (cu *ChromeUtil) cleanup() {
	if cu.cancel != nil {
		cu.cancel()
		cu.cancel = nil
	}
	if cu.allocCancel != nil {
		cu.allocCancel()
		cu.allocCancel = nil
	}
}

// Restart 重启Chrome实例
func (cu *ChromeUtil) Restart() error {
	log.Printf("[CHROME-UTIL] 正在重启Chrome实例...")

	cu.Stop()

	// 等待完全关闭
	time.Sleep(5 * time.Second) // 增加等待时间，确保完全关闭

	cu.restarts++
	err := cu.Start()

	if err != nil {
		log.Printf("[CHROME-UTIL] Chrome实例重启失败: %v", err)
		return err
	}

	log.Printf("[CHROME-UTIL] Chrome实例重启成功 | 总重启次数: %d", cu.restarts)
	return nil
}

// GetContext 获取Chrome上下文用于截图操作，增加并发控制
func (cu *ChromeUtil) GetContext(timeout time.Duration) (context.Context, context.CancelFunc, error) {
	// 获取并发许可
	select {
	case cu.concurrencySem <- struct{}{}:
		// 获得许可，继续执行w
		log.Printf("[CHROME-UTIL] 获得并发许可，当前并发数: %d/%d", len(cu.concurrencySem), cu.maxConcurrent)
	case <-time.After(10 * time.Second): // 增加等待时间
		return nil, nil, fmt.Errorf("等待并发许可超时，当前并发数: %d/%d", len(cu.concurrencySem), cu.maxConcurrent)
	}

	// 检查Chrome实例状态
	cu.mu.RLock()
	needRestart := !cu.isRunning || cu.ctx == nil
	cu.mu.RUnlock()

	if needRestart {
		log.Printf("[CHROME-UTIL] Chrome实例需要重启")
		// 释放读锁后获取写锁
		cu.mu.Lock()
		// 双重检查，避免并发重启
		if !cu.isRunning || cu.ctx == nil {
			err := cu.Restart()
			if err != nil {
				cu.mu.Unlock()
				// 释放并发许可
				<-cu.concurrencySem
				log.Printf("[CHROME-UTIL] Chrome重启失败: %v", err)
				return nil, nil, fmt.Errorf("Chrome重启失败: %v", err)
			}
		}
		cu.mu.Unlock()
	}

	// 健康检查
	if !cu.IsHealthy() {
		log.Printf("[CHROME-UTIL] Chrome实例健康检查失败，尝试重启")
		cu.mu.Lock()
		err := cu.Restart()
		cu.mu.Unlock()
		if err != nil {
			// 释放并发许可
			<-cu.concurrencySem
			return nil, nil, fmt.Errorf("Chrome健康检查重启失败: %v", err)
		}
	}

	// 获取父上下文
	cu.mu.RLock()
	parentCtx := cu.ctx
	cu.mu.RUnlock()

	if parentCtx == nil {
		// 释放并发许可
		<-cu.concurrencySem
		return nil, nil, fmt.Errorf("Chrome父上下文为空")
	}

	// 创建子上下文 - 使用更robust的方式
	childCtx, cancel := chromedp.NewContext(parentCtx)

	// 验证子上下文是否创建成功
	select {
	case <-childCtx.Done():
		cancel()
		// 释放并发许可
		<-cu.concurrencySem
		return nil, nil, fmt.Errorf("Chrome子上下文创建后立即被取消")
	default:
	}

	// 设置超时
	timeoutCtx, timeoutCancel := context.WithTimeout(childCtx, timeout)

	// 返回组合的取消函数，包括释放并发许可
	combinedCancel := func() {
		log.Printf("[CHROME-UTIL] 开始清理Chrome上下文资源")
		timeoutCancel()
		cancel()
		// 释放并发许可
		select {
		case <-cu.concurrencySem:
			log.Printf("[CHROME-UTIL] 释放并发许可，当前并发数: %d/%d", len(cu.concurrencySem), cu.maxConcurrent)
		default:
			log.Printf("[CHROME-UTIL] 警告: 并发许可已被释放")
		}
	}

	log.Printf("[CHROME-UTIL] Chrome上下文创建成功，超时时间: %v", timeout)
	return timeoutCtx, combinedCancel, nil
}

// IsHealthy 检查Chrome实例健康状态
func (cu *ChromeUtil) IsHealthy() bool {
	cu.mu.RLock()
	defer cu.mu.RUnlock()

	if !cu.isRunning || cu.ctx == nil {
		log.Printf("[CHROME-UTIL] 健康检查失败: Chrome未运行或上下文为空 (running=%v, ctx=%v)", cu.isRunning, cu.ctx != nil)
		return false
	}

	// 创建健康检查上下文 - 增加超时时间
	healthCtx, cancel := context.WithTimeout(cu.ctx, 10*time.Second)
	defer cancel()

	// 执行多层健康检查
	log.Printf("[CHROME-UTIL] 开始执行健康检查...")

	// 第1层：基本上下文检查
	select {
	case <-cu.ctx.Done():
		log.Printf("[CHROME-UTIL] 健康检查失败: 主上下文已取消")
		return false
	default:
	}

	// 第2层：简单JavaScript执行检查
	var result interface{}
	err := chromedp.Run(healthCtx,
		chromedp.Evaluate(`true`, &result),
	)

	if err != nil {
		log.Printf("[CHROME-UTIL] 健康检查失败: JavaScript执行失败: %v", err)
		return false
	}

	log.Printf("[CHROME-UTIL] 健康检查成功: JavaScript执行正常")
	return true
}

// GetStats 获取Chrome工具统计信息
func (cu *ChromeUtil) GetStats() map[string]interface{} {
	cu.mu.RLock()
	defer cu.mu.RUnlock()

	return map[string]interface{}{
		"is_running":         cu.isRunning,
		"restarts":           cu.restarts,
		"last_restart":       cu.lastRestart,
		"uptime":             time.Since(cu.lastRestart).String(),
		"is_healthy":         cu.IsHealthy(),
		"max_concurrent":     cu.maxConcurrent,
		"current_concurrent": len(cu.concurrencySem),
	}
}

// InitGlobalChromeUtil 初始化全局Chrome工具 (兼容现有代码)
func InitGlobalChromeUtil() error {
	chromeUtil := GetGlobalChromeUtil()
	return chromeUtil.Start()
}

// TestConcurrency 测试Chrome工具的并发处理能力（仅用于调试）
func (cu *ChromeUtil) TestConcurrency(testCount int) {
	log.Printf("[CHROME-UTIL-TEST] 开始并发测试，测试数量: %d", testCount)

	var wg sync.WaitGroup
	var successCount int32
	var failureCount int32

	for i := 0; i < testCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// 获取上下文
			ctx, cancel, err := cu.GetContext(30 * time.Second)
			if err != nil {
				log.Printf("[CHROME-UTIL-TEST] 测试 %d 获取上下文失败: %v", index, err)
				atomic.AddInt32(&failureCount, 1)
				return
			}
			defer cancel()

			// 执行简单操作
			var title string
			err = chromedp.Run(ctx,
				chromedp.Navigate("about:blank"),
				chromedp.Title(&title),
			)

			if err != nil {
				log.Printf("[CHROME-UTIL-TEST] 测试 %d 执行失败: %v", index, err)
				atomic.AddInt32(&failureCount, 1)
			} else {
				log.Printf("[CHROME-UTIL-TEST] 测试 %d 成功", index)
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()
	log.Printf("[CHROME-UTIL-TEST] 并发测试完成，成功: %d, 失败: %d", successCount, failureCount)
}

// ForceReset 强制重置Chrome实例（用于解决持续的context canceled问题）
func (cu *ChromeUtil) ForceReset() error {
	cu.mu.Lock()
	defer cu.mu.Unlock()

	log.Printf("[CHROME-UTIL] 执行强制重置...")

	// 强制清理所有资源
	cu.cleanup()
	cu.isRunning = false

	// 等待更长时间确保完全清理
	time.Sleep(5 * time.Second)

	// 重新创建信号量
	cu.concurrencySem = make(chan struct{}, cu.maxConcurrent)

	// 重启
	cu.restarts++
	err := cu.Start()

	if err != nil {
		log.Printf("[CHROME-UTIL] 强制重置失败: %v", err)
		return err
	}

	log.Printf("[CHROME-UTIL] 强制重置成功")
	return nil
}

// StartHealthMonitor 启动Chrome健康监控
func (cu *ChromeUtil) StartHealthMonitor() {
	go func() {
		ticker := time.NewTicker(60 * time.Second) // 增加检查间隔到60秒
		defer ticker.Stop()

		consecutiveFailures := 0

		for {
			select {
			case <-ticker.C:
				log.Printf("[CHROME-UTIL] 执行定期健康检查...")

				if !cu.IsHealthy() {
					consecutiveFailures++
					log.Printf("[CHROME-UTIL] 健康检查失败 %d 次", consecutiveFailures)

					// 放宽重置条件：连续5次失败才重置
					if consecutiveFailures >= 5 {
						log.Printf("[CHROME-UTIL] 连续健康检查失败达到阈值，执行强制重置")
						err := cu.ForceReset()
						if err != nil {
							log.Printf("[CHROME-UTIL] 强制重置失败: %v", err)
							// 重置失败，继续计数
						} else {
							log.Printf("[CHROME-UTIL] 强制重置成功，重置失败计数")
							consecutiveFailures = 0
						}
					} else {
						log.Printf("[CHROME-UTIL] 健康检查失败，但未达到重置阈值 (%d/5)", consecutiveFailures)
					}
				} else {
					if consecutiveFailures > 0 {
						log.Printf("[CHROME-UTIL] 健康检查恢复正常，重置失败计数 (之前失败 %d 次)", consecutiveFailures)
						consecutiveFailures = 0
					} else {
						log.Printf("[CHROME-UTIL] 健康检查正常")
					}
				}
			}
		}
	}()

	log.Printf("[CHROME-UTIL] Chrome健康监控已启动 (检查间隔: 60秒, 重置阈值: 5次)")
}

// Diagnose 诊断Chrome实例状态
func (cu *ChromeUtil) Diagnose() map[string]interface{} {
	cu.mu.RLock()
	defer cu.mu.RUnlock()

	diagnosis := map[string]interface{}{
		"timestamp":            time.Now().Format(time.RFC3339),
		"is_running":           cu.isRunning,
		"context_exists":       cu.ctx != nil,
		"alloc_context_exists": cu.allocCtx != nil,
		"restarts":             cu.restarts,
		"last_restart":         cu.lastRestart.Format(time.RFC3339),
		"uptime_seconds":       time.Since(cu.lastRestart).Seconds(),
		"max_concurrent":       cu.maxConcurrent,
		"current_concurrent":   len(cu.concurrencySem),
		"available_permits":    cu.maxConcurrent - len(cu.concurrencySem),
	}

	// 基础状态检查
	if !cu.isRunning {
		diagnosis["issue"] = "Chrome实例未运行"
		diagnosis["severity"] = "critical"
		return diagnosis
	}

	if cu.ctx == nil {
		diagnosis["issue"] = "Chrome上下文为空"
		diagnosis["severity"] = "critical"
		return diagnosis
	}

	// 上下文状态检查
	select {
	case <-cu.ctx.Done():
		diagnosis["issue"] = "Chrome主上下文已取消"
		diagnosis["context_error"] = cu.ctx.Err().Error()
		diagnosis["severity"] = "critical"
		return diagnosis
	default:
		diagnosis["context_status"] = "active"
	}

	// 简单健康检查
	healthCtx, cancel := context.WithTimeout(cu.ctx, 5*time.Second)
	defer cancel()

	var result interface{}
	err := chromedp.Run(healthCtx,
		chromedp.Evaluate(`navigator.userAgent`, &result),
	)

	if err != nil {
		diagnosis["issue"] = "JavaScript执行失败"
		diagnosis["js_error"] = err.Error()
		diagnosis["severity"] = "high"
	} else {
		diagnosis["js_execution"] = "success"
		diagnosis["user_agent"] = result
		diagnosis["severity"] = "none"
	}

	// 并发状态评估
	if len(cu.concurrencySem) >= cu.maxConcurrent {
		diagnosis["concurrency_warning"] = "所有并发许可已用完"
		if diagnosis["severity"] == "none" {
			diagnosis["severity"] = "medium"
		}
	}

	return diagnosis
}

// GetDetailedStats 获取详细的Chrome工具统计信息
func (cu *ChromeUtil) GetDetailedStats() map[string]interface{} {
	stats := cu.GetStats()
	diagnosis := cu.Diagnose()

	return map[string]interface{}{
		"basic_stats":     stats,
		"diagnosis":       diagnosis,
		"recommendations": cu.getRecommendations(diagnosis),
	}
}

// getRecommendations 根据诊断结果提供建议
func (cu *ChromeUtil) getRecommendations(diagnosis map[string]interface{}) []string {
	var recommendations []string

	severity, _ := diagnosis["severity"].(string)

	switch severity {
	case "critical":
		recommendations = append(recommendations, "立即重启Chrome实例")
		recommendations = append(recommendations, "检查系统资源使用情况")
	case "high":
		recommendations = append(recommendations, "考虑重启Chrome实例")
		recommendations = append(recommendations, "检查JavaScript执行环境")
	case "medium":
		recommendations = append(recommendations, "监控并发使用情况")
		recommendations = append(recommendations, "考虑增加并发限制")
	default:
		recommendations = append(recommendations, "Chrome运行正常")
	}

	uptime, ok := diagnosis["uptime_seconds"].(float64)
	if ok && uptime > 3600 { // 运行超过1小时
		recommendations = append(recommendations, "Chrome已运行较长时间，建议定期重启")
	}

	return recommendations
}
