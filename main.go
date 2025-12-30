package main

import (
	"context"
	"whosee/middleware"
	"whosee/providers"
	"whosee/routes"
	"whosee/services"
	"whosee/utils"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 全局变量
var logFile *lumberjack.Logger

// 自定义日志格式
func setupLogger() {
	// 设置日志格式，包含时间戳、文件信息和日志级别
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 确保日志目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("警告: 无法创建日志目录: %v", err)
	}

	// 创建主服务器日志切割器
	logFile = &lumberjack.Logger{
		Filename:   fmt.Sprintf("logs/server_%s.log", time.Now().Format("2006-01-02")),
		MaxSize:    100,  // 每个日志文件最大大小，单位为MB
		MaxBackups: 30,   // 保留的旧日志文件最大数量
		MaxAge:     90,   // 保留旧日志文件的最大天数
		Compress:   true, // 是否压缩旧的日志文件
		LocalTime:  true, // 使用本地时间
	}

	// 同时输出到控制台和文件
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	// 设置Gin的默认日志输出
	gin.DefaultWriter = mw

	// 初始化健康检查日志记录器
	utils.InitHealthLogger()

	log.Println("日志系统初始化完成，启用了日志切割功能")
	log.Println("健康检查日志记录器已初始化")
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

// 组合对外访问URL（用于启动时提示）
func buildPublicURL(listenPort string) string {
	// 优先使用 PUBLIC_URL
	if v := os.Getenv("PUBLIC_URL"); v != "" {
		return v
	}
	proto := os.Getenv("PUBLIC_PROTO")
	if proto == "" {
		if os.Getenv("FORCE_HTTPS") == "true" {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := os.Getenv("PUBLIC_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("PUBLIC_PORT")
	if port == "" {
		// listenPort 可能带":"，去掉前缀
		port = strings.TrimPrefix(listenPort, ":")
	}
	// 如为默认端口则省略
	omitPort := (proto == "http" && port == "80") || (proto == "https" && port == "443")
	if omitPort {
		return fmt.Sprintf("%s://%s", proto, host)
	}
	return fmt.Sprintf("%s://%s:%s", proto, host, port)
}

// 推断环境名，优先 APP_ENV/ENV，其次 GIN_MODE，默认 development
func deriveEnvironment() string {
	if v := os.Getenv("APP_ENV"); v != "" {
		return v
	}
	if v := os.Getenv("ENV"); v != "" {
		return v
	}
	switch strings.ToLower(os.Getenv("GIN_MODE")) {
	case "release":
		return "production"
	case "test":
		return "test"
	default:
		return "development"
	}
}

// 统一的服务就绪横幅，增强可见性
func printReadyBanner(publicURL, listenPort string) {
	version := os.Getenv("APP_VERSION")
	env := deriveEnvironment()
	p := strings.TrimPrefix(listenPort, ":")
	line := strings.Repeat("=", 64)
	log.Printf("\n%s\n服务已就绪 (Whosee Server)\n- 版本: %s\n- 环境: %s\n- 监听端口: %s\n- 对外URL: %s\n%s\n", line, version, env, p, publicURL, line)
}

// 从环境变量中读取CORS配置
func getCorsConfig() cors.Config {
	// 从环境变量读取CORS允许的源，默认为开发环境常用地址
	allowedOrigins := []string{"http://localhost:3000", "http://localhost:5173", "https://whosee.me"}
	if origins := os.Getenv("CORS_ORIGINS"); origins != "" {
		allowedOrigins = strings.Split(origins, ",")
		// 清理空格
		for i := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
		}
	}

	// 从环境变量读取CORS允许的方法，默认为标准HTTP方法
	allowedMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	if methods := os.Getenv("CORS_ALLOWED_METHODS"); methods != "" {
		allowedMethods = strings.Split(methods, ",")
	}

	// 从环境变量读取CORS允许的头，默认为常用头
	allowedHeaders := []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}
	if headers := os.Getenv("CORS_ALLOWED_HEADERS"); headers != "" {
		allowedHeaders = strings.Split(headers, ",")
	}

	// 从环境变量读取CORS暴露的头，默认为空数组
	exposedHeaders := []string{"Content-Length", "X-Cache"}
	if headers := os.Getenv("CORS_EXPOSED_HEADERS"); headers != "" {
		exposedHeaders = strings.Split(headers, ",")
	}

	// 从环境变量读取CORS最大年龄，默认为12小时
	maxAge := 12 * time.Hour
	if ageStr := os.Getenv("CORS_MAX_AGE"); ageStr != "" {
		if age, err := time.ParseDuration(ageStr); err == nil {
			maxAge = age
		}
	}

	// 打印CORS配置信息，便于调试
	log.Printf("CORS配置: 允许的源=%v", allowedOrigins)
	log.Printf("CORS配置: 允许的方法=%v", allowedMethods)

	// 创建并返回CORS配置
	return cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     allowedMethods,
		AllowHeaders:     allowedHeaders,
		ExposeHeaders:    exposedHeaders,
		AllowCredentials: true,
		MaxAge:           maxAge,
	}
}

// ensureSecurityConfig 强制执行关键运行时安全先决条件
// 🔐 安全修复：在服务器启动前验证安全配置，防止带着不安全配置运行
func ensureSecurityConfig() {
	// 验证JWT_SECRET必须设置且非空
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable must be set before starting the server")
	}

	// 生产环境强制使用Release模式，防止泄露调试信息
	env := deriveEnvironment()
	if strings.EqualFold(env, "production") {
		if gin.Mode() != gin.ReleaseMode {
			log.Println("生产环境检测到GIN_MODE!=release，强制切换到Release模式以避免泄露调试信息")
		}
		gin.SetMode(gin.ReleaseMode)
	}
}

