package services

import (
	"sync"
	"testing"
	"time"
	"whosee/types"
)

// MockProvider 模拟WHOIS提供商用于测试
type MockProvider struct {
	name         string
	queryCount   int
	shouldFail   bool
	responseTime time.Duration
	mu           sync.Mutex
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Query(domain string) (*types.WhoisResponse, error, bool) {
	m.mu.Lock()
	m.queryCount++
	m.mu.Unlock()

	// 模拟查询延迟
	if m.responseTime > 0 {
		time.Sleep(m.responseTime)
	}

	if m.shouldFail {
		return nil, nil, false
	}

	return &types.WhoisResponse{
		Domain:         domain,
		Registrar:      "Mock Registrar",
		CreateDate:     "2020-01-01",
		ExpiryDate:     "2025-12-31",
		StatusMessage:  "查询成功",
		SourceProvider: m.name,
		StatusCode:     200,
	}, nil, false
}

// TestSelectProviderConcurrency 测试selectProvider的并发安全性
func TestSelectProviderConcurrency(t *testing.T) {
	// 创建WhoisManager
	manager := NewWhoisManager(nil) // 不需要Redis用于这个测试

	// 添加多个mock providers
	for i := 1; i <= 4; i++ {
		provider := &MockProvider{
			name:         "MockProvider" + string(rune('A'+i-1)),
			responseTime: time.Millisecond * 10,
		}
		manager.AddProvider(provider)
	}

	// 等待初始化完成
	time.Sleep(time.Millisecond * 100)

	// 并发调用selectProvider
	const goroutines = 50
	const iterations = 20

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*iterations)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				provider := manager.selectProvider()
				if provider == nil {
					t.Errorf("Goroutine %d iteration %d: selectProvider returned nil", id, j)
					return
				}

				// 验证provider名称有效
				name := provider.Name()
				if name == "" {
					t.Errorf("Goroutine %d iteration %d: provider name is empty", id, j)
					return
				}
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()
	close(errors)

	// 检查是否有错误
	errorCount := 0
	for err := range errors {
		if err != nil {
			t.Error(err)
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Fatalf("TestSelectProviderConcurrency failed with %d errors", errorCount)
	}

	t.Logf("✅ Successfully completed %d concurrent selectProvider calls (%d goroutines × %d iterations)",
		goroutines*iterations, goroutines, iterations)
}

// TestTestProvidersHealthNonBlocking 测试TestProvidersHealth不阻塞查询
func TestTestProvidersHealthNonBlocking(t *testing.T) {
	// 跳过Redis相关测试（如果没有Redis）
	// 这个测试主要验证锁策略，不需要真实的Redis

	manager := NewWhoisManager(nil)

	// 添加providers（带延迟模拟远程调用）
	for i := 1; i <= 3; i++ {
		provider := &MockProvider{
			name:         "SlowProvider" + string(rune('A'+i-1)),
			responseTime: time.Millisecond * 500, // 模拟慢速API
		}
		manager.AddProvider(provider)
	}

	time.Sleep(time.Millisecond * 100)

	// 启动健康检查（会持续数秒）
	healthCheckDone := make(chan bool)
	go func() {
		manager.TestProvidersHealth()
		healthCheckDone <- true
	}()

	// 等待健康检查开始
	time.Sleep(time.Millisecond * 100)

	// 在健康检查进行期间，尝试并发调用selectProvider
	const concurrentCalls = 20
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < concurrentCalls; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			provider := manager.selectProvider()
			if provider == nil {
				t.Errorf("Call %d: selectProvider blocked or returned nil during health check", id)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	// selectProvider调用应该很快完成，不应该被健康检查阻塞
	// 如果持有全局写锁，这些调用会被阻塞数秒
	maxExpectedTime := time.Millisecond * 500 // 宽容一些，但远小于健康检查时间

	if elapsed > maxExpectedTime {
		t.Errorf("selectProvider calls took %v, expected < %v (might be blocked by health check)",
			elapsed, maxExpectedTime)
	} else {
		t.Logf("✅ %d concurrent selectProvider calls completed in %v (not blocked by health check)",
			concurrentCalls, elapsed)
	}

	// 等待健康检查完成
	select {
	case <-healthCheckDone:
		t.Log("✅ Health check completed successfully")
	case <-time.After(time.Second * 10):
		t.Error("Health check timeout")
	}
}

// TestConcurrentQueryAndHealthCheck 测试Query和HealthCheck并发执行
func TestConcurrentQueryAndHealthCheck(t *testing.T) {
	manager := NewWhoisManager(nil)

	// 添加providers
	for i := 1; i <= 3; i++ {
		provider := &MockProvider{
			name:         "ConcurrentProvider" + string(rune('A'+i-1)),
			responseTime: time.Millisecond * 100,
		}
		manager.AddProvider(provider)
	}

	time.Sleep(time.Millisecond * 100)

	var wg sync.WaitGroup
	const queries = 30
	const healthChecks = 5

	// 启动多个查询goroutine
	for i := 0; i < queries; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			provider := manager.selectProvider()
			if provider == nil {
				t.Errorf("Query %d: selectProvider returned nil", id)
			}
		}(i)
	}

	// 同时启动多个健康检查goroutine
	for i := 0; i < healthChecks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			result := manager.TestProvidersHealth()
			if result == nil {
				t.Errorf("HealthCheck %d: returned nil", id)
			}
		}(i)
	}

	// 等待所有操作完成
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Logf("✅ Successfully completed %d queries and %d health checks concurrently", queries, healthChecks)
	case <-time.After(time.Second * 30):
		t.Fatal("Test timeout: possible deadlock or extreme contention")
	}
}

// TestNoDataRace 验证没有数据竞争（需要使用 go test -race 运行）
func TestNoDataRace(t *testing.T) {
	manager := NewWhoisManager(nil)

	for i := 1; i <= 4; i++ {
		provider := &MockProvider{
			name:         "RaceTestProvider" + string(rune('A'+i-1)),
			responseTime: time.Millisecond * 50,
		}
		manager.AddProvider(provider)
	}

	time.Sleep(time.Millisecond * 100)

	// 大量并发操作
	const goroutines = 100
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 混合操作
			for j := 0; j < 10; j++ {
				switch j % 3 {
				case 0:
					manager.selectProvider()
				case 1:
					manager.GetProvidersStatus()
				case 2:
					if j%5 == 0 { // 不要太频繁调用健康检查
						manager.TestProvidersHealth()
					}
				}
			}
		}()
	}

	wg.Wait()

	t.Logf("✅ No data race detected in %d concurrent goroutines", goroutines)
	t.Log("⚠️  Run with 'go test -race' to verify race detector results")
}
