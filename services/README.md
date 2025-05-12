# 服务目录 (Services)

## 目录作用

服务目录包含应用程序的核心业务逻辑组件。这些服务组件负责处理复杂的业务逻辑，与外部API交互，并管理数据。它们被设计为可重用的模块，与特定HTTP传输层解耦。

## 文件列表与功能

- `container.go` - 服务容器和依赖注入
- `whois.go` - WHOIS查询服务
- `whois_manager.go` - WHOIS查询管理服务
- `dns_checker.go` - DNS服务健康检查
- `screenshot_checker.go` - 网站截图服务
- `itdog_checker.go` - ITDog服务检查
- `health_checker.go` - 系统健康检查服务
- `rate_limiter.go` - 请求限流服务
- `worker_pool.go` - 工作池实现
- `circuit_breaker.go` - 熔断器实现
- `service_breakers.go` - 服务熔断器实现

## 服务容器

`ServiceContainer`是管理所有服务依赖关系的核心组件：

```go
type ServiceContainer struct {
    RedisClient *redis.Client
    WorkerPool  *WorkerPool
    
    // 核心业务服务
    WhoisManager      *WhoisManager
    DNSChecker        *DNSChecker
    ScreenshotChecker *ScreenshotChecker
    ITDogChecker      *ITDogChecker
    
    // 基础设施服务
    HealthChecker *HealthChecker
    Limiter       *RateLimiter
    
    // 熔断器服务
    ServiceBreakers *ServiceBreakers
}
```

## 高并发优化

服务包中实现了多种高并发优化机制：

1. **工作池模式** - 基于CPU核心数动态调整的请求处理池
2. **分布式限流器** - 使用Redis实现的限流机制
3. **熔断器模式** - 防止系统在故障条件下过载
4. **服务级缓存** - 降低外部服务调用频率

## 健康检查系统

健康检查服务提供系统各组件的健康状态：

1. **DNS服务健康检查** - 检查多个DNS服务器的可用性
2. **截图服务健康检查** - 检查网站截图服务是否正常运行
3. **ITDog服务健康检查** - 检查ITDog API的可用性

## 异步服务处理

大多数服务都遵循异步处理模式：

```go
// 异步查询示例
func (w *WhoisManager) AsyncQuery(domain string) string {
    // 生成唯一任务ID
    taskID := generateTaskID(domain)
    
    // 启动异步任务
    go func() {
        // 执行查询
        result, _ := w.Query(domain)
        
        // 将结果存入Redis
        w.storeResult(taskID, result)
    }()
    
    return taskID
}
```

## 设计原则

1. **单一职责原则** - 每个服务只负责一种明确的功能
2. **依赖注入** - 通过构造函数注入依赖项
3. **错误处理** - 错误被处理并返回给调用者
4. **服务边界** - 保持清晰的服务边界定义

## 工作池模式

`worker_pool.go` 实现了基于CPU核心数动态调整的请求处理池：

```go
func NewWorkerPool(maxWorkers int) *WorkerPool {
    if maxWorkers <= 0 {
        // 使用CPU核心数作为默认值
        maxWorkers = runtime.NumCPU()
    }
    
    return &WorkerPool{
        maxWorkers: maxWorkers,
        jobQueue:   make(chan Job, 100),
        quit:       make(chan bool),
    }
}
```

## 分布式限流器

`rate_limiter.go` 使用Redis实现了限流机制：

```go
func (l *RateLimiter) Allow(key string, rate int, period time.Duration) (bool, error) {
    now := time.Now().Unix()
    periodSeconds := int64(period.Seconds())
    windowKey := fmt.Sprintf("%s:%d", key, now/periodSeconds)
    
    // 使用Redis执行限流检查
    return l.redisClient.Eval(ctx, `
        local current = redis.call('INCR', KEYS[1])
        if current == 1 then
            redis.call('EXPIRE', KEYS[1], ARGV[1])
        end
        return current <= tonumber(ARGV[2])
    `, []string{windowKey}, periodSeconds, rate).Bool()
}
```

