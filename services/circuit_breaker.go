/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-10 16:09:00
 * @Description: u7194u65adu5668u6a21u5f0fu5b9eu73b0
 */
package services

import (
	"errors"
	"sync"
	"time"
)

// CircuitState u7194u65adu5668u72b6u6001
type CircuitState int

const (
	StateClosed CircuitState = iota // u5173u95edu72b6u6001 - u6b63u5e38u5de5u4f5c
	StateOpen                       // u5f00u542fu72b6u6001 - u7194u65adu751fu6548
	StateHalfOpen                   // u534au5f00u72b6u6001 - u5c1du8bd5u6062u590d
)

// CircuitBreaker u5b9eu73b0u7194u65adu5668u6a21u5f0f
type CircuitBreaker struct {
	state            CircuitState
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	lastFailureTime  time.Time
	mutex            sync.RWMutex
	onStateChange    func(from, to CircuitState)
}

// NewCircuitBreaker u521bu5efau65b0u7684u7194u65adu5668
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

// OnStateChange u8bbeu7f6eu72b6u6001u53d8u5316u56deu8c03
func (cb *CircuitBreaker) OnStateChange(f func(from, to CircuitState)) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.onStateChange = f
}

// Execute u6267u884cu53d7u7194u65adu5668u4fddu62a4u7684u64cdu4f5c
func (cb *CircuitBreaker) Execute(operation func() error) error {
	// u68c0u67e5u7194u65adu5668u72b6u6001
	if !cb.AllowRequest() {
		return errors.New("circuit open")
	}
	
	err := operation()
	
	// u66f4u65b0u7194u65adu5668u72b6u6001
	cb.RecordResult(err == nil)
	
	return err
}

// AllowRequest u5224u65adu662fu5426u5141u8bb8u8bf7u6c42u901au8fc7
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// u68c0u67e5u662fu5426u5230u8fbeu91cdu7f6eu65f6u95f4
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			// u72b6u6001u8f6cu6362u4e3au534au5f00
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
		// u534au5f00u72b6u6001u53eau5141u8bb8u4e00u4e2au8bf7u6c42u901au8fc7
		return true
	default:
		return true
	}
}

// RecordResult u8bb0u5f55u8bf7u6c42u7ed3u679c
func (cb *CircuitBreaker) RecordResult(success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	if success {
		// u6210u529fu60c5u51b5
		switch cb.state {
		case StateHalfOpen:
			// u534au5f00u72b6u6001u6210u529fu5219u6062u590du5230u5173u95edu72b6u6001
			prevState := cb.state
			cb.state = StateClosed
			cb.failureCount = 0
			if cb.onStateChange != nil {
				cb.onStateChange(prevState, cb.state)
			}
		case StateClosed:
			// u5173u95edu72b6u6001u6210u529fu5219u91cdu7f6eu5931u8d25u8ba1u6570
			cb.failureCount = 0
		}
	} else {
		// u5931u8d25u60c5u51b5
		cb.lastFailureTime = time.Now()
		
		switch cb.state {
		case StateClosed:
			// u5173u95edu72b6u6001u4e0bu589eu52a0u5931u8d25u8ba1u6570
			cb.failureCount++
			// u8fbeu5230u9608u503cu5219u8f6cu4e3au5f00u542fu72b6u6001
			if cb.failureCount >= cb.failureThreshold {
				prevState := cb.state
				cb.state = StateOpen
				if cb.onStateChange != nil {
					cb.onStateChange(prevState, cb.state)
				}
			}
		case StateHalfOpen:
			// u534au5f00u72b6u6001u4e0bu5931u8d25u7acbu5373u8f6cu4e3au5f00u542fu72b6u6001
			prevState := cb.state
			cb.state = StateOpen
			if cb.onStateChange != nil {
				cb.onStateChange(prevState, cb.state)
			}
		}
	}
}
