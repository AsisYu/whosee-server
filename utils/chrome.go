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
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/chromedp"
)

// Chrome浏览器工具全局实例
var (
	globalChromeUtil *ChromeUtil
	chromeUtilOnce   sync.Once
	chromeDownloader *ChromeDownloader
)

// ChromeMode Chrome运行模式
type ChromeMode int

const (
	ChromeModeCold ChromeMode = iota // 冷启动模式（按需启动）
	ChromeModeWarm                   // 预热模式（预先启动）
	ChromeModeAuto                   // 自动模式（智能选择）
)

// ChromeConfig Chrome配置
type ChromeConfig struct {
	Mode                ChromeMode    // 运行模式
	IdleTimeout         time.Duration // 空闲超时时间
	HealthCheckInterval time.Duration // 健康检查间隔
	EnableHealthCheck   bool          // 是否启用健康检查
	PrewarmOnStart      bool          // 启动时是否预热
}

// DefaultChromeConfig 默认Chrome配置（智能混合模式）
var DefaultChromeConfig = ChromeConfig{
	Mode:                ChromeModeAuto,  // 智能混合模式
	IdleTimeout:         3 * time.Minute, // 3分钟空闲后自动关闭
	HealthCheckInterval: 0,               // 不启用定期健康检查
	EnableHealthCheck:   false,           // 关闭定期健康检查
	PrewarmOnStart:      false,           // 不预热，按需启动
}

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
	lastUsed       time.Time     // 最后使用时间
	concurrencySem chan struct{} // 并发控制信号量
	maxConcurrent  int           // 最大并发数
	chromeExecPath string        // Chrome可执行文件路径
	config         ChromeConfig  // Chrome配置
	idleTimer      *time.Timer   // 空闲计时器
	statsLock      sync.RWMutex  // 统计信息锁
	usageCount     int64         // 使用次数
	totalUptime    time.Duration // 总运行时间
}

// GetGlobalChromeUtil 获取全局Chrome工具实例
func GetGlobalChromeUtil() *ChromeUtil {
	chromeUtilOnce.Do(func() {
		// 初始化Chrome下载器
		chromeDownloader = NewChromeDownloader()

		globalChromeUtil = NewChromeUtilWithConfig(DefaultChromeConfig)

		// 根据配置决定是否预热
		if globalChromeUtil.config.PrewarmOnStart {
			log.Printf("[CHROME-UTIL] 预热模式启动Chrome...")
			if err := globalChromeUtil.Start(); err != nil {
				log.Printf("[CHROME-UTIL] Chrome预热失败: %v", err)
			}
		}
	})
	return globalChromeUtil
}

// GetGlobalChromeUtilWithConfig 使用自定义配置获取Chrome工具实例
func GetGlobalChromeUtilWithConfig(config ChromeConfig) *ChromeUtil {
	chromeUtilOnce.Do(func() {
		chromeDownloader = NewChromeDownloader()
		globalChromeUtil = NewChromeUtilWithConfig(config)

		if globalChromeUtil.config.PrewarmOnStart {
			log.Printf("[CHROME-UTIL] 预热模式启动Chrome...")
			if err := globalChromeUtil.Start(); err != nil {
				log.Printf("[CHROME-UTIL] Chrome预热失败: %v", err)
			}
		}
	})
	return globalChromeUtil
}

// NewChromeUtil 创建新的Chrome工具（使用默认配置）
func NewChromeUtil() *ChromeUtil {
	return NewChromeUtilWithConfig(DefaultChromeConfig)
}

// NewChromeUtilWithConfig 创建新的Chrome工具（使用自定义配置）
func NewChromeUtilWithConfig(config ChromeConfig) *ChromeUtil {
	maxConcurrent := 3 // 限制最大并发数为3

	util := &ChromeUtil{
		isRunning:      false,
		restarts:       0,
		maxConcurrent:  maxConcurrent,
		concurrencySem: make(chan struct{}, maxConcurrent),
		config:         config,
		lastUsed:       time.Now(),
		usageCount:     0,
		totalUptime:    0,
	}

	log.Printf("[CHROME-UTIL] 创建Chrome工具实例，模式: %s", util.getModeString())

	return util
}