## 熔断器模式

`circuit_breaker.go` 和 `service_breakers.go` 实现了熔断器模式：

```go
// 熔断器执行函数
func (cb *CircuitBreaker) Execute(request func() (interface{}, error)) (interface{}, error) {
    if !cb.AllowRequest() {
        return nil, ErrCircuitOpen
    }
    
    result, err := request()
    
    cb.RecordResult(err == nil)
    
    return result, err
}
```

## Redis连接池

服务容器在初始化时配置Redis连接池：

```go
func NewRedisClient(addr string) *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr:         addr,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        PoolSize:     10 * runtime.NumCPU(),  // 根据CPU核心数调整池大小
        MinIdleConns: runtime.NumCPU(),      // 保持最少的空闲连接
        MaxConnAge:   30 * time.Minute,      // 最大连接年龄
    })
}
```

## 健康检查系统

`health_checker.go` 实现了系统健康检查服务：

```go
func (h *HealthChecker) DetailedHealth() map[string]interface{} {
    // 首先检查缓存
    cachedStatus, err := h.getCachedHealthStatus()
    if err == nil {
        return cachedStatus
    }
    
    // 如果缓存不可用，执行实时检查
    result := map[string]interface{}{}
    
    // 检查DNS服务
    if h.dnsChecker != nil {
        dnsStatus := h.dnsChecker.CheckHealth()
        result["dns"] = dnsStatus
    }
    
    // 检查截图服务
    if h.screenshotChecker != nil {
        screenshotStatus := h.screenshotChecker.CheckHealth()
        result["screenshot"] = screenshotStatus
    }
    
    // 检查ITDog服务
    if h.itdogChecker != nil {
        itdogStatus := h.itdogChecker.CheckHealth()
        result["itdog"] = itdogStatus
    }
    
    // 缓存检查结果
    h.cacheHealthStatus(result)
    
    return result
}
```

## 截图服务

`screenshot_checker.go` 实现了网站截图服务：

```go
func (s *ScreenshotChecker) TakeScreenshot(url string) (*ScreenshotResult, error) {
    // 使用熔断器执行截图任务
    return s.breaker.Execute(func() (interface{}, error) {
        // 执行截图
        result, err := s.screenshotService.CaptureScreenshot(url)
        if err != nil {
            return nil, err
        }
        
        return result, nil
    })
}
```

## 异步服务处理

大多数服务都遵循异步处理模式：

```go
// 异步查询示例
func (w *WhoisManager) AsyncQuery(domain string) string {
    // 生成唯一任务ID
    taskID := generateTaskID(domain)
    
    // 启动异步任务
    go func() {
        // 执行查询
        result, _ := w.Query(domain)
        
        // 将结果存入Redis
        w.storeResult(taskID, result)
    }()
    
    return taskID
}

// 获取异步查询结果
func (w *WhoisManager) GetAsyncResult(taskID string) (*WhoisResult, error) {
    data, err := w.redisClient.Get(context.Background(), "whois:task:"+taskID).Bytes()
    if err != nil {
        if err == redis.Nil {
            return nil, ErrTaskNotFound
        }
        return nil, err
    }
    
    var result WhoisResult
    if err := json.Unmarshal(data, &result); err != nil {
        return nil, err
    }
    
    return &result, nil
}
```

## 设计原则

1. **单一职责原则** - 每个服务只负责一种明确的功能
2. **依赖注入** - 通过构造函数注入依赖项
3. **错误处理** - 错误被处理并返回给调用者
4. **服务边界** - 保持清晰的服务边界定义

## 服务优化

1. **缓存机制** - 使用Redis实现缓存机制
2. **指标监控** - 使用Prometheus实现指标监控
3. **日志收集** - 使用ELK Stack实现日志收集
4. **服务降级** - 使用熔断器实现服务降级
