/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-12-30
 * @Description: 统一日志系统 - 基于uber-go/zap
 */

package logger

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// base 是全局zap logger实例
	base *zap.Logger
	// sugar 是全局SugaredLogger实例，支持printf风格
	sugar *zap.SugaredLogger
)

// ContextKey 用于从context中获取request ID
type ContextKey string

const RequestIDKey ContextKey = "request_id"

// Init 初始化全局logger
// env: "dev" 使用开发模式（彩色输出），"production" 使用JSON格式
func Init(env string) error {
	var cfg zap.Config

	if env == "dev" || env == "development" {
		// 开发模式：易读的控制台格式
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		// 生产模式：JSON格式，便于日志聚合
		cfg = zap.NewProductionConfig()
	}

	// 统一时间格式
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.CallerKey = "caller"
	cfg.EncoderConfig.FunctionKey = "func" // 可选：包含函数名

	// 构建logger，添加caller信息（文件:行号）
	// AddCallerSkip(1) 跳过logger包装层，显示真实调用位置
	l, err := cfg.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel), // ERROR级别自动添加堆栈
	)
	if err != nil {
		return err
	}

	base = l
	sugar = l.Sugar()

	// 向后兼容：重定向标准库log到zap
	stdLog := zap.NewStdLog(l)
	log.SetOutput(stdLog.Writer())
	log.SetFlags(0) // zap已包含时间戳，移除标准库的

	return nil
}

// Module 创建带模块名称的logger
// 用法: logger.Module("Auth").Infof("user logged in: %s", username)
func Module(name string) *zap.SugaredLogger {
	if sugar == nil {
		// 如果未初始化，使用默认logger
		return zap.NewExample().Sugar().Named(name)
	}
	return sugar.Named(name)
}

// Base 返回原始zap.Logger，用于需要强类型的场景
func Base() *zap.Logger {
	if base == nil {
		return zap.NewExample()
	}
	return base
}

// Sugar 返回SugaredLogger，用于printf风格日志
func Sugar() *zap.SugaredLogger {
	if sugar == nil {
		return zap.NewExample().Sugar()
	}
	return sugar
}

// WithRequest 从Gin context中获取request ID并创建带request_id字段的logger
// 用法: log := logger.WithRequest(c, "Auth")
//       log.Infof("processing request")
func WithRequest(c *gin.Context, moduleName string) *zap.SugaredLogger {
	l := Module(moduleName)

	// 尝试从gin context获取request_id
	if requestID, exists := c.Get("request_id"); exists {
		l = l.With("request_id", requestID)
	}

	// 添加客户端IP
	l = l.With("client_ip", c.ClientIP())

	return l
}

// FromContext 从标准context.Context中获取request ID
// 用法: log := logger.FromContext(ctx, "Service")
func FromContext(ctx context.Context, moduleName string) *zap.SugaredLogger {
	l := Module(moduleName)

	if requestID := ctx.Value(RequestIDKey); requestID != nil {
		l = l.With("request_id", requestID)
	}

	return l
}

// Sync 刷新日志缓冲区，程序退出前应调用
func Sync() {
	if base != nil {
		_ = base.Sync()
	}
}

// DeriveEnvironment 根据环境变量推导运行环境
func DeriveEnvironment() string {
	if ginMode := os.Getenv("GIN_MODE"); ginMode != "" {
		if ginMode == "release" {
			return "production"
		}
		return "dev"
	}

	if env := os.Getenv("APP_ENV"); env != "" {
		return env
	}

	// 默认开发环境
	return "dev"
}
