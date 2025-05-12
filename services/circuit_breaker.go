/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-10 16:09:00
 * @Description: 电路断路器模式实现
 */
package services

import (
	"errors"
	"sync"
	"time"
)

// CircuitState 电路断路器状态
type CircuitState int

const (
	StateClosed CircuitState = iota // 关闭状态 - 正常工作
	StateOpen                       // 开启状态 - 电路断开
	StateHalfOpen                   // 半开启状态 - 允许一次请求
)

// CircuitBreaker 电路断路器模式实现
type CircuitBreaker struct {
	state            CircuitState
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	lastFailureTime  time.Time
	mutex            sync.RWMutex
	onStateChange    func(from, to CircuitState)
}

// NewCircuitBreaker 创建电路断路器实例
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

// OnStateChange 设置状态变化回调函数
func (cb *CircuitBreaker) OnStateChange(f func(from, to CircuitState)) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.onStateChange = f
}

// Execute 执行操作并检查电路断路器状态
func (cb *CircuitBreaker) Execute(operation func() error) error {
	// 检查电路断路器状态
	if !cb.AllowRequest() {
		return errors.New("circuit open")
	}
	
	err := operation()
	
	// 更新电路断路器状态
	cb.RecordResult(err == nil)
	
	return err
}

// AllowRequest 检查是否允许请求
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否到达重置超时时间
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			// 状态切换到半开启状态
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			if cb.state == StateOpen {
				prevState := cb.state
				cb.state = StateHalfOpen
				if cb.onStateChange != nil {
					cb.onStateChange(prevState, cb.state)
				}
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()
			return true
		}
		return false
	case StateHalfOpen:
		// 半开启状态允许一次请求
		return true
	default:
		return true
	}
}

// RecordResult 记录请求结果
func (cb *CircuitBreaker) RecordResult(success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	if success {
		// 成功请求
		switch cb.state {
		case StateHalfOpen:
			// 半开启状态成功请求后切换到关闭状态
			prevState := cb.state
			cb.state = StateClosed
			cb.failureCount = 0
			if cb.onStateChange != nil {
				cb.onStateChange(prevState, cb.state)
			}
		case StateClosed:
			// 关闭状态成功请求后重置失败计数
			cb.failureCount = 0
		}
	} else {
		// 失败请求
		cb.lastFailureTime = time.Now()
		
		switch cb.state {
		case StateClosed:
			// 关闭状态失败请求后增加失败计数
			cb.failureCount++
			// 达到失败阈值后切换到开启状态
			if cb.failureCount >= cb.failureThreshold {
				prevState := cb.state
				cb.state = StateOpen
				if cb.onStateChange != nil {
					cb.onStateChange(prevState, cb.state)
				}
			}
		case StateHalfOpen:
			// 半开启状态失败请求后切换到开启状态
			prevState := cb.state
			cb.state = StateOpen
			if cb.onStateChange != nil {
				cb.onStateChange(prevState, cb.state)
			}
		}
	}
}