func main() {
	// 加载环境变量（.env文件可选，支持纯环境变量部署）
	if err := godotenv.Load(); err != nil {
		if os.IsNotExist(err) {
			log.Println("未找到.env文件，将使用系统环境变量")
		} else {
			log.Fatalf("加载.env文件失败: %v", err)
		}
	}

	// 初始化日志系统
	setupLogger()

	// 🔐 安全修复：验证安全配置（JWT_SECRET、GIN_MODE等）
	ensureSecurityConfig()

	log.Printf("启动服务器，版本：%s，环境：%s", os.Getenv("APP_VERSION"), deriveEnvironment())

	// 首先确保Chrome可用 - 在所有其他服务之前
	log.Println("=== 开始Chrome预检查和下载 ===")
	chromeDownloader := utils.NewChromeDownloader()
	if chromeExecPath, err := chromeDownloader.EnsureChrome(); err != nil {
		log.Printf("Chrome下载失败: %v，将继续使用系统Chrome", err)
	} else {
		log.Printf("Chrome已准备就绪: %s", chromeExecPath)
	}
	log.Println("=== Chrome预检查完成 ===")

	// 初始化Redis客户端
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     os.Getenv("REDIS_PASSWORD"),
		DB:           0,
		PoolSize:     100,              // 连接池大小
		MinIdleConns: 10,               // 最小空闲连接数
		DialTimeout:  5 * time.Second,  // 连接超时
		ReadTimeout:  3 * time.Second,  // 读取超时
		WriteTimeout: 3 * time.Second,  // 写入超时
		PoolTimeout:  4 * time.Second,  // 获取连接超时
		IdleTimeout:  5 * time.Minute,  // 空闲连接超时
		MaxConnAge:   30 * time.Minute, // 连接最大存活时间
	})

	// 初始化服务容器
	numCPU := runtime.NumCPU()
	serviceContainer := services.NewServiceContainer(rdb, numCPU*2)

	// 初始化WHOIS服务提供商并添加到管理器
	whoisFreaksProvider := providers.NewWhoisFreaksProvider()
	whoisXMLProvider := providers.NewWhoisXMLProvider()
	ianaRDAPProvider := providers.NewIANARDAPProvider()
	ianaWhoisProvider := providers.NewIANAWhoisProvider()

	serviceContainer.WhoisManager.AddProvider(whoisFreaksProvider)
	serviceContainer.WhoisManager.AddProvider(whoisXMLProvider)
	serviceContainer.WhoisManager.AddProvider(ianaRDAPProvider)
	serviceContainer.WhoisManager.AddProvider(ianaWhoisProvider)

	// 初始化健康检查器
	serviceContainer.InitializeHealthChecker()

	// 异步初始化Chrome工具（完全非阻塞）
	log.Println("正在后台异步初始化Chrome工具...")
	port := getPort("8080") // 获取端口，以便在Chrome初始化失败时使用
	go func() {
		time.Sleep(3 * time.Second) // 延迟3秒启动，避免与主服务启动冲突

		log.Println("[CHROME] 开始后台初始化Chrome工具...")
		if err := utils.InitGlobalChromeUtil(); err != nil {
			log.Printf("[CHROME] Chrome工具初始化失败: %v，截图功能不可用", err)
			// 在所有启动检查结束后提示对外URL与监听端口
			publicURL := buildPublicURL(port)
			printReadyBanner(publicURL, port)
			return
		}

		log.Println("[CHROME] Chrome工具初始化成功")

		// 启动Chrome健康检查
		chromeUtil := utils.GetGlobalChromeUtil()
		if chromeUtil != nil {
			log.Println("[CHROME] Chrome工具已就绪，启动健康监控")
			chromeUtil.StartHealthMonitor()
		}

		// 在所有启动检查结束后提示对外URL与监听端口
		publicURL := buildPublicURL(port)
		printReadyBanner(publicURL, port)
	}()

	// 创建Gin引擎
	r := gin.Default()

	// 添加静态文件服务
	r.Static("/static/screenshots", "./static/screenshots")
	r.Static("/static/itdog", "./static/itdog")

	// 确保静态资源目录存在
	os.MkdirAll("./static/screenshots", 0755)
	os.MkdirAll("./static/itdog", 0755)

	// 启用CORS中间件
	corsConfig := getCorsConfig()
	r.Use(cors.New(corsConfig))

	// 配置中间件，注入服务组件到上下文
	r.Use(middleware.ServiceMiddleware(serviceContainer))

	// 注册API路由
	routes.RegisterAPIRoutes(r, serviceContainer)

	// 创建HTTP服务器，配置超时参数
	srv := &http.Server{
		Addr:           port,
		Handler:        r,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// 优雅关闭
	go func() {
		// 接收系统终止信号
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("正在关闭服务器...")

		// 关闭服务容器
		serviceContainer.Shutdown()

		// 停止Chrome工具（如果已初始化）
		if chromeUtil := utils.GetGlobalChromeUtil(); chromeUtil != nil {
			log.Println("[CHROME] 正在停止Chrome工具...")
			chromeUtil.Stop()
			log.Println("[CHROME] Chrome工具已停止")
		} else {
			log.Println("[CHROME] Chrome工具未初始化，无需停止")
		}

		// 设置关闭超时上下文
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("服务器被强制关闭: %v", err)
		}

		log.Println("服务器已安全关闭")
	}()

	// 启动服务
	log.Printf("服务器启动在端口%s，环境：%s", port, deriveEnvironment())
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
