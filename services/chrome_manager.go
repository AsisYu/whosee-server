/*
 * @Author: AsisYu
 * @Date: 2025-01-20
 * @Description: 重构的Chrome管理器 - 统一资源管理
 */
package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"dmainwhoseek/utils"

	"github.com/chromedp/chromedp"
)

// ChromeManager Chrome管理器
type ChromeManager struct {
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	allocCtx         context.Context
	allocCancel      context.CancelFunc
	isRunning        int32               // 原子操作，避免锁竞争
	maxConcurrent    int                 // 最大并发数
	currentTasks     int32               // 当前任务数
	semaphore        chan struct{}       // 并发控制信号量
	config           *ChromeManagerConfig
	stats            *ChromeStats
	circuitBreaker   *CircuitBreaker     // 使用现有的熔断器
	lastUsed         time.Time
	startTime        time.Time
}

// ChromeManagerConfig Chrome管理器配置
type ChromeManagerConfig struct {
	MaxConcurrent       int           // 最大并发数
	TaskTimeout         time.Duration // 任务超时时间
	IdleTimeout         time.Duration // 空闲超时时间
	StartupTimeout      time.Duration // 启动超时时间
	HealthCheckInterval time.Duration // 健康检查间隔
	EnableCircuitBreaker bool         // 是否启用熔断器
	ChromeOptions       []chromedp.ExecAllocatorOption // Chrome选项
}

// ChromeStats Chrome统计信息
type ChromeStats struct {
	mu            sync.RWMutex
	totalTasks    int64         // 总任务数
	successTasks  int64         // 成功任务数
	failedTasks   int64         // 失败任务数
	totalDuration time.Duration // 总执行时间
	restarts      int64         // 重启次数
	lastRestart   time.Time     // 最后重启时间
}

// DefaultChromeManagerConfig 默认Chrome管理器配置
var DefaultChromeManagerConfig = &ChromeManagerConfig{
	MaxConcurrent:        3,
	TaskTimeout:          60 * time.Second,
	IdleTimeout:          5 * time.Minute,
	StartupTimeout:       30 * time.Second,
	HealthCheckInterval:  60 * time.Second,
	EnableCircuitBreaker: true,
	ChromeOptions: []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.WindowSize(1920, 1080),
	},
}

var (
	globalChromeManager *ChromeManager
	chromeManagerOnce   sync.Once
)

// GetGlobalChromeManager 获取全局Chrome管理器
func GetGlobalChromeManager() *ChromeManager {
	chromeManagerOnce.Do(func() {
		globalChromeManager = NewChromeManager(DefaultChromeManagerConfig)
	})
	return globalChromeManager
}

// NewChromeManager 创建Chrome管理器
func NewChromeManager(config *ChromeManagerConfig) *ChromeManager {
	if config == nil {
		config = DefaultChromeManagerConfig
	}

	manager := &ChromeManager{
		maxConcurrent:  config.MaxConcurrent,
		semaphore:      make(chan struct{}, config.MaxConcurrent),
		config:         config,
		stats:          &ChromeStats{},
		lastUsed:       time.Now(),
	}

	// 初始化熔断器
	if config.EnableCircuitBreaker {
		manager.circuitBreaker = NewCircuitBreaker(5, 30*time.Second)
	}

	log.Printf("[CHROME-MANAGER] 创建Chrome管理器，最大并发: %d", config.MaxConcurrent)
	return manager
}