// getModeString 获取模式字符串
func (cu *ChromeUtil) getModeString() string {
	switch cu.config.Mode {
	case ChromeModeCold:
		return "冷启动"
	case ChromeModeWarm:
		return "预热模式"
	case ChromeModeAuto:
		return "自动模式"
	default:
		return "未知模式"
	}
}

// diagnoseChromeEnvironment 诊断Chrome环境（简化版）
func (cu *ChromeUtil) diagnoseChromeEnvironment() {
	log.Printf("[CHROME-UTIL] === Chrome环境检查 ===")

	// 检查Chrome可执行文件
	chromeDownloader := NewChromeDownloader()
	execPath := chromeDownloader.GetChromeExecutablePath()

	if execPath == "" {
		log.Printf("[CHROME-UTIL] 警告: 未找到Chrome可执行文件")
	} else {
		log.Printf("[CHROME-UTIL] 找到Chrome: %s", execPath)

		// 检查文件是否存在
		if _, err := os.Stat(execPath); err != nil {
			log.Printf("[CHROME-UTIL] 警告: Chrome文件不可访问: %v", err)
		}
	}

	log.Printf("[CHROME-UTIL] === Chrome环境检查完成 ===")
}

// Start 启动Chrome实例，使用多重启动策略
func (cu *ChromeUtil) Start() error {
	log.Printf("[CHROME-UTIL] 正在启动Chrome实例...")

	// 诊断Chrome环境
	cu.diagnoseChromeEnvironment()

	// 获取Chrome可执行文件路径
	chromeDownloader := NewChromeDownloader()
	execPath := chromeDownloader.GetChromeExecutablePath()

	// 保存Chrome可执行文件路径用于后续诊断
	cu.chromeExecPath = execPath

	// 多重启动策略
	strategies := []struct {
		name string
		fn   func() error
	}{
		{"最小配置", cu.startWithMinimalOptions},
		{"标准配置", cu.startWithStandardOptions},
		{"备用配置", cu.startWithFallbackOptions},
		{"紧急配置", cu.startWithEmergencyOptions},
	}

	var lastErr error
	for i, strategy := range strategies {
		log.Printf("[CHROME-UTIL] 尝试启动策略 %d/%d", i+1, len(strategies))
		err := strategy.fn()
		if err == nil {
			log.Printf("[CHROME-UTIL] Chrome实例启动成功 (策略%d) | 重启次数: %d | 最大并发: %d", i+1, cu.restarts, cu.maxConcurrent)
			cu.lastRestart = time.Now()
			return nil
		}
		lastErr = err
		log.Printf("[CHROME-UTIL] 策略%d失败: %v", i+1, err)

		if i < len(strategies)-1 {
			log.Printf("[CHROME-UTIL] 等待3秒后尝试下一个策略...")
			time.Sleep(3 * time.Second)
		}
	}

	return fmt.Errorf("所有启动策略都失败了，最后错误: %v", lastErr)
}

// startWithMinimalOptions 使用最小配置启动Chrome
func (cu *ChromeUtil) startWithMinimalOptions() error {
	cu.mu.Lock()
	defer cu.mu.Unlock()

	if cu.isRunning {
		log.Printf("[CHROME-UTIL] Chrome实例已在运行")
		return nil
	}

	log.Printf("[CHROME-UTIL] 使用最小配置启动Chrome...")

	// 最小配置选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	log.Printf("[CHROME-UTIL] 最小配置标志数量: %d", len(opts))

	err := cu.tryStartWithOptions(opts)
	if err == nil {
		cu.isRunning = true
	}
	return err
}

// startWithStandardOptions 使用标准配置启动Chrome
func (cu *ChromeUtil) startWithStandardOptions() error {
	cu.mu.Lock()
	defer cu.mu.Unlock()

	if cu.isRunning {
		return nil
	}

	log.Printf("[CHROME-UTIL] 使用标准配置启动Chrome...")

	// 标准配置选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.WindowSize(1920, 1080),
	)

	err := cu.tryStartWithOptions(opts)
	if err == nil {
		cu.isRunning = true
	}
	return err
}

