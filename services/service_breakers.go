/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-10 18:09:00
 * @Description: 服务专用熔断器管理
 */
package services

import (
	"log"
	"sync"
	"time"
)

// ServiceBreakers 管理各种服务的熔断器实例
type ServiceBreakers struct {
	ScreenshotBreaker *CircuitBreaker
	ItdogBreaker      *CircuitBreaker
	once              sync.Once
}

// 全局单例实例
var serviceBreakers *ServiceBreakers
var breakers sync.Once

// GetServiceBreakers 获取服务熔断器管理实例
func GetServiceBreakers() *ServiceBreakers {
	breakers.Do(func() {
		serviceBreakers = &ServiceBreakers{}
		serviceBreakers.init()
	})
	return serviceBreakers
}

// 初始化各服务熔断器
func (sb *ServiceBreakers) init() {
	// 截图服务熔断器 - 5次失败后熔断，60秒后尝试恢复
	sb.ScreenshotBreaker = NewCircuitBreaker(5, 60*time.Second)
	sb.ScreenshotBreaker.OnStateChange(func(from, to CircuitState) {
		log.Printf("截图服务熔断器状态从 %v 变更为 %v", from, to)
		// 当熔断器开启时可以触发告警或其他操作
		if to == StateOpen {
			log.Printf("警告: 截图服务可能不可用，已启动熔断保护")
		} else if to == StateClosed {
			log.Printf("信息: 截图服务已恢复正常")
		}
	})

	// ITDog服务熔断器 - 增加失败阈值和重置时间，降低敏感度
	sb.ItdogBreaker = NewCircuitBreaker(8, 120*time.Second) // 从3次失败改为8次，重置时间从45秒改为120秒
	sb.ItdogBreaker.OnStateChange(func(from, to CircuitState) {
		log.Printf("ITDog服务熔断器状态从 %v 变更为 %v", from, to)
		// 当熔断器开启时可以触发告警或其他操作
		if to == StateOpen {
			log.Printf("警告: ITDog服务连续失败超过阈值，已启动熔断保护")
		} else if to == StateClosed {
			log.Printf("信息: ITDog服务已恢复正常")
		} else if to == StateHalfOpen {
			log.Printf("信息: ITDog服务正在尝试恢复，处于半开状态")
		}
	})
}

// CircuitStateToString 将熔断器状态转换为可读字符串
func CircuitStateToString(state CircuitState) string {
	switch state {
	case StateClosed:
		return "关闭(正常)"
	case StateOpen:
		return "开启(熔断)"
	case StateHalfOpen:
		return "半开(恢复中)"
	default:
		return "未知"
	}
}

// GetScreenshotBreakerStatus 获取截图服务熔断器状态
func (sb *ServiceBreakers) GetScreenshotBreakerStatus() map[string]interface{} {
	sb.ScreenshotBreaker.mutex.RLock()
	defer sb.ScreenshotBreaker.mutex.RUnlock()

	return map[string]interface{}{
		"state":            CircuitStateToString(sb.ScreenshotBreaker.state),
		"failureCount":     sb.ScreenshotBreaker.failureCount,
		"failureThreshold": sb.ScreenshotBreaker.failureThreshold,
		"resetTimeout":     sb.ScreenshotBreaker.resetTimeout.String(),
		"lastFailureTime":  sb.ScreenshotBreaker.lastFailureTime.Format(time.RFC3339),
	}
}

// GetItdogBreakerStatus 获取ITDog服务熔断器状态
func (sb *ServiceBreakers) GetItdogBreakerStatus() map[string]interface{} {
	sb.ItdogBreaker.mutex.RLock()
	defer sb.ItdogBreaker.mutex.RUnlock()

	return map[string]interface{}{
		"state":            CircuitStateToString(sb.ItdogBreaker.state),
		"failureCount":     sb.ItdogBreaker.failureCount,
		"failureThreshold": sb.ItdogBreaker.failureThreshold,
		"resetTimeout":     sb.ItdogBreaker.resetTimeout.String(),
		"lastFailureTime":  sb.ItdogBreaker.lastFailureTime.Format(time.RFC3339),
	}
}
