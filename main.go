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
	"dmainwhoseek/handlers"
	"dmainwhoseek/middleware"
	"dmainwhoseek/providers"
	"dmainwhoseek/services"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// 自定义日志格式
func setupLogger() {
	// 设置日志格式，包含时间戳、文件信息和日志级别
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 创建日志文件
	logFile, err := os.OpenFile(
		fmt.Sprintf("logs/server_%s.log", time.Now().Format("2006-01-02")),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0666,
	)
	if err != nil {
		log.Printf("警告: 无法创建日志文件: %v", err)
		return
	}

	// 同时输出到控制台和文件
	log.SetOutput(logFile)
}

// 配置热加载功能
func setupConfig() {
	// 初始加载
	if err := godotenv.Load(); err != nil {
		log.Printf("警告: 未找到.env文件: %v", err)
	}

	// 加载本地开发环境配置
	if gin.Mode() != gin.ReleaseMode {
		godotenv.Load(".env.local")
	}

	// 只在开发环境且明确启用热加载时监听配置变化
	if gin.Mode() != gin.ReleaseMode && os.Getenv("ENABLE_CONFIG_HOT_RELOAD") == "true" {
		log.Println("配置热加载已启用，间隔5分钟")
		go func() {
			for {
				// 增加重载间隔到5分钟，减少不必要的重载
				time.Sleep(5 * time.Minute)
				if err := godotenv.Load(); err == nil {
					// 只在配置实际变化时记录日志
					log.Println("配置已重载")
				}
			}
		}()
	}
}

// 辅助函数
func getPort(defaultPort string) string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return defaultPort
}

func healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":   "up",
		"version": os.Getenv("APP_VERSION"),
		"time":     time.Now().UTC().Format(time.RFC3339),
	})
}

func main() {
	// 设置日志
	setupLogger()
	
	// 初始化配置
	setupConfig()

	// 设置 gin 模式
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 初始化 Redis
	rdb := handlers.InitRedis()
	log.Println("Redis 连接成功")

	// 中间件顺序优化
	r.Use(
		middleware.Cors(),
		middleware.Security(),
		gin.Recovery(),
		middleware.ErrorHandler(),
		middleware.RateLimit(rdb),
		middleware.EnhancedLogging(),
		middleware.Monitoring(rdb),
	)

	// 初始化WHOIS管理器
	whoisManager := services.NewWhoisManager(rdb)
	whoisManager.AddProvider(providers.NewWhoisFreaksProvider())
	whoisManager.AddProvider(providers.NewWhoisXMLProvider())

	// API 路由组
	api := r.Group("/api")
	{
		// 健康检查
		api.GET("/health", healthCheck)

		// 获取临时token的端点 - 无需认证
		api.POST("/auth/token", middleware.GenerateToken(rdb))

		// 需要认证的路由
		authorized := api.Group("")
		authorized.Use(middleware.AuthRequired(rdb))
		{
			// 提取公共处理逻辑
			queryHandler := func(c *gin.Context) {
				var req struct {
					Domain string `json:"domain" form:"domain" binding:"required"`
				}

				// 同时支持JSON和表单数据
				if err := c.ShouldBind(&req); err != nil {
					c.JSON(400, gin.H{"error": "域名参数必填"})
					return
				}

				response, err, cached := whoisManager.Query(req.Domain)
				if err != nil {
					c.AbortWithStatusJSON(500, gin.H{
						"code":    "WHOIS_QUERY_FAILED",
						"message": "无法获取域名信息",
						"detail":  err.Error(),
					})
					return
				}

				if cached {
					c.Header("X-Cache", "HIT")
				} else {
					c.Header("X-Cache", "MISS")
				}

				c.JSON(200, response)
			}

			authorized.GET("/query", queryHandler)  // 新增GET支持
			authorized.POST("/query", queryHandler)
		}
	}

	// 服务器配置
	port := getPort("3000")
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("服务器启动于端口 %s (%s)", port, gin.Mode())

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