// Start 启动Chrome实例
func (cm *ChromeManager) Start() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if atomic.LoadInt32(&cm.isRunning) == 1 {
		log.Printf("[CHROME-MANAGER] Chrome实例已在运行")
		return nil
	}

	log.Printf("[CHROME-MANAGER] 启动Chrome实例...")
	startTime := time.Now()

	// 获取Chrome可执行文件路径
	chromeDownloader := utils.NewChromeDownloader()
	execPath := chromeDownloader.GetChromeExecutablePath()

	// 构建Chrome选项
	opts := make([]chromedp.ExecAllocatorOption, len(cm.config.ChromeOptions))
	copy(opts, cm.config.ChromeOptions)

	// 如果有自定义Chrome路径，添加到选项中
	if execPath != "" {
		log.Printf("[CHROME-MANAGER] 使用Chrome路径: %s", execPath)
		opts = append(opts, chromedp.ExecPath(execPath))
	}

	// 创建分配器上下文
	cm.allocCtx, cm.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)

	// 创建Chrome上下文
	cm.ctx, cm.cancel = chromedp.NewContext(cm.allocCtx)

	// 验证Chrome实例
	if err := cm.validateChrome(); err != nil {
		cm.cleanup()
		return fmt.Errorf("Chrome实例验证失败: %v", err)
	}

	// 更新状态
	atomic.StoreInt32(&cm.isRunning, 1)
	cm.startTime = time.Now()
	cm.lastUsed = time.Now()

	// 更新统计
	cm.stats.mu.Lock()
	cm.stats.restarts++
	cm.stats.lastRestart = time.Now()
	cm.stats.mu.Unlock()

	duration := time.Since(startTime)
	log.Printf("[CHROME-MANAGER] Chrome实例启动成功，耗时: %v", duration)

	// 启动空闲检查
	go cm.startIdleMonitor()

	return nil
}

// validateChrome 验证Chrome实例
func (cm *ChromeManager) validateChrome() error {
	// 创建独立的验证上下文，避免影响主上下文
	testCtx, testCancel := chromedp.NewContext(cm.allocCtx)
	defer testCancel()

	ctx, cancel := context.WithTimeout(testCtx, cm.config.StartupTimeout)
	defer cancel()

	// 简单的验证操作
	var result string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`navigator.userAgent`, &result),
	)

	if err != nil {
		return fmt.Errorf("Chrome验证失败: %v", err)
	}

	log.Printf("[CHROME-MANAGER] Chrome验证成功")
	return nil
}

// Stop 停止Chrome实例
func (cm *ChromeManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if atomic.LoadInt32(&cm.isRunning) == 0 {
		return
	}

	log.Printf("[CHROME-MANAGER] 停止Chrome实例...")
	cm.cleanup()
	atomic.StoreInt32(&cm.isRunning, 0)

	log.Printf("[CHROME-MANAGER] Chrome实例已停止")
}

// cleanup 清理资源
func (cm *ChromeManager) cleanup() {
	if cm.cancel != nil {
		cm.cancel()
		cm.cancel = nil
	}

	if cm.allocCancel != nil {
		cm.allocCancel()
		cm.allocCancel = nil
	}

	cm.ctx = nil
	cm.allocCtx = nil
}

// GetContext 获取Chrome上下文
func (cm *ChromeManager) GetContext(timeout time.Duration) (context.Context, context.CancelFunc, error) {
	// 检查并启动Chrome
	if err := cm.ensureRunning(); err != nil {
		return nil, nil, fmt.Errorf("确保Chrome运行失败: %v", err)
	}

	// 检查熔断器
	if !cm.AllowRequest() {
		return nil, nil, fmt.Errorf("熔断器开启，拒绝请求")
	}

	// 获取并发许可
	select {
	case cm.semaphore <- struct{}{}:
		// 成功获取许可
	case <-time.After(10 * time.Second):
		return nil, nil, fmt.Errorf("获取并发许可超时")
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if atomic.LoadInt32(&cm.isRunning) == 0 || cm.ctx == nil {
		<-cm.semaphore // 释放许可
		return nil, nil, fmt.Errorf("Chrome实例未运行")
	}

	// 创建任务上下文
	taskCtx, cancel := context.WithTimeout(cm.ctx, timeout)

	// 增加当前任务计数
	atomic.AddInt32(&cm.currentTasks, 1)
	cm.lastUsed = time.Now()

	// 包装cancel函数
	wrappedCancel := func() {
		cancel()
		atomic.AddInt32(&cm.currentTasks, -1)
		<-cm.semaphore // 释放许可
	}

	return taskCtx, wrappedCancel, nil
}

// ensureRunning 确保Chrome正在运行
func (cm *ChromeManager) ensureRunning() error {
	if atomic.LoadInt32(&cm.isRunning) == 1 && cm.isHealthy() {
		return nil
	}

	// 需要启动或重启
	if atomic.LoadInt32(&cm.isRunning) == 1 {
		log.Printf("[CHROME-MANAGER] Chrome实例不健康，重启...")
		cm.Stop()
	}

	return cm.Start()
}

// isHealthy 检查Chrome健康状态
func (cm *ChromeManager) isHealthy() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.ctx == nil {
		return false
	}

	// 检查上下文是否被取消
	select {
	case <-cm.ctx.Done():
		return false
	default:
		return true
	}
}