// startWithFallbackOptions 使用备用配置启动Chrome
func (cu *ChromeUtil) startWithFallbackOptions() error {
	cu.mu.Lock()
	defer cu.mu.Unlock()

	if cu.isRunning {
		return nil
	}

	log.Printf("[CHROME-UTIL] 使用备用配置启动Chrome...")

	// 备用配置选项（更保守）
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("single-process", true), // 单进程模式
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-plugins", true),
	)

	err := cu.tryStartWithOptions(opts)
	if err == nil {
		cu.isRunning = true
	}
	return err
}

// startWithEmergencyOptions 使用紧急配置启动Chrome
func (cu *ChromeUtil) startWithEmergencyOptions() error {
	cu.mu.Lock()
	defer cu.mu.Unlock()

	if cu.isRunning {
		return nil
	}

	log.Printf("[CHROME-UTIL] 使用紧急配置启动Chrome...")

	// 紧急配置选项（最保守）
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("single-process", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
	}

	err := cu.tryStartWithOptions(opts)
	if err == nil {
		cu.isRunning = true
	}
	return err
}

// tryStartWithOptions 尝试使用给定选项启动Chrome
func (cu *ChromeUtil) tryStartWithOptions(opts []chromedp.ExecAllocatorOption) error {
	log.Printf("[CHROME-UTIL] 开始创建Chrome分配器上下文...")

	// 如果有自定义Chrome路径，添加到选项中
	if cu.chromeExecPath != "" {
		log.Printf("[CHROME-UTIL] 使用自定义Chrome路径: %s", cu.chromeExecPath)
		opts = append(opts, chromedp.ExecPath(cu.chromeExecPath))
	}

	// 创建分配器上下文
	startTime := time.Now()
	cu.allocCtx, cu.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	log.Printf("[CHROME-UTIL] 分配器上下文创建完成，耗时: %v", time.Since(startTime))

	// 创建Chrome上下文
	log.Printf("[CHROME-UTIL] 开始创建Chrome上下文...")
	contextStartTime := time.Now()
	cu.ctx, cu.cancel = chromedp.NewContext(cu.allocCtx)
	log.Printf("[CHROME-UTIL] Chrome上下文创建完成，耗时: %v", time.Since(contextStartTime))

	// 立即进行简单测试
	log.Printf("[CHROME-UTIL] 开始Chrome基础功能测试...")
	testCtx, testCancel := context.WithTimeout(cu.ctx, 15*time.Second)
	defer testCancel()

	testStartTime := time.Now()
	err := chromedp.Run(testCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("[CHROME-UTIL] Chrome基础功能测试进行中...")
			return nil
		}),
	)

	testDuration := time.Since(testStartTime)
	log.Printf("[CHROME-UTIL] Chrome基础功能测试完成，耗时: %v", testDuration)

	if err != nil {
		log.Printf("[CHROME-UTIL] Chrome基础功能测试失败: %v", err)
		log.Printf("[CHROME-UTIL] 错误类型: %T", err)

		// 详细错误分析
		errStr := err.Error()
		if strings.Contains(errStr, "context deadline exceeded") {
			log.Printf("[CHROME-UTIL] 诊断: Chrome启动超时，可能原因:")
			log.Printf("[CHROME-UTIL] - 系统资源不足")
			log.Printf("[CHROME-UTIL] - Chrome二进制文件损坏")
			log.Printf("[CHROME-UTIL] - 权限问题")
			log.Printf("[CHROME-UTIL] - 网络或防火墙阻止")
		} else if strings.Contains(errStr, "exec: ") {
			log.Printf("[CHROME-UTIL] 诊断: Chrome执行失败")
			if cu.chromeExecPath != "" {
				log.Printf("[CHROME-UTIL] - 检查Chrome路径是否有效: %s", cu.chromeExecPath)
			} else {
				log.Printf("[CHROME-UTIL] - Chrome可能未正确下载或安装")
			}
		} else if strings.Contains(errStr, "connection refused") {
			log.Printf("[CHROME-UTIL] 诊断: Chrome进程启动失败")
		}

		return fmt.Errorf("Chrome基础功能测试失败: %w", err)
	}

	log.Printf("[CHROME-UTIL] Chrome启动和测试成功")
	return nil
}

// Stop 停止Chrome实例
func (cu *ChromeUtil) Stop() {
	cu.mu.Lock()
	defer cu.mu.Unlock()
	cu.stopInternal()
}

