/*
 * @Author: AsisYu
 * @Date: 2025-04-25
 * @Description: Chrome浏览器实例管理器
 */
package services

import (
	"context"
	"dmainwhoseek/utils"
	"log"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// 全局Chrome管理器实例
var GlobalChromeManager *ChromeManager

// InitGlobalChromeManager 初始化全局Chrome管理器
func InitGlobalChromeManager() error {
	GlobalChromeManager = NewChromeManager()
	return GlobalChromeManager.Start()
}

// GetGlobalChromeManager 获取全局Chrome管理器实例
func GetGlobalChromeManager() *ChromeManager {
	return GlobalChromeManager
}

// ChromeManager Chrome浏览器实例管理器
type ChromeManager struct {
	ctx          context.Context
	cancel       context.CancelFunc
	allocCtx     context.Context
	allocCancel  context.CancelFunc
	mu           sync.RWMutex
	isRunning    bool
	restarts     int
	lastRestart  time.Time
	healthLogger *utils.HealthLogger
}

var (
	chromeManager *ChromeManager
	chromeOnce    sync.Once
)

// GetChromeManager 获取Chrome管理器单例
func GetChromeManager() *ChromeManager {
	chromeOnce.Do(func() {
		chromeManager = NewChromeManager()
	})
	return chromeManager
}

// NewChromeManager 创建新的Chrome管理器
func NewChromeManager() *ChromeManager {
	manager := &ChromeManager{
		isRunning:    false,
		restarts:     0,
		healthLogger: utils.GetHealthLogger(),
	}

	err := manager.Start()
	if err != nil {
		log.Printf("[CHROME-MANAGER] 初始化Chrome实例失败: %v", err)
	}

	return manager
}

// Start 启动Chrome实例
func (cm *ChromeManager) Start() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.isRunning {
		log.Printf("[CHROME-MANAGER] Chrome实例已在运行")
		return nil
	}

	log.Printf("[CHROME-MANAGER] 正在启动Chrome实例...")

	// Chrome启动选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.Flag("memory-pressure-off", true),
		chromedp.Flag("max_old_space_size", "4096"),
		// 减少内存使用
		chromedp.Flag("aggressive-cache-discard", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
	)

	// 创建分配器上下文
	cm.allocCtx, cm.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)

	// 创建Chrome上下文
	cm.ctx, cm.cancel = chromedp.NewContext(cm.allocCtx, chromedp.WithLogf(log.Printf))

	// 设置超时
	timeoutCtx, timeoutCancel := context.WithTimeout(cm.ctx, 30*time.Second)
	defer timeoutCancel()

	// 启动Chrome并访问一个简单页面以确保正常工作
	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate("about:blank"),
	)

	if err != nil {
		log.Printf("[CHROME-MANAGER] Chrome实例启动失败: %v", err)
		cm.cleanup()
		return err
	}

	cm.isRunning = true
	cm.lastRestart = time.Now()
	log.Printf("[CHROME-MANAGER] Chrome实例启动成功 | 重启次数: %d", cm.restarts)

	return nil
}

// Stop 停止Chrome实例
func (cm *ChromeManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.isRunning {
		return
	}

	log.Printf("[CHROME-MANAGER] 正在停止Chrome实例...")
	cm.cleanup()
	cm.isRunning = false
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
}

// Restart 重启Chrome实例
func (cm *ChromeManager) Restart() error {
	log.Printf("[CHROME-MANAGER] 正在重启Chrome实例...")

	cm.Stop()

	// 等待一段时间确保完全关闭
	time.Sleep(2 * time.Second)

	cm.restarts++
	err := cm.Start()

	if err != nil {
		log.Printf("[CHROME-MANAGER] Chrome实例重启失败: %v", err)
		return err
	}

	log.Printf("[CHROME-MANAGER] Chrome实例重启成功 | 总重启次数: %d", cm.restarts)
	return nil
}

// GetContext 获取Chrome上下文用于截图操作
func (cm *ChromeManager) GetContext(timeout time.Duration) (context.Context, context.CancelFunc, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.isRunning || cm.ctx == nil {
		cm.mu.RUnlock()
		err := cm.Restart()
		cm.mu.RLock()
		if err != nil {
			return nil, nil, err
		}
	}

	// 创建子上下文用于特定操作
	childCtx, cancel := chromedp.NewContext(cm.ctx)

	// 设置超时
	timeoutCtx, timeoutCancel := context.WithTimeout(childCtx, timeout)

	// 返回组合的取消函数
	combinedCancel := func() {
		timeoutCancel()
		cancel()
	}

	return timeoutCtx, combinedCancel, nil
}

// IsHealthy 检查Chrome实例是否健康
func (cm *ChromeManager) IsHealthy() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.isRunning || cm.ctx == nil {
		return false
	}

	// 创建一个简单的健康检查上下文
	healthCtx, cancel := context.WithTimeout(cm.ctx, 5*time.Second)
	defer cancel()

	// 尝试执行一个简单的操作
	err := chromedp.Run(healthCtx,
		chromedp.Evaluate(`document.readyState`, nil),
	)

	return err == nil
}

// GetStats 获取Chrome管理器统计信息
func (cm *ChromeManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return map[string]interface{}{
		"is_running":   cm.isRunning,
		"restarts":     cm.restarts,
		"last_restart": cm.lastRestart,
		"uptime":       time.Since(cm.lastRestart).String(),
		"is_healthy":   cm.IsHealthy(),
	}
}

// HealthCheck 定期健康检查并自动重启
func (cm *ChromeManager) HealthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !cm.IsHealthy() {
				cm.healthLogger.Printf("[CHROME-MANAGER] 健康检查失败，正在重启Chrome实例...")
				err := cm.Restart()
				if err != nil {
					cm.healthLogger.Printf("[CHROME-MANAGER] 自动重启失败: %v", err)
				}
			}
		}
	}
}

// StartHealthCheck 启动健康检查协程
func (cm *ChromeManager) StartHealthCheck() {
	go cm.HealthCheck()
	log.Printf("[CHROME-MANAGER] 已启动Chrome健康检查服务")
}
