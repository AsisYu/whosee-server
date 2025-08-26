/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-08-26 17:30:00
 * @Description: 健康检查专用日志记录器
 */
package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// HealthLogger 健康检查专用日志记录器
type HealthLogger struct {
	logger   *log.Logger
	logFile  *lumberjack.Logger
	mu       sync.RWMutex
	enabled  bool
	silent   bool // 是否静默模式（不输出到控制台）
}

var (
	healthLoggerInstance *HealthLogger
	healthLoggerOnce     sync.Once
)

// InitHealthLogger 初始化健康检查日志记录器
func InitHealthLogger() {
	healthLoggerOnce.Do(func() {
		healthLoggerInstance = NewHealthLogger()
	})
}

// GetHealthLogger 获取健康检查日志记录器单例
func GetHealthLogger() *HealthLogger {
	healthLoggerOnce.Do(func() {
		healthLoggerInstance = NewHealthLogger()
	})
	return healthLoggerInstance
}

// NewHealthLogger 创建新的健康检查日志记录器
func NewHealthLogger() *HealthLogger {
	// 确保日志目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("警告: 无法创建健康检查日志目录: %v", err)
	}

	// 创建健康检查专用的日志切割器
	logFile := &lumberjack.Logger{
		Filename:   fmt.Sprintf("logs/health_%s.log", time.Now().Format("2006-01-02")),
		MaxSize:    50,   // 每个日志文件最大大小，单位为MB
		MaxBackups: 15,   // 保留的旧日志文件最大数量
		MaxAge:     30,   // 保留旧日志文件的最大天数
		Compress:   true, // 是否压缩旧的日志文件
		LocalTime:  true, // 使用本地时间
	}

	// 检查是否启用健康检查日志分离
	enabled := os.Getenv("HEALTH_LOG_SEPARATE") == "true"
	silent := os.Getenv("HEALTH_LOG_SILENT") == "true"

	var output io.Writer
	if enabled {
		if silent {
			// 静默模式：只输出到文件
			output = logFile
		} else {
			// 同时输出到控制台和文件
			output = io.MultiWriter(os.Stdout, logFile)
		}
	} else {
		// 未启用分离，使用标准输出
		output = os.Stdout
	}

	// 创建专用的logger实例
	logger := log.New(output, "[HEALTH] ", log.Ldate|log.Ltime|log.Lshortfile)

	return &HealthLogger{
		logger:  logger,
		logFile: logFile,
		enabled: enabled,
		silent:  silent,
	}
}

// Printf 格式化输出健康检查日志
func (hl *HealthLogger) Printf(format string, v ...interface{}) {
	hl.mu.RLock()
	defer hl.mu.RUnlock()

	if hl.enabled {
		hl.logger.Printf(format, v...)
	} else {
		// 如果未启用分离，使用标准log输出
		log.Printf("[HEALTH] "+format, v...)
	}
}

// Println 输出健康检查日志
func (hl *HealthLogger) Println(v ...interface{}) {
	hl.mu.RLock()
	defer hl.mu.RUnlock()

	if hl.enabled {
		hl.logger.Println(v...)
	} else {
		// 如果未启用分离，使用标准log输出
		log.Println(append([]interface{}{"[HEALTH]"}, v...)...)
	}
}

// Print 输出健康检查日志
func (hl *HealthLogger) Print(v ...interface{}) {
	hl.mu.RLock()
	defer hl.mu.RUnlock()

	if hl.enabled {
		hl.logger.Print(v...)
	} else {
		// 如果未启用分离，使用标准log输出
		log.Print(append([]interface{}{"[HEALTH]"}, v...)...)
	}
}

// IsEnabled 检查是否启用了健康检查日志分离
func (hl *HealthLogger) IsEnabled() bool {
	hl.mu.RLock()
	defer hl.mu.RUnlock()
	return hl.enabled
}

// IsSilent 检查是否为静默模式
func (hl *HealthLogger) IsSilent() bool {
	hl.mu.RLock()
	defer hl.mu.RUnlock()
	return hl.silent
}

// GetLogFilePath 获取健康检查日志文件路径
func (hl *HealthLogger) GetLogFilePath() string {
	hl.mu.RLock()
	defer hl.mu.RUnlock()
	if hl.logFile != nil {
		return hl.logFile.Filename
	}
	return ""
}

// Close 关闭健康检查日志记录器
func (hl *HealthLogger) Close() error {
	hl.mu.Lock()
	defer hl.mu.Unlock()

	if hl.logFile != nil {
		return hl.logFile.Close()
	}
	return nil
}

// SetEnabled 动态设置是否启用健康检查日志分离
func (hl *HealthLogger) SetEnabled(enabled bool) {
	hl.mu.Lock()
	defer hl.mu.Unlock()

	hl.enabled = enabled

	// 重新配置输出
	var output io.Writer
	if enabled {
		if hl.silent {
			output = hl.logFile
		} else {
			output = io.MultiWriter(os.Stdout, hl.logFile)
		}
	} else {
		output = os.Stdout
	}

	hl.logger.SetOutput(output)
}

// SetSilent 动态设置是否为静默模式
func (hl *HealthLogger) SetSilent(silent bool) {
	hl.mu.Lock()
	defer hl.mu.Unlock()

	hl.silent = silent

	// 重新配置输出
	if hl.enabled {
		var output io.Writer
		if silent {
			output = hl.logFile
		} else {
			output = io.MultiWriter(os.Stdout, hl.logFile)
		}
		hl.logger.SetOutput(output)
	}
}

// LogHealthCheckStart 记录健康检查开始
func (hl *HealthLogger) LogHealthCheckStart(checkType string) {
	hl.Printf("=== 开始 %s 健康检查 ===", checkType)
}

// LogHealthCheckEnd 记录健康检查结束
func (hl *HealthLogger) LogHealthCheckEnd(checkType string, duration time.Duration) {
	hl.Printf("=== %s 健康检查完成，耗时: %v ===", checkType, duration)
}

// LogProviderTest 记录提供商测试结果
func (hl *HealthLogger) LogProviderTest(provider string, domain string, success bool, responseTime time.Duration, statusCode int) {
	status := "成功"
	if !success {
		status = "失败"
	}
	hl.Printf("提供商 %s 测试 %s: 域名=%s, 响应时间=%v, 状态码=%d", provider, status, domain, responseTime, statusCode)
}

// LogServiceStatus 记录服务状态
func (hl *HealthLogger) LogServiceStatus(service string, total, available int, status string) {
	hl.Printf("%s服务状态: 总数=%d, 可用=%d, 状态=%s", service, total, available, status)
}