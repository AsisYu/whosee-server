/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-18 19:57:27
 * @LastEditors: AsisYu 2773943729@qq.com
 * @LastEditTime: 2025-01-18 22:38:29
 * @FilePath: \dmainwhoseek\server\main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"dmainwhoseek/handlers"
	"dmainwhoseek/middleware"
	"dmainwhoseek/providers"
	"dmainwhoseek/services"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/gin-contrib/cors"
)

// 全局变量
var rdb *redis.Client
var logFile *lumberjack.Logger

// 自定义日志格式
func setupLogger() {
	// 设置日志格式，包含时间戳、文件信息和日志级别
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 确保日志目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("警告: 无法创建日志目录: %v", err)
	}

	// 创建日志切割器
	logFile = &lumberjack.Logger{
		Filename:   fmt.Sprintf("logs/server_%s.log", time.Now().Format("2006-01-02")),
		MaxSize:    100,   // 每个日志文件最大大小，单位为MB
		MaxBackups: 30,    // 保留的旧日志文件最大数量
		MaxAge:     90,    // 保留旧日志文件的最大天数
		Compress:   true,  // 是否压缩旧的日志文件
		LocalTime:  true,  // 使用本地时间
	}

	// 同时输出到控制台和文件
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	
	// 设置Gin的默认日志输出
	gin.DefaultWriter = mw
	
	log.Println("日志系统初始化完成，启用了日志切割功能")
}

// 辅助函数
func getPort(defaultPort string) string {
	port := defaultPort
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	// 确保端口格式正确（带冒号前缀）
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	return port
}