// AllowRequest 检查是否允许请求（熔断器）
func (cm *ChromeManager) AllowRequest() bool {
	if cm.circuitBreaker == nil {
		return true
	}

	return cm.circuitBreaker.AllowRequest()
}

// OnSuccess 记录成功操作
func (cm *ChromeManager) OnSuccess(duration time.Duration) {
	cm.stats.mu.Lock()
	cm.stats.totalTasks++
	cm.stats.successTasks++
	cm.stats.totalDuration += duration
	cm.stats.mu.Unlock()

	if cm.circuitBreaker != nil {
		cm.circuitBreaker.RecordResult(true)
	}
}

// OnFailure 记录失败操作
func (cm *ChromeManager) OnFailure(duration time.Duration) {
	cm.stats.mu.Lock()
	cm.stats.totalTasks++
	cm.stats.failedTasks++
	cm.stats.totalDuration += duration
	cm.stats.mu.Unlock()

	if cm.circuitBreaker != nil {
		cm.circuitBreaker.RecordResult(false)
	}
}

// GetStats 获取统计信息
func (cm *ChromeManager) GetStats() map[string]interface{} {
	cm.stats.mu.RLock()
	defer cm.stats.mu.RUnlock()

	var successRate float64
	if cm.stats.totalTasks > 0 {
		successRate = float64(cm.stats.successTasks) / float64(cm.stats.totalTasks) * 100
	}

	var avgDuration time.Duration
	if cm.stats.totalTasks > 0 {
		avgDuration = cm.stats.totalDuration / time.Duration(cm.stats.totalTasks)
	}

	stats := map[string]interface{}{
		"is_running":       atomic.LoadInt32(&cm.isRunning) == 1,
		"is_healthy":       cm.isHealthy(),
		"current_tasks":    atomic.LoadInt32(&cm.currentTasks),
		"max_concurrent":   cm.maxConcurrent,
		"available_slots":  cm.maxConcurrent - int(atomic.LoadInt32(&cm.currentTasks)),
		"total_tasks":      cm.stats.totalTasks,
		"success_tasks":    cm.stats.successTasks,
		"failed_tasks":     cm.stats.failedTasks,
		"success_rate":     successRate,
		"avg_duration_ms":  avgDuration.Milliseconds(),
		"restarts":         cm.stats.restarts,
		"last_restart":     cm.stats.lastRestart.Format(time.RFC3339),
		"uptime_seconds":   time.Since(cm.startTime).Seconds(),
	}

	return stats
}

// startIdleMonitor 启动空闲监控
func (cm *ChromeManager) startIdleMonitor() {
	if cm.config.IdleTimeout <= 0 {
		return
	}

	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&cm.isRunning) == 0 {
				return // Chrome已停止
			}

			// 检查是否空闲超时
			if time.Since(cm.lastUsed) > cm.config.IdleTimeout && atomic.LoadInt32(&cm.currentTasks) == 0 {
				log.Printf("[CHROME-MANAGER] 空闲超时，自动停止Chrome")
				cm.Stop()
				return
			}
		}
	}
}

// Restart 重启Chrome实例
func (cm *ChromeManager) Restart() error {
	log.Printf("[CHROME-MANAGER] 重启Chrome实例...")
	cm.Stop()
	time.Sleep(2 * time.Second) // 等待完全关闭
	return cm.Start()
}

// IsHealthy 对外接口 - 检查Chrome实例是否健康
func (cm *ChromeManager) IsHealthy() bool {
	return cm.isHealthy()
}