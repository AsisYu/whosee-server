/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: 服务容器，用于统一管理所有服务组件
 */
package services

import (
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// ServiceContainer 服务容器，管理所有服务组件
type ServiceContainer struct {
	RedisClient      *redis.Client
	WorkerPool       *WorkerPool
	WhoisManager     *WhoisManager
	DNSChecker       *DNSChecker
	ScreenshotChecker *ScreenshotChecker
	ITDogChecker     *ITDogChecker
	HealthChecker    *HealthChecker
	Limiter          *RateLimiter
}

// NewServiceContainer 创建新的服务容器
func NewServiceContainer(redisClient *redis.Client, workerPoolSize int) *ServiceContainer {
	container := &ServiceContainer{
		RedisClient: redisClient,
	}

	// 初始化工作池
	log.Printf("初始化工作池，大小: %d", workerPoolSize)
	container.WorkerPool = NewWorkerPool(workerPoolSize)
	container.WorkerPool.Start()

	// 初始化WHOIS管理器
	container.WhoisManager = NewWhoisManager(redisClient)

	// 初始化DNS检查器
	container.DNSChecker = NewDNSChecker()

	// 初始化截图检查器
	container.ScreenshotChecker = NewScreenshotChecker()

	// 初始化ITDog检查器
	container.ITDogChecker = NewITDogChecker()

	return container
}

// InitializeHealthChecker 初始化健康检查器
func (sc *ServiceContainer) InitializeHealthChecker() {
	sc.HealthChecker = NewHealthChecker(sc.WhoisManager, sc.DNSChecker, sc.ScreenshotChecker, sc.ITDogChecker)
	sc.HealthChecker.CheckIntervalDays = 3
	sc.HealthChecker.Start()
	// 服务启动时立即执行一次健康检查
	go sc.HealthChecker.ForceRefresh()
}

// InitializeLimiter 初始化限流器
func (sc *ServiceContainer) InitializeLimiter(key string, rate int, period time.Duration) {
	sc.Limiter = NewRateLimiter(sc.RedisClient, key, rate, period)
}

// Shutdown 关闭所有服务
func (sc *ServiceContainer) Shutdown() {
	// 关闭工作池
	if sc.WorkerPool != nil {
		log.Println("关闭工作池...")
		sc.WorkerPool.Stop()
	}

	// 关闭健康检查器
	if sc.HealthChecker != nil {
		log.Println("关闭健康检查器...")
		sc.HealthChecker.Stop()
	}

	// 关闭 Redis 客户端
	if sc.RedisClient != nil {
		log.Println("关闭 Redis 客户端...")
		sc.RedisClient.Close()
	}

	log.Println("所有服务已关闭")
}