func healthCheck(c *gin.Context) {
	// 获取参数
	detailed := c.DefaultQuery("detailed", "false") == "true"
	// 忽略force参数，不再支持通过API强制刷新
	
	log.Printf("健康检查API调用: detailed=%v, URI=%s", detailed, c.Request.RequestURI)
	
	// 基本响应
	response := gin.H{
		"status":  "up",
		"version": os.Getenv("APP_VERSION"),
		"time":    time.Now().UTC().Format(time.RFC3339),
		"services": gin.H{}, // 初始化services map
	}
	
	// 获取IP地址
	ip := c.ClientIP()
	
	// 判断是否为内部IP或开发环境
	isInternal := isWhitelistedIP(ip, c)
	
	// 验证访问权限：允许白名单内IP直接访问，非白名单IP需要JWT验证
	if !isInternal {
		// 如果不在白名单内，检查JWT令牌或API Key
		if hasValidKey(c) {
			// 有效的API Key，允许访问
			log.Printf("健康检查API通过API Key访问: %s", ip)
		} else {
			// 检查JWT令牌
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" || len(authHeader) < 8 {
				c.JSON(403, gin.H{
					"status":  "error",
					"message": "访问此API需要认证",
					"code":    "AUTH_REQUIRED",
				})
				return
			}
			
			// 验证JWT格式
			tokenString := authHeader[7:] // 移除"Bearer "前缀
			token, err := jwt.ParseWithClaims(tokenString, &middleware.Claims{}, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return []byte(os.Getenv("JWT_SECRET")), nil
			})
			
			if err != nil || !token.Valid {
				log.Printf("健康检查API JWT验证失败: %s, 错误: %v", ip, err)
				c.JSON(401, gin.H{
					"status":  "error",
					"message": "令牌验证失败",
					"code":    "INVALID_TOKEN",
				})
				return
			}
			
			log.Printf("健康检查API通过JWT令牌访问: %s", ip)
			// 令牌有效，继续处理
		}
	}
	
	// 尝试获取健康检查器
	if healthCheckerObj, hasHealthChecker := c.Get("healthChecker"); hasHealthChecker {
		hc, _ := healthCheckerObj.(*services.HealthChecker)
		if hc != nil {
			// 缓存策略：
			// 所有请求都使用缓存数据，不支持通过API强制刷新
			// 健康检查数据只通过服务端定时任务更新
			
			// 不再检查force参数，始终使用缓存
			log.Printf("使用缓存的健康检查结果")
			lastResults := hc.GetLastCheckResults()
			
			// 记录缓存内容键值
			resultKeys := make([]string, 0)
			for k := range lastResults {
				resultKeys = append(resultKeys, k)
			}
			log.Printf("缓存内容键值: %v", resultKeys)
			
			if lastResults != nil && len(lastResults) > 0 {
				// 添加lastCheckTime
				response["lastCheckTime"] = hc.GetLastCheckTime().Format(time.RFC3339)
				
				// 使用缓存中的整体状态
				if cachedStatus, exists := lastResults["status"]; exists && cachedStatus != nil {
					response["status"] = cachedStatus
					log.Printf("使用缓存的状态: %v", cachedStatus)
				}
				
				// 获取缓存的服务信息
				if cachedServices, exists := lastResults["services"]; exists && cachedServices != nil {
					// 尝试转换服务信息为合适的格式
					servicesMap := gin.H{}
					
					switch services := cachedServices.(type) {
					case gin.H:
						servicesMap = services
					case map[string]interface{}:
						for k, v := range services {
							servicesMap[k] = v
						}
					}
					
					// 记录缓存中服务的详细信息
					servicesJson, _ := json.Marshal(servicesMap)
					log.Printf("使用缓存的服务信息: %s", string(servicesJson))
					
					// 设置响应中的services字段
					response["services"] = servicesMap
					
					// 如果是简要模式，简化服务信息
					if !detailed {
						for serviceName, serviceInfo := range servicesMap {
							// 确保serviceInfo是map[string]interface{}
							if serviceMap, ok := serviceInfo.(map[string]interface{}); ok {
								// 创建简化版本，只保留状态和计数信息
								simplifiedService := gin.H{
									"status": serviceMap["status"],
								}
								
								// 保留计数字段
								if total, exists := serviceMap["total"]; exists {
									simplifiedService["total"] = total
								}
								if available, exists := serviceMap["available"]; exists {
									simplifiedService["available"] = available
								}
								
								// 更新服务映射
								servicesMap[serviceName] = simplifiedService
							}
						}
						
						log.Printf("简化后的服务信息: %+v", servicesMap)
					}
					
					// 最终检查响应中的services是否为空
					if servicesVal, exists := response["services"]; !exists || servicesVal == nil {
						response["services"] = gin.H{}
						log.Printf("警告: 响应中服务信息为空，使用空映射")
					}
					
					// 记录最终响应
					responseJson, _ := json.Marshal(response)
					log.Printf("最终返回的响应: %s", string(responseJson))
					
					c.JSON(200, response)
					return
				} else {
					log.Printf("缓存为空或无效，但不执行新的健康检查")
					// 返回等待消息而不是执行新的健康检查
					response["status"] = "pending"
					response["message"] = "健康检查数据正在准备中，请稍后再试"
					c.JSON(200, response)
					return
				}
			} else {
				log.Printf("缓存为空或无效，但不执行新的健康检查")
				// 返回等待消息而不是执行新的健康检查
				response["status"] = "pending"
				response["message"] = "健康检查数据正在准备中，请稍后再试"
				c.JSON(200, response)
				return
			}
		}
	}
	
	log.Printf("没有可用的健康检查器，返回等待消息")
	
	// 如果没有获取到健康检查器，返回等待消息
	response["status"] = "pending"
	response["message"] = "健康检查服务尚未就绪，请稍后再试"
	c.JSON(200, response)
	return
}

// 验证域名是否有效
func isValidDomain(domain string) bool {
	// 基本域名格式验证
	pattern := `^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`
	match, _ := regexp.MatchString(pattern, domain)
	return match
}