// stopInternal 内部停止方法
func (cu *ChromeUtil) stopInternal() {
	if !cu.isRunning {
		log.Printf("[CHROME-UTIL] Chrome实例已停止")
		return
	}

	log.Printf("[CHROME-UTIL] 正在停止Chrome实例...")
	startTime := time.Now()

	// 停止空闲计时器
	if cu.idleTimer != nil {
		cu.idleTimer.Stop()
		cu.idleTimer = nil
	}

	// 更新统计信息
	cu.statsLock.Lock()
	if cu.isRunning {
		cu.totalUptime += time.Since(cu.lastRestart)
	}
	cu.statsLock.Unlock()

	// 取消上下文
	if cu.cancel != nil {
		cu.cancel()
		cu.cancel = nil
	}

	if cu.allocCancel != nil {
		cu.allocCancel()
		cu.allocCancel = nil
	}

	cu.isRunning = false
	cu.ctx = nil
	cu.allocCtx = nil

	log.Printf("[CHROME-UTIL] Chrome实例已停止 | 停止耗时: %v | 总运行时间: %v",
		time.Since(startTime), cu.totalUptime)
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

// GetContext 获取Chrome上下文
func (cu *ChromeUtil) GetContext(timeout time.Duration) (context.Context, context.CancelFunc, error) {
	// 智能启动管理
	if err := cu.EnsureStarted(); err != nil {
		return nil, nil, fmt.Errorf("确保Chrome启动失败: %v", err)
	}

	// 获取信号量
	select {
	case cu.concurrencySem <- struct{}{}:
		// 成功获取信号量
	case <-time.After(30 * time.Second):
		return nil, nil, fmt.Errorf("等待并发槽位超时")
	}

	cu.mu.RLock()
	defer cu.mu.RUnlock()

	if !cu.isRunning || cu.ctx == nil {
		<-cu.concurrencySem // 释放信号量
		return nil, nil, fmt.Errorf("Chrome实例未运行")
	}

	// 创建任务上下文
	taskCtx, cancel := context.WithTimeout(cu.ctx, timeout)

	// 更新使用时间戳
	cu.statsLock.Lock()
	cu.lastUsed = time.Now()
	cu.usageCount++
	cu.statsLock.Unlock()

	// 包装cancel函数，确保释放信号量
	wrappedCancel := func() {
		cancel()
		<-cu.concurrencySem // 释放信号量
	}

	return taskCtx, wrappedCancel, nil
}

// IsHealthy 检查Chrome实例健康状态（简化版）
func (cu *ChromeUtil) IsHealthy() bool {
	cu.mu.RLock()
	defer cu.mu.RUnlock()

	if !cu.isRunning {
		return false
	}

	// 如果主上下文不存在，说明Chrome没有正确初始化
	if cu.ctx == nil {
		return false
	}

	// 首先检查上下文是否被取消
	select {
	case <-cu.ctx.Done():
		// 上下文已被取消，Chrome不健康
		log.Printf("[CHROME-UTIL] 上下文已取消，Chrome不健康: %v", cu.ctx.Err())
		return false
	default:
		// 上下文正常，继续检查
	}

	// 对于自动模式，在上下文正常的前提下采用更宽松的健康检查策略
	if cu.config.Mode == ChromeModeAuto {
		// 如果Chrome最近被使用过（5分钟内）且上下文正常，认为是健康的
		if time.Since(cu.lastUsed) < 5*time.Minute {
			return true
		}
	}

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
	cu.stopInternal()
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

// StartHealthMonitor 启动Chrome健康监控（简化版）
func (cu *ChromeUtil) StartHealthMonitor() {
	// 检查是否需要启动健康监控
	if !cu.config.EnableHealthCheck {
		log.Printf("[CHROME-UTIL] 健康检查已禁用，跳过监控启动")
		return
	}

	go func() {
		ticker := time.NewTicker(cu.config.HealthCheckInterval)
		defer ticker.Stop()

		consecutiveFailures := 0
		totalChecks := 0
		totalFailures := 0

		log.Printf("[CHROME-UTIL] Chrome健康监控已启动 (检查间隔: %v)", cu.config.HealthCheckInterval)

		for {
			select {
			case <-ticker.C:
				totalChecks++

				healthy := cu.IsHealthy()

				if !healthy {
					consecutiveFailures++
					totalFailures++

					log.Printf("[CHROME-UTIL] 健康检查失败 (连续:%d, 总计:%d/%d)",
						consecutiveFailures, totalFailures, totalChecks)

					// 达到重置阈值时进行详细诊断
					if consecutiveFailures >= 5 {
						log.Printf("[CHROME-UTIL] 连续失败达到阈值，开始详细诊断...")
						cu.performDetailedDiagnosis()

						log.Printf("[CHROME-UTIL] 执行强制重置...")
						err := cu.ForceReset()
						if err != nil {
							log.Printf("[CHROME-UTIL] 强制重置失败: %v", err)
						} else {
							log.Printf("[CHROME-UTIL] 强制重置成功")
							consecutiveFailures = 0
						}
					}
				} else {
					if consecutiveFailures > 0 {
						log.Printf("[CHROME-UTIL] 健康检查恢复正常 (之前连续失败%d次)", consecutiveFailures)
						consecutiveFailures = 0
					}
				}

				// 每10次检查输出一次统计
				if totalChecks%10 == 0 {
					successRate := float64(totalChecks-totalFailures) / float64(totalChecks) * 100
					log.Printf("[CHROME-UTIL] 健康统计: 成功率=%.1f%% (%d/%d)",
						successRate, totalChecks-totalFailures, totalChecks)
				}
			}
		}
	}()
}

// analyzeFailurePattern 分析失败模式
func (cu *ChromeUtil) analyzeFailurePattern(consecutiveFailures int) string {
	// 获取当前诊断信息
	diagnosis := cu.Diagnose()

	if !diagnosis["is_running"].(bool) {
		return "chrome_not_running"
	}

	if !diagnosis["context_exists"].(bool) {
		return "context_missing"
	}

	if issue, exists := diagnosis["issue"]; exists {
		issueStr := issue.(string)
		if strings.Contains(issueStr, "上下文已取消") || strings.Contains(issueStr, "context canceled") {
			return "context_canceled"
		}
		if strings.Contains(issueStr, "JavaScript执行失败") {
			return "javascript_execution_failed"
		}
		if strings.Contains(issueStr, "连接") || strings.Contains(issueStr, "connection") {
			return "connection_failed"
		}
	}

	// 根据连续失败次数判断
	if consecutiveFailures == 1 {
		return "transient_failure"
	} else if consecutiveFailures <= 3 {
		return "intermittent_failure"
	} else {
		return "persistent_failure"
	}
}

// calculateResetThreshold 动态计算重置阈值
func (cu *ChromeUtil) calculateResetThreshold(consecutiveFailures, totalFailures, totalChecks int) int {
	baseThreshold := 5

	// 如果总体失败率很高，降低阈值
	if totalChecks > 10 {
		failureRate := float64(totalFailures) / float64(totalChecks)
		if failureRate > 0.5 { // 失败率超过50%
			return 3
		} else if failureRate > 0.3 { // 失败率超过30%
			return 4
		}
	}

	// 如果已经连续失败很多次，保持较高的阈值避免过度重置
	if consecutiveFailures > 10 {
		return 8
	}

	return baseThreshold
}

// getRecoveryRecommendations 获取恢复建议
func (cu *ChromeUtil) getRecoveryRecommendations(failureReasons map[string]int, consecutiveFailures int) []string {
	recommendations := []string{}

	// 基于失败类型提供建议
	for reason, count := range failureReasons {
		switch reason {
		case "context_canceled":
			recommendations = append(recommendations,
				fmt.Sprintf("上下文取消问题(%d次): 检查Chrome进程是否异常退出，考虑增加资源限制", count))
		case "javascript_execution_failed":
			recommendations = append(recommendations,
				fmt.Sprintf("JavaScript执行失败(%d次): 检查Chrome响应性能，可能需要增加超时时间", count))
		case "connection_failed":
			recommendations = append(recommendations,
				fmt.Sprintf("连接失败(%d次): 检查网络状态和Chrome WebSocket连接", count))
		case "chrome_not_running":
			recommendations = append(recommendations,
				fmt.Sprintf("Chrome未运行(%d次): 检查Chrome二进制文件和启动参数", count))
		case "persistent_failure":
			recommendations = append(recommendations,
				fmt.Sprintf("持续失败(%d次): 考虑系统资源不足或Chrome配置问题", count))
		}
	}

	// 基于连续失败次数提供建议
	if consecutiveFailures >= 10 {
		recommendations = append(recommendations, "长期失败: 建议重启整个服务或检查系统环境")
	} else if consecutiveFailures >= 7 {
		recommendations = append(recommendations, "频繁失败: 建议检查系统内存和CPU使用率")
	} else if consecutiveFailures >= 5 {
		recommendations = append(recommendations, "多次失败: 建议检查Chrome启动参数和权限设置")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "常规恢复: 等待自动重置或手动重启Chrome实例")
	}

	return recommendations
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

// EnsureStarted 确保Chrome已启动（智能混合模式）
func (cu *ChromeUtil) EnsureStarted() error {

	// 更新使用统计
	cu.statsLock.Lock()
	cu.usageCount++
	cu.lastUsed = time.Now()
	cu.statsLock.Unlock()

	log.Printf("[CHROME-UTIL] 智能启动管理 (模式:%s, 使用次数:%d)", cu.getModeString(), cu.usageCount)

	// 智能混合模式逻辑
	switch cu.config.Mode {
	case ChromeModeCold:
		// 冷启动：每次都重新启动
		return cu.startWithColdMode()

	case ChromeModeWarm:
		// 热启动：保持运行
		return cu.startWithWarmMode()

	case ChromeModeAuto:
		// 智能模式：按需启动+智能管理
		return cu.startWithAutoMode()

	default:
		return cu.startWithAutoMode()
	}
}

// startWithColdMode 冷启动模式
func (cu *ChromeUtil) startWithColdMode() error {
	// 冷启动模式：每次都启动新的Chrome实例
	if cu.isRunning {
		log.Printf("[CHROME-UTIL] 冷启动模式：关闭现有实例")
		cu.stopInternal()
	}

	log.Printf("[CHROME-UTIL] 冷启动模式：启动新实例")
	return cu.startInternal()
}

// startWithWarmMode 热启动模式
func (cu *ChromeUtil) startWithWarmMode() error {
	// 热启动模式：保持Chrome运行
	if cu.isRunning && cu.IsHealthy() {
		log.Printf("[CHROME-UTIL] 热启动模式：实例已运行")
		cu.resetIdleTimer()
		return nil
	}

	if cu.isRunning {
		log.Printf("[CHROME-UTIL] 热启动模式：实例异常，重启")
		cu.stopInternal()
	}

	log.Printf("[CHROME-UTIL] 热启动模式：启动实例")
	err := cu.startInternal()
	if err == nil {
		cu.resetIdleTimer()
	}
	return err
}

// startWithAutoMode 智能自动模式
func (cu *ChromeUtil) startWithAutoMode() error {
	// 智能模式：根据使用情况动态决策

	// 如果Chrome正在运行且健康，直接使用
	if cu.isRunning && cu.IsHealthy() {
		log.Printf("[CHROME-UTIL] 智能模式：复用现有实例")
		cu.resetIdleTimer()
		return nil
	}

	// 如果Chrome在运行但不健康，重启
	if cu.isRunning {
		log.Printf("[CHROME-UTIL] 智能模式：实例不健康，重启")
		cu.stopInternal()
	}

	// 根据使用频率决定启动策略
	recentUsage := cu.getRecentUsageFrequency()

	if recentUsage > 5 { // 频繁使用
		log.Printf("[CHROME-UTIL] 智能模式：频繁使用，采用热启动策略")
		err := cu.startInternal()
		if err == nil {
			cu.resetIdleTimer() // 设置较长的空闲时间
		}
		return err
	} else {
		log.Printf("[CHROME-UTIL] 智能模式：偶尔使用，采用快速启动策略")
		return cu.startInternal() // 不设置空闲计时器，用完即关
	}
}

// getRecentUsageFrequency 获取最近的使用频率
func (cu *ChromeUtil) getRecentUsageFrequency() int {
	cu.statsLock.RLock()
	defer cu.statsLock.RUnlock()

	// 简单算法：如果10分钟内使用次数超过5次，认为是频繁使用
	if time.Since(cu.lastUsed) < 10*time.Minute && cu.usageCount > 0 {
		return int(cu.usageCount) // 返回总使用次数作为频率指标
	}
	return 0
}

// resetIdleTimer 重置空闲计时器
func (cu *ChromeUtil) resetIdleTimer() {
	// 只有在非冷启动模式且配置了空闲超时才启用计时器
	if cu.config.Mode == ChromeModeCold || cu.config.IdleTimeout <= 0 {
		return
	}

	if cu.idleTimer != nil {
		cu.idleTimer.Stop()
	}

	// 智能空闲超时：根据使用频率调整
	idleTimeout := cu.config.IdleTimeout
	if cu.config.Mode == ChromeModeAuto {
		usage := cu.getRecentUsageFrequency()
		if usage > 5 {
			// 频繁使用，延长空闲时间
			idleTimeout = cu.config.IdleTimeout * 2
			log.Printf("[CHROME-UTIL] 智能模式：频繁使用，延长空闲时间至 %v", idleTimeout)
		} else {
			// 偶尔使用，缩短空闲时间
			idleTimeout = cu.config.IdleTimeout / 2
			log.Printf("[CHROME-UTIL] 智能模式：偶尔使用，缩短空闲时间至 %v", idleTimeout)
		}
	}

	cu.idleTimer = time.AfterFunc(idleTimeout, func() {
		log.Printf("[CHROME-UTIL] 空闲超时(%v)，自动关闭Chrome", idleTimeout)
		cu.Stop()
	})
}

// startInternal 内部启动方法
func (cu *ChromeUtil) startInternal() error {
	if cu.isRunning {
		return nil
	}

	log.Printf("[CHROME-UTIL] 正在启动Chrome实例...")
	startTime := time.Now()

	// 诊断Chrome环境
	cu.diagnoseChromeEnvironment()

	// 获取Chrome可执行文件路径
	if chromeDownloader == nil {
		chromeDownloader = NewChromeDownloader()
	}
	execPath := chromeDownloader.GetChromeExecutablePath()
	cu.chromeExecPath = execPath

	// 多重启动策略
	strategies := []struct {
		name string
		fn   func() error
	}{
		{"最小配置", cu.startWithMinimalOptions},
		{"标准配置", cu.startWithStandardOptions},
		{"备用配置", cu.startWithFallbackOptions},
		{"紧急配置", cu.startWithEmergencyOptions},
	}

	var lastErr error
	for i, strategy := range strategies {
		log.Printf("[CHROME-UTIL] 尝试启动策略 %d/%d: %s", i+1, len(strategies), strategy.name)
		err := strategy.fn()
		if err == nil {
			cu.isRunning = true
			cu.lastRestart = time.Now()

			// 更新统计
			cu.statsLock.Lock()
			if cu.restarts == 0 {
				cu.totalUptime = time.Since(startTime)
			}
			cu.statsLock.Unlock()

			log.Printf("[CHROME-UTIL] Chrome实例启动成功 (策略%d) | 重启次数: %d | 启动耗时: %v",
				i+1, cu.restarts, time.Since(startTime))

			// 仅在预热模式或自动模式下启动健康检查
			if cu.config.EnableHealthCheck && cu.config.Mode != ChromeModeCold {
				cu.StartHealthMonitor()
			}

			return nil
		}
		lastErr = err
		log.Printf("[CHROME-UTIL] 启动策略 %d 失败: %v", i+1, err)
	}

	return fmt.Errorf("所有启动策略都失败了，最后错误: %v", lastErr)
}

// ConfigureChromeMode 配置Chrome运行模式的便捷函数
func ConfigureChromeMode(mode string) ChromeConfig {
	switch strings.ToLower(mode) {
	case "cold", "冷启动":
		return ChromeConfig{
			Mode:                ChromeModeCold,
			IdleTimeout:         0,     // 不需要空闲超时
			HealthCheckInterval: 0,     // 不需要健康检查
			EnableHealthCheck:   false, // 关闭健康检查
			PrewarmOnStart:      false, // 不预热
		}
	case "warm", "预热", "热启动":
		return ChromeConfig{
			Mode:                ChromeModeWarm,
			IdleTimeout:         10 * time.Minute, // 10分钟空闲后关闭
			HealthCheckInterval: 60 * time.Second, // 60秒健康检查
			EnableHealthCheck:   true,             // 启用健康检查
			PrewarmOnStart:      true,             // 预热启动
		}
	case "auto", "自动":
		return ChromeConfig{
			Mode:                ChromeModeAuto,
			IdleTimeout:         5 * time.Minute,  // 5分钟空闲后关闭
			HealthCheckInterval: 60 * time.Second, // 60秒健康检查
			EnableHealthCheck:   true,             // 启用健康检查
			PrewarmOnStart:      false,            // 不预热，按需启动
		}
	default:
		log.Printf("[CHROME-UTIL] 未知模式 '%s'，使用默认冷启动模式", mode)
		return DefaultChromeConfig
	}
}

// SetGlobalChromeMode 设置全局Chrome运行模式
func SetGlobalChromeMode(mode string) {
	config := ConfigureChromeMode(mode)
	log.Printf("[CHROME-UTIL] 设置全局Chrome模式: %s", config.getModeDescription())

	// 如果已经有实例在运行，先停止
	if globalChromeUtil != nil && globalChromeUtil.isRunning {
		log.Printf("[CHROME-UTIL] 停止现有Chrome实例以应用新配置")
		globalChromeUtil.Stop()
	}

	// 重置全局实例
	chromeUtilOnce = sync.Once{}
	globalChromeUtil = nil

	// 使用新配置创建实例
	GetGlobalChromeUtilWithConfig(config)
}

// getModeDescription 获取模式描述
func (config ChromeConfig) getModeDescription() string {
	switch config.Mode {
	case ChromeModeCold:
		return "冷启动模式 - 按需启动，节省资源"
	case ChromeModeWarm:
		return "预热模式 - 预先启动，快速响应"
	case ChromeModeAuto:
		return "自动模式 - 智能管理，平衡性能和资源"
	default:
		return "未知模式"
	}
}

// GetChromeStats 获取Chrome运行统计
func GetChromeStats() map[string]interface{} {
	if globalChromeUtil == nil {
		return map[string]interface{}{
			"status": "未初始化",
		}
	}

	stats := globalChromeUtil.GetDetailedStats()
	stats["mode"] = globalChromeUtil.getModeString()
	stats["mode_description"] = globalChromeUtil.config.getModeDescription()

	return stats
}

// performDetailedDiagnosis 执行详细诊断（仅在出问题时调用）
func (cu *ChromeUtil) performDetailedDiagnosis() {
	log.Printf("[CHROME-UTIL] === 详细诊断开始 ===")

	// 检查Chrome可执行文件
	if cu.chromeExecPath != "" {
		if _, err := os.Stat(cu.chromeExecPath); err != nil {
			log.Printf("[CHROME-UTIL] Chrome可执行文件异常: %v", err)
		} else {
			log.Printf("[CHROME-UTIL] Chrome可执行文件正常: %s", cu.chromeExecPath)
		}
	}

	// 检查上下文状态
	if cu.ctx != nil {
		select {
		case <-cu.ctx.Done():
			log.Printf("[CHROME-UTIL] 上下文已取消: %v", cu.ctx.Err())
		default:
			log.Printf("[CHROME-UTIL] 上下文状态正常")
		}
	} else {
		log.Printf("[CHROME-UTIL] 上下文为空")
	}

	// 系统资源检查
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("[CHROME-UTIL] 内存使用: %dMB, Goroutine: %d",
		m.Alloc/1024/1024, runtime.NumGoroutine())

	// 检查Chrome进程（仅在诊断时）
	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq chrome.exe", "/FO", "CSV")
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			chromeCount := len(lines) - 2
			if chromeCount > 0 {
				log.Printf("[CHROME-UTIL] 发现 %d 个Chrome进程", chromeCount)
			} else {
				log.Printf("[CHROME-UTIL] 未发现Chrome进程")
			}
		}
	}

	log.Printf("[CHROME-UTIL] === 详细诊断结束 ===")
}