// 检查IP是否在白名单内
func isWhitelistedIP(ip string, c *gin.Context) bool {
	// 本地开发环境始终允许访问
	if os.Getenv("APP_ENV") == "development" {
		log.Printf("本地开发环境请求 (IP: %s) - 允许访问详细健康检查", ip)
		return true
	}
	
	// 检查请求是否来自合法的前端应用
	origin := c.GetHeader("Origin")
	referer := c.GetHeader("Referer")
	
	// 配置中的允许来源列表
	allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	
	// 检查Origin是否为允许的前端来源
	if origin != "" {
		for _, allowedOrigin := range allowedOrigins {
			if strings.TrimSpace(allowedOrigin) != "" && strings.Contains(origin, strings.TrimSpace(allowedOrigin)) {
				log.Printf("合法前端请求 (IP: %s, Origin: %s) - 允许访问健康检查", ip, origin)
				return true
			}
		}
	}
	
	// 检查Referer是否为允许的前端来源
	if referer != "" {
		for _, allowedOrigin := range allowedOrigins {
			if strings.TrimSpace(allowedOrigin) != "" && strings.Contains(referer, strings.TrimSpace(allowedOrigin)) {
				log.Printf("合法前端请求 (IP: %s, Referer: %s) - 允许访问健康检查", ip, referer)
				return true
			}
		}
	}
	
	// 读取白名单IP
	trustedIPs := strings.Split(os.Getenv("TRUSTED_IPS"), ",")
	for _, trustedIP := range trustedIPs {
		if strings.TrimSpace(trustedIP) == ip {
			log.Printf("白名单IP请求 (IP: %s) - 允许访问详细健康检查", ip)
			return true
		}
	}
	
	// 检查内部局域网IP
	if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") {
		log.Printf("内部局域网IP请求 (IP: %s) - 允许访问健康检查", ip)
		return true
	}
	
	// 本地回环地址
	if ip == "127.0.0.1" || ip == "::1" {
		log.Printf("本地回环地址请求 (IP: %s) - 允许访问健康检查", ip)
		return true
	}
	
	log.Printf("外部IP请求 (IP: %s) - 拒绝访问详细健康检查", ip)
	return false
}

// 验证API密钥
func hasValidKey(c *gin.Context) bool {
	key := c.GetHeader("X-API-Key")
	if key == "" {
		key = c.Query("api_key")
	}
	
	// 验证API密钥
	validKey := os.Getenv("API_KEY")
	if key != "" && validKey != "" && key == validKey {
		return true
	}
	
	return false
}

// Gin路由器中间件，用于在请求上下文中添加各种服务
func serviceMiddleware(whoisManager, dnsChecker, screenshotChecker, itdogChecker, healthChecker interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 注入WHOIS管理器
		if whoisManager != nil {
			c.Set("whoisManager", whoisManager)
		}
		
		// 注入DNS检查器
		if dnsChecker != nil {
			c.Set("dnsChecker", dnsChecker)
		}
		
		// 注入截图检查器
		if screenshotChecker != nil {
			c.Set("screenshotChecker", screenshotChecker)
		}
		
		// 注入IT Dog检查器
		if itdogChecker != nil {
			c.Set("itdogChecker", itdogChecker)
		}
		
		// 注入健康检查器
		if healthChecker != nil {
			c.Set("healthChecker", healthChecker)
		}
		
		// 继续处理请求
		c.Next()
	}
}

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	
	// 初始化日志系统
	setupLogger()
	
	// 初始化Redis客户端
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	
	rdb = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     os.Getenv("REDIS_PASSWORD"),
		DB:           0,
		PoolSize:     100,                // 连接池大小
		MinIdleConns: 10,                // 最小空闲连接数
		DialTimeout:  5 * time.Second,    // 连接超时
		ReadTimeout:  3 * time.Second,    // 读取超时
		WriteTimeout: 3 * time.Second,    // 写入超时
		PoolTimeout:  4 * time.Second,    // 获取连接超时
		IdleTimeout:  5 * time.Minute,    // 空闲连接超时
		MaxConnAge:   30 * time.Minute,   // 连接最大存活时间
	})
	
	// 初始化工作池 - 基于CPU核心数自动调整工作池大小
	numWorkers := runtime.NumCPU() * 2 // 工作池大小为CPU核心数的2倍
	log.Printf("初始化工作池，大小: %d", numWorkers)
	workerPool := services.NewWorkerPool(numWorkers)
	workerPool.Start()
	defer workerPool.Stop()
	
	// 初始化分布式限流器
	apiLimiter := services.NewRateLimiter(rdb, "limit:whoisapi", 60, time.Minute)
	
	// 初始化各服务组件
	whoisManager := services.NewWhoisManager(rdb)
	
	// 初始化并添加WHOIS提供商
	whoisFreaksProvider := providers.NewWhoisFreaksProvider()
	whoisXMLProvider := providers.NewWhoisXMLProvider()
	
	// 将提供商添加到管理器
	whoisManager.AddProvider(whoisFreaksProvider)
	whoisManager.AddProvider(whoisXMLProvider)
	
	// 初始化其他服务组件
	dnsChecker := services.NewDNSChecker()
	screenshotChecker := services.NewScreenshotChecker()
	itdogChecker := services.NewITDogChecker()
	
	// 启动定时健康检查
	healthChecker := services.NewHealthChecker(whoisManager, dnsChecker, screenshotChecker, itdogChecker)
	healthChecker.CheckIntervalDays = 3
	healthChecker.Start()
	// 服务启动时立即执行一次健康检查
	go healthChecker.ForceRefresh()
	
	// 创建Gin引擎
	r := gin.Default()
	
	// 添加静态文件服务
	r.Static("/static/screenshots", "./static/screenshots")
	r.Static("/static/itdog", "./static/itdog")
	
	// 确保screenshots目录存在
	os.MkdirAll("./static/screenshots", 0755)
	os.MkdirAll("./static/itdog", 0755)
	
	// 启用CORS中间件（确保放在第一个位置）
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://whosee.me", "http://localhost:5173", "http://localhost:3000", "https://whois-api.os.tn"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "X-Cache"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	
	// 配置中间件，注入服务组件到上下文
	r.Use(serviceMiddleware(whoisManager, dnsChecker, screenshotChecker, itdogChecker, healthChecker))
	
	// 注册路由
	r.GET("/api/health", healthCheck)
	
	// 添加身份验证路由
	r.POST("/api/auth/token", middleware.GenerateToken(rdb))
	
	// 添加域名查询路由
	r.GET("/api/query", func(c *gin.Context) {
		// 获取域名参数
		domain := c.Query("domain")
		if domain == "" {
			c.JSON(400, gin.H{"error": "Domain parameter is required"})
			return
		}
		
		// 验证域名格式
		if !isValidDomain(domain) {
			c.JSON(400, gin.H{"error": "Invalid domain format"})
			return
		}
		
		// 将域名放入上下文
		c.Set("domain", domain)
		
		// 创建一个独立于HTTP请求的上下文
		reqCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		
		// 使用分布式限流器检查请求频率
		allowed, err := apiLimiter.Allow(reqCtx, c.ClientIP())
		if err != nil {
			log.Printf("限流检查失败: %v", err)
			// 继续执行，避免因限流器故障影响服务
		} else if !allowed {
			c.JSON(429, gin.H{"error": "Too many requests, please try again later"})
			return
		}
		
		// 创建一个响应通道
		resultChan := make(chan gin.H, 1)
		errorChan := make(chan error, 1)
		
		// 提交任务到工作池
		submitted := workerPool.SubmitWithContext(reqCtx, func() {
			// 创建子上下文供处理函数使用
			handlerCtx := context.WithValue(reqCtx, "domain", domain)
			
			// 调用异步WHOIS查询
			result, err := handlers.AsyncWhoisQuery(handlerCtx, rdb)
			if err != nil {
				errorChan <- err
				return
			}
			resultChan <- result
		})
		
		if !submitted {
			c.JSON(503, gin.H{"error": "Service is busy, please try again later"})
			return
		}
		
		// 等待结果或超时
		select {
		case result := <-resultChan:
			c.JSON(200, result)
		case err := <-errorChan:
			c.JSON(500, gin.H{"error": err.Error()})
		case <-reqCtx.Done():
			c.JSON(504, gin.H{"error": "Request timed out"})
		}
	})
	
	// 添加DNS查询路由
	r.GET("/api/dns", func(c *gin.Context) {
		// 获取域名参数
		domain := c.Query("domain")
		if domain == "" {
			c.JSON(400, gin.H{"error": "Domain parameter is required"})
			return
		}
		
		// 验证域名格式
		if !isValidDomain(domain) {
			c.JSON(400, gin.H{"error": "Invalid domain format"})
			return
		}
		
		// 将域名放入上下文
		c.Set("domain", domain)
		
		// 使用工作池处理DNS查询
		reqCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		
		// 创建响应通道
		resultChan := make(chan gin.H, 1)
		errorChan := make(chan error, 1)
		
		// 提交任务到工作池
		submitted := workerPool.SubmitWithContext(reqCtx, func() {
			// 创建子上下文供处理函数使用
			handlerCtx := context.WithValue(reqCtx, "domain", domain)
			
			// 调用异步DNS查询
			result, err := handlers.AsyncDNSQuery(handlerCtx, rdb)
			if err != nil {
				errorChan <- err
				return
			}
			resultChan <- result
		})
		
		if !submitted {
			c.JSON(503, gin.H{"error": "Service is busy, please try again later"})
			return
		}
		
		// 等待结果或超时
		select {
		case result := <-resultChan:
			c.JSON(200, result)
		case err := <-errorChan:
			c.JSON(500, gin.H{"error": err.Error()})
		case <-reqCtx.Done():
			c.JSON(504, gin.H{"error": "Request timed out"})
		}
	})
	
	// 添加ITDog测速截图路由（先注册更具体的路由）
	r.GET("/api/screenshot/itdog/:domain", func(c *gin.Context) {
		domain := c.Param("domain")
		if domain == "" {
			c.JSON(400, gin.H{"error": "Domain parameter is required"})
			return
		}
		
		// 验证域名格式
		if !isValidDomain(domain) {
			c.JSON(400, gin.H{"error": "Invalid domain format"})
			return
		}
		
		// 将域名放入上下文
		c.Set("domain", domain)
		
		// 使用工作池处理截图请求
		reqCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second) // 截图可能需要更长时间
		defer cancel()
		
		// 创建响应通道
		resultChan := make(chan gin.H, 1)
		errorChan := make(chan error, 1)
		
		// 提交任务到工作池
		submitted := workerPool.SubmitWithContext(reqCtx, func() {
			// 创建子上下文供处理函数使用
			handlerCtx := context.WithValue(reqCtx, "domain", domain)
			
			// 调用异步ITDog截图查询
			result, err := handlers.AsyncItdogScreenshot(handlerCtx, rdb)
			if err != nil {
				errorChan <- err
				return
			}
			resultChan <- result
		})
		
		if !submitted {
			c.JSON(503, gin.H{"error": "Service is busy, please try again later"})
			return
		}
		
		// 等待结果或超时
		select {
		case result := <-resultChan:
			c.JSON(200, result)
		case err := <-errorChan:
			c.JSON(500, gin.H{"error": err.Error()})
		case <-reqCtx.Done():
			c.JSON(504, gin.H{"error": "Request timed out"})
		}
	})
	
	// 添加常规截图路由（后注册更一般的路由）
	r.GET("/api/screenshot/:domain", func(c *gin.Context) {
		domain := c.Param("domain")
		if domain == "" {
			c.JSON(400, gin.H{"error": "Domain parameter is required"})
			return
		}
		
		// 验证域名格式
		if !isValidDomain(domain) {
			c.JSON(400, gin.H{"error": "Invalid domain format"})
			return
		}
		
		// 将域名放入上下文
		c.Set("domain", domain)
		
		// 使用工作池处理截图请求
		reqCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second) // 截图可能需要更长时间
		defer cancel()
		
		// 创建响应通道
		resultChan := make(chan gin.H, 1)
		errorChan := make(chan error, 1)
		
		// 提交任务到工作池
		submitted := workerPool.SubmitWithContext(reqCtx, func() {
			// 创建子上下文供处理函数使用
			handlerCtx := context.WithValue(reqCtx, "domain", domain)
			
			// 调用异步截图查询
			result, err := handlers.AsyncScreenshot(handlerCtx, rdb)
			if err != nil {
				errorChan <- err
				return
			}
			resultChan <- result
		})
		
		if !submitted {
			c.JSON(503, gin.H{"error": "Service is busy, please try again later"})
			return
		}
		
		// 等待结果或超时
		select {
		case result := <-resultChan:
			c.JSON(200, result)
		case err := <-errorChan:
			c.JSON(500, gin.H{"error": err.Error()})
		case <-reqCtx.Done():
			c.JSON(504, gin.H{"error": "Request timed out"})
		}
	})
	
	// 创建HTTP服务器，配置超时参数
	port := getPort("8080")
	srv := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}
	
	// 优雅关闭
	go func() {
		// 接收系统终止信号
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("正在关闭服务器...")
		
		// 设置关闭超时上下文
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("服务器被强制关闭: %v", err)
		}
		
		log.Println("服务器已安全关闭")
	}()
	
	// 启动服务
	log.Printf("服务器启动在端口%s，环境：%s", port, os.Getenv("APP_ENV"))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
